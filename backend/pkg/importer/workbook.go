package importer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var investmentSheets = []string{"Jerivá 575", "Portugal 620-650", "Indaiá 643", "Jerivá 587"}
var requiredSheets = []string{"Estatísticas", "Aluguéis", "Matrículas", "Jerivá 575", "Portugal 620-650", "Indaiá 643", "Jerivá 587", "Débitos"}

type Preview struct {
	Source          string   `json:"source"`
	SHA256          string   `json:"sha256"`
	Sheets          []string `json:"sheets"`
	Properties      int      `json:"properties"`
	Receipts        int      `json:"receipts"`
	ReceiptsTotalC  int64    `json:"receipts_total_c"`
	Expenses        int      `json:"expenses"`
	ExpensesTotalC  int64    `json:"expenses_total_c"`
	Debts           int      `json:"debts"`
	DebtReceivedC   int64    `json:"debt_received_c"`
	DebtPaidC       int64    `json:"debt_paid_c"`
	FormulaRows     int      `json:"formula_rows"`
	Warnings        []string `json:"warnings"`
	AlreadyImported bool     `json:"already_imported"`
}

type Workbook struct{ DB *sql.DB }

func (w Workbook) Preview(path string) (Preview, error) {
	path = filepath.Clean(path)
	file, err := excelize.OpenFile(path, excelize.Options{RawCellValue: true})
	if err != nil {
		return Preview{}, err
	}
	defer file.Close()
	available := map[string]bool{}
	for _, sheet := range file.GetSheetList() {
		available[sheet] = true
	}
	missing := []string{}
	for _, sheet := range requiredSheets {
		if !available[sheet] {
			missing = append(missing, sheet)
		}
	}
	if len(missing) > 0 {
		return Preview{}, fmt.Errorf("abas esperadas não encontradas: %s", strings.Join(missing, ", "))
	}
	contents := map[string][][]string{}
	for _, sheet := range requiredSheets {
		rows, err := file.GetRows(sheet, excelize.Options{RawCellValue: true})
		if err != nil {
			return Preview{}, err
		}
		contents[sheet] = rows
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Preview{}, err
	}
	digest := sha256.Sum256(bytes)
	p := Preview{Source: filepath.Base(path), SHA256: hex.EncodeToString(digest[:]), Sheets: file.GetSheetList(), Warnings: []string{
		"Recebimentos são importados como lançamentos históricos; contratos e parcelas não são inferidos.",
		"O cadastro Indaiá 643 possui uma matrícula para duas unidades de aluguel; o vínculo legal é ambíguo.",
		"ROI da planilha é rendimento bruto ÷ custo; não corresponde ao ROI líquido do dashboard atual.",
	}}
	properties := map[string]bool{}
	for _, row := range dataRows(contents["Estatísticas"]) {
		if cell(row, 0) != "" && cell(row, 0) != "Totais" && number(cell(row, 1)) != 0 && number(cell(row, 2)) != 0 {
			properties[strings.TrimSpace(cell(row, 0))] = true
		}
	}
	for _, row := range dataRows(contents["Matrículas"]) {
		if cell(row, 3) != "" {
			properties[registryName(row)] = true
		}
	}
	p.Properties = len(properties)
	for _, row := range dataRows(contents["Aluguéis"]) {
		if cell(row, 0) == "" || cell(row, 3) == "" || number(cell(row, 6)) == 0 {
			continue
		}
		p.Receipts++
		p.ReceiptsTotalC += cents(cell(row, 6))
	}
	for _, sheet := range investmentSheets {
		for _, row := range dataRows(contents[sheet]) {
			amount := cents(cell(row, 4))
			if amount != 0 {
				p.Expenses++
				p.ExpensesTotalC += amount
			}
		}
	}
	for _, row := range dataRows(contents["Débitos"]) {
		if cell(row, 0) == "" || (cell(row, 2) == "" && cell(row, 3) == "") {
			continue
		}
		p.Debts++
		p.DebtReceivedC += cents(cell(row, 2))
		p.DebtPaidC += cents(cell(row, 3))
	}
	statRows, _ := file.GetRows("Estatísticas", excelize.Options{RawCellValue: true})
	for i := 1; i <= len(statRows); i++ {
		formula, err := file.GetCellFormula("Estatísticas", fmt.Sprintf("B%d", i))
		if err == nil && formula != "" {
			p.FormulaRows++
		}
	}
	var count int
	err = w.DB.QueryRow(`SELECT COUNT(*) FROM import_batches WHERE sha256=?`, p.SHA256).Scan(&count)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "no such table") {
		return Preview{}, err
	}
	p.AlreadyImported = count > 0
	return p, nil
}

func (w Workbook) Import(ctx context.Context, path string) (Preview, string, error) {
	p, err := w.Preview(path)
	if err != nil {
		return Preview{}, "", err
	}
	if p.AlreadyImported {
		return Preview{}, "", errors.New("esta planilha já foi importada para este banco")
	}
	backup := fmt.Sprintf("%s-pre-import-%s.sqlite", strings.TrimSuffix(pathDBName(w.DB), ".sqlite"), time.Now().Format("20060102-150405"))
	if _, err := w.DB.ExecContext(ctx, `VACUUM INTO ?`, backup); err != nil {
		return Preview{}, "", fmt.Errorf("criar backup antes da importação: %w", err)
	}
	file, err := excelize.OpenFile(path, excelize.Options{RawCellValue: true})
	if err != nil {
		return Preview{}, backup, err
	}
	defer file.Close()
	contents := map[string][][]string{}
	for _, sheet := range requiredSheets {
		rows, err := file.GetRows(sheet, excelize.Options{RawCellValue: true})
		if err != nil {
			return Preview{}, backup, err
		}
		contents[sheet] = rows
	}
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return Preview{}, backup, err
	}
	defer tx.Rollback()
	var already int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM import_batches WHERE sha256=?`, p.SHA256).Scan(&already); err != nil {
		return Preview{}, backup, err
	}
	if already > 0 {
		return Preview{}, backup, errors.New("esta planilha já foi importada para este banco")
	}
	var firstMovement, firstDebt int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM movimentacoes`).Scan(&firstMovement); err != nil {
		return Preview{}, backup, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM dividas`).Scan(&firstDebt); err != nil {
		return Preview{}, backup, err
	}
	projects := map[string]int64{}
	for _, name := range investmentSheets {
		id, err := ensureProject(ctx, tx, name)
		if err != nil {
			return Preview{}, backup, err
		}
		projects[name] = id
	}
	propertyIDs := map[string]int64{}
	warnings := append([]string{}, p.Warnings...)
	statSource := map[string]string{}
	for i, row := range dataRows(contents["Estatísticas"]) {
		formula, _ := file.GetCellFormula("Estatísticas", fmt.Sprintf("B%d", i+2))
		for _, name := range investmentSheets {
			if strings.Contains(formula, name) && cell(row, 0) != "" {
				statSource[strings.TrimSpace(cell(row, 0))] = name
				break
			}
		}
	}
	for _, row := range dataRows(contents["Estatísticas"]) {
		name := strings.TrimSpace(cell(row, 0))
		if name == "" || name == "Totais" {
			continue
		}
		if number(cell(row, 1)) == 0 && number(cell(row, 2)) == 0 {
			continue
		}
		var project *int64
		if source := statSource[name]; source != "" {
			value := projects[source]
			project = &value
		}
		address, neighborhood := "Rua Indaiá, 643", "Buritis V"
		if strings.HasPrefix(name, "Portugal") {
			address, neighborhood = "Rua Portugal", "Jd. Europa III"
		}
		id, err := ensureProperty(ctx, tx, name, address, "casa", neighborhood, project, "", "", "", "", "", "proprio")
		if err != nil {
			return Preview{}, backup, err
		}
		propertyIDs[name] = id
	}
	for _, row := range dataRows(contents["Matrículas"]) {
		if cell(row, 3) == "" && cell(row, 2) == "" {
			continue
		}
		name := registryName(row)
		var project *int64
		switch {
		case strings.HasPrefix(name, "Portugal"):
			v := projects["Portugal 620-650"]
			project = &v
		case strings.Contains(cell(row, 3), "Jerivá, 575"):
			v := projects["Jerivá 575"]
			project = &v
		case strings.Contains(cell(row, 3), "Jerivá, 587"):
			v := projects["Jerivá 587"]
			project = &v
		case strings.HasPrefix(name, "Indaiá 643"):
			v := projects["Indaiá 643"]
			project = &v
		}
		patrimonial := "proprio"
		if strings.Contains(strings.ToLower(cell(row, 0)), "vendido") {
			patrimonial = "vendido"
		}
		id, err := ensureProperty(ctx, tx, name, cell(row, 3), strings.ToLower(defaultValue(cell(row, 2), "outros")), cell(row, 1), project, cell(row, 4), cell(row, 5), cell(row, 6), cell(row, 7), cell(row, 8), patrimonial)
		if err != nil {
			return Preview{}, backup, err
		}
		propertyIDs[name] = id
	}
	registryCandidates := map[string][]int64{"Jerivá 575": {}, "Jerivá 587": {}}
	for name, id := range propertyIDs {
		if strings.Contains(name, "Jerivá · Galpão") || strings.Contains(name, "Jerivá, 575") {
			registryCandidates["Jerivá 575"] = append(registryCandidates["Jerivá 575"], id)
		}
		if strings.Contains(name, "Jerivá, 587") {
			registryCandidates["Jerivá 587"] = append(registryCandidates["Jerivá 587"], id)
		}
	}
	for _, sheet := range investmentSheets {
		for index, row := range dataRows(contents[sheet]) {
			raw := cell(row, 4)
			signed := cents(raw)
			if raw == "" || signed == 0 {
				continue
			}
			when, err := parseDate(cell(row, 0))
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("%s!linha %d: despesa sem data ou valor válido, ignorada.", sheet, index+2))
				continue
			}
			amount := signed
			if amount < 0 {
				amount = -amount
			}
			targets := expenseTargets(sheet, propertyIDs)
			if len(registryCandidates[sheet]) > 0 {
				targets = registryCandidates[sheet]
			}
			if len(targets) == 0 {
				warnings = append(warnings, fmt.Sprintf("%s!linha %d: sem unidade para ratear o custo.", sheet, index+2))
			}
			result, err := tx.ExecContext(ctx, `INSERT INTO movimentacoes(tipo,categoria,data,valor_c,descricao,empreendimento_id,referencia,forma_pagamento,fornecedor,observacao,reversao) VALUES('despesa',?,?,?,?,?,?,?,?,?,?)`, defaultValue(cell(row, 1), "outros"), when, amount, cell(row, 3), projects[sheet], "", cell(row, 5), cell(row, 2), "", boolInt(signed < 0))
			if err != nil {
				return Preview{}, backup, err
			}
			movementID, err := result.LastInsertId()
			if err != nil {
				return Preview{}, backup, err
			}
			if len(targets) > 0 {
				each, remainder := amount/int64(len(targets)), amount%int64(len(targets))
				for i, propertyID := range targets {
					piece := each
					if i == 0 {
						piece += remainder
					}
					if _, err := tx.ExecContext(ctx, `INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(?,?,?)`, movementID, propertyID, piece); err != nil {
						return Preview{}, backup, err
					}
				}
			}
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.imovel_id,SUM(CASE WHEN m.reversao=1 THEN -r.valor_c ELSE r.valor_c END) FROM movimentacao_rateios r JOIN movimentacoes m ON m.id=r.movimentacao_id WHERE m.id>? AND m.tipo='despesa' AND m.empreendimento_id IS NOT NULL GROUP BY r.imovel_id`, firstMovement)
	if err != nil {
		return Preview{}, backup, err
	}
	type aggregate struct{ id, amount int64 }
	totals := []aggregate{}
	for rows.Next() {
		var item aggregate
		if err := rows.Scan(&item.id, &item.amount); err != nil {
			rows.Close()
			return Preview{}, backup, err
		}
		totals = append(totals, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Preview{}, backup, err
	}
	for _, item := range totals {
		if _, err := tx.ExecContext(ctx, `UPDATE imoveis SET investimento_importado_c=investimento_importado_c+? WHERE id=?`, item.amount, item.id); err != nil {
			return Preview{}, backup, err
		}
	}
	tenants := map[string]int64{}
	for index, row := range dataRows(contents["Aluguéis"]) {
		if cell(row, 0) == "" {
			continue
		}
		if cell(row, 3) == "" || number(cell(row, 6)) == 0 {
			warnings = append(warnings, fmt.Sprintf("Aluguéis!linha %d: recebimento sem unidade/valor foi ignorado.", index+2))
			continue
		}
		propertyName := cell(row, 3)
		propertyID, exists := propertyIDs[propertyName]
		if !exists {
			propertyID, err = ensureProperty(ctx, tx, propertyName, "", "casa", "", nil, "", "", "", "", "", "proprio")
			if err != nil {
				return Preview{}, backup, err
			}
			propertyIDs[propertyName] = propertyID
		}
		tenantName := cell(row, 4)
		var tenantID any
		if tenantName != "" {
			id, ok := tenants[tenantName]
			if !ok {
				err = tx.QueryRowContext(ctx, `SELECT id FROM inquilinos WHERE nome=? ORDER BY id LIMIT 1`, tenantName).Scan(&id)
				if errors.Is(err, sql.ErrNoRows) {
					result, e := tx.ExecContext(ctx, `INSERT INTO inquilinos(nome) VALUES(?)`, tenantName)
					if e != nil {
						return Preview{}, backup, e
					}
					id, e = result.LastInsertId()
					if e != nil {
						return Preview{}, backup, e
					}
				} else if err != nil {
					return Preview{}, backup, err
				}
				tenants[tenantName] = id
			}
			tenantID = id
		}
		category := strings.ToLower(defaultValue(cell(row, 7), "aluguel"))
		if strings.Contains(category, "mensalidade") {
			category = "aluguel"
		}
		when, err := parseDate(cell(row, 0))
		if err != nil {
			return Preview{}, backup, err
		}
		reference := ""
		if cell(row, 5) != "" {
			referenceDate, e := parseDate(cell(row, 5))
			if e == nil {
				reference = strings.ReplaceAll(referenceDate[:7], "-", "/")
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO movimentacoes(tipo,categoria,data,valor_c,descricao,imovel_id,inquilino_id,referencia,origem,observacao) VALUES('receita',?,?,?,?,?,?,?,?,?)`, category, when, cents(cell(row, 6)), cell(row, 7), propertyID, tenantID, reference, cell(row, 2), "Status original: "+defaultValue(cell(row, 1), "—")); err != nil {
			return Preview{}, backup, err
		}
	}
	type debtEntry struct {
		date        string
		amount      int64
		description string
	}
	type debtGroup struct{ releases, payments []debtEntry }
	debtGroups := map[string]*debtGroup{}
	debtOrder := []string{}
	for _, row := range dataRows(contents["Débitos"]) {
		if cell(row, 0) == "" || (cell(row, 2) == "" && cell(row, 3) == "") {
			continue
		}
		lender := defaultValue(cell(row, 4), "Credor não informado")
		group := debtGroups[lender]
		if group == nil {
			group = &debtGroup{}
			debtGroups[lender] = group
			debtOrder = append(debtOrder, lender)
		}
		when, err := parseDate(cell(row, 0))
		if err != nil {
			return Preview{}, backup, err
		}
		description := defaultValue(cell(row, 1), "Liberação importada")
		if amount := cents(cell(row, 2)); amount != 0 {
			group.releases = append(group.releases, debtEntry{when, amount, description})
		}
		if amount := cents(cell(row, 3)); amount != 0 {
			group.payments = append(group.payments, debtEntry{when, amount, defaultValue(cell(row, 1), "Pagamento importado")})
		}
	}
	importedDebtGroups := 0
	for _, lender := range debtOrder {
		group := debtGroups[lender]
		if len(group.releases) == 0 {
			warnings = append(warnings, "Débitos: pagamentos de "+lender+" sem liberação identificável; não importados como dívida.")
			continue
		}
		first := group.releases[0]
		result, err := tx.ExecContext(ctx, `INSERT INTO dividas(credor,descricao,valor_original_c,data_contratacao) VALUES(?,?,?,?)`, lender, "Importado da planilha", first.amount, first.date)
		if err != nil {
			return Preview{}, backup, err
		}
		debtID, err := result.LastInsertId()
		if err != nil {
			return Preview{}, backup, err
		}
		importedDebtGroups++
		for _, release := range group.releases[1:] {
			if _, err := tx.ExecContext(ctx, `INSERT INTO movimentos_divida(divida_id,tipo,data,valor_c,descricao) VALUES(?,'liberacao',?,?,?)`, debtID, release.date, release.amount, release.description); err != nil {
				return Preview{}, backup, err
			}
		}
		for _, payment := range group.payments {
			if _, err := tx.ExecContext(ctx, `INSERT INTO movimentos_divida(divida_id,tipo,data,valor_c,descricao) VALUES(?,'pagamento',?,?,?)`, debtID, payment.date, payment.amount, payment.description); err != nil {
				return Preview{}, backup, err
			}
		}
	}
	var receiptCount, receiptTotal, expenseCount, expenseTotal int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(valor_c),0) FROM movimentacoes WHERE id>? AND tipo='receita'`, firstMovement).Scan(&receiptCount, &receiptTotal); err != nil {
		return Preview{}, backup, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(CASE WHEN reversao=1 THEN -valor_c ELSE valor_c END),0) FROM movimentacoes WHERE id>? AND tipo='despesa'`, firstMovement).Scan(&expenseCount, &expenseTotal); err != nil {
		return Preview{}, backup, err
	}
	if receiptCount != int64(p.Receipts) || receiptTotal != p.ReceiptsTotalC {
		return Preview{}, backup, errors.New("conciliação de recebimentos divergiu da planilha; importação cancelada")
	}
	if expenseCount != int64(p.Expenses) || expenseTotal != p.ExpensesTotalC {
		return Preview{}, backup, errors.New("conciliação de despesas divergiu da planilha; importação cancelada")
	}
	var debtReceived, debtPaid int64
	for _, lender := range debtOrder {
		group := debtGroups[lender]
		if len(group.releases) > 0 {
			for _, entry := range group.releases {
				debtReceived += entry.amount
			}
			for _, entry := range group.payments {
				debtPaid += entry.amount
			}
		}
	}
	if debtReceived != p.DebtReceivedC || debtPaid != p.DebtPaidC {
		return Preview{}, backup, errors.New("conciliação de débitos divergiu da planilha; importação cancelada")
	}
	totalsJSON := fmt.Sprintf(`{"receipts_c":%d,"expenses_c":%d,"debt_received_c":%d,"debt_paid_c":%d}`, receiptTotal, expenseTotal, debtReceived, debtPaid)
	warningsJSON := toJSON(warnings)
	if _, err := tx.ExecContext(ctx, `INSERT INTO import_batches(source_name,sha256,properties_count,receipts_count,expenses_count,debts_count,warnings_json,totals_json) VALUES(?,?,?,?,?,?,?,?)`, p.Source, p.SHA256, p.Properties, p.Receipts, p.Expenses, importedDebtGroups, warningsJSON, totalsJSON); err != nil {
		return Preview{}, backup, err
	}
	if err := tx.Commit(); err != nil {
		return Preview{}, backup, err
	}
	p.Warnings = warnings
	return p, backup, nil
}

func pathDBName(db *sql.DB) string {
	var path string
	_ = db.QueryRow(`PRAGMA database_list`).Scan(new(int), new(string), &path)
	if path == "" {
		path = "data/rerum.sqlite"
	}
	return path
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
func ensureProject(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO empreendimentos(nome) VALUES(?)`, name); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM empreendimentos WHERE nome=?`, name).Scan(&id)
	return id, err
}
func ensureProperty(ctx context.Context, tx *sql.Tx, name, address, kind, neighborhood string, projectID *int64, iptu, registry, water, energy, observation, patrimonial string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM imoveis WHERE apelido=? ORDER BY id LIMIT 1`, name).Scan(&id)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE imoveis SET endereco=CASE WHEN endereco='' THEN ? ELSE endereco END,bairro=CASE WHEN bairro='' THEN ? ELSE bairro END,matricula_cartorio=CASE WHEN matricula_cartorio='' THEN ? ELSE matricula_cartorio END,inscricao_iptu=CASE WHEN inscricao_iptu='' THEN ? ELSE inscricao_iptu END,unidade_agua=CASE WHEN unidade_agua='' THEN ? ELSE unidade_agua END,unidade_energia=CASE WHEN unidade_energia='' THEN ? ELSE unidade_energia END,empreendimento_id=COALESCE(empreendimento_id,?),situacao_patrimonial=CASE WHEN ?='vendido' THEN 'vendido' ELSE situacao_patrimonial END WHERE id=?`, address, neighborhood, registry, iptu, water, energy, projectID, patrimonial, id)
		return id, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO imoveis(apelido,endereco,tipo,status,observacao,matricula_cartorio,inscricao_iptu,unidade_agua,unidade_energia,empreendimento_id,bairro,situacao_patrimonial) VALUES(?,?,?,'disponivel',?,?,?,?,?,?,?,?)`, name, address, kind, observation, registry, iptu, water, energy, projectID, neighborhood, defaultValue(patrimonial, "proprio"))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
func expenseTargets(sheet string, properties map[string]int64) []int64 {
	keys := []string{}
	switch sheet {
	case "Indaiá 643":
		keys = []string{"Indaiá 643-1", "Indaiá 643-2"}
	case "Portugal 620-650":
		for _, street := range []int{620, 630, 640, 650} {
			for unit := 1; unit <= 2; unit++ {
				keys = append(keys, fmt.Sprintf("Portugal %d-%d", street, unit))
			}
		}
	case "Jerivá 575":
		for key := range properties {
			if strings.Contains(key, "Jerivá 575") {
				keys = append(keys, key)
			}
		}
	default:
		for key := range properties {
			if strings.Contains(key, "Jerivá 587") {
				keys = append(keys, key)
			}
		}
	}
	found := []int64{}
	seen := map[int64]bool{}
	for _, key := range keys {
		if id, ok := properties[key]; ok && !seen[id] {
			found = append(found, id)
			seen[id] = true
		}
	}
	return found
}

func toJSON(value []string) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for i, item := range value {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.Quote(item))
	}
	builder.WriteByte(']')
	return builder.String()
}

func dataRows(rows [][]string) [][]string {
	if len(rows) < 2 {
		return nil
	}
	return rows[1:]
}
func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}
func number(value string) float64 {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "R$", ""), " ", ""))
	if value == "" || value == "-" || value == "—" {
		return 0
	}
	if strings.Contains(value, ",") {
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	}
	n, _ := strconv.ParseFloat(value, 64)
	return n
}
func cents(value string) int64 { return int64(math.Round(number(value) * 100)) }
func parseDate(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "-" {
		return "", errors.New("data ausente")
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "02/01/2006", "1/2/06", "02/01/06"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}
	serial, err := strconv.ParseFloat(value, 64)
	if err == nil {
		parsed, err := excelize.ExcelDateToTime(serial, false)
		if err == nil {
			return parsed.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("data inválida: %s", value)
}

var divisor = regexp.MustCompile(`/\s*(\d+)`)

func registryName(row []string) string {
	neighborhood, kind, address, registry := cell(row, 1), cell(row, 2), cell(row, 3), cell(row, 5)
	if strings.HasPrefix(kind, "Casa 6") || strings.HasPrefix(kind, "Casa 5") {
		return "Portugal " + strings.TrimPrefix(kind, "Casa ")
	}
	if strings.Contains(strings.ToLower(address), "indaiá") && strings.EqualFold(kind, "kitnet") {
		return "Indaiá 643 · cadastro de matrícula"
	}
	if neighborhood == "" {
		neighborhood = "Sem bairro"
	}
	if kind == "" {
		kind = "Imóvel"
	}
	if address == "" {
		address = registry
	}
	if address == "" {
		address = "sem endereço"
	}
	return neighborhood + " · " + kind + " · " + address
}

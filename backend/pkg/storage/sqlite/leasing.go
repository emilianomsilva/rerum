package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"rerum/backend/pkg/domain"
	"rerum/backend/pkg/receipt"
)

func (s *Store) Leases(ctx context.Context, propertyID *int64, active *bool) ([]domain.Locacao, error) {
	query := `SELECT id,imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c,multa_pct,juros_dia_pct,reajuste_indice,status,COALESCE(intermediario,'Contrato direto'),COALESCE(observacao,''),primeiro_vencimento FROM locacoes WHERE 1=1`
	args := make([]any, 0, 2)
	if propertyID != nil {
		query += ` AND imovel_id=?`
		args = append(args, *propertyID)
	}
	if active != nil {
		if *active {
			query += ` AND status='ativa'`
		} else {
			query += ` AND status='encerrada'`
		}
	}
	query += ` ORDER BY inicio DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Locacao, 0)
	for rows.Next() {
		item, err := scanLease(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) LeaseSummaries(ctx context.Context, propertyID *int64, active *bool) ([]domain.LeaseSummary, error) {
	query := `SELECT l.id,l.imovel_id,l.inquilino_id,l.inicio,l.fim,l.dia_vencimento,l.valor_mensal_c,l.multa_pct,l.juros_dia_pct,l.reajuste_indice,l.status,COALESCE(l.intermediario,'Contrato direto'),COALESCE(l.observacao,''),l.primeiro_vencimento,i.apelido,t.nome,COUNT(p.id),MIN(p.periodo_inicio),MAX(p.periodo_fim) FROM locacoes l JOIN imoveis i ON i.id=l.imovel_id JOIN inquilinos t ON t.id=l.inquilino_id LEFT JOIN parcelas p ON p.locacao_id=l.id WHERE 1=1`
	args := make([]any, 0, 2)
	if propertyID != nil {
		query += ` AND l.imovel_id=?`
		args = append(args, *propertyID)
	}
	if active != nil {
		if *active {
			query += ` AND l.status='ativa'`
		} else {
			query += ` AND l.status='encerrada'`
		}
	}
	query += ` GROUP BY l.id ORDER BY l.inicio DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.LeaseSummary{}
	for rows.Next() {
		var item domain.LeaseSummary
		var start, end, firstDue sql.NullString
		if err := rows.Scan(&item.ID, &item.ImovelID, &item.InquilinoID, &item.Inicio, &item.Fim, &item.DiaVencimento, &item.ValorMensalC, &item.MultaPct, &item.JurosDiaPct, &item.ReajusteIndice, &item.Status, &item.Intermediario, &item.Observacao, &firstDue, &item.Imovel, &item.Inquilino, &item.Parcelas, &start, &end); err != nil {
			return nil, err
		}
		if firstDue.Valid {
			item.PrimeiroVencimento = &firstDue.String
		}
		if start.Valid {
			item.PeriodoInicio = &start.String
		}
		if end.Valid {
			item.PeriodoFim = &end.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Lease(ctx context.Context, id int64) (domain.Locacao, error) {
	return lease(ctx, s.DB, id)
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func lease(ctx context.Context, query rowQuerier, id int64) (domain.Locacao, error) {
	item, err := scanLease(query.QueryRowContext(ctx, `SELECT id,imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c,multa_pct,juros_dia_pct,reajuste_indice,status,COALESCE(intermediario,'Contrato direto'),COALESCE(observacao,''),primeiro_vencimento FROM locacoes WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func scanLease(row scanner) (domain.Locacao, error) {
	var item domain.Locacao
	var firstDue sql.NullString
	err := row.Scan(&item.ID, &item.ImovelID, &item.InquilinoID, &item.Inicio, &item.Fim, &item.DiaVencimento, &item.ValorMensalC, &item.MultaPct, &item.JurosDiaPct, &item.ReajusteIndice, &item.Status, &item.Intermediario, &item.Observacao, &firstDue)
	if err != nil {
		return item, err
	}
	if firstDue.Valid {
		item.PrimeiroVencimento = &firstDue.String
	}
	return item, nil
}

func (s *Store) CreateLease(ctx context.Context, item domain.Locacao) (domain.Locacao, []domain.Parcela, error) {
	property, err := s.Property(ctx, item.ImovelID)
	if err != nil || !property.Ativo {
		return item, nil, fmt.Errorf("imóvel %d não encontrado ou inativo", item.ImovelID)
	}
	if property.SituacaoPatrimonial == "vendido" {
		return item, nil, errors.New("imóvel vendido não pode receber nova locação")
	}
	if property.Status == "manutencao" {
		return item, nil, errors.New("imóvel em manutenção não pode receber nova locação")
	}
	tenant, err := s.Tenant(ctx, item.InquilinoID)
	if err != nil || !tenant.Ativo {
		return item, nil, fmt.Errorf("inquilino %d não encontrado ou inativo", item.InquilinoID)
	}
	active, err := s.Leases(ctx, &item.ImovelID, ptr(true))
	if err != nil {
		return item, nil, err
	}
	if len(active) != 0 {
		return item, nil, fmt.Errorf("imóvel %d já possui locação ativa", item.ImovelID)
	}
	item.Status = "ativa"
	if item.Intermediario == "" {
		item.Intermediario = "Contrato direto"
	}
	installments, err := domain.GenerateInstallments(item)
	if err != nil {
		return item, nil, err
	}
	if len(installments) == 0 {
		return item, nil, errors.New("período não gerou parcelas")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return item, nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c,multa_pct,juros_dia_pct,reajuste_indice,status,intermediario,observacao,primeiro_vencimento) VALUES(?,?,?,?,?,?,?,?,?,'ativa',?,?,?)`, item.ImovelID, item.InquilinoID, item.Inicio, item.Fim, item.DiaVencimento, item.ValorMensalC, item.MultaPct, item.JurosDiaPct, item.ReajusteIndice, item.Intermediario, item.Observacao, item.PrimeiroVencimento)
	if err != nil {
		return item, nil, err
	}
	item.ID, err = result.LastInsertId()
	if err != nil {
		return item, nil, err
	}
	for i := range installments {
		installments[i].LocacaoID = item.ID
		p := installments[i]
		parcelResult, execErr := tx.ExecContext(ctx, `INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,multa_c,juros_c,desconto_c,valor_pago_c,status,numero,total_parcelas,periodo_inicio,periodo_fim,tipo,observacao) VALUES(?,?,?,?,0,0,0,0,?,?,?,?,?,?,?)`, p.LocacaoID, p.Competencia, p.Vencimento, p.ValorBaseC, p.Status, p.Numero, p.TotalParcelas, p.PeriodoInicio, p.PeriodoFim, p.Tipo, p.Observacao)
		err = execErr
		if err != nil {
			return item, nil, err
		}
		installments[i].ID, err = parcelResult.LastInsertId()
		if err != nil {
			return item, nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return item, nil, err
	}
	return item, installments, nil
}

func (s *Store) UpdateLease(ctx context.Context, item domain.Locacao) (domain.Locacao, error) {
	current, err := s.Lease(ctx, item.ID)
	if err != nil {
		return item, err
	}
	if item.DiaVencimento < 1 || item.DiaVencimento > 31 {
		item.DiaVencimento = current.DiaVencimento
	}
	if item.ValorMensalC <= 0 {
		item.ValorMensalC = current.ValorMensalC
	}
	if item.Intermediario == "" {
		item.Intermediario = current.Intermediario
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE locacoes SET dia_vencimento=?, valor_mensal_c=?, multa_pct=?, juros_dia_pct=?, reajuste_indice=?, intermediario=?, observacao=? WHERE id=?`, item.DiaVencimento, item.ValorMensalC, item.MultaPct, item.JurosDiaPct, item.ReajusteIndice, item.Intermediario, item.Observacao, item.ID)
	if err != nil {
		return item, err
	}
	return s.Lease(ctx, item.ID)
}

func (s *Store) RenewLease(ctx context.Context, id int64, newEnd string, newMonthlyValueC int64) (domain.Locacao, []domain.Parcela, error) {
	lease, err := s.Lease(ctx, id)
	if err != nil {
		return lease, nil, err
	}
	if lease.Status != "ativa" {
		return lease, nil, fmt.Errorf("somente contratos ativos podem ser renovados (status atual: %s)", lease.Status)
	}
	endDate, err := domain.ParseDate(newEnd)
	if err != nil {
		return lease, nil, fmt.Errorf("data de término inválida: %w", err)
	}
	oldEnd, err := domain.ParseDate(lease.Fim)
	if err != nil {
		return lease, nil, fmt.Errorf("data de término anterior inválida: %w", err)
	}
	if !endDate.After(oldEnd) {
		return lease, nil, errors.New("a nova data de término deve ser posterior ao término atual")
	}
	if newMonthlyValueC <= 0 {
		newMonthlyValueC = lease.ValorMensalC
	}

	var maxNumber int
	err = s.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(numero), 0) FROM parcelas WHERE locacao_id=?`, id).Scan(&maxNumber)
	if err != nil {
		return lease, nil, err
	}

	renewStart := oldEnd.AddDate(0, 0, 1).Format("2006-01-02")
	tempLease := lease
	tempLease.Inicio = renewStart
	tempLease.Fim = newEnd
	tempLease.ValorMensalC = newMonthlyValueC
	tempLease.PrimeiroVencimento = nil
	newInstallments, err := domain.GenerateInstallments(tempLease)
	if err != nil {
		return lease, nil, err
	}
	if len(newInstallments) == 0 {
		return lease, nil, errors.New("período de renovação não gerou novas parcelas")
	}

	totalAll := maxNumber + len(newInstallments)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return lease, nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE locacoes SET fim=?, valor_mensal_c=? WHERE id=?`, newEnd, newMonthlyValueC, id); err != nil {
		return lease, nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE parcelas SET total_parcelas=? WHERE locacao_id=?`, totalAll, id); err != nil {
		return lease, nil, err
	}

	inserted := make([]domain.Parcela, len(newInstallments))
	for i, p := range newInstallments {
		p.LocacaoID = id
		p.Numero = maxNumber + i + 1
		p.TotalParcelas = totalAll
		res, execErr := tx.ExecContext(ctx, `INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,multa_c,juros_c,desconto_c,valor_pago_c,status,numero,total_parcelas,periodo_inicio,periodo_fim,tipo,observacao) VALUES(?,?,?,?,0,0,0,0,?,?,?,?,?,?,'')`, p.LocacaoID, p.Competencia, p.Vencimento, p.ValorBaseC, p.Status, p.Numero, p.TotalParcelas, p.PeriodoInicio, p.PeriodoFim, p.Tipo)
		if execErr != nil {
			return lease, nil, execErr
		}
		p.ID, _ = res.LastInsertId()
		inserted[i] = p
	}
	if err := tx.Commit(); err != nil {
		return lease, nil, err
	}
	lease, _ = s.Lease(ctx, id)
	return lease, inserted, nil
}

func (s *Store) DeleteLease(ctx context.Context, id int64) error {
	var count int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM locacoes WHERE id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacoes WHERE locacao_id=? OR parcela_id IN (SELECT id FROM parcelas WHERE locacao_id=?)`, id, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM parcelas WHERE locacao_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM locacoes WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CloseLease(ctx context.Context, id int64, today string) (int, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE locacoes SET status='encerrada' WHERE id=? AND status='ativa'`, id)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM locacoes WHERE id=?`, id).Scan(&exists); err != nil {
			return 0, err
		}
		if exists == 0 {
			return 0, ErrNotFound
		}
		return 0, tx.Commit()
	}
	result, err = tx.ExecContext(ctx, `UPDATE parcelas SET status='cancelada' WHERE locacao_id=? AND vencimento>=? AND status='aberta' AND valor_pago_c=0`, id, today)
	if err != nil {
		return 0, err
	}
	cancelled, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(cancelled), tx.Commit()
}

func (s *Store) CancelFutureInstallments(ctx context.Context, leaseID int64, today string) (int, error) {
	var exists int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM locacoes WHERE id=?`, leaseID).Scan(&exists); err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, ErrNotFound
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE parcelas SET status='cancelada' WHERE locacao_id=? AND vencimento>=? AND status='aberta' AND valor_pago_c=0`, leaseID, today)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	return int(count), err
}

func (s *Store) Installments(ctx context.Context, leaseID, propertyID *int64) ([]domain.Parcela, error) {
	query := `SELECT p.id,p.locacao_id,p.competencia,p.vencimento,p.valor_base_c,p.multa_c,p.juros_c,p.desconto_c,p.valor_pago_c,p.data_pagamento,p.forma,p.status,p.recibo_path,p.numero,p.total_parcelas,p.periodo_inicio,p.periodo_fim,p.tipo,COALESCE(p.observacao,''),i.apelido,t.nome,p.valor_base_c+p.multa_c+p.juros_c-p.desconto_c,MAX(0,p.valor_base_c+p.multa_c+p.juros_c-p.desconto_c-p.valor_pago_c) FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id JOIN imoveis i ON i.id=l.imovel_id JOIN inquilinos t ON t.id=l.inquilino_id WHERE 1=1`
	args := make([]any, 0, 2)
	if leaseID != nil {
		query += ` AND p.locacao_id=?`
		args = append(args, *leaseID)
	}
	if propertyID != nil {
		query += ` AND l.imovel_id=?`
		args = append(args, *propertyID)
	}
	query += ` ORDER BY p.vencimento,p.id`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Parcela, 0)
	for rows.Next() {
		item, err := scanInstallmentSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanInstallmentSummary(row scanner) (domain.Parcela, error) {
	var item domain.Parcela
	var paid, form, receipt, start, end sql.NullString
	err := row.Scan(&item.ID, &item.LocacaoID, &item.Competencia, &item.Vencimento, &item.ValorBaseC, &item.MultaC, &item.JurosC, &item.DescontoC, &item.ValorPagoC, &paid, &form, &item.Status, &receipt, &item.Numero, &item.TotalParcelas, &start, &end, &item.Tipo, &item.Observacao, &item.Imovel, &item.Inquilino, &item.TotalDevidoC, &item.SaldoC)
	if err != nil {
		return item, err
	}
	if paid.Valid {
		item.DataPagamento = &paid.String
	}
	if form.Valid {
		item.Forma = &form.String
	}
	if receipt.Valid {
		item.ReciboPath = &receipt.String
	}
	if start.Valid {
		item.PeriodoInicio = &start.String
	}
	if end.Valid {
		item.PeriodoFim = &end.String
	}
	return item, nil
}

func (s *Store) Installment(ctx context.Context, id int64) (domain.Parcela, error) {
	return installment(ctx, s.DB, id)
}

func installment(ctx context.Context, query rowQuerier, id int64) (domain.Parcela, error) {
	item, err := scanInstallment(query.QueryRowContext(ctx, `SELECT id,locacao_id,competencia,vencimento,valor_base_c,multa_c,juros_c,desconto_c,valor_pago_c,data_pagamento,forma,status,recibo_path,numero,total_parcelas,periodo_inicio,periodo_fim,tipo,COALESCE(observacao,'') FROM parcelas WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	return item, err
}

func scanInstallment(row scanner) (domain.Parcela, error) {
	var item domain.Parcela
	var paid, form, receipt, start, end sql.NullString
	err := row.Scan(&item.ID, &item.LocacaoID, &item.Competencia, &item.Vencimento, &item.ValorBaseC, &item.MultaC, &item.JurosC, &item.DescontoC, &item.ValorPagoC, &paid, &form, &item.Status, &receipt, &item.Numero, &item.TotalParcelas, &start, &end, &item.Tipo, &item.Observacao)
	if err != nil {
		return item, err
	}
	if paid.Valid {
		item.DataPagamento = &paid.String
	}
	if form.Valid {
		item.Forma = &form.String
	}
	if receipt.Valid {
		item.ReciboPath = &receipt.String
	}
	if start.Valid {
		item.PeriodoInicio = &start.String
	}
	if end.Valid {
		item.PeriodoFim = &end.String
	}
	return item, nil
}

func (s *Store) CreateExtraInstallment(ctx context.Context, p domain.Parcela) (domain.Parcela, error) {
	lease, err := s.Lease(ctx, p.LocacaoID)
	if err != nil {
		return p, err
	}
	if err := validateInstallmentAmounts(p); err != nil {
		return p, err
	}
	if p.ValorPagoC != 0 || p.DataPagamento != nil || p.ReciboPath != nil || p.Forma != nil {
		return p, errors.New("não pode criar parcela com recebimentos já registrados")
	}
	if p.Vencimento == "" {
		return p, errors.New("o vencimento é obrigatório")
	}
	if _, err := domain.ParseDate(p.Vencimento); err != nil {
		return p, fmt.Errorf("vencimento inválido: %w", err)
	}
	if p.Competencia == "" {
		p.Competencia = p.Vencimento[:7]
	}
	if _, err := domain.ParseDate(p.Competencia + "-01"); err != nil {
		return p, errors.New("competência inválida")
	}
	if p.Tipo == "" {
		p.Tipo = "taxa"
	}
	p.Status = "aberta"
	p.Numero, p.TotalParcelas = 0, 0
	res, err := s.DB.ExecContext(ctx, `INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,multa_c,juros_c,desconto_c,valor_pago_c,status,numero,total_parcelas,periodo_inicio,periodo_fim,tipo,observacao) VALUES(?,?,?,?,?,?,?,0,'aberta',0,0,?,?,?,?)`, lease.ID, p.Competencia, p.Vencimento, p.ValorBaseC, p.MultaC, p.JurosC, p.DescontoC, p.PeriodoInicio, p.PeriodoFim, p.Tipo, p.Observacao)
	if err != nil {
		return p, err
	}
	p.ID, err = res.LastInsertId()
	return p, err
}

func (s *Store) ReopenInstallment(ctx context.Context, id int64) (domain.Parcela, error) {
	p, err := s.Installment(ctx, id)
	if err != nil {
		return p, err
	}
	if p.Status != "paga" && p.Status != "parcial" {
		return p, fmt.Errorf("parcela não está paga nem parcial (status: %s)", p.Status)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacoes WHERE parcela_id=?`, id); err != nil {
		return p, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE parcelas SET valor_pago_c=0, data_pagamento=NULL, forma=NULL, recibo_path=NULL, status='aberta', multa_c=0, juros_c=0, desconto_c=0 WHERE id=?`, id); err != nil {
		return p, err
	}
	if err := tx.Commit(); err != nil {
		return p, err
	}
	return s.Installment(ctx, id)
}

// InstallmentPayment keeps every editable amount explicit; nil charges use the contract rules.
type InstallmentPayment struct {
	ParcelaID  int64  `json:"parcela_id"`
	Data       string `json:"data"`
	Forma      string `json:"forma"`
	DescontoC  *int64 `json:"desconto_c"`
	ValorPagoC *int64 `json:"valor_pago_c"`
	MultaC     *int64 `json:"multa_c"`
	JurosC     *int64 `json:"juros_c"`
}

type paidInstallment struct {
	domain.Parcela
	propertyName string
	tenantName   string
}

// PayInstallments writes the entire batch or none of it, including ledger entries.
func (s *Store) PayInstallments(ctx context.Context, payments []InstallmentPayment) ([]domain.Parcela, error) {
	if len(payments) == 0 {
		return nil, errors.New("o lote deve conter ao menos uma parcela")
	}
	seen := make(map[int64]bool, len(payments))
	for _, payment := range payments {
		if payment.ParcelaID < 1 {
			return nil, errors.New("identificador de parcela inválido")
		}
		if seen[payment.ParcelaID] {
			return nil, fmt.Errorf("parcela #%d duplicada no lote", payment.ParcelaID)
		}
		seen[payment.ParcelaID] = true
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	paid := make([]paidInstallment, 0, len(payments))
	for _, payment := range payments {
		item, err := payInstallment(ctx, tx, payment)
		if err != nil {
			return nil, fmt.Errorf("parcela #%d: %w", payment.ParcelaID, err)
		}
		paid = append(paid, item)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	items := make([]domain.Parcela, 0, len(paid))
	for _, item := range paid {
		s.writePaymentReceipt(ctx, &item)
		items = append(items, item.Parcela)
	}
	return items, nil
}

func payInstallment(ctx context.Context, tx *sql.Tx, payment InstallmentPayment) (paidInstallment, error) {
	var p paidInstallment
	var err error
	p.Parcela, err = installment(ctx, tx, payment.ParcelaID)
	if err != nil {
		return p, err
	}
	if p.Status == "paga" || p.Status == "cancelada" {
		return p, fmt.Errorf("parcela bloqueada: status %s", p.Status)
	}

	lease, err := lease(ctx, tx, p.LocacaoID)
	if err != nil {
		return p, err
	}
	due, err := domain.ParseDate(p.Vencimento)
	if err != nil {
		return p, err
	}
	paidAt, err := domain.ParseDate(payment.Data)
	if err != nil {
		return p, fmt.Errorf("data de pagamento inválida: %w", err)
	}
	fee, interest, _ := domain.CalcEncargos(due, paidAt, p.ValorBaseC, lease.MultaPct, lease.JurosDiaPct)
	if fee < p.MultaC {
		fee = p.MultaC
	}
	if interest < p.JurosC {
		interest = p.JurosC
	}
	if payment.MultaC != nil {
		fee = *payment.MultaC
	}
	if payment.JurosC != nil {
		interest = *payment.JurosC
	}
	if fee < 0 || interest < 0 {
		return p, errors.New("multa e juros não podem ser negativos")
	}
	discount := p.DescontoC
	if payment.DescontoC != nil {
		discount = *payment.DescontoC
	}
	gross, err := addNonnegativeAmounts(p.ValorBaseC, fee, interest)
	if err != nil {
		return p, err
	}
	if discount > gross {
		return p, errors.New("o desconto não pode zerar ou ultrapassar o total devido")
	}
	if discount < 0 && gross > maxInt64+discount {
		return p, errors.New("ajuste de multa excede o limite permitido")
	}
	total := gross - discount
	if total <= p.ValorPagoC {
		return p, errors.New("o ajuste não pode deixar saldo zero ou negativo para receber")
	}
	amount := total - p.ValorPagoC
	if payment.ValorPagoC != nil {
		amount = *payment.ValorPagoC
	}
	if amount <= 0 || amount > maxInt64-p.ValorPagoC {
		return p, errors.New("valor de pagamento inválido")
	}
	p.MultaC, p.JurosC, p.DescontoC = fee, interest, discount
	p.ValorPagoC += amount
	p.DataPagamento = &payment.Data
	if payment.Forma == "" {
		payment.Forma = "pix"
	}
	p.Forma = &payment.Forma
	if p.ValorPagoC >= total {
		p.Status = "paga"
	} else {
		p.Status = "parcial"
	}

	if _, err = tx.ExecContext(ctx, `UPDATE parcelas SET multa_c=?,juros_c=?,desconto_c=?,valor_pago_c=?,data_pagamento=?,forma=?,status=? WHERE id=?`, p.MultaC, p.JurosC, p.DescontoC, p.ValorPagoC, p.DataPagamento, p.Forma, p.Status, p.ID); err != nil {
		return p, err
	}

	if err := tx.QueryRowContext(ctx, `SELECT i.apelido,t.nome FROM locacoes l JOIN imoveis i ON i.id=l.imovel_id JOIN inquilinos t ON t.id=l.inquilino_id WHERE l.id=?`, lease.ID).Scan(&p.propertyName, &p.tenantName); err != nil {
		return p, err
	}

	desc := fmt.Sprintf("Recebimento aluguel %s - Parcela %02d/%02d (%s)", p.Competencia, p.Numero, p.TotalParcelas, p.propertyName)
	if p.Tipo != "aluguel" {
		desc = fmt.Sprintf("Recebimento %s %s (%s)", p.Tipo, p.Competencia, p.propertyName)
	}
	ref := fmt.Sprintf("Parcela #%d", p.ID)
	if _, err := tx.ExecContext(ctx, `INSERT INTO movimentacoes(tipo,categoria,data,valor_c,descricao,imovel_id,inquilino_id,locacao_id,parcela_id,referencia,forma_pagamento,origem,natureza) VALUES('receita','aluguel',?,?,?,?,?,?,?,?,?,'locacao','operacional')`, payment.Data, amount, desc, lease.ImovelID, lease.InquilinoID, lease.ID, p.ID, ref, payment.Forma); err != nil {
		return p, err
	}

	return p, nil
}

func (s *Store) writePaymentReceipt(ctx context.Context, p *paidInstallment) {
	if p.Status == "paga" {
		dir := s.ReceiptsDir
		if dir == "" {
			dir = "recibos"
		}
		path := filepath.Join(dir, fmt.Sprintf("recibo_%d.pdf", p.ID))
		period := ""
		if p.PeriodoInicio != nil && p.PeriodoFim != nil {
			period = *p.PeriodoInicio + " a " + *p.PeriodoFim
		}
		lines := []string{
			"Imovel: " + p.propertyName,
			"Inquilino: " + p.tenantName,
			"Competencia: " + p.Competencia + "  Vencimento: " + p.Vencimento,
			fmt.Sprintf("Parcela: %02d/%02d  Periodo: %s", p.Numero, p.TotalParcelas, period),
			"Valor base: " + moneyBRL(p.ValorBaseC),
			"Multa: " + moneyBRL(p.MultaC) + "  Juros: " + moneyBRL(p.JurosC) + "  Ajuste (desconto + / multa -): " + moneyBRL(p.DescontoC),
			"Valor pago: " + moneyBRL(p.ValorPagoC) + " em " + *p.DataPagamento + " via " + *p.Forma,
			fmt.Sprintf("Parcela #%d - Locacao #%d", p.ID, p.LocacaoID),
		}
		if receipt.Write(path, lines) == nil {
			if _, err := s.DB.ExecContext(ctx, `UPDATE parcelas SET recibo_path=? WHERE id=?`, path, p.ID); err == nil {
				p.ReciboPath = &path
			}
		}
	}
}

const maxInt64 = int64(^uint64(0) >> 1)

func addNonnegativeAmounts(values ...int64) (int64, error) {
	var total int64
	for _, value := range values {
		if value < 0 || total > maxInt64-value {
			return 0, errors.New("total da parcela inválido")
		}
		total += value
	}
	return total, nil
}

func moneyBRL(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, value/100, value%100)
}

type Simulation struct {
	BaseC      int64 `json:"base_c"`
	MultaC     int64 `json:"multa_c"`
	JurosC     int64 `json:"juros_c"`
	DescontoC  int64 `json:"desconto_c"`
	TotalC     int64 `json:"total_c"`
	DiasAtraso int   `json:"dias_atraso"`
}

func (s *Store) SimulateInstallment(ctx context.Context, id int64, today string) (Simulation, error) {
	p, err := s.Installment(ctx, id)
	if err != nil {
		return Simulation{}, err
	}
	lease, err := s.Lease(ctx, p.LocacaoID)
	if err != nil {
		return Simulation{}, err
	}
	due, err := domain.ParseDate(p.Vencimento)
	if err != nil {
		return Simulation{}, err
	}
	at, err := domain.ParseDate(today)
	if err != nil {
		return Simulation{}, err
	}
	fee, interest, days := domain.CalcEncargos(due, at, p.ValorBaseC, lease.MultaPct, lease.JurosDiaPct)
	if fee < p.MultaC {
		fee = p.MultaC
	}
	if interest < p.JurosC {
		interest = p.JurosC
	}
	return Simulation{BaseC: p.ValorBaseC, MultaC: fee, JurosC: interest, DescontoC: p.DescontoC, TotalC: p.ValorBaseC + fee + interest - p.DescontoC, DiasAtraso: days}, nil
}

func Situation(p domain.Parcela, today time.Time) string {
	if p.Status == "paga" || p.Status == "cancelada" {
		return p.Status
	}
	due, err := domain.ParseDate(p.Vencimento)
	if err == nil && due.Before(today) {
		if p.Status == "parcial" {
			return "parcial-vencida"
		}
		return "vencida"
	}
	if p.Status == "parcial" {
		return "parcial"
	}
	return "aberta"
}

func ptr[T any](value T) *T { return &value }

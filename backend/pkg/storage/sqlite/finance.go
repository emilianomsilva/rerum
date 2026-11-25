package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"rerum/backend/pkg/domain"
)

func (s *Store) Expenses(ctx context.Context, propertyID *int64) ([]domain.Despesa, error) {
	query := `SELECT d.id,d.imovel_id,d.data,d.categoria,d.valor_c,d.descricao,d.fornecedor,d.forma_pagamento,d.empreendimento_id,i.apelido FROM despesas d JOIN imoveis i ON i.id=d.imovel_id WHERE 1=1`
	args := []any{}
	if propertyID != nil {
		query += ` AND d.imovel_id=?`
		args = append(args, *propertyID)
	}
	query += ` ORDER BY d.data DESC,d.id DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Despesa{}
	for rows.Next() {
		var item domain.Despesa
		var project sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ImovelID, &item.Data, &item.Categoria, &item.ValorC, &item.Descricao, &item.Fornecedor, &item.FormaPagamento, &project, &item.Imovel); err != nil {
			return nil, err
		}
		if project.Valid {
			item.EmpreendimentoID = &project.Int64
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SaveExpense(ctx context.Context, item domain.Despesa) (domain.Despesa, error) {
	if item.ImovelID < 1 || item.ValorC <= 0 {
		return item, errors.New("imóvel e valor positivo são obrigatórios")
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO despesas(imovel_id,data,categoria,valor_c,descricao,fornecedor,forma_pagamento,empreendimento_id) VALUES(?,?,?,?,?,?,?,?)`, item.ImovelID, item.Data, item.Categoria, item.ValorC, item.Descricao, item.Fornecedor, item.FormaPagamento, item.EmpreendimentoID)
	if err != nil {
		return item, err
	}
	item.ID, err = result.LastInsertId()
	return item, err
}

func (s *Store) DeleteExpense(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM despesas WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Projects(ctx context.Context, includeInactive bool) ([]domain.Empreendimento, error) {
	query := `SELECT id,nome,endereco,observacao,ativo FROM empreendimentos`
	if !includeInactive {
		query += ` WHERE ativo=1`
	}
	rows, err := s.DB.QueryContext(ctx, query+` ORDER BY nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Empreendimento{}
	for rows.Next() {
		var x domain.Empreendimento
		var active int
		if err := rows.Scan(&x.ID, &x.Nome, &x.Endereco, &x.Observacao, &active); err != nil {
			return nil, err
		}
		x.Ativo = active != 0
		x.UnidadeIDs = []int64{}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].UnidadeIDs, err = s.projectUnitIDs(ctx, items[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s *Store) Project(ctx context.Context, id int64) (domain.Empreendimento, error) {
	var x domain.Empreendimento
	var active int
	err := s.DB.QueryRowContext(ctx, `SELECT id,nome,endereco,observacao,ativo FROM empreendimentos WHERE id=?`, id).Scan(&x.ID, &x.Nome, &x.Endereco, &x.Observacao, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	if err != nil {
		return x, err
	}
	x.Ativo = active != 0
	x.UnidadeIDs, err = s.projectUnitIDs(ctx, x.ID)
	return x, err
}

func (s *Store) projectUnitIDs(ctx context.Context, id int64) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT imovel_id FROM empreendimento_unidades WHERE empreendimento_id=? ORDER BY imovel_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SaveProject(ctx context.Context, x domain.Empreendimento) (domain.Empreendimento, error) {
	if strings.TrimSpace(x.Nome) == "" {
		return x, errors.New("nome do empreendimento é obrigatório")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO empreendimentos(nome,endereco,observacao,ativo) VALUES(?,?,?,1)`, strings.TrimSpace(x.Nome), x.Endereco, x.Observacao)
	if err != nil {
		return x, err
	}
	x.ID, err = result.LastInsertId()
	if err != nil {
		return x, err
	}
	x.Ativo = true
	for _, uID := range x.UnidadeIDs {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO empreendimento_unidades(empreendimento_id,imovel_id,peso) VALUES(?,?,1)`, x.ID, uID); err != nil {
			return x, err
		}
	}
	return x, tx.Commit()
}

func (s *Store) UpdateProject(ctx context.Context, x domain.Empreendimento) error {
	if strings.TrimSpace(x.Nome) == "" {
		return errors.New("nome do empreendimento é obrigatório")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE empreendimentos SET nome=?, endereco=?, observacao=? WHERE id=?`, strings.TrimSpace(x.Nome), x.Endereco, x.Observacao, x.ID)
	if err != nil {
		return err
	}
	if count, err := res.RowsAffected(); err != nil || count == 0 {
		return ErrNotFound
	}
	if x.UnidadeIDs != nil {
		// Preserve weights of retained units. Editing defaults never changes historical allocations.
		query := `DELETE FROM empreendimento_unidades WHERE empreendimento_id=?`
		args := []any{x.ID}
		if len(x.UnidadeIDs) > 0 {
			query += ` AND imovel_id NOT IN (` + strings.TrimSuffix(strings.Repeat("?,", len(x.UnidadeIDs)), ",") + `)`
			for _, id := range x.UnidadeIDs {
				args = append(args, id)
			}
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
		for _, uID := range x.UnidadeIDs {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO empreendimento_unidades(empreendimento_id,imovel_id,peso) VALUES(?,?,1)`, x.ID, uID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) DeleteProject(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE empreendimentos SET ativo=0 WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

type MovementFilter struct {
	Kind             string
	PropertyID       *int64
	EmpreendimentoID *int64
	Natureza         string
	Start            string
	End              string
}

func (s *Store) FilterMovements(ctx context.Context, f MovementFilter) ([]domain.Movimento, error) {
	query := `SELECT DISTINCT m.id,m.tipo,m.categoria,m.data,m.valor_c,m.descricao,m.imovel_id,m.empreendimento_id,m.inquilino_id,m.locacao_id,m.parcela_id,m.referencia,m.forma_pagamento,m.origem,m.fornecedor,m.observacao,m.reversao,COALESCE(m.natureza,'operacional'),COALESCE(i.apelido,''),COALESCE(t.nome,'') FROM movimentacoes m LEFT JOIN movimentacao_rateios r ON r.movimentacao_id=m.id LEFT JOIN imoveis i ON i.id=m.imovel_id LEFT JOIN inquilinos t ON t.id=m.inquilino_id WHERE 1=1`
	args := []any{}
	if f.Kind != "" {
		query += ` AND m.tipo=?`
		args = append(args, f.Kind)
	}
	if f.PropertyID != nil {
		query += ` AND (m.imovel_id=? OR r.imovel_id=?)`
		args = append(args, *f.PropertyID, *f.PropertyID)
	}
	if f.EmpreendimentoID != nil {
		query += ` AND m.empreendimento_id=?`
		args = append(args, *f.EmpreendimentoID)
	}
	if f.Natureza != "" {
		query += ` AND m.natureza=?`
		args = append(args, f.Natureza)
	}
	if f.Start != "" {
		query += ` AND m.data>=?`
		args = append(args, f.Start)
	}
	if f.End != "" {
		query += ` AND m.data<=?`
		args = append(args, f.End)
	}
	query += ` ORDER BY m.data DESC,m.id DESC`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Movimento{}
	for rows.Next() {
		x, err := scanMovement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Store) Movements(ctx context.Context, kind string, propertyID *int64) ([]domain.Movimento, error) {
	return s.FilterMovements(ctx, MovementFilter{Kind: kind, PropertyID: propertyID})
}

func scanMovement(row scanner) (domain.Movimento, error) {
	var x domain.Movimento
	var property, project, tenant, lease, installment sql.NullInt64
	var reversal int
	err := row.Scan(&x.ID, &x.Tipo, &x.Categoria, &x.Data, &x.ValorC, &x.Descricao, &property, &project, &tenant, &lease, &installment, &x.Referencia, &x.FormaPagamento, &x.Origem, &x.Fornecedor, &x.Observacao, &reversal, &x.Natureza, &x.Imovel, &x.Inquilino)
	if property.Valid {
		x.ImovelID = &property.Int64
	}
	if project.Valid {
		x.EmpreendimentoID = &project.Int64
	}
	if tenant.Valid {
		x.InquilinoID = &tenant.Int64
	}
	if lease.Valid {
		x.LocacaoID = &lease.Int64
	}
	if installment.Valid {
		x.ParcelaID = &installment.Int64
	}
	x.Reversao = reversal != 0
	return x, err
}

func (s *Store) Movement(ctx context.Context, id int64) (domain.Movimento, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT m.id,m.tipo,m.categoria,m.data,m.valor_c,m.descricao,m.imovel_id,m.empreendimento_id,m.inquilino_id,m.locacao_id,m.parcela_id,m.referencia,m.forma_pagamento,m.origem,m.fornecedor,m.observacao,m.reversao,COALESCE(m.natureza,'operacional'),COALESCE(i.apelido,''),COALESCE(t.nome,'') FROM movimentacoes m LEFT JOIN imoveis i ON i.id=m.imovel_id LEFT JOIN inquilinos t ON t.id=m.inquilino_id WHERE m.id=?`, id)
	x, err := scanMovement(row)
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	return x, err
}

func (s *Store) Allocations(ctx context.Context, movementID int64) ([]domain.Rateio, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT imovel_id,valor_c FROM movimentacao_rateios WHERE movimentacao_id=? ORDER BY imovel_id`, movementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.Rateio{}
	for rows.Next() {
		var x domain.Rateio
		if err := rows.Scan(&x.ImovelID, &x.ValorC); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Store) CreateMovement(ctx context.Context, x domain.Movimento, allocations []domain.Rateio) (domain.Movimento, error) {
	if x.Tipo != "receita" && x.Tipo != "despesa" {
		return x, errors.New("tipo de movimentação inválido")
	}
	if x.ValorC <= 0 {
		return x, errors.New("valor deve ser positivo")
	}
	if x.ImovelID == nil && x.EmpreendimentoID == nil {
		return x, errors.New("informe imóvel ou empreendimento")
	}
	if x.Natureza == "" {
		if x.EmpreendimentoID != nil {
			x.Natureza = "investimento"
		} else {
			x.Natureza = "operacional"
		}
	}

	// Automatic allocation for empreendimento if none provided
	if len(allocations) == 0 && x.EmpreendimentoID != nil {
		uRows, uErr := s.DB.QueryContext(ctx, `SELECT imovel_id FROM empreendimento_unidades WHERE empreendimento_id=? ORDER BY imovel_id`, *x.EmpreendimentoID)
		if uErr == nil {
			unitIDs := []int64{}
			for uRows.Next() {
				var uID int64
				if uRows.Scan(&uID) == nil {
					unitIDs = append(unitIDs, uID)
				}
			}
			uRows.Close()
			if len(unitIDs) > 0 {
				base := x.ValorC / int64(len(unitIDs))
				rem := x.ValorC % int64(len(unitIDs))
				for i, uID := range unitIDs {
					v := base
					if i == 0 {
						v += rem
					}
					allocations = append(allocations, domain.Rateio{ImovelID: uID, ValorC: v})
				}
			}
		}
	}

	if len(allocations) > 0 {
		var total int64
		for _, r := range allocations {
			if r.ValorC < 0 {
				return x, errors.New("rateio não pode ser negativo")
			}
			total += r.ValorC
		}
		if total != x.ValorC {
			return x, fmt.Errorf("rateios somam %d centavos, valor da movimentação é %d", total, x.ValorC)
		}
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return x, err
	}
	defer tx.Rollback()
	rev := 0
	if x.Reversao {
		rev = 1
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO movimentacoes(tipo,categoria,data,valor_c,descricao,imovel_id,empreendimento_id,inquilino_id,locacao_id,parcela_id,referencia,forma_pagamento,origem,fornecedor,observacao,reversao,natureza) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, x.Tipo, x.Categoria, x.Data, x.ValorC, x.Descricao, x.ImovelID, x.EmpreendimentoID, x.InquilinoID, x.LocacaoID, x.ParcelaID, x.Referencia, x.FormaPagamento, x.Origem, x.Fornecedor, x.Observacao, rev, x.Natureza)
	if err != nil {
		return x, err
	}
	x.ID, err = result.LastInsertId()
	if err != nil {
		return x, err
	}
	for _, r := range allocations {
		if _, err := tx.ExecContext(ctx, `INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(?,?,?)`, x.ID, r.ImovelID, r.ValorC); err != nil {
			return x, err
		}
	}
	if x.Tipo == "despesa" {
		sign := int64(1)
		if x.Reversao {
			sign = -1
		}
		for _, r := range allocations {
			if _, err := tx.ExecContext(ctx, `UPDATE imoveis SET investimento_importado_c=investimento_importado_c+? WHERE id=?`, sign*r.ValorC, r.ImovelID); err != nil {
				return x, err
			}
		}
	}
	return x, tx.Commit()
}

func (s *Store) UpdateMovement(ctx context.Context, x domain.Movimento, allocations []domain.Rateio) error {
	current, err := s.Movement(ctx, x.ID)
	if err != nil {
		return err
	}
	if x.Tipo != "receita" && x.Tipo != "despesa" {
		return errors.New("tipo de movimentação inválido")
	}
	if x.ValorC <= 0 {
		return errors.New("valor deve ser positivo")
	}
	if x.Natureza == "" {
		x.Natureza = current.Natureza
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Revert old investment allocations if previously applied
	if current.Tipo == "despesa" {
		oldSign := int64(1)
		if current.Reversao {
			oldSign = -1
		}
		oldAllocations, aErr := s.Allocations(ctx, current.ID)
		if aErr == nil {
			for _, r := range oldAllocations {
				if _, err := tx.ExecContext(ctx, `UPDATE imoveis SET investimento_importado_c=investimento_importado_c-? WHERE id=?`, oldSign*r.ValorC, r.ImovelID); err != nil {
					return err
				}
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacao_rateios WHERE movimentacao_id=?`, x.ID); err != nil {
		return err
	}

	if len(allocations) == 0 && x.EmpreendimentoID != nil {
		uRows, uErr := tx.QueryContext(ctx, `SELECT imovel_id FROM empreendimento_unidades WHERE empreendimento_id=? ORDER BY imovel_id`, *x.EmpreendimentoID)
		if uErr == nil {
			unitIDs := []int64{}
			for uRows.Next() {
				var uID int64
				if uRows.Scan(&uID) == nil {
					unitIDs = append(unitIDs, uID)
				}
			}
			uRows.Close()
			if len(unitIDs) > 0 {
				base := x.ValorC / int64(len(unitIDs))
				rem := x.ValorC % int64(len(unitIDs))
				for i, uID := range unitIDs {
					v := base
					if i == 0 {
						v += rem
					}
					allocations = append(allocations, domain.Rateio{ImovelID: uID, ValorC: v})
				}
			}
		}
	}

	if len(allocations) > 0 {
		var total int64
		for _, r := range allocations {
			if r.ValorC < 0 {
				return errors.New("rateio não pode ser negativo")
			}
			total += r.ValorC
		}
		if total != x.ValorC {
			return fmt.Errorf("rateios somam %d centavos, valor da movimentação é %d", total, x.ValorC)
		}
		for _, r := range allocations {
			if _, err := tx.ExecContext(ctx, `INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(?,?,?)`, x.ID, r.ImovelID, r.ValorC); err != nil {
				return err
			}
		}
	}

	rev := 0
	if x.Reversao {
		rev = 1
	}
	_, err = tx.ExecContext(ctx, `UPDATE movimentacoes SET tipo=?,categoria=?,data=?,valor_c=?,descricao=?,imovel_id=?,empreendimento_id=?,inquilino_id=?,referencia=?,forma_pagamento=?,origem=?,fornecedor=?,observacao=?,reversao=?,natureza=? WHERE id=?`, x.Tipo, x.Categoria, x.Data, x.ValorC, x.Descricao, x.ImovelID, x.EmpreendimentoID, x.InquilinoID, x.Referencia, x.FormaPagamento, x.Origem, x.Fornecedor, x.Observacao, rev, x.Natureza, x.ID)
	if err != nil {
		return err
	}

	if x.Tipo == "despesa" {
		sign := int64(1)
		if x.Reversao {
			sign = -1
		}
		for _, r := range allocations {
			if _, err := tx.ExecContext(ctx, `UPDATE imoveis SET investimento_importado_c=investimento_importado_c+? WHERE id=?`, sign*r.ValorC, r.ImovelID); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *Store) DeleteMovement(ctx context.Context, id int64) error {
	current, err := s.Movement(ctx, id)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if current.Tipo == "despesa" {
		oldSign := int64(1)
		if current.Reversao {
			oldSign = -1
		}
		oldAllocations, aErr := s.Allocations(ctx, current.ID)
		if aErr == nil {
			for _, r := range oldAllocations {
				if _, err := tx.ExecContext(ctx, `UPDATE imoveis SET investimento_importado_c=investimento_importado_c-? WHERE id=?`, oldSign*r.ValorC, r.ImovelID); err != nil {
					return err
				}
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacao_rateios WHERE movimentacao_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacoes WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

type DebtSummary struct {
	Debt      domain.Divida            `json:"divida"`
	SaldoC    int64                    `json:"saldo_c"`
	Movements []domain.MovimentoDivida `json:"movimentos"`
}

func (s *Store) Debts(ctx context.Context) ([]DebtSummary, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,credor,descricao,valor_original_c,data_contratacao,vencimento,observacao,ativa FROM dividas ORDER BY data_contratacao DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DebtSummary{}
	for rows.Next() {
		var d domain.Divida
		var due sql.NullString
		var active int
		if err := rows.Scan(&d.ID, &d.Credor, &d.Descricao, &d.ValorOriginalC, &d.DataContratacao, &due, &d.Observacao, &active); err != nil {
			return nil, err
		}
		d.Ativa = active != 0
		if due.Valid {
			d.Vencimento = &due.String
		}
		movements, err := s.DebtMovements(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		balance := d.ValorOriginalC
		for _, m := range movements {
			if m.Tipo == "liberacao" {
				balance += m.ValorC
			} else {
				principal := m.ValorC - m.JurosC
				if principal > 0 {
					balance -= principal
				}
			}
		}
		items = append(items, DebtSummary{Debt: d, SaldoC: balance, Movements: movements})
	}
	return items, rows.Err()
}

func (s *Store) DebtMovements(ctx context.Context, id int64) ([]domain.MovimentoDivida, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id,divida_id,tipo,data,valor_c,COALESCE(juros_c,0),forma_pagamento,descricao FROM movimentos_divida WHERE divida_id=? ORDER BY data,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.MovimentoDivida{}
	for rows.Next() {
		var x domain.MovimentoDivida
		if err := rows.Scan(&x.ID, &x.DividaID, &x.Tipo, &x.Data, &x.ValorC, &x.JurosC, &x.FormaPagamento, &x.Descricao); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (s *Store) CreateDebt(ctx context.Context, d domain.Divida) (domain.Divida, error) {
	if strings.TrimSpace(d.Credor) == "" || d.ValorOriginalC <= 0 {
		return d, errors.New("credor e valor positivo são obrigatórios")
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO dividas(credor,descricao,valor_original_c,data_contratacao,vencimento,observacao,ativa) VALUES(?,?,?,?,?,?,1)`, d.Credor, d.Descricao, d.ValorOriginalC, d.DataContratacao, d.Vencimento, d.Observacao)
	if err != nil {
		return d, err
	}
	d.ID, err = result.LastInsertId()
	d.Ativa = true
	return d, err
}

func (s *Store) UpdateDebt(ctx context.Context, d domain.Divida) error {
	if strings.TrimSpace(d.Credor) == "" || d.ValorOriginalC <= 0 {
		return errors.New("credor e valor positivo são obrigatórios")
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE dividas SET credor=?, descricao=?, valor_original_c=?, data_contratacao=?, vencimento=?, observacao=? WHERE id=?`, strings.TrimSpace(d.Credor), d.Descricao, d.ValorOriginalC, d.DataContratacao, d.Vencimento, d.Observacao, d.ID)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteDebt(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentos_divida WHERE divida_id=?`, id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM dividas WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) AddDebtMovement(ctx context.Context, m domain.MovimentoDivida) error {
	if m.Tipo != "liberacao" && m.Tipo != "pagamento" {
		return errors.New("tipo de movimento de dívida inválido")
	}
	if m.ValorC <= 0 {
		return errors.New("valor deve ser positivo")
	}
	if m.JurosC < 0 {
		return errors.New("juros não podem ser negativos")
	}
	if m.Tipo == "pagamento" && m.JurosC > m.ValorC {
		return errors.New("o valor dos juros não pode exceder o valor total do pagamento")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var balance int64
	if err := tx.QueryRowContext(ctx, `SELECT valor_original_c FROM dividas WHERE id=?`, m.DividaID).Scan(&balance); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT tipo,valor_c,COALESCE(juros_c,0) FROM movimentos_divida WHERE divida_id=?`, m.DividaID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var kind string
		var amount, interest int64
		if err := rows.Scan(&kind, &amount, &interest); err != nil {
			rows.Close()
			return err
		}
		if kind == "liberacao" {
			balance += amount
		} else {
			principal := amount - interest
			if principal > 0 {
				balance -= principal
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if m.Tipo == "pagamento" {
		principal := m.ValorC - m.JurosC
		if principal > balance {
			return errors.New("a amortização do principal não pode exceder o saldo da dívida")
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO movimentos_divida(divida_id,tipo,data,valor_c,juros_c,forma_pagamento,descricao) VALUES(?,?,?,?,?,?,?)`, m.DividaID, m.Tipo, m.Data, m.ValorC, m.JurosC, m.FormaPagamento, m.Descricao); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteDebtMovement(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM movimentos_divida WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, _ := res.RowsAffected()
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

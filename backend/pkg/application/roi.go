package application

import (
	"context"
	"database/sql"
	"time"

	"rerum/backend/pkg/domain"
)

type ROIService struct{ DB *sql.DB }

// Property computes ROI for a single property using strict cash-basis rules:
//   - Capital base = acquisition fields (valor_compra_c + reforma_c + itbi_c + corretagem_c + outros_custos_c)
//   - max(investimento_importado_c, capex movements with natureza='investimento' directly or via rateios).
//   - Receipts = ledger receipts (movimentacoes tipo='receita', respecting reversao and rateios)
//   - any residual paid installment amounts without ledger breakdown (parcelas status<>'cancelada').
//   - Operational expenses = legacy despesas + ledger expenses (tipo='despesa' and natureza!='investimento',
//     respecting reversao and rateios).
//   - Period window defaults to [today - 365 days, today] when start/end are zero, or uses the explicit
//     inclusive [start, end] range provided by the caller.
func (s ROIService) Property(ctx context.Context, id int64, today time.Time) (domain.Roi, error) {
	return s.PropertyRange(ctx, id, today.AddDate(0, 0, -365), today)
}

func (s ROIService) PropertyRange(ctx context.Context, id int64, start, end time.Time) (domain.Roi, error) {
	var property domain.Imovel
	err := s.DB.QueryRowContext(
		ctx,
		`SELECT id,apelido,valor_compra_c,reforma_c,itbi_c,corretagem_c,outros_custos_c,investimento_importado_c FROM imoveis WHERE id=?`,
		id,
	).Scan(
		&property.ID,
		&property.Apelido,
		&property.ValorCompraC,
		&property.ReformaC,
		&property.ITBIC,
		&property.CorretagemC,
		&property.OutrosCustosC,
		&property.InvestimentoImportadoC,
	)
	if err != nil {
		return domain.Roi{}, err
	}

	hasWindow := !start.IsZero() && !end.IsZero()
	if hasWindow && start.After(end) {
		start, end = end, start
	}
	inWindow := func(raw string) bool {
		if !hasWindow {
			return true
		}
		if len(raw) >= 10 {
			raw = raw[:10]
		}
		d, parseErr := domain.ParseDate(raw)
		if parseErr != nil {
			return false
		}
		return !d.Before(start) && !d.After(end)
	}

	baseCost := property.ValorCompraC + property.ReformaC + property.ITBIC + property.CorretagemC + property.OutrosCustosC
	var capexC, receivedTotal, receivedWindow, expensesTotal, expensesWindow int64

	// 1. Legacy expenses table (always operational)
	rows, err := s.DB.QueryContext(ctx, `SELECT valor_c,data FROM despesas WHERE imovel_id=?`, id)
	if err != nil {
		return domain.Roi{}, err
	}
	for rows.Next() {
		var amount int64
		var rawDate string
		if err := rows.Scan(&amount, &rawDate); err != nil {
			rows.Close()
			return domain.Roi{}, err
		}
		expensesTotal += amount
		if inWindow(rawDate) {
			expensesWindow += amount
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Roi{}, err
	}
	rows.Close()

	// 2. Ledger movements (direct or allocated via movimentacao_rateios)
	rows, err = s.DB.QueryContext(
		ctx,
		`SELECT m.id,m.tipo,COALESCE(NULLIF(m.natureza,''),'operacional'),m.data,m.valor_c,m.imovel_id,m.reversao,
			COALESCE((SELECT COUNT(*) FROM movimentacao_rateios r2 WHERE r2.movimentacao_id=m.id),0),
			COALESCE((SELECT SUM(r3.valor_c) FROM movimentacao_rateios r3 WHERE r3.movimentacao_id=m.id AND r3.imovel_id=?),0)
		FROM movimentacoes m
		WHERE m.imovel_id=? OR EXISTS (SELECT 1 FROM movimentacao_rateios r WHERE r.movimentacao_id=m.id AND r.imovel_id=?)
		ORDER BY m.id`,
		id, id, id,
	)
	if err != nil {
		return domain.Roi{}, err
	}
	for rows.Next() {
		var movementID, amount, allocatedToProp int64
		var kind, nature, rawDate string
		var direct sql.NullInt64
		var reversal, allocCount int
		if err := rows.Scan(&movementID, &kind, &nature, &rawDate, &amount, &direct, &reversal, &allocCount, &allocatedToProp); err != nil {
			rows.Close()
			return domain.Roi{}, err
		}
		sign := int64(1)
		if reversal != 0 {
			sign = -1
		}
		var portion int64
		if allocCount > 0 {
			portion = sign * allocatedToProp
		} else if direct.Valid && direct.Int64 == id {
			portion = sign * amount
		}
		if portion == 0 {
			continue
		}

		if kind == "receita" {
			receivedTotal += portion
			if inWindow(rawDate) {
				receivedWindow += portion
			}
			continue
		}

		// Expense: classify strictly by natureza ('investimento' => capital base, otherwise operational)
		if nature == "investimento" {
			capexC += portion
		} else {
			expensesTotal += portion
			if inWindow(rawDate) {
				expensesWindow += portion
			}
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Roi{}, err
	}
	rows.Close()

	// 3. Residual installment payments without corresponding ledger entries (legacy compatibility, zero double-count)
	rows, err = s.DB.QueryContext(
		ctx,
		`SELECT p.valor_pago_c - COALESCE((SELECT SUM(CASE WHEN m.reversao=1 THEN -m.valor_c ELSE m.valor_c END)
			FROM movimentacoes m WHERE m.parcela_id=p.id AND m.tipo='receita'),0),
			p.data_pagamento
		FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id
		WHERE l.imovel_id=? AND p.valor_pago_c>0 AND p.status<>'cancelada'`,
		id,
	)
	if err != nil {
		return domain.Roi{}, err
	}
	for rows.Next() {
		var residual int64
		var paidDate sql.NullString
		if err := rows.Scan(&residual, &paidDate); err != nil {
			rows.Close()
			return domain.Roi{}, err
		}
		if residual <= 0 {
			continue
		}
		receivedTotal += residual
		if !hasWindow {
			receivedWindow += residual
		} else if paidDate.Valid && inWindow(paidDate.String) {
			receivedWindow += residual
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return domain.Roi{}, err
	}
	rows.Close()

	investment := baseCost + max64(property.InvestimentoImportadoC, capexC)
	return domain.Roi{
		ImovelID:        id,
		Apelido:         property.Apelido,
		InvestimentoC:   investment,
		RecebidoTotalC:  receivedTotal,
		DespesasTotalC:  expensesTotal,
		Recebido12mC:    receivedWindow,
		Despesas12mC:    expensesWindow,
		ROIAnualPct:     domain.ROI(investment, receivedWindow-expensesWindow),
		ROIAcumuladoPct: domain.ROI(investment, receivedTotal-expensesTotal),
	}, nil
}

func (s ROIService) Dashboard(ctx context.Context, today time.Time) (domain.Dashboard, error) {
	return s.DashboardRange(ctx, today, today.AddDate(0, 0, -365), today)
}

func (s ROIService) DashboardRange(ctx context.Context, today, roiStart, roiEnd time.Time) (domain.Dashboard, error) {
	result := domain.Dashboard{ROIs: []domain.Roi{}, Vencidas: []domain.DashboardPayment{}, Proximas: []domain.DashboardPayment{}}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,status FROM imoveis WHERE ativo=1 ORDER BY apelido`)
	if err != nil {
		return result, err
	}
	type activeProperty struct {
		id     int64
		status string
	}
	properties := []activeProperty{}
	for rows.Next() {
		var property activeProperty
		if err := rows.Scan(&property.id, &property.status); err != nil {
			rows.Close()
			return result, err
		}
		properties = append(properties, property)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	result.TotalImoveis = len(properties)
	month := today.Format("2006-01")
	monthEnd := today.AddDate(0, 0, 30).Format("2006-01-02")
	var capital, receivedPeriod, expensesPeriod int64

	for _, property := range properties {
		var occupied int
		if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM locacoes WHERE imovel_id=? AND status='ativa'`, property.id).Scan(&occupied); err != nil {
			return result, err
		}

		if occupied > 0 {
			result.Ocupados++
		} else if property.status == "manutencao" {
			result.Manutencao++
		} else if property.status == "disponivel" {
			result.Disponiveis++
		}

		item, err := s.PropertyRange(ctx, property.id, roiStart, roiEnd)
		if err != nil {
			return result, err
		}
		result.ROIs = append(result.ROIs, item)
		capital += item.InvestimentoC
		receivedPeriod += item.Recebido12mC
		expensesPeriod += item.Despesas12mC

		rows, err := s.DB.QueryContext(ctx, `SELECT p.competencia,p.vencimento,p.valor_base_c,p.multa_c,p.juros_c,p.desconto_c,p.valor_pago_c,p.data_pagamento,p.status FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id WHERE l.imovel_id=?`, property.id)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var competency, due, status string
			var base, fee, interest, discount, paid int64
			var paidDate sql.NullString
			if err := rows.Scan(&competency, &due, &base, &fee, &interest, &discount, &paid, &paidDate, &status); err != nil {
				rows.Close()
				return result, err
			}
			if status == "cancelada" {
				continue
			}
			balance := max(0, base+fee+interest-discount-paid)
			dueMonth := ""
			if len(due) >= 7 {
				dueMonth = due[:7]
			}
			if (dueMonth == month || competency == month) && (status == "aberta" || status == "parcial") && balance > 0 {
				result.AReceberMesC += balance
			}
			dueDate, parseErr := domain.ParseDate(due)
			if parseErr == nil && dueDate.Before(today) && balance > 0 && (status == "aberta" || status == "parcial") {
				result.VencidasQtd++
				result.VencidasC += balance
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, err
		}
	}
	result.SerieMensal, err = s.MonthlyCashflow(ctx, today)
	if err != nil {
		return result, err
	}
	current := result.SerieMensal[len(result.SerieMensal)-1]
	result.ReceitaMesC = current.ReceitasC
	result.DespesasOperacionaisMesC = current.DespesasOperacionaisC
	result.CapitalInvestidoC = capital
	result.ResultadoOperacionalMesC = result.ReceitaMesC - result.DespesasOperacionaisMesC
	if capital > 0 {
		roi := domain.ROI(capital, receivedPeriod-expensesPeriod)
		result.ROICarteira12mPct = &roi
	}
	if result.TotalImoveis > 0 {
		result.TaxaOcupacaoPct = float64(result.Ocupados) * 100 / float64(result.TotalImoveis)
		result.TaxaOcupacaoPct = float64(int(result.TaxaOcupacaoPct*10+0.5)) / 10
	}
	result.Vencidas, err = s.dashboardPayments(ctx, today.Format("2006-01-02"), "", true)
	if err != nil {
		return result, err
	}
	result.Proximas, err = s.dashboardPayments(ctx, today.Format("2006-01-02"), monthEnd, false)
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s ROIService) dashboardPayments(ctx context.Context, from, through string, overdue bool) ([]domain.DashboardPayment, error) {
	condition := `date(p.vencimento)<date(?)`
	args := []any{from}
	if !overdue {
		condition = `date(p.vencimento)>=date(?) AND date(p.vencimento)<=date(?)`
		args = append(args, through)
	}
	query := `SELECT p.id,p.numero,p.total_parcelas,p.vencimento,i.apelido,t.nome,MAX(0,p.valor_base_c+p.multa_c+p.juros_c-p.desconto_c-p.valor_pago_c) FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id JOIN imoveis i ON i.id=l.imovel_id JOIN inquilinos t ON t.id=l.inquilino_id WHERE (p.status='aberta' OR p.status='parcial') AND ` + condition + ` ORDER BY p.vencimento,p.id LIMIT 5`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	payments := []domain.DashboardPayment{}
	for rows.Next() {
		var payment domain.DashboardPayment
		if err := rows.Scan(&payment.ID, &payment.Numero, &payment.TotalParcelas, &payment.Vencimento, &payment.Imovel, &payment.Inquilino, &payment.SaldoC); err != nil {
			return nil, err
		}
		if payment.SaldoC > 0 {
			payments = append(payments, payment)
		}
	}
	return payments, rows.Err()
}

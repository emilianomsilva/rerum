package application

import (
	"context"
	"time"

	"rerum/backend/pkg/domain"
)

// MonthlyCashflow reads individual ledger receipts, without moving historic data.
// Old cumulative receipts without a complete ledger are shown at their last known date,
// explicitly flagged as estimates rather than inventing a breakdown.
func (s ROIService) MonthlyCashflow(ctx context.Context, today time.Time) ([]domain.MonthlyCashflow, error) {
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -11, 0)
	end := first.AddDate(0, 12, 0).Format("2006-01-02")
	start := first.Format("2006-01-02")
	items := make([]domain.MonthlyCashflow, 12)
	byMonth := map[string]*domain.MonthlyCashflow{}
	for i := range items {
		items[i].Mes = first.AddDate(0, i, 0).Format("2006-01")
		byMonth[items[i].Mes] = &items[i]
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT substr(data,1,7),
		COALESCE(SUM(CASE WHEN tipo='receita' THEN CASE WHEN reversao=1 THEN -valor_c ELSE valor_c END ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN tipo='despesa' AND COALESCE(NULLIF(natureza,''),'operacional')<>'investimento' THEN CASE WHEN reversao=1 THEN -valor_c ELSE valor_c END ELSE 0 END),0)
		FROM movimentacoes WHERE data>=? AND data<? GROUP BY substr(data,1,7)`, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var month string
		var income, expenses int64
		if err := rows.Scan(&month, &income, &expenses); err != nil {
			rows.Close()
			return nil, err
		}
		if item := byMonth[month]; item != nil {
			item.ReceitasC += income
			item.DespesasOperacionaisC += expenses
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT substr(p.data_pagamento,1,7), SUM(MAX(0,p.valor_pago_c-COALESCE((SELECT SUM(CASE WHEN m.reversao=1 THEN -m.valor_c ELSE m.valor_c END) FROM movimentacoes m WHERE m.parcela_id=p.id AND m.tipo='receita'),0)))
		FROM parcelas p WHERE p.valor_pago_c>0 AND p.data_pagamento>=? AND p.data_pagamento<? AND p.status<>'cancelada' GROUP BY substr(p.data_pagamento,1,7)`, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var month string
		var legacy int64
		if err := rows.Scan(&month, &legacy); err != nil {
			rows.Close()
			return nil, err
		}
		if item := byMonth[month]; item != nil {
			item.ReceitasC += legacy
			item.ReceitasSemDetalhamentoC = legacy
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT substr(data,1,7), SUM(valor_c) FROM despesas WHERE data>=? AND data<? GROUP BY substr(data,1,7)`, start, end)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var month string
		var expenses int64
		if err := rows.Scan(&month, &expenses); err != nil {
			rows.Close()
			return nil, err
		}
		if item := byMonth[month]; item != nil {
			item.DespesasOperacionaisC += expenses
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ResultadoOperacionalC = items[i].ReceitasC - items[i].DespesasOperacionaisC
	}
	return items, nil
}

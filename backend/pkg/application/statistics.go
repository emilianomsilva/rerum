package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"rerum/backend/pkg/domain"
)

type StatisticsService struct{ DB *sql.DB }

type propertyStats struct {
	id               int64
	name             string
	base, imported   int64
	destinadoLocacao bool
}

// Portfolio computes consolidated portfolio metrics using the exact same ROI formula
// across the application:
//   - When start/end are zero, evaluates all-time cashflows.
//   - When start/end are provided, evaluates receipts and operational expenses strictly
//     within the inclusive [start, end] window against each property's capital base.
func (s StatisticsService) Portfolio(ctx context.Context, unitsOnly bool) (domain.Statistics, error) {
	return s.PortfolioRange(ctx, unitsOnly, time.Time{}, time.Time{})
}

func (s StatisticsService) PortfolioRange(ctx context.Context, unitsOnly bool, start, end time.Time) (domain.Statistics, error) {
	result := domain.Statistics{Linhas: []domain.StatisticsLine{}}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,apelido,valor_compra_c+reforma_c+itbi_c+corretagem_c+outros_custos_c,investimento_importado_c,destinado_locacao FROM imoveis WHERE ativo=1 ORDER BY apelido`)
	if err != nil {
		return result, err
	}
	properties := []propertyStats{}
	for rows.Next() {
		var property propertyStats
		var destLoc int
		if err := rows.Scan(&property.id, &property.name, &property.base, &property.imported, &destLoc); err != nil {
			rows.Close()
			return result, err
		}
		property.destinadoLocacao = destLoc != 0
		if !unitsOnly || property.destinadoLocacao {
			properties = append(properties, property)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
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

	investmentCostsTotal := map[int64]int64{}
	investmentCostsPeriod := map[int64]int64{}
	opexCosts := map[int64]int64{}
	income := map[int64]int64{}
	for _, property := range properties {
		investmentCostsTotal[property.id] = 0
		investmentCostsPeriod[property.id] = 0
		opexCosts[property.id] = 0
		income[property.id] = 0
	}

	// 1. Legacy expenses table (operational)
	rows, err = s.DB.QueryContext(ctx, `SELECT imovel_id,valor_c,data FROM despesas`)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var propID, amount int64
		var rawDate string
		if err := rows.Scan(&propID, &amount, &rawDate); err != nil {
			rows.Close()
			return result, err
		}
		if inWindow(rawDate) {
			opexCosts[propID] += amount
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}

	// 2. Ledger movements (respecting reversao, rateios, and date window)
	type movementRow struct {
		id       int64
		kind     string
		amount   int64
		natureza string
		rawDate  string
		property sql.NullInt64
		reversal int
	}
	rows, err = s.DB.QueryContext(ctx, `SELECT id,tipo,valor_c,COALESCE(NULLIF(natureza,''),'operacional'),data,imovel_id,reversao FROM movimentacoes ORDER BY id`)
	if err != nil {
		return result, err
	}
	movements := []movementRow{}
	for rows.Next() {
		var m movementRow
		if err := rows.Scan(&m.id, &m.kind, &m.amount, &m.natureza, &m.rawDate, &m.property, &m.reversal); err != nil {
			rows.Close()
			return result, err
		}
		movements = append(movements, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}

	for _, movement := range movements {
		sign := int64(1)
		if movement.reversal != 0 {
			sign = -1
		}
		allocations, err := s.allocations(ctx, movement.id)
		if err != nil {
			return result, err
		}
		within := inWindow(movement.rawDate)

		if movement.kind == "receita" {
			if !within {
				continue
			}
			if len(allocations) == 0 && movement.property.Valid {
				income[movement.property.Int64] += sign * movement.amount
			}
			for _, allocation := range allocations {
				income[allocation.ImovelID] += sign * allocation.ValorC
			}
			continue
		}

		if movement.natureza == "investimento" {
			if len(allocations) == 0 && movement.property.Valid {
				investmentCostsTotal[movement.property.Int64] += sign * movement.amount
				if within {
					investmentCostsPeriod[movement.property.Int64] += sign * movement.amount
				}
			}
			for _, allocation := range allocations {
				investmentCostsTotal[allocation.ImovelID] += sign * allocation.ValorC
				if within {
					investmentCostsPeriod[allocation.ImovelID] += sign * allocation.ValorC
				}
			}
		} else if within {
			if len(allocations) == 0 && movement.property.Valid {
				opexCosts[movement.property.Int64] += sign * movement.amount
			}
			for _, allocation := range allocations {
				opexCosts[allocation.ImovelID] += sign * allocation.ValorC
			}
		}
	}

	// 3. Residual installment payments without ledger breakdown (avoids double-counting)
	rows, err = s.DB.QueryContext(
		ctx,
		`SELECT l.imovel_id,
			p.valor_pago_c - COALESCE((SELECT SUM(CASE WHEN m.reversao=1 THEN -m.valor_c ELSE m.valor_c END)
				FROM movimentacoes m WHERE m.parcela_id=p.id AND m.tipo='receita'),0),
			p.data_pagamento
		FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id
		WHERE p.valor_pago_c>0 AND p.status<>'cancelada'`,
	)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var propID, residual int64
		var paidDate sql.NullString
		if err := rows.Scan(&propID, &residual, &paidDate); err != nil {
			rows.Close()
			return result, err
		}
		if residual <= 0 {
			continue
		}
		if !hasWindow {
			income[propID] += residual
		} else if paidDate.Valid && inWindow(paidDate.String) {
			income[propID] += residual
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}

	for _, property := range properties {
		cost := property.base + max64(property.imported, investmentCostsTotal[property.id])
		rendimento := income[property.id]
		despesasOp := opexCosts[property.id]
		resLiq := rendimento - despesasOp
		invCol := investmentCostsTotal[property.id]
		if hasWindow {
			invCol = investmentCostsPeriod[property.id]
		}
		line := domain.StatisticsLine{
			InvestimentosC:        invCol,
			ImovelID:              property.id,
			Apelido:               property.name,
			CustoC:                cost,
			RendimentoC:           rendimento,
			DespesasOperacionaisC: despesasOp,
			ResultadoLiquidoC:     resLiq,
			ROIPct:                domain.ROI(cost, rendimento),
			ROILiquidoPct:         domain.ROI(cost, resLiq),
			DestinadoLocacao:      property.destinadoLocacao,
		}
		result.Linhas = append(result.Linhas, line)
		result.InvestimentosTotalC += line.InvestimentosC
		result.CustoTotalC += line.CustoC
		result.RendimentoTotalC += line.RendimentoC
		result.DespesasOperacionaisTotalC += line.DespesasOperacionaisC
		result.ResultadoLiquidoTotalC += line.ResultadoLiquidoC
	}
	result.ROITotalPct = domain.ROI(result.CustoTotalC, result.RendimentoTotalC)
	result.ROILiquidoTotalPct = domain.ROI(result.CustoTotalC, result.ResultadoLiquidoTotalC)
	return result, nil
}

func (s StatisticsService) AnnualReport(ctx context.Context, year int) (domain.AnnualReport, error) {
	yearStr := fmt.Sprintf("%04d", year)
	report := domain.AnnualReport{
		Ano:        year,
		Meses:      make([]domain.AnnualReportMonth, 0, 12),
		Inquilinos: make([]domain.AnnualReportTenantItem, 0),
	}

	for m := 1; m <= 12; m++ {
		monthStr := fmt.Sprintf("%04d-%02d", year, m)
		var monthItem domain.AnnualReportMonth
		monthItem.Mes = monthStr

		// Income in month (respecting reversao)
		_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN reversao=1 THEN -valor_c ELSE valor_c END), 0) FROM movimentacoes WHERE tipo='receita' AND substr(data,1,7)=?`, monthStr).Scan(&monthItem.ReceitasC)

		// Opex in month (respecting reversao + legacy despesas)
		var movOpex, legOpex int64
		_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN reversao=1 THEN -valor_c ELSE valor_c END), 0) FROM movimentacoes WHERE tipo='despesa' AND (natureza='operacional' OR natureza IS NULL OR natureza='') AND substr(data,1,7)=?`, monthStr).Scan(&movOpex)
		_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(valor_c), 0) FROM despesas WHERE substr(data,1,7)=?`, monthStr).Scan(&legOpex)
		monthItem.DespesasOperacionaisC = movOpex + legOpex

		// Investment in month (respecting reversao)
		_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(CASE WHEN reversao=1 THEN -valor_c ELSE valor_c END), 0) FROM movimentacoes WHERE tipo='despesa' AND natureza='investimento' AND substr(data,1,7)=?`, monthStr).Scan(&monthItem.InvestimentosC)

		monthItem.ResultadoLiquidoC = monthItem.ReceitasC - monthItem.DespesasOperacionaisC
		report.Meses = append(report.Meses, monthItem)

		report.TotalReceitasC += monthItem.ReceitasC
		report.TotalDespesasOpexC += monthItem.DespesasOperacionaisC
		report.TotalInvestimentosC += monthItem.InvestimentosC
	}
	report.ResultadoLiquidoC = report.TotalReceitasC - report.TotalDespesasOpexC

	// Itemized by tenant for IRPF
	rows, err := s.DB.QueryContext(ctx, `
		SELECT COALESCE(t.nome, 'Sem inquilino'), COALESCE(t.cpf, ''), COALESCE(i.apelido, 'Sem imóvel'), SUM(CASE WHEN m.reversao=1 THEN -m.valor_c ELSE m.valor_c END)
		FROM movimentacoes m
		LEFT JOIN inquilinos t ON t.id=m.inquilino_id
		LEFT JOIN imoveis i ON i.id=m.imovel_id
		WHERE m.tipo='receita' AND substr(m.data,1,4)=?
		GROUP BY m.inquilino_id, m.imovel_id
		ORDER BY t.nome, i.apelido
	`, yearStr)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ti domain.AnnualReportTenantItem
			if err := rows.Scan(&ti.InquilinoNome, &ti.InquilinoCPF, &ti.ImovelApelido, &ti.TotalRecebidoC); err == nil {
				report.Inquilinos = append(report.Inquilinos, ti)
			}
		}
	}

	return report, nil
}

func (s StatisticsService) allocations(ctx context.Context, movementID int64) ([]domain.Rateio, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT imovel_id,valor_c FROM movimentacao_rateios WHERE movimentacao_id=?`, movementID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	allocations := []domain.Rateio{}
	for rows.Next() {
		var item domain.Rateio
		if err := rows.Scan(&item.ImovelID, &item.ValorC); err != nil {
			return nil, err
		}
		allocations = append(allocations, item)
	}
	return allocations, rows.Err()
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

package application

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"rerum/backend/pkg/migrations"

	_ "github.com/mattn/go-sqlite3"
)

func TestDashboardOperationalDefinitionsAndPaymentWindows(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "dashboard.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db, ""); err != nil {
		t.Fatal(err)
	}

	for _, property := range []struct {
		name, status, ownership string
	}{
		{"Casa", "disponivel", "proprio"},
		{"Unidade em manutenção", "manutencao", "proprio"},
		{"Imóvel vendido", "disponivel", "vendido"},
	} {
		if _, err := db.Exec(`INSERT INTO imoveis(apelido,status,valor_compra_c,ativo,situacao_patrimonial) VALUES(?,?,1000000,1,?)`, property.name, property.status, property.ownership); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO inquilinos(nome) VALUES('Pessoa de teste')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-01','2026-12-31',5,10000)`); err != nil {
		t.Fatal(err)
	}
	for _, installment := range []struct {
		competency, due, paidDate, status string
		base, paid                        int64
	}{
		{"2026-01", "2026-01-05", "2026-02-10", "parcial", 20000, 5000},
		{"2026-02", "2026-02-05", "", "aberta", 10000, 0},
		{"2026-03", "2026-03-01", "", "aberta", 5000, 0},
	} {
		var paidDate any
		if installment.paidDate != "" {
			paidDate = installment.paidDate
		}
		if _, err := db.Exec(`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,valor_pago_c,data_pagamento,status) VALUES(1,?,?,?,?,?,?)`, installment.competency, installment.due, installment.base, installment.paid, paidDate, installment.status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO despesas(imovel_id,data,categoria,valor_c) VALUES(1,'2026-02-03','iptu',2000)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id) VALUES('despesa','manutenção','2026-02-04',3000,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id,reversao) VALUES('despesa','estorno','2026-02-05',1000,1,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO empreendimentos(nome) VALUES('Projeto investimento')`); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,empreendimento_id,natureza) VALUES('despesa','reforma','2026-02-06',5000,1,'investimento')`)
	if err != nil {
		t.Fatal(err)
	}
	movementID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(?,1,5000)`, movementID); err != nil {
		t.Fatal(err)
	}

	today := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	dashboard, err := (ROIService{DB: db}).Dashboard(context.Background(), today)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.TotalImoveis != 3 || dashboard.Ocupados != 1 || dashboard.Disponiveis != 1 || dashboard.Manutencao != 1 || math.Abs(dashboard.TaxaOcupacaoPct-33.3) > 0.01 {
		t.Fatalf("unexpected active portfolio counts: %+v", dashboard)
	}
	if dashboard.ReceitaMesC != 5000 || dashboard.AReceberMesC != 10000 {
		t.Fatalf("monthly receipt/competency totals are wrong: received=%d due=%d", dashboard.ReceitaMesC, dashboard.AReceberMesC)
	}
	if dashboard.DespesasOperacionaisMesC != 4000 || dashboard.ResultadoOperacionalMesC != 1000 {
		t.Fatalf("investment, reversal, or expense classification is wrong: expenses=%d result=%d", dashboard.DespesasOperacionaisMesC, dashboard.ResultadoOperacionalMesC)
	}
	if len(dashboard.Vencidas) != 2 || dashboard.VencidasQtd != 2 || dashboard.VencidasC != 25000 {
		t.Fatalf("overdue balance/list does not match: rows=%+v count=%d balance=%d", dashboard.Vencidas, dashboard.VencidasQtd, dashboard.VencidasC)
	}
	if len(dashboard.Proximas) != 1 || dashboard.Proximas[0].Vencimento != "2026-03-01" {
		t.Fatalf("upcoming window includes the wrong installments: %+v", dashboard.Proximas)
	}
	if dashboard.CapitalInvestidoC != 3_005_000 || dashboard.ROICarteira12mPct == nil || *dashboard.ROICarteira12mPct != 0.03 {
		t.Fatalf("portfolio ROI must use the aggregated capital base: capital=%d roi=%v", dashboard.CapitalInvestidoC, dashboard.ROICarteira12mPct)
	}
}

func TestDashboardAReceberIncludesUnpaidOfCurrentMonth(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "dashboard_receivable.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`INSERT INTO imoveis(id,apelido,status,valor_compra_c,ativo,situacao_patrimonial) VALUES(1,'Apto Teste','ocupado',1000000,1,'proprio')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO inquilinos(id,nome) VALUES(1,'Inquilino Teste')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO locacoes(id,imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c,status) VALUES(1,1,1,'2026-01-01','2026-12-31',10,120000,'ativa')`); err != nil {
		t.Fatal(err)
	}

	// Installment 1: Due on 2026-11-05 (past due, today is 2026-11-10), competence 2026-10 (paid in arrears). Unpaid.
	// Installment 2: Due on 2026-11-20 (future due), competence 2026-11. Unpaid.
	// Installment 3: Due on 2026-11-02, competence 2026-11. Already paid on 2026-11-02.
	// Installment 4: Due on 2026-12-10, competence 2026-11. Due next month.
	if _, err := db.Exec(`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,valor_pago_c,status) VALUES
		(1,'2026-10','2026-11-05',150000,0,'aberta'),
		(1,'2026-11','2026-11-20',120000,0,'aberta'),
		(1,'2026-11','2026-11-02',120000,120000,'paga'),
		(1,'2026-11','2026-12-10',100000,0,'aberta')`); err != nil {
		t.Fatal(err)
	}

	today := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	dashboard, err := (ROIService{DB: db}).Dashboard(context.Background(), today)
	if err != nil {
		t.Fatal(err)
	}

	// Installment 1 (150000, due 2026-11-05, unpaid) + Installment 2 (120000, due 2026-11-20, unpaid) + Installment 4 (100000, comp 2026-11)
	// Both installment 1 (due in current month) and installment 2 (due in current month) must be in AReceberMesC.
	if dashboard.AReceberMesC < 270000 {
		t.Fatalf("expected at least 270000 (150000 past due in month + 120000 future in month), got %d", dashboard.AReceberMesC)
	}
	// Installment 1 is past due relative to today (Nov 10)
	if dashboard.VencidasC != 150000 || dashboard.VencidasQtd != 1 {
		t.Fatalf("expected 1 vencida of 150000, got %d (count %d)", dashboard.VencidasC, dashboard.VencidasQtd)
	}
}

func TestROIAndStatisticsParityAcrossPeriodsAndCostClassifications(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "roi_parity.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrations.Apply(context.Background(), db, ""); err != nil {
		t.Fatal(err)
	}

	// Property 1: base 10_000_000 + imported 2_000_000 + direct capex 3_000_000 => capital = 10M + max(2M, 3M) = 13_000_000
	if _, err := db.Exec(`INSERT INTO imoveis(id,apelido,status,valor_compra_c,investimento_importado_c,ativo,destinado_locacao,situacao_patrimonial) VALUES
		(1,'IND-643-01','disponivel',10000000,2000000,1,1,'proprio')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO empreendimentos(id,nome,ativo) VALUES(1,'Condomínio Indaiá',1)`); err != nil {
		t.Fatal(err)
	}

	// Historical consolidated receipt in 2025-12-31 (outside 2026 fiscal year, inside rolling 12m from Nov 2026)
	if _, err := db.Exec(`INSERT INTO movimentacoes(tipo,natureza,categoria,data,valor_c,imovel_id,reversao) VALUES
		('receita','operacional','aluguel','2025-12-31',1500000,1,0),
		('receita','operacional','aluguel','2026-08-10',500000,1,0),
		('receita','operacional','aluguel','2026-08-12',100000,1,1),
		('despesa','investimento','reforma','2026-03-15',3000000,1,0),
		('despesa','operacional','manutenção','2026-09-01',40000,1,0)`); err != nil {
		t.Fatal(err)
	}

	// Operational expense allocated via project (must count as opex, NOT capex!)
	res, err := db.Exec(`INSERT INTO movimentacoes(tipo,natureza,categoria,data,valor_c,empreendimento_id,reversao) VALUES
		('despesa','operacional','iptu','2026-09-10',60000,1,0)`)
	if err != nil {
		t.Fatal(err)
	}
	movID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(?,1,60000)`, movID); err != nil {
		t.Fatal(err)
	}

	// Legacy operational expense in 2026
	if _, err := db.Exec(`INSERT INTO despesas(imovel_id,data,categoria,valor_c) VALUES(1,'2026-09-15','taxa',30000)`); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	roiSvc := ROIService{DB: db}
	statsSvc := StatisticsService{DB: db}

	// Window 1: Fiscal Year 2026 [2026-01-01, 2026-12-31]
	// Net receipts in 2026 = 500_000 - 100_000 (reversal) = 400_000
	// Opex in 2026 = 40_000 (direct) + 60_000 (allocated project opex) + 30_000 (legacy despesa) = 130_000
	// Net operational result in 2026 = 270_000; Capital = 13_000_000 => Net ROI = round(270_000 * 10000 / 13_000_000)/100 = 2.08%
	fyStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fyEnd := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	propFY, err := roiSvc.PropertyRange(ctx, 1, fyStart, fyEnd)
	if err != nil {
		t.Fatal(err)
	}
	statsFY, err := statsSvc.PortfolioRange(ctx, true, fyStart, fyEnd)
	if err != nil {
		t.Fatal(err)
	}
	if propFY.InvestimentoC != 13_000_000 || statsFY.CustoTotalC != 13_000_000 {
		t.Fatalf("expected capital 13_000_000, got roi=%d stats=%d", propFY.InvestimentoC, statsFY.CustoTotalC)
	}
	if propFY.Recebido12mC != 400_000 || propFY.Despesas12mC != 130_000 {
		t.Fatalf("unexpected 2026 cashflow: rec=%d desp=%d", propFY.Recebido12mC, propFY.Despesas12mC)
	}
	if propFY.ROIAnualPct != 2.08 || statsFY.ROILiquidoTotalPct != 2.08 || statsFY.Linhas[0].ROILiquidoPct != propFY.ROIAnualPct {
		t.Fatalf("ROI mismatch in fiscal year 2026: roiSvc=%v statsSvc=%v", propFY.ROIAnualPct, statsFY.ROILiquidoTotalPct)
	}

	// Window 2: All-time (includes 2025-12-31 receipt of 1_500_000 => total net = 1_770_000 => 13.62%)
	statsAll, err := statsSvc.Portfolio(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if propFY.ROIAcumuladoPct != 13.62 || statsAll.ROILiquidoTotalPct != 13.62 {
		t.Fatalf("all-time ROI mismatch: prop=%v stats=%v", propFY.ROIAcumuladoPct, statsAll.ROILiquidoTotalPct)
	}
}

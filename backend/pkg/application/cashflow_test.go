package application

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"rerum/backend/pkg/migrations"
)

func cashflowDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cashflow.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := migrations.Apply(t.Context(), db, ""); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCashflowUsesRealPartialReceiptDatesAndFlagsLegacyResidual(t *testing.T) {
	db := cashflowDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa')`,
		`INSERT INTO inquilinos(nome) VALUES('Pessoa')`,
		`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-05','2026-04-05',5,100000)`,
		`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,valor_pago_c,data_pagamento,status) VALUES(1,'2026-01','2026-01-05',100000,100000,'2026-02-02','paga'),(1,'2026-01','2026-01-05',20000,20000,'2026-01-10','paga'),(1,'2026-02','2026-02-05',15000,15000,'2026-02-10','paga')`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,parcela_id,imovel_id) VALUES('receita','aluguel','2026-01-05',60000,1,1),('receita','aluguel','2026-02-02',40000,1,1),('receita','aluguel','2026-02-10',5000,3,1)`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,reversao,imovel_id) VALUES('receita','outros','2026-02-01',3000,0,1),('receita','estorno','2026-02-02',500,1,1),('despesa','manutenção','2026-02-03',10000,0,1),('despesa','reembolso','2026-02-04',2000,1,1),('receita','futuro','2026-03-01',99999,0,1),('receita','antigo','2025-01-01',99999,0,1)`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,natureza,imovel_id) VALUES('despesa','obra','2026-02-05',90000,'investimento',1)`,
		`INSERT INTO despesas(imovel_id,data,categoria,valor_c) VALUES(1,'2026-02-05','iptu',1000)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	items, err := (ROIService{DB: db}).MonthlyCashflow(t.Context(), time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 12 || items[0].Mes != "2025-03" || items[11].Mes != "2026-02" {
		t.Fatalf("wrong month window: %+v", items)
	}
	jan, feb := items[10], items[11]
	if jan.ReceitasC != 80000 || jan.ReceitasSemDetalhamentoC != 20000 {
		t.Fatalf("January receipt breakdown: %+v", jan)
	}
	if feb.ReceitasC != 57500 || feb.ReceitasSemDetalhamentoC != 10000 || feb.DespesasOperacionaisC != 9000 || feb.ResultadoOperacionalC != 48500 {
		t.Fatalf("February cashflow: %+v", feb)
	}
	for _, item := range items[:10] {
		if item.ReceitasC != 0 || item.DespesasOperacionaisC != 0 {
			t.Fatalf("zero month missing: %+v", item)
		}
	}
	var paid int64
	if err := db.QueryRow(`SELECT valor_pago_c FROM parcelas WHERE id=1`).Scan(&paid); err != nil || paid != 100000 {
		t.Fatal("cashflow query modified the installment")
	}
}

func TestCashflowEmptyMonthsAndQueryErrors(t *testing.T) {
	db := cashflowDB(t)
	items, err := (ROIService{DB: db}).MonthlyCashflow(t.Context(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(items) != 12 || items[0].Mes != "2025-02" || items[11].Mes != "2026-01" {
		t.Fatalf("empty calendar: %+v %v", items, err)
	}
	db.Close()
	if _, err := (ROIService{DB: db}).MonthlyCashflow(t.Context(), time.Now()); err == nil {
		t.Fatal("query failure should not look like a zero balance")
	}
}

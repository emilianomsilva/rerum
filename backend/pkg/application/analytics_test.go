package application

import (
	"database/sql"
	"strings"
	"testing"
	"time"
)

func analyticsFilter() AnalyticsFilter {
	return AnalyticsFilter{
		Inicio:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Fim:           time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC),
		Eixo:          "tempo",
		Series:        "tipo",
		Metrica:       "ambos",
		Granularidade: "mes",
	}
}

func TestAnalyticsSeriesMatchesCashflowAndDoesNotChangeData(t *testing.T) {
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
	before := analyticsSnapshot(t, db)
	result, err := (AnalyticsService{DB: db}).Series(t.Context(), analyticsFilter())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Categorias) != 2 || result.Categorias[0].Chave != "2026-01" || result.Categorias[1].Chave != "2026-02" {
		t.Fatalf("categorias inesperadas: %+v", result.Categorias)
	}
	if len(result.Series) != 2 {
		t.Fatalf("séries inesperadas: %+v", result.Series)
	}
	receitas, despesas := result.Series[0], result.Series[1]
	if receitas.Chave != "receitas" || despesas.Chave != "despesas" {
		t.Fatalf("ordem de séries inesperada: %+v", result.Series)
	}
	if got := receitas.ValoresC; len(got) != 2 || got[0] != 80000 || got[1] != 57500 {
		t.Fatalf("receitas: %v", got)
	}
	if got := despesas.ValoresC; len(got) != 2 || got[0] != 0 || got[1] != 9000 {
		t.Fatalf("despesas: %v", got)
	}
	if !strings.Contains(strings.Join(result.Avisos, " "), "Recebimentos antigos") {
		t.Fatalf("aviso de recebimentos antigos ausente: %v", result.Avisos)
	}
	if after := analyticsSnapshot(t, db); after != before {
		t.Fatalf("consultas alteraram dados: antes=%q depois=%q", before, after)
	}
}

func TestAnalyticsPropertyAxisUsesAllocationsAndFilters(t *testing.T) {
	db := cashflowDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa A'),('Casa B')`,
		`INSERT INTO empreendimentos(id,nome) VALUES(1,'Projeto')`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id) VALUES('receita','aluguel','2026-01-10',5000,1)`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,empreendimento_id) VALUES('receita','outros','2026-01-15',3000,1),('despesa','rateio','2026-01-20',10000,1)`,
		`INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(3,1,6000),(3,2,4000)`,
		`INSERT INTO despesas(imovel_id,data,categoria,valor_c) VALUES(1,'2026-01-25','iptu',1000)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	filter := analyticsFilter()
	filter.Eixo = "imovel"
	result, err := (AnalyticsService{DB: db}).Series(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Categorias) != 3 {
		t.Fatalf("categorias de imóveis: %+v", result.Categorias)
	}
	byCategory := map[string]map[string]int64{}
	for _, series := range result.Series {
		for index, category := range result.Categorias {
			if byCategory[category.Chave] == nil {
				byCategory[category.Chave] = map[string]int64{}
			}
			byCategory[category.Chave][series.Chave] = series.ValoresC[index]
		}
	}
	if byCategory["imovel:1"]["receitas"] != 5000 || byCategory["imovel:1"]["despesas"] != 7000 {
		t.Fatalf("imóvel A: %+v", byCategory["imovel:1"])
	}
	if byCategory["imovel:2"]["despesas"] != 4000 {
		t.Fatalf("imóvel B: %+v", byCategory["imovel:2"])
	}
	if byCategory["sem_imovel"]["receitas"] != 3000 {
		t.Fatalf("sem imóvel: %+v", byCategory["sem_imovel"])
	}
	if !strings.Contains(strings.Join(result.Avisos, " "), "Sem imóvel") {
		t.Fatalf("aviso de lançamento sem imóvel ausente: %v", result.Avisos)
	}

	filter.ImovelIDs = []int64{1}
	filtered, err := (AnalyticsService{DB: db}).Series(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Categorias) != 1 || filtered.Categorias[0].Chave != "imovel:1" {
		t.Fatalf("filtro de imóvel: %+v", filtered.Categorias)
	}
	if filtered.Series[0].ValoresC[0] != 5000 || filtered.Series[1].ValoresC[0] != 7000 {
		t.Fatalf("valores filtrados: %+v", filtered.Series)
	}
	if strings.Contains(strings.Join(filtered.Avisos, " "), "Sem imóvel") {
		t.Fatalf("aviso de sem imóvel não deveria aparecer com filtro: %v", filtered.Avisos)
	}
}

func TestAnalyticsTimeSeriesByPropertyAndCategoryAxes(t *testing.T) {
	db := cashflowDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa A'),('Casa B')`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id) VALUES('receita','aluguel','2026-01-10',5000,1),('receita','aluguel','2026-02-10',7000,2),('despesa','iptu','2026-01-20',1000,1),('despesa','manutenção','2026-02-20',2000,2)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	filter := analyticsFilter()
	filter.Series = "imovel"
	filter.Metrica = "receitas"
	result, err := (AnalyticsService{DB: db}).Series(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Categorias) != 2 || len(result.Series) != 2 {
		t.Fatalf("séries por imóvel: %+v %+v", result.Categorias, result.Series)
	}
	if result.Series[0].Rotulo != "Casa A" || result.Series[0].ValoresC[0] != 5000 || result.Series[0].ValoresC[1] != 0 {
		t.Fatalf("Casa A: %+v", result.Series[0])
	}
	if result.Series[1].Rotulo != "Casa B" || result.Series[1].ValoresC[1] != 7000 {
		t.Fatalf("Casa B: %+v", result.Series[1])
	}

	filter = analyticsFilter()
	filter.Eixo = "categoria"
	filter.Series = "tipo"
	filter.Metrica = "resultado"
	result, err = (AnalyticsService{DB: db}).Series(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Series) != 1 || result.Series[0].Chave != "resultado" {
		t.Fatalf("resultado por categoria: %+v", result.Series)
	}
	values := map[string]int64{}
	for index, category := range result.Categorias {
		values[category.Rotulo] = result.Series[0].ValoresC[index]
	}
	if values["aluguel"] != 12000 || values["iptu"] != -1000 || values["manutenção"] != -2000 {
		t.Fatalf("valores por categoria: %+v", values)
	}
}

func TestAnalyticsQuarterAndYearBuckets(t *testing.T) {
	db := cashflowDB(t)
	if _, err := db.Exec(`INSERT INTO imoveis(apelido) VALUES('Casa')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id) VALUES('receita','aluguel','2026-02-10',5000,1),('receita','aluguel','2026-05-10',7000,1),('receita','aluguel','2027-01-10',9000,1)`); err != nil {
		t.Fatal(err)
	}
	filter := analyticsFilter()
	filter.Inicio = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	filter.Fim = time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC)
	filter.Granularidade = "trimestre"
	filter.Metrica = "receitas"
	result, err := (AnalyticsService{DB: db}).Series(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Categorias) != 8 || result.Categorias[0].Rotulo != "1º tri/2026" || result.Categorias[7].Rotulo != "4º tri/2027" {
		t.Fatalf("trimestres: %+v", result.Categorias)
	}
	if result.Series[0].ValoresC[0] != 5000 || result.Series[0].ValoresC[1] != 7000 || result.Series[0].ValoresC[4] != 9000 {
		t.Fatalf("valores trimestrais: %+v", result.Series[0].ValoresC)
	}

	filter.Granularidade = "ano"
	result, err = (AnalyticsService{DB: db}).Series(t.Context(), filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Categorias) != 2 || result.Categorias[0].Chave != "2026" || result.Categorias[1].Chave != "2027" {
		t.Fatalf("anos: %+v", result.Categorias)
	}
	if result.Series[0].ValoresC[0] != 12000 || result.Series[0].ValoresC[1] != 9000 {
		t.Fatalf("valores anuais: %+v", result.Series[0].ValoresC)
	}
}

func TestAnalyticsValidationRejectsInvalidCombinations(t *testing.T) {
	for name, filter := range map[string]AnalyticsFilter{
		"ambos com série por imóvel": func() AnalyticsFilter {
			f := analyticsFilter()
			f.Series = "imovel"
			return f
		}(),
		"mesma dimensão": func() AnalyticsFilter {
			f := analyticsFilter()
			f.Eixo = "categoria"
			f.Series = "categoria"
			return f
		}(),
		"período invertido": func() AnalyticsFilter {
			f := analyticsFilter()
			f.Inicio, f.Fim = f.Fim, f.Inicio
			return f
		}(),
		"granularidade": func() AnalyticsFilter {
			f := analyticsFilter()
			f.Granularidade = "semana"
			return f
		}(),
	} {
		if err := ValidateAnalyticsFilter(filter); err == nil {
			t.Fatalf("%s deveria ser rejeitado", name)
		}
	}
}

func analyticsSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	var parts []string
	for _, query := range []string{
		`SELECT COUNT(*)||':'||COALESCE(SUM(valor_pago_c),0) FROM parcelas`,
		`SELECT COUNT(*)||':'||COALESCE(SUM(valor_c),0) FROM movimentacoes`,
		`SELECT COUNT(*)||':'||COALESCE(SUM(valor_c),0) FROM despesas`,
		`SELECT COUNT(*)||':'||COALESCE(SUM(valor_c),0) FROM movimentacao_rateios`,
	} {
		var value string
		if err := db.QueryRow(query).Scan(&value); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "|")
}

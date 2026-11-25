package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"rerum/backend/pkg/domain"
)

// AnalyticsFilter describes a read-only chart query. The service only runs
// SELECTs and never rewrites receipts, installments or ledger entries.
type AnalyticsFilter struct {
	Inicio        time.Time
	Fim           time.Time
	Eixo          string
	Series        string
	Metrica       string
	Granularidade string
	ImovelIDs     []int64
}

type AnalyticsService struct{ DB *sql.DB }

type analyticsFact struct {
	date          time.Time
	kind          string
	categoryKey   string
	categoryLabel string
	categoryGroup string
	propertyID    int64
	amount        int64
	portions      map[int64]int64
}

type analyticsProperty struct {
	id   int64
	name string
}

type analyticsCategory struct {
	key   string
	label string
}

type analyticsSeries struct {
	key   string
	label string
}

type timeBucket struct {
	key   string
	label string
}

const analyticsOtherKey = "__outros__"

// Series returns chart data using the same cash-basis rules as the dashboard:
// ledger receipts, operational expenses, legacy expenses, investment expenses
// and old receipts without a detailed ledger. All calculations are read-only.
func (s AnalyticsService) Series(ctx context.Context, filter AnalyticsFilter) (domain.AnalyticsResult, error) {
	result := domain.AnalyticsResult{
		Categorias: []domain.AnalyticsCategory{},
		Series:     []domain.AnalyticsSeries{},
		Avisos:     []string{},
	}
	if err := validateAnalyticsFilter(filter); err != nil {
		return result, err
	}
	facts, warnings, err := s.facts(ctx, filter)
	if err != nil {
		return result, err
	}
	categoryDefs, categoryGroups := buildAnalyticsCategories(facts, filter.ImovelIDs)
	for index := range facts {
		if grouped, ok := categoryGroups[facts[index].categoryKey]; ok {
			facts[index].categoryGroup = grouped
		} else {
			facts[index].categoryGroup = facts[index].categoryKey
		}
	}
	properties, err := s.propertyDefs(ctx, facts, filter.ImovelIDs)
	if err != nil {
		return result, err
	}

	categories, categoryIndex, err := analyticsAxis(filter, facts, properties, categoryDefs)
	if err != nil {
		return result, err
	}
	seriesDefs, seriesIndex := analyticsSeriesDefs(filter, properties, categoryDefs)
	matrix := make([][]int64, len(seriesDefs))
	for index := range matrix {
		matrix[index] = make([]int64, len(categories))
	}
	for _, fact := range facts {
		selected := selectedPortions(fact, filter.ImovelIDs)
		if len(selected) == 0 {
			continue
		}
		if filter.Eixo == "imovel" {
			for propertyID, amount := range selected {
				category, ok := categoryIndex[propertyAxisKey(propertyID)]
				if !ok {
					continue
				}
				addAnalyticsFact(matrix, seriesIndex, filter, fact, amount, category)
			}
			continue
		}
		categoryKey := analyticsAxisKey(filter.Eixo, filter.Granularidade, fact)
		category, ok := categoryIndex[categoryKey]
		if !ok {
			continue
		}
		if filter.Series == "imovel" {
			for propertyID, amount := range selected {
				series, ok := seriesIndex[propertySeriesKey(propertyID)]
				if !ok {
					continue
				}
				value, include := metricContribution(fact.kind, amount, filter.Metrica)
				if include {
					matrix[series][category] += value
				}
			}
			continue
		}
		addAnalyticsFact(matrix, seriesIndex, filter, fact, selectedTotal(selected), category)
	}

	for index, series := range seriesDefs {
		result.Series = append(result.Series, domain.AnalyticsSeries{
			Chave:    series.key,
			Rotulo:   series.label,
			ValoresC: matrix[index],
		})
	}
	for _, category := range categories {
		result.Categorias = append(result.Categorias, domain.AnalyticsCategory{
			Chave:  category.key,
			Rotulo: category.label,
		})
	}
	result.Avisos = append(result.Avisos, warnings...)
	return result, nil
}

func (s AnalyticsService) facts(ctx context.Context, filter AnalyticsFilter) ([]analyticsFact, []string, error) {
	facts := []analyticsFact{}
	warnings := []string{}
	start := filter.Inicio.Format("2006-01-02")
	end := filter.Fim.Format("2006-01-02")

	allocations := map[int64]map[int64]int64{}
	rows, err := s.DB.QueryContext(ctx, `SELECT r.movimentacao_id,r.imovel_id,r.valor_c
		FROM movimentacao_rateios r JOIN movimentacoes m ON m.id=r.movimentacao_id
		WHERE m.data>=? AND m.data<=?`, start, end)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var movementID, propertyID, amount int64
		if err := rows.Scan(&movementID, &propertyID, &amount); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if allocations[movementID] == nil {
			allocations[movementID] = map[int64]int64{}
		}
		allocations[movementID][propertyID] += amount
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	rows, err = s.DB.QueryContext(ctx, `SELECT m.id,m.tipo,m.categoria,m.data,m.valor_c,m.imovel_id,m.reversao,COALESCE(m.natureza,'operacional')
		FROM movimentacoes m WHERE m.data>=? AND m.data<=? ORDER BY m.id`, start, end)
	if err != nil {
		return nil, nil, err
	}
	allocationMismatch := false
	propertyless := false
	for rows.Next() {
		var id, amount int64
		var kind, category, rawDate, nature string
		var property sql.NullInt64
		var reversal int
		if err := rows.Scan(&id, &kind, &category, &rawDate, &amount, &property, &reversal, &nature); err != nil {
			rows.Close()
			return nil, nil, err
		}
		parsed, err := domain.ParseDate(rawDate)
		if err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("data inválida em movimentação %d: %w", id, err)
		}
		if kind == "despesa" && nature == "investimento" {
			kind = "investimento"
		}
		sign := int64(1)
		if reversal != 0 {
			sign = -1
		}
		fact := analyticsFact{
			date:          parsed,
			kind:          kind,
			categoryKey:   category,
			categoryLabel: category,
			amount:        sign * amount,
		}
		if strings.TrimSpace(category) == "" {
			fact.categoryKey = "sem_categoria"
			fact.categoryLabel = "Sem categoria"
		}
		if linked := allocations[id]; len(linked) > 0 {
			portions := make(map[int64]int64, len(linked))
			var rawTotal int64
			for propertyID, value := range linked {
				rawTotal += value
				portions[propertyID] += sign * value
			}
			fact.portions = portions
			if rawTotal != amount {
				allocationMismatch = true
			}
		} else if property.Valid {
			fact.propertyID = property.Int64
		} else {
			propertyless = true
		}
		facts = append(facts, fact)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	legacyReceipt := false
	rows, err = s.DB.QueryContext(ctx, `SELECT p.data_pagamento,
		p.valor_pago_c-COALESCE((SELECT SUM(CASE WHEN m.reversao=1 THEN -m.valor_c ELSE m.valor_c END)
			FROM movimentacoes m WHERE m.parcela_id=p.id AND m.tipo='receita'),0),
		l.imovel_id
		FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id
		WHERE p.valor_pago_c>0 AND p.data_pagamento>=? AND p.data_pagamento<=? AND p.status<>'cancelada'`, start, end)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var rawDate string
		var residual, propertyID int64
		if err := rows.Scan(&rawDate, &residual, &propertyID); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if residual <= 0 {
			continue
		}
		parsed, err := domain.ParseDate(rawDate)
		if err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("data de pagamento inválida: %w", err)
		}
		legacyReceipt = true
		facts = append(facts, analyticsFact{
			date:          parsed,
			kind:          "receita",
			categoryKey:   "recebimentos_antigos",
			categoryLabel: "Recebimentos antigos",
			propertyID:    propertyID,
			amount:        residual,
		})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	rows, err = s.DB.QueryContext(ctx, `SELECT data,categoria,valor_c,imovel_id FROM despesas
		WHERE data>=? AND data<=? ORDER BY id`, start, end)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var rawDate, category string
		var amount, propertyID int64
		if err := rows.Scan(&rawDate, &category, &amount, &propertyID); err != nil {
			rows.Close()
			return nil, nil, err
		}
		parsed, err := domain.ParseDate(rawDate)
		if err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("data inválida em despesa: %w", err)
		}
		fact := analyticsFact{
			date:          parsed,
			kind:          "despesa",
			categoryKey:   category,
			categoryLabel: category,
			propertyID:    propertyID,
			amount:        amount,
		}
		if strings.TrimSpace(category) == "" {
			fact.categoryKey = "sem_categoria"
			fact.categoryLabel = "Sem categoria"
		}
		facts = append(facts, fact)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}

	if legacyReceipt {
		warnings = appendUnique(warnings, "Recebimentos antigos sem detalhamento por baixa foram incluídos na data de pagamento conhecida.")
	}
	if allocationMismatch {
		warnings = appendUnique(warnings, "Alguns rateios não fecham com o valor original; os gráficos usam os valores rateados.")
	}
	if propertyless && len(filter.ImovelIDs) == 0 {
		warnings = appendUnique(warnings, "Lançamentos sem imóvel aparecem como 'Sem imóvel'.")
	}
	return facts, warnings, nil
}

func (s AnalyticsService) propertyDefs(ctx context.Context, facts []analyticsFact, selectedIDs []int64) ([]analyticsProperty, error) {
	names := map[int64]string{}
	order := []int64{}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,apelido FROM imoveis ORDER BY apelido,id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		names[id] = name
		order = append(order, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	needed := map[int64]bool{}
	for _, fact := range facts {
		if len(fact.portions) > 0 {
			for propertyID := range fact.portions {
				needed[propertyID] = true
			}
		} else {
			needed[fact.propertyID] = true
		}
	}
	selected := map[int64]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	result := []analyticsProperty{}
	if len(selected) > 0 {
		for _, id := range order {
			if selected[id] {
				result = append(result, analyticsProperty{id: id, name: names[id]})
			}
		}
	} else {
		for _, id := range order {
			if needed[id] {
				result = append(result, analyticsProperty{id: id, name: names[id]})
			}
		}
		if needed[0] {
			result = append(result, analyticsProperty{id: 0, name: "Sem imóvel"})
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].id == 0 {
			return false
		}
		if result[j].id == 0 {
			return true
		}
		return result[i].name < result[j].name
	})
	return result, nil
}

func buildAnalyticsCategories(facts []analyticsFact, selectedIDs []int64) ([]analyticsCategory, map[string]string) {
	totals := map[string]int64{}
	labels := map[string]string{}
	for _, fact := range facts {
		selected := selectedPortions(fact, selectedIDs)
		total := selectedTotal(selected)
		if total < 0 {
			total = -total
		}
		if total == 0 {
			continue
		}
		totals[fact.categoryKey] += total
		labels[fact.categoryKey] = fact.categoryLabel
	}
	keys := make([]string, 0, len(totals))
	for key := range totals {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if totals[keys[i]] != totals[keys[j]] {
			return totals[keys[i]] > totals[keys[j]]
		}
		return labels[keys[i]] < labels[keys[j]]
	})
	categories := make([]analyticsCategory, 0, len(keys))
	groups := map[string]string{}
	const limit = 12
	for index, key := range keys {
		if index >= limit {
			groups[key] = analyticsOtherKey
			continue
		}
		categories = append(categories, analyticsCategory{key: key, label: labels[key]})
	}
	if len(keys) > limit {
		categories = append(categories, analyticsCategory{key: analyticsOtherKey, label: "Outros"})
	}
	return categories, groups
}

func analyticsAxis(filter AnalyticsFilter, facts []analyticsFact, properties []analyticsProperty, categories []analyticsCategory) ([]analyticsCategory, map[string]int, error) {
	items := []analyticsCategory{}
	switch filter.Eixo {
	case "tempo":
		buckets, err := timeBuckets(filter.Inicio, filter.Fim, filter.Granularidade)
		if err != nil {
			return nil, nil, err
		}
		for _, bucket := range buckets {
			items = append(items, analyticsCategory{key: bucket.key, label: bucket.label})
		}
	case "imovel":
		for _, property := range properties {
			items = append(items, analyticsCategory{key: propertyAxisKey(property.id), label: property.name})
		}
	case "categoria":
		for _, category := range categories {
			items = append(items, analyticsCategory{key: categoryAxisKey(category.key), label: category.label})
		}
	}
	index := make(map[string]int, len(items))
	for position, item := range items {
		index[item.key] = position
	}
	return items, index, nil
}

func analyticsSeriesDefs(filter AnalyticsFilter, properties []analyticsProperty, categories []analyticsCategory) ([]analyticsSeries, map[string]int) {
	items := []analyticsSeries{}
	switch filter.Series {
	case "tipo":
		switch filter.Metrica {
		case "ambos":
			items = append(items,
				analyticsSeries{key: "receitas", label: "Receitas"},
				analyticsSeries{key: "despesas", label: "Despesas operacionais"},
			)
		case "resultado":
			items = append(items, analyticsSeries{key: "resultado", label: "Resultado operacional"})
		case "investimentos":
			items = append(items, analyticsSeries{key: "investimentos", label: "Investimentos / obras"})
		case "receitas":
			items = append(items, analyticsSeries{key: "receitas", label: "Receitas"})
		case "despesas":
			items = append(items, analyticsSeries{key: "despesas", label: "Despesas operacionais"})
		}
	case "imovel":
		for _, property := range properties {
			items = append(items, analyticsSeries{key: propertySeriesKey(property.id), label: property.name})
		}
	case "categoria":
		for _, category := range categories {
			items = append(items, analyticsSeries{key: categorySeriesKey(category.key), label: category.label})
		}
	}
	index := make(map[string]int, len(items))
	for position, item := range items {
		index[item.key] = position
	}
	return items, index
}

func addAnalyticsFact(matrix [][]int64, seriesIndex map[string]int, filter AnalyticsFilter, fact analyticsFact, amount int64, category int) {
	value, include := metricContribution(fact.kind, amount, filter.Metrica)
	if !include {
		return
	}
	switch filter.Series {
	case "tipo":
		key := filter.Metrica
		if filter.Metrica == "ambos" {
			switch fact.kind {
			case "receita":
				key = "receitas"
			case "despesa":
				key = "despesas"
			default:
				return
			}
		}
		if series, ok := seriesIndex[key]; ok {
			matrix[series][category] += value
		}
	case "categoria":
		if series, ok := seriesIndex[categorySeriesKey(fact.categoryGroup)]; ok {
			matrix[series][category] += value
		}
	}
}

func metricContribution(kind string, amount int64, metric string) (int64, bool) {
	switch metric {
	case "receitas":
		if kind == "receita" {
			return amount, true
		}
	case "despesas":
		if kind == "despesa" {
			return amount, true
		}
	case "investimentos":
		if kind == "investimento" {
			return amount, true
		}
	case "resultado":
		switch kind {
		case "receita":
			return amount, true
		case "despesa":
			return -amount, true
		}
	case "ambos":
		if kind == "receita" || kind == "despesa" {
			return amount, true
		}
	}
	return 0, false
}

func selectedPortions(fact analyticsFact, selectedIDs []int64) map[int64]int64 {
	selected := map[int64]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	result := map[int64]int64{}
	if len(fact.portions) > 0 {
		for propertyID, amount := range fact.portions {
			if len(selected) == 0 || selected[propertyID] {
				result[propertyID] += amount
			}
		}
		return result
	}
	if fact.propertyID != 0 {
		if len(selected) == 0 || selected[fact.propertyID] {
			result[fact.propertyID] = fact.amount
		}
	} else if len(selected) == 0 {
		result[0] = fact.amount
	}
	return result
}

func selectedTotal(portions map[int64]int64) int64 {
	var total int64
	for _, amount := range portions {
		total += amount
	}
	return total
}

func propertyAxisKey(id int64) string {
	if id == 0 {
		return "sem_imovel"
	}
	return fmt.Sprintf("imovel:%d", id)
}

func propertySeriesKey(id int64) string {
	return propertyAxisKey(id)
}

func categoryAxisKey(key string) string {
	return "categoria:" + key
}

func categorySeriesKey(key string) string {
	return "categoria:" + key
}

func analyticsAxisKey(axis, granularity string, fact analyticsFact) string {
	switch axis {
	case "tempo":
		return bucketKey(fact.date, granularity)
	case "categoria":
		return categoryAxisKey(fact.categoryGroup)
	default:
		return propertyAxisKey(fact.propertyID)
	}
}

func timeBuckets(start, end time.Time, granularity string) ([]timeBucket, error) {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	var cursor time.Time
	switch granularity {
	case "dia":
		cursor = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	case "mes":
		cursor = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "trimestre":
		month := time.Month(((int(start.Month())-1)/3)*3 + 1)
		cursor = time.Date(start.Year(), month, 1, 0, 0, 0, 0, time.UTC)
	case "ano":
		cursor = time.Date(start.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return nil, errors.New("granularidade inválida")
	}
	items := []timeBucket{}
	for !cursor.After(end) {
		var key, label string
		switch granularity {
		case "dia":
			key = cursor.Format("2006-01-02")
			label = cursor.Format("02/01")
		case "mes":
			key = cursor.Format("2006-01")
			label = cursor.Format("01/2006")
		case "trimestre":
			quarter := (int(cursor.Month())-1)/3 + 1
			key = fmt.Sprintf("%d-Q%d", cursor.Year(), quarter)
			label = fmt.Sprintf("%dº tri/%d", quarter, cursor.Year())
		case "ano":
			key = cursor.Format("2006")
			label = cursor.Format("2006")
		}
		items = append(items, timeBucket{key: key, label: label})
		if len(items) > 600 {
			return nil, errors.New("período muito longo para a granularidade escolhida")
		}
		switch granularity {
		case "dia":
			cursor = cursor.AddDate(0, 0, 1)
		case "mes":
			cursor = cursor.AddDate(0, 1, 0)
		case "trimestre":
			cursor = cursor.AddDate(0, 3, 0)
		case "ano":
			cursor = cursor.AddDate(1, 0, 0)
		}
	}
	return items, nil
}

func bucketKey(date time.Time, granularity string) string {
	switch granularity {
	case "dia":
		return date.Format("2006-01-02")
	case "mes":
		return date.Format("2006-01")
	case "trimestre":
		return fmt.Sprintf("%d-Q%d", date.Year(), (int(date.Month())-1)/3+1)
	case "ano":
		return date.Format("2006")
	default:
		return date.Format("2006-01")
	}
}

// ValidateAnalyticsFilter checks a chart filter without touching the database.
func ValidateAnalyticsFilter(filter AnalyticsFilter) error {
	return validateAnalyticsFilter(filter)
}

func validateAnalyticsFilter(filter AnalyticsFilter) error {
	switch filter.Eixo {
	case "tempo", "imovel", "categoria":
	default:
		return errors.New("eixo inválido")
	}
	switch filter.Series {
	case "tipo", "imovel", "categoria":
	default:
		return errors.New("série inválida")
	}
	switch filter.Metrica {
	case "receitas", "despesas", "resultado", "ambos", "investimentos":
	default:
		return errors.New("métrica inválida")
	}
	switch filter.Granularidade {
	case "dia", "mes", "trimestre", "ano":
	default:
		return errors.New("granularidade inválida")
	}
	if filter.Metrica == "ambos" && filter.Series != "tipo" {
		return errors.New("a métrica 'receitas × despesas' exige séries por tipo")
	}
	if filter.Eixo != "tempo" && filter.Eixo == filter.Series {
		return errors.New("eixo e séries não podem usar a mesma dimensão")
	}
	if filter.Inicio.IsZero() || filter.Fim.IsZero() {
		return errors.New("período inválido")
	}
	if filter.Inicio.After(filter.Fim) {
		return errors.New("data inicial posterior à final")
	}
	if filter.Fim.Sub(filter.Inicio) > 50*366*24*time.Hour {
		return errors.New("período máximo de 50 anos")
	}
	return nil
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

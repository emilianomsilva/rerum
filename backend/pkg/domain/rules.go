package domain

import (
	"fmt"
	"math"
	"time"
)

func ParseDate(value string) (time.Time, error) { return time.Parse("2006-01-02", value) }

func CalcEncargos(vencimento, hoje time.Time, baseC int64, multaPct, jurosDiaPct float64) (int64, int64, int) {
	dias := int(hoje.Sub(vencimento).Hours() / 24)
	if dias <= 0 || baseC <= 0 {
		return 0, 0, 0
	}
	multa := int64(math.Round(float64(baseC) * multaPct / 100))
	juros := int64(math.Round(float64(baseC) * jurosDiaPct * float64(dias) / 100))
	return multa, juros, dias
}

func ROI(investimentoC, retornoC int64) float64 {
	if investimentoC <= 0 {
		return 0
	}
	return math.Round(float64(retornoC)*10000/float64(investimentoC)) / 100
}

func GenerateInstallments(loc Locacao) ([]Parcela, error) {
	inicio, err := ParseDate(loc.Inicio)
	if err != nil {
		return nil, fmt.Errorf("início inválido: %w", err)
	}
	fim, err := ParseDate(loc.Fim)
	if err != nil {
		return nil, fmt.Errorf("fim inválido: %w", err)
	}
	if inicio.After(fim) {
		return nil, fmt.Errorf("início posterior ao fim do contrato")
	}
	if loc.DiaVencimento < 1 || loc.DiaVencimento > 31 {
		return nil, fmt.Errorf("dia de vencimento deve ser 1..31")
	}
	if loc.ValorMensalC <= 0 {
		return nil, fmt.Errorf("valor mensal deve ser maior que zero")
	}

	months := (fim.Year()-inicio.Year())*12 + int(fim.Month()-inicio.Month())
	if fim.Day() > inicio.Day() {
		months++
	}
	total := months
	if total < 1 {
		total = 1
	}

	var firstDue time.Time
	if loc.PrimeiroVencimento != nil && *loc.PrimeiroVencimento != "" {
		fd, err := ParseDate(*loc.PrimeiroVencimento)
		if err != nil {
			return nil, fmt.Errorf("primeiro vencimento inválido: %w", err)
		}
		firstDue = fd
	} else {
		// Se dia do vencimento coincide com o dia de início, o vencimento é na própria data de início
		if loc.DiaVencimento == inicio.Day() {
			firstDue = inicio
		} else if loc.DiaVencimento > inicio.Day() {
			// Vencimento no mesmo mês de início
			maxDay := lastDayOfMonth(inicio.Year(), int(inicio.Month()))
			firstDue = time.Date(inicio.Year(), inicio.Month(), min(loc.DiaVencimento, maxDay), 0, 0, 0, 0, time.UTC)
		} else {
			// Vencimento no mês seguinte ao início
			nextMonth := inicio.AddDate(0, 1, 0)
			maxDay := lastDayOfMonth(nextMonth.Year(), int(nextMonth.Month()))
			firstDue = time.Date(nextMonth.Year(), nextMonth.Month(), min(loc.DiaVencimento, maxDay), 0, 0, 0, 0, time.UTC)
		}
	}

	parcelas := make([]Parcela, 0, total)
	for k := 0; k < total; k++ {
		start := addMonthsClamped(inicio, k)
		nextStart := addMonthsClamped(inicio, k+1)
		end := nextStart.AddDate(0, 0, -1)
		if k == total-1 && end.Before(fim) {
			end = fim
		}

		due := addMonthsDayClamped(firstDue, k, loc.DiaVencimento)

		startStr := start.Format("2006-01-02")
		endStr := end.Format("2006-01-02")
		dueStr := due.Format("2006-01-02")
		kind := "mensal"
		if k == 0 {
			kind = "inicial"
		}

		parcelas = append(parcelas, Parcela{
			LocacaoID:     loc.ID,
			Competencia:   start.Format("2006-01"),
			Vencimento:    dueStr,
			ValorBaseC:    loc.ValorMensalC,
			Status:        "aberta",
			Numero:        k + 1,
			TotalParcelas: total,
			PeriodoInicio: &startStr,
			PeriodoFim:    &endStr,
			Tipo:          kind,
		})
	}

	return parcelas, nil
}

func lastDayOfMonth(year, month int) int {
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

func addMonthsClamped(t time.Time, months int) time.Time {
	targetYear := t.Year() + (int(t.Month())-1+months)/12
	targetMonth := ((int(t.Month())-1+months)%12+12)%12 + 1
	maxDay := lastDayOfMonth(targetYear, targetMonth)
	day := t.Day()
	if day > maxDay {
		day = maxDay
	}
	return time.Date(targetYear, time.Month(targetMonth), day, 0, 0, 0, 0, time.UTC)
}

func addMonthsDayClamped(base time.Time, monthsOffset, targetDay int) time.Time {
	targetYear := base.Year() + (int(base.Month())-1+monthsOffset)/12
	targetMonth := ((int(base.Month())-1+monthsOffset)%12+12)%12 + 1
	maxDay := lastDayOfMonth(targetYear, targetMonth)
	day := targetDay
	if day <= 0 || day > 31 {
		day = base.Day()
	}
	if day > maxDay {
		day = maxDay
	}
	return time.Date(targetYear, time.Month(targetMonth), day, 0, 0, 0, 0, time.UTC)
}

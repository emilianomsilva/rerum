package domain

import (
	"testing"
	"time"
)

func TestGenerateInstallmentsAndFees(t *testing.T) {
	items, err := GenerateInstallments(Locacao{Inicio: "2026-01-05", Fim: "2026-04-05", DiaVencimento: 5, ValorMensalC: 150000})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 periods spanning Jan-Apr, got %d", len(items))
	}
	if *items[0].PeriodoInicio != "2026-01-05" || *items[0].PeriodoFim != "2026-02-04" {
		t.Fatalf("unexpected first period: %#v", items[0])
	}
	if items[0].TotalParcelas != 3 {
		t.Fatalf("unexpected installment count %d", items[0].TotalParcelas)
	}
	multa, juros, days := CalcEncargos(mustDate(t, "2026-02-05"), mustDate(t, "2026-02-15"), 150000, 2, 0.033)
	if multa != 3000 || juros != 495 || days != 10 {
		t.Fatalf("unexpected fees: %d %d %d", multa, juros, days)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := ParseDate(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

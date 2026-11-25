package importer

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"rerum/backend/pkg/migrations"

	_ "github.com/mattn/go-sqlite3"
	"github.com/xuri/excelize/v2"
)

func testDatabase(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "import-test.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Apply(context.Background(), db, ""); err != nil {
		t.Fatal(err)
	}
	return db
}

func syntheticWorkbook(t *testing.T) string {
	t.Helper()
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	if err := file.SetSheetName("Sheet1", "Estatísticas"); err != nil {
		t.Fatal(err)
	}
	for _, sheet := range []string{"Aluguéis", "Matrículas", "Jerivá 575", "Portugal 620-650", "Indaiá 643", "Jerivá 587", "Débitos"} {
		if _, err := file.NewSheet(sheet); err != nil {
			t.Fatal(err)
		}
	}
	setRow := func(sheet, cell string, values ...any) {
		t.Helper()
		if err := file.SetSheetRow(sheet, cell, &values); err != nil {
			t.Fatal(err)
		}
	}
	setRow("Estatísticas", "A1", "Imóvel", "Custo", "Rendimento")
	setRow("Estatísticas", "A2", "Casa sintética", "10000", "20000")
	setRow("Aluguéis", "A1", "Data", "Status", "Origem", "Imóvel", "Inquilino", "Referência", "Valor", "Categoria")
	setRow("Aluguéis", "A2", "2026-01-05", "Pago", "Pix", "Casa sintética", "Pessoa teste", "2026-01-01", "1500.00", "aluguel")
	setRow("Matrículas", "A1", "Status", "Bairro", "Tipo", "Endereço", "Matrícula")
	for _, sheet := range []string{"Jerivá 575", "Portugal 620-650", "Indaiá 643", "Jerivá 587"} {
		setRow(sheet, "A1", "Data", "Categoria", "Forma", "Descrição", "Valor", "Fornecedor")
	}
	setRow("Jerivá 575", "A2", "2026-01-10", "reforma", "pix", "Custo sintético", "10.00", "Fornecedor teste")
	setRow("Débitos", "A1", "Data", "Descrição", "Liberado", "Pago", "Credor")
	setRow("Débitos", "A2", "2026-01-10", "Dívida de teste", "100.00", "25.00", "Credor teste")
	path := filepath.Join(t.TempDir(), "fixture-synthetic.xlsx")
	if err := file.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPreviewWorkbookReconcilesSyntheticFixture(t *testing.T) {
	db := testDatabase(t)
	preview, err := (Workbook{DB: db}).Preview(syntheticWorkbook(t))
	if err != nil {
		t.Fatal(err)
	}
	if preview.Properties != 1 || preview.Receipts != 1 || preview.ReceiptsTotalC != 150_000 || preview.Expenses != 1 || preview.ExpensesTotalC != 1_000 || preview.Debts != 1 || preview.DebtReceivedC != 10_000 || preview.DebtPaidC != 2_500 || preview.FormulaRows != 0 {
		t.Fatalf("unexpected synthetic source reconciliation: %+v", preview)
	}
}

func TestImportSyntheticWorkbookIsTransactionalAndIdempotent(t *testing.T) {
	db := testDatabase(t)
	workbook := Workbook{DB: db}
	path := syntheticWorkbook(t)
	preview, backup, err := workbook.Import(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Receipts != 1 || backup == "" {
		t.Fatalf("missing import or backup result: %+v %q", preview, backup)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("expected pre-import backup at %q: %v", backup, err)
	}
	var receiptCount, receiptTotal, expenseCount, expenseTotal, debtCount, batchCount int64
	if err := db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(valor_c),0) FROM movimentacoes WHERE tipo='receita'`).Scan(&receiptCount, &receiptTotal); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(valor_c),0) FROM movimentacoes WHERE tipo='despesa'`).Scan(&expenseCount, &expenseTotal); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM dividas`).Scan(&debtCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM import_batches`).Scan(&batchCount); err != nil {
		t.Fatal(err)
	}
	if receiptCount != 1 || receiptTotal != 150_000 || expenseCount != 1 || expenseTotal != 1_000 || debtCount != 1 || batchCount != 1 {
		t.Fatalf("unexpected imported totals: receipts=%d/%d expenses=%d/%d debts=%d batches=%d", receiptCount, receiptTotal, expenseCount, expenseTotal, debtCount, batchCount)
	}
	if _, _, err := workbook.Import(context.Background(), path); err == nil {
		t.Fatal("expected duplicate import to be rejected")
	}
}

func TestOptionalLocalWorkbookPreview(t *testing.T) {
	path := os.Getenv("RERUM_TEST_WORKBOOK")
	if path == "" {
		t.Skip("set RERUM_TEST_WORKBOOK to validate a local workbook explicitly")
	}
	db := testDatabase(t)
	preview, err := (Workbook{DB: db}).Preview(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Sheets) != len(requiredSheets) {
		t.Fatalf("expected %d sheets, got %d", len(requiredSheets), len(preview.Sheets))
	}
}

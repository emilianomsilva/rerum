package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"rerum/backend/pkg/domain"
	"rerum/backend/pkg/storage/sqlite"
)

func paymentFixture(t *testing.T) (*sqlite.Store, http.Handler) {
	t.Helper()
	db := testDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa de teste')`,
		`INSERT INTO inquilinos(nome,cpf,telefone) VALUES('Pessoa de teste','CPF original','Telefone original')`,
		`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c,multa_pct,juros_dia_pct) VALUES(1,1,'2026-01-05','2026-04-05',5,100000,2,0.1)`,
		`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,numero,total_parcelas) VALUES(1,'2026-01','2026-01-05',100000,1,3),(1,'2026-02','2026-02-05',100000,2,3),(1,'2026-03','2026-03-05',100000,3,3)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	store := &sqlite.Store{DB: db, ReceiptsDir: filepath.Join(t.TempDir(), "receipts")}
	return store, (&Server{Store: store}).Handler()
}

func paymentRequest(handler http.Handler, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return response
}

func TestBatchPaymentUsesEachDateAmountAndManualCharges(t *testing.T) {
	store, handler := paymentFixture(t)
	untouched, err := store.Installment(t.Context(), 3)
	if err != nil {
		t.Fatal(err)
	}
	response := paymentRequest(handler, "/api/parcelas/quitar-lote", `{"pagamentos":[
		{"parcela_id":1,"data":"2026-01-10","forma":"dinheiro","valor_pago_c":100579,"multa_c":123,"juros_c":456},
		{"parcela_id":2,"data":"2026-02-03","forma":"pix","valor_pago_c":50000,"multa_c":0,"juros_c":0}
	]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("batch payment: %d %s", response.Code, response.Body.String())
	}
	var saved []domain.Parcela
	if err := json.Unmarshal(response.Body.Bytes(), &saved); err != nil || len(saved) != 2 {
		t.Fatalf("invalid batch response: %v %s", err, response.Body.String())
	}
	if saved[0].ID != 1 || saved[0].Status != "paga" || saved[0].MultaC != 123 || saved[0].JurosC != 456 || saved[0].ValorPagoC != 100579 || *saved[0].DataPagamento != "2026-01-10" || *saved[0].Forma != "dinheiro" {
		t.Fatalf("wrong first payment: %+v", saved[0])
	}
	if saved[1].ID != 2 || saved[1].Status != "parcial" || saved[1].MultaC != 0 || saved[1].JurosC != 0 || saved[1].ValorPagoC != 50000 || *saved[1].DataPagamento != "2026-02-03" {
		t.Fatalf("wrong second payment: %+v", saved[1])
	}
	if saved[0].ReciboPath == nil || saved[1].ReciboPath != nil {
		t.Fatalf("receipts should be generated only for settled installments: %+v", saved)
	}
	content, err := os.ReadFile(*saved[0].ReciboPath)
	if err != nil || !strings.HasPrefix(string(content), "%PDF-1.4") {
		t.Fatalf("missing receipt: %v", err)
	}
	for _, item := range saved {
		var when, form string
		var amount int64
		if err := store.DB.QueryRow(`SELECT data,valor_c,forma_pagamento FROM movimentacoes WHERE parcela_id=?`, item.ID).Scan(&when, &amount, &form); err != nil {
			t.Fatal(err)
		}
		if when != *item.DataPagamento || amount != item.ValorPagoC || form != *item.Forma {
			t.Fatalf("wrong ledger entry for %d: %s %d %s", item.ID, when, amount, form)
		}
	}
	after, err := store.Installment(t.Context(), 3)
	if err != nil || !reflect.DeepEqual(after, untouched) {
		t.Fatalf("unselected installment changed: %+v %v", after, err)
	}
	var cpf, phone string
	if err := store.DB.QueryRow(`SELECT cpf,telefone FROM inquilinos WHERE id=1`).Scan(&cpf, &phone); err != nil || cpf != "CPF original" || phone != "Telefone original" {
		t.Fatalf("tenant data changed: %s %s %v", cpf, phone, err)
	}
}

func TestManualChargesCanBeWaivedAndOmittedChargesStillUseContract(t *testing.T) {
	store, handler := paymentFixture(t)
	if _, err := store.DB.Exec(`UPDATE parcelas SET multa_c=3000,juros_c=1000 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path, body           string
		fee, interest, total int64
	}{
		{"/api/parcelas/1/quitar", `{"data":"2026-01-10","multa_c":0,"juros_c":0,"valor_pago_c":100000}`, 0, 0, 100000},
		{"/api/parcelas/2/quitar", `{"data":"2026-02-10"}`, 2000, 500, 102500},
	} {
		response := paymentRequest(handler, test.path, test.body)
		var item domain.Parcela
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &item) != nil {
			t.Fatalf("single payment: %d %s", response.Code, response.Body.String())
		}
		if item.MultaC != test.fee || item.JurosC != test.interest || item.ValorPagoC != test.total || item.Status != "paga" {
			t.Fatalf("charges not respected: %+v", item)
		}
	}
}

func TestBatchPaymentRollsBackEveryInstallmentOnInvalidRow(t *testing.T) {
	for name, invalid := range map[string]string{
		"zero payment":         `{"parcela_id":2,"data":"2026-02-05","valor_pago_c":0}`,
		"negative payment":     `{"parcela_id":2,"data":"2026-02-05","valor_pago_c":-1}`,
		"negative fee":         `{"parcela_id":2,"data":"2026-02-05","valor_pago_c":100000,"multa_c":-1}`,
		"negative interest":    `{"parcela_id":2,"data":"2026-02-05","valor_pago_c":100000,"juros_c":-1}`,
		"no remaining balance": `{"parcela_id":2,"data":"2026-02-05","valor_pago_c":100000,"desconto_c":100000}`,
		"bad date":             `{"parcela_id":2,"data":"2026-02-30","valor_pago_c":100000}`,
		"missing date":         `{"parcela_id":2,"valor_pago_c":100000}`,
		"missing amount":       `{"parcela_id":2,"data":"2026-02-05"}`,
		"duplicate":            `{"parcela_id":1,"data":"2026-01-05","valor_pago_c":100000}`,
		"missing installment":  `{"parcela_id":9999,"data":"2026-02-05","valor_pago_c":100000}`,
	} {
		t.Run(name, func(t *testing.T) {
			store, handler := paymentFixture(t)
			before, _ := store.Installment(t.Context(), 1)
			response := paymentRequest(handler, "/api/parcelas/quitar-lote", `{"pagamentos":[{"parcela_id":1,"data":"2026-01-05","valor_pago_c":100000},`+invalid+`]}`)
			if response.Code != http.StatusUnprocessableEntity && response.Code != http.StatusNotFound {
				t.Fatalf("expected validation error, got %d %s", response.Code, response.Body.String())
			}
			after, _ := store.Installment(t.Context(), 1)
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("first payment was not rolled back: %+v", after)
			}
			assertNoBatchPaymentSideEffects(t, store)
		})
	}
}

func assertNoBatchPaymentSideEffects(t *testing.T, store *sqlite.Store) {
	t.Helper()
	var count int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM movimentacoes`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("ledger was modified: %d %v", count, err)
	}
	entries, err := os.ReadDir(store.ReceiptsDir)
	if err != nil && !os.IsNotExist(err) || len(entries) != 0 {
		t.Fatalf("receipt was created for failed batch: %v %v", entries, err)
	}
}

func TestBatchPaymentRollsBackOnLedgerFailure(t *testing.T) {
	store, handler := paymentFixture(t)
	if _, err := store.DB.Exec(`CREATE TRIGGER fail_batch BEFORE INSERT ON movimentacoes WHEN NEW.parcela_id=2 BEGIN SELECT RAISE(ABORT,'forced ledger failure'); END`); err != nil {
		t.Fatal(err)
	}
	response := paymentRequest(handler, "/api/parcelas/quitar-lote", `{"pagamentos":[{"parcela_id":1,"data":"2026-01-05","valor_pago_c":100000},{"parcela_id":2,"data":"2026-02-05","valor_pago_c":100000}]}`)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected failed transaction: %d %s", response.Code, response.Body.String())
	}
	for id := int64(1); id <= 2; id++ {
		item, err := store.Installment(t.Context(), id)
		if err != nil || item.ValorPagoC != 0 || item.Status != "aberta" || item.DataPagamento != nil {
			t.Fatalf("payment was not rolled back: %+v %v", item, err)
		}
	}
	assertNoBatchPaymentSideEffects(t, store)
}

func TestBatchPreservesExistingPartialPaymentAndAdjustment(t *testing.T) {
	store, handler := paymentFixture(t)
	if _, err := store.DB.Exec(`UPDATE parcelas SET valor_pago_c=20000,desconto_c=500,status='parcial' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	response := paymentRequest(handler, "/api/parcelas/quitar-lote", `{"pagamentos":[{"parcela_id":1,"data":"2026-01-05","valor_pago_c":79500,"multa_c":0,"juros_c":0}]}`)
	if response.Code != http.StatusOK {
		t.Fatalf("partial payment: %d %s", response.Code, response.Body.String())
	}
	item, err := store.Installment(t.Context(), 1)
	if err != nil || item.ValorPagoC != 99500 || item.DescontoC != 500 || item.Status != "paga" {
		t.Fatalf("existing values were overwritten: %+v %v", item, err)
	}
	var amount int64
	if err := store.DB.QueryRow(`SELECT valor_c FROM movimentacoes WHERE parcela_id=1`).Scan(&amount); err != nil || amount != 79500 {
		t.Fatalf("ledger should contain only the new receipt amount: %d %v", amount, err)
	}
	response = paymentRequest(handler, "/api/parcelas/quitar-lote", `{"pagamentos":[{"parcela_id":2,"data":"2026-02-05","valor_pago_c":100000},{"parcela_id":1,"data":"2026-01-05","valor_pago_c":79500}]}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("repeated confirmation should fail: %d %s", response.Code, response.Body.String())
	}
	other, _ := store.Installment(t.Context(), 2)
	if other.ValorPagoC != 0 || other.Status != "aberta" {
		t.Fatalf("repeated batch was not rolled back: %+v", other)
	}
}

func TestLegacyBatchPayloadStillWorksAndEmptyBatchFails(t *testing.T) {
	store, handler := paymentFixture(t)
	for _, body := range []string{`{}`, `{"pagamentos":[]}`, `{"ids":[]}`, `{"ids":[1],"pagamentos":[]}`} {
		response := paymentRequest(handler, "/api/parcelas/quitar-lote", body)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("empty/ambiguous batch accepted: %s %d %s", body, response.Code, response.Body.String())
		}
	}
	assertNoBatchPaymentSideEffects(t, store)
	response := paymentRequest(handler, "/api/parcelas/quitar-lote", `{"ids":[1,2],"data":"2026-01-05","forma":"pix"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("legacy payload failed: %d %s", response.Code, response.Body.String())
	}
	var count int
	if err := store.DB.QueryRow(`SELECT COUNT(*) FROM parcelas WHERE status='paga'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("legacy batch did not settle both installments: %d %v", count, err)
	}
}

package httpapi

import (
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"rerum/backend/pkg/domain"
)

func TestOpenInstallmentCRUDPreservesContractHistoryAndBackup(t *testing.T) {
	store, handler := paymentFixture(t)
	if _, err := store.DB.Exec(`UPDATE parcelas SET multa_c=1000,juros_c=200,desconto_c=500,periodo_inicio='2026-01-05',periodo_fim='2026-02-04' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	original, _ := store.Installment(t.Context(), 1)
	other, _ := store.Installment(t.Context(), 2)
	portfolioRequest(t, handler, "PATCH", "/api/parcelas/1", map[string]any{"vencimento": "2026-02-12", "valor_base_c": 120000, "multa_c": 0, "juros_c": 500, "observacao": "Conferido"}, 200)
	edited, _ := store.Installment(t.Context(), 1)
	if edited.Vencimento != "2026-02-12" || edited.ValorBaseC != 120000 || edited.MultaC != 0 || edited.JurosC != 500 || edited.DescontoC != 500 {
		t.Fatalf("wrong edited installment: %+v", edited)
	}
	if edited.Competencia != original.Competencia || edited.Numero != original.Numero || edited.TotalParcelas != original.TotalParcelas || !reflect.DeepEqual(edited.PeriodoInicio, original.PeriodoInicio) || !reflect.DeepEqual(edited.PeriodoFim, original.PeriodoFim) {
		t.Fatalf("contract history was rewritten: %+v", edited)
	}
	portfolioRequest(t, handler, "PATCH", "/api/parcelas/1", map[string]any{"observacao": "Só observação"}, 200)
	annotation, _ := store.Installment(t.Context(), 1)
	edited.Observacao = "Só observação"
	if !reflect.DeepEqual(edited, annotation) {
		t.Fatalf("partial PATCH reset unspecified fields: %+v", annotation)
	}
	after, _ := store.Installment(t.Context(), 2)
	if !reflect.DeepEqual(other, after) {
		t.Fatal("editing one installment modified another")
	}
	response := portfolioRequest(t, handler, "DELETE", "/api/parcelas/2", nil, 200)
	var deleted struct {
		BackupPath string `json:"backup_path"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &deleted); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(deleted.BackupPath)
	if err != nil || len(content) < 16 || string(content[:16]) != "SQLite format 3\x00" {
		t.Fatalf("backup missing: %s %v", deleted.BackupPath, err)
	}
	backup, err := sql.Open("sqlite3", "file:"+deleted.BackupPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var count int
	if err := backup.QueryRow(`SELECT COUNT(*) FROM parcelas WHERE id=2`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("deleted record not in backup: %d %v", count, err)
	}
	portfolioRequest(t, handler, "GET", "/api/parcelas/2", nil, 404)
	portfolioRequest(t, handler, "POST", "/api/parcelas/3/cancelar", nil, 200)
	canceled, _ := store.Installment(t.Context(), 3)
	if canceled.Status != "cancelada" || canceled.Numero != 3 || canceled.TotalParcelas != 3 {
		t.Fatalf("cancel should retain the original installment: %+v", canceled)
	}
	created := portfolioRequest(t, handler, "POST", "/api/parcelas", map[string]any{"locacao_id": 1, "vencimento": "2026-04-10", "valor_base_c": 50000, "multa_c": 100, "juros_c": 50, "desconto_c": 200, "tipo": "taxa"}, 201)
	var item domain.Parcela
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil || item.MultaC != 100 || item.JurosC != 50 || item.DescontoC != 200 || item.Numero != 0 || item.Status != "aberta" {
		t.Fatalf("wrong created installment: %+v %v", item, err)
	}
	lease, err := store.Lease(t.Context(), 1)
	if err != nil || lease.Fim != "2026-04-05" || lease.ValorMensalC != 100000 {
		t.Fatalf("contract changed: %+v %v", lease, err)
	}
}

func TestInstallmentCRUDRejectsReceivedOrLinkedRecords(t *testing.T) {
	for name, setup := range map[string]string{
		"partial":      `UPDATE parcelas SET status='parcial',valor_pago_c=20000 WHERE id=1`,
		"paid":         `UPDATE parcelas SET status='paga',valor_pago_c=100000 WHERE id=1`,
		"canceled":     `UPDATE parcelas SET status='cancelada' WHERE id=1`,
		"receipt":      `UPDATE parcelas SET recibo_path='historico.pdf' WHERE id=1`,
		"payment date": `UPDATE parcelas SET data_pagamento='2026-01-05' WHERE id=1`,
		"ledger":       `INSERT INTO movimentacoes(tipo,categoria,data,valor_c,parcela_id,imovel_id) VALUES('receita','aluguel','2026-01-05',100,1,1)`,
	} {
		t.Run(name, func(t *testing.T) {
			store, handler := paymentFixture(t)
			if _, err := store.DB.Exec(setup); err != nil {
				t.Fatal(err)
			}
			before, _ := store.Installment(t.Context(), 1)
			portfolioRequest(t, handler, "PATCH", "/api/parcelas/1", map[string]any{"valor_base_c": 50000}, 422)
			portfolioRequest(t, handler, "DELETE", "/api/parcelas/1", nil, 422)
			portfolioRequest(t, handler, "POST", "/api/parcelas/1/cancelar", nil, 422)
			after, _ := store.Installment(t.Context(), 1)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("protected history was modified")
			}
		})
	}
}

func TestInstallmentCRUDValidationDoesNotWrite(t *testing.T) {
	store, handler := paymentFixture(t)
	before, _ := store.Installment(t.Context(), 1)
	for _, input := range []map[string]any{
		{"valor_base_c": 0}, {"valor_base_c": -1}, {"multa_c": -1}, {"juros_c": -1}, {"vencimento": "2026-02-30"}, {"desconto_c": 100000}, {"tipo": ""},
	} {
		portfolioRequest(t, handler, "PATCH", "/api/parcelas/1", input, 422)
		after, _ := store.Installment(t.Context(), 1)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("invalid update wrote data: %+v", after)
		}
	}
	portfolioRequest(t, handler, "DELETE", "/api/parcelas/9999", nil, 404)
	portfolioRequest(t, handler, "POST", "/api/parcelas/9999/cancelar", nil, 404)
	portfolioRequest(t, handler, "POST", "/api/parcelas", map[string]any{"locacao_id": 1, "valor_base_c": 10000, "vencimento": "2026-01-05", "valor_pago_c": 1}, 422)
}

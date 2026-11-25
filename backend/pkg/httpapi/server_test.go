package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rerum/backend/pkg/domain"
	"rerum/backend/pkg/migrations"
	"rerum/backend/pkg/storage/sqlite"

	_ "github.com/mattn/go-sqlite3"
)

func TestPropertyCRUDHTTP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rerum.sqlite")
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := applyTestMigrations(t, db); err != nil {
		t.Fatal(err)
	}
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()

	create := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/imoveis", strings.NewReader(`{"apelido":"Casa teste","endereco":"Rua A","tipo":"casa","status":"disponivel","valor_compra_c":12000000,"reforma_c":10000,"itbi_c":0,"corretagem_c":0,"outros_custos_c":0,"ativo":true}`))
	handler.ServeHTTP(create, request)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", create.Code, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"investimento_total_c":12010000`) {
		t.Fatalf("investment total missing: %s", create.Body.String())
	}

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/imoveis", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "Casa teste") {
		t.Fatalf("unexpected list response %d: %s", list.Code, list.Body.String())
	}
	archive := httptest.NewRecorder()
	handler.ServeHTTP(archive, httptest.NewRequest(http.MethodDelete, "/api/imoveis/1", nil))
	if archive.Code != http.StatusNoContent {
		t.Fatalf("archive status %d: %s", archive.Code, archive.Body.String())
	}
	var active int
	if err := db.QueryRow(`SELECT ativo FROM imoveis WHERE id=1`).Scan(&active); err != nil || active != 0 {
		t.Fatalf("archive should retain the property and mark it inactive: active=%d err=%v", active, err)
	}

}

func TestDebtMovementsCanBeCreatedAndDeletedButNotUpdated(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO dividas(credor,descricao,valor_original_c,data_contratacao) VALUES('Banco teste','Crédito',100000,'2026-01-01')`); err != nil {
		t.Fatal(err)
	}
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()

	create := httptest.NewRecorder()
	handler.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/dividas/1/movimentos", strings.NewReader(`{"tipo":"pagamento","data":"2026-02-01","valor_c":10000}`)))
	if create.Code != http.StatusCreated {
		t.Fatalf("create movement status %d: %s", create.Code, create.Body.String())
	}
	var id int64
	if err := db.QueryRow(`SELECT id FROM movimentos_divida WHERE divida_id=1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/dividas/movimentos/%d", id)

	update := httptest.NewRecorder()
	handler.ServeHTTP(update, httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"tipo":"pagamento","data":"2026-02-01","valor_c":50000}`)))
	if update.Code != http.StatusMethodNotAllowed {
		t.Fatalf("update movement status %d: %s", update.Code, update.Body.String())
	}
	var amount int64
	if err := db.QueryRow(`SELECT valor_c FROM movimentos_divida WHERE id=?`, id).Scan(&amount); err != nil {
		t.Fatal(err)
	}
	if amount != 10000 {
		t.Fatalf("rejected update changed the movement amount to %d", amount)
	}

	remove := httptest.NewRecorder()
	handler.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, path, nil))
	if remove.Code != http.StatusNoContent {
		t.Fatalf("delete movement status %d: %s", remove.Code, remove.Body.String())
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM movimentos_divida WHERE id=?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("deleted movement is still present")
	}
}

func TestSQLiteBackupEndpoint(t *testing.T) {
	db := testDB(t)
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/backup/sqlite", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("backup status %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.HasPrefix(recorder.Body.String(), "SQLite format 3\x00") {
		t.Fatalf("invalid SQLite backup: %q", recorder.Body.String()[:min(16, recorder.Body.Len())])
	}
}

func TestLeaseGenerationAndPartialPayment(t *testing.T) {
	db := testDB(t)
	server := (&Server{Store: &sqlite.Store{DB: db, ReceiptsDir: filepath.Join(t.TempDir(), "receipts")}}).Handler()

	propertyResponse := httptest.NewRecorder()
	server.ServeHTTP(propertyResponse, httptest.NewRequest(http.MethodPost, "/api/imoveis", strings.NewReader(`{"apelido":"Casa A","tipo":"casa","status":"disponivel"}`)))
	if propertyResponse.Code != http.StatusCreated {
		t.Fatalf("property create %d %s", propertyResponse.Code, propertyResponse.Body.String())
	}
	tenantResponse := httptest.NewRecorder()
	server.ServeHTTP(tenantResponse, httptest.NewRequest(http.MethodPost, "/api/inquilinos", strings.NewReader(`{"nome":"Pessoa A"}`)))
	if tenantResponse.Code != http.StatusCreated {
		t.Fatalf("tenant create %d %s", tenantResponse.Code, tenantResponse.Body.String())
	}

	leaseResponse := httptest.NewRecorder()
	server.ServeHTTP(leaseResponse, httptest.NewRequest(http.MethodPost, "/api/locacoes", strings.NewReader(`{"imovel_id":1,"inquilino_id":1,"inicio":"2026-01-05","fim":"2026-04-05","dia_vencimento":5,"valor_mensal_c":150000,"multa_pct":2,"juros_dia_pct":0.033,"reajuste_indice":"nenhum"}`)))
	if leaseResponse.Code != http.StatusCreated {
		t.Fatalf("lease create %d %s", leaseResponse.Code, leaseResponse.Body.String())
	}

	rows, err := db.Query(`SELECT id FROM parcelas ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 installment rows, got %d", len(ids))
	}
	leaseList := httptest.NewRecorder()
	server.ServeHTTP(leaseList, httptest.NewRequest(http.MethodGet, "/api/locacoes", nil))
	for _, value := range []string{`"imovel":"Casa A"`, `"inquilino":"Pessoa A"`, `"parcelas":3`, `"periodo_inicio":"2026-01-05"`} {
		if !strings.Contains(leaseList.Body.String(), value) {
			t.Fatalf("lease summary missing %s: %s", value, leaseList.Body.String())
		}
	}

	partial := httptest.NewRecorder()
	server.ServeHTTP(partial, httptest.NewRequest(http.MethodPost, "/api/parcelas/1/quitar", strings.NewReader(`{"data":"2026-01-05","forma":"pix","valor_pago_c":120000}`)))
	if partial.Code != http.StatusOK {
		t.Fatalf("partial payment %d %s", partial.Code, partial.Body.String())
	}
	var paid int64
	var status string
	if err := db.QueryRow(`SELECT valor_pago_c,status FROM parcelas WHERE id=1`).Scan(&paid, &status); err != nil {
		t.Fatal(err)
	}
	if paid != 120000 || status != "parcial" {
		t.Fatalf("unexpected partial payment %d %s", paid, status)
	}
	installmentList := httptest.NewRecorder()
	server.ServeHTTP(installmentList, httptest.NewRequest(http.MethodGet, "/api/parcelas", nil))
	for _, value := range []string{`"imovel":"Casa A"`, `"inquilino":"Pessoa A"`, `"total_devido_c":150000`, `"saldo_c":30000`} {
		if !strings.Contains(installmentList.Body.String(), value) {
			t.Fatalf("installment summary missing %s: %s", value, installmentList.Body.String())
		}
	}
	installment := httptest.NewRecorder()
	server.ServeHTTP(installment, httptest.NewRequest(http.MethodGet, "/api/parcelas/1", nil))
	if installment.Code != http.StatusOK || !strings.Contains(installment.Body.String(), `"id":1`) {
		t.Fatalf("get installment failed %d: %s", installment.Code, installment.Body.String())
	}
	final := httptest.NewRecorder()
	server.ServeHTTP(final, httptest.NewRequest(http.MethodPost, "/api/parcelas/1/quitar", strings.NewReader(`{"data":"2026-01-05","forma":"pix","valor_pago_c":30000}`)))
	if final.Code != http.StatusOK {
		t.Fatalf("final payment %d %s", final.Code, final.Body.String())
	}
	var receiptPath string
	if err := db.QueryRow(`SELECT recibo_path FROM parcelas WHERE id=1`).Scan(&receiptPath); err != nil {
		t.Fatal(err)
	}
	if receiptPath == "" {
		t.Fatal("expected generated PDF receipt")
	}
	content, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "%PDF-1.4") {
		t.Fatalf("invalid receipt header %q", content[:min(16, len(content))])
	}
}

func TestCascadeDeletePropertyRemovesDependentsAndPreservesSharedMovement(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO imoveis(apelido) VALUES('Alvo'),('Outra')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO inquilinos(nome) VALUES('Pessoa')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO empreendimentos(nome) VALUES('Projeto')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-05','2026-12-31',5,100000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c) VALUES(1,'2026-01','2026-01-05',100000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO despesas(imovel_id,data,categoria,valor_c) VALUES(1,'2026-01-06','manutencao',5000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id) VALUES('despesa','manutencao','2026-01-06',5000,1)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,empreendimento_id) VALUES('despesa','obra','2026-01-07',10000,1)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO movimentacao_rateios(movimentacao_id,imovel_id,valor_c) VALUES(2,1,5000),(2,2,5000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,empreendimento_id,inquilino_id,locacao_id,parcela_id) VALUES('receita','aluguel','2026-01-05',100000,1,1,1,1)`)
	if err != nil {
		t.Fatal(err)
	}

	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/imoveis/1/cascade", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("cascade delete status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		BackupPath string `json:"backup_path"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.BackupPath == "" {
		t.Fatal("cascade delete did not return the automatic backup path")
	}
	if info, err := os.Stat(response.BackupPath); err != nil || info.Size() == 0 {
		t.Fatalf("automatic backup missing or empty: info=%v err=%v", info, err)
	}

	checks := map[string]int{
		`SELECT COUNT(*) FROM imoveis WHERE id=1`:                               0,
		`SELECT COUNT(*) FROM imoveis WHERE id=2`:                               1,
		`SELECT COUNT(*) FROM locacoes`:                                         0,
		`SELECT COUNT(*) FROM parcelas`:                                         0,
		`SELECT COUNT(*) FROM despesas`:                                         0,
		`SELECT COUNT(*) FROM movimentacoes WHERE imovel_id=1`:                  0,
		`SELECT COUNT(*) FROM movimentacoes WHERE locacao_id=1 OR parcela_id=1`: 0,
		`SELECT COUNT(*) FROM movimentacoes WHERE id=2`:                         1,
		`SELECT COUNT(*) FROM movimentacao_rateios WHERE imovel_id=1`:           0,
		`SELECT COUNT(*) FROM movimentacao_rateios WHERE imovel_id=2`:           1,
		`SELECT COUNT(*) FROM inquilinos WHERE id=1`:                            1,
	}
	for query, expected := range checks {
		var actual int
		if err := db.QueryRow(query).Scan(&actual); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		if actual != expected {
			t.Fatalf("query %q: got %d, want %d", query, actual, expected)
		}
	}
}

func TestCascadeDeleteTenantRemovesTenantHistory(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO imoveis(apelido) VALUES('Casa')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO inquilinos(nome) VALUES('Pessoa')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-05','2026-12-31',5,100000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c) VALUES(1,'2026-01','2026-01-05',100000)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id,inquilino_id,locacao_id,parcela_id) VALUES('receita','aluguel','2026-01-05',100000,1,1,1,1)`)
	if err != nil {
		t.Fatal(err)
	}

	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/api/inquilinos/1/cascade", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("cascade delete status %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		BackupPath string `json:"backup_path"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(response.BackupPath); err != nil || info.Size() == 0 {
		t.Fatalf("automatic backup missing or empty: info=%v err=%v", info, err)
	}
	for query, expected := range map[string]int{
		`SELECT COUNT(*) FROM inquilinos`:    0,
		`SELECT COUNT(*) FROM imoveis`:       1,
		`SELECT COUNT(*) FROM locacoes`:      0,
		`SELECT COUNT(*) FROM parcelas`:      0,
		`SELECT COUNT(*) FROM movimentacoes`: 0,
	} {
		var actual int
		if err := db.QueryRow(query).Scan(&actual); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
		if actual != expected {
			t.Fatalf("query %q: got %d, want %d", query, actual, expected)
		}
	}
}

func TestSignedDiscountAndSurchargeAdjustments(t *testing.T) {
	db := testDB(t)
	_, err := db.Exec(`INSERT INTO imoveis(apelido) VALUES('Casa')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO inquilinos(nome) VALUES('Pessoa')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c,multa_pct,juros_dia_pct) VALUES(1,1,'2026-01-05','2026-03-05',5,100000,0,0)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c) VALUES(1,'2026-01','2026-01-05',100000),(1,'2026-02','2026-02-05',100000)`)
	if err != nil {
		t.Fatal(err)
	}
	handler := (&Server{Store: &sqlite.Store{DB: db, ReceiptsDir: t.TempDir()}}).Handler()

	for _, test := range []struct {
		id       int
		date     string
		adjust   int64
		wantPaid int64
	}{
		{id: 1, date: "2026-01-05", adjust: 10000, wantPaid: 90000},
		{id: 2, date: "2026-02-05", adjust: -5000, wantPaid: 105000},
	} {
		body := fmt.Sprintf(`{"data":%q,"forma":"pix","desconto_c":%d}`, test.date, test.adjust)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/parcelas/%d/quitar", test.id), strings.NewReader(body)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("adjusted payment %d status %d: %s", test.id, recorder.Code, recorder.Body.String())
		}
		var paid, adjustment int64
		var status string
		if err := db.QueryRow(`SELECT valor_pago_c,desconto_c,status FROM parcelas WHERE id=?`, test.id).Scan(&paid, &adjustment, &status); err != nil {
			t.Fatal(err)
		}
		if paid != test.wantPaid || adjustment != test.adjust || status != "paga" {
			t.Fatalf("installment %d got paid=%d adjustment=%d status=%q", test.id, paid, adjustment, status)
		}
	}
}

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "rerum.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := applyTestMigrations(t, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func applyTestMigrations(t *testing.T, db *sql.DB) error {
	t.Helper()
	return migrations.Apply(t.Context(), db, "")
}

func TestNewFeaturesLifecycle(t *testing.T) {
	db := testDB(t)
	handler := (&Server{Store: &sqlite.Store{DB: db, ReceiptsDir: t.TempDir()}}).Handler()

	// 1. Property with new fields & reactivation
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/imoveis", strings.NewReader(`{"apelido":"Kitnet 101","endereco":"Rua Central, 10","tipo":"apartamento","status":"disponivel","valor_compra_c":8000000,"destinado_locacao":true}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create property failed: %s", rec.Body.String())
	}
	// Deactivate and Reactivate property
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/imoveis/1", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("deactivate property failed: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/imoveis/1/reativar", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("reactivate property failed: %s", rec.Body.String())
	}

	// 2. Tenant creation & reactivation
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inquilinos", strings.NewReader(`{"nome":"Carlos Silva","cpf":"123.456.789-00","telefone":"11999998888"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tenant failed: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/inquilinos/1", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("deactivate tenant failed: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/inquilinos/1/reativar", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("reactivate tenant failed: %s", rec.Body.String())
	}

	// 3. Lease creation, update, extra installment, payment, reopen, renewal, delete
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/locacoes", strings.NewReader(`{"imovel_id":1,"inquilino_id":1,"inicio":"2026-01-01","fim":"2026-03-31","dia_vencimento":10,"valor_mensal_c":150000,"primeiro_vencimento":"2026-01-10","intermediario":"Imobiliária X"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lease failed: %s", rec.Body.String())
	}

	// Update lease intermediary
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/locacoes/1", strings.NewReader(`{"intermediario":"Contrato direto","observacao":"Sem fiador"}`)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Sem fiador") {
		t.Fatalf("update lease failed: %s", rec.Body.String())
	}

	// Create extra installment (e.g. caução/taxa)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/parcelas", strings.NewReader(`{"locacao_id":1,"vencimento":"2026-01-05","valor_base_c":30000,"tipo":"taxa","observacao":"Taxa de pintura"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create extra installment failed: %s", rec.Body.String())
	}

	// Update installment 1
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/parcelas/1", strings.NewReader(`{"vencimento":"2026-01-12","valor_base_c":155000,"observacao":"Com reajuste"}`)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Com reajuste") {
		t.Fatalf("update installment failed: %s", rec.Body.String())
	}

	// Pay installment 1
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/parcelas/1/quitar", strings.NewReader(`{"data":"2026-01-12","forma":"pix"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("pay installment failed: %s", rec.Body.String())
	}

	// Verify movement was created
	var countMov int
	_ = db.QueryRow(`SELECT COUNT(*) FROM movimentacoes WHERE parcela_id=1`).Scan(&countMov)
	if countMov != 1 {
		t.Fatalf("expected 1 movement for paid installment, got %d", countMov)
	}

	// Reopen installment 1
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/parcelas/1/estornar", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen installment failed: %s", rec.Body.String())
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM movimentacoes WHERE parcela_id=1`).Scan(&countMov)
	if countMov != 0 {
		t.Fatalf("expected 0 movements after reopening installment, got %d", countMov)
	}

	// Renew lease
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/locacoes/1/renovar", strings.NewReader(`{"novo_fim":"2026-06-30","novo_valor_mensal_c":160000}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("renew lease failed: %s", rec.Body.String())
	}

	// 4. Empreendimentos and Automatic Rateio
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/empreendimentos", strings.NewReader(`{"nome":"Residencial Sol","endereco":"Rua B","unidade_ids":[1]}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project failed: %s", rec.Body.String())
	}

	// Create investment movement for the project with automatic allocation
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/movimentacoes", strings.NewReader(`{"movimento":{"tipo":"despesa","categoria":"obra","data":"2026-01-15","valor_c":50000,"descricao":"Cimento","empreendimento_id":1,"natureza":"investimento"}}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create movement failed: %s", rec.Body.String())
	}
	var createdMov domain.Movimento
	_ = json.NewDecoder(rec.Body).Decode(&createdMov)

	// Check automatic rateio
	var rateioCount int
	var rateioVal int64
	_ = db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(valor_c), 0) FROM movimentacao_rateios WHERE imovel_id=1`).Scan(&rateioCount, &rateioVal)
	if rateioCount != 1 || rateioVal != 50000 {
		t.Fatalf("expected automatic rateio of 50000, got count=%d val=%d", rateioCount, rateioVal)
	}

	// Update movement
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/movimentacoes/%d", createdMov.ID), strings.NewReader(`{"movimento":{"tipo":"despesa","categoria":"obra","data":"2026-01-15","valor_c":60000,"descricao":"Cimento e Areia","empreendimento_id":1,"natureza":"investimento"}}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("update movement failed: %s", rec.Body.String())
	}
	_ = db.QueryRow(`SELECT COALESCE(SUM(valor_c), 0) FROM movimentacao_rateios WHERE imovel_id=1`).Scan(&rateioVal)
	if rateioVal != 60000 {
		t.Fatalf("expected updated rateio 60000, got %d", rateioVal)
	}

	// Delete movement
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/movimentacoes/%d", createdMov.ID), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete movement failed: %s", rec.Body.String())
	}

	// 5. Debts with Juros
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/dividas", strings.NewReader(`{"credor":"Banco Caixa","valor_original_c":500000,"data_contratacao":"2026-01-01"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create debt failed: %s", rec.Body.String())
	}

	// Add debt payment with interest
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/dividas/1/movimentos", strings.NewReader(`{"tipo":"pagamento","data":"2026-02-01","valor_c":60000,"juros_c":10000,"forma_pagamento":"pix"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("add debt payment failed: %s", rec.Body.String())
	}

	// Check balance: 500000 - (60000 - 10000) = 450000
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dividas", nil))
	if !strings.Contains(rec.Body.String(), `"saldo_c":450000`) {
		t.Fatalf("debt balance with interest calculation failed: %s", rec.Body.String())
	}

	// 6. Annual Report
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/relatorios/anual?ano=2026", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ano":2026`) {
		t.Fatalf("annual report endpoint failed: %s", rec.Body.String())
	}
}

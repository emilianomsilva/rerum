package httpapi

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"rerum/backend/pkg/backup"
	"rerum/backend/pkg/domain"
	"rerum/backend/pkg/storage/sqlite"
)

func TestAnalyticsEndpointReturnsReadOnlySeries(t *testing.T) {
	db := testDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa endpoint')`,
		`INSERT INTO movimentacoes(tipo,categoria,data,valor_c,imovel_id) VALUES('receita','aluguel','2026-01-10',5000,1),('despesa','iptu','2026-02-10',1000,1)`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/analytics/series?inicio=2026-01-01&fim=2026-02-28&eixo=tempo&series=tipo&metrica=ambos&granularidade=mes", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("analytics status %d: %s", recorder.Code, recorder.Body.String())
	}
	var result domain.AnalyticsResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Categorias) != 2 || len(result.Series) != 2 {
		t.Fatalf("resposta analítica inesperada: %+v", result)
	}
	if result.Series[0].ValoresC[0] != 5000 || result.Series[1].ValoresC[1] != 1000 {
		t.Fatalf("valores inesperados: %+v", result.Series)
	}

	invalid := httptest.NewRecorder()
	handler.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/analytics/series?series=imovel&metrica=ambos", nil))
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("combinação inválida deveria retornar 422: %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestFullBackupEndpointIncludesReceiptsAndManifest(t *testing.T) {
	db := testDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa backup')`,
		`INSERT INTO inquilinos(nome) VALUES('Pessoa backup')`,
		`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-05','2026-02-05',5,100000)`,
		`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,numero,total_parcelas,recibo_path) VALUES(1,'2026-01','2026-01-05',100000,1,2,'/caminho/antigo/recibo_1.pdf')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	receipts := filepath.Join(t.TempDir(), "recibos")
	if err := os.MkdirAll(receipts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receipts, "recibo_1.pdf"), []byte("%PDF-1.4 backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := (&Server{Store: &sqlite.Store{DB: db, ReceiptsDir: receipts}}).Handler()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/backup/full", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("backup completo status %d: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.Len() < 4 || recorder.Body.String()[:2] != "PK" {
		t.Fatalf("resposta não é ZIP: %q", recorder.Body.String()[:min(8, recorder.Body.Len())])
	}
	archive := filepath.Join(t.TempDir(), "downloaded.zip")
	if err := os.WriteFile(archive, recorder.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	_ = reader.Close()
	for _, name := range []string{backup.DatabaseFile, backup.ManifestFile, backup.ReceiptsFolder + "/recibo_1.pdf"} {
		if !names[name] {
			t.Fatalf("backup completo sem %q: %v", name, names)
		}
	}

	restoredPath := filepath.Join(t.TempDir(), "data", "rerum.sqlite")
	restoredReceipts := filepath.Join(filepath.Dir(restoredPath), "recibos")
	if _, err := backup.Restore(t.Context(), archive, restoredPath, restoredReceipts, 10); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite3", restoredPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var property string
	if err := restored.QueryRow(`SELECT apelido FROM imoveis`).Scan(&property); err != nil {
		t.Fatal(err)
	}
	if property != "Casa backup" {
		t.Fatalf("backup restaurado inesperado: %q", property)
	}
}

func TestReceiptsFallBackToReceiptsDirBasename(t *testing.T) {
	db := testDB(t)
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa recibo')`,
		`INSERT INTO inquilinos(nome) VALUES('Pessoa recibo')`,
		`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-05','2026-02-05',5,100000)`,
		`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,valor_pago_c,data_pagamento,status,numero,total_parcelas,recibo_path) VALUES(1,'2026-01','2026-01-05',100000,100000,'2026-01-05','paga',1,2,'/caminho/que/nao/existe/recibo_1.pdf')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	receipts := filepath.Join(t.TempDir(), "recibos")
	if err := os.MkdirAll(receipts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receipts, "recibo_1.pdf"), []byte("%PDF-1.4 portátil"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := (&Server{Store: &sqlite.Store{DB: db, ReceiptsDir: receipts}}).Handler()

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/parcelas", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("listagem status %d: %s", list.Code, list.Body.String())
	}
	var items []domain.Parcela
	if err := json.Unmarshal(list.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].TemRecibo {
		t.Fatalf("recibo portátil não foi reconhecido: %+v", items)
	}

	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/recibos/1.pdf", nil))
	if download.Code != http.StatusOK {
		t.Fatalf("download status %d: %s", download.Code, download.Body.String())
	}
	if download.Body.String() != "%PDF-1.4 portátil" {
		t.Fatalf("conteúdo do recibo: %q", download.Body.String())
	}
}

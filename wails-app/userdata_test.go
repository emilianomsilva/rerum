package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestImportLegacyDataPreservesWALAndExistingDestination(t *testing.T) {
	source := t.TempDir()
	sourceDB := filepath.Join(source, databaseName)
	db, err := sql.Open("sqlite3", sourceDB+"?_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE fixture (value TEXT); INSERT INTO fixture VALUES ('legado')"); err != nil {
		t.Fatal(err)
	}
	receipts := filepath.Join(source, "recibos")
	if err := os.Mkdir(receipts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receipts, "recibo_1.pdf"), []byte("recibo"), 0o600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(t.TempDir(), "rerum")
	if err := importLegacyData(t.Context(), source, target); err != nil {
		t.Fatal(err)
	}
	if receipt, err := os.ReadFile(filepath.Join(target, "recibos", "recibo_1.pdf")); err != nil || string(receipt) != "recibo" {
		t.Fatalf("recibo importado = %q, erro = %v", receipt, err)
	}

	imported, err := sql.Open("sqlite3", filepath.Join(target, databaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer imported.Close()
	var value string
	if err := imported.QueryRow("SELECT value FROM fixture").Scan(&value); err != nil || value != "legado" {
		t.Fatalf("valor importado = %q, erro = %v", value, err)
	}
	if _, err := imported.Exec("UPDATE fixture SET value = 'atual'"); err != nil {
		t.Fatal(err)
	}
	if err := importLegacyData(t.Context(), source, target); err != nil {
		t.Fatal(err)
	}
	if err := imported.QueryRow("SELECT value FROM fixture").Scan(&value); err != nil || value != "atual" {
		t.Fatalf("valor após reinstalação = %q, erro = %v", value, err)
	}
	if err := db.QueryRow("SELECT value FROM fixture").Scan(&value); err != nil || value != "legado" {
		t.Fatalf("valor na origem = %q, erro = %v", value, err)
	}
}

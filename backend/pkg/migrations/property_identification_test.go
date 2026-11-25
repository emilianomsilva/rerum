package migrations_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"rerum/backend/pkg/migrations"
)

func TestPropertyIdentificationMigrationPreservesExistingData(t *testing.T) {
	dir := t.TempDir()
	source := "sql"
	files, err := filepath.Glob(filepath.Join(source, "*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	const next = "010_property_unit_identification.sql"
	copyMigration := func(name string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(source, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range files {
		if filepath.Base(path) < next {
			copyMigration(filepath.Base(path))
		}
	}
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "migration.sqlite")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := migrations.Apply(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO imoveis(apelido,endereco,matricula_cartorio,valor_compra_c) VALUES('Portugal 620-1','Endereço original, 620-1','10.001',12345)`); err != nil {
		t.Fatal(err)
	}
	copyMigration(next)
	for i := 0; i < 2; i++ {
		if err := migrations.Apply(ctx, db, dir); err != nil {
			t.Fatal(err)
		}
	}
	var alias, address, registry, number, unit string
	var cost int64
	if err := db.QueryRow(`SELECT apelido,endereco,matricula_cartorio,valor_compra_c,numero,unidade FROM imoveis WHERE id=1`).Scan(&alias, &address, &registry, &cost, &number, &unit); err != nil {
		t.Fatal(err)
	}
	if alias != "Portugal 620-1" || address != "Endereço original, 620-1" || registry != "10.001" || cost != 12345 || number != "" || unit != "" {
		t.Fatalf("migration modified original data: %s/%s/%s/%d/%s/%s", alias, address, registry, cost, number, unit)
	}
}

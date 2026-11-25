package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rerum/backend/pkg/migrations"

	_ "github.com/mattn/go-sqlite3"
)

func TestRoundTripIncludesDatabaseReceiptsAndManifest(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "rerum.sqlite")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, dbPath)
	defer db.Close()
	seed(t, db)
	receiptsDir := filepath.Join(root, "data", "recibos")
	if err := os.MkdirAll(receiptsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(receiptsDir, "recibo_1.pdf"), []byte("%PDF-1.4 teste"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "backup.zip")
	if err := Create(context.Background(), db, receiptsDir, archive); err != nil {
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
	for _, name := range []string{DatabaseFile, ManifestFile, ReadmeFile, ReceiptsFolder + "/recibo_1.pdf"} {
		if !names[name] {
			t.Fatalf("backup sem %q: %v", name, names)
		}
	}
	manifest := readManifest(t, archive)
	if manifest.Formato != FormatName || manifest.SchemaVersion != 10 {
		t.Fatalf("manifesto inesperado: %+v", manifest)
	}
	if manifest.Contagens["imoveis"] != 1 || len(manifest.Recibos) != 1 {
		t.Fatalf("contagens/recibos inesperados: %+v", manifest)
	}

	targetRoot := t.TempDir()
	targetDB := filepath.Join(targetRoot, "data", "rerum.sqlite")
	targetReceipts := filepath.Join(targetRoot, "data", "recibos")
	if err := os.MkdirAll(filepath.Dir(targetDB), 0o700); err != nil {
		t.Fatal(err)
	}
	oldDB := openTestDB(t, targetDB)
	if _, err := oldDB.Exec(`INSERT INTO imoveis(apelido) VALUES('Banco antigo')`); err != nil {
		t.Fatal(err)
	}
	_ = oldDB.Close()
	if err := os.MkdirAll(targetReceipts, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetReceipts, "antigo.pdf"), []byte("antigo"), 0o600); err != nil {
		t.Fatal(err)
	}

	safety, err := Restore(context.Background(), archive, targetDB, targetReceipts, 10)
	if err != nil {
		t.Fatal(err)
	}
	if safety == "" {
		t.Fatal("backup de segurança não foi criado")
	}
	if _, err := os.Stat(safety); err != nil {
		t.Fatalf("backup de segurança ausente: %v", err)
	}
	restored, err := sql.Open("sqlite3", targetDB+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var property string
	if err := restored.QueryRow(`SELECT apelido FROM imoveis`).Scan(&property); err != nil {
		t.Fatal(err)
	}
	if property != "Casa teste" {
		t.Fatalf("banco restaurado inesperado: %q", property)
	}
	content, err := os.ReadFile(filepath.Join(targetReceipts, "recibo_1.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "%PDF-1.4 teste" {
		t.Fatalf("recibo restaurado inesperado: %q", content)
	}
	if _, err := os.Stat(filepath.Join(targetReceipts, "antigo.pdf")); !os.IsNotExist(err) {
		t.Fatalf("recibo antigo deveria ter sido substituído: %v", err)
	}
}

func TestCreateIncludesCommittedWALData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "rerum.sqlite")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, dbPath)
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO imoveis(apelido) VALUES('Casa WAL')`); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(root, "backup.zip")
	if err := Create(context.Background(), db, "", archive); err != nil {
		t.Fatal(err)
	}
	restoredPath := filepath.Join(root, "restored.sqlite")
	if _, err := Restore(context.Background(), archive, restoredPath, filepath.Join(root, "recibos"), 10); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite3", restoredPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var count int
	if err := restored.QueryRow(`SELECT COUNT(*) FROM imoveis WHERE apelido='Casa WAL'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dado confirmado no WAL não entrou no backup: %d", count)
	}
}

func TestRestoreRejectsCorruptedBackupAndKeepsCurrentData(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "rerum.sqlite")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, dbPath)
	seed(t, db)
	archive := filepath.Join(root, "backup.zip")
	if err := Create(context.Background(), db, "", archive); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	corrupted := filepath.Join(root, "corrupted.zip")
	rewriteZip(t, archive, corrupted, func(name string, data []byte) []byte {
		if name == DatabaseFile {
			data[len(data)-1] ^= 0xff
		}
		return data
	})

	target := filepath.Join(t.TempDir(), "data", "rerum.sqlite")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	current := openTestDB(t, target)
	if _, err := current.Exec(`INSERT INTO imoveis(apelido) VALUES('Preservado')`); err != nil {
		t.Fatal(err)
	}
	_ = current.Close()
	if _, err := Restore(context.Background(), corrupted, target, filepath.Join(filepath.Dir(target), "recibos"), 10); err == nil {
		t.Fatal("backup corrompido deveria ser recusado")
	}
	check, err := sql.Open("sqlite3", target+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var property string
	if err := check.QueryRow(`SELECT apelido FROM imoveis`).Scan(&property); err != nil {
		t.Fatal(err)
	}
	if property != "Preservado" {
		t.Fatalf("banco atual foi alterado após falha: %q", property)
	}
	matches, _ := filepath.Glob(target + ".pre-restore-*.zip")
	if len(matches) != 0 {
		t.Fatalf("backup de segurança não deveria ser criado para arquivo inválido: %v", matches)
	}
}

func TestRestoreRejectsNewerSchema(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, "data", "rerum.sqlite")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		t.Fatal(err)
	}
	db := openTestDB(t, dbPath)
	seed(t, db)
	archive := filepath.Join(root, "backup.zip")
	if err := Create(context.Background(), db, "", archive); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	newer := filepath.Join(root, "newer.zip")
	rewriteZip(t, archive, newer, func(name string, data []byte) []byte {
		if name != ManifestFile {
			return data
		}
		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		manifest.SchemaVersion = 99
		updated, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		return updated
	})
	if _, err := Restore(context.Background(), newer, filepath.Join(t.TempDir(), "rerum.sqlite"), filepath.Join(t.TempDir(), "recibos"), 10); err == nil {
		t.Fatal("schema mais novo deveria ser recusado")
	}
}

func TestRestoreRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "evil.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("evil"))
	_ = writer.Close()
	_ = file.Close()
	if _, err := Restore(context.Background(), archive, filepath.Join(root, "rerum.sqlite"), filepath.Join(root, "recibos"), 10); err == nil {
		t.Fatal("caminho malicioso deveria ser recusado")
	}
}

func openTestDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(context.Background(), db, ""); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	return db
}

func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, query := range []string{
		`INSERT INTO imoveis(apelido) VALUES('Casa teste')`,
		`INSERT INTO inquilinos(nome) VALUES('Pessoa teste')`,
		`INSERT INTO locacoes(imovel_id,inquilino_id,inicio,fim,dia_vencimento,valor_mensal_c) VALUES(1,1,'2026-01-05','2026-12-05',5,100000)`,
		`INSERT INTO parcelas(locacao_id,competencia,vencimento,valor_base_c,numero,total_parcelas,recibo_path) VALUES(1,'2026-01','2026-01-05',100000,1,12,'/caminho/antigo/recibo_1.pdf')`,
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
}

func readManifest(t *testing.T, archive string) Manifest {
	t.Helper()
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name != ManifestFile {
			continue
		}
		content, err := readZipFile(file, 4<<20)
		if err != nil {
			t.Fatal(err)
		}
		var manifest Manifest
		if err := json.Unmarshal(content, &manifest); err != nil {
			t.Fatal(err)
		}
		return manifest
	}
	t.Fatal("manifesto ausente")
	return Manifest{}
}

func rewriteZip(t *testing.T, source, destination string, mutate func(string, []byte) []byte) {
	t.Helper()
	reader, err := zip.OpenReader(source)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	output, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	for _, file := range reader.File {
		input, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(input)
		_ = input.Close()
		if err != nil {
			t.Fatal(err)
		}
		data = mutate(file.Name, data)
		entry, err := writer.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestManifestReadmeMentionsRestore(t *testing.T) {
	if !strings.Contains(readme, "Restaurar backup completo") {
		t.Fatal("LEIA-ME não orienta a restauração")
	}
}

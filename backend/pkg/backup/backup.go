// Package backup creates and restores complete Rerum backups.
//
// A backup is a ZIP with a consistent SQLite snapshot (VACUUM INTO), the
// receipts directory and a manifest containing schema/version information and
// SHA-256 checksums. Restoring replaces the database and receipts; it never
// merges data.
package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	FormatName      = "rerum-backup"
	FormatVersion   = 1
	ManifestFile    = "manifest.json"
	DatabaseFile    = "rerum.sqlite"
	ReceiptsFolder  = "recibos"
	ReadmeFile      = "LEIA-ME.txt"
	maxDatabaseSize = int64(4 << 30)
	maxReceiptSize  = int64(512 << 20)
	maxTotalSize    = int64(8 << 30)
	maxArchiveFiles = 100_000
)

var ErrInvalidBackup = errors.New("backup inválido")

// Manifest describes the contents of a complete backup.
type Manifest struct {
	Formato       string           `json:"formato"`
	VersaoFormato int              `json:"versao_formato"`
	CriadoEm      string           `json:"criado_em"`
	Aplicativo    string           `json:"aplicativo"`
	SchemaVersion int              `json:"schema_version"`
	Banco         string           `json:"banco"`
	BancoBytes    int64            `json:"banco_bytes"`
	BancoSHA256   string           `json:"banco_sha256"`
	Recibos       []ReceiptEntry   `json:"recibos"`
	Contagens     map[string]int64 `json:"contagens,omitempty"`
}

// ReceiptEntry is one receipt stored inside the backup.
type ReceiptEntry struct {
	Nome   string `json:"nome"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type receiptFile struct {
	name string
	path string
	size int64
	hash string
}

// Create writes a complete backup of db and receiptsDir to destination.
// The SQLite snapshot is created with VACUUM INTO, so committed WAL data is
// included and the destination is a standalone database file.
func Create(ctx context.Context, db *sql.DB, receiptsDir, destination string) error {
	if db == nil {
		return errors.New("banco não informado")
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("destino do backup não informado")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("criar diretório do backup: %w", err)
	}
	tmpDir, err := os.MkdirTemp("", "rerum-backup-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	snapshot := filepath.Join(tmpDir, DatabaseFile)
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, filepath.Clean(snapshot)); err != nil {
		return fmt.Errorf("criar snapshot do banco: %w", err)
	}
	schema, counts, err := inspectDatabase(ctx, snapshot)
	if err != nil {
		return err
	}
	dbSize, dbHash, err := fileDigest(snapshot)
	if err != nil {
		return err
	}
	receipts, err := collectReceipts(receiptsDir)
	if err != nil {
		return err
	}
	manifest := Manifest{
		Formato:       FormatName,
		VersaoFormato: FormatVersion,
		CriadoEm:      time.Now().UTC().Format(time.RFC3339),
		Aplicativo:    "Rerum",
		SchemaVersion: schema,
		Banco:         DatabaseFile,
		BancoBytes:    dbSize,
		BancoSHA256:   dbHash,
		Recibos:       make([]ReceiptEntry, 0, len(receipts)),
		Contagens:     counts,
	}
	for _, receipt := range receipts {
		manifest.Recibos = append(manifest.Recibos, ReceiptEntry{
			Nome:   receipt.name,
			Bytes:  receipt.size,
			SHA256: receipt.hash,
		})
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestBytes = append(manifestBytes, '\n')

	tmpZip, err := os.CreateTemp(filepath.Dir(destination), ".rerum-backup-*.zip")
	if err != nil {
		return fmt.Errorf("criar arquivo de backup: %w", err)
	}
	tmpName := tmpZip.Name()
	defer os.Remove(tmpName)
	writer := zip.NewWriter(tmpZip)
	if err := addFile(writer, DatabaseFile, snapshot, zip.Deflate); err != nil {
		_ = writer.Close()
		_ = tmpZip.Close()
		return err
	}
	for _, receipt := range receipts {
		name := ReceiptsFolder + "/" + receipt.name
		if err := addFile(writer, name, receipt.path, zip.Deflate); err != nil {
			_ = writer.Close()
			_ = tmpZip.Close()
			return err
		}
	}
	if err := addBytes(writer, ManifestFile, manifestBytes, zip.Deflate); err != nil {
		_ = writer.Close()
		_ = tmpZip.Close()
		return err
	}
	if err := addBytes(writer, ReadmeFile, []byte(readme), zip.Deflate); err != nil {
		_ = writer.Close()
		_ = tmpZip.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		_ = tmpZip.Close()
		return err
	}
	if err := tmpZip.Sync(); err != nil {
		_ = tmpZip.Close()
		return err
	}
	if err := tmpZip.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		return fmt.Errorf("gravar backup: %w", err)
	}
	return nil
}

// Restore validates archivePath, creates a safety backup of the current data
// and replaces dbPath/receiptsDir with the backup contents. currentSchema is
// the highest migration version supported by the running application.
func Restore(ctx context.Context, archivePath, dbPath, receiptsDir string, currentSchema int) (string, error) {
	if strings.TrimSpace(archivePath) == "" {
		return "", errors.New("arquivo de backup não informado")
	}
	if strings.TrimSpace(dbPath) == "" {
		return "", errors.New("caminho do banco não informado")
	}
	if strings.TrimSpace(receiptsDir) == "" {
		return "", errors.New("caminho dos recibos não informado")
	}
	tmpDir, err := os.MkdirTemp("", "rerum-restore-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	manifest, err := extractArchive(archivePath, tmpDir)
	if err != nil {
		return "", err
	}
	if manifest.VersaoFormato > FormatVersion {
		return "", fmt.Errorf("%w: formato de backup %d mais novo que o suportado (%d)", ErrInvalidBackup, manifest.VersaoFormato, FormatVersion)
	}
	if manifest.SchemaVersion < 1 {
		return "", fmt.Errorf("%w: backup sem schema do Rerum", ErrInvalidBackup)
	}
	if currentSchema > 0 && manifest.SchemaVersion > currentSchema {
		return "", fmt.Errorf("%w: schema %d mais novo que o suportado (%d)", ErrInvalidBackup, manifest.SchemaVersion, currentSchema)
	}
	extractedDB := filepath.Join(tmpDir, DatabaseFile)
	if err := validateDatabase(ctx, extractedDB, manifest.SchemaVersion); err != nil {
		return "", err
	}

	safety, err := safetyBackup(ctx, dbPath, receiptsDir)
	if err != nil {
		return "", err
	}

	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		return safety, fmt.Errorf("criar diretório do banco: %w", err)
	}
	tmpDB, err := os.CreateTemp(dbDir, ".rerum-restore-*.sqlite")
	if err != nil {
		return safety, fmt.Errorf("preparar banco restaurado: %w", err)
	}
	tmpDBName := tmpDB.Name()
	defer os.Remove(tmpDBName)
	if err := copyFileToWriter(extractedDB, tmpDB); err != nil {
		_ = tmpDB.Close()
		return safety, fmt.Errorf("copiar banco restaurado: %w", err)
	}
	if err := tmpDB.Sync(); err != nil {
		_ = tmpDB.Close()
		return safety, err
	}
	if err := tmpDB.Close(); err != nil {
		return safety, err
	}
	if err := os.Chmod(tmpDBName, 0o600); err != nil {
		return safety, err
	}

	receiptsDir = filepath.Clean(receiptsDir)
	if err := os.MkdirAll(filepath.Dir(receiptsDir), 0o700); err != nil {
		return safety, fmt.Errorf("criar diretório de recibos: %w", err)
	}
	tmpReceipts, err := os.MkdirTemp(filepath.Dir(receiptsDir), ".rerum-recibos-*")
	if err != nil {
		return safety, err
	}
	defer os.RemoveAll(tmpReceipts)
	for _, receipt := range manifest.Recibos {
		source := filepath.Join(tmpDir, ReceiptsFolder, receipt.Nome)
		destination := filepath.Join(tmpReceipts, receipt.Nome)
		if err := copyFile(source, destination); err != nil {
			return safety, fmt.Errorf("preparar recibo %q: %w", receipt.Nome, err)
		}
	}

	backupReceipts := ""
	rollbackReceipts := func() {
		_ = os.RemoveAll(receiptsDir)
		if backupReceipts != "" {
			_ = os.Rename(backupReceipts, receiptsDir)
		}
	}
	if info, statErr := os.Stat(receiptsDir); statErr == nil && info.IsDir() {
		backupReceipts = receiptsDir + ".pre-restore-" + time.Now().Format("20060102-150405.000000000")
		if err := os.Rename(receiptsDir, backupReceipts); err != nil {
			return safety, fmt.Errorf("preservar recibos atuais: %w", err)
		}
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return safety, fmt.Errorf("inspecionar recibos atuais: %w", statErr)
	}
	if err := os.Rename(tmpReceipts, receiptsDir); err != nil {
		rollbackReceipts()
		return safety, fmt.Errorf("substituir recibos: %w", err)
	}

	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	if err := os.Rename(tmpDBName, dbPath); err != nil {
		rollbackReceipts()
		return safety, fmt.Errorf("substituir banco: %w", err)
	}
	return safety, nil
}

func safetyBackup(ctx context.Context, dbPath, receiptsDir string) (string, error) {
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("inspecionar banco atual: %w", err)
	}
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return "", fmt.Errorf("abrir banco atual para backup de segurança: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	destination := dbPath + ".pre-restore-" + time.Now().Format("20060102-150405.000000000") + ".zip"
	if err := Create(ctx, db, receiptsDir, destination); err != nil {
		return "", fmt.Errorf("criar backup de segurança antes da restauração: %w", err)
	}
	return destination, nil
}

func extractArchive(archivePath, destination string) (Manifest, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("%w: abrir ZIP: %v", ErrInvalidBackup, err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveFiles {
		return Manifest{}, fmt.Errorf("%w: arquivo ZIP com entradas demais", ErrInvalidBackup)
	}
	var manifestBytes []byte
	for _, file := range reader.File {
		if file.Name == ManifestFile {
			if manifestBytes != nil {
				return Manifest{}, fmt.Errorf("%w: manifesto duplicado", ErrInvalidBackup)
			}
			manifestBytes, err = readZipFile(file, 4<<20)
			if err != nil {
				return Manifest{}, fmt.Errorf("%w: ler manifesto: %v", ErrInvalidBackup, err)
			}
		}
	}
	if manifestBytes == nil {
		return Manifest{}, fmt.Errorf("%w: manifesto ausente", ErrInvalidBackup)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("%w: manifesto inválido: %v", ErrInvalidBackup, err)
	}
	if manifest.Formato != FormatName {
		return Manifest{}, fmt.Errorf("%w: formato %q desconhecido", ErrInvalidBackup, manifest.Formato)
	}
	if manifest.VersaoFormato < 1 || manifest.VersaoFormato > FormatVersion {
		return Manifest{}, fmt.Errorf("%w: versão de formato %d não suportada", ErrInvalidBackup, manifest.VersaoFormato)
	}
	if manifest.Banco != DatabaseFile {
		return Manifest{}, fmt.Errorf("%w: banco %q desconhecido", ErrInvalidBackup, manifest.Banco)
	}
	if manifest.BancoSHA256 == "" {
		return Manifest{}, fmt.Errorf("%w: checksum do banco ausente", ErrInvalidBackup)
	}
	expectedReceipts := make(map[string]ReceiptEntry, len(manifest.Recibos))
	for _, receipt := range manifest.Recibos {
		if !validReceiptName(receipt.Nome) {
			return Manifest{}, fmt.Errorf("%w: nome de recibo inválido %q", ErrInvalidBackup, receipt.Nome)
		}
		if _, exists := expectedReceipts[receipt.Nome]; exists {
			return Manifest{}, fmt.Errorf("%w: recibo duplicado %q", ErrInvalidBackup, receipt.Nome)
		}
		expectedReceipts[receipt.Nome] = receipt
	}
	if err := os.MkdirAll(filepath.Join(destination, ReceiptsFolder), 0o700); err != nil {
		return Manifest{}, err
	}
	var total int64
	dbExtracted := false
	seenReceipts := make(map[string]bool, len(expectedReceipts))
	for _, file := range reader.File {
		switch {
		case file.Name == ManifestFile || file.Name == ReadmeFile:
			continue
		case file.Name == DatabaseFile:
			if dbExtracted {
				return Manifest{}, fmt.Errorf("%w: banco duplicado", ErrInvalidBackup)
			}
			size, hash, err := extractZipFile(file, filepath.Join(destination, DatabaseFile), maxDatabaseSize)
			if err != nil {
				return Manifest{}, err
			}
			if size != manifest.BancoBytes || !strings.EqualFold(hash, manifest.BancoSHA256) {
				return Manifest{}, fmt.Errorf("%w: checksum do banco não confere", ErrInvalidBackup)
			}
			total += size
			dbExtracted = true
		case strings.HasPrefix(file.Name, ReceiptsFolder+"/"):
			name := strings.TrimPrefix(file.Name, ReceiptsFolder+"/")
			receipt, expected := expectedReceipts[name]
			if !expected || !validReceiptName(name) {
				return Manifest{}, fmt.Errorf("%w: recibo inesperado %q", ErrInvalidBackup, file.Name)
			}
			if seenReceipts[name] {
				return Manifest{}, fmt.Errorf("%w: recibo duplicado %q", ErrInvalidBackup, name)
			}
			size, hash, err := extractZipFile(file, filepath.Join(destination, ReceiptsFolder, name), maxReceiptSize)
			if err != nil {
				return Manifest{}, err
			}
			if size != receipt.Bytes || !strings.EqualFold(hash, receipt.SHA256) {
				return Manifest{}, fmt.Errorf("%w: checksum do recibo %q não confere", ErrInvalidBackup, name)
			}
			total += size
			seenReceipts[name] = true
		default:
			return Manifest{}, fmt.Errorf("%w: arquivo inesperado %q", ErrInvalidBackup, file.Name)
		}
		if total > maxTotalSize {
			return Manifest{}, fmt.Errorf("%w: conteúdo grande demais", ErrInvalidBackup)
		}
	}
	if !dbExtracted {
		return Manifest{}, fmt.Errorf("%w: banco ausente", ErrInvalidBackup)
	}
	for name := range expectedReceipts {
		if !seenReceipts[name] {
			return Manifest{}, fmt.Errorf("%w: recibo %q ausente", ErrInvalidBackup, name)
		}
	}
	return manifest, nil
}

func inspectDatabase(ctx context.Context, path string) (int, map[string]int64, error) {
	db, err := sql.Open("sqlite3", path+"?mode=ro&_query_only=1")
	if err != nil {
		return 0, nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return 0, nil, fmt.Errorf("validar integridade do banco: %w", err)
	}
	if integrity != "ok" {
		return 0, nil, fmt.Errorf("%w: integrity_check=%s", ErrInvalidBackup, integrity)
	}
	var schema int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&schema); err != nil {
		return 0, nil, fmt.Errorf("ler versão do schema: %w", err)
	}
	counts := map[string]int64{}
	for _, table := range []string{"imoveis", "inquilinos", "locacoes", "parcelas", "movimentacoes", "despesas", "dividas"} {
		var count int64
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			return 0, nil, fmt.Errorf("contar %s: %w", table, err)
		}
		counts[table] = count
	}
	return schema, counts, nil
}

func validateDatabase(ctx context.Context, path string, expectedSchema int) error {
	db, err := sql.Open("sqlite3", path+"?mode=ro&_query_only=1")
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var integrity string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("%w: validar banco restaurado: %v", ErrInvalidBackup, err)
	}
	if integrity != "ok" {
		return fmt.Errorf("%w: integrity_check=%s", ErrInvalidBackup, integrity)
	}
	var schema int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&schema); err != nil {
		return fmt.Errorf("%w: ler schema restaurado: %v", ErrInvalidBackup, err)
	}
	if schema != expectedSchema {
		return fmt.Errorf("%w: schema do banco (%d) difere do manifesto (%d)", ErrInvalidBackup, schema, expectedSchema)
	}
	return nil
}

func collectReceipts(dir string) ([]receiptFile, error) {
	if strings.TrimSpace(dir) == "" {
		return []receiptFile{}, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []receiptFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler recibos: %w", err)
	}
	items := make([]receiptFile, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		name := entry.Name()
		if !validReceiptName(name) {
			return nil, fmt.Errorf("nome de recibo inválido %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("recibo duplicado %q", name)
		}
		seen[name] = true
		size, hash, err := fileDigest(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		items = append(items, receiptFile{name: name, path: filepath.Join(dir, name), size: size, hash: hash})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
	return items, nil
}

func validReceiptName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return false
	}
	return filepath.Base(name) == name
}

func fileDigest(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func copyFileToWriter(source string, destination io.Writer) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	_, err = io.Copy(destination, input)
	return err
}

func addFile(writer *zip.Writer, name, source string, method uint16) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	header := &zip.FileHeader{Name: name, Method: method}
	header.SetModTime(time.Now())
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, input)
	return err
}

func addBytes(writer *zip.Writer, name string, content []byte, method uint16) error {
	header := &zip.FileHeader{Name: name, Method: method}
	header.SetModTime(time.Now())
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = entry.Write(content)
	return err
}

func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	content, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, fmt.Errorf("arquivo %q grande demais", file.Name)
	}
	return content, nil
}

func extractZipFile(file *zip.File, destination string, limit int64) (int64, string, error) {
	reader, err := file.Open()
	if err != nil {
		return 0, "", err
	}
	defer reader.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, "", err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(reader, limit+1))
	closeErr := output.Close()
	if copyErr != nil {
		return 0, "", copyErr
	}
	if closeErr != nil {
		return 0, "", closeErr
	}
	if size > limit {
		return 0, "", fmt.Errorf("%w: arquivo %q grande demais", ErrInvalidBackup, file.Name)
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

const readme = `Rerum - backup completo
=======================

Este arquivo ZIP contém:

- rerum.sqlite: snapshot consistente do banco de dados.
- recibos/: cópias dos recibos em PDF.
- manifest.json: versão, schema, tamanhos e checksums SHA-256.

Como restaurar
--------------

Use o próprio Rerum: Arquivo > Restaurar backup completo...
A restauração substitui integralmente o banco e a pasta de recibos atuais.
Um backup de segurança do estado anterior é criado automaticamente.

Não edite o conteúdo deste ZIP. Guarde-o em local seguro: ele contém dados
pessoais, como CPF, endereços e telefones.
`

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

const databaseName = "rerum.sqlite"

func userDataDir() (string, error) {
	if base := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, "rerum"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("localizar diretório do usuário: %w", err)
	}
	return filepath.Join(home, ".local", "share", "rerum"), nil
}

func importLegacyData(ctx context.Context, sourceDir, targetDir string) error {
	if _, err := os.Stat(filepath.Join(targetDir, databaseName)); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sourceDB := filepath.Join(sourceDir, databaseName)
	if _, err := os.Stat(sourceDB); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	sourceDB, err := filepath.Abs(sourceDB)
	if err != nil {
		return err
	}

	parent := filepath.Dir(targetDir)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".rerum-import-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)

	sourceURI := url.URL{Scheme: "file", Path: filepath.ToSlash(sourceDB), RawQuery: "mode=ro&_busy_timeout=5000"}
	db, err := sql.Open("sqlite3", sourceURI.String())
	if err != nil {
		return err
	}
	defer db.Close()
	snapshot := filepath.Join(stage, databaseName)
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", snapshot); err != nil {
		return fmt.Errorf("copiar banco anterior: %w", err)
	}
	if err := os.Chmod(snapshot, 0o600); err != nil {
		return err
	}

	sourceReceipts := filepath.Join(sourceDir, "recibos")
	if info, err := os.Stat(sourceReceipts); err == nil && info.IsDir() {
		if err := os.CopyFS(filepath.Join(stage, "recibos"), os.DirFS(sourceReceipts)); err != nil {
			return fmt.Errorf("copiar recibos anteriores: %w", err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.Rename(stage, targetDir); err != nil {
		return fmt.Errorf("publicar dados em %s: %w", targetDir, err)
	}
	return nil
}

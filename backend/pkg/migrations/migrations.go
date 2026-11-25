package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed sql/*.sql
var embedded embed.FS

func migrationFiles(dir string) (fs.FS, []string, error) {
	var source fs.FS
	if dir == "" {
		source, _ = fs.Sub(embedded, "sql")
	} else {
		source = os.DirFS(dir)
	}
	files, err := fs.Glob(source, "*.sql")
	if err == nil && len(files) == 0 {
		err = fmt.Errorf("nenhuma migração SQL encontrada em %q", dir)
	}
	return source, files, err
}

// MaxVersion returns the highest migration version. An empty dir uses the embedded SQL.
func MaxVersion(dir string) (int, error) {
	_, files, err := migrationFiles(dir)
	if err != nil {
		return 0, err
	}
	maxVersion := 0
	for _, name := range files {
		var version int
		if _, err := fmt.Sscanf(filepath.Base(name), "%d_", &version); err != nil {
			return 0, fmt.Errorf("nome de migração inválido %q: %w", name, err)
		}
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion, nil
}

func Apply(ctx context.Context, db *sql.DB, dir string) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, aplicado_em TEXT NOT NULL DEFAULT (date('now')))`); err != nil {
		return err
	}
	source, files, err := migrationFiles(dir)
	if err != nil {
		return err
	}
	for _, name := range files {
		var version int
		if _, err := fmt.Sscanf(filepath.Base(name), "%d_", &version); err != nil {
			return fmt.Errorf("nome de migração inválido %q: %w", name, err)
		}
		var applied int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version=?`, version).Scan(&applied); err != nil {
			return err
		}
		if applied != 0 {
			continue
		}
		sqlBytes, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(sqlBytes)); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES (?)`, version)
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migração %s: %w", filepath.Base(name), err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

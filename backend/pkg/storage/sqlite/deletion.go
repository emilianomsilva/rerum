package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func (s *Store) DeletePropertyCascade(ctx context.Context, id int64) (string, error) {
	if err := recordExists(ctx, s.DB, `SELECT 1 FROM imoveis WHERE id=?`, id); err != nil {
		return "", err
	}
	backup, err := s.backupBeforeCascade(ctx, "imovel")
	if err != nil {
		return "", err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return backup, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacoes
		WHERE imovel_id=?
		OR locacao_id IN (SELECT id FROM locacoes WHERE imovel_id=?)
		OR parcela_id IN (
			SELECT p.id FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id WHERE l.imovel_id=?
		)`, id, id, id); err != nil {
		return backup, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacao_rateios WHERE imovel_id=?`, id); err != nil {
		return backup, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM despesas WHERE imovel_id=?`, id); err != nil {
		return backup, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM locacoes WHERE imovel_id=?`, id); err != nil {
		return backup, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM imoveis WHERE id=?`, id)
	if err != nil {
		return backup, err
	}
	if count, err := result.RowsAffected(); err != nil {
		return backup, err
	} else if count == 0 {
		return backup, ErrNotFound
	}
	return backup, tx.Commit()
}

func (s *Store) DeleteTenantCascade(ctx context.Context, id int64) (string, error) {
	if err := recordExists(ctx, s.DB, `SELECT 1 FROM inquilinos WHERE id=?`, id); err != nil {
		return "", err
	}
	backup, err := s.backupBeforeCascade(ctx, "inquilino")
	if err != nil {
		return "", err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return backup, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM movimentacoes
		WHERE inquilino_id=?
		OR locacao_id IN (SELECT id FROM locacoes WHERE inquilino_id=?)
		OR parcela_id IN (
			SELECT p.id FROM parcelas p JOIN locacoes l ON l.id=p.locacao_id WHERE l.inquilino_id=?
		)`, id, id, id); err != nil {
		return backup, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM locacoes WHERE inquilino_id=?`, id); err != nil {
		return backup, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM inquilinos WHERE id=?`, id)
	if err != nil {
		return backup, err
	}
	if count, err := result.RowsAffected(); err != nil {
		return backup, err
	} else if count == 0 {
		return backup, ErrNotFound
	}
	return backup, tx.Commit()
}

func recordExists(ctx context.Context, db *sql.DB, query string, id int64) error {
	var found int
	err := db.QueryRowContext(ctx, query, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) backupBeforeCascade(ctx context.Context, entity string) (string, error) {
	rows, err := s.DB.QueryContext(ctx, `PRAGMA database_list`)
	if err != nil {
		return "", err
	}
	var databasePath string
	for rows.Next() {
		var sequence int
		var name, path string
		if err := rows.Scan(&sequence, &name, &path); err != nil {
			rows.Close()
			return "", err
		}
		if name == "main" {
			databasePath = path
			break
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()
	if databasePath == "" {
		return "", errors.New("não foi possível criar backup antes da exclusão em cascata")
	}

	extension := filepath.Ext(databasePath)
	if extension == "" {
		extension = ".sqlite"
	}
	base := strings.TrimSuffix(filepath.Base(databasePath), filepath.Ext(databasePath))
	backup := filepath.Join(filepath.Dir(databasePath), fmt.Sprintf("%s-pre-cascade-%s-%s%s", base, entity, time.Now().Format("20060102-150405.000000000"), extension))
	if _, err := s.DB.ExecContext(ctx, `VACUUM INTO ?`, backup); err != nil {
		return "", fmt.Errorf("criar backup antes da exclusão: %w", err)
	}
	return backup, nil
}

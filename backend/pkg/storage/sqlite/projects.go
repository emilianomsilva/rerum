package sqlite

import (
	"context"
	"errors"
)

var ErrProjectInUse = errors.New("empreendimento possui vínculos; arquive-o para preservar imóveis e histórico financeiro")

func (s *Store) ReactivateProject(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE empreendimentos SET ativo=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	if count, err := result.RowsAffected(); err != nil {
		return err
	} else if count == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUnusedProject never cascades into properties, allocations or financial history.
func (s *Store) DeleteUnusedProject(ctx context.Context, id int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM empreendimentos WHERE id=?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	var linked bool
	if err := tx.QueryRowContext(ctx, `SELECT
		EXISTS(SELECT 1 FROM imoveis WHERE empreendimento_id=?) OR
		EXISTS(SELECT 1 FROM empreendimento_unidades WHERE empreendimento_id=?) OR
		EXISTS(SELECT 1 FROM movimentacoes WHERE empreendimento_id=?) OR
		EXISTS(SELECT 1 FROM despesas WHERE empreendimento_id=?)`, id, id, id, id).Scan(&linked); err != nil {
		return err
	}
	if linked {
		return ErrProjectInUse
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM empreendimentos WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

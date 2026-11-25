package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"rerum/backend/pkg/domain"
)

type InstallmentUpdate struct {
	Vencimento *string `json:"vencimento"`
	ValorBaseC *int64  `json:"valor_base_c"`
	MultaC     *int64  `json:"multa_c"`
	JurosC     *int64  `json:"juros_c"`
	DescontoC  *int64  `json:"desconto_c"`
	Tipo       *string `json:"tipo"`
	Observacao *string `json:"observacao"`
}

func editableInstallment(ctx context.Context, query rowQuerier, id int64) (domain.Parcela, error) {
	p, err := installment(ctx, query, id)
	if err != nil {
		return p, err
	}
	if p.Status != "aberta" || p.ValorPagoC != 0 || p.DataPagamento != nil || p.ReciboPath != nil || p.Forma != nil {
		return p, errors.New("parcela bloqueada: somente parcelas abertas sem recebimentos podem ser alteradas, canceladas ou excluídas")
	}
	var entries int
	if err := query.QueryRowContext(ctx, `SELECT COUNT(*) FROM movimentacoes WHERE parcela_id=?`, id).Scan(&entries); err != nil {
		return p, err
	}
	if entries != 0 {
		return p, errors.New("parcela bloqueada: há lançamentos financeiros vinculados")
	}
	return p, nil
}

func validateInstallmentAmounts(p domain.Parcela) error {
	if p.ValorBaseC <= 0 || p.MultaC < 0 || p.JurosC < 0 {
		return errors.New("valor base deve ser positivo; multa e juros não podem ser negativos")
	}
	total, err := addNonnegativeAmounts(p.ValorBaseC, p.MultaC, p.JurosC)
	if err != nil {
		return err
	}
	if p.DescontoC >= total || (p.DescontoC < 0 && total > maxInt64+p.DescontoC) {
		return errors.New("desconto ou acréscimo inválido para o total da parcela")
	}
	return nil
}

func (s *Store) UpdateOpenInstallment(ctx context.Context, id int64, input InstallmentUpdate) (domain.Parcela, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.Parcela{}, err
	}
	defer tx.Rollback()
	p, err := editableInstallment(ctx, tx, id)
	if err != nil {
		return p, err
	}
	if input.Vencimento != nil {
		if _, err := domain.ParseDate(*input.Vencimento); err != nil {
			return p, fmt.Errorf("vencimento inválido: %w", err)
		}
		p.Vencimento = *input.Vencimento
	}
	for _, field := range []struct{ target, value *int64 }{
		{&p.ValorBaseC, input.ValorBaseC}, {&p.MultaC, input.MultaC}, {&p.JurosC, input.JurosC}, {&p.DescontoC, input.DescontoC},
	} {
		if field.value != nil {
			*field.target = *field.value
		}
	}
	if err := validateInstallmentAmounts(p); err != nil {
		return p, err
	}
	if input.Tipo != nil {
		if strings.TrimSpace(*input.Tipo) == "" {
			return p, errors.New("tipo da parcela é obrigatório")
		}
		p.Tipo = strings.TrimSpace(*input.Tipo)
	}
	if input.Observacao != nil {
		p.Observacao = *input.Observacao
	}
	// Due-date changes do not rewrite the original rental competency, period or numbering.
	_, err = tx.ExecContext(ctx, `UPDATE parcelas SET vencimento=?,valor_base_c=?,multa_c=?,juros_c=?,desconto_c=?,tipo=?,observacao=? WHERE id=?`, p.Vencimento, p.ValorBaseC, p.MultaC, p.JurosC, p.DescontoC, p.Tipo, p.Observacao, p.ID)
	if err != nil {
		return p, err
	}
	return p, tx.Commit()
}

func (s *Store) CancelInstallment(ctx context.Context, id int64) (domain.Parcela, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return domain.Parcela{}, err
	}
	defer tx.Rollback()
	p, err := editableInstallment(ctx, tx, id)
	if err != nil {
		return p, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE parcelas SET status='cancelada' WHERE id=?`, id); err != nil {
		return p, err
	}
	p.Status = "cancelada"
	return p, tx.Commit()
}

func (s *Store) DeleteInstallment(ctx context.Context, id int64) (string, error) {
	if _, err := editableInstallment(ctx, s.DB, id); err != nil {
		return "", err
	}
	backup, err := s.backupBeforeCascade(ctx, "parcela")
	if err != nil {
		return "", err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return backup, err
	}
	defer tx.Rollback()
	// Recheck inside the transaction, in case a payment was recorded during the backup.
	if _, err := editableInstallment(ctx, tx, id); err != nil {
		return backup, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM parcelas WHERE id=?`, id); err != nil {
		return backup, err
	}
	return backup, tx.Commit()
}

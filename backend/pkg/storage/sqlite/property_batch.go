package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"rerum/backend/pkg/domain"
)

// PropertyBatch keeps the cost allocation authoritative in Go, including previews.
type PropertyBatch struct {
	Imoveis   []domain.Imovel `json:"imoveis"`
	ModoCusto string          `json:"modo_custo"`
	CompraC   int64           `json:"valor_compra_c"`
	ReformaC  int64           `json:"reforma_c"`
	OutrosC   int64           `json:"outros_custos_c"`
}

func (b PropertyBatch) Preview() ([]domain.Imovel, error) {
	if len(b.Imoveis) == 0 || len(b.Imoveis) > 100 {
		return nil, errors.New("lote deve conter entre 1 e 100 imóveis")
	}
	if b.ModoCusto != "dividir" && b.ModoCusto != "fixo" {
		return nil, errors.New("modo de custo inválido")
	}
	if b.CompraC < 0 || b.ReformaC < 0 || b.OutrosC < 0 || b.CompraC > 100_000_000_000 || b.ReformaC > 100_000_000_000 || b.OutrosC > 100_000_000_000 {
		return nil, errors.New("custos do lote inválidos")
	}
	items := append([]domain.Imovel(nil), b.Imoveis...)
	seen := map[string]bool{}
	for i := range items {
		item := &items[i]
		item.Apelido = strings.TrimSpace(item.Apelido)
		item.Numero = strings.TrimSpace(item.Numero)
		item.Unidade = strings.TrimSpace(item.Unidade)
		item.MatriculaCartorio = strings.TrimSpace(item.MatriculaCartorio)
		if item.ID != 0 || item.Apelido == "" || (item.Status != "disponivel" && item.Status != "manutencao") {
			return nil, fmt.Errorf("imóvel %d: identificação ou status inválido", i+1)
		}
		if item.DataAquisicao != nil && *item.DataAquisicao != "" {
			if _, err := domain.ParseDate(*item.DataAquisicao); err != nil {
				return nil, fmt.Errorf("imóvel %d: data de aquisição inválida", i+1)
			}
		}
		projectID := int64(0)
		if item.EmpreendimentoID != nil {
			projectID = *item.EmpreendimentoID
		}
		key := fmt.Sprintf("%d:apelido:%q", projectID, strings.ToLower(item.Apelido))
		if item.Numero != "" && item.Unidade != "" {
			key = fmt.Sprintf("%d:unidade:%q:%q", projectID, strings.ToLower(item.Numero), strings.ToLower(item.Unidade))
		}
		if seen[key] {
			return nil, fmt.Errorf("imóvel %d: unidade duplicada no lote", i+1)
		}
		seen[key] = true
		item.Ativo = true
		item.ValorCompraC = b.portion(b.CompraC, i)
		item.ReformaC = b.portion(b.ReformaC, i)
		item.OutrosCustosC = b.portion(b.OutrosC, i)
		item.ITBIC, item.CorretagemC, item.InvestimentoImportadoC = 0, 0, 0
		item.InvestimentoTotalC = item.Investimento()
	}
	return items, nil
}

func (b PropertyBatch) portion(amount int64, index int) int64 {
	if b.ModoCusto == "fixo" {
		return amount
	}
	count := int64(len(b.Imoveis))
	value := amount / count
	if int64(index) < amount%count {
		value++
	}
	return value
}

func (s *Store) SavePropertyBatch(ctx context.Context, batch PropertyBatch) ([]domain.Imovel, error) {
	items, err := batch.Preview()
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for i, item := range items {
		if item.EmpreendimentoID != nil {
			var active bool
			err := tx.QueryRowContext(ctx, `SELECT ativo FROM empreendimentos WHERE id=?`, *item.EmpreendimentoID).Scan(&active)
			if errors.Is(err, sql.ErrNoRows) || err == nil && !active {
				return nil, fmt.Errorf("imóvel %d: empreendimento inválido ou arquivado", i+1)
			}
			if err != nil {
				return nil, err
			}
		}
		saved, err := saveProperty(ctx, tx, item)
		if err != nil {
			return nil, fmt.Errorf("imóvel %d: %w", i+1, err)
		}
		items[i] = saved
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

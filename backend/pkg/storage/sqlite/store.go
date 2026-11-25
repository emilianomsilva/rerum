package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"rerum/backend/pkg/domain"
)

var ErrNotFound = errors.New("registro não encontrado")

type Store struct {
	DB          *sql.DB
	ReceiptsDir string
}

func (s *Store) Properties(ctx context.Context, includeInactive bool) ([]domain.Imovel, error) {
	query := `SELECT ` + propertyColumns + ` FROM imoveis`
	if !includeInactive {
		query += ` WHERE ativo=1`
	}
	query += ` ORDER BY apelido`
	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Imovel, 0)
	for rows.Next() {
		item, err := scanProperty(rows)
		if err != nil {
			return nil, err
		}
		item.InvestimentoTotalC = item.Investimento()
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Property(ctx context.Context, id int64) (domain.Imovel, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+propertyColumns+` FROM imoveis WHERE id=?`, id)
	item, err := scanProperty(row)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	item.InvestimentoTotalC = item.Investimento()
	return item, err
}

type scanner interface{ Scan(...any) error }

const propertyColumns = `id,apelido,endereco,tipo,status,valor_compra_c,reforma_c,itbi_c,corretagem_c,outros_custos_c,data_aquisicao,observacao,ativo,matricula_cartorio,inscricao_iptu,unidade_agua,unidade_energia,empreendimento_id,bairro,situacao_patrimonial,investimento_importado_c,destinado_locacao,matricula_compartilhada,data_venda,valor_venda_c,numero,unidade`

type propertyWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func scanProperty(row scanner) (domain.Imovel, error) {
	var item domain.Imovel
	var acquired, saleDate sql.NullString
	var project sql.NullInt64
	var active, rentDest, sharedReg int
	err := row.Scan(&item.ID, &item.Apelido, &item.Endereco, &item.Tipo, &item.Status, &item.ValorCompraC, &item.ReformaC, &item.ITBIC, &item.CorretagemC, &item.OutrosCustosC, &acquired, &item.Observacao, &active, &item.MatriculaCartorio, &item.InscricaoIPTU, &item.UnidadeAgua, &item.UnidadeEnergia, &project, &item.Bairro, &item.SituacaoPatrimonial, &item.InvestimentoImportadoC, &rentDest, &sharedReg, &saleDate, &item.ValorVendaC, &item.Numero, &item.Unidade)
	if err != nil {
		return item, err
	}
	item.Ativo = active != 0
	item.DestinadoLocacao = rentDest != 0
	item.MatriculaCompartilhada = sharedReg != 0
	if acquired.Valid && acquired.String != "" {
		item.DataAquisicao = &acquired.String
	}
	if saleDate.Valid && saleDate.String != "" {
		item.DataVenda = &saleDate.String
	}
	if project.Valid {
		item.EmpreendimentoID = &project.Int64
	}
	return item, nil
}

func (s *Store) SaveProperty(ctx context.Context, item domain.Imovel) (domain.Imovel, error) {
	return saveProperty(ctx, s.DB, item)
}

func saveProperty(ctx context.Context, writer propertyWriter, item domain.Imovel) (domain.Imovel, error) {
	if strings.TrimSpace(item.Apelido) == "" {
		return item, errors.New("apelido do imóvel é obrigatório")
	}
	item.Numero = strings.TrimSpace(item.Numero)
	item.Unidade = strings.TrimSpace(item.Unidade)
	var acquired any
	if item.DataAquisicao != nil && *item.DataAquisicao != "" {
		acquired = *item.DataAquisicao
	}
	var saleDate any
	if item.DataVenda != nil && *item.DataVenda != "" {
		saleDate = *item.DataVenda
	}
	if strings.TrimSpace(item.SituacaoPatrimonial) == "" {
		item.SituacaoPatrimonial = "proprio"
	}
	if item.SituacaoPatrimonial == "vendido" {
		item.DestinadoLocacao = false
	}
	if item.ID == 0 {
		result, err := writer.ExecContext(ctx, `INSERT INTO imoveis(apelido,endereco,tipo,status,valor_compra_c,reforma_c,itbi_c,corretagem_c,outros_custos_c,data_aquisicao,observacao,ativo,matricula_cartorio,inscricao_iptu,unidade_agua,unidade_energia,empreendimento_id,bairro,situacao_patrimonial,investimento_importado_c,destinado_locacao,matricula_compartilhada,data_venda,valor_venda_c,numero,unidade) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.Apelido, item.Endereco, item.Tipo, item.Status, item.ValorCompraC, item.ReformaC, item.ITBIC, item.CorretagemC, item.OutrosCustosC, acquired, item.Observacao, boolInt(item.Ativo), item.MatriculaCartorio, item.InscricaoIPTU, item.UnidadeAgua, item.UnidadeEnergia, item.EmpreendimentoID, item.Bairro, item.SituacaoPatrimonial, item.InvestimentoImportadoC, boolInt(item.DestinadoLocacao), boolInt(item.MatriculaCompartilhada), saleDate, item.ValorVendaC, item.Numero, item.Unidade)
		if err != nil {
			return item, err
		}
		item.ID, err = result.LastInsertId()
		if err != nil {
			return item, err
		}
	} else {
		result, err := writer.ExecContext(ctx, `UPDATE imoveis SET apelido=?,endereco=?,tipo=?,status=?,valor_compra_c=?,reforma_c=?,itbi_c=?,corretagem_c=?,outros_custos_c=?,data_aquisicao=?,observacao=?,ativo=?,matricula_cartorio=?,inscricao_iptu=?,unidade_agua=?,unidade_energia=?,empreendimento_id=?,bairro=?,situacao_patrimonial=?,destinado_locacao=?,matricula_compartilhada=?,data_venda=?,valor_venda_c=?,numero=?,unidade=? WHERE id=?`, item.Apelido, item.Endereco, item.Tipo, item.Status, item.ValorCompraC, item.ReformaC, item.ITBIC, item.CorretagemC, item.OutrosCustosC, acquired, item.Observacao, boolInt(item.Ativo), item.MatriculaCartorio, item.InscricaoIPTU, item.UnidadeAgua, item.UnidadeEnergia, item.EmpreendimentoID, item.Bairro, item.SituacaoPatrimonial, boolInt(item.DestinadoLocacao), boolInt(item.MatriculaCompartilhada), saleDate, item.ValorVendaC, item.Numero, item.Unidade, item.ID)
		if err != nil {
			return item, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return item, err
		}
		if count == 0 {
			return item, ErrNotFound
		}
	}
	saved, err := scanProperty(writer.QueryRowContext(ctx, `SELECT `+propertyColumns+` FROM imoveis WHERE id=?`, item.ID))
	saved.InvestimentoTotalC = saved.Investimento()
	return saved, err
}

func (s *Store) DeactivateProperty(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE imoveis SET ativo=0 WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) ReactivateProperty(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE imoveis SET ativo=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Tenants(ctx context.Context, includeInactive bool) ([]domain.Inquilino, error) {
	query := `SELECT id,nome,cpf,telefone,email,observacao,ativo,rg,renda_mensal_c,local_trabalho,endereco_atual FROM inquilinos`
	if !includeInactive {
		query += ` WHERE ativo=1`
	}
	query += ` ORDER BY nome`
	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]domain.Inquilino, 0)
	for rows.Next() {
		var item domain.Inquilino
		var active int
		if err := rows.Scan(&item.ID, &item.Nome, &item.CPF, &item.Telefone, &item.Email, &item.Observacao, &active, &item.RG, &item.RendaMensalC, &item.LocalTrabalho, &item.EnderecoAtual); err != nil {
			return nil, err
		}
		item.Ativo = active != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Tenant(ctx context.Context, id int64) (domain.Inquilino, error) {
	var item domain.Inquilino
	var active int
	err := s.DB.QueryRowContext(ctx, `SELECT id,nome,cpf,telefone,email,observacao,ativo,rg,renda_mensal_c,local_trabalho,endereco_atual FROM inquilinos WHERE id=?`, id).Scan(&item.ID, &item.Nome, &item.CPF, &item.Telefone, &item.Email, &item.Observacao, &active, &item.RG, &item.RendaMensalC, &item.LocalTrabalho, &item.EnderecoAtual)
	if errors.Is(err, sql.ErrNoRows) {
		return item, ErrNotFound
	}
	item.Ativo = active != 0
	return item, err
}

func (s *Store) SaveTenant(ctx context.Context, item domain.Inquilino) (domain.Inquilino, error) {
	if strings.TrimSpace(item.Nome) == "" {
		return item, errors.New("nome do inquilino é obrigatório")
	}
	if item.ID == 0 {
		result, err := s.DB.ExecContext(ctx, `INSERT INTO inquilinos(nome,cpf,telefone,email,observacao,ativo,rg,renda_mensal_c,local_trabalho,endereco_atual) VALUES(?,?,?,?,?,?,?,?,?,?)`, item.Nome, item.CPF, item.Telefone, item.Email, item.Observacao, boolInt(item.Ativo), item.RG, item.RendaMensalC, item.LocalTrabalho, item.EnderecoAtual)
		if err != nil {
			return item, err
		}
		item.ID, err = result.LastInsertId()
		if err != nil {
			return item, err
		}
	} else {
		result, err := s.DB.ExecContext(ctx, `UPDATE inquilinos SET nome=?,cpf=?,telefone=?,email=?,observacao=?,rg=?,renda_mensal_c=?,local_trabalho=?,endereco_atual=? WHERE id=?`, item.Nome, item.CPF, item.Telefone, item.Email, item.Observacao, item.RG, item.RendaMensalC, item.LocalTrabalho, item.EnderecoAtual, item.ID)
		if err != nil {
			return item, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return item, err
		}
		if count == 0 {
			return item, ErrNotFound
		}
	}
	return s.Tenant(ctx, item.ID)
}

func (s *Store) DeactivateTenant(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE inquilinos SET ativo=0 WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) ReactivateTenant(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `UPDATE inquilinos SET ativo=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return ErrNotFound
	}
	return err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

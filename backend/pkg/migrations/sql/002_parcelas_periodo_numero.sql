ALTER TABLE parcelas ADD COLUMN numero INTEGER NOT NULL DEFAULT 1;
ALTER TABLE parcelas ADD COLUMN total_parcelas INTEGER NOT NULL DEFAULT 1;
ALTER TABLE parcelas ADD COLUMN periodo_inicio TEXT;
ALTER TABLE parcelas ADD COLUMN periodo_fim TEXT;
ALTER TABLE parcelas ADD COLUMN tipo TEXT NOT NULL DEFAULT 'mensal';

UPDATE parcelas SET periodo_inicio = date(vencimento, '-1 month', '+1 day') WHERE periodo_inicio IS NULL;
UPDATE parcelas SET periodo_fim = date(vencimento, '+1 day') WHERE periodo_fim IS NULL;

ALTER TABLE inquilinos ADD COLUMN rg TEXT NOT NULL DEFAULT '';
ALTER TABLE inquilinos ADD COLUMN renda_mensal_c INTEGER NOT NULL DEFAULT 0;
ALTER TABLE inquilinos ADD COLUMN local_trabalho TEXT NOT NULL DEFAULT '';
ALTER TABLE inquilinos ADD COLUMN endereco_atual TEXT NOT NULL DEFAULT '';

ALTER TABLE imoveis ADD COLUMN matricula_cartorio TEXT NOT NULL DEFAULT '';
ALTER TABLE imoveis ADD COLUMN inscricao_iptu TEXT NOT NULL DEFAULT '';
ALTER TABLE imoveis ADD COLUMN unidade_agua TEXT NOT NULL DEFAULT '';
ALTER TABLE imoveis ADD COLUMN unidade_energia TEXT NOT NULL DEFAULT '';

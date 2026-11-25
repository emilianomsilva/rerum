-- Expansão para suporte nativo integral: portfólio, unidades padrão de rateio, contratos/parcelas flexíveis e juros em dívidas.

-- Imóveis: distinção de destinação para locação, matrícula compartilhada e liquidação de vendas
ALTER TABLE imoveis ADD COLUMN destinado_locacao INTEGER NOT NULL DEFAULT 1 CHECK (destinado_locacao IN (0,1));
ALTER TABLE imoveis ADD COLUMN matricula_compartilhada INTEGER NOT NULL DEFAULT 0 CHECK (matricula_compartilhada IN (0,1));
ALTER TABLE imoveis ADD COLUMN data_venda TEXT;
ALTER TABLE imoveis ADD COLUMN valor_venda_c INTEGER NOT NULL DEFAULT 0;

UPDATE imoveis SET destinado_locacao = 0 WHERE situacao_patrimonial = 'vendido';

-- Unidades padrão por empreendimento para rateio automático
CREATE TABLE IF NOT EXISTS empreendimento_unidades (
  empreendimento_id INTEGER NOT NULL REFERENCES empreendimentos(id) ON DELETE CASCADE,
  imovel_id INTEGER NOT NULL REFERENCES imoveis(id) ON DELETE CASCADE,
  peso INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (empreendimento_id, imovel_id)
);

INSERT OR IGNORE INTO empreendimento_unidades (empreendimento_id, imovel_id, peso)
SELECT empreendimento_id, id, 1
FROM imoveis
WHERE empreendimento_id IS NOT NULL;

-- Locações e parcelas
ALTER TABLE locacoes ADD COLUMN intermediario TEXT NOT NULL DEFAULT 'Contrato direto';
ALTER TABLE locacoes ADD COLUMN observacao TEXT NOT NULL DEFAULT '';
ALTER TABLE locacoes ADD COLUMN primeiro_vencimento TEXT;

ALTER TABLE parcelas ADD COLUMN observacao TEXT NOT NULL DEFAULT '';

-- Livro financeiro unificado: natureza operacional vs investimento (obra/capital)
ALTER TABLE movimentacoes ADD COLUMN natureza TEXT NOT NULL DEFAULT 'operacional'
  CHECK (natureza IN ('operacional','investimento'));

UPDATE movimentacoes
SET natureza = 'investimento'
WHERE tipo = 'despesa' AND empreendimento_id IS NOT NULL;

-- Migração de compatibilidade de despesas operacionais legadas para movimentações
INSERT INTO movimentacoes (tipo, categoria, data, valor_c, descricao, imovel_id, empreendimento_id, forma_pagamento, fornecedor, natureza)
SELECT 'despesa', d.categoria, d.data, d.valor_c, d.descricao, d.imovel_id, d.empreendimento_id, d.forma_pagamento, d.fornecedor, 'operacional'
FROM despesas d
WHERE NOT EXISTS (
  SELECT 1 FROM movimentacoes m
  WHERE m.imovel_id = d.imovel_id AND m.data = d.data AND m.valor_c = d.valor_c AND m.tipo = 'despesa'
);

-- Débitos: suporte a juros e encargos em pagamentos
ALTER TABLE movimentos_divida ADD COLUMN juros_c INTEGER NOT NULL DEFAULT 0 CHECK (juros_c >= 0);

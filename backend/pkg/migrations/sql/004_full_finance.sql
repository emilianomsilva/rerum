-- Expansão compatível: empreendimentos, livro financeiro, rateios e dívidas.
CREATE TABLE IF NOT EXISTS empreendimentos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  nome TEXT NOT NULL UNIQUE,
  endereco TEXT NOT NULL DEFAULT '',
  observacao TEXT NOT NULL DEFAULT '',
  ativo INTEGER NOT NULL DEFAULT 1
);

ALTER TABLE imoveis ADD COLUMN empreendimento_id INTEGER REFERENCES empreendimentos(id);
ALTER TABLE imoveis ADD COLUMN bairro TEXT NOT NULL DEFAULT '';
ALTER TABLE imoveis ADD COLUMN situacao_patrimonial TEXT NOT NULL DEFAULT 'proprio'
  CHECK (situacao_patrimonial IN ('proprio','vendido'));

ALTER TABLE despesas ADD COLUMN empreendimento_id INTEGER REFERENCES empreendimentos(id);
ALTER TABLE despesas ADD COLUMN fornecedor TEXT NOT NULL DEFAULT '';
ALTER TABLE despesas ADD COLUMN forma_pagamento TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS movimentacoes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tipo TEXT NOT NULL CHECK (tipo IN ('receita','despesa')),
  categoria TEXT NOT NULL,
  data TEXT NOT NULL,
  valor_c INTEGER NOT NULL CHECK (valor_c > 0),
  descricao TEXT NOT NULL DEFAULT '',
  imovel_id INTEGER REFERENCES imoveis(id),
  empreendimento_id INTEGER REFERENCES empreendimentos(id),
  inquilino_id INTEGER REFERENCES inquilinos(id),
  locacao_id INTEGER REFERENCES locacoes(id),
  parcela_id INTEGER REFERENCES parcelas(id),
  referencia TEXT NOT NULL DEFAULT '',
  forma_pagamento TEXT NOT NULL DEFAULT '',
  origem TEXT NOT NULL DEFAULT '',
  fornecedor TEXT NOT NULL DEFAULT '',
  observacao TEXT NOT NULL DEFAULT '',
  criado_em TEXT NOT NULL DEFAULT (date('now')),
  CHECK (imovel_id IS NOT NULL OR empreendimento_id IS NOT NULL)
);

CREATE TABLE IF NOT EXISTS movimentacao_rateios (
  movimentacao_id INTEGER NOT NULL REFERENCES movimentacoes(id) ON DELETE CASCADE,
  imovel_id INTEGER NOT NULL REFERENCES imoveis(id),
  valor_c INTEGER NOT NULL CHECK (valor_c >= 0),
  PRIMARY KEY (movimentacao_id, imovel_id)
);

CREATE TABLE IF NOT EXISTS dividas (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  credor TEXT NOT NULL,
  descricao TEXT NOT NULL DEFAULT '',
  valor_original_c INTEGER NOT NULL CHECK (valor_original_c > 0),
  data_contratacao TEXT NOT NULL,
  vencimento TEXT,
  observacao TEXT NOT NULL DEFAULT '',
  ativa INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS movimentos_divida (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  divida_id INTEGER NOT NULL REFERENCES dividas(id) ON DELETE CASCADE,
  tipo TEXT NOT NULL CHECK (tipo IN ('liberacao','pagamento')),
  data TEXT NOT NULL,
  valor_c INTEGER NOT NULL CHECK (valor_c > 0),
  forma_pagamento TEXT NOT NULL DEFAULT '',
  descricao TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_movimentacoes_data ON movimentacoes(data, tipo);
CREATE INDEX IF NOT EXISTS idx_movimentacoes_imovel ON movimentacoes(imovel_id, data);
CREATE INDEX IF NOT EXISTS idx_movimentos_divida ON movimentos_divida(divida_id, data);

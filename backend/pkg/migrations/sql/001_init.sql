-- Rerum MVP schema. Tipos restritos a INTEGER/TEXT/REAL para porte facil (Go/Rust).
-- Datas em TEXT ISO YYYY-MM-DD. Dinheiro em INTEGER centavos.

CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  aplicado_em TEXT NOT NULL DEFAULT (date('now'))
);

CREATE TABLE IF NOT EXISTS imoveis (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  apelido TEXT NOT NULL,
  endereco TEXT NOT NULL DEFAULT '',
  tipo TEXT NOT NULL DEFAULT 'casa',
  status TEXT NOT NULL DEFAULT 'disponivel' CHECK (status IN ('disponivel','ocupado','manutencao')),
  valor_compra_c INTEGER NOT NULL DEFAULT 0,
  reforma_c INTEGER NOT NULL DEFAULT 0,
  itbi_c INTEGER NOT NULL DEFAULT 0,
  corretagem_c INTEGER NOT NULL DEFAULT 0,
  outros_custos_c INTEGER NOT NULL DEFAULT 0,
  data_aquisicao TEXT,
  observacao TEXT NOT NULL DEFAULT '',
  ativo INTEGER NOT NULL DEFAULT 1,
  criado_em TEXT NOT NULL DEFAULT (date('now'))
);

CREATE TABLE IF NOT EXISTS inquilinos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  nome TEXT NOT NULL,
  cpf TEXT NOT NULL DEFAULT '',
  telefone TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  observacao TEXT NOT NULL DEFAULT '',
  ativo INTEGER NOT NULL DEFAULT 1,
  criado_em TEXT NOT NULL DEFAULT (date('now'))
);

CREATE TABLE IF NOT EXISTS locacoes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  imovel_id INTEGER NOT NULL REFERENCES imoveis(id),
  inquilino_id INTEGER NOT NULL REFERENCES inquilinos(id),
  inicio TEXT NOT NULL,
  fim TEXT NOT NULL,
  dia_vencimento INTEGER NOT NULL,
  valor_mensal_c INTEGER NOT NULL,
  multa_pct REAL NOT NULL DEFAULT 2.0,
  juros_dia_pct REAL NOT NULL DEFAULT 0.033,
  reajuste_indice TEXT NOT NULL DEFAULT 'nenhum',
  status TEXT NOT NULL DEFAULT 'ativa' CHECK (status IN ('ativa','encerrada')),
  criado_em TEXT NOT NULL DEFAULT (date('now'))
);

CREATE TABLE IF NOT EXISTS parcelas (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  locacao_id INTEGER NOT NULL REFERENCES locacoes(id) ON DELETE CASCADE,
  competencia TEXT NOT NULL,
  vencimento TEXT NOT NULL,
  valor_base_c INTEGER NOT NULL,
  multa_c INTEGER NOT NULL DEFAULT 0,
  juros_c INTEGER NOT NULL DEFAULT 0,
  desconto_c INTEGER NOT NULL DEFAULT 0,
  valor_pago_c INTEGER NOT NULL DEFAULT 0,
  data_pagamento TEXT,
  forma TEXT,
  status TEXT NOT NULL DEFAULT 'aberta' CHECK (status IN ('aberta','parcial','paga','cancelada')),
  recibo_path TEXT,
  criado_em TEXT NOT NULL DEFAULT (date('now'))
);

CREATE TABLE IF NOT EXISTS despesas (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  imovel_id INTEGER NOT NULL REFERENCES imoveis(id) ON DELETE CASCADE,
  data TEXT NOT NULL,
  categoria TEXT NOT NULL,
  valor_c INTEGER NOT NULL,
  descricao TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS config (
  chave TEXT PRIMARY KEY,
  valor TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_locacoes_imovel ON locacoes(imovel_id, status);
CREATE INDEX IF NOT EXISTS idx_parcelas_locacao ON parcelas(locacao_id, vencimento, status);
CREATE INDEX IF NOT EXISTS idx_parcelas_venc ON parcelas(vencimento, status);
CREATE INDEX IF NOT EXISTS idx_despesas_imovel ON despesas(imovel_id, data);

INSERT OR IGNORE INTO config(chave, valor) VALUES ('multa_padrao_pct', '2.0');
INSERT OR IGNORE INTO config(chave, valor) VALUES ('juros_dia_pct', '0.033');

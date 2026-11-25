ALTER TABLE movimentacoes ADD COLUMN reversao INTEGER NOT NULL DEFAULT 0
  CHECK (reversao IN (0,1));

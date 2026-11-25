-- Identificação explícita, sem inferir números dos endereços ou apelidos antigos.
ALTER TABLE imoveis ADD COLUMN numero TEXT NOT NULL DEFAULT '';
ALTER TABLE imoveis ADD COLUMN unidade TEXT NOT NULL DEFAULT '';

ALTER TABLE imoveis ADD COLUMN investimento_importado_c INTEGER NOT NULL DEFAULT 0;

UPDATE imoveis
SET investimento_importado_c = COALESCE((
  SELECT SUM(CASE WHEN m.reversao=1 THEN -r.valor_c ELSE r.valor_c END)
  FROM movimentacao_rateios r
  JOIN movimentacoes m ON m.id=r.movimentacao_id
  WHERE r.imovel_id=imoveis.id AND m.tipo='despesa' AND m.empreendimento_id IS NOT NULL
), 0);

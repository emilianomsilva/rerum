-- Ocupação passa a ser derivada de locações ativas, não um status manual.
UPDATE imoveis SET status = 'disponivel' WHERE status = 'ocupado';

CREATE TABLE IF NOT EXISTS import_batches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_name TEXT NOT NULL,
  sha256 TEXT NOT NULL UNIQUE,
  imported_at TEXT NOT NULL DEFAULT (datetime('now')),
  properties_count INTEGER NOT NULL DEFAULT 0,
  receipts_count INTEGER NOT NULL DEFAULT 0,
  expenses_count INTEGER NOT NULL DEFAULT 0,
  debts_count INTEGER NOT NULL DEFAULT 0,
  warnings_json TEXT NOT NULL DEFAULT '[]'
);

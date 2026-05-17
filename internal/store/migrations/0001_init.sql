-- 0001_init: items, accounts, transactions, sync_cursors + FTS5 over transactions.
--
-- Schema versioning is tracked via PRAGMA user_version. Each .sql file in this
-- directory is applied in lexical order; user_version is bumped to the highest
-- migration index after a clean apply. Migrations are append-only: never edit
-- an applied file.

CREATE TABLE IF NOT EXISTS items (
  id               TEXT PRIMARY KEY,
  provider         TEXT NOT NULL,
  env              TEXT NOT NULL,
  institution_id   TEXT,
  institution_name TEXT,
  added_at         TEXT NOT NULL,
  last_synced_at   TEXT
);

CREATE TABLE IF NOT EXISTS accounts (
  id                TEXT PRIMARY KEY,
  item_id           TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  name              TEXT NOT NULL,
  official_name     TEXT,
  mask              TEXT,
  type              TEXT,
  subtype           TEXT,
  currency          TEXT,
  balance           REAL,
  available_balance REAL,
  updated_at        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_accounts_item ON accounts(item_id);

CREATE TABLE IF NOT EXISTS transactions (
  id            TEXT PRIMARY KEY,
  item_id       TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  account_id    TEXT NOT NULL,
  date          TEXT NOT NULL,
  amount        REAL NOT NULL,
  currency      TEXT,
  name          TEXT,
  merchant_name TEXT,
  pending       INTEGER NOT NULL DEFAULT 0,
  category_json TEXT,
  updated_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tx_account_date ON transactions(account_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_tx_item_date    ON transactions(item_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_tx_date         ON transactions(date DESC);

-- FTS5 over the human-readable fields. content='transactions' keeps the index
-- in sync via triggers below; rebuild via `INSERT INTO transactions_fts(transactions_fts) VALUES('rebuild')`.
CREATE VIRTUAL TABLE IF NOT EXISTS transactions_fts USING fts5(
  name, merchant_name, category_json,
  content='transactions', content_rowid='rowid',
  tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS transactions_ai AFTER INSERT ON transactions BEGIN
  INSERT INTO transactions_fts(rowid, name, merchant_name, category_json)
  VALUES (new.rowid, new.name, new.merchant_name, new.category_json);
END;
CREATE TRIGGER IF NOT EXISTS transactions_ad AFTER DELETE ON transactions BEGIN
  INSERT INTO transactions_fts(transactions_fts, rowid, name, merchant_name, category_json)
  VALUES ('delete', old.rowid, old.name, old.merchant_name, old.category_json);
END;
CREATE TRIGGER IF NOT EXISTS transactions_au AFTER UPDATE ON transactions BEGIN
  INSERT INTO transactions_fts(transactions_fts, rowid, name, merchant_name, category_json)
  VALUES ('delete', old.rowid, old.name, old.merchant_name, old.category_json);
  INSERT INTO transactions_fts(rowid, name, merchant_name, category_json)
  VALUES (new.rowid, new.name, new.merchant_name, new.category_json);
END;

CREATE TABLE IF NOT EXISTS sync_cursors (
  item_id    TEXT NOT NULL,
  resource   TEXT NOT NULL,
  cursor     TEXT,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (item_id, resource)
);

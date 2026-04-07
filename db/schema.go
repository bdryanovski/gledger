package db

// Schema contains all CREATE TABLE and CREATE INDEX statements for the
// DoubleBook SQLite query cache.
const Schema = `
CREATE TABLE IF NOT EXISTS transactions (
    id          TEXT PRIMARY KEY,
    date        TEXT NOT NULL,
    description TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT '',
    comment     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS postings (
    id             TEXT PRIMARY KEY,
    transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    account        TEXT NOT NULL,
    amount         REAL NOT NULL,
    currency       TEXT NOT NULL DEFAULT 'USD',
    comment        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS transaction_tags (
    transaction_id TEXT NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
    key            TEXT NOT NULL,
    value          TEXT NOT NULL,
    PRIMARY KEY (transaction_id, key)
);

CREATE TABLE IF NOT EXISTS posting_tags (
    posting_id TEXT NOT NULL REFERENCES postings(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    PRIMARY KEY (posting_id, key)
);

CREATE TABLE IF NOT EXISTS exchange_rates (
    date          TEXT NOT NULL,
    from_currency TEXT NOT NULL,
    to_currency   TEXT NOT NULL,
    rate          REAL NOT NULL,
    PRIMARY KEY (date, from_currency, to_currency)
);

CREATE TABLE IF NOT EXISTS db_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes for FQL query performance.
CREATE INDEX IF NOT EXISTS idx_txn_date     ON transactions(date);
CREATE INDEX IF NOT EXISTS idx_post_txn     ON postings(transaction_id);
CREATE INDEX IF NOT EXISTS idx_post_account ON postings(account);
`

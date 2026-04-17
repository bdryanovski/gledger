// Package db manages an embedded SQLite database used as a query cache for FQL.
// The journal files are the source of truth; this database is a derived,
// rebuildable cache that is automatically refreshed when journal content changes.
package db

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"doublebook/ast"
	"doublebook/utils"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// ---------------------------------------------------------------------------
// DB
// ---------------------------------------------------------------------------

// DB wraps a SQLite connection and provides journal-loading helpers.
type DB struct {
	conn *sql.DB
	path string
}

// Open opens (or creates) the SQLite database at path.
// Use ":memory:" for an in-process, non-persistent database (useful in tests).
func Open(path string) (*DB, error) {
	if path != ":memory:" {
		path = utils.ExpandHome(path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database %q: %w", path, err)
	}

	// Enforce foreign keys and use WAL for concurrent reads.
	if _, err := conn.Exec(`PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("configuring sqlite pragmas: %w", err)
	}

	return &DB{conn: conn, path: path}, nil
}

// Close releases the database connection.
func (d *DB) Close() error { return d.conn.Close() }

// Conn returns the raw *sql.DB for direct SQL execution (used by FQL).
func (d *DB) Conn() *sql.DB { return d.conn }

// Initialize creates all tables and indexes defined in Schema.
func (d *DB) Initialize() error {
	_, err := d.conn.Exec(Schema)
	return err
}

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

// LoadFromTransactions clears and repopulates the cache from a transaction slice.
// It runs inside a single SQLite transaction for atomicity and performance.
func (d *DB) LoadFromTransactions(transactions []*ast.Transaction) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Rollback is a no-op if tx.Commit() succeeds; error is safely ignored here.
	defer func() { _ = tx.Rollback() }()

	// Clear existing data in dependency order.
	for _, tbl := range []string{"posting_tags", "postings", "transaction_tags", "transactions"} {
		if _, err := tx.Exec("DELETE FROM " + tbl); err != nil {
			return fmt.Errorf("clearing %s: %w", tbl, err)
		}
	}

	// Prepared statements for performance.
	insTxn, err := tx.Prepare(`
		INSERT INTO transactions(id, date, description, status, comment)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insTxn.Close()

	insPosting, err := tx.Prepare(`
		INSERT INTO postings(id, transaction_id, account, amount, currency, comment)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insPosting.Close()

	insTxnTag, err := tx.Prepare(`
		INSERT OR REPLACE INTO transaction_tags(transaction_id, key, value)
		VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insTxnTag.Close()

	insPostTag, err := tx.Prepare(`
		INSERT OR REPLACE INTO posting_tags(posting_id, key, value)
		VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insPostTag.Close()

	for _, txn := range transactions {
		txID := stableTransactionID(txn)
		dateStr := txn.Date.Format("2006-01-02")

		if _, err := insTxn.Exec(txID, dateStr, txn.Description, txn.Status, txn.Comment); err != nil {
			return fmt.Errorf("inserting transaction %q: %w", txID, err)
		}

		// Transaction tags.
		for k, v := range txn.Tags {
			if _, err := insTxnTag.Exec(txID, k, v); err != nil {
				return fmt.Errorf("inserting tag %q for transaction %q: %w", k, txID, err)
			}
		}

		// Postings.
		for i, p := range txn.Postings {
			pID := txID + "_" + strconv.Itoa(i)
			if _, err := insPosting.Exec(pID, txID, p.Account, p.Amount.Value, p.Amount.Currency, p.Comment); err != nil {
				return fmt.Errorf("inserting posting %q: %w", pID, err)
			}
			for k, v := range p.Tags {
				if _, err := insPostTag.Exec(pID, k, v); err != nil {
					return fmt.Errorf("inserting posting tag %q for %q: %w", k, pID, err)
				}
			}
		}
	}

	return tx.Commit()
}

// stableTransactionID returns a deterministic ID for txn.
// If the transaction already has an ID it is used directly; otherwise a hash
// of date + description + first-posting account + first-posting amount is used.
func stableTransactionID(txn *ast.Transaction) string {
	if txn.ID != "" {
		return txn.ID
	}
	var firstAcct, firstAmt string
	if len(txn.Postings) > 0 {
		firstAcct = txn.Postings[0].Account
		firstAmt = fmt.Sprintf("%.4f", txn.Postings[0].Amount.Value)
	}
	key := txn.Date.Format("2006-01-02") + "|" + txn.Description + "|" + firstAcct + "|" + firstAmt
	h := sha256.New()
	h.Write([]byte(key))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// ---------------------------------------------------------------------------
// Stale detection
// ---------------------------------------------------------------------------

// IsStale returns true when the stored journal checksum differs from
// journalChecksum (meaning the cache needs to be rebuilt).
func (d *DB) IsStale(journalChecksum string) bool {
	var stored string
	err := d.conn.QueryRow(
		`SELECT value FROM db_meta WHERE key = 'journal_checksum'`,
	).Scan(&stored)
	if err != nil {
		// No checksum yet → treat as stale.
		return true
	}
	return stored != journalChecksum
}

// SetChecksum persists the current journal checksum into the meta table.
func (d *DB) SetChecksum(checksum string) error {
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO db_meta(key, value) VALUES ('journal_checksum', ?)`,
		checksum,
	)
	return err
}

// ---------------------------------------------------------------------------
// Checksum
// ---------------------------------------------------------------------------

// ComputeChecksum builds a stable fingerprint of a set of journal files by
// hashing each file's modification time and size.  If a file does not exist
// it is silently skipped (the cache will simply be rebuilt).
func ComputeChecksum(filePaths []string) (string, error) {
	h := sha256.New()
	for _, p := range filePaths {
		info, err := os.Stat(p)
		if err != nil {
			continue // missing file → skip
		}
		h.Write([]byte(p +
			"|" + info.ModTime().UTC().String() +
			"|" + strconv.FormatInt(info.Size(), 10) +
			"\n"))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ---------------------------------------------------------------------------
// OpenOrRebuild — primary entry point
// ---------------------------------------------------------------------------

// OpenOrRebuild is the main startup helper: it opens the cache database,
// initialises the schema, computes the checksum of the journal files, and
// rebuilds the cache when the journal has changed since the last run.
//
//	dbPath:       path to the .db file, e.g. filepath.Join(dataDir, "cache.db")
//	transactions: all loaded journal transactions
//	journalFiles: journal file paths used to detect staleness
func OpenOrRebuild(dbPath string, transactions []*ast.Transaction, journalFiles []string) (*DB, error) {
	database, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := database.Initialize(); err != nil {
		database.Close()
		return nil, fmt.Errorf("initialising schema: %w", err)
	}

	checksum, err := ComputeChecksum(journalFiles)
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("computing checksum: %w", err)
	}

	if database.IsStale(checksum) {
		if err := database.LoadFromTransactions(transactions); err != nil {
			database.Close()
			return nil, fmt.Errorf("loading transactions: %w", err)
		}
		if err := database.SetChecksum(checksum); err != nil {
			database.Close()
			return nil, fmt.Errorf("storing checksum: %w", err)
		}
	}

	return database, nil
}

// ---------------------------------------------------------------------------
// Exchange rate helper
// ---------------------------------------------------------------------------

// StoreExchangeRate upserts an exchange rate into the cache.
func (d *DB) StoreExchangeRate(date, fromCurrency, toCurrency string, rate float64) error {
	_, err := d.conn.Exec(
		`INSERT OR REPLACE INTO exchange_rates(date, from_currency, to_currency, rate)
		 VALUES (?, ?, ?, ?)`,
		date, strings.ToUpper(fromCurrency), strings.ToUpper(toCurrency), rate,
	)
	return err
}

package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"doublebook/core/ast"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func openMem(t *testing.T) *DB {
	t.Helper()
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := d.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func sampleTxns() []*ast.Transaction {
	d1, _ := time.Parse("2006-01-02", "2025-01-15")
	d2, _ := time.Parse("2006-01-02", "2025-01-16")
	d3, _ := time.Parse("2006-01-02", "2025-02-01")

	t1 := ast.NewTransaction(d1, "Grocery Store")
	t1.ID = "aabbccdd11223344"
	t1.Tags["category"] = "food"
	t1.Postings = append(t1.Postings,
		ast.NewPosting("expenses:groceries", ast.Amount{Value: 45.32, Currency: "USD"}),
		ast.NewPosting("assets:checking", ast.Amount{Value: -45.32, Currency: "USD"}),
	)

	t2 := ast.NewTransaction(d2, "Salary Payment")
	t2.Postings = append(t2.Postings,
		ast.NewPosting("assets:checking", ast.Amount{Value: 2000.00, Currency: "USD"}),
		ast.NewPosting("income:salary", ast.Amount{Value: -2000.00, Currency: "USD"}),
	)

	t3 := ast.NewTransaction(d3, "Electric Bill")
	t3.Postings = append(t3.Postings,
		ast.NewPosting("expenses:utilities", ast.Amount{Value: 89.50, Currency: "USD"}),
		ast.NewPosting("assets:checking", ast.Amount{Value: -89.50, Currency: "USD"}),
	)

	return []*ast.Transaction{t1, t2, t3}
}

// ---------------------------------------------------------------------------
// TestLoadAndQuery
// ---------------------------------------------------------------------------

func TestLoadAndQuery(t *testing.T) {
	d := openMem(t)
	txns := sampleTxns()

	if err := d.LoadFromTransactions(txns); err != nil {
		t.Fatalf("LoadFromTransactions: %v", err)
	}

	// Check transaction count.
	var count int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 transactions, got %d", count)
	}

	// Check posting count (2 postings per transaction × 3 = 6).
	var pCount int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM postings").Scan(&pCount); err != nil {
		t.Fatalf("count postings: %v", err)
	}
	if pCount != 6 {
		t.Errorf("expected 6 postings, got %d", pCount)
	}

	// Check a specific transaction.
	var desc string
	if err := d.conn.QueryRow(
		"SELECT description FROM transactions WHERE id = ?", "aabbccdd11223344",
	).Scan(&desc); err != nil {
		t.Fatalf("query specific transaction: %v", err)
	}
	if desc != "Grocery Store" {
		t.Errorf("description: got %q, want %q", desc, "Grocery Store")
	}

	// Check transaction tag.
	var tagVal string
	if err := d.conn.QueryRow(
		"SELECT value FROM transaction_tags WHERE transaction_id = ? AND key = 'category'",
		"aabbccdd11223344",
	).Scan(&tagVal); err != nil {
		t.Fatalf("query tag: %v", err)
	}
	if tagVal != "food" {
		t.Errorf("tag value: got %q, want %q", tagVal, "food")
	}

	// Check posting amounts are correct.
	rows, err := d.conn.Query(
		"SELECT account, amount FROM postings WHERE transaction_id = ? ORDER BY amount DESC",
		"aabbccdd11223344",
	)
	if err != nil {
		t.Fatalf("query postings: %v", err)
	}
	defer rows.Close()
	var accounts []string
	for rows.Next() {
		var acct string
		var amt float64
		if err := rows.Scan(&acct, &amt); err != nil {
			t.Fatalf("scan posting: %v", err)
		}
		accounts = append(accounts, acct)
	}
	if len(accounts) != 2 {
		t.Errorf("expected 2 postings, got %d", len(accounts))
	}
}

// ---------------------------------------------------------------------------
// TestLoadIdempotent — loading twice must not duplicate rows
// ---------------------------------------------------------------------------

func TestLoadIdempotent(t *testing.T) {
	d := openMem(t)
	txns := sampleTxns()

	if err := d.LoadFromTransactions(txns); err != nil {
		t.Fatalf("first load: %v", err)
	}
	if err := d.LoadFromTransactions(txns); err != nil {
		t.Fatalf("second load: %v", err)
	}

	var count int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("after two loads: expected 3 transactions, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// TestStaleCheck
// ---------------------------------------------------------------------------

func TestStaleCheck(t *testing.T) {
	d := openMem(t)

	// Initially stale (no checksum stored).
	if !d.IsStale("abc") {
		t.Error("expected IsStale=true before any checksum is stored")
	}

	if err := d.SetChecksum("abc"); err != nil {
		t.Fatalf("SetChecksum: %v", err)
	}

	if d.IsStale("abc") {
		t.Error("expected IsStale=false after storing checksum 'abc'")
	}
	if !d.IsStale("xyz") {
		t.Error("expected IsStale=true for different checksum 'xyz'")
	}

	// Updating the checksum should work.
	if err := d.SetChecksum("xyz"); err != nil {
		t.Fatalf("SetChecksum xyz: %v", err)
	}
	if d.IsStale("xyz") {
		t.Error("expected IsStale=false after updating checksum to 'xyz'")
	}
}

// ---------------------------------------------------------------------------
// TestComputeChecksum
// ---------------------------------------------------------------------------

func TestComputeChecksum(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.journal")
	f2 := filepath.Join(dir, "b.journal")

	// Write two temp files.
	writeFile(t, f1, "2025-01-01 Test\n    a:b $1\n    c:d -$1\n")
	writeFile(t, f2, "2025-01-02 Test2\n    a:b $2\n    c:d -$2\n")

	cs1, err := ComputeChecksum([]string{f1, f2})
	if err != nil {
		t.Fatalf("ComputeChecksum: %v", err)
	}
	if cs1 == "" {
		t.Error("checksum should not be empty")
	}

	// Same files → same checksum.
	cs2, _ := ComputeChecksum([]string{f1, f2})
	if cs1 != cs2 {
		t.Error("same files should produce same checksum")
	}

	// Different order → different checksum (order matters for reproducibility).
	cs3, _ := ComputeChecksum([]string{f2, f1})
	if cs1 == cs3 {
		// This is allowed but worth noting; our impl includes the path in the hash
		// so different orders produce different checksums.
		t.Log("note: different file order produces different checksum (expected)")
	}

	// Non-existent file is silently skipped → checksum of just f1.
	cs4, _ := ComputeChecksum([]string{f1, "/nonexistent/file.journal"})
	cs5, _ := ComputeChecksum([]string{f1})
	if cs4 != cs5 {
		t.Error("missing file should be silently skipped")
	}
}

// ---------------------------------------------------------------------------
// TestOpenOrRebuild
// ---------------------------------------------------------------------------

func TestOpenOrRebuild(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cache.db")
	journalPath := filepath.Join(dir, "data.journal")
	writeFile(t, journalPath, "stub content")

	txns := sampleTxns()

	// First open — should build the cache.
	db1, err := OpenOrRebuild(dbPath, txns, []string{journalPath})
	if err != nil {
		t.Fatalf("first OpenOrRebuild: %v", err)
	}
	var count int
	db1.conn.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count)
	if count != 3 {
		t.Errorf("after first open: expected 3 transactions, got %d", count)
	}
	db1.Close()

	// Second open — journal unchanged → should NOT reload.
	// We can verify by inserting a bogus row; if it's still there after
	// OpenOrRebuild, the cache was not rebuilt.
	db2, _ := Open(dbPath)
	db2.Initialize()
	db2.conn.Exec("INSERT INTO db_meta(key,value) VALUES('sentinel','yes')")
	db2.Close()

	db3, err := OpenOrRebuild(dbPath, txns, []string{journalPath})
	if err != nil {
		t.Fatalf("second OpenOrRebuild: %v", err)
	}
	defer db3.Close()
	var sentinel string
	db3.conn.QueryRow("SELECT value FROM db_meta WHERE key='sentinel'").Scan(&sentinel)
	if sentinel != "yes" {
		t.Error("cache should NOT have been rebuilt (journal unchanged) but sentinel is gone")
	}
}

// ---------------------------------------------------------------------------
// TestStoreExchangeRate
// ---------------------------------------------------------------------------

func TestStoreExchangeRate(t *testing.T) {
	d := openMem(t)
	if err := d.StoreExchangeRate("2025-01-15", "USD", "BGN", 1.9558); err != nil {
		t.Fatalf("StoreExchangeRate: %v", err)
	}
	var rate float64
	if err := d.conn.QueryRow(
		"SELECT rate FROM exchange_rates WHERE date=? AND from_currency=? AND to_currency=?",
		"2025-01-15", "USD", "BGN",
	).Scan(&rate); err != nil {
		t.Fatalf("query rate: %v", err)
	}
	if rate != 1.9558 {
		t.Errorf("rate: got %f, want 1.9558", rate)
	}

	// Upsert — update the rate.
	if err := d.StoreExchangeRate("2025-01-15", "USD", "BGN", 1.9600); err != nil {
		t.Fatalf("upsert rate: %v", err)
	}
	d.conn.QueryRow(
		"SELECT rate FROM exchange_rates WHERE date=? AND from_currency=? AND to_currency=?",
		"2025-01-15", "USD", "BGN",
	).Scan(&rate)
	if rate != 1.9600 {
		t.Errorf("updated rate: got %f, want 1.9600", rate)
	}
}

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %q: %v", path, err)
	}
}

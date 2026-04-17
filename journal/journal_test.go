package journal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"doublebook/ast"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeJournal writes a minimal valid journal string to path.
func makeJournal(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// sampleTxn builds a balanced two-posting transaction.
func sampleTxn(dateStr, desc string, amount float64) *ast.Transaction {
	date, _ := time.Parse("2006-01-02", dateStr)
	txn := ast.NewTransaction(date, desc)
	txn.Postings = append(txn.Postings,
		ast.NewPosting("expenses:food", ast.Amount{Value: amount, Currency: "USD"}),
		ast.NewPosting("assets:cash", ast.Amount{Value: -amount, Currency: "USD"}),
	)
	return txn
}

const minimalJournal = `2025-01-15 Test
    expenses:food    $10.00
    assets:cash     -$10.00
`

// ---------------------------------------------------------------------------
// Resolve tests
// ---------------------------------------------------------------------------

func TestResolve_NoFiles(t *testing.T) {
	dir := t.TempDir()
	paths := Resolve("personal", dir)
	if len(paths) != 0 {
		t.Errorf("expected empty slice, got %v", paths)
	}
}

func TestResolve_SingleFile(t *testing.T) {
	dir := t.TempDir()
	makeJournal(t, filepath.Join(dir, "personal.journal"), minimalJournal)

	paths := Resolve("personal", dir)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if !strings.HasSuffix(paths[0], "personal.journal") {
		t.Errorf("path %q does not end with personal.journal", paths[0])
	}
}

func TestResolve_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	makeJournal(t, filepath.Join(dir, "personal.journal"), minimalJournal)
	makeJournal(t, filepath.Join(dir, "personal.1.journal"), minimalJournal)
	makeJournal(t, filepath.Join(dir, "personal.2.journal"), minimalJournal)
	// personal.4.journal exists but personal.3.journal does NOT → stop at 2.
	makeJournal(t, filepath.Join(dir, "personal.4.journal"), minimalJournal)

	paths := Resolve("personal", dir)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths (stopping at gap), got %d: %v", len(paths), paths)
	}
	for i, want := range []string{"personal.journal", "personal.1.journal", "personal.2.journal"} {
		if !strings.HasSuffix(paths[i], want) {
			t.Errorf("paths[%d] = %q, want suffix %q", i, paths[i], want)
		}
	}
}

func TestResolve_ExactPath(t *testing.T) {
	dir := t.TempDir()
	exact := filepath.Join(dir, "my.journal")
	makeJournal(t, exact, minimalJournal)

	paths := Resolve(exact, dir)
	if len(paths) != 1 || paths[0] != exact {
		t.Errorf("expected [%q], got %v", exact, paths)
	}
}

func TestResolve_MissingExactPath(t *testing.T) {
	dir := t.TempDir()
	paths := Resolve(filepath.Join(dir, "nonexistent.journal"), dir)
	if len(paths) != 0 {
		t.Errorf("expected empty, got %v", paths)
	}
}

// ---------------------------------------------------------------------------
// Load tests
// ---------------------------------------------------------------------------

func TestLoad_SingleFile(t *testing.T) {
	dir := t.TempDir()
	makeJournal(t, filepath.Join(dir, "data.journal"), minimalJournal)

	txns, err := Load("data", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(txns))
	}
}

func TestLoad_NoFiles_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	txns, err := Load("data", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(txns))
	}
}

func TestLoad_MergesAndSorts(t *testing.T) {
	dir := t.TempDir()

	// File 1 has a later date.
	makeJournal(t, filepath.Join(dir, "data.journal"), `2025-02-01 Later
    expenses:food    $20.00
    assets:cash     -$20.00
`)
	// File 2 has an earlier date.
	makeJournal(t, filepath.Join(dir, "data.1.journal"), `2025-01-15 Earlier
    expenses:food    $10.00
    assets:cash     -$10.00
`)

	txns, err := Load("data", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txns))
	}
	// Must be sorted: Earlier first, Later second.
	if txns[0].Description != "Earlier" {
		t.Errorf("txns[0] should be 'Earlier', got %q", txns[0].Description)
	}
	if txns[1].Description != "Later" {
		t.Errorf("txns[1] should be 'Later', got %q", txns[1].Description)
	}
}

func TestLoadFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.journal")
	makeJournal(t, path, minimalJournal)

	txns, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(txns))
	}
}

// ---------------------------------------------------------------------------
// WriteAll tests
// ---------------------------------------------------------------------------

func TestWriteAll_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.journal")

	txn := sampleTxn("2025-01-15", "Test", 42.00)
	if err := WriteAll(path, []*ast.Transaction{txn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Round-trip: parse the written file.
	txns, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("expected 1 transaction after round-trip, got %d", len(txns))
	}
	if txns[0].Description != "Test" {
		t.Errorf("description: got %q, want Test", txns[0].Description)
	}
}

func TestWriteAll_Overwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.journal")

	// Write once with 2 transactions.
	makeJournal(t, path, minimalJournal+"\n"+minimalJournal)

	// Overwrite with 1 transaction.
	txn := sampleTxn("2025-03-01", "Single", 5.00)
	if err := WriteAll(path, []*ast.Transaction{txn}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	txns, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("expected 1 transaction after overwrite, got %d", len(txns))
	}
}

// ---------------------------------------------------------------------------
// AppendTransaction tests
// ---------------------------------------------------------------------------

func TestAppendTransaction_CreatesBaseFile(t *testing.T) {
	dir := t.TempDir()
	txn := sampleTxn("2025-01-15", "First", 10.00)

	if err := AppendTransaction("data", dir, txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	base := filepath.Join(dir, "data.journal")
	if _, err := os.Stat(base); err != nil {
		t.Errorf("expected base file %q to exist: %v", base, err)
	}
}

func TestAppendTransaction_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	makeJournal(t, filepath.Join(dir, "data.journal"), minimalJournal)

	txn := sampleTxn("2025-02-01", "Second", 20.00)
	if err := AppendTransaction("data", dir, txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	txns, err := Load("data", dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(txns) != 2 {
		t.Errorf("expected 2 transactions, got %d", len(txns))
	}
}

func TestAppendTransaction_SplitsAtLimit(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "data.journal")

	// Create a file that is nearly at the size limit.
	padding := strings.Repeat("x", MaxJournalSize-50)
	// Write raw padding as a comment so the file "exists" at near-limit size.
	if err := os.WriteFile(base, []byte("; "+padding+"\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	txn := sampleTxn("2025-01-15", "Overflow", 10.00)
	if err := AppendTransaction("data", dir, txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The transaction should have been written to the OVERFLOW file, not the base.
	overflow := filepath.Join(dir, "data.1.journal")
	if _, err := os.Stat(overflow); err != nil {
		t.Errorf("expected overflow file %q to be created: %v", overflow, err)
	}

	// The base file must NOT have grown (no transaction appended to it).
	info, _ := os.Stat(base)
	if info.Size() > int64(MaxJournalSize) {
		t.Errorf("base file grew beyond limit: %d bytes", info.Size())
	}
}

func TestAppendTransaction_MultipleOverflows(t *testing.T) {
	dir := t.TempDir()

	// Set up: base + overflow 1 both nearly full.
	padding := strings.Repeat("x", MaxJournalSize-50)
	for _, name := range []string{"data.journal", "data.1.journal"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("; "+padding+"\n"), 0644); err != nil {
			t.Fatalf("setup %q: %v", name, err)
		}
	}

	txn := sampleTxn("2025-01-15", "Overflow2", 10.00)
	if err := AppendTransaction("data", dir, txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	overflow2 := filepath.Join(dir, "data.2.journal")
	if _, err := os.Stat(overflow2); err != nil {
		t.Errorf("expected data.2.journal to be created: %v", err)
	}
}

package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// writeCSV writes content to a temp file and returns its path.
func writeCSV(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.csv")
	if err != nil {
		t.Fatalf("create temp csv: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	f.Close()
	return f.Name()
}

// minimalImportMap returns an ImportMap configured for the test CSVs.
func minimalImportMap(debitCol, creditCol, descCol, refCol int) *ImportMap {
	m := &ImportMap{
		Name:                 "test-bank",
		Delimiter:            ",",
		Encoding:             "utf-8",
		SkipLines:            1,
		DateFormat:           "2006-01-02",
		SourceAccount:        "assets:checking:test",
		DefaultDebitAccount:  "expenses:unknown",
		DefaultCreditAccount: "income:unknown",
		Currency:             "USD",
		Columns: ColumnMap{
			Date:         0,
			DebitAmount:  intPtr(debitCol),
			CreditAmount: intPtr(creditCol),
			Description:  intPtr(descCol),
			Reference:    intPtr(refCol),
		},
	}
	return m
}

func intPtr(v int) *int { return &v }

// ---------------------------------------------------------------------------
// TestImportCSV_BasicUTF8
// ---------------------------------------------------------------------------

func TestImportCSV_BasicUTF8(t *testing.T) {
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,45.32,,Grocery Store,REF001\n"+
		"2025-01-16,,2000.00,Salary,REF002\n")

	imap := minimalImportMap(1, 2, 3, 4)
	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Imported) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(result.Imported))
	}
	if result.TotalRows != 2 {
		t.Errorf("TotalRows: got %d, want 2", result.TotalRows)
	}
	if len(result.Errors) != 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_BalancedTransactions
// ---------------------------------------------------------------------------

func TestImportCSV_BalancedTransactions(t *testing.T) {
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,45.32,,Groceries,REF001\n")

	imap := minimalImportMap(1, 2, 3, 4)
	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Imported) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(result.Imported))
	}
	txn := result.Imported[0]
	if !txn.IsBalanced() {
		t.Errorf("transaction is not balanced: balance=%.4f", txn.Balance())
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_HasID
// ---------------------------------------------------------------------------

func TestImportCSV_HasID(t *testing.T) {
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,45.32,,Groceries,UNIQUE-REF-001\n")

	imap := minimalImportMap(1, 2, 3, 4)
	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Imported) != 1 {
		t.Fatalf("expected 1 transaction")
	}
	if result.Imported[0].ID == "" {
		t.Error("transaction should have a non-empty ID")
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_Deduplication — re-import same file skips all rows
// ---------------------------------------------------------------------------

func TestImportCSV_Deduplication(t *testing.T) {
	csvContent := "date,debit,credit,desc,ref\n" +
		"2025-01-15,45.32,,Groceries,REF001\n" +
		"2025-01-16,89.50,,Electric Bill,REF002\n"

	csvPath := writeCSV(t, csvContent)
	imap := minimalImportMap(1, 2, 3, 4)

	// First import.
	first, err := ImportCSV(csvPath, imap, nil)
	if err != nil {
		t.Fatalf("first import error: %v", err)
	}
	if len(first.Imported) != 2 {
		t.Fatalf("first import: expected 2, got %d", len(first.Imported))
	}

	// Build existingIDs from the first result.
	existingIDs := make(map[string]bool)
	for _, txn := range first.Imported {
		existingIDs[txn.ID] = true
	}

	// Second import with the existing IDs.
	second, err := ImportCSV(csvPath, imap, existingIDs)
	if err != nil {
		t.Fatalf("second import error: %v", err)
	}
	if len(second.Imported) != 0 {
		t.Errorf("second import: expected 0 new transactions, got %d", len(second.Imported))
	}
	if second.Skipped != 2 {
		t.Errorf("second import: expected 2 skipped, got %d", second.Skipped)
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_DuplicatesWithinSameFile
// ---------------------------------------------------------------------------

func TestImportCSV_DuplicatesWithinSameFile(t *testing.T) {
	// Two rows with the same reference → only one should be imported.
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,45.32,,Groceries,SAME-REF\n"+
		"2025-01-15,45.32,,Groceries,SAME-REF\n")

	imap := minimalImportMap(1, 2, 3, 4)
	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Imported) != 1 {
		t.Errorf("expected 1 unique transaction, got %d", len(result.Imported))
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_WithTransforms
// ---------------------------------------------------------------------------

func TestImportCSV_WithTransforms(t *testing.T) {
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,73.48,,BILLA 127,REF001\n"+
		"2025-01-16,89.50,,Electric,REF002\n")

	imap := minimalImportMap(1, 2, 3, 4)
	imap.Transforms = []Transform{
		{
			DescriptionContains: "BILLA",
			DebitAccount:        "expenses:groceries",
			Tags:                map[string]string{"merchant": "billa"},
		},
	}

	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Imported) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(result.Imported))
	}

	// First transaction (BILLA) should use expenses:groceries.
	billaFound := false
	for _, txn := range result.Imported {
		for _, p := range txn.Postings {
			if p.Account == "expenses:groceries" {
				billaFound = true
			}
		}
	}
	if !billaFound {
		t.Error("expected transform to set debit account to 'expenses:groceries'")
	}

	// Second transaction should use the default debit account.
	defaultFound := false
	for _, txn := range result.Imported {
		if txn.Description == "Electric" {
			for _, p := range txn.Postings {
				if p.Account == "expenses:unknown" {
					defaultFound = true
				}
			}
		}
	}
	if !defaultFound {
		t.Error("expected non-matching row to use default debit account")
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_EmptyAmountSkipped
// ---------------------------------------------------------------------------

func TestImportCSV_EmptyAmountSkipped(t *testing.T) {
	// Row with no amount in either debit or credit column is silently skipped.
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,,,No amount here,REF001\n"+
		"2025-01-16,45.32,,Has amount,REF002\n")

	imap := minimalImportMap(1, 2, 3, 4)
	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only 1 should be imported; the empty-amount row is silently dropped.
	if len(result.Imported) != 1 {
		t.Errorf("expected 1 transaction (skipped empty-amount row), got %d", len(result.Imported))
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_SingleAmountColumn
// ---------------------------------------------------------------------------

func TestImportCSV_SingleAmountColumn(t *testing.T) {
	csv := writeCSV(t, "date,amount,desc,ref\n"+
		"2025-01-15,-45.32,Expense,REF001\n"+ // negative = debit
		"2025-01-16,2000.00,Income,REF002\n") // positive = credit

	col1 := 1
	imap := &ImportMap{
		Name:                 "test",
		Delimiter:            ",",
		Encoding:             "utf-8",
		SkipLines:            1,
		DateFormat:           "2006-01-02",
		SourceAccount:        "assets:checking",
		DefaultDebitAccount:  "expenses:unknown",
		DefaultCreditAccount: "income:unknown",
		Currency:             "USD",
		Columns: ColumnMap{
			Date:        0,
			Amount:      &col1,
			Description: intPtr(2),
			Reference:   intPtr(3),
		},
	}

	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Imported) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(result.Imported))
	}
	for _, txn := range result.Imported {
		if !txn.IsBalanced() {
			t.Errorf("transaction %q is not balanced", txn.Description)
		}
	}
}

// ---------------------------------------------------------------------------
// TestImportCSV_SampleFile — integration test with the real sample CSV
// ---------------------------------------------------------------------------

func TestImportCSV_SampleFile(t *testing.T) {
	sampleCSV := filepath.Join("..", "sample", "sample.csv")
	sampleMap := filepath.Join("..", "sample", "sample.importmap.json")

	// Skip if sample files are not present.
	if _, err := os.Stat(sampleCSV); err != nil {
		t.Skipf("sample.csv not found: %v", err)
	}
	if _, err := os.Stat(sampleMap); err != nil {
		t.Skipf("sample.importmap.json not found: %v", err)
	}

	imap, err := LoadImportMap(sampleMap)
	if err != nil {
		t.Fatalf("loading importmap: %v", err)
	}

	result, err := ImportCSV(sampleCSV, imap, nil)
	if err != nil {
		t.Fatalf("import error: %v", err)
	}

	if len(result.Imported) == 0 {
		t.Error("expected > 0 imported transactions from sample.csv")
	}

	// All transactions must be balanced.
	unbalanced := 0
	for _, txn := range result.Imported {
		if !txn.IsBalanced() {
			unbalanced++
			t.Logf("unbalanced: %s %s balance=%.4f", txn.Date.Format("2006-01-02"), txn.Description, txn.Balance())
		}
	}
	if unbalanced > 0 {
		t.Errorf("%d unbalanced transactions out of %d", unbalanced, len(result.Imported))
	}

	// All transactions must have an ID.
	noID := 0
	for _, txn := range result.Imported {
		if txn.ID == "" {
			noID++
		}
	}
	if noID > 0 {
		t.Errorf("%d transactions without an ID", noID)
	}

	t.Logf("sample.csv: imported=%d skipped=%d errors=%d total=%d",
		len(result.Imported), result.Skipped, len(result.Errors), result.TotalRows)
}

// ---------------------------------------------------------------------------
// TestExtractIDs
// ---------------------------------------------------------------------------

func TestExtractIDs(t *testing.T) {
	csv := writeCSV(t, "date,debit,credit,desc,ref\n"+
		"2025-01-15,45.32,,A,REF001\n"+
		"2025-01-16,89.50,,B,REF002\n")

	imap := minimalImportMap(1, 2, 3, 4)
	result, err := ImportCSV(csv, imap, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := ExtractIDs(result.Imported)
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}
	for _, txn := range result.Imported {
		if !ids[txn.ID] {
			t.Errorf("ID %q not found in extracted set", txn.ID)
		}
	}
}

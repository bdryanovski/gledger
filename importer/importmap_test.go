package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeJSON(t *testing.T, dir string, name string, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_Valid — fully populated map
// ---------------------------------------------------------------------------

func TestLoadImportMap_Valid(t *testing.T) {
	dir := t.TempDir()
	debitCol := 2
	creditCol := 4
	descCol := 7
	refCol := 12

	raw := map[string]interface{}{
		"name":                   "test-bank",
		"delimiter":              "|",
		"encoding":               "utf-8",
		"skip_lines":             1,
		"date_format":            "02/01/2006",
		"source_account":         "assets:checking:test",
		"default_debit_account":  "expenses:unknown",
		"default_credit_account": "income:unknown",
		"currency":               "BGN",
		"columns": map[string]interface{}{
			"date":          0,
			"debit_amount":  debitCol,
			"credit_amount": creditCol,
			"description":   descCol,
			"reference":     refCol,
		},
	}
	p := writeJSON(t, dir, "valid.importmap.json", raw)

	m, err := LoadImportMap(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Name != "test-bank" {
		t.Errorf("Name: got %q, want %q", m.Name, "test-bank")
	}
	if m.Delimiter != "|" {
		t.Errorf("Delimiter: got %q", m.Delimiter)
	}
	if m.Columns.Date != 0 {
		t.Errorf("columns.date: got %d, want 0", m.Columns.Date)
	}
	if ColIdx(m.Columns.DebitAmount) != debitCol {
		t.Errorf("columns.debit_amount: got %d, want %d", ColIdx(m.Columns.DebitAmount), debitCol)
	}
	if ColIdx(m.Columns.CreditAmount) != creditCol {
		t.Errorf("columns.credit_amount: got %d", ColIdx(m.Columns.CreditAmount))
	}
	if ColIdx(m.Columns.Description) != descCol {
		t.Errorf("columns.description: got %d", ColIdx(m.Columns.Description))
	}
	if ColIdx(m.Columns.Reference) != refCol {
		t.Errorf("columns.reference: got %d", ColIdx(m.Columns.Reference))
	}
	if m.Currency != "BGN" {
		t.Errorf("Currency: got %q", m.Currency)
	}
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_Defaults — minimal map, defaults filled in
// ---------------------------------------------------------------------------

func TestLoadImportMap_Defaults(t *testing.T) {
	dir := t.TempDir()
	amtCol := 1
	raw := map[string]interface{}{
		"source_account": "assets:checking:mybank",
		"columns": map[string]interface{}{
			"date":   0,
			"amount": amtCol,
		},
	}
	p := writeJSON(t, dir, "minimal.importmap.json", raw)

	m, err := LoadImportMap(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Defaults must be applied.
	if m.Delimiter != "," {
		t.Errorf("default Delimiter: got %q, want \",\"", m.Delimiter)
	}
	if m.Encoding != "utf-8" {
		t.Errorf("default Encoding: got %q", m.Encoding)
	}
	if m.SkipLines != 1 {
		t.Errorf("default SkipLines: got %d, want 1", m.SkipLines)
	}
	if m.DateFormat != "2006-01-02" {
		t.Errorf("default DateFormat: got %q", m.DateFormat)
	}
	if m.DefaultDebitAccount != "expenses:unknown" {
		t.Errorf("default DefaultDebitAccount: got %q", m.DefaultDebitAccount)
	}
	if m.DefaultCreditAccount != "income:unknown" {
		t.Errorf("default DefaultCreditAccount: got %q", m.DefaultCreditAccount)
	}
	if m.Currency != "USD" {
		t.Errorf("default Currency: got %q", m.Currency)
	}
	if ColIdx(m.Columns.Amount) != amtCol {
		t.Errorf("columns.amount: got %d, want %d", ColIdx(m.Columns.Amount), amtCol)
	}
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_InvalidMissingSourceAccount
// ---------------------------------------------------------------------------

func TestLoadImportMap_InvalidMissingSourceAccount(t *testing.T) {
	dir := t.TempDir()
	amtCol := 1
	raw := map[string]interface{}{
		// source_account intentionally omitted
		"columns": map[string]interface{}{
			"date":   0,
			"amount": amtCol,
		},
	}
	p := writeJSON(t, dir, "no_source.importmap.json", raw)
	_, err := LoadImportMap(p)
	if err == nil {
		t.Fatal("expected error for missing source_account, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_InvalidNoAmountColumn
// ---------------------------------------------------------------------------

func TestLoadImportMap_InvalidNoAmountColumn(t *testing.T) {
	dir := t.TempDir()
	raw := map[string]interface{}{
		"source_account": "assets:checking",
		"columns": map[string]interface{}{
			"date": 0,
			// No amount, debit_amount, or credit_amount
		},
	}
	p := writeJSON(t, dir, "no_amount.importmap.json", raw)
	_, err := LoadImportMap(p)
	if err == nil {
		t.Fatal("expected error for missing amount columns, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_Transforms
// ---------------------------------------------------------------------------

func TestLoadImportMap_Transforms(t *testing.T) {
	dir := t.TempDir()
	amtCol := 2
	raw := map[string]interface{}{
		"source_account": "assets:checking",
		"columns":        map[string]interface{}{"date": 0, "amount": amtCol},
		"transforms": []map[string]interface{}{
			{
				"description_contains": "BILLA",
				"debit_account":        "expenses:groceries",
				"tags":                 map[string]string{"merchant": "billa"},
			},
		},
	}
	p := writeJSON(t, dir, "transforms.importmap.json", raw)
	m, err := LoadImportMap(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Transforms) != 1 {
		t.Fatalf("expected 1 transform, got %d", len(m.Transforms))
	}
	tr := m.Transforms[0]
	if tr.DescriptionContains != "BILLA" {
		t.Errorf("transform description_contains: got %q", tr.DescriptionContains)
	}
	if tr.DebitAccount != "expenses:groceries" {
		t.Errorf("transform debit_account: got %q", tr.DebitAccount)
	}
	if tr.Tags["merchant"] != "billa" {
		t.Errorf("transform tags[merchant]: got %q", tr.Tags["merchant"])
	}
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_MissingFile
// ---------------------------------------------------------------------------

func TestLoadImportMap_MissingFile(t *testing.T) {
	_, err := LoadImportMap("/nonexistent/path/to.importmap.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestLoadImportMap_InvalidJSON
// ---------------------------------------------------------------------------

func TestLoadImportMap_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.importmap.json")
	if err := os.WriteFile(p, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadImportMap(p)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestColIdx helper
// ---------------------------------------------------------------------------

func TestColIdx(t *testing.T) {
	if ColIdx(nil) != -1 {
		t.Error("ColIdx(nil) should be -1")
	}
	v := 5
	if ColIdx(&v) != 5 {
		t.Error("ColIdx(&5) should be 5")
	}
	zero := 0
	if ColIdx(&zero) != 0 {
		t.Error("ColIdx(&0) should be 0 (not -1)")
	}
}

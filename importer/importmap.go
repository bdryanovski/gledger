// Package importer handles CSV import: importmap configuration files,
// CSV parsing with encoding conversion, and deduplication.
package importer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"doublebook/utils"
)

// ---------------------------------------------------------------------------
// ImportMap — top-level configuration
// ---------------------------------------------------------------------------

// ImportMap describes how to parse a specific bank's CSV export format and
// map its columns to DoubleBook journal entries.
type ImportMap struct {
	// Identity
	Name string `json:"name"` // human-readable label, e.g. "my-bank"

	// CSV format
	Delimiter  string `json:"delimiter"`   // field delimiter, default ","
	Encoding   string `json:"encoding"`    // source encoding, default "utf-8"
	SkipLines  int    `json:"skip_lines"`  // header lines to skip, default 1
	DateFormat string `json:"date_format"` // Go time format, e.g. "02/01/2006"
	TimeColumn *int   `json:"time_column"` // optional column index for time-of-day
	TimeFormat string `json:"time_format"` // Go time format for time, e.g. "15:04"

	// Column mapping
	Columns ColumnMap `json:"columns"`

	// Account defaults
	SourceAccount        string `json:"source_account"`         // the bank account, e.g. "assets:checking:mybank"
	DefaultDebitAccount  string `json:"default_debit_account"`  // where expenses go, e.g. "expenses:unknown"
	DefaultCreditAccount string `json:"default_credit_account"` // where income goes,   e.g. "income:unknown"

	// Currency
	Currency string `json:"currency"` // default currency when not in CSV, e.g. "BGN"

	// Auto-categorisation rules applied after parsing
	Transforms []Transform `json:"transforms"`
}

// ---------------------------------------------------------------------------
// ColumnMap — maps CSV column indices to journal fields
// ---------------------------------------------------------------------------

// ColumnMap specifies which CSV column index (0-based) corresponds to each
// journal field.  Pointer fields are nil when that column is absent in the CSV.
type ColumnMap struct {
	// Required: column containing the transaction date.
	Date int `json:"date"`

	// Amounts — use DebitAmount+CreditAmount OR Amount (not both).
	//   DebitAmount:  positive value in a "money out" column (expenses).
	//   CreditAmount: positive value in a "money in" column (income).
	//   Amount:       single signed column (negative = debit, positive = credit).
	DebitAmount  *int `json:"debit_amount"`
	CreditAmount *int `json:"credit_amount"`
	Amount       *int `json:"amount"`

	// Optional metadata columns.
	Description *int `json:"description"` // merchant / narration text
	Reference   *int `json:"reference"`   // bank's unique transaction ID
	Currency    *int `json:"currency"`    // per-row currency code
	Category    *int `json:"category"`    // bank's own category label (informational)
	Balance     *int `json:"balance"`     // running balance (informational, not imported)
}

// HasDebitCredit reports whether separate debit/credit columns are defined.
func (c *ColumnMap) HasDebitCredit() bool {
	return c.DebitAmount != nil || c.CreditAmount != nil
}

// HasAmount reports whether a single signed amount column is defined.
func (c *ColumnMap) HasAmount() bool { return c.Amount != nil }

// ColIdx returns the integer value of an optional column pointer, or -1 if nil.
func ColIdx(p *int) int {
	if p == nil {
		return -1
	}
	return *p
}

// ---------------------------------------------------------------------------
// Transform — auto-categorisation rule
// ---------------------------------------------------------------------------

// Transform is a rule applied to each imported row.  All specified match
// conditions must be true for the overrides to be applied.
type Transform struct {
	// Match conditions (all must be satisfied simultaneously).
	DescriptionContains string   `json:"description_contains"`
	AmountMin           *float64 `json:"amount_min"`
	AmountMax           *float64 `json:"amount_max"`

	// Overrides applied when the row matches.
	DebitAccount  string            `json:"debit_account"`  // override DefaultDebitAccount
	CreditAccount string            `json:"credit_account"` // override DefaultCreditAccount
	Tags          map[string]string `json:"tags"`           // extra tags to attach
	Category      string            `json:"category"`       // sets tags["category"]
}

// ---------------------------------------------------------------------------
// LoadImportMap
// ---------------------------------------------------------------------------

// LoadImportMap reads the importmap.json at path, applies defaults, validates,
// and returns the ready-to-use ImportMap.
func LoadImportMap(path string) (*ImportMap, error) {
	path = utils.ExpandHome(path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading importmap %q: %w", path, err)
	}

	var m ImportMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing importmap %q: %w", path, err)
	}

	applyDefaults(&m)

	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid importmap %q: %w", path, err)
	}

	return &m, nil
}

// applyDefaults fills in zero-value fields with sensible defaults.
func applyDefaults(m *ImportMap) {
	if m.Delimiter == "" {
		m.Delimiter = ","
	}
	if m.Encoding == "" {
		m.Encoding = "utf-8"
	}
	if m.SkipLines == 0 {
		m.SkipLines = 1
	}
	if m.DateFormat == "" {
		m.DateFormat = "2006-01-02"
	}
	if m.DefaultDebitAccount == "" {
		m.DefaultDebitAccount = "expenses:unknown"
	}
	if m.DefaultCreditAccount == "" {
		m.DefaultCreditAccount = "income:unknown"
	}
	if m.Currency == "" {
		m.Currency = "USD"
	}
	if m.Name == "" {
		m.Name = "unnamed"
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

// Validate checks required fields and returns a descriptive error when
// something is missing or inconsistent.
func (m *ImportMap) Validate() error {
	var errs []string

	// source_account is mandatory.
	if strings.TrimSpace(m.SourceAccount) == "" {
		errs = append(errs, "source_account is required")
	}

	// columns.date must be a valid (non-negative) index.
	if m.Columns.Date < 0 {
		errs = append(errs, "columns.date must be ≥ 0")
	}

	// Must have at least one way to read an amount.
	if !m.Columns.HasAmount() && !m.Columns.HasDebitCredit() {
		errs = append(errs, "must specify either columns.amount or at least one of columns.debit_amount / columns.credit_amount")
	}

	// delimiter must be a single character (multi-byte accepted for e.g. tab).
	if len([]rune(m.Delimiter)) == 0 {
		errs = append(errs, "delimiter cannot be empty")
	}

	// Warn about unknown encodings.
	switch strings.ToLower(m.Encoding) {
	case "utf-8", "utf8",
		"windows-1251", "cp1251", "win1251",
		"windows-1252", "cp1252",
		"iso-8859-1", "latin-1", "iso8859-1":
		// OK
	default:
		errs = append(errs, fmt.Sprintf("unsupported encoding %q (supported: utf-8, windows-1251, windows-1252, iso-8859-1)", m.Encoding))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

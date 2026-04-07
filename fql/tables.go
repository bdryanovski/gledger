package fql

import "strings"

// ---------------------------------------------------------------------------
// VirtualTable / VirtualColumn
// ---------------------------------------------------------------------------

// VirtualTable defines a named table that FQL queries can SELECT from.
// Its SQL field is an inner SELECT that produces the table's rows from the
// underlying SQLite cache.
type VirtualTable struct {
	Name    string
	Columns []VirtualColumn
	SQL     string // inner SELECT wrapped in a subquery by the compiler
}

// VirtualColumn describes one column of a virtual table.
type VirtualColumn struct {
	Name        string
	Type        string // "text" | "real" | "integer"
	Description string
}

// HasColumn returns true when the table exposes a column with the given name
// (case-insensitive).
func (vt *VirtualTable) HasColumn(name string) bool {
	lower := strings.ToLower(name)
	for _, c := range vt.Columns {
		if strings.ToLower(c.Name) == lower {
			return true
		}
	}
	return false
}

// ColumnNames returns all column names as a slice.
func (vt *VirtualTable) ColumnNames() []string {
	out := make([]string, len(vt.Columns))
	for i, c := range vt.Columns {
		out[i] = c.Name
	}
	return out
}

// ---------------------------------------------------------------------------
// Tables registry
// ---------------------------------------------------------------------------

// Tables is the registry of all virtual tables available to FQL queries.
// Keys are lowercase table names.
var Tables = map[string]*VirtualTable{
	"transactions": {
		Name: "transactions",
		Columns: []VirtualColumn{
			{Name: "id", Type: "text", Description: "Unique transaction identifier"},
			{Name: "date", Type: "text", Description: "Transaction date (YYYY-MM-DD)"},
			{Name: "description", Type: "text", Description: "Transaction description"},
			{Name: "status", Type: "text", Description: "Status: '' uncleared, '!' pending, '*' cleared"},
			{Name: "account", Type: "text", Description: "Posting account name"},
			{Name: "amount", Type: "real", Description: "Posting amount (negative = credit)"},
			{Name: "currency", Type: "text", Description: "Amount currency code"},
			{Name: "tags", Type: "text", Description: "Semicolon-separated key=value tag pairs"},
		},
		SQL: `
SELECT
    t.id,
    t.date,
    t.description,
    t.status,
    p.account,
    p.amount,
    p.currency,
    COALESCE(
        (SELECT GROUP_CONCAT(key || '=' || value, ';')
         FROM transaction_tags WHERE transaction_id = t.id),
        ''
    ) AS tags
FROM transactions t
JOIN postings p ON p.transaction_id = t.id`,
	},

	"accounts": {
		Name: "accounts",
		Columns: []VirtualColumn{
			{Name: "name", Type: "text", Description: "Full account name"},
			{Name: "type", Type: "text", Description: "Account type: asset/liability/equity/income/expense/other"},
			{Name: "transaction_count", Type: "integer", Description: "Number of postings"},
			{Name: "total_amount", Type: "real", Description: "Sum of all posting amounts"},
			{Name: "last_transaction", Type: "text", Description: "Date of most recent transaction"},
			{Name: "first_transaction", Type: "text", Description: "Date of earliest transaction"},
		},
		SQL: `
SELECT
    p.account AS name,
    CASE
        WHEN p.account LIKE 'assets%'      THEN 'asset'
        WHEN p.account LIKE 'liabilities%' THEN 'liability'
        WHEN p.account LIKE 'equity%'      THEN 'equity'
        WHEN p.account LIKE 'income%'      THEN 'income'
        WHEN p.account LIKE 'expenses%'    THEN 'expense'
        ELSE 'other'
    END AS type,
    COUNT(*) AS transaction_count,
    SUM(p.amount) AS total_amount,
    MAX(t.date) AS last_transaction,
    MIN(t.date) AS first_transaction
FROM postings p
JOIN transactions t ON t.id = p.transaction_id
GROUP BY p.account`,
	},

	"spending": {
		Name: "spending",
		Columns: []VirtualColumn{
			{Name: "date", Type: "text", Description: "Transaction date"},
			{Name: "month", Type: "text", Description: "Year-month (YYYY-MM)"},
			{Name: "year", Type: "text", Description: "Year (YYYY)"},
			{Name: "account", Type: "text", Description: "Account name"},
			{Name: "transaction_count", Type: "integer", Description: "Number of postings"},
			{Name: "total_amount", Type: "real", Description: "Sum of amounts for this date+account"},
			{Name: "avg_amount", Type: "real", Description: "Average amount"},
			{Name: "min_amount", Type: "real", Description: "Minimum amount"},
			{Name: "max_amount", Type: "real", Description: "Maximum amount"},
		},
		SQL: `
SELECT
    t.date,
    substr(t.date, 1, 7) AS month,
    substr(t.date, 1, 4) AS year,
    p.account,
    COUNT(*) AS transaction_count,
    SUM(p.amount) AS total_amount,
    AVG(p.amount) AS avg_amount,
    MIN(p.amount) AS min_amount,
    MAX(p.amount) AS max_amount
FROM transactions t
JOIN postings p ON p.transaction_id = t.id
GROUP BY t.date, p.account`,
	},
}

// TableNames returns the sorted list of available table names for error messages.
func TableNames() []string {
	names := make([]string, 0, len(Tables))
	for k := range Tables {
		names = append(names, k)
	}
	return names
}

// Package sqlexport provides the sql-export plugin which exports the journal
// to a standalone, queryable SQLite file.
package sqlexport

import (
	"flag"
	"fmt"

	"doublebook/core/ast"
	"doublebook/infra/db"
	"doublebook/plugin"
)

// SQLExportPlugin exports the journal to a standalone SQLite database file.
// The exported file can be queried with any SQL tool (sqlite3, DBeaver, etc.)
// and includes convenience views for common queries.
type SQLExportPlugin struct {
	plugin.DefaultPlugin
	OutputPath string
}

func (p *SQLExportPlugin) Name() string    { return "sql-export" }
func (p *SQLExportPlugin) Version() string { return "1.0.0" }
func (p *SQLExportPlugin) Description() string {
	return "Export the journal to a standalone SQLite database file"
}

func (p *SQLExportPlugin) Initialize(config map[string]interface{}) error {
	if config != nil {
		if v, ok := config["output_path"].(string); ok {
			p.OutputPath = v
		}
	}
	return nil
}

// RunCommand handles: doublebook plugin run sql-export [--output path]
func (p *SQLExportPlugin) RunCommand(transactions []*ast.Transaction, args []string) error {
	fs := flag.NewFlagSet("sql-export", flag.ContinueOnError)
	outputFlag := fs.String("output", "", "Output SQLite file path (default: doublebook-export.db)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	outputPath := p.OutputPath
	if *outputFlag != "" {
		outputPath = *outputFlag
	}
	if outputPath == "" {
		outputPath = "doublebook-export.db"
	}

	fmt.Printf("Exporting %d transactions to %q…\n", len(transactions), outputPath)

	// Open a fresh SQLite database at the output path.
	database, err := db.Open(outputPath)
	if err != nil {
		return fmt.Errorf("opening export database: %w", err)
	}
	defer database.Close()

	if err := database.Initialize(); err != nil {
		return fmt.Errorf("initialising schema: %w", err)
	}

	if err := database.LoadFromTransactions(transactions); err != nil {
		return fmt.Errorf("loading transactions: %w", err)
	}

	// Create additional convenience views.
	if err := createViews(database); err != nil {
		return fmt.Errorf("creating views: %w", err)
	}

	// Count exported items.
	postings := 0
	for _, t := range transactions {
		postings += len(t.Postings)
	}

	fmt.Printf("\nExport complete:\n")
	fmt.Printf("  Transactions: %d\n", len(transactions))
	fmt.Printf("  Postings:     %d\n", postings)
	fmt.Printf("  File:         %s\n", outputPath)
	fmt.Printf("\nQuery with: sqlite3 %q\n", outputPath)
	fmt.Printf("  SELECT * FROM v_balances;\n")
	fmt.Printf("  SELECT * FROM v_register LIMIT 20;\n")
	fmt.Printf("  SELECT * FROM v_monthly;\n")
	return nil
}

const viewSQL = `
-- All postings with transaction info (register view)
CREATE VIEW IF NOT EXISTS v_register AS
SELECT
    t.date,
    t.description,
    t.status,
    p.account,
    p.amount,
    p.currency,
    t.id AS transaction_id
FROM transactions t
JOIN postings p ON p.transaction_id = t.id
ORDER BY t.date, t.id;

-- Account balances
CREATE VIEW IF NOT EXISTS v_balances AS
SELECT
    p.account,
    SUM(p.amount) AS balance,
    p.currency,
    COUNT(*) AS posting_count
FROM postings p
GROUP BY p.account, p.currency
ORDER BY p.account;

-- Monthly spending by account
CREATE VIEW IF NOT EXISTS v_monthly AS
SELECT
    substr(t.date, 1, 7) AS month,
    p.account,
    SUM(p.amount) AS total,
    COUNT(*) AS count
FROM transactions t
JOIN postings p ON p.transaction_id = t.id
GROUP BY substr(t.date, 1, 7), p.account
ORDER BY month, p.account;
`

func createViews(database *db.DB) error {
	_, err := database.Conn().Exec(viewSQL)
	return err
}

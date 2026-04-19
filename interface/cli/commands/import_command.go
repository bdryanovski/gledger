package commands

import (
	"crypto/sha256"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"doublebook/core/ast"
	"doublebook/infra/config"
	"doublebook/ingest/legacy"
	"doublebook/ingest/rules"
	"doublebook/engine/interpreter"
	"doublebook/core/journal"
)

// ImportCommand implements `doublebook import --map FILE.importmap.json data.csv`.
// Also supports the new rules engine with `doublebook import --rules NAME data.csv`.
func ImportCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	mapFlag := fs.String("map", "", "Path to the *.importmap.json file (legacy)")
	rulesFlag := fs.String("rules", "", "Name or path to rules file (new rules engine)")
	dryRun := fs.Bool("dry-run", false, "Show what would be imported without writing to journal")
	verbose := fs.Bool("verbose", false, "Print each imported transaction")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *mapFlag == "" && *rulesFlag == "" {
		return fmt.Errorf("--map or --rules is required\n\nUsage:\n  doublebook import --rules NAME data.csv    (new rules engine)\n  doublebook import --map FILE.json data.csv (legacy)")
	}

	// Use rules engine if --rules specified
	if *rulesFlag != "" {
		return importWithRules(ctx, *rulesFlag, fs.Args(), *dryRun, *verbose)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		return fmt.Errorf("CSV file path is required\n\nUsage: doublebook import --map FILE.importmap.json data.csv")
	}
	csvPath := remaining[0]

	// Validate files exist before doing any work.
	if _, err := os.Stat(*mapFlag); err != nil {
		return fmt.Errorf("importmap file not found: %q", *mapFlag)
	}
	if _, err := os.Stat(csvPath); err != nil {
		return fmt.Errorf("CSV file not found: %q", csvPath)
	}

	// Load the importmap.
	imap, err := legacy.LoadImportMap(*mapFlag)
	if err != nil {
		return fmt.Errorf("loading importmap: %w", err)
	}

	// Load the existing journal to build the deduplication ID set.
	interp := interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		// Non-fatal: warn but continue. Without existing IDs we may re-import
		// entries, but the user can fix their journal and re-run with --dry-run.
		fmt.Fprintf(os.Stderr,
			"  warning: could not load existing journal for deduplication:\n  %v\n\n", err)
	}
	existingIDs := legacy.ExtractIDs(interp.GetTransactions())

	// Print header.
	fmt.Printf("Importing from %q using %q\n\n", csvPath, *mapFlag)
	if *dryRun {
		fmt.Println("  DRY RUN — no changes will be written")
		fmt.Println()
	}

	// Run the import.
	result, err := legacy.ImportCSV(csvPath, imap, existingIDs)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	// Print verbose transaction listing.
	if *verbose && len(result.Imported) > 0 {
		sep := strings.Repeat("─", 72)
		fmt.Println(sep)
		for _, txn := range result.Imported {
			fmt.Printf("%s  %s\n", txn.Date.Format("2006-01-02"), txn.Description)
			for _, p := range txn.Postings {
				fmt.Printf("    %-36s  %s\n", p.Account, p.Amount.String())
			}
			fmt.Println()
		}
		fmt.Println(sep)
		fmt.Println()
	}

	// Summary.
	verb := "Imported"
	if *dryRun {
		verb = "Would import"
	}
	fmt.Printf("  Processed   %4d rows\n", result.TotalRows)
	fmt.Printf("  %-11s %4d new transactions\n", verb+":", len(result.Imported))
	fmt.Printf("  Skipped:    %4d duplicates\n", result.Skipped)
	fmt.Printf("  Errors:     %4d rows failed\n", len(result.Errors))

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("  Warnings:")
		for _, e := range result.Errors {
			fmt.Printf("    Row %d: %s\n", e.Row, e.Reason)
		}
	}

	if *dryRun {
		fmt.Println("\n  (Use without --dry-run to actually import)")
		return nil
	}

	if len(result.Imported) == 0 {
		fmt.Println("  Nothing new to import.")
		return nil
	}

	// Append each transaction to the journal.
	name := ctx.EffectiveJournalName()
	dir := ctx.Config.DataDirPath()
	failed := 0
	for _, txn := range result.Imported {
		if err := journal.AppendTransaction(name, dir, txn); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not write transaction %q: %v\n", txn.Description, err)
			failed++
		}
	}

	fmt.Printf("\n  Journal updated: %s/%s.journal", dir, name)
	if failed > 0 {
		fmt.Printf(" (%d write errors)", failed)
	}
	fmt.Println()

	return nil
}

// importWithRules imports using the new rules engine.
func importWithRules(ctx *config.CLIContext, rulesName string, args []string, dryRun, verbose bool) error {
	if len(args) == 0 {
		return fmt.Errorf("CSV/Excel file path is required")
	}
	filePath := args[0]

	// Find and load rules
	rulesPath, err := rules.FindRuleSet(rulesName)
	if err != nil {
		return fmt.Errorf("finding rules: %w", err)
	}

	ruleSet, err := rules.LoadRuleSet(rulesPath)
	if err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}

	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Create engine
	engine, err := rules.NewEngine(ruleSet)
	if err != nil {
		return fmt.Errorf("creating rules engine: %w", err)
	}

	// Read file data
	rows, err := readDataFile(filePath, ruleSet.Format)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Print header
	fmt.Printf("Importing from %q using rules %q\n\n", filePath, ruleSet.Name)
	if dryRun {
		fmt.Println("  DRY RUN — no changes will be written")
		fmt.Println()
	}

	// Load existing transactions for deduplication
	interp := interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not load existing journal for deduplication:\n  %v\n\n", err)
	}
	existingIDs := legacy.ExtractIDs(interp.GetTransactions())

	// Process rows
	result := engine.ProcessRows(rows)

	// Filter duplicates
	var newTxns []*ast.Transaction
	skipped := 0
	for _, txn := range result.Transactions {
		// Generate ID for dedup
		id := generateTxnID(txn)
		txn.ID = id
		if existingIDs[id] {
			skipped++
			continue
		}
		existingIDs[id] = true
		newTxns = append(newTxns, txn)
	}

	// Print verbose listing
	if verbose && len(newTxns) > 0 {
		sep := strings.Repeat("─", 72)
		fmt.Println(sep)
		for _, txn := range newTxns {
			fmt.Printf("%s  %s\n", txn.Date.Format("2006-01-02"), txn.Description)
			for _, p := range txn.Postings {
				fmt.Printf("    %-36s  %s\n", p.Account, p.Amount.String())
			}
			fmt.Println()
		}
		fmt.Println(sep)
		fmt.Println()
	}

	// Summary
	verb := "Imported"
	if dryRun {
		verb = "Would import"
	}
	fmt.Printf("  Processed:  %4d rows\n", result.TotalRows)
	fmt.Printf("  %-11s %4d new transactions\n", verb+":", len(newTxns))
	fmt.Printf("  Skipped:    %4d duplicates\n", skipped)
	fmt.Printf("  Errors:     %4d rows failed\n", len(result.Errors))

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("  Errors:")
		for _, e := range result.Errors {
			fmt.Printf("    Row %d: %s\n", e.Row, e.Reason)
		}
	}

	if dryRun {
		fmt.Println("\n  (Use without --dry-run to actually import)")
		return nil
	}

	if len(newTxns) == 0 {
		fmt.Println("  Nothing new to import.")
		return nil
	}

	// Write to journal
	name := ctx.EffectiveJournalName()
	dir := ctx.Config.DataDirPath()
	failed := 0
	for _, txn := range newTxns {
		if err := journal.AppendTransaction(name, dir, txn); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not write transaction %q: %v\n", txn.Description, err)
			failed++
		}
	}

	fmt.Printf("\n  Journal updated: %s/%s.journal", dir, name)
	if failed > 0 {
		fmt.Printf(" (%d write errors)", failed)
	}
	fmt.Println()

	return nil
}

// generateTxnID generates a unique ID for a transaction.
func generateTxnID(txn *ast.Transaction) string {
	var amtStr string
	if len(txn.Postings) > 0 {
		amtStr = fmt.Sprintf("%.4f", txn.Postings[0].Amount.Value)
	}
	key := txn.Date.Format("2006-01-02") + "|" + amtStr + "|" + txn.Description
	h := sha256.New()
	h.Write([]byte(key))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// readDataFile reads rows from a CSV or Excel file.
func readDataFile(path string, format rules.FileFormat) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Determine delimiter
	delimiter := format.Delimiter
	if delimiter == "" {
		delimiter = ","
	}

	reader := csv.NewReader(f)
	reader.Comma = rune(delimiter[0])
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1

	// Skip header lines
	for i := 0; i < format.SkipLines; i++ {
		if _, err := reader.Read(); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("skipping header line %d: %w", i+1, err)
		}
	}

	// Read all rows
	var rows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip bad rows
		}
		rows = append(rows, row)
	}

	return rows, nil
}

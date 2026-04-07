package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"doublebook/config"
	"doublebook/importer"
	Interpreter "doublebook/interpreter"
	"doublebook/journal"
)

// ImportCommand implements `doublebook import --map FILE.importmap.json data.csv`.
func ImportCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	mapFlag := fs.String("map", "", "Path to the *.importmap.json file (required)")
	dryRun := fs.Bool("dry-run", false, "Show what would be imported without writing to journal")
	verbose := fs.Bool("verbose", false, "Print each imported transaction")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *mapFlag == "" {
		return fmt.Errorf("--map is required\n\nUsage: doublebook import --map FILE.importmap.json data.csv")
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
	imap, err := importer.LoadImportMap(*mapFlag)
	if err != nil {
		return fmt.Errorf("loading importmap: %w", err)
	}

	// Load the existing journal to build the deduplication ID set.
	interp := Interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		// Non-fatal: warn but continue. Without existing IDs we may re-import
		// entries, but the user can fix their journal and re-run with --dry-run.
		fmt.Fprintf(os.Stderr,
			"  warning: could not load existing journal for deduplication:\n  %v\n\n", err)
	}
	existingIDs := importer.ExtractIDs(interp.GetTransactions())

	// Print header.
	fmt.Printf("Importing from %q using %q\n\n", csvPath, *mapFlag)
	if *dryRun {
		fmt.Println("  DRY RUN — no changes will be written")
		fmt.Println()
	}

	// Run the import.
	result, err := importer.ImportCSV(csvPath, imap, existingIDs)
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

package commands

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"doublebook/config"
)

// ResetCommand deletes all journal data after confirmation.
func ResetCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("reset", flag.ContinueOnError)
	force := fs.Bool("force", false, "Skip confirmation prompt")
	dryRun := fs.Bool("dry-run", false, "Show what would be deleted without deleting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	journalName := ctx.EffectiveJournalName()
	dataDir := ctx.Config.DataDirPath()

	// Find all journal files matching the pattern
	files, err := findJournalFiles(dataDir, journalName)
	if err != nil {
		return fmt.Errorf("finding journal files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No journal files found. Nothing to reset.")
		return nil
	}

	// Show what will be deleted
	fmt.Println("The following files will be deleted:")
	fmt.Println()
	for _, f := range files {
		fmt.Printf("  %s\n", f)
	}
	fmt.Println()

	if *dryRun {
		fmt.Println("(dry-run mode — no files were deleted)")
		return nil
	}

	// Confirmation
	if !*force {
		fmt.Print("Are you sure you want to delete all journal data? Type 'yes' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "yes" {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	// Delete files
	var deleted, failed int
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			fmt.Printf("  Failed to delete %s: %v\n", f, err)
			failed++
		} else {
			deleted++
		}
	}

	fmt.Printf("\nDeleted %d file(s)", deleted)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println(".")

	if deleted > 0 {
		fmt.Println("Journal data has been reset. You can start fresh!")
	}

	return nil
}

// findJournalFiles returns all journal files for the given journal name.
// It looks for patterns like: name.journal, name.1.journal, name.2.journal, etc.
func findJournalFiles(dataDir, journalName string) ([]string, error) {
	var files []string

	// Primary journal file
	primary := filepath.Join(dataDir, journalName+".journal")
	if _, err := os.Stat(primary); err == nil {
		files = append(files, primary)
	}

	// Numbered journal files (name.1.journal, name.2.journal, ...)
	pattern := filepath.Join(dataDir, journalName+".*.journal")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	files = append(files, matches...)

	return files, nil
}

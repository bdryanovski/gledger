package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"doublebook/infra/config"
	"doublebook/infra/db"
	"doublebook/engine/fql"
	"doublebook/engine/interpreter"
	"doublebook/core/journal"
	"doublebook/interface/tui"
	"doublebook/infra/utils"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// FQLCommand implements `doublebook fql [--query "..."]`.
//
//   - Without --query: launches the fullscreen interactive REPL.
//   - With --query:    runs a single FQL query, prints the result, and exits.
func FQLCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("fql", flag.ContinueOnError)
	queryFlag := fs.String("query", "", "Run a single FQL query and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Build the SQLite query cache from the journal.
	database, err := initFQLDB(ctx)
	if err != nil {
		return fmt.Errorf("initialising FQL database: %w", err)
	}
	defer database.Close()

	// ── Single-shot mode ────────────────────────────────────────────────
	if *queryFlag != "" {
		w := termWidth()
		output, err := fql.Execute(*queryFlag, database, w)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		fmt.Print(output)
		return nil
	}

	// ── Interactive REPL ─────────────────────────────────────────────────
	w, h := termWidth(), termHeight()
	model := tui.NewFQLModel(database, w, h)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	_, err = prog.Run()
	return err
}

// ---------------------------------------------------------------------------
// DB initialisation
// ---------------------------------------------------------------------------

// initFQLDB loads the journal and opens (or rebuilds) the SQLite query cache.
func initFQLDB(ctx *config.CLIContext) (*db.DB, error) {
	// Load all journal transactions via the interpreter.
	interp := interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		// Non-fatal: we may have a partial or empty journal.
		fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
	}
	txns := interp.GetTransactions()

	// Resolve journal file paths for checksum computation.
	dataDir := utils.ExpandHome(ctx.Config.DataDir)
	journalFiles := journal.Resolve(ctx.EffectiveJournalName(), dataDir)

	// The cache file lives alongside the journal files.
	dbPath := filepath.Join(dataDir, "cache.db")

	return db.OpenOrRebuild(dbPath, txns, journalFiles)
}

// ---------------------------------------------------------------------------
// Terminal size helpers
// ---------------------------------------------------------------------------

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func termHeight() int {
	_, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

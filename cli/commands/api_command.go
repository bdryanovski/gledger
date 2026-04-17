package commands

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"doublebook/api"
	"doublebook/config"
	"doublebook/currency"
	"doublebook/db"
	"doublebook/interpreter"
	"doublebook/journal"
	"doublebook/utils"
)

// APICommand starts the DoubleBook REST API server.
func APICommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	portFlag := fs.Int("port", 0, "Override API port (default from config)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	port := ctx.Config.APIPort
	if *portFlag != 0 {
		port = *portFlag
	}

	// Load journal.
	interp := interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	// Build SQLite cache.
	dataDir := utils.ExpandHome(ctx.Config.DataDir)
	journalFiles := journal.Resolve(ctx.EffectiveJournalName(), dataDir)
	dbPath := filepath.Join(dataDir, "cache.db")
	database, err := db.OpenOrRebuild(dbPath, interp.GetTransactions(), journalFiles)
	if err != nil {
		return fmt.Errorf("initialising query cache: %w", err)
	}
	defer database.Close()

	// Currency converter.
	conv, err := currency.NewCachingConverter(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: currency converter unavailable: %v\n", err)
	}

	srv := api.NewServer(port, interp, database, conv)

	fmt.Printf("DoubleBook API running on http://localhost:%d\n", port)
	fmt.Println("Press Ctrl+C to stop.")

	// Graceful shutdown on Ctrl+C.
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\nShutting down…")
		os.Exit(0)
	}()

	return srv.Start()
}

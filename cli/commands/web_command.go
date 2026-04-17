package commands

import (
	"flag"
	"fmt"
	"net/http"
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
	"doublebook/web"
)

// WebCommand starts both the API server (port 5555) and the web UI server (port 4444).
func WebCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	webPortFlag := fs.Int("web-port", 0, "Override web UI port (default from config)")
	apiPortFlag := fs.Int("api-port", 0, "Override API port (default from config)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	webPort := ctx.Config.WebPort
	if *webPortFlag != 0 {
		webPort = *webPortFlag
	}
	apiPort := ctx.Config.APIPort
	if *apiPortFlag != 0 {
		apiPort = *apiPortFlag
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
	conv, _ := currency.NewCachingConverter(dataDir)

	// Start API server in background.
	apiSrv := api.NewServer(apiPort, interp, database, conv)
	go func() {
		if err := apiSrv.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "API server error: %v\n", err)
		}
	}()

	// Start web server (serves embedded React app + proxies /api to port 5555).
	webMux := http.NewServeMux()

	// All /api requests go to the API server.
	webMux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		// Use the API handler directly (same process, no network hop).
		apiSrv.Handler().ServeHTTP(w, r)
	})

	// Everything else: serve the React SPA.
	webMux.Handle("/", web.Handler())

	addr := fmt.Sprintf(":%d", webPort)
	webServer := &http.Server{Addr: addr, Handler: webMux}

	fmt.Printf("\n  DoubleBook Web UI  →  http://localhost:%d\n", webPort)
	fmt.Printf("  DoubleBook API     →  http://localhost:%d\n\n", apiPort)
	fmt.Println("  Press Ctrl+C to stop.")

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		fmt.Println("\n  Shutting down…")
		os.Exit(0)
	}()

	return webServer.ListenAndServe()
}

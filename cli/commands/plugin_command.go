package commands

import (
	"fmt"
	"strings"

	"doublebook/config"
	Interpreter "doublebook/interpreter"
	"doublebook/plugin/extensions/recurring"
	"doublebook/plugin/extensions/sqlexport"
)

// PluginCommand implements `doublebook plugin [list|run <name> [args...]]`.
func PluginCommand(ctx *config.CLIContext, args []string) error {
	if len(args) == 0 || args[0] == "list" {
		return runPluginList(ctx)
	}
	if args[0] == "run" {
		if len(args) < 2 {
			return fmt.Errorf("usage: doublebook plugin run <name> [args...]")
		}
		return runPlugin(ctx, args[1], args[2:])
	}
	return fmt.Errorf("usage: doublebook plugin [list|run <name> [args...]]")
}

func runPluginList(ctx *config.CLIContext) error {
	interp := Interpreter.NewInterpreter(ctx.Config)
	_ = interp.LoadJournal(ctx.EffectiveJournalName())

	fmt.Println("Built-in plugins:")
	fmt.Println()

	plugins := []struct{ name, version, desc string }{
		{"sql-export", "1.0.0", "Export journal to a queryable SQLite file"},
		{"recurring", "1.0.0", "Manage and report on recurring payment schedules"},
		{"example", "1.0.0", "Example plugin — demonstrates the plugin API"},
	}
	for _, p := range plugins {
		fmt.Printf("  %-14s  v%s  —  %s\n", p.name, p.version, p.desc)
	}
	fmt.Println()
	fmt.Println("Run:  doublebook plugin run <name> [--help]")
	return nil
}

func runPlugin(ctx *config.CLIContext, name string, args []string) error {
	// Load the journal so plugins have data to work with.
	interp := Interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		fmt.Printf("  warning: %v\n", err)
	}
	txns := interp.GetTransactions()

	switch strings.ToLower(name) {
	case "sql-export":
		p := &sqlexport.SQLExportPlugin{}
		_ = p.Initialize(nil)
		return p.RunCommand(txns, args)

	case "recurring":
		p := &recurring.RecurringPlugin{}
		_ = p.Initialize(map[string]interface{}{
			"config_dir": ctx.Config.DataDirPath(),
		})
		return p.RunCommand(txns, args)

	default:
		return fmt.Errorf("unknown plugin %q. Run 'doublebook plugin list' to see available plugins", name)
	}
}

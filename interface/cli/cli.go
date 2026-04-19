package cli

import (
	"fmt"

	"doublebook/interface/cli/commands"
)

// Run is the main CLI entry point.  It parses global flags first, then
// dispatches to the appropriate command handler.
func Run(args []string) error {
	if len(args) == 0 {
		return runHelp(nil)
	}

	// Parse global flags (--journal, --begin, --end, --verbose).
	ctx, remaining, err := NewContext(args)
	if err != nil {
		return fmt.Errorf("flag error: %w", err)
	}

	if len(remaining) == 0 {
		return runHelp(nil)
	}

	command := remaining[0]
	cmdArgs := remaining[1:]

	switch command {
	case "balance", "bal":
		return commands.BalanceCommand(ctx, cmdArgs)
	case "register", "reg", "r":
		return commands.RegisterCommand(ctx, cmdArgs)
	case "list", "ls":
		return commands.RegisterCommand(ctx, cmdArgs)
	case "is", "income-statement", "income":
		return commands.ISCommand(ctx, cmdArgs)
	case "plugin":
		return commands.PluginCommand(ctx, cmdArgs)
	case "api":
		return commands.APICommand(ctx, cmdArgs)
	case "web":
		return commands.WebCommand(ctx, cmdArgs)
	case "fql", "query":
		return commands.FQLCommand(ctx, cmdArgs)
	case "dashboard", "dash":
		return commands.DashboardCommand(ctx, cmdArgs)
	case "import":
		return commands.ImportCommand(ctx, cmdArgs)
	case "map", "mapper":
		return commands.MapCommand(ctx, cmdArgs)
	case "insert":
		return commands.InsertCommand(ctx, cmdArgs)
	case "add":
		// 'add' also uses the interactive insert form.
		return commands.InsertCommand(ctx, cmdArgs)
	case "reset":
		return commands.ResetCommand(ctx, cmdArgs)
	case "help", "-h", "--help":
		return runHelp(cmdArgs)
	case "version", "-v", "--version":
		return runVersion(cmdArgs)
	default:
		return fmt.Errorf("unknown command %q — run 'doublebook help' for usage", command)
	}
}

func runHelp(_ []string) error {
	fmt.Print(`
DoubleBook — plain-text double-entry accounting

Usage:
  doublebook [global flags] <command> [command flags]

Global flags:
  --journal NAME    Use journal stem NAME (default: "data")
  --begin DATE      Filter: only include transactions on/after DATE (YYYY-MM-DD)
  --end DATE        Filter: only include transactions on/before DATE
  --verbose         Enable verbose output

Commands:
  balance, bal           Show account balances
  register, reg, r       Show a transaction register with running total
  list, ls               Alias for register
  is, income-statement   Show income statement (revenues vs expenses)
  dashboard, dash        Show financial dashboard with charts and trends
  fql, query             Financial Query Language (REPL or --query "...")
  insert, add            Interactive form to add a new transaction
  import                 Import transactions from a CSV file
  map, mapper            Interactive column mapper for CSV/Excel files
  reset                  Delete all journal data (with confirmation)
  help                   Show this help message
  version                Show version information

Examples:
  doublebook balance
  doublebook balance --tree
  doublebook register
  doublebook register --account expenses --limit 10
  doublebook --journal personal register
  doublebook --begin 2025-01-01 --end 2025-01-31 register
  doublebook add --date 2025-01-15 --description "Coffee" --amount 5.00 \
                 --from assets:checking --to expenses:dining
  doublebook reset                    # asks for confirmation
  doublebook reset --force            # skip confirmation
  doublebook reset --dry-run          # show what would be deleted

Journal files:
  DoubleBook looks for <journal>.journal, <journal>.1.journal, etc.
  in ~/.doublebook/ (configurable via ~/.doublebook/config.yaml).
`)
	return nil
}

func runVersion(_ []string) error {
	fmt.Println("doublebook version 0.1.0")
	return nil
}

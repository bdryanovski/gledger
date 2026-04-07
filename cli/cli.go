package cli

import (
	"fmt"

	"doublebook/cli/commands"
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
		// list is an alias for register
		return commands.RegisterCommand(ctx, cmdArgs)
	case "add":
		return commands.AddCommand(ctx, cmdArgs)
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
  balance, bal      Show account balances
  register, reg     Show a transaction register with running total
  list, ls          Alias for register
  add               Add a transaction via command-line flags
  help              Show this help message
  version           Show version information

Examples:
  doublebook balance
  doublebook balance --tree
  doublebook register
  doublebook register --account expenses --limit 10
  doublebook --journal personal register
  doublebook --begin 2025-01-01 --end 2025-01-31 register
  doublebook add --date 2025-01-15 --description "Coffee" --amount 5.00 \
                 --from assets:checking --to expenses:dining

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

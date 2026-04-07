package commands

import (
	"doublebook/config"
	Interpreter "doublebook/interpreter"
	"fmt"
)

// ListCommand prints a plain-text transaction register to stdout.
// Format matches hledger's register output style.
//
// NOTE: This is a temporary implementation that will be superseded by the
// full register command in T1.10.
func ListCommand(args []string) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	interp := Interpreter.NewInterpreter(cfg)
	if err := interp.LoadFromFile(cfg.DataFile); err != nil {
		// Non-fatal: the journal file may not exist yet.
		fmt.Printf("Note: could not load journal: %v\n\n", err)
	}

	txns := interp.GetTransactions()
	if len(txns) == 0 {
		fmt.Println("No transactions found.")
		return nil
	}

	for i, txn := range txns {
		// Blank line between transactions (but not before the first).
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("%s %s\n", txn.Date.Format("2006-01-02"), txn.Description)
		for _, posting := range txn.Postings {
			fmt.Printf("    %-36s  %s\n", posting.Account, posting.Amount.String())
		}
	}

	return nil
}

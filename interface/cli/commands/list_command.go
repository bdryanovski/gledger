package commands

import (
	"fmt"

	"doublebook/infra/config"
	"doublebook/engine/interpreter"
)

// ListCommand prints a plain-text transaction register to stdout,
// respecting the BeginDate/EndDate filters from ctx.
//
// NOTE: This is a temporary implementation superseded by the full register
// command in T1.10.
func ListCommand(ctx *config.CLIContext, args []string) error {
	interp := interpreter.NewInterpreter(ctx.Config)

	// Use LoadJournal for multi-file support when a journal name is given,
	// falling back to the single-file path for backward compat.
	journalName := ctx.EffectiveJournalName()
	if err := interp.LoadJournal(journalName); err != nil {
		fmt.Printf("Note: could not load journal: %v\n\n", err)
	}

	filter := interpreter.Filter{
		BeginDate: ctx.BeginDate,
		EndDate:   ctx.EndDate,
	}

	txns := interp.FilteredTransactions(filter)
	if len(txns) == 0 {
		fmt.Println("No transactions found.")
		return nil
	}

	for i, txn := range txns {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s %s\n", txn.Date.Format("2006-01-02"), txn.Description)
		for _, p := range txn.Postings {
			fmt.Printf("    %-36s  %s\n", p.Account, p.Amount.String())
		}
	}

	return nil
}

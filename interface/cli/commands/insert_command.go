package commands

import (
	"fmt"
	"sort"

	"doublebook/infra/config"
	"doublebook/engine/interpreter"
	"doublebook/core/journal"
	"doublebook/interface/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// InsertCommand runs the interactive inline form for adding a new transaction.
func InsertCommand(ctx *config.CLIContext, args []string) error {
	// Load the journal so we can populate autocomplete data.
	interp := interpreter.NewInterpreter(ctx.Config)
	_ = interp.LoadJournal(ctx.EffectiveJournalName())

	accounts := collectKnownAccounts(interp)
	currencies := collectKnownCurrencies(interp, ctx.Config.Currency)

	// Build and run the insert form (non-fullscreen / inline mode).
	model := tui.NewInsertModel(accounts, currencies, ctx.Config.Currency)
	prog := tea.NewProgram(model)

	finalRaw, err := prog.Run()
	if err != nil {
		return fmt.Errorf("insert form error: %w", err)
	}

	final, ok := finalRaw.(tui.InsertModel)
	if !ok {
		return fmt.Errorf("unexpected model type after insert form")
	}

	if final.Aborted() {
		fmt.Println("Aborted.")
		return nil
	}

	txn := final.Result()
	if txn == nil {
		return nil
	}

	// Persist to journal.
	name, dir := ctx.EffectiveJournalName(), ctx.Config.DataDirPath()
	if err := journal.AppendTransaction(name, dir, txn); err != nil {
		return fmt.Errorf("saving transaction: %w", err)
	}

	fmt.Printf("\n  Added: %s  %s\n", txn.Date.Format("2006-01-02"), txn.Description)
	for _, p := range txn.Postings {
		fmt.Printf("    %-36s  %s\n", p.Account, p.Amount.String())
	}
	return nil
}

// ---------------------------------------------------------------------------
// Autocomplete data helpers
// ---------------------------------------------------------------------------

// collectKnownAccounts returns a sorted, deduplicated list of all account
// names that appear in the loaded journal.
func collectKnownAccounts(interp *interpreter.Interpreter) []string {
	seen := make(map[string]bool)
	for _, txn := range interp.GetTransactions() {
		for _, p := range txn.Postings {
			seen[p.Account] = true
		}
	}
	accts := make([]string, 0, len(seen))
	for a := range seen {
		accts = append(accts, a)
	}
	sort.Strings(accts)
	return accts
}

// collectKnownCurrencies returns a sorted list of currencies found in the
// journal, always including the base currency and common defaults.
func collectKnownCurrencies(interp *interpreter.Interpreter, baseCurrency string) []string {
	seen := map[string]bool{
		baseCurrency: true,
		"USD":        true, "EUR": true, "GBP": true,
		"BGN": true, "CHF": true, "CAD": true,
	}
	for _, txn := range interp.GetTransactions() {
		for _, p := range txn.Postings {
			if p.Amount.Currency != "" {
				seen[p.Amount.Currency] = true
			}
		}
	}
	currencies := make([]string, 0, len(seen))
	for c := range seen {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)
	return currencies
}

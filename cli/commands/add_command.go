package commands

import (
	"flag"
	"fmt"
	"time"

	AST "doublebook/ast"
	"doublebook/config"
	Interpreter "doublebook/interpreter"
)

// AddCommand adds a single transaction via command-line flags.
// It uses the journal name and data directory from ctx.
func AddCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	dateFlag := fs.String("date", "", "Transaction date (YYYY-MM-DD)")
	descFlag := fs.String("description", "", "Transaction description")
	amountFlag := fs.Float64("amount", 0.0, "Transaction amount")
	fromFlag := fs.String("from", "", "Debit account (money leaves here)")
	toFlag := fs.String("to", "", "Credit account (money goes here)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *dateFlag == "" || *descFlag == "" || *fromFlag == "" || *toFlag == "" || *amountFlag == 0 {
		return fmt.Errorf("usage: doublebook add --date DATE --description DESC --amount AMT --from ACCT --to ACCT")
	}

	txn, err := buildTransaction(*dateFlag, *descFlag, *amountFlag, *fromFlag, *toFlag, ctx.Config.Currency)
	if err != nil {
		return fmt.Errorf("building transaction: %w", err)
	}

	interp := Interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadFromFile(ctx.Config.DataFile); err != nil {
		return fmt.Errorf("loading journal: %w", err)
	}

	if err := interp.AddTransaction(txn); err != nil {
		return fmt.Errorf("adding transaction: %w", err)
	}

	if err := interp.SaveToFile(ctx.Config.DataFile); err != nil {
		return fmt.Errorf("saving journal: %w", err)
	}

	fmt.Println("Transaction added successfully!")
	printTransaction(txn)
	return nil
}

// buildTransaction constructs a balanced two-posting transaction.
func buildTransaction(date, description string, amount float64, from, to, currency string) (*AST.Transaction, error) {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}

	txn := AST.NewTransaction(d, description)
	txn.Postings = append(txn.Postings,
		AST.NewPosting(from, AST.Amount{Value: -amount, Currency: currency}),
		AST.NewPosting(to, AST.Amount{Value: amount, Currency: currency}),
	)
	return txn, nil
}

func printTransaction(txn *AST.Transaction) {
	fmt.Printf("%s  %s\n", txn.Date.Format("2006-01-02"), txn.Description)
	for _, p := range txn.Postings {
		fmt.Printf("    %-40s  %s\n", p.Account, p.Amount.String())
	}
}

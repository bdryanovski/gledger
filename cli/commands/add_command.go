package commands

import (
	"flag"
	"fmt"
	AST "gledger/ast"
	"gledger/config"
	Interpreter "gledger/interpreter"
	"time"
)

func AddCommand(args []string) error {
	addFlags := flag.NewFlagSet("add", flag.ExitOnError)

	// Define a transaction input
	dateFlag := addFlags.String("date", "", "Date of the transaction (YYYY-MM-DD)")
	descriptionFlag := addFlags.String("description", "", "Description of the transaction")
	amountFlag := addFlags.Float64("amount", 0.0, "Amount of the transaction")
	fromFlag := addFlags.String("from", "", "Account for the transaction")
	toFlag := addFlags.String("to", "", "Account for the transaction")

	addFlags.Parse(args)

	config, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return err
	}

	interpreter := Interpreter.NewInterpreter(config)
	if err := interpreter.LoadFromFile(config.DataFile); err != nil {
		fmt.Printf("Error loading data file: %v\n", err)
		return err
	}

	var transaction *AST.Transaction

	if *dateFlag != "" {
		transaction, err = createTransaction(*dateFlag, *descriptionFlag, *amountFlag, *fromFlag, *toFlag)
	}

	if err != nil {
		fmt.Printf("Error creating transaction: %v\n", err)
		return err
	}

	if err := interpreter.AddTransaction(transaction); err != nil {
		fmt.Printf("Error adding transaction: %v\n", err)
		return err
	}

	if err := interpreter.SaveToFile(config.DataFile); err != nil {
		fmt.Printf("Error saving data file: %v\n", err)
		return err
	}

	fmt.Println("✓ Transaction added successfully!")
	printTransaction(transaction, 0)

	return nil
}

func createTransaction(date, description string, amount float64, from, to string) (*AST.Transaction, error) {
	parseDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %v", err)
	}

	if description == "" || from == "" || to == "" || amount == 0 {
		return nil, fmt.Errorf("description, from, to and amount are required fields")
	}

	return &AST.Transaction{
		Date:        parseDate,
		Description: description,
		Postings: []AST.Posting{
			{Account: from, Amount: AST.Amount{Value: -amount, Currency: "USD"}},
			{Account: to, Amount: AST.Amount{Value: amount, Currency: "USD"}},
		},
	}, nil
}

func printTransaction(transaction *AST.Transaction, index int) {
	if index > 0 {
		fmt.Printf("[%d] ", index)
	}

	fmt.Printf("%s  %s\n", transaction.Date.Format("2006-01-02"), transaction.Description)
	for _, posting := range transaction.Postings {
		fmt.Printf("    %-40s  %.2f\n", posting.Account, posting.Amount.Value)
	}
}

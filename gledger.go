package main

import "fmt"
import "os"

import "gledger/parser"

func main() {
	data, err := os.ReadFile("./example/transactions.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	txns, err := Parser.ParseTransactions(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed %d transactions\n", len(txns))

	for i, txn := range txns {
		fmt.Printf("\n--- Transaction %d ---\n", i+1)
		fmt.Printf("  Date:        %s\n", txn.Date.Format("2006-01-02"))
		fmt.Printf("  Description: %s\n", txn.Description)
		fmt.Printf("  Balanced:    %v\n", txn.IsBalanced())
		for _, p := range txn.Postings {
			fmt.Printf("    %-30s %s\n", p.Account, p.Amount.String())
		}
	}
}

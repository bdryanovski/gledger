package main

import "fmt"
import "os"

import "gledger/interpreter"

func main() {

	interpreter := Interpreter.NewInterpreter()
	error := interpreter.LoadFromFile("./example/transactions.txt")

	if error != nil {
		fmt.Fprintf(os.Stderr, "Error loading transactions: %v\n", error)
		os.Exit(1)
	}

	fmt.Printf("Loaded transactions from file\n")

	fmt.Printf("Calculating balances...\n")
	balances := interpreter.CalculateBalances()

	fmt.Printf("Account balances:\n")
	for account, balance := range balances {
		fmt.Printf("  %-30s %f\n", account, balance)
	}

	fmt.Print(interpreter.GenerateBalanceReport())

}

package Interpreter

import (
	"fmt"
	"gledger/ast"
	"gledger/parser"
	"gledger/plugin"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Interpreter struct {
	transactions []*AST.Transaction
	plugins      *Plugin.PluginManager
}

func NewInterpreter() *Interpreter {
	interpreter := &Interpreter{
		transactions: []*AST.Transaction{},
		plugins:      Plugin.NewPluginManager(),
	}
	/**
	* should register plugins here
	 */

	// interpreter.plugins.Register()

	return interpreter
}

func (interpreter *Interpreter) LoadFromFile(filename string) error {
	if strings.HasPrefix(filename, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("Error getting home directory: %v", err)
		}
		filename = filepath.Join(home, filename[2:])
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Error reading file: %v", err)
	}

	transactions, err := Parser.ParseTransactions(string(data))
	if err != nil {
		return fmt.Errorf("Parse error: %v", err)
	}

	for _, transaction := range transactions {
		if err := interpreter.plugins.ExecuteOnParse(transaction); err != nil {
			return fmt.Errorf("Plugin OnParse error: %v", err)
		}
	}

	interpreter.transactions = transactions
	return nil
}

func (interpreter *Interpreter) CalculateBalances() map[string]float64 {
	balances := make(map[string]float64)
	for _, transaction := range interpreter.transactions {
		for _, posting := range transaction.Postings {
			balances[posting.Account] += posting.Amount.Value
		}
	}
	return balances
}

func (interpreter *Interpreter) GenerateBalanceReport() string {
	balances := interpreter.CalculateBalances()

	// Group by account type (first part before colon)
	groups := make(map[string]map[string]float64)

	for account, balance := range balances {
		parts := strings.Split(account, ":")
		accountType := parts[0]

		if groups[accountType] == nil {
			groups[accountType] = make(map[string]float64)
		}
		groups[accountType][account] = balance
	}

	var report strings.Builder
	report.WriteString("BALANCE REPORT\n")
	report.WriteString("══════════════════════════════════════════════\n\n")

	// Define order for account types
	order := []string{"assets", "liabilities", "equity", "income", "expenses"}

	for _, accountType := range order {
		accounts := groups[accountType]
		if len(accounts) == 0 {
			continue
		}

		report.WriteString(fmt.Sprintf("%s:\n", strings.ToUpper(accountType)))

		// Sort accounts within group
		var names []string
		for name := range accounts {
			names = append(names, name)
		}
		sort.Strings(names)

		total := 0.0
		for _, name := range names {
			balance := accounts[name]
			total += balance
			report.WriteString(fmt.Sprintf("  %-40s %10.2f\n", name, balance))
		}

		report.WriteString(fmt.Sprintf("  %-40s %10.2f\n", "Total", total))
		report.WriteString("\n")
	}

	return report.String()
}

func (interpreter *Interpreter) GetTransactions() []*AST.Transaction {
	return interpreter.transactions
}

func (interpreter *Interpreter) AddTransaction(transaction *AST.Transaction) error {
	if !transaction.IsBalanced() {
		return fmt.Errorf("Transaction is not balanced: sum is %.2f", transaction.Balance())
	}

	if err := interpreter.plugins.ExecuteOnAdd(transaction); err != nil {
		return fmt.Errorf("Plugin OnAdd error: %v", err)
	}

	interpreter.transactions = append(interpreter.transactions, transaction)

	sort.Slice(interpreter.transactions, func(i, j int) bool {
		return interpreter.transactions[i].Date.Before(interpreter.transactions[j].Date)
	})

	return nil
}

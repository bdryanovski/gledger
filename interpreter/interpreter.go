package Interpreter

import (
	"fmt"
	"gledger/parser"
	"gledger/plugin"
	"os"
	"path/filepath"
	"strings"
)

type Interpreter struct {
	transactions []*Parser.Transaction
	plugins      *Plugin.PluginManager
}

func NewInterpreter() *Interpreter {
	interpreter := &Interpreter{
		transactions: []*Parser.Transaction{},
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

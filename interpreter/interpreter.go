// Package Interpreter provides the core engine for DoubleBook: loading journals,
// filtering transactions, calculating balances, and generating reports.
package Interpreter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	AST "doublebook/ast"
	"doublebook/config"
	"doublebook/journal"
	Plugin "doublebook/plugin"
	TemplatePlugin "doublebook/plugin/extentions"
	"doublebook/utils"
)

// ---------------------------------------------------------------------------
// Interpreter
// ---------------------------------------------------------------------------

// Interpreter is the central engine that holds loaded transactions and
// provides all query/report capabilities.
type Interpreter struct {
	transactions []*AST.Transaction
	plugins      *Plugin.PluginManager
	config       *config.Config
}

// NewInterpreter creates an Interpreter and registers built-in plugins.
func NewInterpreter(cfg *config.Config) *Interpreter {
	i := &Interpreter{
		transactions: []*AST.Transaction{},
		plugins:      Plugin.NewPluginManager(),
		config:       cfg,
	}
	i.plugins.Register(TemplatePlugin.NewTemplatePlugin(), nil) //nolint:errcheck
	return i
}

// ---------------------------------------------------------------------------
// Journal name helpers
// ---------------------------------------------------------------------------

// journalStem extracts the bare name stem and data directory from
// config.DataFile (e.g. "~/.doublebook/data.journal" → "data", "~/.doublebook").
func (i *Interpreter) journalStem() (name, dir string) {
	expanded := utils.ExpandHome(i.config.DataFile)
	dir = filepath.Dir(expanded)
	base := filepath.Base(expanded)
	name = strings.TrimSuffix(base, ".journal")
	return
}

// ---------------------------------------------------------------------------
// Loading
// ---------------------------------------------------------------------------

// LoadJournal loads all journal parts for the configured journal name
// (e.g. data.journal, data.1.journal, …) and fires the OnParse plugin hooks.
func (i *Interpreter) LoadJournal(name string) error {
	stem := name
	if stem == "" {
		stem, _ = i.journalStem()
	}
	_, dir := i.journalStem()

	txns, err := journal.Load(stem, dir)
	if err != nil {
		return fmt.Errorf("loading journal %q: %w", stem, err)
	}

	for _, txn := range txns {
		if err := i.plugins.ExecuteOnParse(txn); err != nil {
			return fmt.Errorf("plugin OnParse error: %w", err)
		}
	}

	i.transactions = txns
	return nil
}

// LoadFromFile loads a single journal file at an exact path.
// Kept for backward compatibility with existing callers (TUI, legacy CLI).
func (i *Interpreter) LoadFromFile(filename string) error {
	txns, err := journal.LoadFromPath(filename)
	if err != nil {
		// File not found is non-fatal — start with empty journal.
		if strings.Contains(err.Error(), "no such file") {
			i.transactions = []*AST.Transaction{}
			return nil
		}
		return fmt.Errorf("loading file %q: %w", filename, err)
	}

	for _, txn := range txns {
		if err := i.plugins.ExecuteOnParse(txn); err != nil {
			return fmt.Errorf("plugin OnParse error: %w", err)
		}
	}

	i.transactions = txns
	return nil
}

// ---------------------------------------------------------------------------
// Saving / Appending
// ---------------------------------------------------------------------------

// SaveToFile writes ALL in-memory transactions to an exact file path,
// overwriting it atomically.  Used by the TUI on quit.
func (i *Interpreter) SaveToFile(filename string) error {
	return journal.WriteAll(filename, i.transactions)
}

// AppendTransaction persists txn to the journal using the journal package's
// size-aware append logic (handles file splitting at 1 MB).
func (i *Interpreter) AppendTransaction(txn *AST.Transaction) error {
	name, dir := i.journalStem()
	return journal.AppendTransaction(name, dir, txn)
}

// ---------------------------------------------------------------------------
// In-memory add
// ---------------------------------------------------------------------------

// AddTransaction validates txn, fires plugin hooks, adds it to the in-memory
// slice, and keeps the slice sorted by date.
// It does NOT persist to disk — call AppendTransaction or SaveToFile for that.
func (i *Interpreter) AddTransaction(txn *AST.Transaction) error {
	if !txn.IsBalanced() {
		return fmt.Errorf("transaction %q is not balanced (sum: %.4f)", txn.Description, txn.Balance())
	}

	if err := i.plugins.ExecuteOnAdd(txn); err != nil {
		return fmt.Errorf("plugin OnAdd error: %w", err)
	}

	i.transactions = append(i.transactions, txn)
	sort.Slice(i.transactions, func(a, b int) bool {
		return i.transactions[a].Date.Before(i.transactions[b].Date)
	})
	return nil
}

// GetTransactions returns all in-memory transactions (unsorted copy of slice).
func (i *Interpreter) GetTransactions() []*AST.Transaction {
	return i.transactions
}

// GetConfig returns the interpreter's loaded configuration.
func (i *Interpreter) GetConfig() *config.Config {
	return i.config
}

// ---------------------------------------------------------------------------
// Filter
// ---------------------------------------------------------------------------

// Filter holds optional criteria for narrowing down the transaction set.
// Zero values mean "no restriction".
type Filter struct {
	BeginDate   string            // "YYYY-MM-DD" inclusive lower bound
	EndDate     string            // "YYYY-MM-DD" inclusive upper bound
	Account     string            // substring match against any posting account
	Description string            // case-insensitive substring match
	Tags        map[string]string // all specified tags must be present and equal
}

// FilteredTransactions returns the subset of transactions that satisfy f.
func (i *Interpreter) FilteredTransactions(f Filter) []*AST.Transaction {
	var out []*AST.Transaction
	for _, txn := range i.transactions {
		if !matchesFilter(txn, f) {
			continue
		}
		out = append(out, txn)
	}
	return out
}

func matchesFilter(txn *AST.Transaction, f Filter) bool {
	dateStr := txn.Date.Format("2006-01-02")

	if f.BeginDate != "" && dateStr < f.BeginDate {
		return false
	}
	if f.EndDate != "" && dateStr > f.EndDate {
		return false
	}
	if f.Description != "" &&
		!strings.Contains(strings.ToLower(txn.Description), strings.ToLower(f.Description)) {
		return false
	}
	// All required tags must match.
	for k, v := range f.Tags {
		if txn.Tags[k] != v {
			return false
		}
	}
	// At least one posting must match the account filter.
	if f.Account != "" {
		found := false
		for _, p := range txn.Postings {
			if strings.Contains(p.Account, f.Account) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Flat balance calculation (backward compatible)
// ---------------------------------------------------------------------------

// CalculateBalances returns a flat map of account name → balance value,
// including ALL transactions (no filter).
func (i *Interpreter) CalculateBalances() map[string]float64 {
	balances := make(map[string]float64)
	for _, txn := range i.transactions {
		for _, p := range txn.Postings {
			balances[p.Account] += p.Amount.Value
		}
	}
	return balances
}

// ---------------------------------------------------------------------------
// Account hierarchy tree
// ---------------------------------------------------------------------------

// AccountNode is one node in the account hierarchy tree.
type AccountNode struct {
	Name     string     // last component of the account name (e.g. "groceries")
	FullName string     // full dotted path (e.g. "expenses:food:groceries")
	Amount   AST.Amount // total balance (including all descendants)
	Children []*AccountNode
}

// CalculateBalancesTree returns account balances organised as a hierarchy.
// The returned map is keyed by full account name and every node's Amount
// already includes the sum of all its descendants.
func (i *Interpreter) CalculateBalancesTree(f Filter) map[string]*AccountNode {
	txns := i.FilteredTransactions(f)

	// Step 1: leaf amounts (per full account name).
	type accAmount struct {
		value    float64
		currency string
	}
	leafAmounts := make(map[string]accAmount)
	for _, txn := range txns {
		for _, p := range txn.Postings {
			a := leafAmounts[p.Account]
			a.value += p.Amount.Value
			if a.currency == "" {
				a.currency = p.Amount.Currency
			}
			leafAmounts[p.Account] = a
		}
	}

	// Step 2: create/update nodes for each account prefix, propagating
	//         the leaf amount upward to all ancestors.
	nodes := make(map[string]*AccountNode)

	for acc, la := range leafAmounts {
		parts := strings.Split(acc, ":")
		for depth := 1; depth <= len(parts); depth++ {
			fullName := strings.Join(parts[:depth], ":")
			if _, ok := nodes[fullName]; !ok {
				nodes[fullName] = &AccountNode{
					Name:     parts[depth-1],
					FullName: fullName,
					Amount:   AST.Amount{Currency: la.currency},
				}
			}
			nodes[fullName].Amount.Value += la.value
			if nodes[fullName].Amount.Currency == "" {
				nodes[fullName].Amount.Currency = la.currency
			}
		}
	}

	// Step 3: wire parent → child relationships.
	for fullName, node := range nodes {
		parts := strings.Split(fullName, ":")
		if len(parts) < 2 {
			continue
		}
		parentName := strings.Join(parts[:len(parts)-1], ":")
		parent, ok := nodes[parentName]
		if !ok {
			continue
		}
		// Add only if not already present.
		found := false
		for _, c := range parent.Children {
			if c.FullName == fullName {
				found = true
				break
			}
		}
		if !found {
			parent.Children = append(parent.Children, node)
		}
	}

	// Step 4: sort children alphabetically.
	for _, node := range nodes {
		sort.Slice(node.Children, func(a, b int) bool {
			return node.Children[a].Name < node.Children[b].Name
		})
	}

	return nodes
}

// GroupAccountsByType partitions top-level AccountNodes by their account type
// (the root component of the full name).
// Returns a map of type → sorted nodes.
// Recognised types: "assets", "liabilities", "equity", "income", "expenses".
// Everything else goes into "other".
func GroupAccountsByType(nodes map[string]*AccountNode) map[string][]*AccountNode {
	known := map[string]bool{
		"assets": true, "liabilities": true, "equity": true,
		"income": true, "expenses": true,
	}
	groups := make(map[string][]*AccountNode)

	for fullName, node := range nodes {
		// Only root nodes (no ':' in full name).
		if strings.Contains(fullName, ":") {
			continue
		}
		group := fullName
		if !known[group] {
			group = "other"
		}
		groups[group] = append(groups[group], node)
	}

	for _, slice := range groups {
		sort.Slice(slice, func(a, b int) bool {
			return slice[a].FullName < slice[b].FullName
		})
	}
	return groups
}

// ---------------------------------------------------------------------------
// Income Statement
// ---------------------------------------------------------------------------

// IncomeStatement holds a summary of revenues vs expenses for a period.
type IncomeStatement struct {
	Period    string
	Revenues  map[string]AST.Amount // account → positive revenue amount
	Expenses  map[string]AST.Amount // account → positive expense amount
	NetIncome AST.Amount
}

// GenerateIncomeStatement calculates an income statement for the transactions
// matching the given filter.
//
// Income accounts (income:*) are negated so they appear as positive revenues.
// Expense accounts (expenses:*) are shown as positive values.
func (i *Interpreter) GenerateIncomeStatement(f Filter) *IncomeStatement {
	txns := i.FilteredTransactions(f)

	stmt := &IncomeStatement{
		Revenues: make(map[string]AST.Amount),
		Expenses: make(map[string]AST.Amount),
	}

	currency := i.config.Currency

	for _, txn := range txns {
		for _, p := range txn.Postings {
			acc := p.Account
			amt := p.Amount

			switch {
			case strings.HasPrefix(acc, "income") && (len(acc) == 6 || acc[6] == ':'):
				// Negate: income postings are credits (negative) in double-entry.
				cur := stmt.Revenues[acc]
				cur.Value += -amt.Value
				if cur.Currency == "" {
					cur.Currency = amt.Currency
				}
				stmt.Revenues[acc] = cur

			case strings.HasPrefix(acc, "expenses") && (len(acc) == 8 || acc[8] == ':'):
				cur := stmt.Expenses[acc]
				cur.Value += amt.Value
				if cur.Currency == "" {
					cur.Currency = amt.Currency
				}
				stmt.Expenses[acc] = cur
			}
		}
	}

	var totalRevenue, totalExpenses float64
	for _, a := range stmt.Revenues {
		totalRevenue += a.Value
	}
	for _, a := range stmt.Expenses {
		totalExpenses += a.Value
	}
	stmt.NetIncome = AST.Amount{
		Value:    totalRevenue - totalExpenses,
		Currency: currency,
	}

	return stmt
}

// ---------------------------------------------------------------------------
// Reports
// ---------------------------------------------------------------------------

// GenerateBalanceReport produces a human-readable balance report grouped by
// account type, using the hierarchy tree for display.
func (i *Interpreter) GenerateBalanceReport() string {
	nodes := i.CalculateBalancesTree(Filter{})
	groups := GroupAccountsByType(nodes)

	var b strings.Builder
	b.WriteString("BALANCE REPORT\n")
	b.WriteString("══════════════════════════════════════════════\n\n")

	order := []string{"assets", "liabilities", "equity", "income", "expenses", "other"}
	grandTotal := 0.0
	currency := i.config.Currency

	for _, acctType := range order {
		rootNodes := groups[acctType]
		if len(rootNodes) == 0 {
			continue
		}
		b.WriteString(strings.ToUpper(acctType) + ":\n")
		for _, root := range rootNodes {
			writeNodeLines(&b, nodes, root, 1)
		}
		b.WriteString("\n")
	}

	// Grand total line.
	for _, txn := range i.transactions {
		for _, p := range txn.Postings {
			grandTotal += p.Amount.Value
		}
	}
	b.WriteString("──────────────────────────────────────────────\n")
	total := AST.Amount{Value: grandTotal, Currency: currency}
	b.WriteString(fmt.Sprintf("%46s\n", total.String()))

	return b.String()
}

// writeNodeLines recursively appends a node and its children to b, indented
// by depth * 2 spaces.  Only the node's own line is printed (children follow).
func writeNodeLines(b *strings.Builder, allNodes map[string]*AccountNode, node *AccountNode, depth int) {
	indent := strings.Repeat("  ", depth)
	// Right-align the amount in a 12-char field.
	line := fmt.Sprintf("%s%-36s  %s\n",
		indent,
		node.FullName,
		node.Amount.String(),
	)
	b.WriteString(line)

	// Only print children if this node has more than one sub-level, to avoid
	// redundant lines for single-child branches.
	for _, child := range node.Children {
		writeNodeLines(b, allNodes, child, depth+1)
	}
}

// GetPluginReports collects OnReport output from all registered plugins.
func (i *Interpreter) GetPluginReports() []string {
	return i.plugins.ExecuteOnReport(i.transactions)
}

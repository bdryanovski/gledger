package commands

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"doublebook/config"
	Interpreter "doublebook/interpreter"
)

// BalanceCommand prints account balances, either as a flat sorted list or as
// an indented account hierarchy.
func BalanceCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("balance", flag.ContinueOnError)
	treeMode := fs.Bool("tree", false, "Show account hierarchy")
	beginFlag := fs.String("begin", "", "Only include transactions on or after DATE")
	endFlag := fs.String("end", "", "Only include transactions on or before DATE")
	accountFlag := fs.String("account", "", "Only show accounts matching this pattern")
	noTotal := fs.Bool("no-total", false, "Omit the grand-total line")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Command-level dates override the global context flags.
	beginDate := ctx.BeginDate
	if *beginFlag != "" {
		d, err := normalizeDate(*beginFlag)
		if err != nil {
			return fmt.Errorf("--begin: %w", err)
		}
		beginDate = d
	}
	endDate := ctx.EndDate
	if *endFlag != "" {
		d, err := normalizeDate(*endFlag)
		if err != nil {
			return fmt.Errorf("--end: %w", err)
		}
		endDate = d
	}

	interp := Interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		return fmt.Errorf("loading journal: %w", err)
	}

	filter := Interpreter.Filter{
		BeginDate: beginDate,
		EndDate:   endDate,
	}

	accountFilter := *accountFlag

	if *treeMode {
		return printTreeBalance(interp, filter, accountFilter, *noTotal, ctx.Config.Currency)
	}
	return printFlatBalance(interp, filter, accountFilter, *noTotal, ctx.Config.Currency)
}

// ---------------------------------------------------------------------------
// Flat balance
// ---------------------------------------------------------------------------

func printFlatBalance(
	interp *Interpreter.Interpreter,
	filter Interpreter.Filter,
	accountFilter string,
	noTotal bool,
	currency string,
) error {
	// Compute per-account balances using filtered transactions.
	txns := interp.FilteredTransactions(filter)
	balances := make(map[string]float64)
	balanceCurrency := make(map[string]string)
	for _, txn := range txns {
		for _, p := range txn.Postings {
			balances[p.Account] += p.Amount.Value
			if balanceCurrency[p.Account] == "" {
				balanceCurrency[p.Account] = p.Amount.Currency
			}
		}
	}

	// Build display rows: skip zero balances and apply account filter.
	type row struct {
		account string
		amtStr  string
	}
	var rows []row
	var accounts []string
	for acc := range balances {
		accounts = append(accounts, acc)
	}
	sort.Strings(accounts)

	grandTotal := 0.0
	for _, acc := range accounts {
		val := balances[acc]
		if val == 0 {
			continue
		}
		if accountFilter != "" && !strings.Contains(acc, accountFilter) {
			continue
		}
		cur := balanceCurrency[acc]
		if cur == "" {
			cur = currency
		}
		rows = append(rows, row{
			account: acc,
			amtStr:  formatAmount(val, cur),
		})
		grandTotal += val
	}

	if len(rows) == 0 {
		fmt.Println("No accounts found.")
		return nil
	}

	// Two-pass: find max amount width.
	maxW := 0
	for _, r := range rows {
		if len(r.amtStr) > maxW {
			maxW = len(r.amtStr)
		}
	}
	totalStr := formatAmount(grandTotal, currency)
	if len(totalStr) > maxW {
		maxW = len(totalStr)
	}

	// Render.
	for _, r := range rows {
		fmt.Printf("%*s  %s\n", maxW, r.amtStr, r.account)
	}

	if !noTotal {
		sep := strings.Repeat("─", maxW+2+20)
		fmt.Println(sep)
		fmt.Printf("%*s\n", maxW, totalStr)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Tree balance
// ---------------------------------------------------------------------------

// treeRow holds a pre-computed display line for the tree render.
type treeRow struct {
	depth  int
	name   string // short name (last component)
	full   string // full account path
	amtStr string
}

func printTreeBalance(
	interp *Interpreter.Interpreter,
	filter Interpreter.Filter,
	accountFilter string,
	noTotal bool,
	currency string,
) error {
	nodes := interp.CalculateBalancesTree(filter)
	groups := Interpreter.GroupAccountsByType(nodes)

	// Ordered account types for display.
	order := []string{"assets", "liabilities", "equity", "income", "expenses", "other"}

	// Collect all rows in display order (two passes).
	var rows []treeRow
	grandTotal := 0.0

	for _, acctType := range order {
		rootNodes := groups[acctType]
		for _, root := range rootNodes {
			// Apply account filter at root level.
			if accountFilter != "" && !strings.Contains(root.FullName, accountFilter) {
				continue
			}
			if root.Amount.Value == 0 {
				continue
			}
			collectTreeRows(root, 0, &rows)
			grandTotal += root.Amount.Value
		}
	}

	if len(rows) == 0 {
		fmt.Println("No accounts found.")
		return nil
	}

	// First pass: find max amount string width.
	maxW := 0
	for _, r := range rows {
		if len(r.amtStr) > maxW {
			maxW = len(r.amtStr)
		}
	}
	totalStr := formatAmount(grandTotal, currency)
	if len(totalStr) > maxW {
		maxW = len(totalStr)
	}

	// Second pass: render with consistent alignment.
	for _, r := range rows {
		indent := strings.Repeat("  ", r.depth)
		fmt.Printf("%*s  %s%s\n", maxW, r.amtStr, indent, r.name)
	}

	if !noTotal {
		sep := strings.Repeat("─", maxW+2+20)
		fmt.Println(sep)
		fmt.Printf("%*s\n", maxW, totalStr)
	}

	return nil
}

// collectTreeRows recursively adds nodes to rows, skipping zero-balance nodes.
func collectTreeRows(node *Interpreter.AccountNode, depth int, rows *[]treeRow) {
	if node.Amount.Value == 0 {
		return
	}
	*rows = append(*rows, treeRow{
		depth:  depth,
		name:   node.Name,
		full:   node.FullName,
		amtStr: node.Amount.String(),
	})
	for _, child := range node.Children {
		collectTreeRows(child, depth+1, rows)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// formatAmount renders a float64 + currency code as a display string,
// mirroring the logic in AST.Amount.String() to avoid importing the ast
// package directly.
func formatAmount(value float64, currency string) string {
	neg := value < 0
	abs := value
	if neg {
		abs = -abs
	}

	// Build "1,234.56" from the absolute value.
	raw := fmt.Sprintf("%.2f", abs)
	parts := strings.SplitN(raw, ".", 2)
	intPart := parts[0]
	var b strings.Builder
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	num := b.String() + "." + parts[1]

	sign := ""
	if neg {
		sign = "-"
	}
	switch currency {
	case "USD", "$", "":
		return sign + "$" + num
	case "GBP", "£":
		return sign + "£" + num
	case "EUR", "€":
		return sign + "€" + num
	default:
		return sign + num + " " + currency
	}
}

// normalizeDate is a local alias so this file doesn't need to import cli.
func normalizeDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD or YYYY/MM/DD", s)
	}
	n := strings.ReplaceAll(s, "/", "-")
	if n[4] != '-' || n[7] != '-' {
		return "", fmt.Errorf("invalid date %q", s)
	}
	return n, nil
}

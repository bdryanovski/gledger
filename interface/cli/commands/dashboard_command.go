package commands

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"doublebook/core/ast"
	"doublebook/infra/config"
	"doublebook/engine/fql"
	"doublebook/engine/interpreter"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Dashboard styles
var (
	dashHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("75")).
			MarginBottom(1)

	dashSectionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				MarginTop(1)

	dashBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	dashPosStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))  // green
	dashNegStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // red
	dashDimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	dashValStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

// DashboardCommand displays a financial dashboard with charts and summaries.
func DashboardCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	monthsFlag := fs.Int("months", 6, "Number of months to show in trends")
	beginFlag := fs.String("begin", "", "Start date (YYYY-MM-DD)")
	endFlag := fs.String("end", "", "End date (YYYY-MM-DD)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Determine date range
	endDate := ctx.EndDate
	if *endFlag != "" {
		d, err := normalizeDate(*endFlag)
		if err != nil {
			return fmt.Errorf("--end: %w", err)
		}
		endDate = d
	}
	if endDate == "" {
		endDate = time.Now().Format("2006-01-02")
	}

	beginDate := ctx.BeginDate
	if *beginFlag != "" {
		d, err := normalizeDate(*beginFlag)
		if err != nil {
			return fmt.Errorf("--begin: %w", err)
		}
		beginDate = d
	}
	if beginDate == "" {
		// Default to N months ago
		t, _ := time.Parse("2006-01-02", endDate)
		beginDate = t.AddDate(0, -*monthsFlag, 0).Format("2006-01-02")
	}

	// Get terminal width
	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w
	}

	// Load data
	interp := interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		return fmt.Errorf("loading journal: %w", err)
	}

	filter := interpreter.Filter{
		BeginDate: beginDate,
		EndDate:   endDate,
	}
	txns := interp.FilteredTransactions(filter)

	// Print header
	fmt.Println(dashHeaderStyle.Render("╔══════════════════════════════════════════════════════════════╗"))
	fmt.Println(dashHeaderStyle.Render("║                    DOUBLEBOOK DASHBOARD                      ║"))
	fmt.Println(dashHeaderStyle.Render("╚══════════════════════════════════════════════════════════════╝"))
	fmt.Println()
	fmt.Printf("%s %s to %s\n\n",
		dashDimStyle.Render("Period:"),
		dashValStyle.Render(beginDate),
		dashValStyle.Render(endDate))

	// Section 1: Summary Cards
	printSummaryCards(txns, ctx.Config.Currency, width)

	// Section 2: Spending Over Time
	printSpendingTrend(txns, beginDate, endDate, width)

	// Section 3: Balance Changes Over Time
	printBalanceTrend(txns, beginDate, endDate, width)

	// Section 4: Top Expense Categories
	printTopExpenses(txns, ctx.Config.Currency, width)

	// Section 5: Recent Transactions
	printRecentTransactions(txns, ctx.Config.Currency, 8)

	return nil
}

// printSummaryCards shows key financial metrics
func printSummaryCards(txns []*ast.Transaction, currency string, width int) {
	var totalIncome, totalExpenses, netAssets, netLiabilities float64

	for _, txn := range txns {
		for _, p := range txn.Postings {
			switch {
			case strings.HasPrefix(p.Account, "income:"):
				totalIncome += -p.Amount.Value // Income is negative in double-entry
			case strings.HasPrefix(p.Account, "expenses:"):
				totalExpenses += p.Amount.Value
			case strings.HasPrefix(p.Account, "assets:"):
				netAssets += p.Amount.Value
			case strings.HasPrefix(p.Account, "liabilities:"):
				netLiabilities += p.Amount.Value
			}
		}
	}

	netIncome := totalIncome - totalExpenses
	savingsRate := 0.0
	if totalIncome > 0 {
		savingsRate = (netIncome / totalIncome) * 100
	}

	fmt.Println(dashSectionStyle.Render("─── SUMMARY ───────────────────────────────────────────────────"))
	fmt.Println()

	// Build summary cards
	cardWidth := (width - 10) / 4
	if cardWidth < 15 {
		cardWidth = 15
	}

	cards := []struct {
		label string
		value string
		color lipgloss.Style
	}{
		{"Income", formatDashAmount(totalIncome, currency), dashPosStyle},
		{"Expenses", formatDashAmount(totalExpenses, currency), dashNegStyle},
		{"Net", formatDashAmount(netIncome, currency), colorBySign(netIncome)},
		{"Savings Rate", fmt.Sprintf("%.1f%%", savingsRate), colorBySign(savingsRate)},
	}

	for _, card := range cards {
		label := dashDimStyle.Render(fmt.Sprintf("%-12s", card.label))
		value := card.color.Render(card.value)
		fmt.Printf("  %s %s\n", label, value)
	}
	fmt.Println()
}

// printSpendingTrend shows monthly income, expenses, and gain
func printSpendingTrend(txns []*ast.Transaction, beginDate, endDate string, width int) {
	fmt.Println(dashSectionStyle.Render("─── MONTHLY BREAKDOWN ─────────────────────────────────────────"))
	fmt.Println()

	// Aggregate by month
	type monthData struct {
		income   float64
		expenses float64
	}
	monthly := make(map[string]*monthData)

	for _, txn := range txns {
		month := txn.Date.Format("2006-01")
		if monthly[month] == nil {
			monthly[month] = &monthData{}
		}
		for _, p := range txn.Postings {
			if strings.HasPrefix(p.Account, "income:") {
				monthly[month].income += -p.Amount.Value // Income is negative in double-entry
			} else if strings.HasPrefix(p.Account, "expenses:") {
				monthly[month].expenses += p.Amount.Value
			}
		}
	}

	// Sort months
	var months []string
	for m := range monthly {
		months = append(months, m)
	}
	sort.Strings(months)

	if len(months) == 0 {
		fmt.Println(dashDimStyle.Render("  No data for this period"))
		fmt.Println()
		return
	}

	// Find max gain for bar scaling
	maxGain := 0.0
	for _, d := range monthly {
		g := d.income - d.expenses
		if g > maxGain {
			maxGain = g
		}
		if -g > maxGain {
			maxGain = -g
		}
	}

	// Print header (plain text to avoid lipgloss formatting issues)
	fmt.Printf("  %-7s  %12s  %12s  %12s\n", "Month", "Income", "Expenses", "Gain")
	fmt.Printf("  %s\n", strings.Repeat("─", 54))

	// Print each month
	for _, m := range months {
		data := monthly[m]
		gain := data.income - data.expenses

		// Format month name (e.g., "2025-01" -> "Jan 25")
		monthLabel := m
		if t, err := time.Parse("2006-01", m); err == nil {
			monthLabel = t.Format("Jan 06")
		}

		// Format amounts
		incomeStr := formatDashAmount(data.income, "$")
		expenseStr := formatDashAmount(data.expenses, "$")
		gainStr := formatDashAmount(gain, "$")

		// Mini bar for gain visualization
		barWidth := 15
		bar := ""
		if maxGain > 0 && gain != 0 {
			barLen := int((gain / maxGain) * float64(barWidth))
			if barLen < 0 {
				barLen = -barLen
			}
			if barLen > barWidth {
				barLen = barWidth
			}
			if barLen < 1 {
				barLen = 1
			}
			if gain >= 0 {
				bar = dashPosStyle.Render(strings.Repeat("█", barLen))
			} else {
				bar = dashNegStyle.Render(strings.Repeat("█", barLen))
			}
		}

		// Print row with colors
		fmt.Printf("  %-7s  %s  %s  %s  %s\n",
			monthLabel,
			dashPosStyle.Render(fmt.Sprintf("%12s", incomeStr)),
			dashNegStyle.Render(fmt.Sprintf("%12s", expenseStr)),
			colorBySign(gain).Render(fmt.Sprintf("%12s", gainStr)),
			bar)
	}

	fmt.Println()
}

// printBalanceTrend shows running balance over time
func printBalanceTrend(txns []*ast.Transaction, beginDate, endDate string, width int) {
	fmt.Println(dashSectionStyle.Render("─── BALANCE TREND ─────────────────────────────────────────────"))
	fmt.Println()

	// Sort transactions by date
	sortedTxns := make([]*ast.Transaction, len(txns))
	copy(sortedTxns, txns)
	sort.Slice(sortedTxns, func(i, j int) bool {
		return sortedTxns[i].Date.Before(sortedTxns[j].Date)
	})

	// Calculate daily net worth (assets - liabilities)
	dailyBalance := make(map[string]float64)
	runningBalance := 0.0

	for _, txn := range sortedTxns {
		day := txn.Date.Format("2006-01-02")
		for _, p := range txn.Postings {
			if strings.HasPrefix(p.Account, "assets:") {
				runningBalance += p.Amount.Value
			} else if strings.HasPrefix(p.Account, "liabilities:") {
				runningBalance -= p.Amount.Value
			}
		}
		dailyBalance[day] = runningBalance
	}

	// Aggregate by month for display
	monthlyBalance := make(map[string]float64)
	for day, bal := range dailyBalance {
		month := day[:7]
		monthlyBalance[month] = bal // Take last day's balance for the month
	}

	var months []string
	for m := range monthlyBalance {
		months = append(months, m)
	}
	sort.Strings(months)

	if len(months) == 0 {
		fmt.Println(dashDimStyle.Render("  No balance data for this period"))
		fmt.Println()
		return
	}

	// Print sparkline
	values := make([]float64, len(months))
	for i, m := range months {
		values[i] = monthlyBalance[m]
	}

	sparkline := fql.RenderSparkline(values)
	fmt.Printf("  %s → %s\n", months[0], months[len(months)-1])
	fmt.Printf("  %s\n", sparkline)

	// Show start and end balance
	startBal := values[0]
	endBal := values[len(values)-1]
	change := endBal - startBal
	changeStr := formatDashAmount(change, "$")
	changeStyle := colorBySign(change)

	fmt.Printf("\n  %s %s  →  %s %s  (%s)\n",
		dashDimStyle.Render("Start:"),
		formatDashAmount(startBal, "$"),
		dashDimStyle.Render("End:"),
		formatDashAmount(endBal, "$"),
		changeStyle.Render(changeStr))
	fmt.Println()
}

// printTopExpenses shows top expense categories
func printTopExpenses(txns []*ast.Transaction, currency string, width int) {
	fmt.Println(dashSectionStyle.Render("─── TOP EXPENSES ──────────────────────────────────────────────"))
	fmt.Println()

	// Aggregate by top-level expense category
	categorySpending := make(map[string]float64)
	for _, txn := range txns {
		for _, p := range txn.Postings {
			if strings.HasPrefix(p.Account, "expenses:") {
				// Get second level (e.g., "expenses:food" from "expenses:food:groceries")
				parts := strings.Split(p.Account, ":")
				category := p.Account
				if len(parts) >= 2 {
					category = parts[0] + ":" + parts[1]
				}
				categorySpending[category] += p.Amount.Value
			}
		}
	}

	// Sort by amount
	type catAmount struct {
		category string
		amount   float64
	}
	var sorted []catAmount
	for cat, amt := range categorySpending {
		sorted = append(sorted, catAmount{cat, amt})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].amount > sorted[j].amount
	})

	// Take top 8
	if len(sorted) > 8 {
		sorted = sorted[:8]
	}

	if len(sorted) == 0 {
		fmt.Println(dashDimStyle.Render("  No expense categories found"))
		fmt.Println()
		return
	}

	// Create chart
	result := &fql.ChartResult{
		Type:   fql.ChartBar,
		Labels: make([]string, len(sorted)),
		Values: make([]float64, len(sorted)),
	}
	for i, ca := range sorted {
		// Shorten category name
		label := ca.category
		if strings.HasPrefix(label, "expenses:") {
			label = label[9:]
		}
		result.Labels[i] = label
		result.Values[i] = ca.amount
	}

	fmt.Print(fql.RenderBarChart(result, width-4))
	fmt.Println()
}

// printRecentTransactions shows the most recent transactions
func printRecentTransactions(txns []*ast.Transaction, currency string, limit int) {
	fmt.Println(dashSectionStyle.Render("─── RECENT TRANSACTIONS ───────────────────────────────────────"))
	fmt.Println()

	if len(txns) == 0 {
		fmt.Println(dashDimStyle.Render("  No transactions found"))
		fmt.Println()
		return
	}

	// Sort by date descending
	sorted := make([]*ast.Transaction, len(txns))
	copy(sorted, txns)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.After(sorted[j].Date)
	})

	if len(sorted) > limit {
		sorted = sorted[:limit]
	}

	for _, txn := range sorted {
		date := txn.Date.Format("Jan 02")
		desc := txn.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		// Find primary amount (first posting)
		var amount float64
		var account string
		for _, p := range txn.Postings {
			if !strings.HasPrefix(p.Account, "assets:") &&
				!strings.HasPrefix(p.Account, "liabilities:") {
				amount = p.Amount.Value
				account = p.Account
				break
			}
		}

		// Color based on whether it's income or expense
		amtStr := formatDashAmount(amount, currency)
		amtStyle := colorBySign(-amount) // Flip for display

		// Shorten account
		if len(account) > 20 {
			parts := strings.Split(account, ":")
			if len(parts) > 1 {
				account = parts[len(parts)-1]
			}
		}

		fmt.Printf("  %s  %-30s  %s  %s\n",
			dashDimStyle.Render(date),
			desc,
			amtStyle.Render(fmt.Sprintf("%10s", amtStr)),
			dashDimStyle.Render(account))
	}
	fmt.Println()
}

// Helper functions

func colorBySign(v float64) lipgloss.Style {
	if v >= 0 {
		return dashPosStyle
	}
	return dashNegStyle
}

func formatDashAmount(value float64, currency string) string {
	neg := value < 0
	abs := value
	if neg {
		abs = -abs
	}

	// Format with commas
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

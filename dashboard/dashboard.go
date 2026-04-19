// Package dashboard provides reusable dashboard data structures and FQL queries
// for generating financial summaries and visualizations.
package dashboard

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"doublebook/ast"
)

// ---------------------------------------------------------------------------
// Data Structures
// ---------------------------------------------------------------------------

// Summary holds high-level financial metrics for a period.
type Summary struct {
	BeginDate   string
	EndDate     string
	TotalIncome float64
	TotalExpenses float64
	NetIncome   float64
	SavingsRate float64
	NetAssets   float64
	NetLiabilities float64
}

// MonthData holds income, expenses, and gain for a single month.
type MonthData struct {
	Month    string  // Format: "2006-01"
	Label    string  // Format: "Jan 06"
	Income   float64
	Expenses float64
	Gain     float64
}

// CategoryData holds spending for a category.
type CategoryData struct {
	Category string
	Amount   float64
}

// TransactionSummary holds a simplified transaction for display.
type TransactionSummary struct {
	Date        string
	Description string
	Amount      float64
	Account     string
}

// DashboardData holds all computed dashboard data.
type DashboardData struct {
	Summary      Summary
	Monthly      []MonthData
	TopExpenses  []CategoryData
	Recent       []TransactionSummary
	BalanceTrend []float64 // Monthly balance values for sparkline
}

// ---------------------------------------------------------------------------
// FQL Queries - These can be run directly with `doublebook fql`
// ---------------------------------------------------------------------------

// FQLQueries contains the FQL queries used to generate dashboard data.
// Users can run these directly in the FQL REPL or CLI.
var FQLQueries = struct {
	// MonthlySummary shows income, expenses, and gain by month
	MonthlySummary string

	// TopExpenseCategories shows top spending categories
	TopExpenseCategories string

	// MonthlyIncome shows income by month
	MonthlyIncome string

	// MonthlyExpenses shows expenses by month
	MonthlyExpenses string

	// RecentTransactions shows recent transactions
	RecentTransactions string

	// AccountBalances shows current balances
	AccountBalances string

	// SpendingTrend shows spending over time
	SpendingTrend string
}{
	MonthlySummary: `
SELECT 
    month,
    SUM(CASE WHEN account LIKE 'income:%' THEN -total_amount ELSE 0 END) AS income,
    SUM(CASE WHEN account LIKE 'expenses:%' THEN total_amount ELSE 0 END) AS expenses
FROM spending
GROUP BY month
ORDER BY month DESC
LIMIT 12`,

	TopExpenseCategories: `
SELECT account, SUM(amount) AS total
FROM transactions
WHERE account LIKE 'expenses:%'
GROUP BY account
ORDER BY total DESC
LIMIT 10`,

	MonthlyIncome: `
SELECT month, SUM(-total_amount) AS income
FROM spending
WHERE account LIKE 'income:%'
GROUP BY month
ORDER BY month DESC
LIMIT 12`,

	MonthlyExpenses: `
SELECT month, SUM(total_amount) AS expenses
FROM spending
WHERE account LIKE 'expenses:%'
GROUP BY month
ORDER BY month DESC
LIMIT 12`,

	RecentTransactions: `
SELECT date, description, account, amount
FROM transactions
ORDER BY date DESC
LIMIT 20`,

	AccountBalances: `
SELECT account, total_amount AS balance
FROM accounts
ORDER BY total_amount DESC`,

	SpendingTrend: `
SELECT month, SUM(total_amount) AS spending
FROM spending
WHERE account LIKE 'expenses:%'
GROUP BY month
ORDER BY month`,
}

// ---------------------------------------------------------------------------
// Data Computation
// ---------------------------------------------------------------------------

// ComputeDashboard calculates all dashboard data from transactions.
func ComputeDashboard(txns []*ast.Transaction, beginDate, endDate string) *DashboardData {
	data := &DashboardData{}

	// Compute summary
	data.Summary = computeSummary(txns, beginDate, endDate)

	// Compute monthly breakdown
	data.Monthly = computeMonthly(txns)

	// Compute top expenses
	data.TopExpenses = computeTopExpenses(txns, 8)

	// Compute recent transactions
	data.Recent = computeRecent(txns, 10)

	// Compute balance trend
	data.BalanceTrend = computeBalanceTrend(txns)

	return data
}

func computeSummary(txns []*ast.Transaction, beginDate, endDate string) Summary {
	s := Summary{
		BeginDate: beginDate,
		EndDate:   endDate,
	}

	for _, txn := range txns {
		for _, p := range txn.Postings {
			switch {
			case strings.HasPrefix(p.Account, "income:"):
				s.TotalIncome += -p.Amount.Value
			case strings.HasPrefix(p.Account, "expenses:"):
				s.TotalExpenses += p.Amount.Value
			case strings.HasPrefix(p.Account, "assets:"):
				s.NetAssets += p.Amount.Value
			case strings.HasPrefix(p.Account, "liabilities:"):
				s.NetLiabilities += p.Amount.Value
			}
		}
	}

	s.NetIncome = s.TotalIncome - s.TotalExpenses
	if s.TotalIncome > 0 {
		s.SavingsRate = (s.NetIncome / s.TotalIncome) * 100
	}

	return s
}

func computeMonthly(txns []*ast.Transaction) []MonthData {
	monthly := make(map[string]*MonthData)

	for _, txn := range txns {
		month := txn.Date.Format("2006-01")
		if monthly[month] == nil {
			label := month
			if t, err := time.Parse("2006-01", month); err == nil {
				label = t.Format("Jan 06")
			}
			monthly[month] = &MonthData{Month: month, Label: label}
		}
		for _, p := range txn.Postings {
			if strings.HasPrefix(p.Account, "income:") {
				monthly[month].Income += -p.Amount.Value
			} else if strings.HasPrefix(p.Account, "expenses:") {
				monthly[month].Expenses += p.Amount.Value
			}
		}
	}

	// Calculate gain and sort
	var result []MonthData
	for _, m := range monthly {
		m.Gain = m.Income - m.Expenses
		result = append(result, *m)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Month < result[j].Month
	})

	return result
}

func computeTopExpenses(txns []*ast.Transaction, limit int) []CategoryData {
	categories := make(map[string]float64)

	for _, txn := range txns {
		for _, p := range txn.Postings {
			if strings.HasPrefix(p.Account, "expenses:") {
				// Get second level category
				parts := strings.Split(p.Account, ":")
				category := p.Account
				if len(parts) >= 2 {
					category = parts[0] + ":" + parts[1]
				}
				categories[category] += p.Amount.Value
			}
		}
	}

	var result []CategoryData
	for cat, amt := range categories {
		// Clean up category name for display
		displayName := cat
		if strings.HasPrefix(displayName, "expenses:") {
			displayName = displayName[9:]
		}
		result = append(result, CategoryData{Category: displayName, Amount: amt})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Amount > result[j].Amount
	})

	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

func computeRecent(txns []*ast.Transaction, limit int) []TransactionSummary {
	// Sort by date descending
	sorted := make([]*ast.Transaction, len(txns))
	copy(sorted, txns)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.After(sorted[j].Date)
	})

	var result []TransactionSummary
	for _, txn := range sorted {
		if len(result) >= limit {
			break
		}

		// Find the primary posting (non-asset/liability)
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

		// Shorten account name
		if len(account) > 20 {
			parts := strings.Split(account, ":")
			if len(parts) > 1 {
				account = parts[len(parts)-1]
			}
		}

		result = append(result, TransactionSummary{
			Date:        txn.Date.Format("Jan 02"),
			Description: truncate(txn.Description, 30),
			Amount:      amount,
			Account:     account,
		})
	}

	return result
}

func computeBalanceTrend(txns []*ast.Transaction) []float64 {
	// Sort by date
	sorted := make([]*ast.Transaction, len(txns))
	copy(sorted, txns)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	// Calculate monthly balances
	monthlyBalance := make(map[string]float64)
	runningBalance := 0.0

	for _, txn := range sorted {
		month := txn.Date.Format("2006-01")
		for _, p := range txn.Postings {
			if strings.HasPrefix(p.Account, "assets:") {
				runningBalance += p.Amount.Value
			} else if strings.HasPrefix(p.Account, "liabilities:") {
				runningBalance -= p.Amount.Value
			}
		}
		monthlyBalance[month] = runningBalance
	}

	// Sort months and extract values
	var months []string
	for m := range monthlyBalance {
		months = append(months, m)
	}
	sort.Strings(months)

	var values []float64
	for _, m := range months {
		values = append(values, monthlyBalance[m])
	}

	return values
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ---------------------------------------------------------------------------
// Formatting Helpers
// ---------------------------------------------------------------------------

// FormatAmount formats a float as a currency string.
func FormatAmount(value float64, currency string) string {
	neg := value < 0
	abs := value
	if neg {
		abs = -abs
	}

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

// Package recurring provides the recurring-payments plugin which tracks
// scheduled recurring expenses and generates reports on actual vs projected spend.
package recurring

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	AST "doublebook/ast"
	Plugin "doublebook/plugin"
	"doublebook/utils"
)

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

// Schedule describes one recurring payment.
type Schedule struct {
	ID            string            `json:"id"`
	Description   string            `json:"description"`
	Amount        float64           `json:"amount"`
	Currency      string            `json:"currency"`
	DebitAccount  string            `json:"debit_account"`
	CreditAccount string            `json:"credit_account"`
	Frequency     string            `json:"frequency"` // daily|weekly|monthly|yearly
	DayOfMonth    int               `json:"day_of_month"`
	DayOfWeek     int               `json:"day_of_week"` // 0=Sun … 6=Sat
	Month         int               `json:"month"`
	IntervalDays  int               `json:"interval_days"`
	StartDate     string            `json:"start_date"`
	EndDate       *string           `json:"end_date"`
	Tags          map[string]string `json:"tags"`
	Active        bool              `json:"active"`
}

// RecurringConfig is the top-level structure of recurring.json.
type RecurringConfig struct {
	Schedules []Schedule `json:"schedules"`
}

// ---------------------------------------------------------------------------
// RecurringPlugin
// ---------------------------------------------------------------------------

// RecurringPlugin manages recurring payment schedules.
type RecurringPlugin struct {
	Plugin.DefaultPlugin
	configPath string
	config     *RecurringConfig
}

func (p *RecurringPlugin) Name() string    { return "recurring" }
func (p *RecurringPlugin) Version() string { return "1.0.0" }
func (p *RecurringPlugin) Description() string {
	return "Track and report on recurring payment schedules"
}

func (p *RecurringPlugin) Initialize(cfg map[string]interface{}) error {
	if cfg != nil {
		if dir, ok := cfg["config_dir"].(string); ok {
			p.configPath = filepath.Join(utils.ExpandHome(dir), "recurring.json")
		}
		if path, ok := cfg["config_path"].(string); ok {
			p.configPath = utils.ExpandHome(path)
		}
	}
	if p.configPath == "" {
		home, _ := os.UserHomeDir()
		p.configPath = filepath.Join(home, ".doublebook", "recurring.json")
	}
	return p.loadConfig()
}

func (p *RecurringPlugin) loadConfig() error {
	data, err := os.ReadFile(p.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			p.config = &RecurringConfig{}
			return nil
		}
		return fmt.Errorf("reading %q: %w", p.configPath, err)
	}
	p.config = &RecurringConfig{}
	return json.Unmarshal(data, p.config)
}

// OnReport appends the recurring summary to any report that uses plugins.
func (p *RecurringPlugin) OnReport(transactions []*AST.Transaction) string {
	if p.config == nil || len(p.config.Schedules) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Recurring Payments\n")
	b.WriteString(strings.Repeat("─", 56) + "\n")
	now := time.Now()
	for _, s := range p.config.Schedules {
		if !s.Active {
			continue
		}
		next := nextDate(&s, now)
		b.WriteString(fmt.Sprintf("  %-28s  %s/%-9s  Next: %s\n",
			s.Description,
			formatAmt(s.Amount, s.Currency),
			s.Frequency,
			next.Format("2006-01-02"),
		))
	}
	return b.String()
}

// RunCommand handles: doublebook plugin run recurring [status|list|generate]
func (p *RecurringPlugin) RunCommand(transactions []*AST.Transaction, args []string) error {
	fs := flag.NewFlagSet("recurring", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	subcmd := "status"
	if fs.NArg() > 0 {
		subcmd = fs.Arg(0)
	}

	switch subcmd {
	case "status":
		return p.cmdStatus(transactions)
	case "list":
		return p.cmdList()
	case "generate":
		return p.cmdGenerate(transactions)
	default:
		return fmt.Errorf("usage: doublebook plugin run recurring [status|list|generate]")
	}
}

func (p *RecurringPlugin) cmdList() error {
	if len(p.config.Schedules) == 0 {
		fmt.Printf("No schedules found in %s\n", p.configPath)
		fmt.Println("\nCreate a recurring.json file to get started. Example:")
		fmt.Println(`  {
    "schedules": [
      {
        "id": "rent",
        "description": "Monthly Rent",
        "amount": 1200.00,
        "currency": "USD",
        "debit_account": "expenses:housing:rent",
        "credit_account": "assets:checking",
        "frequency": "monthly",
        "day_of_month": 1,
        "start_date": "2025-01-01",
        "active": true
      }
    ]
  }`)
		return nil
	}

	fmt.Printf("Recurring schedules (%d total):\n\n", len(p.config.Schedules))
	for _, s := range p.config.Schedules {
		status := "✓ active"
		if !s.Active {
			status = "✗ inactive"
		}
		fmt.Printf("  [%s] %-6s  %-26s  %s/%s\n",
			status, s.ID, s.Description,
			formatAmt(s.Amount, s.Currency), s.Frequency)
	}
	return nil
}

func (p *RecurringPlugin) cmdStatus(transactions []*AST.Transaction) error {
	if len(p.config.Schedules) == 0 {
		fmt.Println("No recurring schedules configured.")
		fmt.Printf("Config file: %s\n", p.configPath)
		return nil
	}

	now := time.Now()
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)

	fmt.Println("Recurring Payment Status")
	fmt.Println(strings.Repeat("─", 64))
	fmt.Println()

	for _, s := range p.config.Schedules {
		next := nextDate(&s, now)
		dates := datesSince(&s, yearStart, now)

		// Count matching payments.
		matched := 0
		total := 0.0
		for _, d := range dates {
			dStr := d.Format("2006-01-02")
			for _, txn := range transactions {
				tStr := txn.Date.Format("2006-01-02")
				if tStr == dStr &&
					strings.Contains(strings.ToLower(txn.Description), strings.ToLower(s.Description)) {
					matched++
					total += s.Amount
					break
				}
			}
		}

		projected := float64(schedulesInYear(&s, now.Year())) * s.Amount
		status := "✓"
		if !s.Active {
			status = "–"
		}

		fmt.Printf("%s  %s\n", status, s.Description)
		fmt.Printf("   %s/%s  Next: %s\n",
			formatAmt(s.Amount, s.Currency), s.Frequency, next.Format("2006-01-02"))
		fmt.Printf("   Paid this year: %s (%d payments)  Projected: %s\n\n",
			formatAmt(total, s.Currency), matched,
			formatAmt(projected, s.Currency))
	}
	return nil
}

func (p *RecurringPlugin) cmdGenerate(transactions []*AST.Transaction) error {
	now := time.Now()
	generated := 0

	for _, s := range p.config.Schedules {
		if !s.Active {
			continue
		}
		start, err := time.Parse("2006-01-02", s.StartDate)
		if err != nil {
			continue
		}
		dates := datesSince(&s, start, now)
		for _, d := range dates {
			dStr := d.Format("2006-01-02")
			found := false
			for _, txn := range transactions {
				if txn.Date.Format("2006-01-02") == dStr &&
					strings.Contains(strings.ToLower(txn.Description), strings.ToLower(s.Description)) {
					found = true
					break
				}
			}
			if !found {
				txn := buildScheduleTxn(&s, d)
				fmt.Print(txn.String())
				fmt.Println()
				generated++
			}
		}
	}

	if generated == 0 {
		fmt.Println("All recurring payments are accounted for.")
	} else {
		fmt.Printf("Generated %d pending transaction(s).\n", generated)
		fmt.Println("Review and append to your journal if correct.")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nextDate(s *Schedule, after time.Time) time.Time {
	t := after.Add(24 * time.Hour)
	for i := 0; i < 400; i++ {
		if matchesSchedule(s, t) {
			return t
		}
		t = t.Add(24 * time.Hour)
	}
	return after
}

func datesSince(s *Schedule, from, to time.Time) []time.Time {
	var out []time.Time
	t := from
	for !t.After(to) {
		if matchesSchedule(s, t) {
			out = append(out, t)
		}
		t = t.Add(24 * time.Hour)
	}
	return out
}

func matchesSchedule(s *Schedule, t time.Time) bool {
	switch s.Frequency {
	case "daily":
		return true
	case "weekly":
		return int(t.Weekday()) == s.DayOfWeek
	case "monthly":
		dom := s.DayOfMonth
		if dom <= 0 {
			dom = 1
		}
		// Handle months shorter than day_of_month.
		last := lastDayOfMonth(t.Year(), t.Month())
		if dom > last {
			dom = last
		}
		return t.Day() == dom
	case "yearly":
		m := s.Month
		if m <= 0 {
			m = 1
		}
		return int(t.Month()) == m && t.Day() == s.DayOfMonth
	case "custom":
		start, err := time.Parse("2006-01-02", s.StartDate)
		if err != nil || s.IntervalDays <= 0 {
			return false
		}
		diff := t.Sub(start).Hours() / 24
		return diff >= 0 && math.Mod(diff, float64(s.IntervalDays)) < 1
	}
	return false
}

func lastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func schedulesInYear(s *Schedule, year int) int {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year, 12, 31, 0, 0, 0, 0, time.Local)
	return len(datesSince(s, start, end))
}

func buildScheduleTxn(s *Schedule, date time.Time) *AST.Transaction {
	txn := AST.NewTransaction(date, s.Description)
	txn.Tags["recurring_id"] = s.ID
	for k, v := range s.Tags {
		txn.Tags[k] = v
	}
	txn.Postings = append(txn.Postings,
		AST.NewPosting(s.DebitAccount, AST.Amount{Value: s.Amount, Currency: s.Currency}),
		AST.NewPosting(s.CreditAccount, AST.Amount{Value: -s.Amount, Currency: s.Currency}),
	)
	return txn
}

func formatAmt(v float64, currency string) string {
	switch strings.ToUpper(currency) {
	case "USD":
		return fmt.Sprintf("$%.2f", v)
	case "EUR":
		return fmt.Sprintf("€%.2f", v)
	case "GBP":
		return fmt.Sprintf("£%.2f", v)
	default:
		return fmt.Sprintf("%.2f %s", v, currency)
	}
}

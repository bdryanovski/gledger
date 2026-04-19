package commands

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"doublebook/infra/config"
	"doublebook/engine/interpreter"

	"github.com/charmbracelet/lipgloss"
)

// Column widths for the register output.
const (
	regColDate    = 10
	regColDesc    = 25
	regColAccount = 30
	regColAmount  = 12
	regColRunning = 12
)

// Register styles — lipgloss strips colours automatically when stdout is
// not a TTY or when NO_COLOR is set.
var (
	regStyleDate    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	regStyleDesc    = lipgloss.NewStyle()
	regStyleAccount = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	regStyleAmount  = lipgloss.NewStyle()
	regStyleRunning = lipgloss.NewStyle().Bold(true)
	regStylePos     = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))
	regStyleNeg     = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

// RegisterCommand prints a chronological transaction register with a running
// balance, respecting the global context flags and any command-level flags.
func RegisterCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	beginFlag := fs.String("begin", "", "Only include transactions on or after DATE")
	endFlag := fs.String("end", "", "Only include transactions on or before DATE")
	accountFlag := fs.String("account", "", "Only show postings whose account matches this pattern")
	descFlag := fs.String("desc", "", "Only show transactions whose description contains TEXT")
	limitFlag := fs.Int("limit", 0, "Show only the last N transactions (0 = all)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Local date flags override global context dates.
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

	interp := interpreter.NewInterpreter(ctx.Config)
	if err := interp.LoadJournal(ctx.EffectiveJournalName()); err != nil {
		return fmt.Errorf("loading journal: %w", err)
	}

	filter := interpreter.Filter{
		BeginDate:   beginDate,
		EndDate:     endDate,
		Description: *descFlag,
	}
	// Note: we do NOT put accountFlag in the interpreter filter because we want
	// the filter to match whole transactions (not just individual postings).
	// We apply the account filter at the posting level below.
	txns := interp.FilteredTransactions(filter)

	if len(txns) == 0 {
		fmt.Fprintln(os.Stderr, "No transactions found.")
		return nil
	}

	// Apply --limit: take the last N transactions.
	if *limitFlag > 0 && *limitFlag < len(txns) {
		txns = txns[len(txns)-*limitFlag:]
	}

	// Disable colour output when NO_COLOR is set.
	noColor := os.Getenv("NO_COLOR") != ""

	// Running total across all displayed postings.
	running := 0.0
	currency := ctx.Config.Currency

	printed := false
	for _, txn := range txns {
		firstPosting := true

		for _, p := range txn.Postings {
			// Account filter: skip postings that don't match.
			if *accountFlag != "" && !strings.Contains(p.Account, *accountFlag) {
				continue
			}

			running += p.Amount.Value

			dateStr := ""
			descStr := ""
			if firstPosting {
				dateStr = txn.Date.Format("2006-01-02")
				descStr = truncateRune(txn.Description, regColDesc)
				firstPosting = false
			}

			acctStr := truncateRune(p.Account, regColAccount)
			amtStr := formatRegAmount(p.Amount.Value, p.Amount.Currency)
			runStr := formatRegAmount(running, currency)

			if noColor {
				fmt.Printf("%-*s  %-*s  %-*s  %*s  %*s\n",
					regColDate, dateStr,
					regColDesc, descStr,
					regColAccount, acctStr,
					regColAmount, amtStr,
					regColRunning, runStr,
				)
			} else {
				dateFmt := regStyleDate.Render(fmt.Sprintf("%-*s", regColDate, dateStr))
				descFmt := regStyleDesc.Render(fmt.Sprintf("%-*s", regColDesc, descStr))
				acctFmt := regStyleAccount.Render(fmt.Sprintf("%-*s", regColAccount, acctStr))
				amtFmt := colorRegAmount(amtStr, p.Amount.Value, regColAmount)
				runFmt := regStyleRunning.Render(fmt.Sprintf("%*s", regColRunning, runStr))
				fmt.Printf("%s  %s  %s  %s  %s\n",
					dateFmt, descFmt, acctFmt, amtFmt, runFmt)
			}
			printed = true
		}
	}

	if !printed {
		fmt.Fprintln(os.Stderr, "No transactions matched the given filters.")
	}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// truncateRune truncates s to at most maxLen runes, replacing the last
// character with '…' if truncation occurs.
func truncateRune(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}

// formatRegAmount formats a float64 as a display string for the register.
// It delegates to the same logic as balance_command.go's formatAmount.
func formatRegAmount(value float64, currency string) string {
	return formatAmount(value, currency)
}

// colorRegAmount wraps an amount string in the appropriate colour style and
// right-pads it to width characters.
func colorRegAmount(amtStr string, value float64, width int) string {
	padded := fmt.Sprintf("%*s", width, amtStr)
	if value < 0 {
		return regStyleNeg.Render(padded)
	}
	return regStylePos.Render(padded)
}

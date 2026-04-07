package commands

import (
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	AST "doublebook/ast"
	"doublebook/config"
	Interpreter "doublebook/interpreter"
)

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

// ISCommand implements `doublebook is` (income statement).
func ISCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("is", flag.ContinueOnError)
	beginFlag := fs.String("begin", "", "Start date (inclusive)")
	endFlag := fs.String("end", "", "End date (inclusive)")
	daily := fs.Bool("daily", false, "Break down by day")
	weekly := fs.Bool("weekly", false, "Break down by week")
	monthly := fs.Bool("monthly", false, "Break down by month")
	yearly := fs.Bool("yearly", false, "Break down by year")
	pretty := fs.Bool("pretty", false, "Render as a box-drawing table")
	rowTotal := fs.Bool("row-total", false, "Add a Total column")
	average := fs.Bool("average", false, "Add an Average column")
	noTotal := fs.Bool("no-total", false, "Omit the Net income row")
	_ = fs.Bool("tree", false, "Show account hierarchy (future)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve dates — command flags override global context.
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

	// Auto-detect period if none specified and pretty mode is on.
	period := detectPeriod(*daily, *weekly, *monthly, *yearly, beginDate, endDate)

	if *pretty {
		return printPrettyIS(interp, beginDate, endDate, period, *rowTotal, *average, *noTotal, ctx.Config.Currency)
	}
	return printSimpleIS(interp, beginDate, endDate, *noTotal, ctx.Config.Currency)
}

// detectPeriod selects the period granularity string: "daily", "weekly",
// "monthly", or "yearly".  When none of the explicit flags are set it is
// auto-detected from the date range span.
func detectPeriod(daily, weekly, monthly, yearly bool, begin, end string) string {
	switch {
	case daily:
		return "daily"
	case weekly:
		return "weekly"
	case monthly:
		return "monthly"
	case yearly:
		return "yearly"
	}
	// Auto-detect based on span.
	if begin == "" || end == "" {
		return "monthly"
	}
	b, err1 := time.Parse("2006-01-02", begin)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return "monthly"
	}
	days := int(e.Sub(b).Hours() / 24)
	if days > 62 {
		return "monthly"
	}
	return "daily"
}

// ---------------------------------------------------------------------------
// Simple (plain-text) mode
// ---------------------------------------------------------------------------

func printSimpleIS(
	interp *Interpreter.Interpreter,
	beginDate, endDate string,
	noTotal bool,
	currency string,
) error {
	filter := Interpreter.Filter{BeginDate: beginDate, EndDate: endDate}
	stmt := interp.GenerateIncomeStatement(filter)

	// Title.
	title := "Income Statement"
	if beginDate != "" || endDate != "" {
		b := beginDate
		if b == "" {
			b = "..."
		}
		e := endDate
		if e == "" {
			e = "..."
		}
		title += " " + b + ".." + e
	}
	fmt.Println(title)
	fmt.Println()

	// Revenues.
	fmt.Println("Revenues")
	revAccts := sortedAmountKeys(stmt.Revenues)
	revTotal := 0.0
	for _, acc := range revAccts {
		amt := stmt.Revenues[acc]
		fmt.Printf("  %-36s  %s\n", acc, formatAmount(amt.Value, amt.Currency))
		revTotal += amt.Value
	}
	fmt.Printf("%-38s  %s\n", "Total revenues", formatAmount(revTotal, currency))
	fmt.Println()

	// Expenses.
	fmt.Println("Expenses")
	expAccts := sortedAmountKeys(stmt.Expenses)
	expTotal := 0.0
	for _, acc := range expAccts {
		amt := stmt.Expenses[acc]
		fmt.Printf("  %-36s  %s\n", acc, formatAmount(amt.Value, amt.Currency))
		expTotal += amt.Value
	}
	fmt.Printf("%-38s  %s\n", "Total expenses", formatAmount(expTotal, currency))
	fmt.Println()

	// Net income.
	if !noTotal {
		fmt.Printf("%-38s  %s\n", "Net income", stmt.NetIncome.String())
	}

	return nil
}

// sortedAmountKeys returns the sorted keys of a map[string]AST.Amount.
func sortedAmountKeys(m map[string]AST.Amount) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ---------------------------------------------------------------------------
// Pretty (box-drawing) mode
// ---------------------------------------------------------------------------

// periodLabel converts a date to the bucket label for the given period.
func periodLabel(date time.Time, period string) string {
	switch period {
	case "monthly":
		return date.Format("2006-01")
	case "yearly":
		return date.Format("2006")
	case "weekly":
		// Use Monday of the week.
		weekday := int(date.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := date.AddDate(0, 0, -(weekday - 1))
		return monday.Format("2006-01-02")
	default: // daily
		return date.Format("2006-01-02")
	}
}

// enumeratePeriods generates all period label strings from begin to end
// (inclusive) for the given granularity.  Both dates must be non-empty.
func enumeratePeriods(begin, end, period string) []string {
	b, err1 := time.Parse("2006-01-02", begin)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return nil
	}

	seen := make(map[string]bool)
	var labels []string
	cur := b
	for !cur.After(e) {
		lbl := periodLabel(cur, period)
		if !seen[lbl] {
			seen[lbl] = true
			labels = append(labels, lbl)
		}
		switch period {
		case "monthly":
			cur = cur.AddDate(0, 1, 0)
			// Jump to 1st of month to avoid day-overflow.
			cur = time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.UTC)
		case "yearly":
			cur = cur.AddDate(1, 0, 0)
			cur = time.Date(cur.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		case "weekly":
			cur = cur.AddDate(0, 0, 7)
		default: // daily
			cur = cur.AddDate(0, 0, 1)
		}
	}
	return labels
}

// periodDateRange returns the begin/end filter dates for a single period bucket.
func periodDateRange(label, period string) (string, string) {
	switch period {
	case "monthly":
		t, err := time.Parse("2006-01", label)
		if err != nil {
			return label, label
		}
		firstDay := t.Format("2006-01-02")
		lastDay := time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		return firstDay, lastDay
	case "yearly":
		return label + "-01-01", label + "-12-31"
	case "weekly":
		t, err := time.Parse("2006-01-02", label)
		if err != nil {
			return label, label
		}
		return label, t.AddDate(0, 0, 6).Format("2006-01-02")
	default: // daily
		return label, label
	}
}

func printPrettyIS(
	interp *Interpreter.Interpreter,
	beginDate, endDate, period string,
	showTotal, showAverage, noTotal bool,
	currency string,
) error {
	// If no date range given, fill from the journal's actual extent.
	if beginDate == "" || endDate == "" {
		txns := interp.FilteredTransactions(Interpreter.Filter{})
		if len(txns) > 0 {
			if beginDate == "" {
				beginDate = txns[0].Date.Format("2006-01-02")
			}
			if endDate == "" {
				endDate = txns[len(txns)-1].Date.Format("2006-01-02") // was: beginDate (typo)
			}
		}
		// Last resort: use the current month.
		if beginDate == "" {
			beginDate = time.Now().Format("2006-01") + "-01"
		}
		if endDate == "" {
			endDate = time.Now().Format("2006-01-02")
		}
	}

	periods := enumeratePeriods(beginDate, endDate, period)
	if len(periods) == 0 {
		// Fallback: treat the whole range as a single "all" period.
		// Use begin..end as a display label and override periodDateRange below.
		lbl := beginDate
		if endDate != "" && endDate != beginDate {
			lbl = beginDate + "…" + endDate
		}
		periods = []string{lbl}
	}

	// Compute income statement per period.
	type periodIS struct {
		revenues map[string]float64
		expenses map[string]float64
	}
	periodData := make(map[string]*periodIS, len(periods))
	for _, p := range periods {
		pb, pe := periodDateRange(p, period)
		// If periodDateRange couldn't parse the label (fallback case), use the
		// global date range so we still get data.
		if pb == p && pe == p {
			pb = beginDate
			pe = endDate
		}
		f := Interpreter.Filter{BeginDate: pb, EndDate: pe}
		stmt := interp.GenerateIncomeStatement(f)
		pd := &periodIS{
			revenues: make(map[string]float64),
			expenses: make(map[string]float64),
		}
		for acc, amt := range stmt.Revenues {
			pd.revenues[acc] += amt.Value
		}
		for acc, amt := range stmt.Expenses {
			pd.expenses[acc] += amt.Value
		}
		periodData[p] = pd
	}

	// Collect all account names.
	revAcctSet := make(map[string]bool)
	expAcctSet := make(map[string]bool)
	for _, pd := range periodData {
		for acc := range pd.revenues {
			revAcctSet[acc] = true
		}
		for acc := range pd.expenses {
			expAcctSet[acc] = true
		}
	}
	revAccts := sortedStringSet(revAcctSet)
	expAccts := sortedStringSet(expAcctSet)

	// Column width heuristics.
	labelW := 20
	for _, acc := range revAccts {
		if len(acc)+2 > labelW {
			labelW = len(acc) + 2
		}
	}
	for _, acc := range expAccts {
		if len(acc)+2 > labelW {
			labelW = len(acc) + 2
		}
	}

	colW := 12
	for _, p := range periods {
		if len(p)+2 > colW {
			colW = len(p) + 2
		}
	}

	// Build column headers.
	headers := make([]string, 0, len(periods)+2)
	for _, p := range periods {
		headers = append(headers, p)
	}
	if showTotal {
		headers = append(headers, "Total")
	}
	if showAverage {
		headers = append(headers, "Average")
	}

	// Helper: compute per-period values + optional total/average.
	periodVals := func(acc string, src func(*periodIS) map[string]float64) []string {
		vals := make([]string, 0, len(headers))
		total := 0.0
		for _, p := range periods {
			v := src(periodData[p])[acc]
			total += v
			if v == 0 {
				vals = append(vals, "0")
			} else {
				vals = append(vals, formatAmount(v, currency))
			}
		}
		if showTotal {
			vals = append(vals, formatAmount(total, currency))
		}
		if showAverage && len(periods) > 0 {
			vals = append(vals, formatAmount(total/float64(len(periods)), currency))
		}
		return vals
	}

	sectionTotals := func(src func(*periodIS) map[string]float64) []string {
		vals := make([]string, 0, len(headers))
		total := 0.0
		for _, p := range periods {
			sum := 0.0
			for _, v := range src(periodData[p]) {
				sum += v
			}
			total += sum
			if sum == 0 {
				vals = append(vals, "0")
			} else {
				vals = append(vals, formatAmount(sum, currency))
			}
		}
		if showTotal {
			vals = append(vals, formatAmount(total, currency))
		}
		if showAverage && len(periods) > 0 {
			vals = append(vals, formatAmount(total/float64(len(periods)), currency))
		}
		return vals
	}

	netVals := func() []string {
		vals := make([]string, 0, len(headers))
		total := 0.0
		for _, p := range periods {
			rev := 0.0
			for _, v := range periodData[p].revenues {
				rev += v
			}
			exp := 0.0
			for _, v := range periodData[p].expenses {
				exp += v
			}
			net := rev - exp
			total += net
			if net == 0 {
				vals = append(vals, "0")
			} else {
				vals = append(vals, formatAmount(net, currency))
			}
		}
		if showTotal {
			vals = append(vals, formatAmount(total, currency))
		}
		if showAverage && len(periods) > 0 {
			vals = append(vals, formatAmount(total/float64(len(periods)), currency))
		}
		return vals
	}

	// Render title.
	periodName := map[string]string{
		"daily": "Daily", "weekly": "Weekly",
		"monthly": "Monthly", "yearly": "Yearly",
	}[period]
	fmt.Printf("%s Income Statement %s..%s\n\n", periodName, beginDate, endDate)

	// Build and render the box-drawing table.
	bt := &boxRenderer{labelW: labelW, colW: colW, numCols: len(headers)}

	fmt.Print(bt.topBorder())

	// Header row.
	fmt.Print(bt.row(center("", labelW), headersCenter(headers, colW)))
	fmt.Print(bt.doubleRow())

	// Revenues section.
	fmt.Print(bt.row(center("Revenues", labelW), emptyVals(len(headers), colW)))
	fmt.Print(bt.singleRow())
	for _, acc := range revAccts {
		vals := periodVals(acc, func(pd *periodIS) map[string]float64 { return pd.revenues })
		fmt.Print(bt.row(padRight(acc, labelW), valsCenter(vals, colW)))
	}
	fmt.Print(bt.singleRow())
	fmt.Print(bt.row(center("", labelW), valsCenter(sectionTotals(func(pd *periodIS) map[string]float64 { return pd.revenues }), colW)))
	fmt.Print(bt.doubleRow())

	// Expenses section.
	fmt.Print(bt.row(center("Expenses", labelW), emptyVals(len(headers), colW)))
	fmt.Print(bt.singleRow())
	for _, acc := range expAccts {
		vals := periodVals(acc, func(pd *periodIS) map[string]float64 { return pd.expenses })
		fmt.Print(bt.row(padRight(acc, labelW), valsCenter(vals, colW)))
	}
	fmt.Print(bt.singleRow())
	fmt.Print(bt.row(center("", labelW), valsCenter(sectionTotals(func(pd *periodIS) map[string]float64 { return pd.expenses }), colW)))

	if !noTotal {
		fmt.Print(bt.doubleRow())
		fmt.Print(bt.row(padRight("Net:", labelW), valsCenter(netVals(), colW)))
	}
	fmt.Print(bt.bottomBorder())

	return nil
}

// ---------------------------------------------------------------------------
// Box-drawing renderer
// ---------------------------------------------------------------------------

type boxRenderer struct {
	labelW  int // width of the label (first) column content
	colW    int // width of each data column content
	numCols int // number of data columns
}

func (b *boxRenderer) topBorder() string {
	s := "┌" + strings.Repeat("─", b.labelW+2) + "╥"
	for i := 0; i < b.numCols; i++ {
		s += strings.Repeat("─", b.colW+2)
		if i < b.numCols-1 {
			s += "┬"
		}
	}
	return s + "┐\n"
}

func (b *boxRenderer) bottomBorder() string {
	s := "└" + strings.Repeat("─", b.labelW+2) + "╨"
	for i := 0; i < b.numCols; i++ {
		s += strings.Repeat("─", b.colW+2)
		if i < b.numCols-1 {
			s += "┴"
		}
	}
	return s + "┘\n"
}

func (b *boxRenderer) singleRow() string {
	s := "├" + strings.Repeat("─", b.labelW+2) + "╫"
	for i := 0; i < b.numCols; i++ {
		s += strings.Repeat("─", b.colW+2)
		if i < b.numCols-1 {
			s += "┼"
		}
	}
	return s + "┤\n"
}

func (b *boxRenderer) doubleRow() string {
	s := "╞" + strings.Repeat("═", b.labelW+2) + "╬"
	for i := 0; i < b.numCols; i++ {
		s += strings.Repeat("═", b.colW+2)
		if i < b.numCols-1 {
			s += "╪"
		}
	}
	return s + "╡\n"
}

// row renders a content row: │ label ║ val │ val │ val │
func (b *boxRenderer) row(label string, vals []string) string {
	s := "│ " + label + " ║"
	for i, v := range vals {
		s += " " + v + " "
		if i < len(vals)-1 {
			s += "│"
		}
	}
	return s + "│\n"
}

// ---------------------------------------------------------------------------
// String helpers
// ---------------------------------------------------------------------------

func center(s string, width int) string {
	if len([]rune(s)) >= width {
		return string([]rune(s)[:width])
	}
	total := width - len([]rune(s))
	left := total / 2
	right := total - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func padRight(s string, width int) string {
	r := []rune(s)
	if len(r) >= width {
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len(r))
}

func headersCenter(headers []string, colW int) []string {
	out := make([]string, len(headers))
	for i, h := range headers {
		out[i] = center(h, colW)
	}
	return out
}

func valsCenter(vals []string, colW int) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = center(v, colW)
	}
	return out
}

func emptyVals(n, colW int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = strings.Repeat(" ", colW)
	}
	return out
}

func sortedStringSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

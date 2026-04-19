package fql

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Chart types and result
// ---------------------------------------------------------------------------

// ChartType selects the visual representation for query results.
type ChartType int

const (
	ChartNone ChartType = iota // fall back to a table
	ChartBar                   // horizontal bar chart (category → value)
	ChartLine                  // sparkline / time-series chart
)

// ChartResult carries data ready to render as a chart.
type ChartResult struct {
	Type   ChartType
	Labels []string  // x-axis labels (account names, dates, months, …)
	Values []float64 // one value per label
	Title  string
}

// ---------------------------------------------------------------------------
// Chart styles
// ---------------------------------------------------------------------------

var (
	chartStylePos   = lipgloss.NewStyle().Foreground(lipgloss.Color("71"))  // green
	chartStyleNeg   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // red
	chartStyleLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	chartStyleValue = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	chartStyleHead  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("75"))
)

// ---------------------------------------------------------------------------
// Chart type auto-detection
// ---------------------------------------------------------------------------

// DetectChartType inspects the column names of a result set and heuristically
// suggests the best chart type.
//
//   - Exactly 1 "date" or "month" or "year" column + 1 numeric column → ChartLine
//   - Exactly 1 text column + 1 numeric column                         → ChartBar
//   - Otherwise                                                        → ChartNone
func DetectChartType(columns []string, rows [][]interface{}) ChartType {
	if len(columns) != 2 || len(rows) == 0 {
		return ChartNone
	}

	col0Lower := strings.ToLower(columns[0])
	isDateLabel := col0Lower == "date" || col0Lower == "month" || col0Lower == "year"

	// Check if column 1 is numeric by inspecting the first row.
	col1Numeric := isNumeric(rows[0][1])
	if !col1Numeric {
		return ChartNone
	}

	if isDateLabel {
		return ChartLine
	}

	// Column 0 should be a non-numeric label.
	if !isNumeric(rows[0][0]) {
		return ChartBar
	}
	return ChartNone
}

func isNumeric(v interface{}) bool {
	switch v.(type) {
	case float64, int64, int, float32:
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Horizontal bar chart
// ---------------------------------------------------------------------------

// RenderBarChart renders a horizontal bar chart with █ block characters.
//
// Output example:
//
//	expenses:groceries  ████████████████████  $864.50
//	expenses:housing    █████████████         $600.00
//	income:salary       ████                  $200.00
func RenderBarChart(result *ChartResult, width int) string {
	if len(result.Labels) == 0 {
		return "(no data)"
	}

	// Determine max absolute value for scaling.
	maxAbs := 0.0
	for _, v := range result.Values {
		if a := math.Abs(v); a > maxAbs {
			maxAbs = a
		}
	}

	// Determine max label width.
	maxLabelW := 0
	for _, l := range result.Labels {
		if len([]rune(l)) > maxLabelW {
			maxLabelW = len([]rune(l))
		}
	}

	// Determine max value string width.
	maxValW := 0
	valStrs := make([]string, len(result.Values))
	for i, v := range result.Values {
		valStrs[i] = formatChartAmount(v)
		if len(valStrs[i]) > maxValW {
			maxValW = len(valStrs[i])
		}
	}

	// Available bar width.
	// Layout: "  " + label + "  " + bar + "  " + value
	overhead := 2 + maxLabelW + 2 + 2 + maxValW
	maxBarW := width - overhead
	if maxBarW < 5 {
		maxBarW = 5
	}

	var b strings.Builder

	if result.Title != "" {
		b.WriteString(chartStyleHead.Render(result.Title))
		b.WriteByte('\n')
		b.WriteByte('\n')
	}

	for i, label := range result.Labels {
		v := result.Values[i]

		// Scale bar length.
		barLen := 0
		if maxAbs > 0 {
			barLen = int(math.Round(math.Abs(v) / maxAbs * float64(maxBarW)))
		}
		if barLen < 1 && v != 0 {
			barLen = 1
		}

		bar := strings.Repeat("█", barLen)
		paddedLabel := fmt.Sprintf("%-*s", maxLabelW, label)
		paddedValue := fmt.Sprintf("%*s", maxValW, valStrs[i])

		// Color the bar and value by sign.
		if v < 0 {
			b.WriteString(chartStyleLabel.Render("  "+paddedLabel+"  ") +
				chartStyleNeg.Render(bar) +
				strings.Repeat(" ", maxBarW-barLen) +
				"  " + chartStyleNeg.Render(paddedValue))
		} else {
			b.WriteString(chartStyleLabel.Render("  "+paddedLabel+"  ") +
				chartStylePos.Render(bar) +
				strings.Repeat(" ", maxBarW-barLen) +
				"  " + chartStylePos.Render(paddedValue))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Sparkline / line chart
// ---------------------------------------------------------------------------

// sparkBlocks maps a 0-7 scale to Unicode block elements.
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderLineChart renders time-series data as a labelled sparkline.
// Each row shows: label + sparkline-bar + value.
//
// Output example:
//
//	2025-01  ████████████████  $1,398.56
//	2025-02  ████████          $340.34
func RenderLineChart(result *ChartResult, width, _ int) string {
	// Delegate to the bar chart renderer — it produces an identical layout
	// and is most readable in a terminal.  A horizontal bar IS a sparkline
	// for time-series data when read top-to-bottom in time order.
	return RenderBarChart(result, width)
}

// RenderSparkline renders all values as a compact single-row sparkline
// followed by min/max annotations.
//
//	▁▂▃▄▅▆▇█▇▆▅▄▃▂▁  min:-$340.34  max:$1,398.56
func RenderSparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	minV, maxV := values[0], values[0]
	for _, v := range values {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	var b strings.Builder
	rng := maxV - minV
	for _, v := range values {
		idx := 0
		if rng > 0 {
			idx = int(math.Round((v - minV) / rng * 7))
		}
		if idx < 0 {
			idx = 0
		}
		if idx > 7 {
			idx = 7
		}
		b.WriteRune(sparkBlocks[idx])
	}

	b.WriteString("  min:")
	b.WriteString(formatChartAmount(minV))
	b.WriteString("  max:")
	b.WriteString(formatChartAmount(maxV))
	return b.String()
}

// ---------------------------------------------------------------------------
// Table renderer
// ---------------------------------------------------------------------------

const (
	tableMaxRows      = 50 // rows shown before truncation notice
	tableTruncatedMsg = "... use LIMIT to paginate ..."
)

// RenderTable renders query results as a box-drawing table.
// Numeric columns are right-aligned; text columns are left-aligned.
// Long values are truncated with '…'. Total width is capped at width.
func RenderTable(columns []string, rows [][]interface{}, width int) string {
	if len(columns) == 0 {
		return "(no columns)"
	}

	truncated := 0
	if len(rows) > tableMaxRows {
		truncated = len(rows) - tableMaxRows
		rows = rows[:tableMaxRows]
	}

	// Detect numeric columns from the first row.
	numCols := len(columns)
	isNum := make([]bool, numCols)
	if len(rows) > 0 {
		for j := 0; j < numCols; j++ {
			if j < len(rows[0]) {
				isNum[j] = isNumeric(rows[0][j])
			}
		}
	}

	// Calculate column widths: max of header and data.
	colW := make([]int, numCols)
	for j, h := range columns {
		colW[j] = len(h)
	}
	for _, row := range rows {
		for j := 0; j < numCols; j++ {
			if j < len(row) {
				s := cellStr(row[j])
				if len([]rune(s)) > colW[j] {
					colW[j] = len([]rune(s))
				}
			}
		}
	}

	// Cap total width.
	total := 1 // leading │
	for _, w := range colW {
		total += w + 3 // " content " + │
	}
	if total > width && width > 10 {
		excess := total - width
		// Shrink widest columns first.
		for excess > 0 {
			maxW, maxIdx := 0, 0
			for j, w := range colW {
				if w > maxW {
					maxW, maxIdx = w, j
				}
			}
			if maxW <= 3 {
				break
			}
			colW[maxIdx]--
			excess--
		}
	}

	var b strings.Builder

	// ── Top border ────────────────────────────────────────────────────────
	b.WriteString("┌")
	for j, w := range colW {
		b.WriteString(strings.Repeat("─", w+2))
		if j < numCols-1 {
			b.WriteString("┬")
		}
	}
	b.WriteString("┐\n")

	// ── Header ───────────────────────────────────────────────────────────
	b.WriteString("│")
	for j, h := range columns {
		cell := truncateTo(h, colW[j])
		padded := fmt.Sprintf(" %-*s ", colW[j], cell)
		b.WriteString(chartStyleHead.Render(padded))
		b.WriteString("│")
	}
	b.WriteByte('\n')

	// ── Header separator ─────────────────────────────────────────────────
	b.WriteString("├")
	for j, w := range colW {
		b.WriteString(strings.Repeat("─", w+2))
		if j < numCols-1 {
			b.WriteString("┼")
		}
	}
	b.WriteString("┤\n")

	// ── Rows ─────────────────────────────────────────────────────────────
	for _, row := range rows {
		b.WriteString("│")
		for j := 0; j < numCols; j++ {
			var raw string
			if j < len(row) {
				raw = cellStr(row[j])
			}
			cell := truncateTo(raw, colW[j])
			var padded string
			if isNum[j] {
				padded = fmt.Sprintf(" %*s ", colW[j], cell)
			} else {
				padded = fmt.Sprintf(" %-*s ", colW[j], cell)
			}
			b.WriteString(chartStyleValue.Render(padded))
			b.WriteString("│")
		}
		b.WriteByte('\n')
	}

	// ── Bottom border ─────────────────────────────────────────────────────
	b.WriteString("└")
	for j, w := range colW {
		b.WriteString(strings.Repeat("─", w+2))
		if j < numCols-1 {
			b.WriteString("┴")
		}
	}
	b.WriteString("┘\n")

	// ── Truncation notice ─────────────────────────────────────────────────
	if truncated > 0 {
		b.WriteString(chartStyleLabel.Render(
			fmt.Sprintf("  Showing %d of %d rows. %s\n",
				tableMaxRows, tableMaxRows+truncated, tableTruncatedMsg)))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// cellStr converts a database value to a display string.
func cellStr(v interface{}) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case float64:
		if val == math.Trunc(val) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%.2f", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// truncateTo truncates s to maxLen runes, appending '…' if needed.
func truncateTo(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(r[:maxLen-1]) + "…"
}

// formatChartAmount formats a float64 as a compact amount string.
func formatChartAmount(v float64) string {
	if v == 0 {
		return "0"
	}
	neg := ""
	abs := v
	if v < 0 {
		neg = "-"
		abs = -abs
	}
	if abs >= 1_000_000 {
		return fmt.Sprintf("%s$%.2fM", neg, abs/1_000_000)
	}
	if abs >= 1_000 {
		return fmt.Sprintf("%s$%.2fK", neg, abs/1_000)
	}
	return fmt.Sprintf("%s$%.2f", neg, abs)
}

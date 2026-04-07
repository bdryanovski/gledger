package fql

import (
	"database/sql"
	"fmt"
	"strings"

	"doublebook/db"
)

// ---------------------------------------------------------------------------
// Execute — top-level query runner
// ---------------------------------------------------------------------------

// Execute compiles fqlQuery, runs it against database, and returns formatted
// output: a bar/line chart when the result shape supports it, or a table.
//
// width is the terminal width used for chart and table sizing.
func Execute(fqlQuery string, database *db.DB, width int) (string, error) {
	if width <= 0 {
		width = 80
	}

	// Compile FQL → SQL.
	compiled, err := Compile(fqlQuery)
	if err != nil {
		return "", err
	}

	// Execute against SQLite.
	columns, rows, err := queryDB(database.Conn(), compiled.SQL, compiled.Params)
	if err != nil {
		return "", fmt.Errorf("query execution error: %w", err)
	}

	if len(rows) == 0 {
		return chartStyleLabel.Render("  (no results)") + "\n", nil
	}

	// Auto-detect chart type.
	ct := DetectChartType(columns, rows)

	switch ct {
	case ChartBar, ChartLine:
		// Extract label (col 0) and value (col 1).
		result := &ChartResult{Type: ct}
		for _, row := range rows {
			if len(row) < 2 {
				continue
			}
			label := fmt.Sprintf("%v", row[0])
			val := toFloat64(row[1])
			result.Labels = append(result.Labels, label)
			result.Values = append(result.Values, val)
		}
		if ct == ChartLine {
			return RenderLineChart(result, width, 12), nil
		}
		return RenderBarChart(result, width), nil

	default:
		// Table fallback.
		return RenderTable(columns, rows, width), nil
	}
}

// ---------------------------------------------------------------------------
// Database query helpers
// ---------------------------------------------------------------------------

// queryDB executes a parameterized SQL query and returns columns + rows.
func queryDB(conn *sql.DB, sqlStr string, params []interface{}) ([]string, [][]interface{}, error) {
	rows, err := conn.Query(sqlStr, params...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var result [][]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		// Convert []byte values to strings for display.
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				vals[i] = string(b)
			}
		}
		result = append(result, vals)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return columns, result, nil
}

// toFloat64 attempts to convert an interface{} database value to float64.
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

// ---------------------------------------------------------------------------
// Column/row utilities (used by REPL)
// ---------------------------------------------------------------------------

// FormatResultSet returns a formatted string for any query result, always
// as a table (used when the caller wants to force table output).
func FormatResultSet(columns []string, rows [][]interface{}, width int) string {
	return RenderTable(columns, rows, width)
}

// RunQuery compiles and executes an FQL query, returning raw columns and rows.
// Useful when the caller wants to process results programmatically.
func RunQuery(fqlQuery string, database *db.DB) ([]string, [][]interface{}, error) {
	compiled, err := Compile(fqlQuery)
	if err != nil {
		return nil, nil, err
	}
	return queryDB(database.Conn(), compiled.SQL, compiled.Params)
}

// AvailableTables returns a formatted string listing all virtual tables and
// their columns — used in the REPL welcome message.
func AvailableTables() string {
	var b strings.Builder
	order := []string{"transactions", "accounts", "spending"}
	for _, name := range order {
		tbl, ok := Tables[name]
		if !ok {
			continue
		}
		b.WriteString(chartStyleHead.Render(name))
		b.WriteString(": ")
		cols := make([]string, len(tbl.Columns))
		for i, c := range tbl.Columns {
			cols[i] = c.Name
		}
		b.WriteString(chartStyleLabel.Render(strings.Join(cols, ", ")))
		b.WriteByte('\n')
	}
	return b.String()
}

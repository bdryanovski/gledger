package fql

import (
	"fmt"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// Compiled result
// ---------------------------------------------------------------------------

// Compiled holds the output of FQL → SQL compilation.
type Compiled struct {
	SQL    string        // parameterized SQLite SQL
	Params []interface{} // positional ? params in order
}

// ---------------------------------------------------------------------------
// Compiler
// ---------------------------------------------------------------------------

// Compiler translates a *Query AST into parameterized SQLite SQL.
type Compiler struct {
	params []interface{}
}

// Compile is the top-level entry point: parse an FQL string and compile it
// to parameterized SQL in one step.
func Compile(fqlQuery string) (*Compiled, error) {
	p, err := NewParser(fqlQuery)
	if err != nil {
		return nil, fmt.Errorf("FQL parse error: %w", err)
	}
	q, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("FQL parse error: %w", err)
	}
	c := &Compiler{}
	return c.Compile(q)
}

// Compile validates and compiles a *Query AST.
func (c *Compiler) Compile(q *Query) (*Compiled, error) {
	// ── 1. Validate table ────────────────────────────────────────────────
	tbl, ok := Tables[q.From]
	if !ok {
		avail := TableNames()
		sort.Strings(avail)
		return nil, fmt.Errorf("unknown virtual table %q. Available: %s",
			q.From, strings.Join(avail, ", "))
	}

	// ── 2. Validate SELECT columns (direct identifier references only) ───
	for _, col := range q.Columns {
		if col.Star {
			continue
		}
		if ident, ok := col.Expr.(*Identifier); ok {
			if !tbl.HasColumn(ident.Name) {
				return nil, fmt.Errorf("unknown column %q in SELECT. Available columns for %q: %s",
					ident.Name, q.From, strings.Join(tbl.ColumnNames(), ", "))
			}
		}
	}

	// ── 3. Build SQL ─────────────────────────────────────────────────────
	var b strings.Builder

	// SELECT [DISTINCT] <columns>
	b.WriteString("SELECT ")
	if q.Distinct {
		b.WriteString("DISTINCT ")
	}
	b.WriteString(c.compileColumns(q.Columns))

	// FROM (<virtual table SQL>) AS _vt
	b.WriteString("\nFROM (\n")
	b.WriteString(strings.TrimSpace(tbl.SQL))
	b.WriteString("\n) AS _vt")

	// WHERE
	if q.Where != nil {
		b.WriteString("\nWHERE ")
		b.WriteString(c.compileExpr(q.Where))
	}

	// GROUP BY
	if len(q.GroupBy) > 0 {
		b.WriteString("\nGROUP BY ")
		b.WriteString(strings.Join(q.GroupBy, ", "))
	}

	// HAVING
	if q.Having != nil {
		b.WriteString("\nHAVING ")
		b.WriteString(c.compileExpr(q.Having))
	}

	// ORDER BY
	if len(q.OrderBy) > 0 {
		b.WriteString("\nORDER BY ")
		specs := make([]string, len(q.OrderBy))
		for i, s := range q.OrderBy {
			if s.Desc {
				specs[i] = s.Column + " DESC"
			} else {
				specs[i] = s.Column + " ASC"
			}
		}
		b.WriteString(strings.Join(specs, ", "))
	}

	// LIMIT / OFFSET
	if q.Limit != nil {
		b.WriteString(fmt.Sprintf("\nLIMIT %d", *q.Limit))
	}
	if q.Offset != nil {
		b.WriteString(fmt.Sprintf("\nOFFSET %d", *q.Offset))
	}

	return &Compiled{SQL: b.String(), Params: c.params}, nil
}

// ---------------------------------------------------------------------------
// Column list compilation
// ---------------------------------------------------------------------------

func (c *Compiler) compileColumns(cols []SelectColumn) string {
	if len(cols) == 1 && cols[0].Star {
		return "*"
	}
	parts := make([]string, len(cols))
	for i, col := range cols {
		parts[i] = c.compileSelectColumn(col)
	}
	return strings.Join(parts, ", ")
}

func (c *Compiler) compileSelectColumn(col SelectColumn) string {
	if col.Star {
		return "*"
	}
	expr := c.compileExpr(col.Expr)
	if col.Alias != "" {
		return expr + " AS " + col.Alias
	}
	return expr
}

// ---------------------------------------------------------------------------
// Expression compilation
// ---------------------------------------------------------------------------

func (c *Compiler) compileExpr(expr Expression) string {
	switch e := expr.(type) {
	case *BinaryExpr:
		return "(" + c.compileExpr(e.Left) + " " + e.Op + " " + c.compileExpr(e.Right) + ")"

	case *UnaryExpr:
		return e.Op + " (" + c.compileExpr(e.Expr) + ")"

	case *Identifier:
		return e.Name

	case *NumberLiteral:
		c.params = append(c.params, e.Value)
		return "?"

	case *StringLiteral:
		c.params = append(c.params, e.Value)
		return "?"

	case *StarLiteral:
		return "*"

	case *FunctionCall:
		if e.Star {
			return strings.ToUpper(e.Name) + "(*)"
		}
		args := make([]string, len(e.Args))
		for i, a := range e.Args {
			args[i] = c.compileExpr(a)
		}
		return strings.ToUpper(e.Name) + "(" + strings.Join(args, ", ") + ")"

	case *InExpr:
		placeholders := make([]string, len(e.Values))
		for i, v := range e.Values {
			placeholders[i] = c.compileExpr(v)
		}
		not := ""
		if e.Not {
			not = "NOT "
		}
		return c.compileExpr(e.Left) + " " + not + "IN (" + strings.Join(placeholders, ", ") + ")"

	case *BetweenExpr:
		return c.compileExpr(e.Expr) + " BETWEEN " +
			c.compileExpr(e.Low) + " AND " + c.compileExpr(e.High)

	case *IsNullExpr:
		if e.Not {
			return c.compileExpr(e.Expr) + " IS NOT NULL"
		}
		return c.compileExpr(e.Expr) + " IS NULL"

	default:
		return "/* unknown expr */"
	}
}

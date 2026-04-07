package fql

import (
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustCompile(t *testing.T, fql string) *Compiled {
	t.Helper()
	c, err := Compile(fql)
	if err != nil {
		t.Fatalf("Compile(%q) unexpected error: %v", fql, err)
	}
	return c
}

func assertSQL(t *testing.T, c *Compiled, substr string) {
	t.Helper()
	if !strings.Contains(c.SQL, substr) {
		t.Errorf("expected SQL to contain %q\ngot:\n%s", substr, c.SQL)
	}
}

func assertParamCount(t *testing.T, c *Compiled, n int) {
	t.Helper()
	if len(c.Params) != n {
		t.Errorf("expected %d params, got %d: %v", n, len(c.Params), c.Params)
	}
}

func assertParamValue(t *testing.T, c *Compiled, idx int, want interface{}) {
	t.Helper()
	if idx >= len(c.Params) {
		t.Errorf("param[%d] out of range (len=%d)", idx, len(c.Params))
		return
	}
	got := c.Params[idx]
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Errorf("param[%d]: got %v (%T), want %v (%T)", idx, got, got, want, want)
	}
}

// ---------------------------------------------------------------------------
// Basic SELECT
// ---------------------------------------------------------------------------

func TestBasicSelect(t *testing.T) {
	c := mustCompile(t, "SELECT id, date, description, amount FROM transactions LIMIT 10")
	assertSQL(t, c, "LIMIT 10")
	assertSQL(t, c, "SELECT id, date, description, amount")
	assertParamCount(t, c, 0)
}

func TestSelectStar(t *testing.T) {
	c := mustCompile(t, "SELECT * FROM transactions")
	assertSQL(t, c, "SELECT *")
}

func TestSelectDistinct(t *testing.T) {
	c := mustCompile(t, "SELECT DISTINCT account FROM transactions")
	assertSQL(t, c, "SELECT DISTINCT account")
}

// ---------------------------------------------------------------------------
// Bug fixes — negative numbers
// ---------------------------------------------------------------------------

func TestNegativeNumber(t *testing.T) {
	// Was: "Unexpected character '-'"
	c := mustCompile(t, "SELECT id, date, amount FROM transactions WHERE amount < -100")
	assertParamCount(t, c, 1)
	if c.Params[0] != float64(-100) {
		t.Errorf("param[0]: got %v, want -100.0", c.Params[0])
	}
}

func TestNegativeFloat(t *testing.T) {
	c := mustCompile(t, "SELECT date, amount FROM transactions WHERE amount < -0.01")
	assertParamCount(t, c, 1)
	if c.Params[0] != float64(-0.01) {
		t.Errorf("param[0]: got %v, want -0.01", c.Params[0])
	}
}

func TestHavingNegative(t *testing.T) {
	// Was: "Unexpected character '-'" in HAVING
	c := mustCompile(t,
		"SELECT account, SUM(amount) AS total FROM transactions GROUP BY account HAVING total < -50")
	assertSQL(t, c, "HAVING")
	assertSQL(t, c, "GROUP BY account")
}

func TestCompoundOrWithNegative(t *testing.T) {
	// Was: "Unexpected character '-'" in compound WHERE
	c := mustCompile(t,
		"SELECT id, amount FROM transactions WHERE (account = 'expenses:food' OR account = 'expenses:transport') AND amount < -20")
	assertParamCount(t, c, 3) // 'expenses:food', 'expenses:transport', -20.0
	assertSQL(t, c, "WHERE")
}

// ---------------------------------------------------------------------------
// Bug fix — COUNT(*) without alias
// ---------------------------------------------------------------------------

func TestCountStarNoAlias(t *testing.T) {
	// Was: required alias after COUNT(*)
	c := mustCompile(t, "SELECT account, COUNT(*) FROM transactions GROUP BY account")
	assertSQL(t, c, "COUNT(*)")
	assertSQL(t, c, "GROUP BY account")
}

func TestCountStarWithAlias(t *testing.T) {
	c := mustCompile(t, "SELECT account, COUNT(*) AS cnt FROM transactions GROUP BY account")
	assertSQL(t, c, "COUNT(*) AS cnt")
}

// ---------------------------------------------------------------------------
// WHERE clauses
// ---------------------------------------------------------------------------

func TestWhereEquals(t *testing.T) {
	c := mustCompile(t, "SELECT id FROM transactions WHERE account = 'expenses:food'")
	assertParamCount(t, c, 1)
	if c.Params[0] != "expenses:food" {
		t.Errorf("param[0]: got %v, want 'expenses:food'", c.Params[0])
	}
}

func TestWhereNotEquals(t *testing.T) {
	c := mustCompile(t, "SELECT id FROM transactions WHERE account != 'income:salary'")
	assertSQL(t, c, "!=")
	assertParamCount(t, c, 1)
}

func TestWhereLike(t *testing.T) {
	c := mustCompile(t, "SELECT id, description FROM transactions WHERE description LIKE '%bill%'")
	assertSQL(t, c, "LIKE")
	assertParamCount(t, c, 1)
	if c.Params[0] != "%bill%" {
		t.Errorf("LIKE param: got %v", c.Params[0])
	}
}

func TestWhereIsNull(t *testing.T) {
	c := mustCompile(t, "SELECT id, description FROM transactions WHERE description IS NOT NULL")
	assertSQL(t, c, "IS NOT NULL")
	assertParamCount(t, c, 0)
}

func TestWhereIsNullAffirmative(t *testing.T) {
	c := mustCompile(t, "SELECT id FROM transactions WHERE status IS NULL")
	assertSQL(t, c, "IS NULL")
}

// ---------------------------------------------------------------------------
// IN / NOT IN
// ---------------------------------------------------------------------------

func TestInList(t *testing.T) {
	c := mustCompile(t, "SELECT id FROM transactions WHERE account IN ('expenses:food', 'expenses:transport')")
	assertParamCount(t, c, 2)
	assertSQL(t, c, "IN (?, ?)")
}

func TestNotInList(t *testing.T) {
	c := mustCompile(t, "SELECT id FROM transactions WHERE account NOT IN ('income:salary')")
	assertSQL(t, c, "NOT IN")
	assertParamCount(t, c, 1)
}

// ---------------------------------------------------------------------------
// BETWEEN
// ---------------------------------------------------------------------------

func TestDateRange(t *testing.T) {
	c := mustCompile(t, "SELECT date, amount FROM transactions WHERE date BETWEEN '2024-01-01' AND '2024-12-31'")
	assertSQL(t, c, "BETWEEN ? AND ?")
	assertParamCount(t, c, 2)
	if c.Params[0] != "2024-01-01" {
		t.Errorf("BETWEEN low: got %v", c.Params[0])
	}
	if c.Params[1] != "2024-12-31" {
		t.Errorf("BETWEEN high: got %v", c.Params[1])
	}
}

// ---------------------------------------------------------------------------
// Aggregates
// ---------------------------------------------------------------------------

func TestSumAggregate(t *testing.T) {
	c := mustCompile(t, "SELECT account, SUM(amount) AS total FROM transactions GROUP BY account ORDER BY total ASC")
	assertSQL(t, c, "SUM(amount) AS total")
	assertSQL(t, c, "ORDER BY total ASC")
}

func TestAvgMinMax(t *testing.T) {
	c := mustCompile(t, "SELECT account, AVG(amount) AS avg_a, MIN(amount) AS min_a, MAX(amount) AS max_a FROM transactions GROUP BY account")
	assertSQL(t, c, "AVG(amount)")
	assertSQL(t, c, "MIN(amount)")
	assertSQL(t, c, "MAX(amount)")
}

// ---------------------------------------------------------------------------
// ORDER BY / LIMIT / OFFSET
// ---------------------------------------------------------------------------

func TestOrderByDesc(t *testing.T) {
	c := mustCompile(t, "SELECT date, amount FROM transactions ORDER BY date DESC LIMIT 5 OFFSET 10")
	assertSQL(t, c, "ORDER BY date DESC")
	assertSQL(t, c, "LIMIT 5")
	assertSQL(t, c, "OFFSET 10")
}

// ---------------------------------------------------------------------------
// All virtual tables
// ---------------------------------------------------------------------------

func TestAccountsTable(t *testing.T) {
	c := mustCompile(t, "SELECT name, total_amount, transaction_count FROM accounts ORDER BY total_amount DESC")
	assertSQL(t, c, "ORDER BY total_amount DESC")
}

func TestSpendingTable(t *testing.T) {
	c := mustCompile(t, "SELECT month, account, SUM(total_amount) AS monthly FROM spending WHERE total_amount < 0 GROUP BY month, account")
	assertSQL(t, c, "GROUP BY month, account")
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestUnknownTable(t *testing.T) {
	_, err := Compile("SELECT id FROM real_transactions")
	if err == nil {
		t.Fatal("expected error for unknown table")
	}
	if !strings.Contains(err.Error(), "unknown virtual table") {
		t.Errorf("error should mention unknown table: %v", err)
	}
}

func TestUnknownColumn(t *testing.T) {
	_, err := Compile("SELECT secret_column FROM transactions")
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
	if !strings.Contains(err.Error(), "unknown column") {
		t.Errorf("error should mention unknown column: %v", err)
	}
}

func TestMissingFrom(t *testing.T) {
	_, err := Compile("SELECT id WHERE amount > 0")
	if err == nil {
		t.Fatal("expected error for missing FROM")
	}
}

func TestSyntaxError(t *testing.T) {
	_, err := Compile("SELECT FROM transactions")
	if err == nil {
		t.Fatal("expected error for invalid syntax")
	}
}

func TestEmptyQuery(t *testing.T) {
	_, err := Compile("")
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

// ---------------------------------------------------------------------------
// Parameterization (no SQL injection)
// ---------------------------------------------------------------------------

func TestStringParamNotInSQL(t *testing.T) {
	c := mustCompile(t, "SELECT id FROM transactions WHERE account = 'DROP TABLE transactions'")
	if strings.Contains(c.SQL, "DROP TABLE") {
		t.Error("SQL injection: literal value must not appear in compiled SQL")
	}
	assertParamCount(t, c, 1)
}

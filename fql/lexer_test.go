package fql

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustTokenize(t *testing.T, input string) []Token {
	t.Helper()
	l := NewLexer(input)
	if err := l.Tokenize(); err != nil {
		t.Fatalf("Tokenize(%q) error: %v", input, err)
	}
	// Strip trailing EOF.
	toks := l.Tokens()
	if len(toks) > 0 && toks[len(toks)-1].Type == FQL_EOF {
		toks = toks[:len(toks)-1]
	}
	return toks
}

func assertToken(t *testing.T, tok Token, wantType TokenType, wantLit string) {
	t.Helper()
	if tok.Type != wantType {
		t.Errorf("token type: got %s, want %s (literal=%q)", tok.Type, wantType, tok.Literal)
	}
	if tok.Literal != wantLit {
		t.Errorf("token literal: got %q, want %q", tok.Literal, wantLit)
	}
}

func assertNoErrors(t *testing.T, tokens []Token) {
	t.Helper()
	for _, tok := range tokens {
		if tok.Type == FQL_ERROR {
			t.Errorf("unexpected FQL_ERROR token: %q", tok.Literal)
		}
	}
}

// ---------------------------------------------------------------------------
// TestLexNegativeNumber
// ---------------------------------------------------------------------------

func TestLexNegativeNumber(t *testing.T) {
	// "amount < -100" → IDENT LT NUMBER(-100)
	toks := mustTokenize(t, "amount < -100")
	if len(toks) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(toks), toks)
	}
	assertToken(t, toks[0], FQL_IDENT, "amount")
	assertToken(t, toks[1], FQL_LT, "<")
	assertToken(t, toks[2], FQL_NUMBER, "-100")
}

func TestLexNegativeFloat(t *testing.T) {
	toks := mustTokenize(t, "total < -50.5")
	if len(toks) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(toks), toks)
	}
	assertToken(t, toks[2], FQL_NUMBER, "-50.5")
}

// ---------------------------------------------------------------------------
// TestLexNegativeInHaving
// ---------------------------------------------------------------------------

func TestLexNegativeInHaving(t *testing.T) {
	q := "SELECT category, SUM(amount) AS total FROM transactions GROUP BY category HAVING total < -50"
	l := NewLexer(q)
	if err := l.Tokenize(); err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	toks := l.Tokens()
	assertNoErrors(t, toks)

	// Find the -50 token.
	found := false
	for _, tok := range toks {
		if tok.Type == FQL_NUMBER && tok.Literal == "-50" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected NUMBER(-50) in HAVING clause, not found in %v", toks)
	}
}

// ---------------------------------------------------------------------------
// TestLexCountStar
// ---------------------------------------------------------------------------

func TestLexCountStar(t *testing.T) {
	toks := mustTokenize(t, "COUNT(*)")
	if len(toks) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(toks), toks)
	}
	assertToken(t, toks[0], FQL_COUNT, "COUNT")
	assertToken(t, toks[1], FQL_LPAREN, "(")
	assertToken(t, toks[2], FQL_STAR, "*")
	assertToken(t, toks[3], FQL_RPAREN, ")")
}

// ---------------------------------------------------------------------------
// TestLexStringLiteral
// ---------------------------------------------------------------------------

func TestLexStringLiteral(t *testing.T) {
	toks := mustTokenize(t, "'food'")
	if len(toks) != 1 {
		t.Fatalf("expected 1 token, got %d", len(toks))
	}
	assertToken(t, toks[0], FQL_STRING, "food")
}

func TestLexStringLiteralEscapedQuote(t *testing.T) {
	// '' inside a string is an escaped single quote.
	toks := mustTokenize(t, "'it''s fine'")
	if len(toks) != 1 {
		t.Fatalf("expected 1 token, got %d: %v", len(toks), toks)
	}
	if toks[0].Literal != "it's fine" {
		t.Errorf("escaped quote: got %q, want %q", toks[0].Literal, "it's fine")
	}
}

// ---------------------------------------------------------------------------
// TestLexKeywordCaseInsensitive
// ---------------------------------------------------------------------------

func TestLexKeywordCaseInsensitive(t *testing.T) {
	cases := []string{"SELECT", "select", "Select", "SeLeCt"}
	for _, c := range cases {
		toks := mustTokenize(t, c)
		if len(toks) != 1 || toks[0].Type != FQL_SELECT {
			t.Errorf("input %q: expected FQL_SELECT, got %v", c, toks)
		}
	}
}

func TestLexAllKeywords(t *testing.T) {
	pairs := []struct {
		input string
		want  TokenType
	}{
		{"select", FQL_SELECT}, {"distinct", FQL_DISTINCT},
		{"from", FQL_FROM}, {"where", FQL_WHERE},
		{"and", FQL_AND}, {"or", FQL_OR}, {"not", FQL_NOT},
		{"in", FQL_IN}, {"like", FQL_LIKE}, {"between", FQL_BETWEEN},
		{"is", FQL_IS}, {"null", FQL_NULL},
		{"group", FQL_GROUP}, {"by", FQL_BY},
		{"having", FQL_HAVING}, {"order", FQL_ORDER},
		{"asc", FQL_ASC}, {"desc", FQL_DESC},
		{"limit", FQL_LIMIT}, {"offset", FQL_OFFSET}, {"as", FQL_AS},
		{"count", FQL_COUNT}, {"sum", FQL_SUM}, {"avg", FQL_AVG},
		{"min", FQL_MIN}, {"max", FQL_MAX},
	}
	for _, p := range pairs {
		toks := mustTokenize(t, p.input)
		if len(toks) != 1 || toks[0].Type != p.want {
			t.Errorf("keyword %q: got %v, want %s", p.input, toks, p.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestLexIdentifiers
// ---------------------------------------------------------------------------

func TestLexIdentWithUnderscore(t *testing.T) {
	toks := mustTokenize(t, "account_name")
	if len(toks) != 1 {
		t.Fatalf("expected 1 token, got %d", len(toks))
	}
	assertToken(t, toks[0], FQL_IDENT, "account_name")
}

func TestLexIdentWithDigits(t *testing.T) {
	toks := mustTokenize(t, "col2")
	if len(toks) != 1 {
		t.Fatalf("expected 1 token, got %d", len(toks))
	}
	assertToken(t, toks[0], FQL_IDENT, "col2")
}

// ---------------------------------------------------------------------------
// TestLexOperators
// ---------------------------------------------------------------------------

func TestLexOperators(t *testing.T) {
	cases := []struct {
		input string
		want  TokenType
		lit   string
	}{
		{"=", FQL_EQ, "="},
		{"!=", FQL_NEQ, "!="},
		{"<>", FQL_NEQ, "<>"},
		{"<", FQL_LT, "<"},
		{"<=", FQL_LTE, "<="},
		{">", FQL_GT, ">"},
		{">=", FQL_GTE, ">="},
	}
	for _, c := range cases {
		toks := mustTokenize(t, c.input)
		if len(toks) != 1 {
			t.Errorf("input %q: expected 1 token, got %d", c.input, len(toks))
			continue
		}
		assertToken(t, toks[0], c.want, c.lit)
	}
}

// ---------------------------------------------------------------------------
// TestLexComplexQuery
// ---------------------------------------------------------------------------

func TestLexComplexQuery(t *testing.T) {
	q := "SELECT id, date, amount FROM transactions WHERE amount < -100 ORDER BY date DESC LIMIT 10"
	l := NewLexer(q)
	if err := l.Tokenize(); err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	assertNoErrors(t, l.Tokens())

	// Spot-check the negative number.
	toks := l.Tokens()
	found := false
	for _, tok := range toks {
		if tok.Type == FQL_NUMBER && tok.Literal == "-100" {
			found = true
		}
	}
	if !found {
		t.Error("expected NUMBER(-100) in query")
	}
}

func TestLexInList(t *testing.T) {
	toks := mustTokenize(t, "account IN ('food', 'transport')")
	assertNoErrors(t, toks)
	// Should contain: IDENT IN LPAREN STRING COMMA STRING RPAREN
	if len(toks) != 7 {
		t.Errorf("expected 7 tokens, got %d: %v", len(toks), toks)
	}
}

func TestLexCompoundWhereNegative(t *testing.T) {
	// Reproduces the bug: (category = 'food') AND amount < -20
	q := "SELECT id FROM transactions WHERE (category = 'food') AND amount < -20"
	l := NewLexer(q)
	if err := l.Tokenize(); err != nil {
		t.Fatalf("Tokenize error: %v", err)
	}
	assertNoErrors(t, l.Tokens())
	found := false
	for _, tok := range l.Tokens() {
		if tok.Type == FQL_NUMBER && tok.Literal == "-20" {
			found = true
		}
	}
	if !found {
		t.Error("expected NUMBER(-20) in compound WHERE clause")
	}
}

// ---------------------------------------------------------------------------
// TestLexNavigation (Next / Peek / PeekAt)
// ---------------------------------------------------------------------------

func TestLexNavigation(t *testing.T) {
	l := NewLexer("SELECT id FROM t")
	if err := l.Tokenize(); err != nil {
		t.Fatal(err)
	}

	// Peek should not advance.
	first := l.Peek()
	second := l.Peek()
	if first.Type != second.Type || first.Literal != second.Literal {
		t.Error("Peek should not advance the index")
	}

	// Next should advance.
	tok := l.Next()
	assertToken(t, tok, FQL_SELECT, "SELECT")
	tok = l.Next()
	assertToken(t, tok, FQL_IDENT, "id")

	// PeekAt(0) is current, PeekAt(1) is one ahead.
	cur := l.PeekAt(0)
	next := l.PeekAt(1)
	if cur.Type == next.Type && cur.Literal == next.Literal {
		// Only fails if they happen to be identical, which they shouldn't here.
		// FROM ≠ t
	}
	assertToken(t, cur, FQL_FROM, "FROM")
}

// ---------------------------------------------------------------------------
// TestLexUnterminatedString — should return an error
// ---------------------------------------------------------------------------

func TestLexUnterminatedString(t *testing.T) {
	l := NewLexer("'unterminated")
	err := l.Tokenize()
	if err == nil {
		t.Error("expected error for unterminated string, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestLexNegativeAtStart
// ---------------------------------------------------------------------------

func TestLexNegativeAtStart(t *testing.T) {
	// -5 at the start of input is a unary minus (e.g. standalone expression).
	toks := mustTokenize(t, "-5")
	if len(toks) != 1 {
		t.Fatalf("expected 1 token, got %d: %v", len(toks), toks)
	}
	assertToken(t, toks[0], FQL_NUMBER, "-5")
}

func TestLexNegativeAfterComma(t *testing.T) {
	toks := mustTokenize(t, "f(a, -3)")
	assertNoErrors(t, toks)
	// Should have: IDENT LPAREN IDENT COMMA NUMBER RPAREN
	found := false
	for _, tok := range toks {
		if tok.Type == FQL_NUMBER && tok.Literal == "-3" {
			found = true
		}
	}
	if !found {
		t.Error("expected NUMBER(-3) after comma")
	}
}

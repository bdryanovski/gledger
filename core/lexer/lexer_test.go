package lexer

import (
	"testing"

	"doublebook/core/ast"
)

// ---------------------------------------------------------------------------
// Multi-currency amounts
// ---------------------------------------------------------------------------

func TestMultiCurrencyAmounts(t *testing.T) {
	cases := []struct {
		input    string
		wantType ast.TokenType
		wantVal  string
	}{
		{"$45.32", ast.TOKEN_AMOUNT, "$45.32"},
		{"-$45.32", ast.TOKEN_AMOUNT, "-$45.32"},
		{"£10.50", ast.TOKEN_AMOUNT, "£10.50"},
		{"-£10.50", ast.TOKEN_AMOUNT, "-£10.50"},
		{"€25.00", ast.TOKEN_AMOUNT, "€25.00"},
		{"-€25.00", ast.TOKEN_AMOUNT, "-€25.00"},
		{"100 BGN", ast.TOKEN_AMOUNT, "100 BGN"},
		{"-100 BGN", ast.TOKEN_AMOUNT, "-100 BGN"},
		{"45.32", ast.TOKEN_AMOUNT, "45.32"},
		{"-45.32", ast.TOKEN_AMOUNT, "-45.32"},
		{"1,234.56", ast.TOKEN_AMOUNT, "1,234.56"},
	}

	for _, tc := range cases {
		l := NewLexer(tc.input)
		tok := l.NextToken()
		if tok.Type != tc.wantType {
			t.Errorf("input %q: got token type %v, want %v", tc.input, tok.Type, tc.wantType)
		}
		if tok.Value != tc.wantVal {
			t.Errorf("input %q: got value %q, want %q", tc.input, tok.Value, tc.wantVal)
		}
	}
}

// ---------------------------------------------------------------------------
// Date tokenization
// ---------------------------------------------------------------------------

func TestDateToken(t *testing.T) {
	l := NewLexer("2025-01-15 ")
	tok := l.NextToken()
	if tok.Type != ast.TOKEN_DATE {
		t.Errorf("expected TOKEN_DATE, got %v", tok.Type)
	}
	if tok.Value != "2025-01-15" {
		t.Errorf("expected '2025-01-15', got %q", tok.Value)
	}
}

// A bare number like "45.32" must NOT be misread as a date.
func TestBareNumberNotDate(t *testing.T) {
	l := NewLexer("45.32")
	tok := l.NextToken()
	if tok.Type != ast.TOKEN_AMOUNT {
		t.Errorf("expected TOKEN_AMOUNT, got %v (value=%q)", tok.Type, tok.Value)
	}
}

// ---------------------------------------------------------------------------
// Status markers
// ---------------------------------------------------------------------------

func TestStatusTokenCleared(t *testing.T) {
	input := "2025-01-15 * Cleared transaction\n"
	l := NewLexer(input)
	tokens := collectTokens(l)

	// Expected: DATE, STATUS("*"), STRING("Cleared"), STRING("transaction"), NEWLINE
	if len(tokens) < 4 {
		t.Fatalf("expected at least 4 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Type != ast.TOKEN_DATE {
		t.Errorf("tokens[0]: expected TOKEN_DATE, got %v", tokens[0].Type)
	}
	if tokens[1].Type != ast.TOKEN_STATUS || tokens[1].Value != "*" {
		t.Errorf("tokens[1]: expected TOKEN_STATUS '*', got %v %q", tokens[1].Type, tokens[1].Value)
	}
}

func TestStatusTokenPending(t *testing.T) {
	input := "2025-01-15 ! Pending\n"
	l := NewLexer(input)
	tokens := collectTokens(l)
	if len(tokens) < 3 {
		t.Fatalf("too few tokens: %d", len(tokens))
	}
	if tokens[1].Type != ast.TOKEN_STATUS || tokens[1].Value != "!" {
		t.Errorf("tokens[1]: expected TOKEN_STATUS '!', got %v %q", tokens[1].Type, tokens[1].Value)
	}
}

// ---------------------------------------------------------------------------
// Comments
// ---------------------------------------------------------------------------

func TestCommentAtTopLevel(t *testing.T) {
	input := "; this is a comment\n"
	l := NewLexer(input)
	tok := l.NextToken()
	if tok.Type != ast.TOKEN_COMMENT {
		t.Errorf("expected TOKEN_COMMENT, got %v", tok.Type)
	}
	if tok.Value != "; this is a comment" {
		t.Errorf("comment value wrong: %q", tok.Value)
	}
}

func TestCommentBetweenTransactions(t *testing.T) {
	input := "2025-01-15 Test\n    a:b  $10\n    c:d  -$10\n\n; standalone comment\n2025-01-16 Test2\n    a:b  $5\n    c:d  -$5\n"
	l := NewLexer(input)
	for {
		tok := l.NextToken()
		if tok.Type == ast.TOKEN_EOF {
			break
		}
		if tok.Type == ast.TOKEN_ERROR {
			t.Errorf("unexpected TOKEN_ERROR: %q at line %d col %d", tok.Value, tok.Line, tok.Column)
		}
	}
}

func TestInlinePostingComment(t *testing.T) {
	// A posting line: indent + account + amount + inline comment
	input := "    expenses:food    $45.32  ; category: food\n"
	l := NewLexer(input)
	tokens := collectTokens(l)

	// Expected sequence: INDENT, ACCOUNT, AMOUNT, COMMENT, NEWLINE
	types := tokenTypes(tokens)
	want := []ast.TokenType{
		ast.TOKEN_INDENT,
		ast.TOKEN_ACCOUNT,
		ast.TOKEN_AMOUNT,
		ast.TOKEN_COMMENT,
		ast.TOKEN_NEWLINE,
	}
	if len(types) != len(want) {
		t.Fatalf("token count: got %d %v, want %d %v", len(types), types, len(want), want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Errorf("token[%d]: got %v, want %v", i, types[i], want[i])
		}
	}
	if tokens[2].Value != "$45.32" {
		t.Errorf("amount token: got %q, want %q", tokens[2].Value, "$45.32")
	}
}

// ---------------------------------------------------------------------------
// Indentation
// ---------------------------------------------------------------------------

func TestIndentToken(t *testing.T) {
	input := "    expenses:food  $10\n"
	l := NewLexer(input)
	tok := l.NextToken()
	if tok.Type != ast.TOKEN_INDENT {
		t.Errorf("expected TOKEN_INDENT, got %v", tok.Type)
	}
}

// ---------------------------------------------------------------------------
// Account names
// ---------------------------------------------------------------------------

func TestAccountWithHyphen(t *testing.T) {
	l := NewLexer("    assets:bank-of-america  $100\n")
	tokens := collectTokens(l)
	if len(tokens) < 2 {
		t.Fatal("too few tokens")
	}
	if tokens[1].Type != ast.TOKEN_ACCOUNT {
		t.Errorf("expected TOKEN_ACCOUNT, got %v", tokens[1].Type)
	}
	if tokens[1].Value != "assets:bank-of-america" {
		t.Errorf("account value: got %q", tokens[1].Value)
	}
}

// ---------------------------------------------------------------------------
// Full transaction round-trip
// ---------------------------------------------------------------------------

func TestFullTransaction(t *testing.T) {
	input := "2025-01-15 Grocery Store\n    expenses:groceries    $45.32\n    assets:checking       -$45.32\n"
	l := NewLexer(input)
	tokens := collectTokens(l)

	// No error tokens.
	for _, tok := range tokens {
		if tok.Type == ast.TOKEN_ERROR {
			t.Errorf("unexpected TOKEN_ERROR: %q", tok.Value)
		}
	}
	// First token is a date.
	if tokens[0].Type != ast.TOKEN_DATE {
		t.Errorf("first token: expected TOKEN_DATE, got %v", tokens[0].Type)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func collectTokens(l *Lexer) []ast.Token {
	var out []ast.Token
	for {
		tok := l.NextToken()
		if tok.Type == ast.TOKEN_EOF {
			break
		}
		out = append(out, tok)
	}
	return out
}

func tokenTypes(tokens []ast.Token) []ast.TokenType {
	types := make([]ast.TokenType, len(tokens))
	for i, t := range tokens {
		types[i] = t.Type
	}
	return types
}

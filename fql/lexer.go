// Package fql implements the Financial Query Language — a SQL-like language
// for querying DoubleBook journal data through virtual tables backed by SQLite.
package fql

import (
	"fmt"
	"strings"
	"unicode"
)

// ---------------------------------------------------------------------------
// Token types
// ---------------------------------------------------------------------------

// TokenType classifies a single FQL token.
type TokenType int

const (
	// Literals
	FQL_NUMBER TokenType = iota // integer or float, may be negative
	FQL_STRING                  // 'single-quoted string'
	FQL_IDENT                   // identifier: table/column/alias name
	FQL_STAR                    // * — SELECT * or COUNT(*)

	// Keywords (matched case-insensitively)
	FQL_SELECT
	FQL_DISTINCT
	FQL_FROM
	FQL_WHERE
	FQL_AND
	FQL_OR
	FQL_NOT
	FQL_IN
	FQL_LIKE
	FQL_BETWEEN
	FQL_IS
	FQL_NULL
	FQL_GROUP
	FQL_BY
	FQL_HAVING
	FQL_ORDER
	FQL_ASC
	FQL_DESC
	FQL_LIMIT
	FQL_OFFSET
	FQL_AS

	// Aggregate functions
	FQL_COUNT
	FQL_SUM
	FQL_AVG
	FQL_MIN
	FQL_MAX

	// Comparison operators
	FQL_EQ  // =
	FQL_NEQ // != or <>
	FQL_LT  // <
	FQL_LTE // <=
	FQL_GT  // >
	FQL_GTE // >=

	// Arithmetic (only minus needed — unary context handled in lexer)
	FQL_MINUS // standalone '-' between expressions (rare in FQL)

	// Punctuation
	FQL_COMMA  // ,
	FQL_LPAREN // (
	FQL_RPAREN // )
	FQL_DOT    // .

	// Sentinels
	FQL_EOF
	FQL_ERROR
)

// tokenTypeName returns a human-readable name for debugging.
func (tt TokenType) String() string {
	names := map[TokenType]string{
		FQL_NUMBER: "NUMBER", FQL_STRING: "STRING", FQL_IDENT: "IDENT",
		FQL_STAR: "STAR", FQL_SELECT: "SELECT", FQL_DISTINCT: "DISTINCT",
		FQL_FROM: "FROM", FQL_WHERE: "WHERE", FQL_AND: "AND", FQL_OR: "OR",
		FQL_NOT: "NOT", FQL_IN: "IN", FQL_LIKE: "LIKE", FQL_BETWEEN: "BETWEEN",
		FQL_IS: "IS", FQL_NULL: "NULL", FQL_GROUP: "GROUP", FQL_BY: "BY",
		FQL_HAVING: "HAVING", FQL_ORDER: "ORDER", FQL_ASC: "ASC", FQL_DESC: "DESC",
		FQL_LIMIT: "LIMIT", FQL_OFFSET: "OFFSET", FQL_AS: "AS",
		FQL_COUNT: "COUNT", FQL_SUM: "SUM", FQL_AVG: "AVG",
		FQL_MIN: "MIN", FQL_MAX: "MAX",
		FQL_EQ: "EQ", FQL_NEQ: "NEQ", FQL_LT: "LT", FQL_LTE: "LTE",
		FQL_GT: "GT", FQL_GTE: "GTE", FQL_MINUS: "MINUS",
		FQL_COMMA: "COMMA", FQL_LPAREN: "LPAREN", FQL_RPAREN: "RPAREN",
		FQL_DOT: "DOT", FQL_EOF: "EOF", FQL_ERROR: "ERROR",
	}
	if s, ok := names[tt]; ok {
		return s
	}
	return fmt.Sprintf("TokenType(%d)", int(tt))
}

// ---------------------------------------------------------------------------
// Token
// ---------------------------------------------------------------------------

// Token is a single lexical unit from an FQL query string.
type Token struct {
	Type    TokenType
	Literal string // raw text (string literals have quotes stripped)
	Pos     int    // byte offset in the original input
}

func (t Token) String() string {
	return fmt.Sprintf("%s(%q)@%d", t.Type, t.Literal, t.Pos)
}

// ---------------------------------------------------------------------------
// Keyword map
// ---------------------------------------------------------------------------

var keywords = map[string]TokenType{
	"select":   FQL_SELECT,
	"distinct": FQL_DISTINCT,
	"from":     FQL_FROM,
	"where":    FQL_WHERE,
	"and":      FQL_AND,
	"or":       FQL_OR,
	"not":      FQL_NOT,
	"in":       FQL_IN,
	"like":     FQL_LIKE,
	"between":  FQL_BETWEEN,
	"is":       FQL_IS,
	"null":     FQL_NULL,
	"group":    FQL_GROUP,
	"by":       FQL_BY,
	"having":   FQL_HAVING,
	"order":    FQL_ORDER,
	"asc":      FQL_ASC,
	"desc":     FQL_DESC,
	"limit":    FQL_LIMIT,
	"offset":   FQL_OFFSET,
	"as":       FQL_AS,
	"count":    FQL_COUNT,
	"sum":      FQL_SUM,
	"avg":      FQL_AVG,
	"min":      FQL_MIN,
	"max":      FQL_MAX,
}

// ---------------------------------------------------------------------------
// Lexer
// ---------------------------------------------------------------------------

// Lexer tokenizes an FQL query string upfront into a flat []Token slice.
// All navigation (Next, Peek, PeekAt) operates over the cached slice.
type Lexer struct {
	input  string
	pos    int     // current scan position in input
	tokens []Token // fully tokenized output
	index  int     // current read position in tokens
}

// NewLexer creates a Lexer for the given FQL input.
func NewLexer(input string) *Lexer {
	return &Lexer{input: input}
}

// Tokenize scans the full input and populates l.tokens.
// Returns an error only for unexpected characters; callers can also inspect
// FQL_ERROR tokens in the output.
func (l *Lexer) Tokenize() error {
	l.tokens = l.tokens[:0]
	l.pos = 0

	for l.pos < len(l.input) {
		// Skip whitespace.
		if unicode.IsSpace(rune(l.input[l.pos])) {
			l.pos++
			continue
		}

		ch := l.input[l.pos]
		start := l.pos

		switch {
		case ch == '\'':
			if err := l.scanString(start); err != nil {
				return err
			}

		case isDigit(ch):
			l.scanNumber(start, false)

		case ch == '-':
			if isUnaryContext(l.tokens) {
				// Negative number literal (e.g. amount < -100).
				l.scanNumber(start, true)
			} else {
				l.tokens = append(l.tokens, Token{FQL_MINUS, "-", start})
				l.pos++
			}

		case isLetter(ch):
			l.scanIdent(start)

		case ch == '*':
			l.tokens = append(l.tokens, Token{FQL_STAR, "*", start})
			l.pos++

		case ch == '=':
			l.tokens = append(l.tokens, Token{FQL_EQ, "=", start})
			l.pos++

		case ch == '!':
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
				l.tokens = append(l.tokens, Token{FQL_NEQ, "!=", start})
				l.pos += 2
			} else {
				l.tokens = append(l.tokens, Token{FQL_ERROR, string(ch), start})
				l.pos++
			}

		case ch == '<':
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
				l.tokens = append(l.tokens, Token{FQL_LTE, "<=", start})
				l.pos += 2
			} else if l.pos+1 < len(l.input) && l.input[l.pos+1] == '>' {
				l.tokens = append(l.tokens, Token{FQL_NEQ, "<>", start})
				l.pos += 2
			} else {
				l.tokens = append(l.tokens, Token{FQL_LT, "<", start})
				l.pos++
			}

		case ch == '>':
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '=' {
				l.tokens = append(l.tokens, Token{FQL_GTE, ">=", start})
				l.pos += 2
			} else {
				l.tokens = append(l.tokens, Token{FQL_GT, ">", start})
				l.pos++
			}

		case ch == ',':
			l.tokens = append(l.tokens, Token{FQL_COMMA, ",", start})
			l.pos++

		case ch == '(':
			l.tokens = append(l.tokens, Token{FQL_LPAREN, "(", start})
			l.pos++

		case ch == ')':
			l.tokens = append(l.tokens, Token{FQL_RPAREN, ")", start})
			l.pos++

		case ch == '.':
			l.tokens = append(l.tokens, Token{FQL_DOT, ".", start})
			l.pos++

		default:
			return fmt.Errorf("unexpected character %q at position %d in query: ...%s...",
				string(ch), start, context(l.input, start))
		}
	}

	l.tokens = append(l.tokens, Token{FQL_EOF, "", l.pos})
	return nil
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

// Next returns the current token and advances the index.
func (l *Lexer) Next() Token {
	if l.index >= len(l.tokens) {
		return Token{Type: FQL_EOF}
	}
	t := l.tokens[l.index]
	l.index++
	return t
}

// Peek returns the current token without advancing.
func (l *Lexer) Peek() Token {
	return l.PeekAt(0)
}

// PeekAt returns the token n positions ahead of the current index without advancing.
func (l *Lexer) PeekAt(n int) Token {
	idx := l.index + n
	if idx >= len(l.tokens) {
		return Token{Type: FQL_EOF}
	}
	return l.tokens[idx]
}

// Tokens returns all tokenized tokens (including the trailing EOF).
// Only valid after Tokenize() has been called.
func (l *Lexer) Tokens() []Token { return l.tokens }

// ---------------------------------------------------------------------------
// Scanners
// ---------------------------------------------------------------------------

// scanString scans a single-quoted string literal.
// Supports ” as an escaped single quote within the string.
func (l *Lexer) scanString(start int) error {
	l.pos++ // consume opening '
	var b strings.Builder
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == '\'' {
			// Check for escaped quote ''.
			if l.pos+1 < len(l.input) && l.input[l.pos+1] == '\'' {
				b.WriteByte('\'')
				l.pos += 2
				continue
			}
			l.pos++ // consume closing '
			l.tokens = append(l.tokens, Token{FQL_STRING, b.String(), start})
			return nil
		}
		b.WriteByte(ch)
		l.pos++
	}
	return fmt.Errorf("unterminated string literal at position %d", start)
}

// scanNumber scans an integer or decimal number.
// If withMinus is true the leading '-' at l.pos has already been confirmed.
func (l *Lexer) scanNumber(start int, withMinus bool) {
	var b strings.Builder
	if withMinus {
		b.WriteByte('-')
		l.pos++ // consume '-'
	}
	for l.pos < len(l.input) && (isDigit(l.input[l.pos]) || l.input[l.pos] == '.') {
		b.WriteByte(l.input[l.pos])
		l.pos++
	}
	l.tokens = append(l.tokens, Token{FQL_NUMBER, b.String(), start})
}

// scanIdent scans an identifier or keyword (case-insensitive keyword matching).
func (l *Lexer) scanIdent(start int) {
	var b strings.Builder
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		b.WriteByte(l.input[l.pos])
		l.pos++
	}
	word := b.String()
	lower := strings.ToLower(word)
	if tt, ok := keywords[lower]; ok {
		l.tokens = append(l.tokens, Token{tt, word, start})
	} else {
		l.tokens = append(l.tokens, Token{FQL_IDENT, word, start})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isUnaryContext returns true when a '-' at the current position should be
// treated as a unary (negative-number) prefix rather than a binary minus.
//
// A '-' is unary when:
//   - it is the very first token (empty token list), or
//   - the previous meaningful token is an operator, comma, opening paren,
//     or one of the keywords that precede an expression.
func isUnaryContext(tokens []Token) bool {
	if len(tokens) == 0 {
		return true
	}
	switch tokens[len(tokens)-1].Type {
	case FQL_EQ, FQL_NEQ, FQL_LT, FQL_LTE, FQL_GT, FQL_GTE,
		FQL_COMMA, FQL_LPAREN,
		FQL_AND, FQL_OR, FQL_NOT,
		FQL_BETWEEN, FQL_IN, FQL_HAVING, FQL_WHERE,
		FQL_AS, FQL_BY, FQL_LIMIT, FQL_OFFSET:
		return true
	}
	return false
}

func isDigit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func isLetter(ch byte) bool { return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' }

// context returns up to 20 chars around pos for error messages.
func context(input string, pos int) string {
	start := pos - 10
	if start < 0 {
		start = 0
	}
	end := pos + 10
	if end > len(input) {
		end = len(input)
	}
	return input[start:end]
}

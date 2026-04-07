// Package lexer converts a plain-text hledger-compatible journal into a stream
// of tokens consumed by the parser.  It works over a []rune slice so that
// multi-byte Unicode currency symbols (£, €) are handled correctly.
package lexer

import (
	"strings"
	"unicode"

	AST "doublebook/ast"
)

// Lexer holds the tokenisation state.
type Lexer struct {
	runes []rune // full input decoded to runes
	pos   int    // current position in runes
	line  int    // 1-based line number
	col   int    // 0-based column; 0 = first character of a new line
}

// CreateLexer creates a new Lexer for the given input string.
// Kept for backward compatibility; NewLexer is the preferred constructor.
func CreateLexer(input string) *Lexer {
	return &Lexer{runes: []rune(input), line: 1}
}

// NewLexer is an alias for CreateLexer.
func NewLexer(input string) *Lexer { return CreateLexer(input) }

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (l *Lexer) atEOF() bool { return l.pos >= len(l.runes) }

// peek returns the current rune without consuming it (0 at EOF).
func (l *Lexer) peek() rune {
	if l.atEOF() {
		return 0
	}
	return l.runes[l.pos]
}

// peekAt returns the rune n positions ahead of the current position (0 if out of range).
func (l *Lexer) peekAt(n int) rune {
	i := l.pos + n
	if i >= len(l.runes) {
		return 0
	}
	return l.runes[i]
}

// advance consumes and returns the current rune, updating line/col counters.
func (l *Lexer) advance() rune {
	if l.atEOF() {
		return 0
	}
	r := l.runes[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
	return r
}

// skipSpaces consumes space and tab characters (but not newlines).
func (l *Lexer) skipSpaces() {
	for l.peek() == ' ' || l.peek() == '\t' {
		l.advance()
	}
}

// tok is a compact Token constructor.
func (l *Lexer) tok(typ AST.TokenType, value string, line, col int) AST.Token {
	return AST.Token{Type: typ, Value: value, Line: line, Column: col}
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// NextToken returns the next token from the input stream.
// Calling NextToken after TOKEN_EOF continues to return TOKEN_EOF.
func (l *Lexer) NextToken() AST.Token {
	// ── Start-of-line: detect indentation ────────────────────────────────
	// We check this before skipping spaces so a posting's leading whitespace
	// is emitted as TOKEN_INDENT rather than silently consumed.
	if l.col == 0 && !l.atEOF() {
		r := l.peek()
		if r == ' ' || r == '\t' {
			startLine := l.line
			var indent strings.Builder
			for l.peek() == ' ' || l.peek() == '\t' {
				indent.WriteRune(l.advance())
			}
			if indent.Len() >= 2 {
				return l.tok(AST.TOKEN_INDENT, indent.String(), startLine, 0)
			}
			// Fewer than 2 spaces: not a valid posting indent; fall through.
		}
	}

	// ── Skip intra-line whitespace ────────────────────────────────────────
	if l.col > 0 {
		l.skipSpaces()
	}

	if l.atEOF() {
		return l.tok(AST.TOKEN_EOF, "", l.line, l.col)
	}

	startLine := l.line
	startCol := l.col
	r := l.peek()

	switch {
	// ── Newline ───────────────────────────────────────────────────────────
	case r == '\n':
		l.advance()
		return l.tok(AST.TOKEN_NEWLINE, "\n", startLine, startCol)

	// ── Comment ───────────────────────────────────────────────────────────
	case r == ';':
		return l.scanComment(startLine, startCol)

	// ── Status marker ─────────────────────────────────────────────────────
	// '!' = pending,  '*' = cleared.  These appear after a date token.
	case r == '*' || r == '!':
		l.advance()
		return l.tok(AST.TOKEN_STATUS, string(r), startLine, startCol)

	// ── Currency-symbol-prefixed amount: $, £, € ──────────────────────────
	case r == '$' || r == '£' || r == '€':
		return l.scanAmount(startLine, startCol)

	// ── Negative sign before currency symbol or digit ─────────────────────
	case r == '-' && isCurrencyOrDigit(l.peekAt(1)):
		return l.scanAmount(startLine, startCol)

	// ── Digit: date (YYYY-MM-DD) or bare/comma amount ────────────────────
	case isDigitRune(r):
		if l.looksLikeDate() {
			return l.scanDate(startLine, startCol)
		}
		return l.scanAmount(startLine, startCol)

	// ── Letter: account name, description word, or 3-letter currency+amount
	case isLetterRune(r):
		return l.scanWord(startLine, startCol)

	// ── Unknown character ─────────────────────────────────────────────────
	default:
		l.advance()
		return l.tok(AST.TOKEN_ERROR, string(r), startLine, startCol)
	}
}

// ---------------------------------------------------------------------------
// Scanners
// ---------------------------------------------------------------------------

// scanComment reads from ';' to the end of the line (exclusive of '\n').
func (l *Lexer) scanComment(line, col int) AST.Token {
	var b strings.Builder
	for l.peek() != '\n' && !l.atEOF() {
		b.WriteRune(l.advance())
	}
	return l.tok(AST.TOKEN_COMMENT, b.String(), line, col)
}

// looksLikeDate returns true when the next 10 runes match the pattern
// YYYY-MM-DD and the character immediately after is a non-digit boundary.
func (l *Lexer) looksLikeDate() bool {
	if l.pos+9 >= len(l.runes) {
		return false
	}
	for i := 0; i < 4; i++ {
		if !isDigitRune(l.peekAt(i)) {
			return false
		}
	}
	if l.peekAt(4) != '-' {
		return false
	}
	for i := 5; i < 7; i++ {
		if !isDigitRune(l.peekAt(i)) {
			return false
		}
	}
	if l.peekAt(7) != '-' {
		return false
	}
	for i := 8; i < 10; i++ {
		if !isDigitRune(l.peekAt(i)) {
			return false
		}
	}
	after := l.peekAt(10)
	return after == ' ' || after == '\t' || after == '\n' || after == 0
}

// scanDate reads exactly 10 runes forming a YYYY-MM-DD date.
func (l *Lexer) scanDate(line, col int) AST.Token {
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteRune(l.advance())
	}
	return l.tok(AST.TOKEN_DATE, b.String(), line, col)
}

// scanAmount handles all amount formats:
//
//	-$45.32   -£10.50   -€25.00
//	$45.32    £10.50    €25.00
//	45.32     -45.32
//	1,234.56
//	100 BGN   -100 BGN
//
// The full literal is returned as TOKEN_AMOUNT; utils.ParseAmount interprets it.
func (l *Lexer) scanAmount(line, col int) AST.Token {
	var b strings.Builder

	// Optional leading minus sign.
	if l.peek() == '-' {
		b.WriteRune(l.advance())
	}

	// Optional currency prefix symbol ($, £, €).
	if r := l.peek(); r == '$' || r == '£' || r == '€' {
		b.WriteRune(l.advance())
	}

	// Digits, decimal point, and comma thousands-separators.
	for isDigitRune(l.peek()) || l.peek() == '.' || l.peek() == ',' {
		b.WriteRune(l.advance())
	}

	// Optional trailing currency code: " BGN", " EUR", etc.
	// Require: space + exactly 3 uppercase ASCII letters + non-letter boundary.
	if (l.peek() == ' ' || l.peek() == '\t') && l.trailing3LetterCode() != "" {
		b.WriteRune(l.advance()) // consume the space
		b.WriteRune(l.advance()) // 1st letter
		b.WriteRune(l.advance()) // 2nd letter
		b.WriteRune(l.advance()) // 3rd letter
	}

	return l.tok(AST.TOKEN_AMOUNT, b.String(), line, col)
}

// trailing3LetterCode peeks past the current space to see if there are 3
// uppercase ASCII letters followed by a non-letter.  Returns the code (e.g.
// "BGN") or "" if the pattern is not present.
// Precondition: l.peek() is a space or tab.
func (l *Lexer) trailing3LetterCode() string {
	a, b, c := l.peekAt(1), l.peekAt(2), l.peekAt(3)
	if isUpperAlpha(a) && isUpperAlpha(b) && isUpperAlpha(c) {
		after := l.peekAt(4)
		if !isLetterRune(after) {
			return string([]rune{a, b, c})
		}
	}
	return ""
}

// scanWord reads a run of letters, digits, ':', '_', and hyphens-before-letters.
// If the result contains ':' it is TOKEN_ACCOUNT; otherwise TOKEN_STRING.
//
// Note: "BGN 100" (leading currency code) is NOT handled here — the code is
// emitted as a TOKEN_STRING and the "100" as a TOKEN_AMOUNT.  The standard
// hledger format uses the trailing form "100 BGN" which scanAmount handles.
func (l *Lexer) scanWord(line, col int) AST.Token {
	var b strings.Builder
	for {
		r := l.peek()
		if isLetterRune(r) || isDigitRune(r) || r == ':' || r == '_' {
			b.WriteRune(l.advance())
			continue
		}
		// Allow hyphens inside account names / description words (e.g. bank-of-america).
		if r == '-' && isLetterOrDigit(l.peekAt(1)) {
			b.WriteRune(l.advance())
			continue
		}
		break
	}
	s := b.String()
	if strings.Contains(s, ":") {
		return l.tok(AST.TOKEN_ACCOUNT, s, line, col)
	}
	return l.tok(AST.TOKEN_STRING, s, line, col)
}

// ---------------------------------------------------------------------------
// Character predicates
// ---------------------------------------------------------------------------

func isDigitRune(r rune) bool     { return r >= '0' && r <= '9' }
func isUpperAlpha(r rune) bool    { return r >= 'A' && r <= 'Z' }
func isLetterRune(r rune) bool    { return unicode.IsLetter(r) }
func isLetterOrDigit(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// isCurrencyOrDigit returns true for characters that can validly follow a '-'
// to form a negative amount.
func isCurrencyOrDigit(r rune) bool {
	return r == '$' || r == '£' || r == '€' || isDigitRune(r)
}

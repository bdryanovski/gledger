// Package Parser converts a stream of tokens from the lexer into a slice of
// *ast.Transaction values that represent the journal entries.
package parser

import (
	"fmt"
	"strings"
	"time"

	"doublebook/ast"
	"doublebook/lexer"
	"doublebook/utils"
)

// ---------------------------------------------------------------------------
// Parser type
// ---------------------------------------------------------------------------

// Parser holds the two-token look-ahead state used during parsing.
type Parser struct {
	lex     *lexer.Lexer
	current ast.Token // token being examined now
	peek    ast.Token // one token ahead (look-ahead)
}

func newParser(input string) *Parser {
	p := &Parser{lex: lexer.NewLexer(input)}
	p.advance() // fill peek
	p.advance() // fill current (peek becomes current, next peek is fetched)
	return p
}

// advance shifts the look-ahead window forward by one token.
func (p *Parser) advance() {
	p.current = p.peek
	p.peek = p.lex.NextToken()
}

// skipBlanks advances past TOKEN_NEWLINE and top-level TOKEN_COMMENT tokens.
// Called at the top of the main loop so we always land on TOKEN_DATE or TOKEN_EOF.
func (p *Parser) skipBlanks() {
	for p.current.Type == ast.TOKEN_NEWLINE ||
		p.current.Type == ast.TOKEN_COMMENT {
		p.advance()
	}
}

// isIndentedComment returns true when the current token is TOKEN_INDENT and
// the next token is TOKEN_COMMENT — i.e. the line is `    ; some text`.
func (p *Parser) isIndentedComment() bool {
	return p.current.Type == ast.TOKEN_INDENT &&
		p.peek.Type == ast.TOKEN_COMMENT
}

// ---------------------------------------------------------------------------
// Top-level entry point
// ---------------------------------------------------------------------------

// ParseTransactions parses raw hledger-compatible journal text and returns
// the ordered list of transactions.
func ParseTransactions(input string) ([]*ast.Transaction, error) {
	p := newParser(input)
	return p.parse()
}

func (p *Parser) parse() ([]*ast.Transaction, error) {
	var txns []*ast.Transaction

	p.skipBlanks()
	for p.current.Type != ast.TOKEN_EOF {
		txn, err := p.parseTransaction()
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", p.current.Line, err)
		}
		if txn != nil {
			txns = append(txns, txn)
		}
		p.skipBlanks()
	}
	return txns, nil
}

// ---------------------------------------------------------------------------
// Transaction parser
// ---------------------------------------------------------------------------

func (p *Parser) parseTransaction() (*ast.Transaction, error) {
	// ── 1. Date ──────────────────────────────────────────────────────────
	if p.current.Type != ast.TOKEN_DATE {
		return nil, fmt.Errorf("expected date, got %q", p.current.Value)
	}
	date, err := time.Parse("2006-01-02", p.current.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", p.current.Value, err)
	}
	p.advance()

	// ── 2. Optional status marker (* or !) ───────────────────────────────
	status := ""
	if p.current.Type == ast.TOKEN_STATUS {
		status = p.current.Value
		p.advance()
	}

	// ── 3. Description — everything up to newline or inline comment ───────
	//
	// We consume ALL token types (not just TOKEN_STRING) so that descriptions
	// containing slashes, numbers, parentheses, etc. are preserved verbatim.
	// Example: "BILLA 127 01", "WAY4BOOK-2025/01/02", "Payment (ref: 123)"
	var descParts []string
	for p.current.Type != ast.TOKEN_NEWLINE &&
		p.current.Type != ast.TOKEN_EOF &&
		p.current.Type != ast.TOKEN_COMMENT {
		val := strings.TrimSpace(p.current.Value)
		if val != "" {
			descParts = append(descParts, val)
		}
		p.advance()
	}
	if len(descParts) == 0 {
		return nil, fmt.Errorf("missing description after date")
	}
	description := strings.Join(descParts, " ")

	// ── 4. Optional trailing comment on the header line ──────────────────
	headerComment := ""
	if p.current.Type == ast.TOKEN_COMMENT {
		_, _, headerComment = parseTagsFromComment(p.current.Value)
		p.advance()
	}

	// ── 5. Newline terminating the header ────────────────────────────────
	if p.current.Type != ast.TOKEN_NEWLINE {
		return nil, fmt.Errorf("expected newline after description, got %q", p.current.Value)
	}
	p.advance()

	// ── 6. Build the transaction skeleton ────────────────────────────────
	txn := ast.NewTransaction(date, description)
	txn.Status = status
	txn.Comment = headerComment

	// ── 7. Transaction-level comment lines (before/between postings) ──────
	// These are either:
	//   • top-level `;` lines  (TOKEN_COMMENT)
	//   • indented `;` lines   (TOKEN_INDENT + TOKEN_COMMENT)
	// We consume them all, extracting id/tags as we go.
	for {
		if p.current.Type == ast.TOKEN_COMMENT {
			id, tags, rest := parseTagsFromComment(p.current.Value)
			applyMeta(txn, id, tags, rest)
			p.advance()
			if p.current.Type == ast.TOKEN_NEWLINE {
				p.advance()
			}
		} else if p.isIndentedComment() {
			p.advance() // skip indent
			id, tags, rest := parseTagsFromComment(p.current.Value)
			applyMeta(txn, id, tags, rest)
			p.advance() // skip comment
			if p.current.Type == ast.TOKEN_NEWLINE {
				p.advance()
			}
		} else {
			break
		}
	}

	// ── 8. Postings ───────────────────────────────────────────────────────
	for p.current.Type == ast.TOKEN_INDENT {
		// Indented comment between postings → treat as transaction metadata.
		if p.isIndentedComment() {
			p.advance() // skip indent
			id, tags, rest := parseTagsFromComment(p.current.Value)
			applyMeta(txn, id, tags, rest)
			p.advance() // skip comment
			if p.current.Type == ast.TOKEN_NEWLINE {
				p.advance()
			}
			continue
		}

		posting, err := p.parsePosting()
		if err != nil {
			return nil, err
		}
		txn.Postings = append(txn.Postings, posting)
	}

	// ── 9. Validate posting count ─────────────────────────────────────────
	if len(txn.Postings) < 2 {
		return nil, fmt.Errorf("transaction %q must have at least 2 postings (got %d)",
			description, len(txn.Postings))
	}

	// Count omitted amounts — FillImpliedAmounts allows at most 1.
	omitted := 0
	for _, posting := range txn.Postings {
		if posting.AmountOmitted {
			omitted++
		}
	}
	if omitted > 1 {
		return nil, fmt.Errorf("transaction %q has %d postings with implied amounts; at most 1 is allowed",
			description, omitted)
	}

	// ── 10. Fill implied amount ───────────────────────────────────────────
	if err := txn.FillImpliedAmounts(); err != nil {
		return nil, err
	}

	// ── 11. Balance check ─────────────────────────────────────────────────
	if !txn.IsBalanced() {
		return nil, fmt.Errorf("transaction %q on %s is unbalanced (sum: %.4f)",
			description, date.Format("2006-01-02"), txn.Balance())
	}

	return txn, nil
}

// ---------------------------------------------------------------------------
// Posting parser
// ---------------------------------------------------------------------------

func (p *Parser) parsePosting() (*ast.Posting, error) {
	if p.current.Type != ast.TOKEN_INDENT {
		return nil, fmt.Errorf("expected indent, got %q", p.current.Value)
	}
	p.advance() // consume indent

	if p.current.Type != ast.TOKEN_ACCOUNT {
		return nil, fmt.Errorf("expected account name, got %q (type %v)",
			p.current.Value, p.current.Type)
	}
	account := p.current.Value
	p.advance()

	var posting *ast.Posting

	switch p.current.Type {
	case ast.TOKEN_AMOUNT:
		// Normal posting: explicit amount.
		amount, err := utils.ParseAmount(p.current.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid amount %q: %w", p.current.Value, err)
		}
		p.advance()
		posting = ast.NewPosting(account, amount)

	case ast.TOKEN_NEWLINE, ast.TOKEN_EOF:
		// Implied amount: no amount on this line.
		posting = ast.NewOmittedPosting(account)

	case ast.TOKEN_COMMENT:
		// Implied amount: comment follows directly (no amount written).
		posting = ast.NewOmittedPosting(account)

	default:
		return nil, fmt.Errorf("expected amount or end of posting line, got %q (type %v)",
			p.current.Value, p.current.Type)
	}

	// Optional inline comment after the amount.
	if p.current.Type == ast.TOKEN_COMMENT {
		_, tags, rest := parseTagsFromComment(p.current.Value)
		for k, v := range tags {
			posting.Tags[k] = v
		}
		posting.Comment = rest
		p.advance()
	}

	// Consume the newline that ends the posting line.
	if p.current.Type == ast.TOKEN_NEWLINE {
		p.advance()
	}

	return posting, nil
}

// ---------------------------------------------------------------------------
// Tag / comment helpers
// ---------------------------------------------------------------------------

// parseTagsFromComment extracts structured data from a TOKEN_COMMENT literal.
//
// The literal looks like:  "; key: value"  or  "; id: abc123"  or  "; plain note"
//
// Returns:
//   - id    — non-empty when key == "id"
//   - tags  — map of key→value for all other key: value pairs
//   - rest  — the raw comment text when no key:value was found
func parseTagsFromComment(literal string) (id string, tags map[string]string, rest string) {
	tags = make(map[string]string)

	// Strip the leading ";" and any surrounding whitespace.
	s := strings.TrimPrefix(literal, ";")
	s = strings.TrimSpace(s)

	if s == "" {
		return
	}

	// Look for "key: value" — key must be a single word (no spaces).
	key, value, found := strings.Cut(s, ":")
	if found {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && !strings.ContainsAny(key, " \t") {
			if key == "id" {
				id = value
			} else {
				tags[key] = value
			}
			return
		}
	}

	// Not a structured tag — treat as plain comment text.
	rest = s
	return
}

// applyMeta merges parsed id/tags/rest into the transaction.
func applyMeta(txn *ast.Transaction, id string, tags map[string]string, rest string) {
	if id != "" {
		txn.ID = id
	}
	for k, v := range tags {
		txn.Tags[k] = v
	}
	if rest != "" && txn.Comment == "" {
		txn.Comment = rest
	}
}

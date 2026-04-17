// Package AST defines the Abstract Syntax Tree types for the DoubleBook journal format.
// These types are the shared data model used by the lexer, parser, interpreter, and all
// downstream packages.
package ast

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Token types
// ---------------------------------------------------------------------------

// TokenType identifies the kind of token produced by the lexer.
type TokenType int

const (
	TOKEN_EOF       TokenType = iota // End of file
	TOKEN_DATE                       // YYYY-MM-DD
	TOKEN_STRING                     // Human-readable word/phrase
	TOKEN_ACCOUNT                    // expenses:food (colon-separated)
	TOKEN_AMOUNT                     // $45.32, -£10.00, 100 BGN, etc.
	TOKEN_NEWLINE                    // \n
	TOKEN_INDENT                     // Leading whitespace on a posting line (2+ spaces/tabs)
	TOKEN_ERROR                      // Lexer error — unrecognised character
	TOKEN_COMMENT                    // ; comment text
	TOKEN_STATUS                     // ! or * transaction status marker
	TOKEN_TAG_KEY                    // key part of "; key: value" in a comment
	TOKEN_TAG_VALUE                  // value part of "; key: value" in a comment
	TOKEN_CURRENCY                   // Standalone 3-letter currency code (BGN, EUR, …)
)

// Token is a single lexical unit with position information for error messages.
type Token struct {
	Type   TokenType
	Value  string
	Line   int
	Column int
}

// ---------------------------------------------------------------------------
// Amount
// ---------------------------------------------------------------------------

// Amount holds a monetary value and its currency code.
type Amount struct {
	Value    float64
	Currency string // ISO 4217 code: "USD", "GBP", "EUR", "BGN", …
}

// IsNegative reports whether the amount is less than zero.
func (a Amount) IsNegative() bool { return a.Value < 0 }

// Abs returns a copy of the amount with a non-negative value.
func (a Amount) Abs() Amount { return Amount{Value: math.Abs(a.Value), Currency: a.Currency} }

// Negate returns a copy of the amount with its sign flipped.
func (a Amount) Negate() Amount { return Amount{Value: -a.Value, Currency: a.Currency} }

// Add adds two amounts. Returns an error when the currencies differ.
func (a Amount) Add(other Amount) (Amount, error) {
	if a.Currency != other.Currency {
		return Amount{}, fmt.Errorf("cannot add %s and %s amounts", a.Currency, other.Currency)
	}
	return Amount{Value: a.Value + other.Value, Currency: a.Currency}, nil
}

// String formats the amount for display, choosing the appropriate symbol prefix
// for well-known currencies and a trailing code for others.
//
//	USD → $1,234.56  or  -$1,234.56
//	GBP → £1,234.56
//	EUR → €1,234.56
//	BGN → 1,234.56 BGN
func (a *Amount) String() string {
	abs := math.Abs(a.Value)
	formatted := formatWithCommas(abs)
	sign := ""
	if a.Value < 0 {
		sign = "-"
	}

	switch a.Currency {
	case "USD", "$", "":
		return sign + "$" + formatted
	case "GBP", "£":
		return sign + "£" + formatted
	case "EUR", "€":
		return sign + "€" + formatted
	default:
		return sign + formatted + " " + a.Currency
	}
}

// formatWithCommas formats a non-negative float with 2 decimal places and
// thousands separators, e.g. 1234567.89 → "1,234,567.89".
func formatWithCommas(v float64) string {
	// Format with 2 decimal places.
	raw := fmt.Sprintf("%.2f", v)
	parts := strings.SplitN(raw, ".", 2)
	intPart := parts[0]
	decPart := parts[1]

	// Insert commas into the integer part.
	var b strings.Builder
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	return b.String() + "." + decPart
}

// ---------------------------------------------------------------------------
// Posting
// ---------------------------------------------------------------------------

// Posting is one leg of a double-entry transaction.
type Posting struct {
	Account       string
	Amount        Amount
	AmountOmitted bool              // true when the amount was not written; filled by FillImpliedAmounts
	Tags          map[string]string // inline tags from the posting's trailing comment
	Comment       string            // raw comment text on the posting line (without leading "; ")
}

// String serialises the posting as it would appear in a journal file.
// Example output: "    expenses:groceries          $45.32"
func (p *Posting) String() string {
	var b strings.Builder

	// Two-space indent, account left-aligned in a 36-char field.
	b.WriteString(fmt.Sprintf("    %-36s", p.Account))

	if !p.AmountOmitted {
		b.WriteString("  ")
		b.WriteString(p.Amount.String())
	}

	// Emit tags and comment if present.
	var extras []string
	tagKeys := make([]string, 0, len(p.Tags))
	for k := range p.Tags {
		tagKeys = append(tagKeys, k)
	}
	sort.Strings(tagKeys)
	for _, k := range tagKeys {
		extras = append(extras, k+": "+p.Tags[k])
	}
	if p.Comment != "" {
		extras = append(extras, p.Comment)
	}
	if len(extras) > 0 {
		b.WriteString("  ; ")
		b.WriteString(strings.Join(extras, ", "))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Transaction
// ---------------------------------------------------------------------------

// Transaction is a balanced set of postings with a date and description.
// The format is hledger-compatible plain-text accounting.
type Transaction struct {
	Date        time.Time
	Description string
	Postings    []*Posting
	Tags        map[string]string // key:value pairs from "; key: value" comment lines
	ID          string            // unique ID from "; id: <hash>" comment
	Comment     string            // any other comment lines attached to this transaction
	Status      string            // "", "!", or "*" — uncleared / pending / cleared
}

// Balance returns the algebraic sum of all posting amounts.
// Postings with AmountOmitted=true contribute zero (their amount is unknown until
// FillImpliedAmounts is called).
func (t *Transaction) Balance() float64 {
	sum := 0.0
	for _, p := range t.Postings {
		if !p.AmountOmitted {
			sum += p.Amount.Value
		}
	}
	return sum
}

// IsBalanced reports whether the transaction is balanced (postings sum to zero).
// A transaction with exactly one omitted posting is considered balanced — the
// implied amount will be filled by FillImpliedAmounts.
func (t *Transaction) IsBalanced() bool {
	omitted := 0
	for _, p := range t.Postings {
		if p.AmountOmitted {
			omitted++
		}
	}
	if omitted == 1 {
		// The omitted posting will absorb the remainder — always balanced.
		return true
	}
	if omitted > 1 {
		return false
	}
	b := t.Balance()
	return b > -0.005 && b < 0.005
}

// FillImpliedAmounts computes and sets the Amount on any posting whose
// AmountOmitted field is true.
//
// Rules:
//   - If no posting is omitted, this is a no-op.
//   - If exactly one posting is omitted, its amount is set to the negation of
//     the sum of all other postings' amounts.
//   - If more than one posting is omitted, an error is returned.
func (t *Transaction) FillImpliedAmounts() error {
	var omittedIdx int = -1
	omittedCount := 0
	for i, p := range t.Postings {
		if p.AmountOmitted {
			omittedCount++
			omittedIdx = i
		}
	}

	switch omittedCount {
	case 0:
		return nil
	case 1:
		// Sum all explicit postings.
		sum := 0.0
		currency := ""
		for _, p := range t.Postings {
			if !p.AmountOmitted {
				sum += p.Amount.Value
				if currency == "" {
					currency = p.Amount.Currency
				}
			}
		}
		if currency == "" {
			currency = "USD"
		}
		t.Postings[omittedIdx].Amount = Amount{Value: -sum, Currency: currency}
		t.Postings[omittedIdx].AmountOmitted = false
		return nil
	default:
		return fmt.Errorf("transaction %q: cannot have more than one posting with an implied amount (found %d)", t.Description, omittedCount)
	}
}

// String serialises the transaction as hledger-compatible plain text.
//
// Example output:
//
//	2025-01-15 * Grocery Store  ; shopping
//	    ; category: food
//	    expenses:groceries          $45.32
//	    assets:checking            -$45.32
func (t *Transaction) String() string {
	var b strings.Builder

	// Header line: date [status] description [; comment]
	b.WriteString(t.Date.Format("2006-01-02"))
	if t.Status != "" {
		b.WriteString(" ")
		b.WriteString(t.Status)
	}
	b.WriteString(" ")
	b.WriteString(t.Description)
	if t.Comment != "" {
		b.WriteString("  ; ")
		b.WriteString(t.Comment)
	}
	b.WriteString("\n")

	// ID comment line.
	if t.ID != "" {
		b.WriteString("    ; id: ")
		b.WriteString(t.ID)
		b.WriteString("\n")
	}

	// Tag comment lines (sorted for deterministic output).
	tagKeys := make([]string, 0, len(t.Tags))
	for k := range t.Tags {
		tagKeys = append(tagKeys, k)
	}
	sort.Strings(tagKeys)
	for _, k := range tagKeys {
		b.WriteString("    ; ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(t.Tags[k])
		b.WriteString("\n")
	}

	// Posting lines.
	for _, p := range t.Postings {
		b.WriteString(p.String())
		b.WriteString("\n")
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// NewTransaction creates a Transaction with initialized maps.
func NewTransaction(date time.Time, description string) *Transaction {
	return &Transaction{
		Date:        date,
		Description: description,
		Postings:    []*Posting{},
		Tags:        make(map[string]string),
	}
}

// NewPosting creates a Posting with initialized maps.
func NewPosting(account string, amount Amount) *Posting {
	return &Posting{
		Account: account,
		Amount:  amount,
		Tags:    make(map[string]string),
	}
}

// NewOmittedPosting creates a Posting whose amount will be inferred from the
// other postings in the same transaction (implied balance).
func NewOmittedPosting(account string) *Posting {
	return &Posting{
		Account:       account,
		AmountOmitted: true,
		Tags:          make(map[string]string),
	}
}

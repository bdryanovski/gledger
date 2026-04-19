package rules

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"doublebook/core/ast"
)

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine applies a RuleSet to transform raw data rows into transactions.
type Engine struct {
	ruleSet *RuleSet
}

// parseAmount strips thousands separators (commas and spaces) and parses a float.
func parseAmount(s string) (float64, error) {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

// NewEngine creates a new rules engine with the given ruleset.
func NewEngine(rs *RuleSet) (*Engine, error) {
	if err := rs.Validate(); err != nil {
		return nil, fmt.Errorf("invalid ruleset: %w", err)
	}
	if err := rs.Compile(); err != nil {
		return nil, fmt.Errorf("compiling ruleset: %w", err)
	}
	return &Engine{ruleSet: rs}, nil
}

// RuleSet returns the engine's ruleset.
func (e *Engine) RuleSet() *RuleSet {
	return e.ruleSet
}

// ---------------------------------------------------------------------------
// Processing
// ---------------------------------------------------------------------------

// ProcessResult contains the result of processing rows through the engine.
type ProcessResult struct {
	Transactions []*ast.Transaction
	Errors       []ProcessError
	TotalRows    int
}

// ProcessError describes an error processing a specific row.
type ProcessError struct {
	Row    int
	Field  string
	Reason string
}

// ProcessRows applies the ruleset to raw data rows.
func (e *Engine) ProcessRows(rows [][]string) *ProcessResult {
	result := &ProcessResult{
		TotalRows: len(rows),
	}

	for i, row := range rows {
		txn, err := e.processRow(row, i+1)
		if err != nil {
			result.Errors = append(result.Errors, ProcessError{
				Row:    i + 1,
				Reason: err.Error(),
			})
			continue
		}
		if txn != nil {
			result.Transactions = append(result.Transactions, txn)
		}
	}

	return result
}

// processRow transforms a single row into a transaction.
func (e *Engine) processRow(row []string, rowNum int) (*ast.Transaction, error) {
	// Extract field values
	fields := make(map[string]string)
	
	for _, mapping := range e.ruleSet.Mappings {
		value, err := e.applyMapping(mapping, row)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", mapping.Field, err)
		}
		fields[mapping.Field] = value
	}

	// Parse date
	dateStr := fields["date"]
	if dateStr == "" {
		return nil, fmt.Errorf("date field is empty")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		// Try other formats
		for _, fmt := range []string{"02/01/2006", "01/02/2006", "2006/01/02"} {
			date, err = time.Parse(fmt, dateStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, fmt.Errorf("cannot parse date %q", dateStr)
		}
	}

	// Parse description
	description := fields["description"]
	if description == "" {
		description = e.ruleSet.Name + " " + date.Format("2006-01-02")
	}

	// Parse amount
	var amountValue float64
	var isCredit bool

	if amt := fields["amount"]; amt != "" {
		f, err := parseAmount(amt)
		if err != nil {
			return nil, fmt.Errorf("cannot parse amount %q: %w", amt, err)
		}
		amountValue = math.Abs(f)
		isCredit = f > 0
	} else {
		// Try debit/credit columns
		debitAmt := fields["debit_amount"]
		creditAmt := fields["credit_amount"]
		
		if debitAmt == "" && creditAmt == "" {
			// Skip row silently
			return nil, nil
		}
		
		if debitAmt != "" {
			f, err := parseAmount(debitAmt)
			if err != nil {
				return nil, fmt.Errorf("cannot parse debit_amount %q: %w", debitAmt, err)
			}
			amountValue = math.Abs(f)
			isCredit = false
		} else {
			f, err := parseAmount(creditAmt)
			if err != nil {
				return nil, fmt.Errorf("cannot parse credit_amount %q: %w", creditAmt, err)
			}
			amountValue = math.Abs(f)
			isCredit = true
		}
	}

	if amountValue == 0 {
		return nil, nil // Skip zero-amount rows
	}

	// Determine accounts
	sourceAccount := e.ruleSet.SourceAccount
	debitAccount := e.ruleSet.DefaultDebitAccount
	creditAccount := e.ruleSet.DefaultCreditAccount

	if acc := fields["debit_account"]; acc != "" {
		debitAccount = acc
	}
	if acc := fields["credit_account"]; acc != "" {
		creditAccount = acc
	}

	// Apply category rules
	for _, rule := range e.ruleSet.Categories {
		if e.matchesCategoryRule(rule, description, amountValue, isCredit) {
			if rule.SetAccount != "" {
				if isCredit {
					creditAccount = rule.SetAccount
				} else {
					debitAccount = rule.SetAccount
				}
			}
			break // First matching rule wins
		}
	}

	// Build tags
	tags := map[string]string{"source": e.ruleSet.Name}
	
	// Add tags from mappings
	for field, value := range fields {
		if strings.HasPrefix(field, "tag:") && value != "" {
			tagName := strings.TrimPrefix(field, "tag:")
			tags[tagName] = value
		}
	}
	
	// Apply category rule tags
	for _, rule := range e.ruleSet.Categories {
		if e.matchesCategoryRule(rule, description, amountValue, isCredit) {
			for k, v := range rule.SetTags {
				tags[k] = v
			}
			if rule.SetCategory != "" {
				tags["category"] = rule.SetCategory
			}
			break
		}
	}

	// Currency
	currency := e.ruleSet.Currency
	if c := fields["currency"]; c != "" {
		currency = c
	}
	if currency == "" {
		currency = "USD"
	}

	// Build transaction
	txn := ast.NewTransaction(date, description)
	txn.Tags = tags

	if isCredit {
		txn.Postings = append(txn.Postings,
			ast.NewPosting(sourceAccount, ast.Amount{Value: amountValue, Currency: currency}),
			ast.NewPosting(creditAccount, ast.Amount{Value: -amountValue, Currency: currency}),
		)
	} else {
		txn.Postings = append(txn.Postings,
			ast.NewPosting(sourceAccount, ast.Amount{Value: -amountValue, Currency: currency}),
			ast.NewPosting(debitAccount, ast.Amount{Value: amountValue, Currency: currency}),
		)
	}

	return txn, nil
}

// ---------------------------------------------------------------------------
// Mapping Application
// ---------------------------------------------------------------------------

// applyMapping evaluates a field mapping against a row.
func (e *Engine) applyMapping(m FieldMapping, row []string) (string, error) {
	switch {
	case m.Direct != nil:
		return e.applyDirect(m.Direct, row)
	case m.Combine != nil:
		return e.applyCombine(m.Combine, row)
	case m.Transform != nil:
		return e.applyTransform(m.Transform, row)
	case m.Lookup != nil:
		return e.applyLookup(m.Lookup, row)
	case m.Constant != nil:
		return m.Constant.Value, nil
	case m.Condition != nil:
		return e.applyCondition(m.Condition, row)
	default:
		return "", fmt.Errorf("no mapping type specified")
	}
}

func (e *Engine) applyDirect(d *DirectMapping, row []string) (string, error) {
	if d.Column < 0 || d.Column >= len(row) {
		return "", nil
	}
	return strings.TrimSpace(row[d.Column]), nil
}

func (e *Engine) applyCombine(c *CombineMapping, row []string) (string, error) {
	values := make([]string, len(c.Columns))
	for i, col := range c.Columns {
		if col >= 0 && col < len(row) {
			v := row[col]
			if c.Trim {
				v = strings.TrimSpace(v)
			}
			values[i] = v
		}
	}

	if c.Format != "" {
		result := c.Format
		for i, v := range values {
			placeholder := fmt.Sprintf("{%d}", i)
			result = strings.ReplaceAll(result, placeholder, v)
		}
		return result, nil
	}

	sep := c.Separator
	if sep == "" {
		sep = " "
	}
	
	// Filter empty values
	nonEmpty := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			nonEmpty = append(nonEmpty, v)
		}
	}
	return strings.Join(nonEmpty, sep), nil
}

func (e *Engine) applyTransform(t *TransformMapping, row []string) (string, error) {
	fn, ok := GetTransform(t.Function)
	if !ok {
		return "", fmt.Errorf("unknown transform function %q", t.Function)
	}

	// Gather input values
	var values []string
	if len(t.Columns) > 0 {
		for _, col := range t.Columns {
			if col >= 0 && col < len(row) {
				values = append(values, row[col])
			} else {
				values = append(values, "")
			}
		}
	} else if t.Column >= 0 && t.Column < len(row) {
		values = []string{row[t.Column]}
	}

	return fn(values, t.Args)
}

func (e *Engine) applyLookup(l *LookupMapping, row []string) (string, error) {
	if l.Column < 0 || l.Column >= len(row) {
		return l.Default, nil
	}

	value := strings.TrimSpace(row[l.Column])
	if !l.CaseSensitive {
		value = strings.ToLower(value)
	}

	for k, v := range l.Table {
		key := k
		if !l.CaseSensitive {
			key = strings.ToLower(k)
		}
		if value == key {
			return v, nil
		}
	}

	return l.Default, nil
}

func (e *Engine) applyCondition(c *ConditionalMapping, row []string) (string, error) {
	for _, branch := range c.Conditions {
		if e.evaluateCondition(branch.When, row) {
			return e.applyMapping(*branch.Mapping, row)
		}
	}
	
	if c.Default != nil {
		return e.applyMapping(*c.Default, row)
	}
	
	return "", nil
}

// ---------------------------------------------------------------------------
// Condition Evaluation
// ---------------------------------------------------------------------------

func (e *Engine) evaluateCondition(cond Condition, row []string) bool {
	if cond.Column < 0 || cond.Column >= len(row) {
		return false
	}
	
	value := strings.TrimSpace(row[cond.Column])
	
	// String conditions
	if cond.Contains != "" {
		if !strings.Contains(strings.ToLower(value), strings.ToLower(cond.Contains)) {
			return false
		}
	}
	
	if cond.Equals != "" {
		if strings.ToLower(value) != strings.ToLower(cond.Equals) {
			return false
		}
	}
	
	if cond.Regex != "" && cond.compiledRegex != nil {
		if !cond.compiledRegex.MatchString(value) {
			return false
		}
	}
	
	// Numeric conditions
	if cond.GreaterThan != nil || cond.LessThan != nil {
		numValue, err := parseAmount(value)
		if err != nil {
			return false
		}
		
		if cond.GreaterThan != nil && numValue <= *cond.GreaterThan {
			return false
		}
		if cond.LessThan != nil && numValue >= *cond.LessThan {
			return false
		}
	}
	
	return true
}

// ---------------------------------------------------------------------------
// Category Rule Matching
// ---------------------------------------------------------------------------

func (e *Engine) matchesCategoryRule(rule CategoryRule, description string, amount float64, isCredit bool) bool {
	match := rule.Match
	
	// Description contains - match if ANY pattern is found
	if len(match.DescriptionContains) > 0 {
		found := false
		for _, pattern := range match.DescriptionContains {
			if strings.Contains(strings.ToLower(description), strings.ToLower(pattern)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Description regex
	if match.compiledRegex != nil {
		if !match.compiledRegex.MatchString(description) {
			return false
		}
	}
	
	// Amount range
	if match.AmountMin != nil && amount < *match.AmountMin {
		return false
	}
	if match.AmountMax != nil && amount > *match.AmountMax {
		return false
	}
	
	// Is debit/credit
	if match.IsDebit != nil {
		if *match.IsDebit != !isCredit {
			return false
		}
	}
	
	return true
}

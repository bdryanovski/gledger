package fql

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Query AST
// ---------------------------------------------------------------------------

// Query is the top-level node of a parsed FQL statement.
type Query struct {
	Distinct bool
	Columns  []SelectColumn
	From     string     // virtual table name (lower-cased)
	Where    Expression // nil if no WHERE clause
	GroupBy  []string
	Having   Expression // nil if no HAVING clause
	OrderBy  []OrderSpec
	Limit    *int
	Offset   *int
}

// SelectColumn is one item in the SELECT list.
type SelectColumn struct {
	Expr  Expression
	Alias string // from AS clause; may be empty
	Star  bool   // true for bare SELECT *
}

// OrderSpec is one item in an ORDER BY clause.
type OrderSpec struct {
	Column string
	Desc   bool
}

// ---------------------------------------------------------------------------
// Expression types
// ---------------------------------------------------------------------------

// Expression is implemented by all expression node types.
type Expression interface{ exprNode() }

// BinaryExpr covers =, !=, <, <=, >, >=, LIKE, AND, OR.
type BinaryExpr struct {
	Left  Expression
	Op    string
	Right Expression
}

// UnaryExpr covers NOT.
type UnaryExpr struct {
	Op   string
	Expr Expression
}

// Identifier is a bare column or alias reference.
type Identifier struct{ Name string }

// NumberLiteral is a numeric constant (possibly negative).
type NumberLiteral struct {
	Value float64
	Raw   string
}

// StringLiteral is a single-quoted string constant.
type StringLiteral struct{ Value string }

// StarLiteral is * inside COUNT(*).
type StarLiteral struct{}

// FunctionCall is COUNT(*), SUM(expr), etc.
type FunctionCall struct {
	Name string
	Args []Expression
	Star bool // true for COUNT(*)
}

// InExpr is col [NOT] IN (val, val, …).
type InExpr struct {
	Left   Expression
	Values []Expression
	Not    bool
}

// BetweenExpr is col BETWEEN low AND high.
type BetweenExpr struct {
	Expr Expression
	Low  Expression
	High Expression
}

// IsNullExpr is col IS [NOT] NULL.
type IsNullExpr struct {
	Expr Expression
	Not  bool
}

// exprNode marker implementations.
func (*BinaryExpr) exprNode()    {}
func (*UnaryExpr) exprNode()     {}
func (*Identifier) exprNode()    {}
func (*NumberLiteral) exprNode() {}
func (*StringLiteral) exprNode() {}
func (*StarLiteral) exprNode()   {}
func (*FunctionCall) exprNode()  {}
func (*InExpr) exprNode()        {}
func (*BetweenExpr) exprNode()   {}
func (*IsNullExpr) exprNode()    {}

// ---------------------------------------------------------------------------
// Parser
// ---------------------------------------------------------------------------

// Parser converts a stream of FQL tokens into a *Query AST.
type Parser struct {
	l *Lexer
}

// NewParser creates and returns a Parser for the given FQL input.
// Returns an error if the input cannot be tokenized.
func NewParser(input string) (*Parser, error) {
	l := NewLexer(input)
	if err := l.Tokenize(); err != nil {
		return nil, err
	}
	return &Parser{l: l}, nil
}

// Parse parses the FQL and returns a Query AST.
func (p *Parser) Parse() (*Query, error) {
	q := &Query{}

	// SELECT
	if err := p.expect(FQL_SELECT); err != nil {
		return nil, err
	}

	// DISTINCT?
	if p.peek().Type == FQL_DISTINCT {
		p.next()
		q.Distinct = true
	}

	// Column list.
	cols, err := p.parseColumnList()
	if err != nil {
		return nil, err
	}
	q.Columns = cols

	// FROM
	if p.peek().Type != FQL_FROM {
		return nil, fmt.Errorf("expected keyword 'FROM' at position %d, got %s (%s)",
			p.peek().Pos, p.peek().Literal, p.peek().Type)
	}
	p.next()

	// Table name.
	tbl := p.next()
	if tbl.Type != FQL_IDENT {
		return nil, fmt.Errorf("expected table name after FROM, got %q", tbl.Literal)
	}
	q.From = strings.ToLower(tbl.Literal)

	// Optional clauses.
	for {
		tok := p.peek()
		switch tok.Type {
		case FQL_WHERE:
			p.next()
			q.Where, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		case FQL_GROUP:
			p.next()
			if err := p.expect(FQL_BY); err != nil {
				return nil, err
			}
			q.GroupBy, err = p.parseIdentList()
			if err != nil {
				return nil, err
			}
		case FQL_HAVING:
			p.next()
			q.Having, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		case FQL_ORDER:
			p.next()
			if err := p.expect(FQL_BY); err != nil {
				return nil, err
			}
			q.OrderBy, err = p.parseOrderList()
			if err != nil {
				return nil, err
			}
		case FQL_LIMIT:
			p.next()
			n, err := p.parseInt()
			if err != nil {
				return nil, fmt.Errorf("LIMIT: %w", err)
			}
			q.Limit = &n
		case FQL_OFFSET:
			p.next()
			n, err := p.parseInt()
			if err != nil {
				return nil, fmt.Errorf("OFFSET: %w", err)
			}
			q.Offset = &n
		case FQL_EOF:
			return q, nil
		default:
			return nil, fmt.Errorf("unexpected token %q (%s) at position %d",
				tok.Literal, tok.Type, tok.Pos)
		}
	}
}

// ---------------------------------------------------------------------------
// Column list parsing
// ---------------------------------------------------------------------------

func (p *Parser) parseColumnList() ([]SelectColumn, error) {
	// SELECT * shorthand.
	if p.peek().Type == FQL_STAR {
		p.next()
		return []SelectColumn{{Star: true}}, nil
	}

	var cols []SelectColumn
	for {
		col, err := p.parseSelectColumn()
		if err != nil {
			return nil, err
		}
		cols = append(cols, col)
		if p.peek().Type != FQL_COMMA {
			break
		}
		p.next() // consume ','
	}
	return cols, nil
}

func (p *Parser) parseSelectColumn() (SelectColumn, error) {
	expr, err := p.parsePrimary()
	if err != nil {
		return SelectColumn{}, err
	}

	col := SelectColumn{Expr: expr}

	// Optional AS alias.
	if p.peek().Type == FQL_AS {
		p.next()
		alias := p.next()
		if alias.Type != FQL_IDENT {
			return SelectColumn{}, fmt.Errorf("expected alias after AS, got %q", alias.Literal)
		}
		col.Alias = alias.Literal
	}

	return col, nil
}

// ---------------------------------------------------------------------------
// Expression parsing (recursive descent, correct operator precedence)
// ---------------------------------------------------------------------------

// parseExpr → OR chain
func (p *Parser) parseExpr() (Expression, error) {
	return p.parseOrExpr()
}

func (p *Parser) parseOrExpr() (Expression, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == FQL_OR {
		p.next()
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "OR", Right: right}
	}
	return left, nil
}

func (p *Parser) parseAndExpr() (Expression, error) {
	left, err := p.parseNotExpr()
	if err != nil {
		return nil, err
	}
	for p.peek().Type == FQL_AND {
		p.next()
		right, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "AND", Right: right}
	}
	return left, nil
}

func (p *Parser) parseNotExpr() (Expression, error) {
	if p.peek().Type == FQL_NOT {
		p.next()
		expr, err := p.parseNotExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "NOT", Expr: expr}, nil
	}
	return p.parseCompareExpr()
}

func (p *Parser) parseCompareExpr() (Expression, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	tok := p.peek()
	switch tok.Type {
	case FQL_EQ, FQL_NEQ, FQL_LT, FQL_LTE, FQL_GT, FQL_GTE, FQL_LIKE:
		p.next()
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		op := map[TokenType]string{
			FQL_EQ: "=", FQL_NEQ: "!=", FQL_LT: "<", FQL_LTE: "<=",
			FQL_GT: ">", FQL_GTE: ">=", FQL_LIKE: "LIKE",
		}[tok.Type]
		return &BinaryExpr{Left: left, Op: op, Right: right}, nil

	case FQL_IS:
		p.next()
		not := false
		if p.peek().Type == FQL_NOT {
			p.next()
			not = true
		}
		if err := p.expect(FQL_NULL); err != nil {
			return nil, fmt.Errorf("IS [NOT] NULL: %w", err)
		}
		return &IsNullExpr{Expr: left, Not: not}, nil

	case FQL_BETWEEN:
		p.next()
		low, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		if err := p.expect(FQL_AND); err != nil {
			return nil, fmt.Errorf("BETWEEN … AND: %w", err)
		}
		high, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &BetweenExpr{Expr: left, Low: low, High: high}, nil

	case FQL_IN:
		p.next()
		vals, err := p.parseParenList()
		if err != nil {
			return nil, err
		}
		return &InExpr{Left: left, Values: vals, Not: false}, nil

	case FQL_NOT:
		// NOT IN
		p.next()
		if err := p.expect(FQL_IN); err != nil {
			return nil, fmt.Errorf("NOT IN: %w", err)
		}
		vals, err := p.parseParenList()
		if err != nil {
			return nil, err
		}
		return &InExpr{Left: left, Values: vals, Not: true}, nil
	}

	return left, nil
}

func (p *Parser) parsePrimary() (Expression, error) {
	tok := p.peek()
	switch tok.Type {
	case FQL_NUMBER:
		p.next()
		v, err := strconv.ParseFloat(tok.Literal, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", tok.Literal, err)
		}
		return &NumberLiteral{Value: v, Raw: tok.Literal}, nil

	case FQL_STRING:
		p.next()
		return &StringLiteral{Value: tok.Literal}, nil

	case FQL_STAR:
		p.next()
		return &StarLiteral{}, nil

	case FQL_NULL:
		p.next()
		return &Identifier{Name: "NULL"}, nil

	case FQL_LPAREN:
		p.next()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(FQL_RPAREN); err != nil {
			return nil, fmt.Errorf("closing ')': %w", err)
		}
		return expr, nil

	// Aggregate functions + identifiers.
	case FQL_COUNT, FQL_SUM, FQL_AVG, FQL_MIN, FQL_MAX, FQL_IDENT:
		return p.parseIdentOrFunc()

	default:
		return nil, fmt.Errorf("unexpected token %q (%s) in expression at position %d",
			tok.Literal, tok.Type, tok.Pos)
	}
}

// parseIdentOrFunc handles both plain identifiers and function calls.
func (p *Parser) parseIdentOrFunc() (Expression, error) {
	nameTok := p.next()
	name := nameTok.Literal

	// Not followed by '(' → plain identifier.
	if p.peek().Type != FQL_LPAREN {
		return &Identifier{Name: name}, nil
	}

	// Function call.
	p.next() // consume '('
	fn := &FunctionCall{Name: strings.ToUpper(name)}

	if p.peek().Type == FQL_STAR {
		// COUNT(*)
		p.next()
		fn.Star = true
	} else if p.peek().Type == FQL_RPAREN {
		// COUNT() — empty args
	} else {
		for {
			arg, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			fn.Args = append(fn.Args, arg)
			if p.peek().Type != FQL_COMMA {
				break
			}
			p.next()
		}
	}

	if err := p.expect(FQL_RPAREN); err != nil {
		return nil, fmt.Errorf("closing ')' in function call: %w", err)
	}
	return fn, nil
}

// ---------------------------------------------------------------------------
// List parsers
// ---------------------------------------------------------------------------

func (p *Parser) parseIdentList() ([]string, error) {
	var out []string
	for {
		tok := p.next()
		if tok.Type != FQL_IDENT {
			return nil, fmt.Errorf("expected identifier in list, got %q", tok.Literal)
		}
		out = append(out, tok.Literal)
		if p.peek().Type != FQL_COMMA {
			break
		}
		p.next()
	}
	return out, nil
}

func (p *Parser) parseOrderList() ([]OrderSpec, error) {
	var out []OrderSpec
	for {
		tok := p.next()
		if tok.Type != FQL_IDENT {
			return nil, fmt.Errorf("expected column name in ORDER BY, got %q", tok.Literal)
		}
		spec := OrderSpec{Column: tok.Literal}
		if p.peek().Type == FQL_DESC {
			p.next()
			spec.Desc = true
		} else if p.peek().Type == FQL_ASC {
			p.next()
		}
		out = append(out, spec)
		if p.peek().Type != FQL_COMMA {
			break
		}
		p.next()
	}
	return out, nil
}

func (p *Parser) parseParenList() ([]Expression, error) {
	if err := p.expect(FQL_LPAREN); err != nil {
		return nil, err
	}
	var vals []Expression
	for {
		expr, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		vals = append(vals, expr)
		if p.peek().Type != FQL_COMMA {
			break
		}
		p.next()
	}
	if err := p.expect(FQL_RPAREN); err != nil {
		return nil, err
	}
	return vals, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (p *Parser) next() Token { return p.l.Next() }
func (p *Parser) peek() Token { return p.l.Peek() }

func (p *Parser) expect(tt TokenType) error {
	tok := p.next()
	if tok.Type != tt {
		return fmt.Errorf("expected %s at position %d, got %q (%s)",
			tt, tok.Pos, tok.Literal, tok.Type)
	}
	return nil
}

func (p *Parser) parseInt() (int, error) {
	tok := p.next()
	if tok.Type != FQL_NUMBER {
		return 0, fmt.Errorf("expected integer, got %q", tok.Literal)
	}
	v, err := strconv.Atoi(tok.Literal)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", tok.Literal, err)
	}
	return v, nil
}

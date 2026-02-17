package Parser

import (
	"fmt"
	AST "gledger/ast"
	"gledger/lexer"
	"gledger/utils"
	"time"
)

type Parser struct {
	lexer   *lexer.Lexer
	current AST.Token // current token
	peek    AST.Token // next token
}

func runParser(input string) *Parser {
	lexer := lexer.CreateLexer(input)
	parser := &Parser{lexer: lexer}

	parser.nextToken()
	parser.nextToken()

	return parser
}

func (parser *Parser) nextToken() {
	parser.current = parser.peek
	parser.peek = parser.lexer.NextToken()
}

func (parser *Parser) skipNewLines() {
	for parser.current.Type == AST.TOKEN_NEWLINE {
		parser.nextToken()
	}
}

func (parser *Parser) Parse() ([]*AST.Transaction, error) {
	var transactions []*AST.Transaction

	parser.skipNewLines()
	for parser.current.Type != AST.TOKEN_EOF {
		t, err := parser.parserTransaction()
		if err != nil {
			return nil, fmt.Errorf("Error parsing transaction at line %d: %v", parser.current.Line, err)
		}

		if t != nil {
			transactions = append(transactions, t)
		}

		parser.skipNewLines()

	}
	return transactions, nil
}

func (parser *Parser) parserTransaction() (*AST.Transaction, error) {
	if parser.current.Type != AST.TOKEN_DATE {
		return nil, fmt.Errorf("Expected date at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	date, err := time.Parse("2006-01-02", parser.current.Value)
	if err != nil {
		return nil, fmt.Errorf("Invalid date format at line %d: %v", parser.current.Line, err)
	}

	parser.nextToken()

	description := ""

	for parser.current.Type == AST.TOKEN_STRING {
		if description != "" {
			description += " "
		}
		description += parser.current.Value
		parser.nextToken()
	}

	if description == "" {
		return nil, fmt.Errorf("Missing description at line %d", parser.current.Line)
	}

	if parser.current.Type != AST.TOKEN_NEWLINE {
		return nil, fmt.Errorf("Expected newline after description at line %d", parser.current.Line)
	}
	parser.nextToken()

	postings := []AST.Posting{}

	for parser.current.Type == AST.TOKEN_INDENT {
		posting, err := parser.parsePosting()

		if err != nil {
			return nil, fmt.Errorf("Error parsing posting at line %d: %v", parser.current.Line, err)
		}
		postings = append(postings, posting)
	}

	if len(postings) < 2 {
		return nil, fmt.Errorf("Transaction must have at least two posting at line %d", parser.current.Line)
	}

	currentTransaction := &AST.Transaction{
		Date:        date,
		Description: description,
		Postings:    postings,
	}

	if !currentTransaction.IsBalanced() {
		return nil, fmt.Errorf("Transaction is not balanced at line %d (sum: %.2f)", parser.current.Line, currentTransaction.Balance())
	}

	return currentTransaction, nil
}

func (parser *Parser) parsePosting() (AST.Posting, error) {
	if parser.current.Type != AST.TOKEN_INDENT {
		return AST.Posting{}, fmt.Errorf("Expected indent at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	parser.nextToken()

	if parser.current.Type != AST.TOKEN_ACCOUNT {
		return AST.Posting{}, fmt.Errorf("Expected account at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	account := parser.current.Value
	parser.nextToken()

	if parser.current.Type != AST.TOKEN_AMOUNT {
		return AST.Posting{}, fmt.Errorf("Expected amount at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	amount, err := utils.ParseAmount(parser.current.Value)
	if err != nil {
		return AST.Posting{}, fmt.Errorf("Invalid amount format at line %d: %v", parser.current.Line, err)
	}

	parser.nextToken()

	if parser.current.Type != AST.TOKEN_NEWLINE {
		return AST.Posting{}, fmt.Errorf("Expected newline after posting at line %d", parser.current.Line)
	}
	parser.nextToken()

	return AST.Posting{Account: account, Amount: amount}, nil
}

// parseTransactions takes raw ledger text and returns parsed transactions.
// This is the testable core logic, separated from main().
func ParseTransactions(input string) ([]*AST.Transaction, error) {
	parser := runParser(input)
	return parser.Parse()
}

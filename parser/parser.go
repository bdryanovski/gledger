package parser

import (
	"fmt"
	"gledger/ast"
	"gledger/lexer"
	"strconv"
	"strings"
	"time"
)

type Parser struct {
	lexer   *lexer.Lexer
	current AST.Token // current token
	peek    AST.Token // next token
}

type Amount struct {
	Value    float64
	Currency string
}

// Too lazy to do it right now, we will do it later
func (amount *Amount) String() string {
	if amount.Value < 0 {
		return fmt.Sprintf("-$%.2f", -amount.Value)
	}
	return fmt.Sprintf("$%.2f", amount.Value)
}

type Posting struct {
	Account string
	Amount  Amount
}

type Transaction struct {
	Date        time.Time
	Description string
	Postings    []Posting
}

// Calculate the balance of a transaction by summing up the amounts of its postings
func (transaction *Transaction) Balance() float64 {
	balance := 0.0
	for _, posting := range transaction.Postings {
		balance += posting.Amount.Value
	}
	return balance
}

// Debug safety check
func (transaction *Transaction) IsBalanced() bool {
	balance := transaction.Balance()
	// floating points are failing me sometimes
	return balance > -0.01 && balance < 0.01
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

func (parser *Parser) Parse() ([]*Transaction, error) {
	var transactions []*Transaction

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

func (parser *Parser) parserTransaction() (*Transaction, error) {
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

	postings := []Posting{}

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

	currentTransaction := &Transaction{
		Date:        date,
		Description: description,
		Postings:    postings,
	}

	if !currentTransaction.IsBalanced() {
		return nil, fmt.Errorf("Transaction is not balanced at line %d (sum: %.2f)", parser.current.Line, currentTransaction.Balance())
	}

	return currentTransaction, nil
}

func (parser *Parser) parsePosting() (Posting, error) {
	if parser.current.Type != AST.TOKEN_INDENT {
		return Posting{}, fmt.Errorf("Expected indent at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	parser.nextToken()

	if parser.current.Type != AST.TOKEN_ACCOUNT {
		return Posting{}, fmt.Errorf("Expected account at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	account := parser.current.Value
	parser.nextToken()

	if parser.current.Type != AST.TOKEN_AMOUNT {
		return Posting{}, fmt.Errorf("Expected amount at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	amount, err := parserAmount(parser.current.Value)
	if err != nil {
		return Posting{}, fmt.Errorf("Invalid amount format at line %d: %v", parser.current.Line, err)
	}

	parser.nextToken()

	if parser.current.Type != AST.TOKEN_NEWLINE {
		return Posting{}, fmt.Errorf("Expected newline after posting at line %d", parser.current.Line)
	}
	parser.nextToken()

	return Posting{Account: account, Amount: amount}, nil
}

// Helper function to parse amount string into Amount struct
func parserAmount(amountStr string) (Amount, error) {
	amount := strings.TrimSpace(amountStr)
	amount = strings.ReplaceAll(amount, "$", "")
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return Amount{}, err
	}

	// Extend it for more currencies
	return Amount{Value: value, Currency: "USD"}, nil

}

// parseTransactions takes raw ledger text and returns parsed transactions.
// This is the testable core logic, separated from main().
func ParseTransactions(input string) ([]*Transaction, error) {
	parser := runParser(input)
	return parser.Parse()
}

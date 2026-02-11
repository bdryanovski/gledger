package main

import "fmt"
import "strings"
import "time"
import "strconv"
import "os"

/**
 * AST - Abstract Syntax Tree
 */

type TokenType int

const (
	TOKEN_EOF     TokenType = iota // End of File
	TOKEN_DATE                     // YYYY-MM-DD
	TOKEN_STRING                   // Human readable name
	TOKEN_ACCOUNT                  // expenses:food
	TOKEN_AMOUNT                   // 100.00 USD
	TOKEN_NEWLINE                  // \n
	TOKEN_INDENT                   // Indentation (for nested transactions)
	TOKEN_ERROR                    // Error token
)

/**
 * Now we need to know where the token are into the source code
 */

type Token struct {
	Type   TokenType // Type of the token
	Value  string    // The actual text of the token
	Line   int       // Line number in the source code
	Column int       // Column number in the source code
}

type Lexer struct {
	input      string
	position   int
	line       int
	column     int
	lastColumn int
}

func CreateLexer(input string) *Lexer {
	return &Lexer{
		input:      input,
		position:   0,
		line:       1,
		column:     0,
		lastColumn: 0,
	}
}

/**
 * Help the lexer peek what is in front of it without moving the cursor
 */
func (lexer *Lexer) peek() byte {
	if lexer.position >= len(lexer.input) {
		return 0 // EOF
	}
	return lexer.input[lexer.position]
}

/**
 * Advance the lexer by one character and update line and column numbers
 */
func (lexer *Lexer) advance() byte {
	if lexer.position >= len(lexer.input) {
		return 0 // EOF
	}

	character := lexer.input[lexer.position]
	lexer.position++
	lexer.column++

	/**
	 * If we encounter a newline, we need to update the line and reset the column
	 */
	if character == '\n' {
		lexer.line++
		lexer.column = 0
	}

	return character
}

func (lexer *Lexer) skipWhitespace() {
	for lexer.peek() == ' ' || lexer.peek() == '\t' {
		lexer.advance()
	}
}

func isDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

func isLetter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

/** Tokens **/

func (lexer *Lexer) nextToken() Token {
	lexer.lastColumn = lexer.column

	if lexer.column > 0 {
		lexer.skipWhitespace()
	}

	// End of file
	if lexer.position >= len(lexer.input) {
		return Token{Type: TOKEN_EOF, Value: "", Line: lexer.line, Column: lexer.column}
	}

	character := lexer.peek()

	if character == '\n' {
		character = lexer.advance()
		return Token{Type: TOKEN_NEWLINE, Value: "\n", Line: lexer.line - 1, Column: lexer.lastColumn}
	}

	// Identation
	if lexer.column == 0 && (character == ' ' || character == '\t') {
		indent := ""
		for lexer.peek() == ' ' || lexer.peek() == '\t' {
			indent += string(lexer.advance())
		}

		if len(indent) >= 2 {
			return Token{Type: TOKEN_INDENT, Value: indent, Line: lexer.line, Column: 0}
		}
	}

	// Digits
	if isDigit(character) {
		date := "" // set as empty string for now
		for isDigit(lexer.peek()) || lexer.peek() == '-' {
			date += string(lexer.advance())
		}

		// Pretty lame check but it should work for now, we will improve it later
		if len(date) == 10 && date[4] == '-' && date[7] == '-' {
			return Token{Type: TOKEN_DATE, Value: date, Line: lexer.line, Column: lexer.lastColumn}
		}

		lexer.position -= len(date)
		lexer.column -= len(date)
	}

	// maybe is amount ?
	if character == '$' || character == '-' || isDigit(character) {
		amount := ""

		// Could be negative ?
		if character == '-' {
			amount += string(lexer.advance())
		}

		// Currency symbol ?
		if lexer.peek() == '$' {
			amount += string(lexer.advance())
		}

		// Digits and decimal point
		for isDigit(lexer.peek()) || lexer.peek() == '.' {
			amount += string(lexer.advance())
		}

		if len(amount) > 0 {
			return Token{Type: TOKEN_AMOUNT, Value: amount, Line: lexer.line, Column: lexer.lastColumn}
		}
	}

	// accounts
	if isLetter(character) {
		account := ""

		for isLetter(lexer.peek()) || lexer.peek() == ':' || lexer.peek() == '_' || isDigit(lexer.peek()) {
			account += string(lexer.advance())
		}

		if strings.Contains(account, ":") {
			return Token{Type: TOKEN_ACCOUNT, Value: account, Line: lexer.line, Column: lexer.lastColumn}
		}

		return Token{Type: TOKEN_STRING, Value: account, Line: lexer.line, Column: lexer.lastColumn}
	}

	// random things ...

	if character != 'n' && character != 0 {
		value := ""
		for lexer.peek() != '\n' && lexer.peek() != 0 && lexer.peek() != '$' {
			if lexer.peek() == ' ' {
				nextChar := lexer.position + 1
				if nextChar < len(lexer.input) {
					nextCharacter := lexer.input[nextChar]
					if nextCharacter == '$' || nextCharacter == '-' {
						break
					}
				}
			}
			value += string(lexer.advance())

		}

		value = strings.TrimSpace(value) // clean up
		if len(value) > 0 {
			return Token{Type: TOKEN_STRING, Value: value, Line: lexer.line, Column: lexer.lastColumn}
		}
	}

	// If we reach here, it's an error
	return Token{Type: TOKEN_ERROR, Value: string(character), Line: lexer.line, Column: lexer.column}
}

/** Ast **/

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

/** Parser **/
type Parser struct {
	lexer   *Lexer
	current Token // current token
	peek    Token // next token
}

func runParser(input string) *Parser {
	lexer := CreateLexer(input)
	parser := &Parser{lexer: lexer}

	parser.nextToken()
	parser.nextToken()

	return parser
}

func (parser *Parser) nextToken() {
	parser.current = parser.peek
	parser.peek = parser.lexer.nextToken()
}

func (parser *Parser) skipNewLines() {
	for parser.current.Type == TOKEN_NEWLINE {
		parser.nextToken()
	}
}

func (parser *Parser) Parse() ([]*Transaction, error) {
	var transactions []*Transaction

	parser.skipNewLines()
	for parser.current.Type != TOKEN_EOF {
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
	if parser.current.Type != TOKEN_DATE {
		return nil, fmt.Errorf("Expected date at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	date, err := time.Parse("2006-01-02", parser.current.Value)
	if err != nil {
		return nil, fmt.Errorf("Invalid date format at line %d: %v", parser.current.Line, err)
	}

	parser.nextToken()

	description := ""

	for parser.current.Type == TOKEN_STRING {
		if description != "" {
			description += " "
		}
		description += parser.current.Value
		parser.nextToken()
	}

	if description == "" {
		return nil, fmt.Errorf("Missing description at line %d", parser.current.Line)
	}

	if parser.current.Type != TOKEN_NEWLINE {
		return nil, fmt.Errorf("Expected newline after description at line %d", parser.current.Line)
	}
	parser.nextToken()

	postings := []Posting{}

	for parser.current.Type == TOKEN_INDENT {
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
	if parser.current.Type != TOKEN_INDENT {
		return Posting{}, fmt.Errorf("Expected indent at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	parser.nextToken()

	if parser.current.Type != TOKEN_ACCOUNT {
		return Posting{}, fmt.Errorf("Expected account at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	account := parser.current.Value
	parser.nextToken()

	if parser.current.Type != TOKEN_AMOUNT {
		return Posting{}, fmt.Errorf("Expected amount at line %d, got %s", parser.current.Line, parser.current.Value)
	}

	amount, err := parserAmount(parser.current.Value)
	if err != nil {
		return Posting{}, fmt.Errorf("Invalid amount format at line %d: %v", parser.current.Line, err)
	}

	parser.nextToken()

	if parser.current.Type != TOKEN_NEWLINE {
		return Posting{}, fmt.Errorf("Expected newline after posting at line %d", parser.current.Line)
	}
	parser.nextToken()

	return Posting{Account: account, Amount: amount}, nil
}

// Helper function to parse amount string into Amount struct
func parserAmount(amountStr string) (Amount, error) {
	amount := strings.TrimSpace(amountStr)
	amount = strings.Replace(amount, "$", "", -1)
	value, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return Amount{}, err
	}

	// Extend it for more currencies
	return Amount{Value: value, Currency: "USD"}, nil

}

// parseTransactions takes raw ledger text and returns parsed transactions.
// This is the testable core logic, separated from main().
func parseTransactions(input string) ([]*Transaction, error) {
	parser := runParser(input)
	return parser.Parse()
}

func main() {
	data, err := os.ReadFile("./example/transactions.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	txns, err := parseTransactions(string(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Parsed %d transactions\n", len(txns))

	for i, txn := range txns {
		fmt.Printf("\n--- Transaction %d ---\n", i+1)
		fmt.Printf("  Date:        %s\n", txn.Date.Format("2006-01-02"))
		fmt.Printf("  Description: %s\n", txn.Description)
		fmt.Printf("  Balanced:    %v\n", txn.IsBalanced())
		for _, p := range txn.Postings {
			fmt.Printf("    %-30s %s\n", p.Account, p.Amount.String())
		}
	}
}

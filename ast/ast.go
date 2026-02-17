package AST

import (
	"fmt"
	"time"
)

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
	TOKEN_COMMENT                  // Comment (ignored by the parser)
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

/**
 * Some generic types later on use by the parser and the interpreter
 */

type Posting struct {
	Account string
	Amount  Amount
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

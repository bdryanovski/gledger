package AST

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

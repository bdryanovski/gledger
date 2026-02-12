package lexer

import "strings"
import "gledger/ast"

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

func (lexer *Lexer) NextToken() AST.Token {
	lexer.lastColumn = lexer.column

	if lexer.column > 0 {
		lexer.skipWhitespace()
	}

	// End of file
	if lexer.position >= len(lexer.input) {
		return AST.Token{Type: AST.TOKEN_EOF, Value: "", Line: lexer.line, Column: lexer.column}
	}

	character := lexer.peek()

	if character == '\n' {
		character = lexer.advance()
		return AST.Token{Type: AST.TOKEN_NEWLINE, Value: "\n", Line: lexer.line - 1, Column: lexer.lastColumn}
	}

	// Identation
	if lexer.column == 0 && (character == ' ' || character == '\t') {
		indent := ""
		for lexer.peek() == ' ' || lexer.peek() == '\t' {
			indent += string(lexer.advance())
		}

		if len(indent) >= 2 {
			return AST.Token{Type: AST.TOKEN_INDENT, Value: indent, Line: lexer.line, Column: 0}
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
			return AST.Token{Type: AST.TOKEN_DATE, Value: date, Line: lexer.line, Column: lexer.lastColumn}
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
			return AST.Token{Type: AST.TOKEN_AMOUNT, Value: amount, Line: lexer.line, Column: lexer.lastColumn}
		}
	}

	// accounts
	if isLetter(character) {
		account := ""

		for isLetter(lexer.peek()) || lexer.peek() == ':' || lexer.peek() == '_' || isDigit(lexer.peek()) {
			account += string(lexer.advance())
		}

		if strings.Contains(account, ":") {
			return AST.Token{Type: AST.TOKEN_ACCOUNT, Value: account, Line: lexer.line, Column: lexer.lastColumn}
		}

		return AST.Token{Type: AST.TOKEN_STRING, Value: account, Line: lexer.line, Column: lexer.lastColumn}
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
			return AST.Token{Type: AST.TOKEN_STRING, Value: value, Line: lexer.line, Column: lexer.lastColumn}
		}
	}

	// If we reach here, it's an error
	return AST.Token{Type: AST.TOKEN_ERROR, Value: string(character), Line: lexer.line, Column: lexer.column}
}

/** Utils */

func isDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

func isLetter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

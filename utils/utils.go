package utils

import (
	"gledger/ast"
	"strconv"
	"strings"
)

func ParseAmount(s string) (AST.Amount, error) {
	s = strings.TrimSpace(s)

	// Remove currency symbol
	s = strings.Replace(s, "$", "", -1)

	// Parse the numeric value
	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return AST.Amount{}, err
	}

	return AST.Amount{
		Value:    value,
		Currency: "USD",
	}, nil
}

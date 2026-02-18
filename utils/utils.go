package utils

import (
	AST "gledger/ast"
	"os"
	"path/filepath"
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

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

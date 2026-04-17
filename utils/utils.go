package utils

import (
	AST "doublebook/ast"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseAmount parses an amount string into an AST.Amount, detecting the currency
// from the symbol prefix or trailing 3-letter code.
//
// Supported formats:
//
//	$45.32         → USD  45.32
//	-$45.32        → USD -45.32
//	£10.50         → GBP  10.50
//	€25.00         → EUR  25.00
//	100 BGN        → BGN  100.0
//	-100 BGN       → BGN -100.0
//	BGN 100        → BGN  100.0
//	45.32          → USD  45.32  (default)
//	1,234.56       → USD  1234.56 (commas stripped)
func ParseAmount(s string) (AST.Amount, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return AST.Amount{}, fmt.Errorf("empty amount string")
	}

	currency := "USD"
	negative := false

	// Extract a leading negative sign before any currency symbol.
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}

	// Detect currency from prefix symbol.
	switch {
	case strings.HasPrefix(s, "$"):
		currency = "USD"
		s = s[1:]
	case strings.HasPrefix(s, "£"): // U+00A3, 2 bytes in UTF-8
		currency = "GBP"
		s = strings.TrimPrefix(s, "£")
	case strings.HasPrefix(s, "€"): // U+20AC, 3 bytes in UTF-8
		currency = "EUR"
		s = strings.TrimPrefix(s, "€")
	default:
		// No prefix symbol — check for a 3-letter code.
		parts := strings.Fields(s)
		switch {
		case len(parts) == 2 && len(parts[1]) == 3 && isUpperAlpha(parts[1]):
			// "100 BGN" or "-100 BGN" (after stripping the leading minus above)
			currency = parts[1]
			s = parts[0]
		case len(parts) == 2 && len(parts[0]) == 3 && isUpperAlpha(parts[0]):
			// "BGN 100"
			currency = parts[0]
			s = parts[1]
		}
	}

	// Re-apply the negative sign to the numeric part.
	if negative {
		s = "-" + s
	}

	// Strip thousands separators.
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)

	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return AST.Amount{}, fmt.Errorf("invalid amount %q: %v", s, err)
	}

	return AST.Amount{Value: value, Currency: currency}, nil
}

// IsAccountCreditNormal returns true when account is a credit-normal account
// (healthy / "green" when its balance is negative) according to the given
// prefix list.  Comparison is case-insensitive.
//
// Credit-normal accounts in standard double-entry:
//
//	income, liabilities, equity — balances are credits (negative)
//
// The prefix list comes from config.Config.CreditNormalPrefixes so users can
// extend it in ~/.doublebook/config.yaml without rebuilding the binary.
func IsAccountCreditNormal(account string, creditPrefixes []string) bool {
	lower := strings.ToLower(strings.TrimSpace(account))
	for _, prefix := range creditPrefixes {
		p := strings.ToLower(strings.TrimSpace(prefix))
		// Match "income" against "income:salary" OR "income" exactly.
		if lower == p || strings.HasPrefix(lower, p+":") || strings.HasPrefix(lower, p+"/") {
			return true
		}
	}
	return false
}

// isUpperAlpha returns true if every character in s is an ASCII uppercase letter.
func isUpperAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ExpandHome expands a leading "~/" to the current user's home directory.
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

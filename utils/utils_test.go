package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseAmount tests
// ---------------------------------------------------------------------------

func TestParseAmount_USD_DollarSign(t *testing.T) {
	amount, err := parseAmountHelper(t, "$45.32")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 45.32 {
		t.Errorf("Expected value 45.32, got %.2f", amount.Value)
	}
	if amount.Currency != "USD" {
		t.Errorf("Expected currency USD, got %q", amount.Currency)
	}
}

func TestParseAmount_USD_Negative(t *testing.T) {
	amount, err := parseAmountHelper(t, "-$45.32")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != -45.32 {
		t.Errorf("Expected value -45.32, got %.2f", amount.Value)
	}
	if amount.Currency != "USD" {
		t.Errorf("Expected currency USD, got %q", amount.Currency)
	}
}

func TestParseAmount_GBP(t *testing.T) {
	amount, err := parseAmountHelper(t, "£10.50")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 10.50 {
		t.Errorf("Expected value 10.50, got %.2f", amount.Value)
	}
	if amount.Currency != "GBP" {
		t.Errorf("Expected currency GBP, got %q", amount.Currency)
	}
}

func TestParseAmount_EUR(t *testing.T) {
	amount, err := parseAmountHelper(t, "€25.00")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 25.00 {
		t.Errorf("Expected value 25.00, got %.2f", amount.Value)
	}
	if amount.Currency != "EUR" {
		t.Errorf("Expected currency EUR, got %q", amount.Currency)
	}
}

func TestParseAmount_CurrencyCodeSuffix(t *testing.T) {
	amount, err := parseAmountHelper(t, "100 BGN")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 100.0 {
		t.Errorf("Expected value 100.0, got %.2f", amount.Value)
	}
	if amount.Currency != "BGN" {
		t.Errorf("Expected currency BGN, got %q", amount.Currency)
	}
}

func TestParseAmount_CurrencyCodeSuffix_Negative(t *testing.T) {
	amount, err := parseAmountHelper(t, "-100 BGN")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != -100.0 {
		t.Errorf("Expected value -100.0, got %.2f", amount.Value)
	}
	if amount.Currency != "BGN" {
		t.Errorf("Expected currency BGN, got %q", amount.Currency)
	}
}

func TestParseAmount_CurrencyCodePrefix(t *testing.T) {
	amount, err := parseAmountHelper(t, "BGN 100")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 100.0 {
		t.Errorf("Expected value 100.0, got %.2f", amount.Value)
	}
	if amount.Currency != "BGN" {
		t.Errorf("Expected currency BGN, got %q", amount.Currency)
	}
}

func TestParseAmount_NoCurrency_DefaultUSD(t *testing.T) {
	amount, err := parseAmountHelper(t, "45.32")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 45.32 {
		t.Errorf("Expected value 45.32, got %.2f", amount.Value)
	}
	if amount.Currency != "USD" {
		t.Errorf("Expected default currency USD, got %q", amount.Currency)
	}
}

func TestParseAmount_WithCommas(t *testing.T) {
	amount, err := parseAmountHelper(t, "$1,234.56")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 1234.56 {
		t.Errorf("Expected value 1234.56, got %.2f", amount.Value)
	}
}

func TestParseAmount_LargeNumber(t *testing.T) {
	amount, err := parseAmountHelper(t, "$1,234,567.89")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 1234567.89 {
		t.Errorf("Expected value 1234567.89, got %.2f", amount.Value)
	}
}

func TestParseAmount_WithWhitespace(t *testing.T) {
	amount, err := parseAmountHelper(t, "  $45.32  ")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 45.32 {
		t.Errorf("Expected value 45.32, got %.2f", amount.Value)
	}
}

func TestParseAmount_Empty(t *testing.T) {
	_, err := ParseAmount("")
	if err == nil {
		t.Error("ParseAmount should fail for empty string")
	}
}

func TestParseAmount_Invalid(t *testing.T) {
	_, err := ParseAmount("abc")
	if err == nil {
		t.Error("ParseAmount should fail for invalid amount")
	}
}

func TestParseAmount_ZeroValue(t *testing.T) {
	amount, err := parseAmountHelper(t, "$0.00")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 0.0 {
		t.Errorf("Expected value 0.0, got %.2f", amount.Value)
	}
}

func TestParseAmount_WholeNumber(t *testing.T) {
	amount, err := parseAmountHelper(t, "$100")
	if err != nil {
		t.Fatalf("ParseAmount failed: %v", err)
	}

	if amount.Value != 100.0 {
		t.Errorf("Expected value 100.0, got %.2f", amount.Value)
	}
}

// Helper function that fails test on error
func parseAmountHelper(t *testing.T, s string) (amount struct {
	Value    float64
	Currency string
}, err error) {
	result, err := ParseAmount(s)
	return struct {
		Value    float64
		Currency string
	}{result.Value, result.Currency}, err
}

// ---------------------------------------------------------------------------
// IsAccountCreditNormal tests
// ---------------------------------------------------------------------------

func TestIsAccountCreditNormal_Income(t *testing.T) {
	prefixes := []string{"income", "liabilities", "equity"}

	if !IsAccountCreditNormal("income", prefixes) {
		t.Error("Expected 'income' to be credit normal")
	}

	if !IsAccountCreditNormal("income:salary", prefixes) {
		t.Error("Expected 'income:salary' to be credit normal")
	}

	if !IsAccountCreditNormal("income:freelance:consulting", prefixes) {
		t.Error("Expected 'income:freelance:consulting' to be credit normal")
	}
}

func TestIsAccountCreditNormal_Liabilities(t *testing.T) {
	prefixes := []string{"income", "liabilities", "equity"}

	if !IsAccountCreditNormal("liabilities", prefixes) {
		t.Error("Expected 'liabilities' to be credit normal")
	}

	if !IsAccountCreditNormal("liabilities:credit-card", prefixes) {
		t.Error("Expected 'liabilities:credit-card' to be credit normal")
	}
}

func TestIsAccountCreditNormal_Equity(t *testing.T) {
	prefixes := []string{"income", "liabilities", "equity"}

	if !IsAccountCreditNormal("equity", prefixes) {
		t.Error("Expected 'equity' to be credit normal")
	}

	if !IsAccountCreditNormal("equity:opening-balances", prefixes) {
		t.Error("Expected 'equity:opening-balances' to be credit normal")
	}
}

func TestIsAccountCreditNormal_DebitNormal(t *testing.T) {
	prefixes := []string{"income", "liabilities", "equity"}

	if IsAccountCreditNormal("assets", prefixes) {
		t.Error("Expected 'assets' to NOT be credit normal")
	}

	if IsAccountCreditNormal("assets:checking", prefixes) {
		t.Error("Expected 'assets:checking' to NOT be credit normal")
	}

	if IsAccountCreditNormal("expenses", prefixes) {
		t.Error("Expected 'expenses' to NOT be credit normal")
	}

	if IsAccountCreditNormal("expenses:food", prefixes) {
		t.Error("Expected 'expenses:food' to NOT be credit normal")
	}
}

func TestIsAccountCreditNormal_CaseInsensitive(t *testing.T) {
	prefixes := []string{"income", "liabilities", "equity"}

	if !IsAccountCreditNormal("INCOME", prefixes) {
		t.Error("Expected 'INCOME' (uppercase) to be credit normal")
	}

	if !IsAccountCreditNormal("Income:Salary", prefixes) {
		t.Error("Expected 'Income:Salary' (mixed case) to be credit normal")
	}
}

func TestIsAccountCreditNormal_WithSlash(t *testing.T) {
	prefixes := []string{"income", "liabilities", "equity"}

	if !IsAccountCreditNormal("income/salary", prefixes) {
		t.Error("Expected 'income/salary' (slash separator) to be credit normal")
	}
}

func TestIsAccountCreditNormal_EmptyPrefixes(t *testing.T) {
	prefixes := []string{}

	if IsAccountCreditNormal("income", prefixes) {
		t.Error("Expected no account to be credit normal with empty prefix list")
	}
}

func TestIsAccountCreditNormal_CustomPrefixes(t *testing.T) {
	prefixes := []string{"revenue", "passivo"} // Custom/non-English

	if !IsAccountCreditNormal("revenue:sales", prefixes) {
		t.Error("Expected 'revenue:sales' to be credit normal with custom prefixes")
	}

	if !IsAccountCreditNormal("passivo:credit", prefixes) {
		t.Error("Expected 'passivo:credit' to be credit normal with custom prefixes")
	}
}

func TestIsAccountCreditNormal_PrefixNotFullWord(t *testing.T) {
	prefixes := []string{"income"}

	// "incomeX" should NOT match "income" prefix
	// The function checks for exact match OR followed by ":" or "/"
	if IsAccountCreditNormal("incomefoo", prefixes) {
		t.Error("Expected 'incomefoo' to NOT be credit normal (not a valid subaccount)")
	}
}

// ---------------------------------------------------------------------------
// ExpandHome tests
// ---------------------------------------------------------------------------

func TestExpandHome_WithTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	result := ExpandHome("~/test/path")
	expected := filepath.Join(home, "test/path")

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	result := ExpandHome("/absolute/path")

	if result != "/absolute/path" {
		t.Errorf("Expected path unchanged, got %q", result)
	}
}

func TestExpandHome_RelativePath(t *testing.T) {
	result := ExpandHome("relative/path")

	if result != "relative/path" {
		t.Errorf("Expected path unchanged, got %q", result)
	}
}

func TestExpandHome_TildeInMiddle(t *testing.T) {
	// Only leading ~/ should be expanded
	result := ExpandHome("/path/~/test")

	if result != "/path/~/test" {
		t.Errorf("Expected path unchanged, got %q", result)
	}
}

func TestExpandHome_JustTilde(t *testing.T) {
	// Just "~" without slash should not be expanded by this function
	result := ExpandHome("~")

	if result != "~" {
		t.Errorf("Expected '~' unchanged, got %q", result)
	}
}

func TestExpandHome_EmptyString(t *testing.T) {
	result := ExpandHome("")

	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// isUpperAlpha tests (internal function, test via ParseAmount behavior)
// ---------------------------------------------------------------------------

func TestParseAmount_ValidCurrencyCode(t *testing.T) {
	// Test that 3-letter uppercase codes are recognized
	testCases := []struct {
		input    string
		currency string
	}{
		{"100 USD", "USD"},
		{"100 EUR", "EUR"},
		{"100 GBP", "GBP"},
		{"100 JPY", "JPY"},
		{"100 CHF", "CHF"},
	}

	for _, tc := range testCases {
		amount, err := ParseAmount(tc.input)
		if err != nil {
			t.Errorf("ParseAmount(%q) failed: %v", tc.input, err)
			continue
		}
		if amount.Currency != tc.currency {
			t.Errorf("ParseAmount(%q): expected currency %q, got %q", tc.input, tc.currency, amount.Currency)
		}
	}
}

func TestParseAmount_InvalidCurrencyCode(t *testing.T) {
	// Lowercase currency codes are not recognized - the entire string
	// including "usd" is passed to ParseFloat which fails
	_, err := ParseAmount("100 usd")
	if err == nil {
		t.Error("ParseAmount should fail for lowercase currency code '100 usd'")
	}
}

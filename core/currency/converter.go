package currency

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// CachingConverter
// ---------------------------------------------------------------------------

// CachingConverter checks the local cache before hitting the Frankfurter API.
// Newly fetched rates are saved to the cache automatically.
type CachingConverter struct {
	Client RateProvider
	Cache  *RateCache
}

// NewCachingConverter creates a converter backed by a JSON cache file
// at cacheDir/rates.json and the live Frankfurter API.
func NewCachingConverter(cacheDir string) (*CachingConverter, error) {
	cache, err := NewRateCache(filepath.Join(cacheDir, "rates.json"))
	if err != nil {
		return nil, fmt.Errorf("opening rate cache: %w", err)
	}
	return &CachingConverter{
		Client: NewFrankfurterClient(),
		Cache:  cache,
	}, nil
}

// GetRate returns the exchange rate from fromCurrency to toCurrency on date.
//
// Lookup order:
//  1. Same currency → 1.0 immediately
//  2. Local cache hit → return cached value
//  3. Frankfurter API → cache result, return value
//  4. API failure → return 1.0 with a warning (fail-open so import continues)
func (c *CachingConverter) GetRate(date, fromCurrency, toCurrency string) (float64, error) {
	from := strings.ToUpper(fromCurrency)
	to := strings.ToUpper(toCurrency)

	if from == to {
		return 1.0, nil
	}

	// Cache hit.
	if rate, ok := c.Cache.Get(date, from, to); ok {
		return rate, nil
	}

	// Live fetch.
	rate, err := c.Client.GetRate(date, from, to)
	if err != nil {
		// Fail-open: return 1.0 so the rest of the import can continue.
		return 1.0, fmt.Errorf("rate lookup failed (%s→%s on %s): %w — using 1.0 as fallback", from, to, date, err)
	}

	c.Cache.Set(date, from, to, rate)
	// Best-effort cache save; don't fail the whole operation.
	_ = c.Cache.Save()

	return rate, nil
}

// Convert returns the converted amount and the rate that was used.
// Returns (originalAmount, 1.0, err) when conversion fails.
func (c *CachingConverter) Convert(amount float64, date, fromCurrency, toCurrency string) (converted, rate float64, err error) {
	rate, err = c.GetRate(date, fromCurrency, toCurrency)
	if err != nil {
		return amount, 1.0, err
	}
	return roundCurrency(amount * rate), rate, nil
}

// roundCurrency rounds to 2 decimal places.
func roundCurrency(v float64) float64 {
	return math.Round(v*100) / 100
}

// ---------------------------------------------------------------------------
// Journal annotation helper
// ---------------------------------------------------------------------------

// AnnotateAmount builds a comment string suitable for appending to a journal
// posting that records a currency conversion.
//
// Example output:
//
//	; converted: $51.16 @ 0.5116 USD/BGN on 2025-01-15
func AnnotateAmount(
	originalAmount float64, originalCurrency string,
	convertedAmount float64, baseCurrency string,
	date string, rate float64,
) string {
	convStr := formatMoney(convertedAmount, baseCurrency)
	return fmt.Sprintf("; converted: %s @ %.4f %s/%s on %s",
		convStr, rate, baseCurrency, originalCurrency, date)
}

// formatMoney formats an amount with its currency symbol/code.
func formatMoney(v float64, currency string) string {
	neg := ""
	abs := v
	if v < 0 {
		neg = "-"
		abs = -abs
	}
	switch strings.ToUpper(currency) {
	case "USD":
		return fmt.Sprintf("%s$%.2f", neg, abs)
	case "GBP":
		return fmt.Sprintf("%s£%.2f", neg, abs)
	case "EUR":
		return fmt.Sprintf("%s€%.2f", neg, abs)
	default:
		return fmt.Sprintf("%s%.2f %s", neg, abs, currency)
	}
}

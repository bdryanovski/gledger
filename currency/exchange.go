// Package currency provides exchange-rate fetching, caching, and conversion.
// Rates are sourced from api.frankfurter.app (free, ECB data, no API key).
package currency

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// ExchangeRate is a single point-in-time currency rate.
type ExchangeRate struct {
	Date         string
	FromCurrency string
	ToCurrency   string
	Rate         float64
}

// RateProvider is the interface for fetching an exchange rate.
type RateProvider interface {
	GetRate(date, fromCurrency, toCurrency string) (float64, error)
}

// ---------------------------------------------------------------------------
// FrankfurterClient
// ---------------------------------------------------------------------------

// FrankfurterClient fetches historical exchange rates from api.frankfurter.app.
// The API is free, requires no API key, and is backed by ECB reference data.
type FrankfurterClient struct {
	BaseURL    string       // default: "https://api.frankfurter.app"
	HTTPClient *http.Client // default timeout: 10s
}

// NewFrankfurterClient creates a client with sensible defaults.
func NewFrankfurterClient() *FrankfurterClient {
	return &FrankfurterClient{
		BaseURL: "https://api.frankfurter.app",
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetRate returns the rate to convert 1 unit of fromCurrency into toCurrency
// on the given date (YYYY-MM-DD).
//
// If fromCurrency == toCurrency, 1.0 is returned immediately without an HTTP call.
// Weekends and public holidays return the nearest previous business day's rate
// (this is the API's documented behaviour).
func (f *FrankfurterClient) GetRate(date, fromCurrency, toCurrency string) (float64, error) {
	from := strings.ToUpper(fromCurrency)
	to := strings.ToUpper(toCurrency)

	if from == to {
		return 1.0, nil
	}

	url := fmt.Sprintf("%s/%s?base=%s&symbols=%s", f.BaseURL, date, from, to)

	resp, err := f.HTTPClient.Get(url)
	if err != nil {
		return 0, fmt.Errorf("frankfurter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("frankfurter returned HTTP %d for %s→%s on %s",
			resp.StatusCode, from, to, date)
	}

	var result struct {
		Amount float64            `json:"amount"`
		Base   string             `json:"base"`
		Date   string             `json:"date"`
		Rates  map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding frankfurter response: %w", err)
	}

	rate, ok := result.Rates[to]
	if !ok {
		return 0, fmt.Errorf("currency %q not found in Frankfurter response", to)
	}
	return rate, nil
}

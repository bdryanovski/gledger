package currency

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// FrankfurterClient
// ---------------------------------------------------------------------------

func TestGetRateSameCurrency(t *testing.T) {
	// Should return 1.0 without any HTTP call.
	client := NewFrankfurterClient()
	rate, err := client.GetRate("2025-01-15", "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("same-currency rate: got %f, want 1.0", rate)
	}
}

func TestGetRateSameCurrencyLowercase(t *testing.T) {
	client := NewFrankfurterClient()
	rate, err := client.GetRate("2025-01-15", "eur", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("expected 1.0 for same currency, got %f", rate)
	}
}

// Network test — skipped unless ENABLE_NETWORK_TESTS=1 is set.
func TestGetRateLive(t *testing.T) {
	if os.Getenv("ENABLE_NETWORK_TESTS") != "1" {
		t.Skip("skipping live network test (set ENABLE_NETWORK_TESTS=1 to enable)")
	}
	client := NewFrankfurterClient()
	rate, err := client.GetRate("2025-01-15", "USD", "EUR")
	if err != nil {
		t.Fatalf("live fetch error: %v", err)
	}
	if rate <= 0 || rate > 10 {
		t.Errorf("suspicious rate %f for USD→EUR", rate)
	}
	t.Logf("USD→EUR on 2025-01-15: %.4f", rate)
}

// ---------------------------------------------------------------------------
// RateCache
// ---------------------------------------------------------------------------

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rates.json")

	// Write.
	c1, err := NewRateCache(path)
	if err != nil {
		t.Fatalf("NewRateCache: %v", err)
	}
	c1.Set("2025-01-15", "USD", "BGN", 1.9558)
	c1.Set("2025-01-15", "USD", "EUR", 0.9682)
	if err := c1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read back.
	c2, err := NewRateCache(path)
	if err != nil {
		t.Fatalf("NewRateCache (reload): %v", err)
	}

	rate, ok := c2.Get("2025-01-15", "USD", "BGN")
	if !ok {
		t.Fatal("BGN rate not found after reload")
	}
	if rate != 1.9558 {
		t.Errorf("BGN rate: got %f, want 1.9558", rate)
	}

	rate, ok = c2.Get("2025-01-15", "USD", "EUR")
	if !ok {
		t.Fatal("EUR rate not found after reload")
	}
	if rate != 0.9682 {
		t.Errorf("EUR rate: got %f, want 0.9682", rate)
	}
}

func TestCacheMiss(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewRateCache(filepath.Join(dir, "rates.json"))
	_, ok := c.Get("2025-01-15", "USD", "UNKNOWN")
	if ok {
		t.Error("expected cache miss for unknown currency pair")
	}
}

func TestCacheSaveOnlyWhenDirty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rates.json")

	c, _ := NewRateCache(path)
	// Save without setting anything — file should not be created.
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("cache file should NOT be created when nothing is dirty")
	}

	// Set a value and save — file should now exist.
	c.Set("2025-01-15", "USD", "GBP", 0.8085)
	if err := c.Save(); err != nil {
		t.Fatalf("Save after Set: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("cache file should exist after Save: %v", err)
	}
}

func TestCacheSize(t *testing.T) {
	dir := t.TempDir()
	c, _ := NewRateCache(filepath.Join(dir, "rates.json"))
	if c.Size() != 0 {
		t.Errorf("new cache size: got %d, want 0", c.Size())
	}
	c.Set("2025-01-15", "USD", "BGN", 1.9558)
	c.Set("2025-01-15", "USD", "EUR", 0.9682)
	if c.Size() != 2 {
		t.Errorf("size after 2 sets: got %d, want 2", c.Size())
	}
}

// ---------------------------------------------------------------------------
// CachingConverter
// ---------------------------------------------------------------------------

func TestConverterSameCurrency(t *testing.T) {
	dir := t.TempDir()
	conv, err := NewCachingConverter(dir)
	if err != nil {
		t.Fatalf("NewCachingConverter: %v", err)
	}
	rate, err := conv.GetRate("2025-01-15", "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("same-currency rate: got %f, want 1.0", rate)
	}
}

func TestConverterUsesCache(t *testing.T) {
	dir := t.TempDir()
	conv, err := NewCachingConverter(dir)
	if err != nil {
		t.Fatalf("NewCachingConverter: %v", err)
	}

	// Pre-populate cache — no API call made.
	conv.Cache.Set("2025-01-15", "USD", "BGN", 1.9558)

	rate, err := conv.GetRate("2025-01-15", "USD", "BGN")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rate != 1.9558 {
		t.Errorf("rate: got %f, want 1.9558", rate)
	}
}

func TestConvert(t *testing.T) {
	dir := t.TempDir()
	conv, _ := NewCachingConverter(dir)
	conv.Cache.Set("2025-01-15", "USD", "BGN", 1.9558)

	converted, rate, err := conv.Convert(100, "2025-01-15", "USD", "BGN")
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if rate != 1.9558 {
		t.Errorf("rate: got %f", rate)
	}
	// 100 * 1.9558 = 195.58
	if converted != 195.58 {
		t.Errorf("converted: got %f, want 195.58", converted)
	}
}

// ---------------------------------------------------------------------------
// AnnotateAmount
// ---------------------------------------------------------------------------

func TestAnnotateAmount(t *testing.T) {
	s := AnnotateAmount(100, "BGN", 51.16, "USD", "2025-01-15", 0.5116)
	if !strings.Contains(s, "converted") {
		t.Error("annotation missing 'converted'")
	}
	if !strings.Contains(s, "BGN") {
		t.Error("annotation missing original currency")
	}
	if !strings.Contains(s, "$51.16") {
		t.Error("annotation missing converted amount")
	}
	if !strings.Contains(s, "2025-01-15") {
		t.Error("annotation missing date")
	}
}

func TestAnnotateAmountNegative(t *testing.T) {
	s := AnnotateAmount(-50, "EUR", -55.0, "USD", "2025-06-01", 1.10)
	if !strings.Contains(s, "converted") {
		t.Error("missing 'converted'")
	}
}

func TestFormatMoney(t *testing.T) {
	cases := []struct {
		v    float64
		cur  string
		want string
	}{
		{51.16, "USD", "$51.16"},
		{-51.16, "USD", "-$51.16"},
		{10.50, "GBP", "£10.50"},
		{25.00, "EUR", "€25.00"},
		{100.0, "BGN", "100.00 BGN"},
	}
	for _, c := range cases {
		got := formatMoney(c.v, c.cur)
		if got != c.want {
			t.Errorf("formatMoney(%v, %q): got %q, want %q", c.v, c.cur, got, c.want)
		}
	}
}

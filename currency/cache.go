package currency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"doublebook/utils"
)

// ---------------------------------------------------------------------------
// RateCache
// ---------------------------------------------------------------------------

// RateCache is a thread-safe, file-backed cache for exchange rates.
// Keys are "YYYY-MM-DD|FROM|TO" strings; values are float64 rates.
type RateCache struct {
	Path  string
	rates map[string]float64
	mu    sync.RWMutex
	dirty bool
}

// cacheKey builds the map key for a rate entry.
func cacheKey(date, from, to string) string {
	return date + "|" + from + "|" + to
}

// NewRateCache opens (or creates) a cache at path.
// The file is loaded immediately if it exists.
func NewRateCache(path string) (*RateCache, error) {
	c := &RateCache{
		Path:  utils.ExpandHome(path),
		rates: make(map[string]float64),
	}
	// Load if exists; ignore missing-file errors.
	if err := c.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return c, nil
}

// Get looks up a rate. Returns (rate, true) if found, (0, false) otherwise.
func (c *RateCache) Get(date, from, to string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.rates[cacheKey(date, from, to)]
	return v, ok
}

// Set stores a rate and marks the cache as dirty.
func (c *RateCache) Set(date, from, to string, rate float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rates[cacheKey(date, from, to)] = rate
	c.dirty = true
}

// Save flushes the in-memory cache to disk if it has been modified.
func (c *RateCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.dirty {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.Path), 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	data, err := json.MarshalIndent(c.rates, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling rate cache: %w", err)
	}
	if err := os.WriteFile(c.Path, data, 0644); err != nil {
		return fmt.Errorf("writing rate cache: %w", err)
	}
	c.dirty = false
	return nil
}

// Load reads the cache from disk, replacing in-memory entries.
// Returns os.ErrNotExist (unwrapped) when the file does not exist.
func (c *RateCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := os.ReadFile(c.Path)
	if err != nil {
		return err
	}
	var loaded map[string]float64
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing rate cache: %w", err)
	}
	for k, v := range loaded {
		c.rates[k] = v
	}
	c.dirty = false
	return nil
}

// Size returns the number of cached entries.
func (c *RateCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.rates)
}

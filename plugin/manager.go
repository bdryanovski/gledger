package Plugin

import (
	"fmt"
	"strings"

	AST "doublebook/ast"
)

// ---------------------------------------------------------------------------
// PluginReport
// ---------------------------------------------------------------------------

// PluginReport holds one plugin's OnReport output.
type PluginReport struct {
	PluginName string
	Content    string
}

// ---------------------------------------------------------------------------
// PluginManager
// ---------------------------------------------------------------------------

// PluginManager manages the lifecycle of all registered plugins.
type PluginManager struct {
	plugins []Plugin
}

// NewPluginManager creates an empty PluginManager.
func NewPluginManager() *PluginManager {
	return &PluginManager{}
}

// Register initialises p with config and adds it to the manager.
// Returns an error if Initialize fails.
// Passing nil config is equivalent to an empty map.
func (pm *PluginManager) Register(p Plugin, config map[string]interface{}) error {
	if config == nil {
		config = make(map[string]interface{})
	}
	if err := p.Initialize(config); err != nil {
		return fmt.Errorf("plugin %q Initialize error: %w", p.Name(), err)
	}
	pm.plugins = append(pm.plugins, p)
	return nil
}

// Get returns the plugin with the given name (case-insensitive), or (nil, false).
func (pm *PluginManager) Get(name string) (Plugin, bool) {
	lower := strings.ToLower(name)
	for _, p := range pm.plugins {
		if strings.ToLower(p.Name()) == lower {
			return p, true
		}
	}
	return nil, false
}

// List returns the names of all registered plugins.
func (pm *PluginManager) List() []string {
	names := make([]string, len(pm.plugins))
	for i, p := range pm.plugins {
		names[i] = p.Name()
	}
	return names
}

// ShutdownAll calls Shutdown on every plugin.
// All errors are collected and returned as a single combined error.
func (pm *PluginManager) ShutdownAll() error {
	var errs []string
	for _, p := range pm.plugins {
		if err := p.Shutdown(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("plugin shutdown errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Hook executors
// ---------------------------------------------------------------------------

// ExecuteOnParse calls OnParse on every plugin with all loaded transactions.
func (pm *PluginManager) ExecuteOnParse(txn *AST.Transaction) error {
	for _, p := range pm.plugins {
		if err := p.OnParse([]*AST.Transaction{txn}); err != nil {
			return fmt.Errorf("plugin %q OnParse: %w", p.Name(), err)
		}
	}
	return nil
}

// ExecuteOnAdd calls OnAdd on every plugin for a newly added transaction.
func (pm *PluginManager) ExecuteOnAdd(txn *AST.Transaction) error {
	for _, p := range pm.plugins {
		if err := p.OnAdd(txn); err != nil {
			return fmt.Errorf("plugin %q OnAdd: %w", p.Name(), err)
		}
	}
	return nil
}

// ExecuteOnFilter pipes transactions through each plugin's OnFilter in order.
// Each plugin receives the output of the previous one.
func (pm *PluginManager) ExecuteOnFilter(txns []*AST.Transaction) []*AST.Transaction {
	result := txns
	for _, p := range pm.plugins {
		result = p.OnFilter(result)
	}
	return result
}

// ExecuteOnReport collects non-empty OnReport strings from all plugins.
func (pm *PluginManager) ExecuteOnReport(txns []*AST.Transaction) []string {
	var out []string
	for _, p := range pm.plugins {
		if s := p.OnReport(txns); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ExecuteOnImport calls OnImport on every plugin.
func (pm *PluginManager) ExecuteOnImport(rows []ImportRow, mapName string) error {
	for _, p := range pm.plugins {
		if err := p.OnImport(rows, mapName); err != nil {
			return fmt.Errorf("plugin %q OnImport: %w", p.Name(), err)
		}
	}
	return nil
}

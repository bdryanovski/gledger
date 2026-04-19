// Package Plugin defines the DoubleBook plugin interface and the DefaultPlugin
// embed helper that provides no-op implementations for all optional hooks.
package plugin

import (
	"doublebook/core/ast"
)

// ---------------------------------------------------------------------------
// Plugin interface
// ---------------------------------------------------------------------------

// Plugin is the interface every DoubleBook plugin must implement.
// Embed DefaultPlugin to get no-op defaults for all hook methods.
type Plugin interface {
	// Identity
	Name() string
	Version() string
	Description() string

	// Lifecycle
	Initialize(config map[string]interface{}) error
	Shutdown() error

	// Hooks (embed DefaultPlugin for no-op defaults)
	OnParse(transactions []*ast.Transaction) error
	OnAdd(transaction *ast.Transaction) error
	OnFilter(transactions []*ast.Transaction) []*ast.Transaction
	OnReport(transactions []*ast.Transaction) string
	OnImport(rows []ImportRow, importMapName string) error
}

// ImportRow is a single CSV row passed to OnImport before it is written.
type ImportRow struct {
	Date        string
	Description string
	Amount      float64
	Currency    string
	Account     string
	Reference   string
}

// ---------------------------------------------------------------------------
// DefaultPlugin — embed for no-op hook implementations
// ---------------------------------------------------------------------------

// DefaultPlugin provides no-op implementations of every optional hook.
// Embed this struct in your plugin so you only need to override the hooks
// that are relevant to you.
//
//	type MyPlugin struct {
//	    plugin.DefaultPlugin  // inherits all no-op hooks
//	}
type DefaultPlugin struct{}

func (d *DefaultPlugin) OnParse(_ []*ast.Transaction) error { return nil }
func (d *DefaultPlugin) OnAdd(_ *ast.Transaction) error     { return nil }
func (d *DefaultPlugin) OnFilter(txns []*ast.Transaction) []*ast.Transaction {
	return txns
}
func (d *DefaultPlugin) OnReport(_ []*ast.Transaction) string      { return "" }
func (d *DefaultPlugin) OnImport(_ []ImportRow, _ string) error    { return nil }
func (d *DefaultPlugin) Shutdown() error                           { return nil }
func (d *DefaultPlugin) Version() string                           { return "0.0.0" }
func (d *DefaultPlugin) Description() string                       { return "" }
func (d *DefaultPlugin) Initialize(_ map[string]interface{}) error { return nil }

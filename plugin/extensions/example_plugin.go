// Package extensions contains built-in DoubleBook plugins.
package extensions

import (
	"fmt"
	"strings"

	AST "doublebook/ast"
	Plugin "doublebook/plugin"
)

// ---------------------------------------------------------------------------
// ExamplePlugin
// ---------------------------------------------------------------------------

// ExamplePlugin is the default plugin included with every DoubleBook install.
// It demonstrates the plugin interface and produces a simple report.
// Use it as a starting point for writing your own plugin.
type ExamplePlugin struct {
	Plugin.DefaultPlugin // inherit all no-op hooks
	transactionCount     int
}

func NewExamplePlugin() *ExamplePlugin { return &ExamplePlugin{} }

func (p *ExamplePlugin) Name() string        { return "example" }
func (p *ExamplePlugin) Version() string     { return "1.0.0" }
func (p *ExamplePlugin) Description() string { return "Example plugin — demonstrates the plugin API" }

func (p *ExamplePlugin) Initialize(config map[string]interface{}) error {
	p.transactionCount = 0
	return nil
}

func (p *ExamplePlugin) OnParse(transactions []*AST.Transaction) error {
	p.transactionCount += len(transactions)
	return nil
}

func (p *ExamplePlugin) OnReport(transactions []*AST.Transaction) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Example Plugin — %d transactions loaded\n", len(transactions)))
	return b.String()
}

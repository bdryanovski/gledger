// Package extensions contains built-in plugins for DoubleBook.
// TemplatePlugin is kept for backward compatibility.
package extensions

import (
	"doublebook/core/ast"
	"doublebook/plugin"
)

// TemplatePlugin is the original placeholder plugin.
// It now embeds DefaultPlugin to satisfy the full Plugin interface.
type TemplatePlugin struct {
	plugin.DefaultPlugin
}

func NewTemplatePlugin() *TemplatePlugin {
	return &TemplatePlugin{}
}

func (p *TemplatePlugin) Name() string        { return "template" }
func (p *TemplatePlugin) Version() string     { return "1.0.0" }
func (p *TemplatePlugin) Description() string { return "Template plugin (placeholder)" }

func (p *TemplatePlugin) OnReport(transactions []*ast.Transaction) string {
	return "" // no-op by default
}

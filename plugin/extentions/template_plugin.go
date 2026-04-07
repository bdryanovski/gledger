// Package TemplatePlugin is kept for backward compatibility.
// New plugins should be placed in plugin/extensions/.
package TemplatePlugin

import (
	AST "doublebook/ast"
	Plugin "doublebook/plugin"
)

// TemplatePlugin is the original placeholder plugin.
// It now embeds DefaultPlugin to satisfy the full Plugin interface.
type TemplatePlugin struct {
	Plugin.DefaultPlugin
	data map[string]interface{}
}

func NewTemplatePlugin() *TemplatePlugin {
	return &TemplatePlugin{data: make(map[string]interface{})}
}

func (p *TemplatePlugin) Name() string        { return "template" }
func (p *TemplatePlugin) Version() string     { return "1.0.0" }
func (p *TemplatePlugin) Description() string { return "Template plugin (placeholder)" }

func (p *TemplatePlugin) OnReport(transactions []*AST.Transaction) string {
	return "" // no-op by default
}

package TemplatePlugin

import (
	AST "gledger/ast"
	"strings"
)

type TemplatePlugin struct {
	data map[string]interface{}
}

func NewTemplatePlugin() *TemplatePlugin {
	return &TemplatePlugin{data: make(map[string]interface{})}
}

func (p *TemplatePlugin) Name() string {
	return "template"
}

func (p *TemplatePlugin) OnParse(transactions *AST.Transaction) error {
	return nil
}

func (p *TemplatePlugin) OnFilter(transactions []*AST.Transaction) []*AST.Transaction {
	return transactions
}

func (p *TemplatePlugin) OnAdd(transactions *AST.Transaction) error {
	return nil
}

func (p *TemplatePlugin) OnReport(transactions []*AST.Transaction) string {
	var report strings.Builder

	report.WriteString("Template Plugin Report\n")
	report.WriteString("====================\n")
	report.WriteString("This is a template plugin. You can customize this report to display any information you want.\n")

	return report.String()
}

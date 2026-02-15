package Plugin

import (
	"fmt"
	"gledger/ast"
)

type Plugin interface {
	Name() string
	OnParse(transaction *AST.Transaction) error
	OnAdd(transaction *AST.Transaction) error
	OnFilter(transaction []*AST.Transaction) []*AST.Transaction
	OnReport(transaction []*AST.Transaction) string
}

type PluginManager struct {
	plugins []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: []Plugin{},
	}
}

func (pm *PluginManager) Register(plugin Plugin) {
	pm.plugins = append(pm.plugins, plugin)
}

func (pm *PluginManager) ExecuteOnParse(transaction *AST.Transaction) error {
	for _, plugin := range pm.plugins {
		if err := plugin.OnParse(transaction); err != nil {
			return fmt.Errorf("Plugin %s OnParse error: %v", plugin.Name(), err)
		}
	}
	return nil
}

func (pm *PluginManager) ExecuteOnAdd(transaction *AST.Transaction) error {
	for _, plugin := range pm.plugins {
		if err := plugin.OnAdd(transaction); err != nil {
			return fmt.Errorf("Plugin %s OnAdd error: %v", plugin.Name(), err)
		}
	}
	return nil
}

func (pm *PluginManager) ExecuteOnFilter(transactions []*AST.Transaction) []*AST.Transaction {
	result := transactions
	for _, plugin := range pm.plugins {
		result = plugin.OnFilter(result)
	}
	return result
}

func (pm *PluginManager) ExecuteOnReport(transactions []*AST.Transaction) []string {
	var reports []string
	for _, plugin := range pm.plugins {
		report := plugin.OnReport(transactions)
		if report != "" {
			reports = append(reports, report)
		}
	}
	return reports
}

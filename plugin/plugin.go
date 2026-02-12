package plugin

import (
	"fmt"
	"gledger/parser"
)

type Plugin interface {
	Name() string
	OnParse(transaction *Parser.Transaction) error
	OnAdd(transaction *Parser.Transaction) error
	OnFilter(transaction []*Parser.Transaction) []*Parser.Transaction
	OnReport(transaction []*Parser.Transaction) string
}

type PluginManager struct {
	plugins []Plugin
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins: []Plugin{},
	}
}

func (pluginmanager *PluginManager) Register(plugin Plugin) {
	pluginmanager.plugins = append(pluginmanager.plugins, plugin)
}

func (pluginmanager *PluginManager) ExecuteOnParse(transaction *Parser.Transaction) error {
	for _, plugin := range pluginmanager.plugins {
		if err := plugin.OnParse(transaction); err != nil {
			return fmt.Errorf("Plugin %s OnParse error: %v", plugin.Name(), err)
		}
	}
	return nil
}

func (pluginmanager *PluginManager) ExecuteOnAdd(transaction *Parser.Transaction) error {
	for _, plugin := range pluginmanager.plugins {
		if err := plugin.OnAdd(transaction); err != nil {
			return fmt.Errorf("Plugin %s OnAdd error: %v", plugin.Name(), err)
		}
	}
	return nil
}

func (pluginmanager *PluginManager) ExecuteOnFilter(transactions []*Parser.Transaction) []*Parser.Transaction {
	result := transactions
	for _, plugin := range pluginmanager.plugins {
		result = plugin.OnFilter(result)
	}
	return result
}

func (pluginmanager *PluginManager) ExecuteOnReport(transactions []*Parser.Transaction) []string {
	var reports []string
	for _, plugin := range pluginmanager.plugins {
		report := plugin.OnReport(transactions)
		if report != "" {
			reports = append(reports, report)
		}
	}
	return reports
}

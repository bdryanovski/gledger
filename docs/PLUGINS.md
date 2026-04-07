# DoubleBook Plugin System

DoubleBook can be extended with plugins that hook into the transaction lifecycle,
filter data, generate custom reports, and add new CLI subcommands.

---

## Built-in Plugins

| Name | Command | Description |
|------|---------|-------------|
| `sql-export` | `doublebook plugin run sql-export` | Export journal to a queryable SQLite file |
| `recurring` | `doublebook plugin run recurring` | Track and report on recurring payment schedules |
| `example` | — | Starter template demonstrating the plugin API |

```bash
# List all registered plugins
doublebook plugin list

# Run a plugin's default command
doublebook plugin run sql-export --output ~/finance.db
doublebook plugin run recurring status
doublebook plugin run recurring list
doublebook plugin run recurring generate
```

---

## Plugin Interface

Every plugin must implement the `Plugin` interface defined in `plugin/plugin.go`:

```go
type Plugin interface {
    // Identity
    Name() string        // unique identifier, e.g. "my-plugin"
    Version() string     // semantic version string, e.g. "1.0.0"
    Description() string // one-line human-readable description

    // Lifecycle — called by DoubleBook automatically
    Initialize(config map[string]interface{}) error
    Shutdown() error

    // Hooks — embed DefaultPlugin for no-op defaults
    OnParse(transactions []*ast.Transaction) error
    OnAdd(transaction *ast.Transaction) error
    OnFilter(transactions []*ast.Transaction) []*ast.Transaction
    OnReport(transactions []*ast.Transaction) string
    OnImport(rows []ImportRow, importMapName string) error
}
```

### Hook Reference

| Hook | When called | Use for |
|------|-------------|---------|
| `OnParse` | After journal files are loaded | Index data, validate entries, enrich transactions |
| `OnAdd` | After a new transaction is written | Sync to external systems, send notifications |
| `OnFilter` | When transactions are filtered for a query | Add additional filtering rules |
| `OnReport` | When generating reports | Append custom sections to any report |
| `OnImport` | After CSV rows are parsed but before writing | Auto-categorise, tag, or transform imported rows |

### Lifecycle

```
Initialize → OnParse → [OnAdd / OnFilter / OnReport / OnImport] → Shutdown
```

- **Initialize** is called once when the plugin is registered. Receive your configuration here.
- **Shutdown** is called when DoubleBook exits cleanly. Flush caches, close connections.

---

## Quick Start: Writing a Plugin

### 1. Create your plugin package

```
plugin/extensions/myplugin/myplugin.go
```

### 2. Embed `DefaultPlugin` for no-op defaults

```go
package myplugin

import (
    "fmt"
    "strings"
    AST "doublebook/ast"
    Plugin "doublebook/plugin"
)

// TaxSummaryPlugin estimates tax on income accounts.
type TaxSummaryPlugin struct {
    Plugin.DefaultPlugin  // provides no-op defaults for all hooks
    TaxRate float64
}

func (p *TaxSummaryPlugin) Name() string        { return "tax-summary" }
func (p *TaxSummaryPlugin) Version() string     { return "1.0.0" }
func (p *TaxSummaryPlugin) Description() string {
    return "Calculates estimated tax on income accounts"
}

// Initialize reads plugin-specific config.
// Called once at startup with values from config.yaml.
func (p *TaxSummaryPlugin) Initialize(config map[string]interface{}) error {
    if rate, ok := config["tax_rate"].(float64); ok {
        p.TaxRate = rate
    } else {
        p.TaxRate = 0.20 // default 20%
    }
    return nil
}

// OnReport appends estimated tax info to any report.
func (p *TaxSummaryPlugin) OnReport(transactions []*AST.Transaction) string {
    var totalIncome float64
    for _, txn := range transactions {
        for _, posting := range txn.Postings {
            if strings.HasPrefix(posting.Account, "income:") {
                totalIncome += posting.Amount.Value
            }
        }
    }
    totalIncome = -totalIncome // income is negative in double-entry
    estimatedTax := totalIncome * p.TaxRate

    return fmt.Sprintf(
        "Tax Summary\n  Total Income:    $%.2f\n  Tax (%d%%):       $%.2f\n",
        totalIncome, int(p.TaxRate*100), estimatedTax,
    )
}
```

### 3. Register your plugin

Edit `plugin/manager.go` and add your plugin to the `builtinPlugins` map, then
register it in `interpreter/interpreter.go`:

```go
// In interpreter/interpreter.go — NewInterpreter():
import MyPlugin "doublebook/plugin/extensions/myplugin"

// Inside NewInterpreter():
i.plugins.Register(&MyPlugin.TaxSummaryPlugin{}, map[string]interface{}{
    "tax_rate": 0.25,
})
```

Or load it from `~/.doublebook/config.yaml`:

```yaml
plugins:
  - tax-summary

plugin_config:
  tax-summary:
    tax_rate: 0.25
```

### 4. Add a CLI subcommand (optional)

If your plugin needs its own CLI interface, add a `RunCommand` method and wire
it up in `cli/commands/plugin_command.go`:

```go
// In RunCommand method of your plugin:
func (p *TaxSummaryPlugin) RunCommand(txns []*AST.Transaction, args []string) error {
    report := p.OnReport(txns)
    fmt.Print(report)
    return nil
}

// In plugin_command.go runPlugin():
case "tax-summary":
    p := &myplugin.TaxSummaryPlugin{}
    _ = p.Initialize(nil)
    return p.RunCommand(txns, args)
```

Then run it:

```bash
doublebook plugin run tax-summary
```

---

## `DefaultPlugin` Reference

Embed `DefaultPlugin` to avoid implementing every hook:

```go
type DefaultPlugin struct{}

func (d *DefaultPlugin) OnParse(_ []*ast.Transaction) error                    { return nil }
func (d *DefaultPlugin) OnAdd(_ *ast.Transaction) error                        { return nil }
func (d *DefaultPlugin) OnFilter(txns []*ast.Transaction) []*ast.Transaction   { return txns }
func (d *DefaultPlugin) OnReport(_ []*ast.Transaction) string                  { return "" }
func (d *DefaultPlugin) OnImport(_ []ImportRow, _ string) error                { return nil }
func (d *DefaultPlugin) Shutdown() error                                        { return nil }
func (d *DefaultPlugin) Version() string                                        { return "0.0.0" }
func (d *DefaultPlugin) Description() string                                    { return "" }
func (d *DefaultPlugin) Initialize(_ map[string]interface{}) error              { return nil }
```

Override only the hooks you need. New hooks added in future DoubleBook versions
will be added to `DefaultPlugin` first so your plugin won't break.

---

## Configuration

Configure plugins in `~/.doublebook/config.yaml`:

```yaml
# List plugins to auto-load on startup
plugins:
  - sql-export
  - recurring
  - tax-summary          # your custom plugin

# Per-plugin configuration
plugin_config:
  sql-export:
    output_path: ~/reports/finance.db
  recurring:
    config_path: ~/.doublebook/recurring.json
  tax-summary:
    tax_rate: 0.25
```

---

## recurring.json Format

The `recurring` plugin reads `~/.doublebook/recurring.json`:

```json
{
  "schedules": [
    {
      "id": "rent",
      "description": "Monthly Rent",
      "amount": 1200.00,
      "currency": "USD",
      "debit_account": "expenses:housing:rent",
      "credit_account": "assets:checking",
      "frequency": "monthly",
      "day_of_month": 1,
      "start_date": "2025-01-01",
      "end_date": null,
      "tags": { "category": "housing" },
      "active": true
    },
    {
      "id": "netflix",
      "description": "Netflix",
      "amount": 15.99,
      "currency": "USD",
      "debit_account": "expenses:entertainment:streaming",
      "credit_account": "assets:checking",
      "frequency": "monthly",
      "day_of_month": 15,
      "start_date": "2025-01-15",
      "active": true
    }
  ]
}
```

**Frequency values:** `daily` | `weekly` | `monthly` | `yearly` | `custom`

| Frequency | Extra fields |
|-----------|-------------|
| `weekly` | `day_of_week` (0=Sun … 6=Sat) |
| `monthly` | `day_of_month` (1–31; clamped to last day of month) |
| `yearly` | `month` + `day_of_month` |
| `custom` | `interval_days` |

**Commands:**

```bash
doublebook plugin run recurring status    # Show all schedules with next due date
doublebook plugin run recurring list      # List all schedules
doublebook plugin run recurring generate  # Print unmatched transactions (ready to append)
```

---

## Plugin Limitations

- **No hot-reload**: DoubleBook must be restarted after adding or modifying a plugin.
- **Same process**: Plugins run in the same process as DoubleBook. A crashing plugin
  will crash the whole application. Wrap risky code in `recover()`.
- **Built-in only**: External `.so` plugin files are not yet supported. All plugins
  must be compiled into the binary. Fork the repository and add your plugin to
  `plugin/extensions/` to include it in your personal build.

---

## Example Use Cases

| Goal | Hooks to implement | Notes |
|------|--------------------|-------|
| Auto-tag transactions by description | `OnImport`, `OnAdd` | Match description patterns, set tags |
| Enforce account naming rules | `OnAdd` | Return error if account violates convention |
| Sync to external budget app | `OnAdd` | POST to API when transaction is added |
| Custom PDF report | `OnReport` | Return formatted string; pipe to `enscript` |
| Export to CSV | `OnParse` | Write all transactions to a CSV on load |
| Budget alerts | `OnReport` | Compare spending to budget limits, warn |
| Anomaly detection | `OnParse` | Flag unusually large transactions |

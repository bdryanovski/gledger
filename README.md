# DoubleBook

A plain-text double-entry accounting application compatible with [hledger](https://hledger.org/) journal format.

## Features

- **Plain-text accounting** - Human-readable journal files
- **Double-entry bookkeeping** - Every transaction balances
- **Multiple interfaces**:
  - Terminal UI (TUI) with interactive navigation
  - Command-line interface (CLI) for scripting
  - REST API server
  - Web UI (React-based dashboard)
- **Financial Query Language (FQL)** - SQL-like queries on your financial data
- **CSV Import** - Import bank statements with customizable mappings
- **Plugin System** - Extend functionality with custom plugins
- **Multi-currency support** - Track multiple currencies with exchange rates

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/yourusername/gledger.git
cd gledger

# Build the binary
make build

# Or build with web UI included
make build/full
```

### Requirements

- Go 1.21 or later
- Node.js 18+ (for web UI build only)

## Quick Start

### 1. Create a Journal File

Create `~/.doublebook/data.journal`:

```
2025-01-15 Grocery Store
    expenses:groceries                $45.32
    assets:checking                  -$45.32

2025-01-16 Salary Payment
    assets:checking                $2000.00
    income:salary                 -$2000.00

2025-01-18 Electric Bill
    expenses:utilities                $89.50
    assets:checking                  -$89.50
```

### 2. View Your Balances

```bash
# Show account balances
doublebook balance

# Show as a tree structure
doublebook balance --tree
```

### 3. View Transaction Register

```bash
# Show all transactions with running totals
doublebook register

# Filter by account
doublebook register --account expenses

# Limit number of results
doublebook register --limit 10
```

## Usage

### Terminal UI (TUI)

Launch the interactive terminal interface:

```bash
doublebook
```

### CLI Commands

```
doublebook [global flags] <command> [command flags]

Global flags:
  --journal NAME    Use journal stem NAME (default: "data")
  --begin DATE      Filter: only include transactions on/after DATE (YYYY-MM-DD)
  --end DATE        Filter: only include transactions on/before DATE
  --verbose         Enable verbose output

Commands:
  balance, bal           Show account balances
  register, reg, r       Show a transaction register with running total
  list, ls               Alias for register
  is, income-statement   Show income statement (revenues vs expenses)
  fql, query             Financial Query Language (REPL or --query "...")
  insert, add            Interactive form to add a new transaction
  import                 Import transactions from a CSV file
  plugin                 Manage and run plugins
  api                    Start REST API server
  web                    Start web UI server
  help                   Show help message
  version                Show version information
```

### Examples

```bash
# View balance for a specific date range
doublebook --begin 2025-01-01 --end 2025-01-31 balance

# Use a different journal
doublebook --journal personal register

# Add a transaction interactively
doublebook add

# Add a transaction with arguments
doublebook add --date 2025-01-15 --description "Coffee" --amount 5.00 \
               --from assets:checking --to expenses:dining

# Import from CSV
doublebook import --file bank-statement.csv --map bank-import.yaml

# Start the web interface
doublebook web --port 8080

# Start the API server
doublebook api --port 5555

# Run FQL query
doublebook fql --query "SELECT account, sum(amount) FROM transactions GROUP BY account"
```

## Journal File Format

DoubleBook uses a plain-text format compatible with hledger:

```
; Comments start with semicolon
2025-01-15 Transaction Description
    account:subaccount           $100.00
    another:account             -$100.00

; Status markers: * = cleared, ! = pending
2025-01-16 * Cleared transaction
    expenses:food                 $25.00
    assets:cash

; The last posting amount can be omitted (will be inferred)
2025-01-17 Implied amount
    expenses:transport            $50.00
    assets:checking
```

## Configuration

Configuration file: `~/.doublebook/config.yaml`

```yaml
# Default journal name (without .journal extension)
journal_name: data

# Directory for journal files
data_dir: ~/.doublebook

# Currency symbol
default_currency: $

# Plugins to load
plugins:
  - sql-export
  - recurring
```

## Financial Query Language (FQL)

Query your financial data using SQL-like syntax:

```bash
# Start interactive REPL
doublebook fql

# Run a single query
doublebook fql --query "SELECT * FROM transactions WHERE account LIKE 'expenses:%'"
```

Example queries:

```sql
-- Monthly expense totals
SELECT month, SUM(amount) AS total
FROM spending
WHERE account LIKE 'expenses:%'
GROUP BY month
ORDER BY month

-- Top spending categories
SELECT account, SUM(amount) AS total
FROM transactions
WHERE account LIKE 'expenses:%'
GROUP BY account
ORDER BY total DESC
LIMIT 10

-- Recent transactions
SELECT date, description, amount
FROM transactions
ORDER BY date DESC
LIMIT 20
```

See [docs/FQL.md](docs/FQL.md) for complete FQL documentation.

## Plugins

DoubleBook supports a plugin system for extending functionality:

```bash
# List installed plugins
doublebook plugin list

# Run a plugin command
doublebook plugin run sql-export --output ~/finance.db
doublebook plugin run recurring status
```

### Built-in Plugins

| Plugin | Description |
|--------|-------------|
| `sql-export` | Export journal to a queryable SQLite file |
| `recurring` | Track and report on recurring payment schedules |

See [docs/PLUGINS.md](docs/PLUGINS.md) for plugin development documentation.

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test/cover

# Format code
make tidy

# Run quality checks
make audit

# Build web UI
make web-build

# Run web UI dev server (hot reload)
make dev-web
```

## Project Structure

```
gledger/
├── ast/            # Abstract Syntax Tree types
├── lexer/          # Tokenizer for journal files
├── parser/         # Parser for journal format
├── interpreter/    # Core business logic engine
├── journal/        # Journal file loading/writing
├── cli/            # Command-line interface
│   └── commands/   # CLI command implementations
├── ui/             # Terminal UI (Bubbletea)
├── fql/            # Financial Query Language
├── db/             # SQLite cache/backend
├── config/         # Configuration management
├── currency/       # Currency conversion
├── importer/       # CSV import functionality
├── plugin/         # Plugin system
├── api/            # REST API server
├── web/            # Web server with embedded frontend
├── web-ui/         # React frontend source
├── utils/          # Shared utilities
├── sample/         # Sample CSV and import mappings
├── example/        # Example journal files
└── docs/           # Documentation
```

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]

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

# Import from CSV (legacy importmap)
doublebook import --file bank-statement.csv --map bank-import.json

# Import from CSV (new rules engine)
doublebook import --rules my-bank bank-statement.csv

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

## Import Rules System

The import system converts CSV/Excel bank statements into double-entry journal transactions. There are two import methods:

1. **Rules Engine** (YAML-based) - Recommended, more powerful and flexible
2. **Legacy ImportMap** (JSON-based) - Simpler, still supported

### Quick Start: Creating Rules

```bash
# Interactive mapper - creates rules file from CSV
doublebook map bank-statement.csv

# List existing rules
doublebook map --list

# Show rule details
doublebook map --show my-bank

# Import with rules
doublebook import --rules my-bank bank-statement.csv

# Dry run (preview without writing)
doublebook import --rules my-bank --dry-run bank-statement.csv
```

### Rules File Location

Rules are stored in `~/.doublebook/rules/` with extensions:
- `.yaml`, `.yml`
- `.rules.yaml`, `.rules.yml`

### Rule File Structure

```yaml
# Identity
name: my-bank                              # Required: human-readable label
description: My Bank checking account      # Optional: purpose description
version: "1"                               # Optional: ruleset version

# File Format Settings
format:
  type: csv                                # "csv" or "excel"
  delimiter: ","                           # CSV delimiter: ",", ";", "\t", "|"
  encoding: utf-8                          # Encoding (see supported list below)
  skip_lines: 1                            # Header lines to skip
  sheet_name: ""                           # For Excel: sheet name
  sheet_index: 0                           # For Excel: sheet index (0-based)

# Account Settings
source_account: assets:checking:mybank     # Required: your bank account
default_debit_account: expenses:unknown    # Default for expenses
default_credit_account: income:unknown     # Default for income
currency: USD                              # Default currency

# Column Definitions (auto-discovered from file preview)
columns:
  - index: 0
    name: Date
    samples: ["01/15/2025", "01/16/2025"]
  - index: 1
    name: Description
    samples: ["GROCERY STORE", "COFFEE SHOP"]

# Field Mappings (Required: must include "date" and amount field)
mappings:
  - field: date
    direct:
      column: 0
  # ... more mappings

# Post-Import Categorization Rules
categories:
  - name: groceries
    match:
      description_contains: ["grocery", "supermarket"]
    set_account: expenses:food:groceries
```

### Supported Encodings

- `utf-8` (default)
- `windows-1251` / `cp1251` / `win1251` (Cyrillic)
- `windows-1252` / `cp1252` (Western European)
- `iso-8859-1` / `latin-1`

### Field Mapping Types

Every mapping must specify a `field` and exactly ONE mapping type.

#### Target Fields

| Field | Description | Required |
|-------|-------------|----------|
| `date` | Transaction date | Yes |
| `amount` | Single amount column (positive=credit, negative=debit) | Yes (or debit/credit) |
| `debit_amount` | Expense/outflow column | Yes (or amount) |
| `credit_amount` | Income/inflow column | Yes (or amount) |
| `description` | Transaction description | No |
| `reference` | Transaction ID/reference | No |
| `currency` | Transaction currency | No |
| `debit_account` | Override expense account | No |
| `credit_account` | Override income account | No |
| `tag:<name>` | Custom tag (e.g., `tag:merchant`) | No |

#### 1. Direct Mapping

Maps a single column directly:

```yaml
- field: date
  direct:
    column: 0              # 0-based column index
```

#### 2. Combine Mapping

Combines multiple columns:

```yaml
- field: description
  combine:
    columns: [1, 2, 3]
    format: "{0} - {1} ({2})"  # Template with placeholders
    # OR
    separator: " - "           # Simple join
    trim: true                 # Trim whitespace from each value
```

#### 3. Transform Mapping

Applies a transform function:

```yaml
- field: date
  transform:
    column: 0
    function: parse_date
    args:
      input_format: "02/01/2006"    # DD/MM/YYYY
      output_format: "2006-01-02"   # YYYY-MM-DD (default)

- field: amount
  transform:
    column: 3
    function: parse_number
    args:
      decimal: ","
      thousands: "."
```

#### 4. Lookup Mapping

Maps values via lookup table:

```yaml
- field: debit_account
  lookup:
    column: 4
    table:
      FOOD: expenses:food
      TRANSPORT: expenses:transport
      UTILITIES: expenses:utilities
    default: expenses:unknown
    case_sensitive: false
```

#### 5. Constant Mapping

Sets a fixed value:

```yaml
- field: "tag:source"
  constant:
    value: "bank-import"
```

#### 6. Conditional Mapping

Branching logic based on conditions:

```yaml
- field: debit_account
  condition:
    conditions:
      - when:
          column: 2
          contains: "amazon"        # Case-insensitive substring
        mapping:
          constant:
            value: expenses:shopping
      - when:
          column: 3
          greater_than: 500.0       # Numeric comparison
        mapping:
          constant:
            value: expenses:large-purchases
    default:
      constant:
        value: expenses:unknown
```

**Condition operators:**
- `contains` - Case-insensitive substring match
- `equals` - Case-insensitive exact match
- `regex` - Regular expression match
- `greater_than` - Numeric > comparison
- `less_than` - Numeric < comparison

### Transform Functions Reference

#### String Transforms

| Function | Description | Arguments |
|----------|-------------|-----------|
| `uppercase` | Converts to UPPERCASE | none |
| `lowercase` | Converts to lowercase | none |
| `titlecase` | Converts to Title Case | none |
| `trim` | Removes whitespace | `chars` - specific chars to trim |
| `replace` | Replaces all occurrences | `old`, `new` |
| `regex_extract` | Extracts text matching pattern | `pattern` (required), `default` |
| `prefix` | Adds prefix to non-empty values | `prefix` |
| `suffix` | Adds suffix to non-empty values | `suffix` |
| `truncate` | Limits string length | `length` (default: 50), `ellipsis` (default: "...") |
| `clean` | Removes extra whitespace/non-printable chars | none |

#### Combine Transforms

| Function | Description | Arguments |
|----------|-------------|-----------|
| `concat` | Joins all input values together | none |
| `join` | Joins values with separator, filtering empty | `separator` (default: " ") |
| `format` | Formats using `{0}`, `{1}` placeholders | `template` (required) |

#### Date Transforms

| Function | Description | Arguments |
|----------|-------------|-----------|
| `parse_date` | Parses date to standard format | `input_format`, `output_format` |
| `format_date` | Reformats an ISO date | `format` |

**Go date format reference (use these patterns):**
- `2006` = 4-digit year
- `01` = 2-digit month (zero-padded)
- `02` = 2-digit day (zero-padded)
- `15` = 24-hour hour
- `04` = minute
- `05` = second

**Common formats:**
| Format | Pattern |
|--------|---------|
| DD/MM/YYYY | `02/01/2006` |
| MM/DD/YYYY | `01/02/2006` |
| YYYY-MM-DD | `2006-01-02` |
| DD.MM.YYYY | `02.01.2006` |
| DD-MM-YYYY | `02-01-2006` |

**Auto-detected input formats:**
- `2006-01-02`, `02/01/2006`, `01/02/2006`, `2006/01/02`
- `02-01-2006`, `01-02-2006`, `02.01.2006`, `2006.01.02`
- `Jan 2, 2006`, `January 2, 2006`, `2 Jan 2006`

#### Number Transforms

| Function | Description | Arguments |
|----------|-------------|-----------|
| `parse_number` | Parses numbers (handles currency, thousands separators) | `decimal`, `thousands` |
| `abs` | Returns absolute value | none |
| `negate` | Negates the number | none |

**`parse_number` auto-handling:**
- Strips currency symbols: `$`, `€`, `£`, `лв`, `BGN`
- Auto-detects decimal separator
- Handles parentheses for negatives: `(100.00)` → `-100.00`

#### Conditional Transforms

| Function | Description | Arguments |
|----------|-------------|-----------|
| `default` | Returns default if input empty | `value` |
| `coalesce` | Returns first non-empty from multiple columns | `default` |
| `if_empty` | Returns different values for empty/non-empty | `then`, `else` |
| `if_contains` | Returns value based on substring match | `search` (required), `then`, `else` |

#### Lookup Transforms

| Function | Description | Arguments |
|----------|-------------|-----------|
| `map` | Maps input to output via key-value pairs | Any key=value pair, `default` |
| `extract_account` | Searches description for patterns | Pattern=account pairs, `default` |

### Category Rules (Post-Import)

Applied after field mappings to auto-categorize transactions:

```yaml
categories:
  # Match by description keywords
  - name: groceries
    match:
      description_contains:
        - grocery
        - supermarket
        - whole foods
        - trader joe
    set_account: expenses:food:groceries
    set_category: groceries

  # Match by regex
  - name: amazon
    match:
      description_regex: "AMZN|AMAZON"
    set_account: expenses:shopping
    set_tags:
      merchant: amazon

  # Match by amount range
  - name: large-expenses
    match:
      amount_min: 500
      is_debit: true
    set_tags:
      review: required

  # Combine conditions (all must match)
  - name: small-coffee
    match:
      description_contains: ["starbucks", "coffee"]
      amount_max: 20
      is_debit: true
    set_account: expenses:food:coffee
```

**Match conditions:**
- `description_contains: [strings]` - ANY pattern matches (case-insensitive)
- `description_regex: string` - Regex pattern
- `amount_min: float` - Minimum amount
- `amount_max: float` - Maximum amount
- `is_debit: bool` - true=expense, false=income

**Actions:**
- `set_account` - Override debit or credit account
- `set_category` - Sets `tags["category"]`
- `set_tags` - Additional key-value tags

### Import Process Flow

```
1. Load rules file
2. Read CSV/Excel with specified encoding
3. Skip header lines (format.skip_lines)
4. For each row:
   a. Apply field mappings → extract values
   b. Parse date (tries multiple formats)
   c. Parse amount (signed or debit/credit)
   d. Skip rows with zero/empty amounts
   e. Determine accounts (defaults or mapped)
   f. Apply category rules (first match wins)
   g. Generate transaction ID: SHA256(date|amount|description)[:16]
5. Deduplicate against existing journal
6. Write new transactions to journal
```

### Validation Rules

**Required in every rules file:**
- `name` must be non-empty
- `source_account` must be non-empty
- Must have mapping for `date` field
- Must have mapping for `amount`, `debit_amount`, or `credit_amount`

**Common errors:**
- **Duplicate field mappings:** Later mappings override earlier ones (this was your original bug!)
- **Wrong column index:** Check 0-based column numbering
- **Date format mismatch:** Ensure `input_format` matches your actual data

### Complete Example

```yaml
name: my-bank
description: Main checking account from MyBank
version: "1"

format:
  type: csv
  delimiter: ","
  encoding: utf-8
  skip_lines: 1

source_account: assets:checking:mybank
default_debit_account: expenses:unknown
default_credit_account: income:unknown
currency: USD

columns:
  - index: 0
    name: Date
    samples: ["01/15/2025"]
  - index: 1
    name: Description
    samples: ["GROCERY STORE"]
  - index: 2
    name: Debit
    samples: ["45.32"]
  - index: 3
    name: Credit
    samples: ["2000.00"]
  - index: 4
    name: Reference
    samples: ["TXN123456"]

mappings:
  # Parse date from DD/MM/YYYY format
  - field: date
    transform:
      column: 0
      function: parse_date
      args:
        input_format: "02/01/2006"

  # Combine and clean description
  - field: description
    transform:
      column: 1
      function: clean

  # Parse debit amount (expenses)
  - field: debit_amount
    transform:
      column: 2
      function: parse_number

  # Parse credit amount (income)
  - field: credit_amount
    transform:
      column: 3
      function: parse_number

  # Store reference as tag
  - field: reference
    direct:
      column: 4

categories:
  - name: groceries
    match:
      description_contains: ["grocery", "supermarket", "whole foods"]
    set_account: expenses:food:groceries

  - name: salary
    match:
      description_contains: ["payroll", "salary", "direct deposit"]
      is_debit: false
    set_account: income:salary

  - name: utilities
    match:
      description_contains: ["electric", "gas", "water", "utility"]
    set_account: expenses:utilities

  - name: large-expense-review
    match:
      amount_min: 500
      is_debit: true
    set_tags:
      needs_review: "true"
```

### Legacy ImportMap Format (JSON)

For simpler cases, you can use the legacy JSON format:

```json
{
  "name": "my-bank",
  "delimiter": ",",
  "encoding": "utf-8",
  "skip_lines": 1,
  "date_format": "02/01/2006",
  "columns": {
    "date": 0,
    "debit_amount": 2,
    "credit_amount": 3,
    "description": 1,
    "reference": 4
  },
  "source_account": "assets:checking:mybank",
  "default_debit_account": "expenses:unknown",
  "default_credit_account": "income:unknown",
  "currency": "USD",
  "transforms": [
    {
      "description_contains": "GROCERY",
      "debit_account": "expenses:food:groceries"
    }
  ]
}
```

Use with: `doublebook import --map mybank.importmap.json bank.csv`

### Troubleshooting

**"cannot parse date" errors:**
- Check `date` field is mapped to correct column
- Verify only ONE `date` mapping exists (duplicates override!)
- Ensure `input_format` matches your data format

**Zero transactions imported:**
- Check `skip_lines` isn't skipping data rows
- Verify amount column(s) contain valid numbers
- Check delimiter matches your file

**Wrong accounts assigned:**
- Category rules apply first-match-wins
- Order rules from most specific to least specific
- Use `--dry-run` to preview results

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

The codebase follows a domain-driven layout with clear separation of concerns:

```
gledger/
├── cmd/                    # Application entry points
│   └── doublebook/         # Main CLI application
│       └── main.go
│
├── core/                   # Core domain (minimal dependencies)
│   ├── ast/                # Transaction, Posting, Amount types
│   ├── lexer/              # Journal file tokenizer
│   ├── parser/             # Journal format parser
│   ├── journal/            # Journal file I/O
│   └── currency/           # Currency handling & conversion
│
├── engine/                 # Business logic layer
│   ├── interpreter/        # Query engine, balance calculations
│   ├── fql/                # Financial Query Language
│   └── dashboard/          # Dashboard query definitions
│
├── ingest/                 # Data import functionality
│   ├── rules/              # Rules engine (YAML-based) - recommended
│   │   ├── rule.go         # RuleSet, FieldMapping, CategoryRule
│   │   ├── engine.go       # Row processing engine
│   │   ├── transform.go    # 22 transform functions
│   │   ├── loader.go       # YAML loading/saving
│   │   ├── mapper.go       # Interactive TUI mapper
│   │   └── preview.go      # File preview, auto-detection
│   └── legacy/             # Legacy importer (ImportMap JSON)
│
├── interface/              # User-facing layers
│   ├── cli/                # Command-line interface
│   │   └── commands/       # CLI command implementations
│   ├── tui/                # Terminal UI (Bubbletea)
│   ├── api/                # REST API server
│   │   └── handlers/       # HTTP handlers
│   └── web/                # Web server with embedded frontend
│
├── infra/                  # Infrastructure & utilities
│   ├── config/             # Configuration management
│   ├── db/                 # SQLite cache/backend
│   └── utils/              # Shared utilities
│
├── plugin/                 # Plugin system
│   └── extensions/         # Built-in plugins
│       ├── sqlexport/      # SQLite export plugin
│       └── recurring/      # Recurring transactions plugin
│
├── web-ui/                 # React frontend (separate build)
│   └── src/
│
├── examples/               # Example files
│   ├── data/               # Sample journals and CSV files
│   └── imports/            # Sample import mappings
│
└── docs/                   # Documentation
    ├── site/               # Astro documentation website
    ├── development/        # Development task history
    ├── FQL.md              # FQL reference
    └── PLUGINS.md          # Plugin development guide
```

### Architecture Layers

| Layer | Purpose | Dependencies |
|-------|---------|--------------|
| `core/` | Domain types & parsing | Minimal (stdlib only) |
| `engine/` | Business logic | core/ |
| `ingest/` | Data import | core/, infra/ |
| `interface/` | User interfaces | engine/, ingest/, infra/ |
| `infra/` | Infrastructure | core/ |
| `plugin/` | Extensions | engine/, infra/ |

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]

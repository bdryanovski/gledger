# Financial Query Language (FQL)

FQL is a SQL-like query language for analyzing your DoubleBook financial data. It provides a simple yet powerful way to query transactions, accounts, and spending patterns.

## Quick Start

```bash
# Start the interactive REPL
doublebook fql

# Run a single query
doublebook fql --query "SELECT * FROM transactions LIMIT 10"
```

## Available Tables

FQL provides three virtual tables that aggregate your journal data:

### `transactions`

Individual postings from all transactions.

| Column | Type | Description |
|--------|------|-------------|
| `id` | text | Unique transaction identifier |
| `date` | text | Transaction date (YYYY-MM-DD) |
| `description` | text | Transaction description |
| `status` | text | Status: '' uncleared, '!' pending, '*' cleared |
| `account` | text | Posting account name |
| `amount` | real | Posting amount (negative = credit) |
| `currency` | text | Amount currency code |
| `tags` | text | Semicolon-separated key=value tag pairs |

### `accounts`

Aggregated account statistics.

| Column | Type | Description |
|--------|------|-------------|
| `account` | text | Full account name |
| `type` | text | Account type: asset/liability/equity/income/expense/other |
| `transaction_count` | integer | Number of postings |
| `total_amount` | real | Sum of all posting amounts |
| `last_transaction` | text | Date of most recent transaction |
| `first_transaction` | text | Date of earliest transaction |

### `spending`

Daily spending aggregated by account.

| Column | Type | Description |
|--------|------|-------------|
| `date` | text | Transaction date |
| `month` | text | Year-month (YYYY-MM) |
| `year` | text | Year (YYYY) |
| `account` | text | Account name |
| `transaction_count` | integer | Number of postings |
| `total_amount` | real | Sum of amounts for this date+account |
| `avg_amount` | real | Average amount |
| `min_amount` | real | Minimum amount |
| `max_amount` | real | Maximum amount |

## SQL Syntax

FQL supports a subset of SQL syntax:

### SELECT Statement

```sql
SELECT [DISTINCT] column1, column2, ...
FROM table_name
[WHERE condition]
[GROUP BY column1, column2, ...]
[HAVING condition]
[ORDER BY column1 [ASC|DESC], ...]
[LIMIT count]
[OFFSET start]
```

### Column Selection

```sql
-- All columns
SELECT * FROM transactions

-- Specific columns
SELECT date, description, amount FROM transactions

-- With aliases
SELECT account, SUM(amount) AS total FROM transactions GROUP BY account

-- Distinct values
SELECT DISTINCT account FROM transactions
```

### Aggregate Functions

| Function | Description |
|----------|-------------|
| `COUNT(*)` | Count all rows |
| `COUNT(column)` | Count non-null values |
| `SUM(column)` | Sum of values |
| `AVG(column)` | Average of values |
| `MIN(column)` | Minimum value |
| `MAX(column)` | Maximum value |

### WHERE Clause

#### Comparison Operators

```sql
-- Equal
WHERE amount = 100

-- Not equal
WHERE status != '*'
WHERE status <> '*'

-- Comparison
WHERE amount > 100
WHERE amount >= 100
WHERE amount < 50
WHERE amount <= 50

-- Negative numbers
WHERE amount < -100
```

#### LIKE Operator

```sql
-- Pattern matching (% = any characters, _ = single character)
WHERE account LIKE 'expenses:%'
WHERE description LIKE '%grocery%'
```

#### IN Operator

```sql
WHERE account IN ('assets:checking', 'assets:savings')
WHERE status IN ('*', '!')
```

#### BETWEEN Operator

```sql
WHERE amount BETWEEN 10 AND 100
WHERE date BETWEEN '2025-01-01' AND '2025-12-31'
```

#### NULL Checks

```sql
WHERE status IS NULL
WHERE status IS NOT NULL
```

#### Logical Operators

```sql
-- AND
WHERE account LIKE 'expenses:%' AND amount > 100

-- OR
WHERE account = 'assets:checking' OR account = 'assets:savings'

-- NOT
WHERE NOT account LIKE 'income:%'

-- Combining
WHERE (account LIKE 'expenses:%' OR account LIKE 'income:%') AND amount > 0
```

### GROUP BY and HAVING

```sql
-- Group by account
SELECT account, SUM(amount) AS total
FROM transactions
GROUP BY account

-- Filter groups with HAVING
SELECT account, SUM(amount) AS total
FROM transactions
GROUP BY account
HAVING total > 1000

-- Multiple group columns
SELECT month, account, SUM(amount) AS total
FROM spending
GROUP BY month, account
```

### ORDER BY

```sql
-- Ascending (default)
ORDER BY date ASC

-- Descending
ORDER BY amount DESC

-- Multiple columns
ORDER BY date DESC, amount ASC
```

### LIMIT and OFFSET

```sql
-- First 10 results
LIMIT 10

-- Skip first 20, then get 10
LIMIT 10 OFFSET 20
```

## Example Queries

### Recent Transactions

```sql
SELECT date, description, account, amount
FROM transactions
ORDER BY date DESC
LIMIT 20
```

### Account Balances

```sql
SELECT account, total_amount AS balance
FROM accounts
ORDER BY total_amount DESC
```

### Monthly Spending by Category

```sql
SELECT month, account, SUM(amount) AS total
FROM spending
WHERE account LIKE 'expenses:%'
GROUP BY month, account
ORDER BY month DESC, total DESC
```

### Top Expense Categories

```sql
SELECT account, SUM(amount) AS total
FROM transactions
WHERE account LIKE 'expenses:%'
GROUP BY account
ORDER BY total DESC
LIMIT 10
```

### Income vs Expenses by Month

```sql
-- Income (as positive values)
SELECT month, SUM(-amount) AS income
FROM spending
WHERE account LIKE 'income:%'
GROUP BY month
ORDER BY month

-- Expenses
SELECT month, SUM(amount) AS expenses
FROM spending
WHERE account LIKE 'expenses:%'
GROUP BY month
ORDER BY month
```

### Large Transactions

```sql
SELECT date, description, account, amount
FROM transactions
WHERE amount > 500 OR amount < -500
ORDER BY amount DESC
```

### Transactions with Tags

```sql
SELECT date, description, tags
FROM transactions
WHERE tags LIKE '%category=%'
```

### Account Activity Summary

```sql
SELECT
    account,
    transaction_count,
    total_amount,
    first_transaction,
    last_transaction
FROM accounts
WHERE type = 'expense'
ORDER BY transaction_count DESC
```

### Daily Spending Average

```sql
SELECT
    date,
    COUNT(*) AS transactions,
    SUM(amount) AS total,
    AVG(amount) AS average
FROM transactions
WHERE account LIKE 'expenses:%'
GROUP BY date
HAVING total > 100
ORDER BY date DESC
```

### Year-over-Year Comparison

```sql
SELECT
    year,
    SUM(amount) AS total_spending
FROM spending
WHERE account LIKE 'expenses:%'
GROUP BY year
ORDER BY year
```

## Output Formats

FQL automatically detects the query result shape and renders:

- **Bar charts**: For queries with 2 columns where the second is numeric
- **Line charts**: For time-series data (date/month columns)
- **Tables**: For all other result shapes

## API Usage

FQL is also available via the REST API:

```bash
curl -X POST http://localhost:5555/api/fql \
  -H "Content-Type: application/json" \
  -d '{"query": "SELECT * FROM transactions LIMIT 10"}'
```

Response format:

```json
{
  "columns": ["id", "date", "description", "status", "account", "amount", "currency", "tags"],
  "rows": [...],
  "row_count": 10
}
```

## Error Handling

FQL provides clear error messages for common issues:

```
Error: unknown virtual table "txns". Available: accounts, spending, transactions
Error: unknown column "desc" in SELECT. Available columns for "transactions": id, date, description, ...
Error: FQL parse error: expected identifier after SELECT
```

## Tips

1. **Use LIKE for hierarchical accounts**: `account LIKE 'expenses:food:%'` matches all food subcategories
2. **Negative amounts are credits**: Income appears as negative, expenses as positive
3. **Use the spending table for aggregates**: It pre-calculates monthly/yearly totals
4. **Filter early**: Put specific WHERE conditions before GROUP BY for better performance
5. **Alias aggregates**: `SUM(amount) AS total` makes HAVING and ORDER BY cleaner

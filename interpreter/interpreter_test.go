package interpreter

import (
	"testing"
	"time"

	"doublebook/ast"
	"doublebook/config"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestConfig() *config.Config {
	return &config.Config{
		DataFile:    "/tmp/test.journal",
		DataDir:     "/tmp",
		JournalName: "test",
		Currency:    "USD",
	}
}

func newTestTransaction(dateStr, description string, postings ...*ast.Posting) *ast.Transaction {
	date, _ := time.Parse("2006-01-02", dateStr)
	txn := ast.NewTransaction(date, description)
	txn.Postings = postings
	return txn
}

func newTestPosting(account string, value float64, currency string) *ast.Posting {
	return ast.NewPosting(account, ast.Amount{Value: value, Currency: currency})
}

// ---------------------------------------------------------------------------
// NewInterpreter tests
// ---------------------------------------------------------------------------

func TestNewInterpreter(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	if interp == nil {
		t.Fatal("NewInterpreter returned nil")
	}

	if interp.config != cfg {
		t.Error("Interpreter config not set correctly")
	}

	if interp.transactions == nil {
		t.Error("Interpreter transactions slice not initialized")
	}

	if interp.plugins == nil {
		t.Error("Interpreter plugin manager not initialized")
	}
}

func TestGetConfig(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	if interp.GetConfig() != cfg {
		t.Error("GetConfig did not return the expected config")
	}
}

// ---------------------------------------------------------------------------
// AddTransaction tests
// ---------------------------------------------------------------------------

func TestAddTransaction_Balanced(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	txn := newTestTransaction("2025-01-15", "Test Transaction",
		newTestPosting("expenses:food", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	)

	err := interp.AddTransaction(txn)
	if err != nil {
		t.Fatalf("AddTransaction failed for balanced transaction: %v", err)
	}

	if len(interp.GetTransactions()) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(interp.GetTransactions()))
	}
}

func TestAddTransaction_Unbalanced(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	txn := newTestTransaction("2025-01-15", "Unbalanced Transaction",
		newTestPosting("expenses:food", 50.00, "USD"),
		newTestPosting("assets:checking", -30.00, "USD"),
	)

	err := interp.AddTransaction(txn)
	if err == nil {
		t.Fatal("AddTransaction should fail for unbalanced transaction")
	}
}

func TestAddTransaction_SortsbyDate(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	// Add transactions out of order
	txn2 := newTestTransaction("2025-01-20", "Second",
		newTestPosting("expenses:food", 20.00, "USD"),
		newTestPosting("assets:checking", -20.00, "USD"),
	)
	txn1 := newTestTransaction("2025-01-10", "First",
		newTestPosting("expenses:food", 10.00, "USD"),
		newTestPosting("assets:checking", -10.00, "USD"),
	)
	txn3 := newTestTransaction("2025-01-15", "Third",
		newTestPosting("expenses:food", 15.00, "USD"),
		newTestPosting("assets:checking", -15.00, "USD"),
	)

	interp.AddTransaction(txn2)
	interp.AddTransaction(txn1)
	interp.AddTransaction(txn3)

	txns := interp.GetTransactions()
	if len(txns) != 3 {
		t.Fatalf("Expected 3 transactions, got %d", len(txns))
	}

	// Should be sorted: First (10th), Third (15th), Second (20th)
	if txns[0].Description != "First" {
		t.Errorf("Expected first transaction to be 'First', got %q", txns[0].Description)
	}
	if txns[1].Description != "Third" {
		t.Errorf("Expected second transaction to be 'Third', got %q", txns[1].Description)
	}
	if txns[2].Description != "Second" {
		t.Errorf("Expected third transaction to be 'Second', got %q", txns[2].Description)
	}
}

func TestAddTransaction_ImpliedAmount(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	// Create transaction with one omitted posting
	txn := newTestTransaction("2025-01-15", "With Implied Amount",
		newTestPosting("expenses:food", 50.00, "USD"),
		ast.NewOmittedPosting("assets:checking"),
	)
	txn.FillImpliedAmounts()

	err := interp.AddTransaction(txn)
	if err != nil {
		t.Fatalf("AddTransaction failed for transaction with implied amount: %v", err)
	}

	// Verify the implied amount was filled correctly
	added := interp.GetTransactions()[0]
	if added.Postings[1].Amount.Value != -50.00 {
		t.Errorf("Expected implied amount -50.00, got %.2f", added.Postings[1].Amount.Value)
	}
}

// ---------------------------------------------------------------------------
// Filter tests
// ---------------------------------------------------------------------------

func setupFilterTestInterpreter(t *testing.T) *Interpreter {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	transactions := []*ast.Transaction{
		newTestTransaction("2025-01-10", "Grocery Store",
			newTestPosting("expenses:groceries", 45.00, "USD"),
			newTestPosting("assets:checking", -45.00, "USD"),
		),
		newTestTransaction("2025-01-15", "Salary",
			newTestPosting("assets:checking", 2000.00, "USD"),
			newTestPosting("income:salary", -2000.00, "USD"),
		),
		newTestTransaction("2025-01-20", "Restaurant",
			newTestPosting("expenses:dining", 65.00, "USD"),
			newTestPosting("assets:checking", -65.00, "USD"),
		),
		newTestTransaction("2025-02-01", "Rent",
			newTestPosting("expenses:housing", 1200.00, "USD"),
			newTestPosting("assets:checking", -1200.00, "USD"),
		),
	}

	// Add tags to some transactions
	transactions[0].Tags = map[string]string{"category": "food"}
	transactions[2].Tags = map[string]string{"category": "food", "location": "downtown"}

	for _, txn := range transactions {
		if err := interp.AddTransaction(txn); err != nil {
			t.Fatalf("Failed to add transaction: %v", err)
		}
	}

	return interp
}

func TestFilteredTransactions_NoFilter(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{})
	if len(result) != 4 {
		t.Errorf("Expected 4 transactions with no filter, got %d", len(result))
	}
}

func TestFilteredTransactions_BeginDate(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{BeginDate: "2025-01-15"})
	if len(result) != 3 {
		t.Errorf("Expected 3 transactions from 2025-01-15, got %d", len(result))
	}
}

func TestFilteredTransactions_EndDate(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{EndDate: "2025-01-20"})
	if len(result) != 3 {
		t.Errorf("Expected 3 transactions until 2025-01-20, got %d", len(result))
	}
}

func TestFilteredTransactions_DateRange(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{
		BeginDate: "2025-01-15",
		EndDate:   "2025-01-20",
	})
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions in range, got %d", len(result))
	}
}

func TestFilteredTransactions_AccountFilter(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{Account: "expenses:groceries"})
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction with groceries account, got %d", len(result))
	}

	// Partial match
	result = interp.FilteredTransactions(Filter{Account: "expenses"})
	if len(result) != 3 {
		t.Errorf("Expected 3 transactions with expenses account, got %d", len(result))
	}
}

func TestFilteredTransactions_DescriptionFilter(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{Description: "grocery"})
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction with 'grocery' description, got %d", len(result))
	}

	// Case insensitive
	result = interp.FilteredTransactions(Filter{Description: "SALARY"})
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction with 'SALARY' description (case insensitive), got %d", len(result))
	}
}

func TestFilteredTransactions_TagFilter(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{
		Tags: map[string]string{"category": "food"},
	})
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions with category:food tag, got %d", len(result))
	}

	// Multiple tags (AND)
	result = interp.FilteredTransactions(Filter{
		Tags: map[string]string{"category": "food", "location": "downtown"},
	})
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction with both tags, got %d", len(result))
	}
}

func TestFilteredTransactions_CombinedFilters(t *testing.T) {
	interp := setupFilterTestInterpreter(t)

	result := interp.FilteredTransactions(Filter{
		BeginDate:   "2025-01-01",
		EndDate:     "2025-01-31",
		Account:     "expenses",
		Description: "store",
	})
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction matching all criteria, got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// CalculateBalances tests
// ---------------------------------------------------------------------------

func TestCalculateBalances(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Tx1",
		newTestPosting("expenses:food", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-15", "Tx2",
		newTestPosting("expenses:food", 30.00, "USD"),
		newTestPosting("assets:checking", -30.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-20", "Tx3",
		newTestPosting("assets:checking", 1000.00, "USD"),
		newTestPosting("income:salary", -1000.00, "USD"),
	))

	balances := interp.CalculateBalances()

	expectedBalances := map[string]float64{
		"expenses:food":   80.00,
		"assets:checking": 920.00,
		"income:salary":   -1000.00,
	}

	for account, expected := range expectedBalances {
		actual, ok := balances[account]
		if !ok {
			t.Errorf("Account %q not found in balances", account)
			continue
		}
		if actual != expected {
			t.Errorf("Balance for %q: expected %.2f, got %.2f", account, expected, actual)
		}
	}
}

func TestCalculateBalances_Empty(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	balances := interp.CalculateBalances()
	if len(balances) != 0 {
		t.Errorf("Expected empty balances, got %d entries", len(balances))
	}
}

// ---------------------------------------------------------------------------
// CalculateBalancesTree tests
// ---------------------------------------------------------------------------

func TestCalculateBalancesTree(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Groceries",
		newTestPosting("expenses:food:groceries", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-15", "Restaurant",
		newTestPosting("expenses:food:dining", 30.00, "USD"),
		newTestPosting("assets:checking", -30.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-20", "Gas",
		newTestPosting("expenses:transport", 40.00, "USD"),
		newTestPosting("assets:checking", -40.00, "USD"),
	))

	nodes := interp.CalculateBalancesTree(Filter{})

	// Check root expenses node
	expensesNode, ok := nodes["expenses"]
	if !ok {
		t.Fatal("expenses node not found")
	}
	if expensesNode.Amount.Value != 120.00 {
		t.Errorf("expenses total: expected 120.00, got %.2f", expensesNode.Amount.Value)
	}

	// Check expenses:food node
	foodNode, ok := nodes["expenses:food"]
	if !ok {
		t.Fatal("expenses:food node not found")
	}
	if foodNode.Amount.Value != 80.00 {
		t.Errorf("expenses:food total: expected 80.00, got %.2f", foodNode.Amount.Value)
	}

	// Check leaf nodes
	groceriesNode, ok := nodes["expenses:food:groceries"]
	if !ok {
		t.Fatal("expenses:food:groceries node not found")
	}
	if groceriesNode.Amount.Value != 50.00 {
		t.Errorf("expenses:food:groceries: expected 50.00, got %.2f", groceriesNode.Amount.Value)
	}
}

func TestCalculateBalancesTree_WithFilter(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Groceries",
		newTestPosting("expenses:food", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-02-15", "More Groceries",
		newTestPosting("expenses:food", 75.00, "USD"),
		newTestPosting("assets:checking", -75.00, "USD"),
	))

	// Filter to January only
	nodes := interp.CalculateBalancesTree(Filter{
		BeginDate: "2025-01-01",
		EndDate:   "2025-01-31",
	})

	foodNode, ok := nodes["expenses:food"]
	if !ok {
		t.Fatal("expenses:food node not found")
	}
	if foodNode.Amount.Value != 50.00 {
		t.Errorf("expenses:food (January only): expected 50.00, got %.2f", foodNode.Amount.Value)
	}
}

func TestCalculateBalancesTree_ParentChildRelationships(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Test",
		newTestPosting("expenses:food:groceries", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	))

	nodes := interp.CalculateBalancesTree(Filter{})

	// Check parent-child relationship
	expensesNode := nodes["expenses"]
	if len(expensesNode.Children) != 1 {
		t.Errorf("expenses should have 1 child, got %d", len(expensesNode.Children))
	}
	if expensesNode.Children[0].FullName != "expenses:food" {
		t.Errorf("expenses child should be expenses:food, got %q", expensesNode.Children[0].FullName)
	}

	foodNode := nodes["expenses:food"]
	if len(foodNode.Children) != 1 {
		t.Errorf("expenses:food should have 1 child, got %d", len(foodNode.Children))
	}
	if foodNode.Children[0].FullName != "expenses:food:groceries" {
		t.Errorf("expenses:food child should be expenses:food:groceries, got %q", foodNode.Children[0].FullName)
	}
}

// ---------------------------------------------------------------------------
// GroupAccountsByType tests
// ---------------------------------------------------------------------------

func TestGroupAccountsByType(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Test",
		newTestPosting("expenses:food", 50.00, "USD"),
		newTestPosting("assets:checking", -30.00, "USD"),
		newTestPosting("liabilities:credit", -20.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-15", "Income",
		newTestPosting("assets:checking", 1000.00, "USD"),
		newTestPosting("income:salary", -1000.00, "USD"),
	))

	nodes := interp.CalculateBalancesTree(Filter{})
	groups := GroupAccountsByType(nodes)

	// Check each group
	if len(groups["assets"]) != 1 {
		t.Errorf("Expected 1 asset account group, got %d", len(groups["assets"]))
	}
	if len(groups["expenses"]) != 1 {
		t.Errorf("Expected 1 expenses account group, got %d", len(groups["expenses"]))
	}
	if len(groups["income"]) != 1 {
		t.Errorf("Expected 1 income account group, got %d", len(groups["income"]))
	}
	if len(groups["liabilities"]) != 1 {
		t.Errorf("Expected 1 liabilities account group, got %d", len(groups["liabilities"]))
	}
}

func TestGroupAccountsByType_UnknownType(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Test",
		newTestPosting("custom:category", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	))

	nodes := interp.CalculateBalancesTree(Filter{})
	groups := GroupAccountsByType(nodes)

	// Unknown type should go to "other"
	if len(groups["other"]) != 1 {
		t.Errorf("Expected 1 'other' account group for custom account, got %d", len(groups["other"]))
	}
}

// ---------------------------------------------------------------------------
// Income Statement tests
// ---------------------------------------------------------------------------

func TestGenerateIncomeStatement(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	// Income transactions (negative in double-entry)
	interp.AddTransaction(newTestTransaction("2025-01-15", "Salary",
		newTestPosting("assets:checking", 2000.00, "USD"),
		newTestPosting("income:salary", -2000.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-20", "Freelance",
		newTestPosting("assets:checking", 500.00, "USD"),
		newTestPosting("income:freelance", -500.00, "USD"),
	))

	// Expense transactions (positive in double-entry)
	interp.AddTransaction(newTestTransaction("2025-01-10", "Groceries",
		newTestPosting("expenses:food", 150.00, "USD"),
		newTestPosting("assets:checking", -150.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-01-25", "Rent",
		newTestPosting("expenses:housing", 1200.00, "USD"),
		newTestPosting("assets:checking", -1200.00, "USD"),
	))

	stmt := interp.GenerateIncomeStatement(Filter{})

	// Check revenues (should be positive after negation)
	if stmt.Revenues["income:salary"].Value != 2000.00 {
		t.Errorf("Expected salary revenue 2000.00, got %.2f", stmt.Revenues["income:salary"].Value)
	}
	if stmt.Revenues["income:freelance"].Value != 500.00 {
		t.Errorf("Expected freelance revenue 500.00, got %.2f", stmt.Revenues["income:freelance"].Value)
	}

	// Check expenses
	if stmt.Expenses["expenses:food"].Value != 150.00 {
		t.Errorf("Expected food expenses 150.00, got %.2f", stmt.Expenses["expenses:food"].Value)
	}
	if stmt.Expenses["expenses:housing"].Value != 1200.00 {
		t.Errorf("Expected housing expenses 1200.00, got %.2f", stmt.Expenses["expenses:housing"].Value)
	}

	// Check net income: 2500 (revenue) - 1350 (expenses) = 1150
	expectedNetIncome := 2500.00 - 1350.00
	if stmt.NetIncome.Value != expectedNetIncome {
		t.Errorf("Expected net income %.2f, got %.2f", expectedNetIncome, stmt.NetIncome.Value)
	}
}

func TestGenerateIncomeStatement_WithFilter(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-15", "January Salary",
		newTestPosting("assets:checking", 2000.00, "USD"),
		newTestPosting("income:salary", -2000.00, "USD"),
	))
	interp.AddTransaction(newTestTransaction("2025-02-15", "February Salary",
		newTestPosting("assets:checking", 2000.00, "USD"),
		newTestPosting("income:salary", -2000.00, "USD"),
	))

	// Filter to January only
	stmt := interp.GenerateIncomeStatement(Filter{
		BeginDate: "2025-01-01",
		EndDate:   "2025-01-31",
	})

	// Should only include January salary
	if stmt.Revenues["income:salary"].Value != 2000.00 {
		t.Errorf("Expected January salary revenue 2000.00, got %.2f", stmt.Revenues["income:salary"].Value)
	}
}

func TestGenerateIncomeStatement_Empty(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	stmt := interp.GenerateIncomeStatement(Filter{})

	if len(stmt.Revenues) != 0 {
		t.Errorf("Expected 0 revenues, got %d", len(stmt.Revenues))
	}
	if len(stmt.Expenses) != 0 {
		t.Errorf("Expected 0 expenses, got %d", len(stmt.Expenses))
	}
	if stmt.NetIncome.Value != 0 {
		t.Errorf("Expected net income 0, got %.2f", stmt.NetIncome.Value)
	}
}

func TestGenerateIncomeStatement_OnlyIncome(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-15", "Salary",
		newTestPosting("assets:checking", 2000.00, "USD"),
		newTestPosting("income:salary", -2000.00, "USD"),
	))

	stmt := interp.GenerateIncomeStatement(Filter{})

	if stmt.NetIncome.Value != 2000.00 {
		t.Errorf("Expected net income 2000.00 (income only), got %.2f", stmt.NetIncome.Value)
	}
}

func TestGenerateIncomeStatement_OnlyExpenses(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Groceries",
		newTestPosting("expenses:food", 150.00, "USD"),
		newTestPosting("assets:checking", -150.00, "USD"),
	))

	stmt := interp.GenerateIncomeStatement(Filter{})

	if stmt.NetIncome.Value != -150.00 {
		t.Errorf("Expected net income -150.00 (expenses only), got %.2f", stmt.NetIncome.Value)
	}
}

// ---------------------------------------------------------------------------
// Balance Report tests
// ---------------------------------------------------------------------------

func TestGenerateBalanceReport(t *testing.T) {
	cfg := newTestConfig()
	interp := NewInterpreter(cfg)

	interp.AddTransaction(newTestTransaction("2025-01-10", "Groceries",
		newTestPosting("expenses:food", 50.00, "USD"),
		newTestPosting("assets:checking", -50.00, "USD"),
	))

	report := interp.GenerateBalanceReport()

	if report == "" {
		t.Error("GenerateBalanceReport returned empty string")
	}

	// Check that the report contains expected sections
	if !contains(report, "BALANCE REPORT") {
		t.Error("Report should contain 'BALANCE REPORT' header")
	}
	if !contains(report, "ASSETS") {
		t.Error("Report should contain 'ASSETS' section")
	}
	if !contains(report, "EXPENSES") {
		t.Error("Report should contain 'EXPENSES' section")
	}
}

// ---------------------------------------------------------------------------
// journalStem tests
// ---------------------------------------------------------------------------

func TestJournalStem(t *testing.T) {
	cfg := &config.Config{
		DataFile:    "~/.doublebook/myjournal.journal",
		DataDir:     "~/.doublebook",
		JournalName: "myjournal",
	}
	interp := NewInterpreter(cfg)

	name, dir := interp.journalStem()

	if name != "myjournal" {
		t.Errorf("Expected journal name 'myjournal', got %q", name)
	}

	// Dir should be expanded (no ~)
	if contains(dir, "~") {
		t.Error("Directory should have ~ expanded")
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

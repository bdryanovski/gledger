package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doublebook/ast"
	"doublebook/config"
	"doublebook/db"
	"doublebook/interpreter"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestInterpreter(t *testing.T) *interpreter.Interpreter {
	cfg := &config.Config{
		DataFile:    "/tmp/test.journal",
		DataDir:     "/tmp",
		JournalName: "test",
		Currency:    "USD",
	}
	return interpreter.NewInterpreter(cfg)
}

func newTestHandlers(t *testing.T) (*Handlers, *interpreter.Interpreter) {
	interp := newTestInterpreter(t)
	// Use nil for db and converter since we won't test those in basic tests
	h := New(interp, nil, nil)
	return h, interp
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

func addTestTransactions(t *testing.T, interp *interpreter.Interpreter) {
	transactions := []*ast.Transaction{
		newTestTransaction("2025-01-10", "Groceries",
			newTestPosting("expenses:food:groceries", 45.00, "USD"),
			newTestPosting("assets:checking", -45.00, "USD"),
		),
		newTestTransaction("2025-01-15", "Salary",
			newTestPosting("assets:checking", 2000.00, "USD"),
			newTestPosting("income:salary", -2000.00, "USD"),
		),
		newTestTransaction("2025-01-20", "Restaurant",
			newTestPosting("expenses:food:dining", 65.00, "USD"),
			newTestPosting("assets:checking", -65.00, "USD"),
		),
	}

	for _, txn := range transactions {
		if err := interp.AddTransaction(txn); err != nil {
			t.Fatalf("Failed to add test transaction: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// ListTransactions tests
// ---------------------------------------------------------------------------

func TestListTransactions_Basic(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/transactions", nil)
	rr := httptest.NewRecorder()

	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	transactions, ok := response["transactions"].([]interface{})
	if !ok {
		t.Fatal("Response should contain 'transactions' array")
	}

	if len(transactions) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(transactions))
	}

	total, ok := response["total"].(float64)
	if !ok || total != 3 {
		t.Errorf("Expected total 3, got %v", response["total"])
	}
}

func TestListTransactions_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/api/transactions", nil)
	rr := httptest.NewRecorder()

	h.ListTransactions(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestListTransactions_WithDateFilter(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/transactions?begin=2025-01-15", nil)
	rr := httptest.NewRecorder()

	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	transactions := response["transactions"].([]interface{})
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions after date filter, got %d", len(transactions))
	}
}

func TestListTransactions_WithPagination(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/transactions?limit=2&offset=0", nil)
	rr := httptest.NewRecorder()

	h.ListTransactions(rr, req)

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	transactions := response["transactions"].([]interface{})
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions with limit, got %d", len(transactions))
	}

	total := response["total"].(float64)
	if total != 3 {
		t.Errorf("Total should still be 3, got %.0f", total)
	}
}

func TestListTransactions_WithAccountFilter(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/transactions?account=expenses:food:groceries", nil)
	rr := httptest.NewRecorder()

	h.ListTransactions(rr, req)

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	transactions := response["transactions"].([]interface{})
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction with account filter, got %d", len(transactions))
	}
}

func TestListTransactions_Empty(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/transactions", nil)
	rr := httptest.NewRecorder()

	h.ListTransactions(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	transactions := response["transactions"].([]interface{})
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(transactions))
	}
}

// ---------------------------------------------------------------------------
// ListAccounts tests
// ---------------------------------------------------------------------------

func TestListAccounts_Basic(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/accounts", nil)
	rr := httptest.NewRecorder()

	h.ListAccounts(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	accounts, ok := response["accounts"].([]interface{})
	if !ok {
		t.Fatal("Response should contain 'accounts' array")
	}

	// Should have root accounts for each type
	if len(accounts) < 3 {
		t.Errorf("Expected at least 3 accounts, got %d", len(accounts))
	}
}

func TestListAccounts_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/api/accounts", nil)
	rr := httptest.NewRecorder()

	h.ListAccounts(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestListAccounts_WithTypeFilter(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/accounts?type=expenses", nil)
	rr := httptest.NewRecorder()

	h.ListAccounts(rr, req)

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	accounts := response["accounts"].([]interface{})
	for _, acc := range accounts {
		accMap := acc.(map[string]interface{})
		if accMap["type"] != "expenses" {
			t.Errorf("Expected only expenses accounts, got type %q", accMap["type"])
		}
	}
}

// ---------------------------------------------------------------------------
// BalanceReport tests
// ---------------------------------------------------------------------------

func TestBalanceReport_Basic(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/reports/balance", nil)
	rr := httptest.NewRecorder()

	h.BalanceReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	// Should have at least some account types
	if len(response) == 0 {
		t.Error("Balance report should not be empty")
	}
}

func TestBalanceReport_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/api/reports/balance", nil)
	rr := httptest.NewRecorder()

	h.BalanceReport(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestBalanceReport_WithDateFilter(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/reports/balance?begin=2025-01-15&end=2025-01-20", nil)
	rr := httptest.NewRecorder()

	h.BalanceReport(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// ---------------------------------------------------------------------------
// IncomeStatement tests
// ---------------------------------------------------------------------------

func TestIncomeStatement_Basic(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/reports/income-statement", nil)
	rr := httptest.NewRecorder()

	h.IncomeStatement(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	// Check structure
	if _, ok := response["revenues"]; !ok {
		t.Error("Response should contain 'revenues'")
	}
	if _, ok := response["expenses"]; !ok {
		t.Error("Response should contain 'expenses'")
	}
	if _, ok := response["net_income"]; !ok {
		t.Error("Response should contain 'net_income'")
	}
}

func TestIncomeStatement_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/api/reports/income-statement", nil)
	rr := httptest.NewRecorder()

	h.IncomeStatement(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestIncomeStatement_Values(t *testing.T) {
	h, interp := newTestHandlers(t)
	addTestTransactions(t, interp)

	req := httptest.NewRequest("GET", "/api/reports/income-statement", nil)
	rr := httptest.NewRecorder()

	h.IncomeStatement(rr, req)

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	// Check net income calculation
	netIncome := response["net_income"].(map[string]interface{})
	value := netIncome["value"].(float64)

	// Revenue: 2000, Expenses: 45 + 65 = 110, Net: 2000 - 110 = 1890
	expectedNetIncome := 1890.0
	if value != expectedNetIncome {
		t.Errorf("Expected net income %.2f, got %.2f", expectedNetIncome, value)
	}
}

// ---------------------------------------------------------------------------
// FQLQuery tests
// ---------------------------------------------------------------------------

func TestFQLQuery_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/fql", nil)
	rr := httptest.NewRecorder()

	h.FQLQuery(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestFQLQuery_InvalidJSON(t *testing.T) {
	h, _ := newTestHandlers(t)

	body := bytes.NewBufferString("invalid json")
	req := httptest.NewRequest("POST", "/api/fql", body)
	rr := httptest.NewRecorder()

	h.FQLQuery(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestFQLQuery_EmptyQuery(t *testing.T) {
	h, _ := newTestHandlers(t)

	body := bytes.NewBufferString(`{"query": ""}`)
	req := httptest.NewRequest("POST", "/api/fql", body)
	rr := httptest.NewRecorder()

	h.FQLQuery(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "query field is required" {
		t.Errorf("Expected 'query field is required' error, got %v", response["error"])
	}
}

func TestFQLQuery_WithDatabase(t *testing.T) {
	interp := newTestInterpreter(t)
	addTestTransactions(t, interp)

	// Create a temporary database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Close()

	// Initialize schema and load transactions
	if err := database.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	if err := database.LoadFromTransactions(interp.GetTransactions()); err != nil {
		t.Fatalf("Failed to load transactions: %v", err)
	}

	h := New(interp, database, nil)

	body := bytes.NewBufferString(`{"query": "SELECT COUNT(*) FROM transactions"}`)
	req := httptest.NewRequest("POST", "/api/fql", body)
	rr := httptest.NewRecorder()

	h.FQLQuery(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

// ---------------------------------------------------------------------------
// ExchangeRate tests
// ---------------------------------------------------------------------------

func TestExchangeRate_MethodNotAllowed(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("POST", "/api/exchange-rates", nil)
	rr := httptest.NewRecorder()

	h.ExchangeRate(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestExchangeRate_MissingParams(t *testing.T) {
	h, _ := newTestHandlers(t)

	testCases := []struct {
		name string
		url  string
	}{
		{"missing all", "/api/exchange-rates"},
		{"missing from", "/api/exchange-rates?to=EUR&date=2025-01-15"},
		{"missing to", "/api/exchange-rates?from=USD&date=2025-01-15"},
		{"missing date", "/api/exchange-rates?from=USD&to=EUR"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			rr := httptest.NewRecorder()

			h.ExchangeRate(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
			}
		})
	}
}

func TestExchangeRate_NoConverter(t *testing.T) {
	h, _ := newTestHandlers(t)

	req := httptest.NewRequest("GET", "/api/exchange-rates?from=USD&to=EUR&date=2025-01-15", nil)
	rr := httptest.NewRecorder()

	h.ExchangeRate(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "currency converter not configured" {
		t.Errorf("Expected 'currency converter not configured' error, got %v", response["error"])
	}
}

// ---------------------------------------------------------------------------
// Response helper tests
// ---------------------------------------------------------------------------

func TestRespond(t *testing.T) {
	rr := httptest.NewRecorder()

	respond(rr, http.StatusOK, map[string]string{"message": "test"})

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response map[string]string
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["message"] != "test" {
		t.Errorf("Expected message 'test', got %q", response["message"])
	}
}

func TestRespondError(t *testing.T) {
	rr := httptest.NewRecorder()

	respondError(rr, http.StatusBadRequest, "test error")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	var response map[string]string
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "test error" {
		t.Errorf("Expected error 'test error', got %q", response["error"])
	}
}

// ---------------------------------------------------------------------------
// Handlers constructor test
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	interp := newTestInterpreter(t)
	h := New(interp, nil, nil)

	if h == nil {
		t.Fatal("New returned nil")
	}

	if h.interp != interp {
		t.Error("Interpreter not set correctly")
	}
}

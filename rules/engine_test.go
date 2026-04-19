package rules

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Transform Function Tests
// ---------------------------------------------------------------------------

func TestTransformUppercase(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"hello"}, "HELLO"},
		{[]string{"Hello World"}, "HELLO WORLD"},
		{[]string{""}, ""},
		{[]string{}, ""},
	}

	for _, tc := range tests {
		result, err := transformUppercase(tc.input, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("uppercase(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTransformLowercase(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"HELLO"}, "hello"},
		{[]string{"Hello World"}, "hello world"},
		{[]string{""}, ""},
	}

	for _, tc := range tests {
		result, err := transformLowercase(tc.input, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("lowercase(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTransformTrim(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{[]string{"  hello  "}, nil, "hello"},
		{[]string{"\t\nhello\n\t"}, nil, "hello"},
		{[]string{"***hello***"}, map[string]string{"chars": "*"}, "hello"},
	}

	for _, tc := range tests {
		result, err := transformTrim(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("trim(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

func TestTransformReplace(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{
			[]string{"hello world"},
			map[string]string{"old": "world", "new": "universe"},
			"hello universe",
		},
		{
			[]string{"foo-bar-baz"},
			map[string]string{"old": "-", "new": "_"},
			"foo_bar_baz",
		},
	}

	for _, tc := range tests {
		result, err := transformReplace(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("replace(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

func TestTransformConcat(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"hello", " ", "world"}, "hello world"},
		{[]string{"a", "b", "c"}, "abc"},
		{[]string{}, ""},
	}

	for _, tc := range tests {
		result, err := transformConcat(tc.input, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("concat(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTransformJoin(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{[]string{"hello", "world"}, map[string]string{"separator": " "}, "hello world"},
		{[]string{"a", "b", "c"}, map[string]string{"separator": "-"}, "a-b-c"},
		{[]string{"hello", "", "world"}, map[string]string{"separator": " "}, "hello world"},
	}

	for _, tc := range tests {
		result, err := transformJoin(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("join(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

func TestTransformFormat(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{
			[]string{"John", "Doe"},
			map[string]string{"template": "{0} {1}"},
			"John Doe",
		},
		{
			[]string{"2024", "01", "15"},
			map[string]string{"template": "{0}-{1}-{2}"},
			"2024-01-15",
		},
	}

	for _, tc := range tests {
		result, err := transformFormat(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("format(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

func TestTransformParseNumber(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{[]string{"1234.56"}, nil, "1234.56"},
		{[]string{"1,234.56"}, nil, "1234.56"},
		{[]string{"$100.00"}, nil, "100.00"},
		{[]string{"(50.00)"}, nil, "-50.00"},
		{[]string{"1.234,56"}, nil, "1234.56"}, // European format
	}

	for _, tc := range tests {
		result, err := transformParseNumber(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error for %v: %v", tc.input, err)
			continue
		}
		if result != tc.expected {
			t.Errorf("parse_number(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTransformAbs(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"-100.00"}, "100.00"},
		{[]string{"100.00"}, "100.00"},
		{[]string{"0"}, "0.00"},
	}

	for _, tc := range tests {
		result, err := transformAbs(tc.input, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("abs(%v) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTransformDefault(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{[]string{""}, map[string]string{"value": "default"}, "default"},
		{[]string{"actual"}, map[string]string{"value": "default"}, "actual"},
		{[]string{"  "}, map[string]string{"value": "default"}, "default"},
	}

	for _, tc := range tests {
		result, err := transformDefault(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("default(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

func TestTransformCoalesce(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{[]string{"", "", "third"}, nil, "third"},
		{[]string{"first", "second"}, nil, "first"},
		{[]string{"", "", ""}, map[string]string{"default": "none"}, "none"},
	}

	for _, tc := range tests {
		result, err := transformCoalesce(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("coalesce(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

func TestTransformRegexExtract(t *testing.T) {
	tests := []struct {
		input    []string
		args     map[string]string
		expected string
	}{
		{
			[]string{"Order #12345"},
			map[string]string{"pattern": `#(\d+)`},
			"12345",
		},
		{
			[]string{"No match here"},
			map[string]string{"pattern": `#(\d+)`, "default": "unknown"},
			"unknown",
		},
	}

	for _, tc := range tests {
		result, err := transformRegexExtract(tc.input, tc.args)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("regex_extract(%v, %v) = %q, want %q", tc.input, tc.args, result, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// RuleSet Validation Tests
// ---------------------------------------------------------------------------

func TestRuleSetValidation(t *testing.T) {
	// Valid ruleset
	valid := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "amount", Direct: &DirectMapping{Column: 1}},
		},
	}
	
	if err := valid.Validate(); err != nil {
		t.Errorf("valid ruleset should not error: %v", err)
	}

	// Missing name
	noName := &RuleSet{
		SourceAccount: "assets:checking",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "amount", Direct: &DirectMapping{Column: 1}},
		},
	}
	if err := noName.Validate(); err == nil {
		t.Error("ruleset without name should error")
	}

	// Missing source account
	noSource := &RuleSet{
		Name: "test",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "amount", Direct: &DirectMapping{Column: 1}},
		},
	}
	if err := noSource.Validate(); err == nil {
		t.Error("ruleset without source_account should error")
	}

	// Missing date mapping
	noDate := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Mappings: []FieldMapping{
			{Field: "amount", Direct: &DirectMapping{Column: 1}},
		},
	}
	if err := noDate.Validate(); err == nil {
		t.Error("ruleset without date mapping should error")
	}

	// Missing amount mapping
	noAmount := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
		},
	}
	if err := noAmount.Validate(); err == nil {
		t.Error("ruleset without amount mapping should error")
	}
}

func TestFieldMappingValidation(t *testing.T) {
	// Valid mapping with Direct
	valid := FieldMapping{
		Field:  "date",
		Direct: &DirectMapping{Column: 0},
	}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid mapping should not error: %v", err)
	}

	// Missing field name
	noField := FieldMapping{
		Direct: &DirectMapping{Column: 0},
	}
	if err := noField.Validate(); err == nil {
		t.Error("mapping without field name should error")
	}

	// No mapping type
	noType := FieldMapping{
		Field: "date",
	}
	if err := noType.Validate(); err == nil {
		t.Error("mapping without type should error")
	}

	// Multiple mapping types
	multi := FieldMapping{
		Field:    "date",
		Direct:   &DirectMapping{Column: 0},
		Constant: &ConstantMapping{Value: "test"},
	}
	if err := multi.Validate(); err == nil {
		t.Error("mapping with multiple types should error")
	}
}

// ---------------------------------------------------------------------------
// Engine Processing Tests
// ---------------------------------------------------------------------------

func TestEngineDirectMapping(t *testing.T) {
	rs := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Currency:      "USD",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "description", Direct: &DirectMapping{Column: 1}},
			{Field: "amount", Direct: &DirectMapping{Column: 2}},
		},
	}

	engine, err := NewEngine(rs)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	rows := [][]string{
		{"2024-01-15", "Coffee Shop", "-5.00"},
		{"2024-01-16", "Salary", "3000.00"},
	}

	result := engine.ProcessRows(rows)
	
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	
	if len(result.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(result.Transactions))
	}

	// First transaction (expense)
	txn1 := result.Transactions[0]
	if txn1.Description != "Coffee Shop" {
		t.Errorf("expected description 'Coffee Shop', got %q", txn1.Description)
	}

	// Second transaction (income)
	txn2 := result.Transactions[1]
	if txn2.Description != "Salary" {
		t.Errorf("expected description 'Salary', got %q", txn2.Description)
	}
}

func TestEngineCombineMapping(t *testing.T) {
	rs := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Currency:      "USD",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{
				Field: "description",
				Combine: &CombineMapping{
					Columns:   []int{1, 2},
					Separator: " - ",
					Trim:      true,
				},
			},
			{Field: "amount", Direct: &DirectMapping{Column: 3}},
		},
	}

	engine, err := NewEngine(rs)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	rows := [][]string{
		{"2024-01-15", "POS", "Coffee Shop", "-5.00"},
	}

	result := engine.ProcessRows(rows)
	
	if len(result.Errors) > 0 {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
	
	if len(result.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(result.Transactions))
	}

	if result.Transactions[0].Description != "POS - Coffee Shop" {
		t.Errorf("expected combined description, got %q", result.Transactions[0].Description)
	}
}

func TestEngineTransformMapping(t *testing.T) {
	rs := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Currency:      "USD",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{
				Field: "description",
				Transform: &TransformMapping{
					Column:   1,
					Function: "uppercase",
				},
			},
			{Field: "amount", Direct: &DirectMapping{Column: 2}},
		},
	}

	engine, err := NewEngine(rs)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	rows := [][]string{
		{"2024-01-15", "coffee shop", "-5.00"},
	}

	result := engine.ProcessRows(rows)
	
	if len(result.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(result.Transactions))
	}

	if result.Transactions[0].Description != "COFFEE SHOP" {
		t.Errorf("expected uppercase description, got %q", result.Transactions[0].Description)
	}
}

func TestEngineLookupMapping(t *testing.T) {
	rs := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Currency:      "USD",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "description", Direct: &DirectMapping{Column: 1}},
			{Field: "amount", Direct: &DirectMapping{Column: 2}},
			{
				Field: "debit_account",
				Lookup: &LookupMapping{
					Column: 3,
					Table: map[string]string{
						"FOOD":      "expenses:food",
						"TRANSPORT": "expenses:transport",
					},
					Default: "expenses:unknown",
				},
			},
		},
	}

	engine, err := NewEngine(rs)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	rows := [][]string{
		{"2024-01-15", "Coffee", "-5.00", "FOOD"},
		{"2024-01-16", "Bus fare", "-2.50", "TRANSPORT"},
		{"2024-01-17", "Something", "-10.00", "OTHER"},
	}

	result := engine.ProcessRows(rows)
	
	if len(result.Transactions) != 3 {
		t.Fatalf("expected 3 transactions, got %d", len(result.Transactions))
	}

	// Check accounts
	txn1 := result.Transactions[0]
	if txn1.Postings[1].Account != "expenses:food" {
		t.Errorf("expected expenses:food, got %q", txn1.Postings[1].Account)
	}

	txn2 := result.Transactions[1]
	if txn2.Postings[1].Account != "expenses:transport" {
		t.Errorf("expected expenses:transport, got %q", txn2.Postings[1].Account)
	}

	txn3 := result.Transactions[2]
	if txn3.Postings[1].Account != "expenses:unknown" {
		t.Errorf("expected expenses:unknown (default), got %q", txn3.Postings[1].Account)
	}
}

func TestEngineCategoryRules(t *testing.T) {
	rs := &RuleSet{
		Name:                 "test",
		SourceAccount:        "assets:checking",
		DefaultDebitAccount:  "expenses:unknown",
		Currency:             "USD",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "description", Direct: &DirectMapping{Column: 1}},
			{Field: "amount", Direct: &DirectMapping{Column: 2}},
		},
		Categories: []CategoryRule{
			{
				Name: "coffee shops",
				Match: CategoryMatch{
					DescriptionContains: []string{"coffee", "starbucks"},
				},
				SetAccount:  "expenses:dining:coffee",
				SetCategory: "coffee",
			},
			{
				Name: "large expenses",
				Match: CategoryMatch{
					AmountMin: floatPtr(100),
				},
				SetCategory: "large",
			},
		},
	}

	engine, err := NewEngine(rs)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	rows := [][]string{
		{"2024-01-15", "Coffee Shop", "-5.00"},
		{"2024-01-16", "Electronics Store", "-500.00"},
	}

	result := engine.ProcessRows(rows)
	
	if len(result.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(result.Transactions))
	}

	// First should be categorized as coffee
	txn1 := result.Transactions[0]
	if txn1.Postings[1].Account != "expenses:dining:coffee" {
		t.Errorf("expected expenses:dining:coffee, got %q", txn1.Postings[1].Account)
	}
	if txn1.Tags["category"] != "coffee" {
		t.Errorf("expected category=coffee tag, got %q", txn1.Tags["category"])
	}

	// Second should have large category
	txn2 := result.Transactions[1]
	if txn2.Tags["category"] != "large" {
		t.Errorf("expected category=large tag, got %q", txn2.Tags["category"])
	}
}

func TestEngineSkipsEmptyAmount(t *testing.T) {
	rs := &RuleSet{
		Name:          "test",
		SourceAccount: "assets:checking",
		Currency:      "USD",
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "description", Direct: &DirectMapping{Column: 1}},
			{Field: "amount", Direct: &DirectMapping{Column: 2}},
		},
	}

	engine, err := NewEngine(rs)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	rows := [][]string{
		{"2024-01-15", "Valid", "-5.00"},
		{"2024-01-16", "Empty amount", ""},
		{"2024-01-17", "Zero", "0"},
	}

	result := engine.ProcessRows(rows)
	
	// Should only have 1 transaction (empty and zero amounts skipped)
	if len(result.Transactions) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(result.Transactions))
	}
}

// ---------------------------------------------------------------------------
// YAML Loading/Saving Tests
// ---------------------------------------------------------------------------

func TestSaveAndLoadRuleSet(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.rules.yaml")

	original := &RuleSet{
		Name:                 "test-bank",
		Description:          "Test bank import rules",
		Version:              "1",
		SourceAccount:        "assets:checking:test",
		DefaultDebitAccount:  "expenses:unknown",
		DefaultCreditAccount: "income:unknown",
		Currency:             "USD",
		Format: FileFormat{
			Type:      "csv",
			Delimiter: ",",
			Encoding:  "utf-8",
			SkipLines: 1,
		},
		Mappings: []FieldMapping{
			{Field: "date", Direct: &DirectMapping{Column: 0}},
			{Field: "description", Direct: &DirectMapping{Column: 1}},
			{Field: "amount", Direct: &DirectMapping{Column: 2}},
		},
		Categories: []CategoryRule{
			{
				Name: "groceries",
				Match: CategoryMatch{
					DescriptionContains: []string{"grocery", "supermarket"},
				},
				SetAccount: "expenses:groceries",
			},
		},
	}

	// Save
	if err := SaveRuleSet(original, path); err != nil {
		t.Fatalf("failed to save ruleset: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("ruleset file was not created")
	}

	// Load
	loaded, err := LoadRuleSet(path)
	if err != nil {
		t.Fatalf("failed to load ruleset: %v", err)
	}

	// Verify
	if loaded.Name != original.Name {
		t.Errorf("name mismatch: got %q, want %q", loaded.Name, original.Name)
	}
	if loaded.SourceAccount != original.SourceAccount {
		t.Errorf("source_account mismatch: got %q, want %q", loaded.SourceAccount, original.SourceAccount)
	}
	if len(loaded.Mappings) != len(original.Mappings) {
		t.Errorf("mappings count mismatch: got %d, want %d", len(loaded.Mappings), len(original.Mappings))
	}
	if len(loaded.Categories) != len(original.Categories) {
		t.Errorf("categories count mismatch: got %d, want %d", len(loaded.Categories), len(original.Categories))
	}
}

// ---------------------------------------------------------------------------
// Helper Functions
// ---------------------------------------------------------------------------

func floatPtr(f float64) *float64 {
	return &f
}

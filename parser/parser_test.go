package Parser

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Basic transaction
// ---------------------------------------------------------------------------

func TestBasicTransaction(t *testing.T) {
	input := "2025-01-15 Grocery Store\n    expenses:groceries    $45.32\n    assets:checking      -$45.32\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
	txn := txns[0]
	if txn.Date.Format("2006-01-02") != "2025-01-15" {
		t.Errorf("wrong date: %s", txn.Date.Format("2006-01-02"))
	}
	if txn.Description != "Grocery Store" {
		t.Errorf("wrong description: %q", txn.Description)
	}
	if len(txn.Postings) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(txn.Postings))
	}
	if txn.Postings[0].Amount.Value != 45.32 {
		t.Errorf("posting[0] amount: got %.2f, want 45.32", txn.Postings[0].Amount.Value)
	}
	if txn.Postings[1].Amount.Value != -45.32 {
		t.Errorf("posting[1] amount: got %.2f, want -45.32", txn.Postings[1].Amount.Value)
	}
}

// ---------------------------------------------------------------------------
// Implied (omitted) amount
// ---------------------------------------------------------------------------

func TestImpliedAmount(t *testing.T) {
	input := "2025-01-15 Grocery Store\n    expenses:groceries    $45.32\n    assets:checking\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(txns))
	}
	posting := txns[0].Postings[1]
	if posting.AmountOmitted {
		t.Error("AmountOmitted should be false after FillImpliedAmounts")
	}
	if posting.Amount.Value != -45.32 {
		t.Errorf("implied amount: got %.4f, want -45.32", posting.Amount.Value)
	}
	if posting.Amount.Currency != "USD" {
		t.Errorf("implied currency: got %q, want USD", posting.Amount.Currency)
	}
}

// Two omitted amounts must produce an error.
func TestTwoImpliedAmountsError(t *testing.T) {
	input := "2025-01-15 Bad\n    expenses:food\n    assets:cash\n"
	_, err := ParseTransactions(input)
	if err == nil {
		t.Fatal("expected error for two implied amounts, got nil")
	}
}

// ---------------------------------------------------------------------------
// Status markers
// ---------------------------------------------------------------------------

func TestStatusCleared(t *testing.T) {
	input := "2025-01-15 * Cleared payment\n    expenses:food    $10.00\n    assets:cash     -$10.00\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txns[0].Status != "*" {
		t.Errorf("status: got %q, want \"*\"", txns[0].Status)
	}
}

func TestStatusPending(t *testing.T) {
	input := "2025-01-15 ! Pending payment\n    expenses:food    $10.00\n    assets:cash     -$10.00\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txns[0].Status != "!" {
		t.Errorf("status: got %q, want \"!\"", txns[0].Status)
	}
}

func TestNoStatus(t *testing.T) {
	input := "2025-01-15 Uncleared\n    expenses:food    $10.00\n    assets:cash     -$10.00\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txns[0].Status != "" {
		t.Errorf("expected empty status, got %q", txns[0].Status)
	}
}

// ---------------------------------------------------------------------------
// Tags and IDs from comments
// ---------------------------------------------------------------------------

func TestTransactionTags(t *testing.T) {
	input := "2025-01-15 Grocery Store\n    ; category: food\n    ; id: abc123\n    expenses:groceries    $45.32\n    assets:checking      -$45.32\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	txn := txns[0]
	if txn.ID != "abc123" {
		t.Errorf("id: got %q, want \"abc123\"", txn.ID)
	}
	if txn.Tags["category"] != "food" {
		t.Errorf("tag category: got %q, want \"food\"", txn.Tags["category"])
	}
}

func TestPostingInlineTags(t *testing.T) {
	input := "2025-01-15 Grocery Store\n    expenses:groceries    $45.32  ; merchant: billa\n    assets:checking      -$45.32\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p0 := txns[0].Postings[0]
	if p0.Tags["merchant"] != "billa" {
		t.Errorf("posting tag merchant: got %q, want \"billa\"", p0.Tags["merchant"])
	}
}

// ---------------------------------------------------------------------------
// Multi-transaction files with comments between them
// ---------------------------------------------------------------------------

func TestCommentBetweenTransactions(t *testing.T) {
	input := "2025-01-15 First\n    expenses:food    $10.00\n    assets:cash     -$10.00\n\n; This comment is between transactions\n\n2025-01-16 Second\n    expenses:food    $5.00\n    assets:cash     -$5.00\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(txns))
	}
}

func TestMultipleTransactions(t *testing.T) {
	input := "2025-01-15 A\n    expenses:food  $10\n    assets:cash   -$10\n\n2025-01-16 B\n    expenses:food  $5\n    assets:cash   -$5\n\n2025-01-17 C\n    expenses:food  $3\n    assets:cash   -$3\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 3 {
		t.Fatalf("expected 3 transactions, got %d", len(txns))
	}
}

// ---------------------------------------------------------------------------
// Multi-currency
// ---------------------------------------------------------------------------

func TestMultiCurrencyBGN(t *testing.T) {
	input := "2025-01-15 Foreign Purchase\n    expenses:travel    100 BGN\n    assets:checking   -100 BGN\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txns[0].Postings[0].Amount.Currency != "BGN" {
		t.Errorf("currency: got %q, want BGN", txns[0].Postings[0].Amount.Currency)
	}
	if txns[0].Postings[0].Amount.Value != 100 {
		t.Errorf("amount: got %.2f, want 100", txns[0].Postings[0].Amount.Value)
	}
}

func TestMultiCurrencyGBP(t *testing.T) {
	input := "2025-01-15 UK Purchase\n    expenses:travel    £50.00\n    assets:checking   -£50.00\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if txns[0].Postings[0].Amount.Currency != "GBP" {
		t.Errorf("currency: got %q, want GBP", txns[0].Postings[0].Amount.Currency)
	}
}

// ---------------------------------------------------------------------------
// Balance validation
// ---------------------------------------------------------------------------

func TestUnbalancedTransactionError(t *testing.T) {
	input := "2025-01-15 Unbalanced\n    expenses:food    $10.00\n    assets:cash     -$9.00\n"
	_, err := ParseTransactions(input)
	if err == nil {
		t.Fatal("expected error for unbalanced transaction, got nil")
	}
}

// ---------------------------------------------------------------------------
// Real example file (comment inline in journal)
// ---------------------------------------------------------------------------

func TestExampleJournalWithComment(t *testing.T) {
	// Reproduces the pattern in example/transactions.journal where a standalone
	// "; This is a comment" line appears between transactions.
	input := "2025-01-15 A\n    expenses:food  $10\n    assets:cash   -$10\n\n; This is a comment\n2025-01-16 B\n    expenses:food  $5\n    assets:cash   -$5\n"
	txns, err := ParseTransactions(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(txns) != 2 {
		t.Fatalf("expected 2, got %d", len(txns))
	}
}

// ---------------------------------------------------------------------------
// parseTagsFromComment unit tests
// ---------------------------------------------------------------------------

func TestParseTagsFromComment(t *testing.T) {
	cases := []struct {
		input    string
		wantID   string
		wantKey  string
		wantVal  string
		wantRest string
	}{
		{"; id: abc123def", "abc123def", "", "", ""},
		{"; category: food", "", "category", "food", ""},
		{"; merchant: billa", "", "merchant", "billa", ""},
		{"; just a plain note", "", "", "", "just a plain note"},
		{";no-leading-space: value", "", "no-leading-space", "value", ""},
	}

	for _, tc := range cases {
		id, tags, rest := parseTagsFromComment(tc.input)
		if id != tc.wantID {
			t.Errorf("input %q: id got %q, want %q", tc.input, id, tc.wantID)
		}
		if tc.wantKey != "" && tags[tc.wantKey] != tc.wantVal {
			t.Errorf("input %q: tags[%q] got %q, want %q", tc.input, tc.wantKey, tags[tc.wantKey], tc.wantVal)
		}
		if rest != tc.wantRest {
			t.Errorf("input %q: rest got %q, want %q", tc.input, rest, tc.wantRest)
		}
	}
}

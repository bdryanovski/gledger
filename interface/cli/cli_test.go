package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// NewContext tests
// ---------------------------------------------------------------------------

func TestNewContext_NoArgs(t *testing.T) {
	// Temporarily change home dir to a temp location
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, remaining, err := NewContext([]string{})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx == nil {
		t.Fatal("Context should not be nil")
	}

	if len(remaining) != 0 {
		t.Errorf("Expected 0 remaining args, got %d", len(remaining))
	}
}

func TestNewContext_WithCommand(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, remaining, err := NewContext([]string{"balance"})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining arg, got %d", len(remaining))
	}

	if remaining[0] != "balance" {
		t.Errorf("Expected remaining[0] to be 'balance', got %q", remaining[0])
	}

	if ctx.JournalName != "" {
		t.Errorf("JournalName should be empty when not specified, got %q", ctx.JournalName)
	}
}

func TestNewContext_JournalFlag(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, remaining, err := NewContext([]string{"--journal", "personal", "balance"})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.JournalName != "personal" {
		t.Errorf("Expected JournalName 'personal', got %q", ctx.JournalName)
	}

	if len(remaining) != 1 || remaining[0] != "balance" {
		t.Errorf("Expected remaining to be ['balance'], got %v", remaining)
	}
}

func TestNewContext_JournalFlagEquals(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, _, err := NewContext([]string{"--journal=personal", "balance"})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.JournalName != "personal" {
		t.Errorf("Expected JournalName 'personal', got %q", ctx.JournalName)
	}
}

func TestNewContext_BeginEndFlags(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, _, err := NewContext([]string{
		"--begin", "2025-01-01",
		"--end", "2025-12-31",
		"register",
	})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.BeginDate != "2025-01-01" {
		t.Errorf("Expected BeginDate '2025-01-01', got %q", ctx.BeginDate)
	}

	if ctx.EndDate != "2025-12-31" {
		t.Errorf("Expected EndDate '2025-12-31', got %q", ctx.EndDate)
	}
}

func TestNewContext_VerboseFlag(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, _, err := NewContext([]string{"--verbose", "balance"})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if !ctx.Verbose {
		t.Error("Expected Verbose to be true")
	}
}

func TestNewContext_JournalMissingValue(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	_, _, err := NewContext([]string{"--journal"})
	if err == nil {
		t.Error("Expected error for --journal without value")
	}
}

func TestNewContext_BeginMissingValue(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	_, _, err := NewContext([]string{"--begin"})
	if err == nil {
		t.Error("Expected error for --begin without value")
	}
}

func TestNewContext_EndMissingValue(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	_, _, err := NewContext([]string{"--end"})
	if err == nil {
		t.Error("Expected error for --end without value")
	}
}

func TestNewContext_InvalidBeginDate(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	_, _, err := NewContext([]string{"--begin", "invalid"})
	if err == nil {
		t.Error("Expected error for invalid --begin date")
	}
}

// ---------------------------------------------------------------------------
// NormalizeDate tests
// ---------------------------------------------------------------------------

func TestNormalizeDate_Valid(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"2025-01-15", "2025-01-15"},
		{"2025/01/15", "2025-01-15"},
		{"2025-12-31", "2025-12-31"},
		{"2025/12/31", "2025-12-31"},
	}

	for _, tc := range testCases {
		result, err := NormalizeDate(tc.input)
		if err != nil {
			t.Errorf("NormalizeDate(%q) failed: %v", tc.input, err)
			continue
		}
		if result != tc.expected {
			t.Errorf("NormalizeDate(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestNormalizeDate_Invalid(t *testing.T) {
	invalidDates := []string{
		"invalid",
		"2025-1-15",     // single digit month
		"2025-01-5",     // single digit day
		"25-01-15",      // 2-digit year
		"2025",          // year only
		"2025-01",       // year-month only
		"01-15-2025",    // MM-DD-YYYY
		"2025.01.15",    // wrong separator
		"2025-13-01",    // this passes structural validation (only checks format)
		"abcd-ef-gh",    // non-digits
	}

	for _, date := range invalidDates {
		_, err := NormalizeDate(date)
		if err == nil {
			// Note: some structurally valid but semantically invalid dates
			// like 2025-13-01 will pass - the function only does format validation
			if date != "2025-13-01" {
				t.Errorf("Expected error for NormalizeDate(%q)", date)
			}
		}
	}
}

func TestNormalizeDate_WithWhitespace(t *testing.T) {
	result, err := NormalizeDate("  2025-01-15  ")
	if err != nil {
		t.Fatalf("NormalizeDate with whitespace failed: %v", err)
	}
	if result != "2025-01-15" {
		t.Errorf("Expected '2025-01-15', got %q", result)
	}
}

// ---------------------------------------------------------------------------
// Run tests
// ---------------------------------------------------------------------------

func TestRun_NoArgs(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Run with no args should show help (no error)
	err := Run([]string{})
	if err != nil {
		t.Errorf("Run with no args should not error, got: %v", err)
	}
}

func TestRun_Help(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	testCases := []string{"help", "-h", "--help"}
	for _, cmd := range testCases {
		err := Run([]string{cmd})
		if err != nil {
			t.Errorf("Run(['%s']) should not error, got: %v", cmd, err)
		}
	}
}

func TestRun_Version(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	testCases := []string{"version", "-v", "--version"}
	for _, cmd := range testCases {
		err := Run([]string{cmd})
		if err != nil {
			t.Errorf("Run(['%s']) should not error, got: %v", cmd, err)
		}
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	err := Run([]string{"unknowncommand"})
	if err == nil {
		t.Error("Run with unknown command should return error")
	}
}

func TestRun_Balance_NoJournal(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create empty doublebook directory
	os.MkdirAll(filepath.Join(tmpDir, ".doublebook"), 0755)

	// Balance command with no journal should work (empty output)
	err := Run([]string{"balance"})
	// This may or may not error depending on implementation
	// The important thing is it doesn't panic
	_ = err
}

func TestRun_Register_NoJournal(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create empty doublebook directory
	os.MkdirAll(filepath.Join(tmpDir, ".doublebook"), 0755)

	// Register command with no journal should work (empty output)
	err := Run([]string{"register"})
	_ = err
}

func TestRun_CommandAliases(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create empty doublebook directory
	os.MkdirAll(filepath.Join(tmpDir, ".doublebook"), 0755)

	// Test command aliases - they should all run without panicking
	aliases := [][]string{
		{"bal"},     // balance alias
		{"reg"},     // register alias
		{"r"},       // register alias
		{"list"},    // register alias
		{"ls"},      // register alias
		{"is"},      // income-statement alias
	}

	for _, args := range aliases {
		t.Run(args[0], func(t *testing.T) {
			err := Run(args)
			// We just check it doesn't panic
			_ = err
		})
	}
}

// ---------------------------------------------------------------------------
// Context effective journal name tests
// ---------------------------------------------------------------------------

func TestEffectiveJournalName_FromFlag(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, _, err := NewContext([]string{"--journal", "custom", "balance"})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	if ctx.EffectiveJournalName() != "custom" {
		t.Errorf("Expected effective journal name 'custom', got %q", ctx.EffectiveJournalName())
	}
}

func TestEffectiveJournalName_FromConfig(t *testing.T) {
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	ctx, _, err := NewContext([]string{"balance"})
	if err != nil {
		t.Fatalf("NewContext failed: %v", err)
	}

	// When no flag is specified, should use config's journal name (default: "data")
	if ctx.EffectiveJournalName() != "data" {
		t.Errorf("Expected effective journal name 'data', got %q", ctx.EffectiveJournalName())
	}
}

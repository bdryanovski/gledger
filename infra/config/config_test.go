package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// DefaultConfig tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}

	// Check essential defaults
	if cfg.JournalName != "data" {
		t.Errorf("Expected JournalName 'data', got %q", cfg.JournalName)
	}

	if cfg.Currency != "USD" {
		t.Errorf("Expected Currency 'USD', got %q", cfg.Currency)
	}

	if cfg.APIPort != 5555 {
		t.Errorf("Expected APIPort 5555, got %d", cfg.APIPort)
	}

	if cfg.WebPort != 4444 {
		t.Errorf("Expected WebPort 4444, got %d", cfg.WebPort)
	}

	if cfg.DateFormat != "2006-01-02" {
		t.Errorf("Expected DateFormat '2006-01-02', got %q", cfg.DateFormat)
	}
}

func TestDefaultConfig_DataDir(t *testing.T) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	expectedDir := filepath.Join(home, ".doublebook")
	if cfg.DataDir != expectedDir {
		t.Errorf("Expected DataDir %q, got %q", expectedDir, cfg.DataDir)
	}
}

func TestDefaultConfig_DataFile(t *testing.T) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	expectedFile := filepath.Join(home, ".doublebook", "data.journal")
	if cfg.DataFile != expectedFile {
		t.Errorf("Expected DataFile %q, got %q", expectedFile, cfg.DataFile)
	}
}

func TestDefaultConfig_Aliases(t *testing.T) {
	cfg := DefaultConfig()

	expectedAliases := map[string]string{
		"exp":  "expenses",
		"inc":  "income",
		"ast":  "assets",
		"liab": "liabilities",
		"eq":   "equity",
	}

	for alias, expected := range expectedAliases {
		actual, ok := cfg.Aliases[alias]
		if !ok {
			t.Errorf("Missing alias %q", alias)
			continue
		}
		if actual != expected {
			t.Errorf("Alias %q: expected %q, got %q", alias, expected, actual)
		}
	}
}

func TestDefaultConfig_CreditNormalPrefixes(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.CreditNormalPrefixes) == 0 {
		t.Error("CreditNormalPrefixes should not be empty")
	}

	// Check for essential prefixes
	expectedPrefixes := []string{"income", "liabilities", "equity"}
	for _, expected := range expectedPrefixes {
		found := false
		for _, prefix := range cfg.CreditNormalPrefixes {
			if prefix == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing expected credit normal prefix: %q", expected)
		}
	}
}

func TestDefaultConfig_Theme(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Theme.PrimaryColor == "" {
		t.Error("Theme.PrimaryColor should not be empty")
	}

	if cfg.Theme.BorderStyle == "" {
		t.Error("Theme.BorderStyle should not be empty")
	}
}

// ---------------------------------------------------------------------------
// DataDirPath tests
// ---------------------------------------------------------------------------

func TestDataDirPath_NoTilde(t *testing.T) {
	cfg := &Config{
		DataDir: "/absolute/path/to/data",
	}

	result := cfg.DataDirPath()
	if result != "/absolute/path/to/data" {
		t.Errorf("Expected path unchanged, got %q", result)
	}
}

func TestDataDirPath_WithTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	cfg := &Config{
		DataDir: "~/.doublebook",
	}

	result := cfg.DataDirPath()
	expected := filepath.Join(home, ".doublebook")

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// ---------------------------------------------------------------------------
// CLIContext tests
// ---------------------------------------------------------------------------

func TestCLIContext_EffectiveJournalName_FromFlag(t *testing.T) {
	ctx := &CLIContext{
		Config: &Config{
			JournalName: "default",
		},
		JournalName: "custom",
	}

	result := ctx.EffectiveJournalName()
	if result != "custom" {
		t.Errorf("Expected 'custom' from flag, got %q", result)
	}
}

func TestCLIContext_EffectiveJournalName_FromConfig(t *testing.T) {
	ctx := &CLIContext{
		Config: &Config{
			JournalName: "default",
		},
		JournalName: "", // empty flag
	}

	result := ctx.EffectiveJournalName()
	if result != "default" {
		t.Errorf("Expected 'default' from config, got %q", result)
	}
}

// ---------------------------------------------------------------------------
// LoadConfig and Save tests (using temp directory)
// ---------------------------------------------------------------------------

func TestLoadConfig_FileNotExists(t *testing.T) {
	// Temporarily change home dir to a temp location
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadConfig returned nil")
	}

	// Should return default config
	if cfg.JournalName != "data" {
		t.Errorf("Expected default JournalName 'data', got %q", cfg.JournalName)
	}
}

func TestConfig_Save(t *testing.T) {
	// Temporarily change home dir to a temp location
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := DefaultConfig()
	cfg.JournalName = "test-journal"

	err := cfg.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ".doublebook", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestLoadConfig_ExistingFile(t *testing.T) {
	// Temporarily change home dir to a temp location
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create config directory and file
	configDir := filepath.Join(tmpDir, ".doublebook")
	os.MkdirAll(configDir, 0755)

	configContent := `
journal_name: my-journal
currency: EUR
api_port: 8080
web_port: 8081
`
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.JournalName != "my-journal" {
		t.Errorf("Expected JournalName 'my-journal', got %q", cfg.JournalName)
	}

	if cfg.Currency != "EUR" {
		t.Errorf("Expected Currency 'EUR', got %q", cfg.Currency)
	}

	if cfg.APIPort != 8080 {
		t.Errorf("Expected APIPort 8080, got %d", cfg.APIPort)
	}

	if cfg.WebPort != 8081 {
		t.Errorf("Expected WebPort 8081, got %d", cfg.WebPort)
	}
}

func TestLoadConfig_BackfillsDefaults(t *testing.T) {
	// Temporarily change home dir to a temp location
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create config directory and file with minimal content
	configDir := filepath.Join(tmpDir, ".doublebook")
	os.MkdirAll(configDir, 0755)

	// Partial config - missing many fields
	configContent := `
journal_name: ""
`
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Should backfill missing values
	if cfg.JournalName != "data" {
		t.Errorf("Expected backfilled JournalName 'data', got %q", cfg.JournalName)
	}

	if cfg.APIPort != 5555 {
		t.Errorf("Expected backfilled APIPort 5555, got %d", cfg.APIPort)
	}

	if cfg.WebPort != 4444 {
		t.Errorf("Expected backfilled WebPort 4444, got %d", cfg.WebPort)
	}

	if cfg.Currency != "USD" {
		t.Errorf("Expected backfilled Currency 'USD', got %q", cfg.Currency)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Temporarily change home dir to a temp location
	originalHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create config directory and file with invalid YAML
	configDir := filepath.Join(tmpDir, ".doublebook")
	os.MkdirAll(configDir, 0755)

	configContent := `
invalid yaml content [[[
`
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	_, err := LoadConfig()
	if err == nil {
		t.Error("LoadConfig should fail for invalid YAML")
	}
}

// ---------------------------------------------------------------------------
// ThemeConfig tests
// ---------------------------------------------------------------------------

func TestThemeConfig_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Theme.PrimaryColor != "#7C3AED" {
		t.Errorf("Expected default PrimaryColor '#7C3AED', got %q", cfg.Theme.PrimaryColor)
	}

	if cfg.Theme.BorderStyle != "rounded" {
		t.Errorf("Expected default BorderStyle 'rounded', got %q", cfg.Theme.BorderStyle)
	}
}

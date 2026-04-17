// Package config manages DoubleBook's persistent configuration and the
// per-invocation CLI context (global flags).
package config

import (
	"os"
	"path/filepath"

	"doublebook/utils"
	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Config — persistent settings (stored in ~/.doublebook/config.yaml)
// ---------------------------------------------------------------------------

// Config holds all persistent settings for DoubleBook.
type Config struct {
	// Journal settings
	DataFile    string `yaml:"data_file"`    // full path to the primary journal file
	DataDir     string `yaml:"data_dir"`     // directory that holds all journal files
	JournalName string `yaml:"journal_name"` // bare stem used for multi-file resolution ("data")

	// Display settings
	DateFormat string      `yaml:"date_format"` // Go time format, default "2006-01-02"
	Currency   string      `yaml:"currency"`    // base/reporting currency, default "USD"
	Theme      ThemeConfig `yaml:"theme"`

	// Server settings
	APIPort int `yaml:"api_port"` // REST API port, default 5555
	WebPort int `yaml:"web_port"` // Web UI port,  default 4444

	// Account aliases (e.g. "exp" → "expenses")
	Aliases map[string]string `yaml:"aliases"`

	// Plugin names to auto-load on startup
	Plugins []string `yaml:"plugins"`

	// CreditNormalPrefixes lists account name prefixes whose balances are
	// "healthy" (shown green) when NEGATIVE. All other accounts are treated
	// as debit-normal (healthy when positive).
	//
	// Edit this list to support non-English account names, custom conventions,
	// or additional account types without rebuilding the binary.
	//
	// Default: ["income", "liabilities", "equity"]
	CreditNormalPrefixes []string `yaml:"credit_normal_prefixes"`
}

// ThemeConfig holds visual styling preferences.
type ThemeConfig struct {
	PrimaryColor string `yaml:"primary_color"`
	BorderStyle  string `yaml:"border_style"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".doublebook")
	return &Config{
		DataDir:     dataDir,
		DataFile:    filepath.Join(dataDir, "data.journal"),
		JournalName: "data",
		DateFormat:  "2006-01-02",
		Currency:    "USD",
		APIPort:     5555,
		WebPort:     4444,
		Aliases: map[string]string{
			"exp":  "expenses",
			"inc":  "income",
			"ast":  "assets",
			"liab": "liabilities",
			"eq":   "equity",
		},
		Theme: ThemeConfig{
			PrimaryColor: "#7C3AED",
			BorderStyle:  "rounded",
		},
		CreditNormalPrefixes: []string{
			"income",
			"liabilities",
			"equity",
			"revenue",  // common alternative to "income"
			"revenues", // plural form
		},
	}
}

// DataDirPath returns DataDir with any "~/" prefix expanded to the real home
// directory path.
func (c *Config) DataDirPath() string {
	return utils.ExpandHome(c.DataDir)
}

// LoadConfig reads the config file at ~/.doublebook/config.yaml.
// If the file does not exist a default config is created and returned.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	configPath := filepath.Join(home, ".doublebook", "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			// Best-effort save; ignore errors so first-run always succeeds.
			_ = cfg.Save()
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Back-fill any fields that were absent in older config files.
	if cfg.JournalName == "" {
		cfg.JournalName = "data"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = filepath.Join(home, ".doublebook")
	}
	if cfg.APIPort == 0 {
		cfg.APIPort = 5555
	}
	if cfg.WebPort == 0 {
		cfg.WebPort = 4444
	}
	if cfg.Currency == "" {
		cfg.Currency = "USD"
	}
	if len(cfg.CreditNormalPrefixes) == 0 {
		cfg.CreditNormalPrefixes = []string{"income", "liabilities", "equity", "revenue", "revenues"}
	}

	return &cfg, nil
}

// Save writes the config to ~/.doublebook/config.yaml, creating the directory
// if needed.
func (c *Config) Save() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, ".doublebook")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644)
}

// ---------------------------------------------------------------------------
// CLIContext — per-invocation runtime state (global flags + config)
// ---------------------------------------------------------------------------

// CLIContext bundles the loaded Config with global flags parsed from the
// command line.  It is passed to every command handler.
//
// Defined here (in the config package) so both the cli and cli/commands
// packages can import it without creating a circular dependency.
type CLIContext struct {
	Config      *Config
	JournalName string // from --journal flag; overrides Config.JournalName when non-empty
	BeginDate   string // from --begin flag, normalised to YYYY-MM-DD
	EndDate     string // from --end flag,   normalised to YYYY-MM-DD
	Verbose     bool   // from --verbose flag
}

// EffectiveJournalName returns JournalName if set, otherwise Config.JournalName.
func (c *CLIContext) EffectiveJournalName() string {
	if c.JournalName != "" {
		return c.JournalName
	}
	return c.Config.JournalName
}

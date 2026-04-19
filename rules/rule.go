// Package rules provides a flexible Rules Engine for mapping CSV/Excel columns
// to DoubleBook transaction fields. It supports direct mappings, column
// combinations, transformations, conditionals, and lookups.
package rules

import (
	"fmt"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// RuleSet — top-level configuration
// ---------------------------------------------------------------------------

// RuleSet is the top-level configuration for importing data from a specific
// source (e.g., a bank's CSV export format).
type RuleSet struct {
	// Identity
	Name        string `yaml:"name" json:"name"`               // human-readable label
	Description string `yaml:"description" json:"description"` // what this ruleset is for
	Version     string `yaml:"version" json:"version"`         // ruleset version

	// File format settings
	Format FileFormat `yaml:"format" json:"format"`

	// Column definitions (discovered from file)
	Columns []ColumnDef `yaml:"columns" json:"columns"`

	// Field mappings
	Mappings []FieldMapping `yaml:"mappings" json:"mappings"`

	// Account defaults
	SourceAccount        string `yaml:"source_account" json:"source_account"`
	DefaultDebitAccount  string `yaml:"default_debit_account" json:"default_debit_account"`
	DefaultCreditAccount string `yaml:"default_credit_account" json:"default_credit_account"`

	// Currency
	Currency string `yaml:"currency" json:"currency"`

	// Post-import categorization rules
	Categories []CategoryRule `yaml:"categories" json:"categories"`
}

// FileFormat describes the source file format.
type FileFormat struct {
	Type       string `yaml:"type" json:"type"`             // "csv" or "excel"
	Delimiter  string `yaml:"delimiter" json:"delimiter"`   // for CSV: ",", ";", "\t"
	Encoding   string `yaml:"encoding" json:"encoding"`     // "utf-8", "windows-1252", etc.
	SkipLines  int    `yaml:"skip_lines" json:"skip_lines"` // header lines to skip
	SheetName  string `yaml:"sheet_name" json:"sheet_name"` // for Excel: which sheet to use
	SheetIndex int    `yaml:"sheet_index" json:"sheet_index"` // for Excel: sheet index (0-based)
}

// ColumnDef describes a column discovered in the source file.
type ColumnDef struct {
	Index   int      `yaml:"index" json:"index"`     // 0-based column index
	Name    string   `yaml:"name" json:"name"`       // column header name
	Samples []string `yaml:"samples" json:"samples"` // sample values from first N rows
}

// ---------------------------------------------------------------------------
// Field Mappings
// ---------------------------------------------------------------------------

// FieldMapping defines how to populate a transaction field.
type FieldMapping struct {
	// Target field in the transaction
	Field string `yaml:"field" json:"field"` // "date", "description", "amount", "debit_account", "credit_account", "tag:xxx"

	// Mapping type (only one should be set)
	Direct    *DirectMapping    `yaml:"direct,omitempty" json:"direct,omitempty"`
	Combine   *CombineMapping   `yaml:"combine,omitempty" json:"combine,omitempty"`
	Transform *TransformMapping `yaml:"transform,omitempty" json:"transform,omitempty"`
	Lookup    *LookupMapping    `yaml:"lookup,omitempty" json:"lookup,omitempty"`
	Constant  *ConstantMapping  `yaml:"constant,omitempty" json:"constant,omitempty"`
	Condition *ConditionalMapping `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// DirectMapping maps a single column directly to the field.
type DirectMapping struct {
	Column int `yaml:"column" json:"column"` // 0-based column index
}

// CombineMapping combines multiple columns using a format string.
type CombineMapping struct {
	Columns   []int  `yaml:"columns" json:"columns"`     // column indices to combine
	Format    string `yaml:"format" json:"format"`       // format string with {0}, {1}, etc. placeholders
	Separator string `yaml:"separator" json:"separator"` // simple separator (alternative to format)
	Trim      bool   `yaml:"trim" json:"trim"`           // trim whitespace from each column
}

// TransformMapping applies a transformation function to column(s).
type TransformMapping struct {
	Column    int               `yaml:"column" json:"column"`       // primary column index
	Columns   []int             `yaml:"columns" json:"columns"`     // for multi-column transforms
	Function  string            `yaml:"function" json:"function"`   // transform function name
	Args      map[string]string `yaml:"args" json:"args"`           // function arguments
}

// LookupMapping maps values using a lookup table.
type LookupMapping struct {
	Column   int               `yaml:"column" json:"column"`   // column to look up
	Table    map[string]string `yaml:"table" json:"table"`     // value -> replacement
	Default  string            `yaml:"default" json:"default"` // value if not found
	CaseSensitive bool         `yaml:"case_sensitive" json:"case_sensitive"`
}

// ConstantMapping sets a constant value.
type ConstantMapping struct {
	Value string `yaml:"value" json:"value"`
}

// ConditionalMapping applies different mappings based on conditions.
type ConditionalMapping struct {
	Conditions []ConditionBranch `yaml:"conditions" json:"conditions"`
	Default    *FieldMapping     `yaml:"default,omitempty" json:"default,omitempty"`
}

// ConditionBranch represents one if-then branch.
type ConditionBranch struct {
	When    Condition     `yaml:"when" json:"when"`
	Mapping *FieldMapping `yaml:"mapping" json:"mapping"`
}

// Condition specifies match criteria.
type Condition struct {
	Column    int     `yaml:"column" json:"column"`       // column to check
	Contains  string  `yaml:"contains" json:"contains"`   // substring match
	Equals    string  `yaml:"equals" json:"equals"`       // exact match
	Regex     string  `yaml:"regex" json:"regex"`         // regex match
	GreaterThan *float64 `yaml:"greater_than" json:"greater_than"`
	LessThan    *float64 `yaml:"less_than" json:"less_than"`
	
	// Compiled regex (not serialized)
	compiledRegex *regexp.Regexp
}

// ---------------------------------------------------------------------------
// Category Rules (post-import)
// ---------------------------------------------------------------------------

// CategoryRule defines automatic categorization based on transaction content.
type CategoryRule struct {
	Name        string            `yaml:"name" json:"name"`               // rule name for debugging
	Match       CategoryMatch     `yaml:"match" json:"match"`             // match conditions
	SetAccount  string            `yaml:"set_account" json:"set_account"` // account to set
	SetTags     map[string]string `yaml:"set_tags" json:"set_tags"`       // tags to add
	SetCategory string            `yaml:"set_category" json:"set_category"` // shorthand for set_tags["category"]
}

// CategoryMatch defines what to match for categorization.
type CategoryMatch struct {
	DescriptionContains []string  `yaml:"description_contains" json:"description_contains"`
	DescriptionRegex    string    `yaml:"description_regex" json:"description_regex"`
	AmountMin           *float64  `yaml:"amount_min" json:"amount_min"`
	AmountMax           *float64  `yaml:"amount_max" json:"amount_max"`
	IsDebit             *bool     `yaml:"is_debit" json:"is_debit"` // true = expense, false = income
	
	// Compiled regex (not serialized)
	compiledRegex *regexp.Regexp
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Validate checks that the RuleSet is properly configured.
func (rs *RuleSet) Validate() error {
	var errs []string

	if rs.Name == "" {
		errs = append(errs, "name is required")
	}

	if rs.SourceAccount == "" {
		errs = append(errs, "source_account is required")
	}

	// Check required field mappings exist
	hasDate := false
	hasAmount := false
	for _, m := range rs.Mappings {
		if m.Field == "date" {
			hasDate = true
		}
		if m.Field == "amount" || m.Field == "debit_amount" || m.Field == "credit_amount" {
			hasAmount = true
		}
	}

	if !hasDate {
		errs = append(errs, "mapping for 'date' field is required")
	}
	if !hasAmount {
		errs = append(errs, "mapping for amount field is required (amount, debit_amount, or credit_amount)")
	}

	// Validate each mapping
	for i, m := range rs.Mappings {
		if err := m.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("mapping[%d]: %v", i, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Validate checks that a FieldMapping is properly configured.
func (m *FieldMapping) Validate() error {
	if m.Field == "" {
		return fmt.Errorf("field name is required")
	}

	// Count how many mapping types are set
	count := 0
	if m.Direct != nil {
		count++
	}
	if m.Combine != nil {
		count++
	}
	if m.Transform != nil {
		count++
	}
	if m.Lookup != nil {
		count++
	}
	if m.Constant != nil {
		count++
	}
	if m.Condition != nil {
		count++
	}

	if count == 0 {
		return fmt.Errorf("no mapping type specified for field %q", m.Field)
	}
	if count > 1 {
		return fmt.Errorf("multiple mapping types specified for field %q (only one allowed)", m.Field)
	}

	return nil
}

// Compile prepares any regex patterns in the RuleSet for efficient matching.
func (rs *RuleSet) Compile() error {
	for i := range rs.Mappings {
		if rs.Mappings[i].Condition != nil {
			for j := range rs.Mappings[i].Condition.Conditions {
				cond := &rs.Mappings[i].Condition.Conditions[j].When
				if cond.Regex != "" {
					re, err := regexp.Compile(cond.Regex)
					if err != nil {
						return fmt.Errorf("invalid regex in mapping[%d].condition[%d]: %w", i, j, err)
					}
					cond.compiledRegex = re
				}
			}
		}
	}

	for i := range rs.Categories {
		cat := &rs.Categories[i]
		if cat.Match.DescriptionRegex != "" {
			re, err := regexp.Compile(cat.Match.DescriptionRegex)
			if err != nil {
				return fmt.Errorf("invalid regex in category[%d]: %w", i, err)
			}
			cat.Match.compiledRegex = re
		}
	}

	return nil
}

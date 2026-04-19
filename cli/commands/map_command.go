package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"doublebook/config"
	"doublebook/rules"
)

// MapCommand runs the interactive column mapper for CSV/Excel files.
func MapCommand(ctx *config.CLIContext, args []string) error {
	fs := flag.NewFlagSet("map", flag.ContinueOnError)
	outputPath := fs.String("output", "", "Output path for the rules file (default: ~/.doublebook/rules/<name>.rules.yaml)")
	listRules := fs.Bool("list", false, "List existing rules")
	showRule := fs.String("show", "", "Show details of a rule file")
	
	if err := fs.Parse(args); err != nil {
		return err
	}

	// List rules
	if *listRules {
		return listExistingRules()
	}

	// Show rule details
	if *showRule != "" {
		return showRuleDetails(*showRule)
	}

	// Need a file argument for mapping
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: doublebook map <file.csv|file.xlsx> [--output path]")
	}

	filePath := fs.Arg(0)
	
	// Check file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Run interactive mapper
	fmt.Printf("Opening interactive mapper for: %s\n", filePath)
	fmt.Println("Press any key to continue...")
	
	ruleSet, err := rules.RunMapper(filePath)
	if err != nil {
		return err
	}

	// Optionally save to custom path
	if *outputPath != "" {
		if err := rules.SaveRuleSet(ruleSet, *outputPath); err != nil {
			return fmt.Errorf("saving rules: %w", err)
		}
		fmt.Printf("Rules saved to: %s\n", *outputPath)
	} else {
		fmt.Printf("Rules saved to: %s\n", filepath.Join(rules.DefaultRulesDir(), ruleSet.Name+".rules.yaml"))
	}

	return nil
}

func listExistingRules() error {
	dir := rules.DefaultRulesDir()
	files, err := rules.ListRuleSets(dir)
	if err != nil {
		return fmt.Errorf("listing rules: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No rules found.")
		fmt.Printf("Rules directory: %s\n", dir)
		return nil
	}

	fmt.Println("Available import rules:")
	fmt.Println()
	for _, file := range files {
		rs, err := rules.LoadRuleSet(file)
		if err != nil {
			fmt.Printf("  %s (error: %v)\n", filepath.Base(file), err)
			continue
		}
		desc := rs.Description
		if desc == "" {
			desc = "No description"
		}
		fmt.Printf("  %-20s %s\n", rs.Name, desc)
	}
	fmt.Println()
	fmt.Printf("Rules directory: %s\n", dir)
	return nil
}

func showRuleDetails(name string) error {
	path, err := rules.FindRuleSet(name)
	if err != nil {
		return err
	}

	rs, err := rules.LoadRuleSet(path)
	if err != nil {
		return fmt.Errorf("loading rule: %w", err)
	}

	fmt.Printf("Rule: %s\n", rs.Name)
	fmt.Printf("File: %s\n", path)
	if rs.Description != "" {
		fmt.Printf("Description: %s\n", rs.Description)
	}
	fmt.Println()

	fmt.Println("Settings:")
	fmt.Printf("  Source account: %s\n", rs.SourceAccount)
	fmt.Printf("  Default debit:  %s\n", rs.DefaultDebitAccount)
	fmt.Printf("  Default credit: %s\n", rs.DefaultCreditAccount)
	fmt.Printf("  Currency:       %s\n", rs.Currency)
	fmt.Println()

	fmt.Println("Format:")
	fmt.Printf("  Type:       %s\n", rs.Format.Type)
	fmt.Printf("  Delimiter:  %q\n", rs.Format.Delimiter)
	fmt.Printf("  Encoding:   %s\n", rs.Format.Encoding)
	fmt.Printf("  Skip lines: %d\n", rs.Format.SkipLines)
	fmt.Println()

	fmt.Println("Mappings:")
	for _, m := range rs.Mappings {
		mappingType := "unknown"
		if m.Direct != nil {
			mappingType = fmt.Sprintf("column %d", m.Direct.Column)
		} else if m.Combine != nil {
			mappingType = fmt.Sprintf("combine columns %v", m.Combine.Columns)
		} else if m.Transform != nil {
			mappingType = fmt.Sprintf("transform(%s)", m.Transform.Function)
		} else if m.Lookup != nil {
			mappingType = fmt.Sprintf("lookup column %d", m.Lookup.Column)
		} else if m.Constant != nil {
			mappingType = fmt.Sprintf("constant %q", m.Constant.Value)
		}
		fmt.Printf("  %-20s <- %s\n", m.Field, mappingType)
	}

	if len(rs.Categories) > 0 {
		fmt.Println()
		fmt.Println("Category rules:")
		for _, cat := range rs.Categories {
			fmt.Printf("  %s:\n", cat.Name)
			if len(cat.Match.DescriptionContains) > 0 {
				fmt.Printf("    Match: %v\n", cat.Match.DescriptionContains)
			}
			if cat.SetAccount != "" {
				fmt.Printf("    Set account: %s\n", cat.SetAccount)
			}
			if cat.SetCategory != "" {
				fmt.Printf("    Set category: %s\n", cat.SetCategory)
			}
		}
	}

	return nil
}

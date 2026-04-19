package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// TransformFunc is a function that transforms input value(s) to an output string.
type TransformFunc func(values []string, args map[string]string) (string, error)

// builtinTransforms contains all built-in transform functions.
var builtinTransforms = map[string]TransformFunc{
	// String transforms
	"uppercase":   transformUppercase,
	"lowercase":   transformLowercase,
	"titlecase":   transformTitlecase,
	"trim":        transformTrim,
	"replace":     transformReplace,
	"regex_extract": transformRegexExtract,
	"prefix":      transformPrefix,
	"suffix":      transformSuffix,
	"truncate":    transformTruncate,
	"clean":       transformClean,
	
	// Concatenation
	"concat":      transformConcat,
	"join":        transformJoin,
	"format":      transformFormat,
	
	// Date transforms
	"parse_date":  transformParseDate,
	"format_date": transformFormatDate,
	
	// Number transforms
	"parse_number": transformParseNumber,
	"abs":          transformAbs,
	"negate":       transformNegate,
	
	// Conditional
	"default":      transformDefault,
	"coalesce":     transformCoalesce,
	"if_empty":     transformIfEmpty,
	"if_contains":  transformIfContains,
	
	// Lookup
	"map":          transformMap,
	"extract_account": transformExtractAccount,
}

// GetTransform returns a transform function by name.
func GetTransform(name string) (TransformFunc, bool) {
	fn, ok := builtinTransforms[name]
	return fn, ok
}

// ListTransforms returns all available transform function names.
func ListTransforms() []string {
	names := make([]string, 0, len(builtinTransforms))
	for name := range builtinTransforms {
		names = append(names, name)
	}
	return names
}

// ---------------------------------------------------------------------------
// String Transforms
// ---------------------------------------------------------------------------

func transformUppercase(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	return strings.ToUpper(values[0]), nil
}

func transformLowercase(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	return strings.ToLower(values[0]), nil
}

func transformTitlecase(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	return strings.Title(strings.ToLower(values[0])), nil
}

func transformTrim(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	chars := args["chars"]
	if chars == "" {
		return strings.TrimSpace(values[0]), nil
	}
	return strings.Trim(values[0], chars), nil
}

func transformReplace(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	old := args["old"]
	new := args["new"]
	return strings.ReplaceAll(values[0], old, new), nil
}

func transformRegexExtract(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	pattern := args["pattern"]
	if pattern == "" {
		return "", fmt.Errorf("regex_extract requires 'pattern' argument")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}
	
	matches := re.FindStringSubmatch(values[0])
	if len(matches) == 0 {
		return args["default"], nil
	}
	
	// Return first capture group if exists, otherwise full match
	if len(matches) > 1 {
		return matches[1], nil
	}
	return matches[0], nil
}

func transformPrefix(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	prefix := args["prefix"]
	if values[0] == "" {
		return "", nil
	}
	return prefix + values[0], nil
}

func transformSuffix(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	suffix := args["suffix"]
	if values[0] == "" {
		return "", nil
	}
	return values[0] + suffix, nil
}

func transformTruncate(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	maxLen := 50
	if lenStr := args["length"]; lenStr != "" {
		if n, err := strconv.Atoi(lenStr); err == nil && n > 0 {
			maxLen = n
		}
	}
	s := values[0]
	if len(s) <= maxLen {
		return s, nil
	}
	ellipsis := args["ellipsis"]
	if ellipsis == "" {
		ellipsis = "..."
	}
	return s[:maxLen-len(ellipsis)] + ellipsis, nil
}

func transformClean(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	s := values[0]
	// Remove extra whitespace
	s = strings.Join(strings.Fields(s), " ")
	// Remove non-printable characters
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			return r
		}
		return -1
	}, s)
	return strings.TrimSpace(s), nil
}

// ---------------------------------------------------------------------------
// Concatenation Transforms
// ---------------------------------------------------------------------------

func transformConcat(values []string, args map[string]string) (string, error) {
	return strings.Join(values, ""), nil
}

func transformJoin(values []string, args map[string]string) (string, error) {
	separator := args["separator"]
	if separator == "" {
		separator = " "
	}
	// Filter out empty values
	nonEmpty := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(v))
		}
	}
	return strings.Join(nonEmpty, separator), nil
}

func transformFormat(values []string, args map[string]string) (string, error) {
	template := args["template"]
	if template == "" {
		return "", fmt.Errorf("format requires 'template' argument")
	}
	
	result := template
	for i, v := range values {
		placeholder := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, placeholder, v)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Date Transforms
// ---------------------------------------------------------------------------

func transformParseDate(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || values[0] == "" {
		return "", nil
	}
	
	inputFormat := args["input_format"]
	outputFormat := args["output_format"]
	if outputFormat == "" {
		outputFormat = "2006-01-02"
	}
	
	// Common date formats to try
	formats := []string{
		inputFormat,
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
		"02-01-2006",
		"01-02-2006",
		"02.01.2006",
		"2006.01.02",
		"Jan 2, 2006",
		"January 2, 2006",
		"2 Jan 2006",
		"2 January 2006",
	}
	
	var t time.Time
	var err error
	for _, fmt := range formats {
		if fmt == "" {
			continue
		}
		t, err = time.Parse(fmt, strings.TrimSpace(values[0]))
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", fmt.Errorf("cannot parse date %q", values[0])
	}
	
	return t.Format(outputFormat), nil
}

func transformFormatDate(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || values[0] == "" {
		return "", nil
	}
	
	outputFormat := args["format"]
	if outputFormat == "" {
		outputFormat = "2006-01-02"
	}
	
	// Parse as standard format first
	t, err := time.Parse("2006-01-02", strings.TrimSpace(values[0]))
	if err != nil {
		return "", fmt.Errorf("cannot parse date %q (expected YYYY-MM-DD)", values[0])
	}
	
	return t.Format(outputFormat), nil
}

// ---------------------------------------------------------------------------
// Number Transforms
// ---------------------------------------------------------------------------

func transformParseNumber(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || values[0] == "" {
		return "0", nil
	}
	
	s := values[0]
	
	// Remove currency symbols and spaces
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "$")
	s = strings.TrimPrefix(s, "€")
	s = strings.TrimPrefix(s, "£")
	s = strings.TrimPrefix(s, "лв")
	s = strings.TrimPrefix(s, "BGN")
	s = strings.TrimSpace(s)
	
	// Handle different decimal/thousands separators
	decimalSep := args["decimal"]
	thousandsSep := args["thousands"]
	
	if decimalSep == "" {
		// Auto-detect: if both . and , present, last one is decimal
		dotIdx := strings.LastIndex(s, ".")
		commaIdx := strings.LastIndex(s, ",")
		
		if dotIdx > commaIdx {
			// . is decimal separator
			s = strings.ReplaceAll(s, ",", "")
		} else if commaIdx > dotIdx {
			// , is decimal separator
			s = strings.ReplaceAll(s, ".", "")
			s = strings.ReplaceAll(s, ",", ".")
		} else {
			// Only one or neither
			s = strings.ReplaceAll(s, ",", "")
		}
	} else {
		if thousandsSep != "" {
			s = strings.ReplaceAll(s, thousandsSep, "")
		}
		if decimalSep != "." {
			s = strings.ReplaceAll(s, decimalSep, ".")
		}
	}
	
	// Handle parentheses for negative numbers: (100.00) -> -100.00
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + s[1:len(s)-1]
	}
	
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", fmt.Errorf("cannot parse number %q: %w", values[0], err)
	}
	
	return fmt.Sprintf("%.2f", f), nil
}

func transformAbs(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || values[0] == "" {
		return "0", nil
	}
	
	f, err := parseAmount(values[0])
	if err != nil {
		return "", fmt.Errorf("cannot parse number: %w", err)
	}
	
	if f < 0 {
		f = -f
	}
	return fmt.Sprintf("%.2f", f), nil
}

func transformNegate(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || values[0] == "" {
		return "0", nil
	}
	
	f, err := parseAmount(values[0])
	if err != nil {
		return "", fmt.Errorf("cannot parse number: %w", err)
	}
	
	return fmt.Sprintf("%.2f", -f), nil
}

// ---------------------------------------------------------------------------
// Conditional Transforms
// ---------------------------------------------------------------------------

func transformDefault(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return args["value"], nil
	}
	return values[0], nil
}

func transformCoalesce(values []string, args map[string]string) (string, error) {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v, nil
		}
	}
	return args["default"], nil
}

func transformIfEmpty(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return args["then"], nil
	}
	if elseVal := args["else"]; elseVal != "" {
		return elseVal, nil
	}
	return values[0], nil
}

func transformIfContains(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return args["else"], nil
	}
	
	search := args["search"]
	if search == "" {
		return "", fmt.Errorf("if_contains requires 'search' argument")
	}
	
	if strings.Contains(strings.ToLower(values[0]), strings.ToLower(search)) {
		return args["then"], nil
	}
	return args["else"], nil
}

// ---------------------------------------------------------------------------
// Lookup Transforms
// ---------------------------------------------------------------------------

func transformMap(values []string, args map[string]string) (string, error) {
	if len(values) == 0 {
		return args["default"], nil
	}
	
	key := strings.TrimSpace(values[0])
	if result, ok := args[key]; ok {
		return result, nil
	}
	
	// Try lowercase
	if result, ok := args[strings.ToLower(key)]; ok {
		return result, nil
	}
	
	return args["default"], nil
}

func transformExtractAccount(values []string, args map[string]string) (string, error) {
	if len(values) == 0 || values[0] == "" {
		return args["default"], nil
	}
	
	description := strings.ToLower(values[0])
	
	// Check each mapping
	for key, account := range args {
		if key == "default" {
			continue
		}
		// Key can be a comma-separated list of patterns
		patterns := strings.Split(key, ",")
		for _, pattern := range patterns {
			pattern = strings.TrimSpace(strings.ToLower(pattern))
			if pattern != "" && strings.Contains(description, pattern) {
				return account, nil
			}
		}
	}
	
	return args["default"], nil
}

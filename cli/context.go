package cli

import (
	"fmt"
	"strings"

	"doublebook/config"
)

// NewContext parses global flags out of args, loads the config, and returns
// the resulting CLIContext together with the remaining (non-global) args.
//
// Global flags consumed:
//
//	--journal NAME   override the journal stem
//	--begin DATE     filter start date (YYYY-MM-DD or YYYY/MM/DD)
//	--end DATE       filter end date
//	--verbose        enable verbose output
//
// All other args are returned unchanged in the second return value.
func NewContext(args []string) (*config.CLIContext, []string, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	ctx := &config.CLIContext{Config: cfg}
	remaining := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--journal" || arg == "-journal":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--journal requires a value")
			}
			i++
			ctx.JournalName = args[i]
			// Also propagate into config so downstream code that reads config directly
			// picks up the override.
			ctx.Config.JournalName = args[i]

		case strings.HasPrefix(arg, "--journal="):
			ctx.JournalName = strings.TrimPrefix(arg, "--journal=")
			ctx.Config.JournalName = ctx.JournalName

		case arg == "--begin" || arg == "-begin":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--begin requires a value")
			}
			i++
			d, err := NormalizeDate(args[i])
			if err != nil {
				return nil, nil, fmt.Errorf("--begin: %w", err)
			}
			ctx.BeginDate = d

		case strings.HasPrefix(arg, "--begin="):
			d, err := NormalizeDate(strings.TrimPrefix(arg, "--begin="))
			if err != nil {
				return nil, nil, fmt.Errorf("--begin: %w", err)
			}
			ctx.BeginDate = d

		case arg == "--end" || arg == "-end":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--end requires a value")
			}
			i++
			d, err := NormalizeDate(args[i])
			if err != nil {
				return nil, nil, fmt.Errorf("--end: %w", err)
			}
			ctx.EndDate = d

		case strings.HasPrefix(arg, "--end="):
			d, err := NormalizeDate(strings.TrimPrefix(arg, "--end="))
			if err != nil {
				return nil, nil, fmt.Errorf("--end: %w", err)
			}
			ctx.EndDate = d

		case arg == "--verbose" || arg == "-verbose" || arg == "-v":
			ctx.Verbose = true

		default:
			remaining = append(remaining, arg)
		}
	}

	return ctx, remaining, nil
}

// NormalizeDate accepts a date in YYYY-MM-DD or YYYY/MM/DD format and
// returns it normalised to YYYY-MM-DD.
// Returns an error for any other format.
func NormalizeDate(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD or YYYY/MM/DD", s)
	}

	// Normalise YYYY/MM/DD → YYYY-MM-DD
	normalized := strings.ReplaceAll(s, "/", "-")

	// Basic structural validation: DDDD-DD-DD
	if normalized[4] != '-' || normalized[7] != '-' {
		return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD or YYYY/MM/DD", s)
	}
	for _, i := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if normalized[i] < '0' || normalized[i] > '9' {
			return "", fmt.Errorf("invalid date %q: non-digit character", s)
		}
	}

	return normalized, nil
}

package tui

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"doublebook/core/ast"
	"doublebook/infra/utils"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	insStylePrompt  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(20)
	insStyleFocused = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	insStyleBlur    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	insStyleSugg    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	insStyleSuggSel = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	insStyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	insStyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("71")).Bold(true)
	insStyleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	insStyleFooter  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	insStyleSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)

var accountRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*(:([a-zA-Z][a-zA-Z0-9_-]*))*$`)

// ---------------------------------------------------------------------------
// Field indices
// ---------------------------------------------------------------------------

const (
	fDate     = 0
	fDesc     = 1
	fAcct1    = 2
	fAmount   = 3
	fCurr     = 4
	fAcct2    = 5
	fTags     = 6
	numFields = 7
)

// ---------------------------------------------------------------------------
// InsertModel
// ---------------------------------------------------------------------------

// InsertModel is the Bubbletea model for the inline insert form.
type InsertModel struct {
	inputs     [numFields]textinput.Model
	focusIndex int

	// Autocomplete
	suggestions []string
	suggIdx     int
	showSugg    bool

	// Known data for autocomplete
	knownAccounts   []string
	knownCurrencies []string

	// Feedback
	fieldError string

	// Result
	done    bool
	aborted bool
	result  *ast.Transaction

	defaultCurrency string
}

// NewInsertModel builds an InsertModel pre-loaded with autocomplete data.
func NewInsertModel(accounts, currencies []string, defaultCurrency string) InsertModel {
	specs := []struct {
		prompt      string
		placeholder string
		charLimit   int
	}{
		{"Date:", "YYYY-MM-DD", 10},
		{"Description:", "Grocery Store", 80},
		{"Debit Account:", "expenses:food", 60},
		{"Amount:", "$0.00", 20},
		{"Currency:", "USD", 6},
		{"Credit Account:", "assets:checking", 60},
		{"Tags:", "key:value, ... (optional)", 100},
	}

	var inputs [numFields]textinput.Model
	for i, s := range specs {
		inp := textinput.New()
		inp.Prompt = insStylePrompt.Render(fmt.Sprintf("  %-17s", s.prompt)) + " "
		inp.Placeholder = s.placeholder
		inp.CharLimit = s.charLimit
		inp.Width = 40
		inputs[i] = inp
	}

	// Defaults.
	inputs[fDate].SetValue(time.Now().Format("2006-01-02"))
	inputs[fCurr].SetValue(defaultCurrency)
	inputs[fDate].Focus()

	if len(currencies) == 0 {
		currencies = []string{"USD", "EUR", "GBP", "BGN", "CHF", "CAD", "AUD", "JPY"}
	}

	return InsertModel{
		inputs:          inputs,
		knownAccounts:   accounts,
		knownCurrencies: currencies,
		defaultCurrency: defaultCurrency,
	}
}

// Aborted reports whether the user pressed Esc/Ctrl+C.
func (m InsertModel) Aborted() bool { return m.aborted }

// Result returns the built transaction, or nil if not yet submitted.
func (m InsertModel) Result() *ast.Transaction { return m.result }

// ---------------------------------------------------------------------------
// Bubbletea interface
// ---------------------------------------------------------------------------

func (m InsertModel) Init() tea.Cmd { return textinput.Blink }

func (m InsertModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		// Global abort.
		if key == "ctrl+c" || key == "esc" {
			if m.showSugg {
				// First Esc just closes suggestions.
				m.showSugg = false
				m.suggestions = nil
				return m, nil
			}
			m.aborted = true
			return m, tea.Quit
		}

		// Suggestion navigation.
		if m.showSugg {
			switch key {
			case "up", "k":
				if m.suggIdx > 0 {
					m.suggIdx--
				}
				return m, nil
			case "down", "j":
				if m.suggIdx < len(m.suggestions)-1 {
					m.suggIdx++
				}
				return m, nil
			case "enter":
				// Select suggestion.
				if m.suggIdx < len(m.suggestions) {
					m.inputs[m.focusIndex].SetValue(m.suggestions[m.suggIdx])
				}
				m.showSugg = false
				m.suggestions = nil
				m.suggIdx = 0
				m.fieldError = ""
				return m, nil
			}
		}

		// Navigation keys.
		switch key {
		case "tab", "down":
			return m.advanceField()
		case "shift+tab", "up":
			if !m.showSugg {
				return m.retreatField()
			}
		case "enter":
			if m.focusIndex == numFields-1 {
				return m.trySubmit()
			}
			return m.advanceField()
		}

		// Forward to focused input.
		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		m.updateSuggestions()
		m.fieldError = ""
		return m, cmd
	}

	// Propagate to focused input for non-key messages (e.g. blink).
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

// ---------------------------------------------------------------------------
// Navigation
// ---------------------------------------------------------------------------

func (m InsertModel) advanceField() (tea.Model, tea.Cmd) {
	m.showSugg = false
	m.suggestions = nil
	m.suggIdx = 0

	// Validate current field before advancing.
	if err := m.validateField(m.focusIndex); err != "" {
		m.fieldError = err
		return m, nil
	}
	m.fieldError = ""

	m.inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex + 1) % numFields
	return m, m.inputs[m.focusIndex].Focus()
}

func (m InsertModel) retreatField() (tea.Model, tea.Cmd) {
	m.showSugg = false
	m.suggestions = nil
	m.suggIdx = 0
	m.fieldError = ""

	m.inputs[m.focusIndex].Blur()
	m.focusIndex = (m.focusIndex - 1 + numFields) % numFields
	return m, m.inputs[m.focusIndex].Focus()
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func (m InsertModel) validateField(idx int) string {
	v := strings.TrimSpace(m.inputs[idx].Value())
	switch idx {
	case fDate:
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "Invalid date — use YYYY-MM-DD (e.g. 2025-01-15)"
		}
	case fDesc:
		if v == "" {
			return "Description cannot be empty"
		}
	case fAcct1, fAcct2:
		if v == "" {
			return "Account cannot be empty"
		}
		if !accountRE.MatchString(v) {
			return "Account must be like 'expenses:food' (letters, digits, colons)"
		}
	case fAmount:
		if v == "" {
			return "Amount cannot be empty"
		}
		if _, err := utils.ParseAmount(v); err != nil {
			return "Invalid amount — try '45.32' or '$45.32' or '100 BGN'"
		}
	case fCurr:
		if v == "" {
			m.inputs[idx].SetValue(m.defaultCurrency)
		}
	}
	return ""
}

// validateAll runs all field validations and returns the first error (field, message).
func (m InsertModel) validateAll() (int, string) {
	for i := 0; i < numFields; i++ {
		if i == fTags {
			continue // tags are optional
		}
		if err := m.validateField(i); err != "" {
			return i, err
		}
	}
	return -1, ""
}

// ---------------------------------------------------------------------------
// Submission
// ---------------------------------------------------------------------------

func (m InsertModel) trySubmit() (tea.Model, tea.Cmd) {
	m.showSugg = false
	m.suggestions = nil

	idx, err := m.validateAll()
	if err != "" {
		m.fieldError = err
		m.inputs[m.focusIndex].Blur()
		m.focusIndex = idx
		_ = m.inputs[m.focusIndex].Focus()
		return m, nil
	}
	m.fieldError = ""

	// Build the transaction.
	dateStr := strings.TrimSpace(m.inputs[fDate].Value())
	desc := strings.TrimSpace(m.inputs[fDesc].Value())
	acct1 := strings.TrimSpace(m.inputs[fAcct1].Value())
	amtStr := strings.TrimSpace(m.inputs[fAmount].Value())
	curr := strings.TrimSpace(m.inputs[fCurr].Value())
	acct2 := strings.TrimSpace(m.inputs[fAcct2].Value())
	tagsStr := strings.TrimSpace(m.inputs[fTags].Value())

	date, _ := time.Parse("2006-01-02", dateStr)

	// If currency is set but not embedded in amount, prefix it.
	amount, _ := utils.ParseAmount(amtStr)
	if amount.Currency == "USD" && curr != "" && curr != "USD" {
		// Re-parse with explicit currency suffix.
		amount, _ = utils.ParseAmount(amtStr + " " + curr)
	}

	txn := ast.NewTransaction(date, desc)
	txn.ID = generateInsertID(dateStr, desc, acct1, amtStr)
	txn.Postings = append(txn.Postings,
		ast.NewPosting(acct1, amount),
		ast.NewPosting(acct2, amount.Negate()),
	)

	// Parse tags.
	if tagsStr != "" {
		for _, part := range strings.Split(tagsStr, ",") {
			part = strings.TrimSpace(part)
			k, v, found := strings.Cut(part, ":")
			if found {
				txn.Tags[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}

	m.result = txn
	m.done = true
	return m, tea.Quit
}

func generateInsertID(date, desc, account, amount string) string {
	h := sha256.New()
	h.Write([]byte(date + "|" + desc + "|" + account + "|" + amount))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// ---------------------------------------------------------------------------
// Autocomplete
// ---------------------------------------------------------------------------

func (m *InsertModel) updateSuggestions() {
	fi := m.focusIndex
	val := m.inputs[fi].Value()

	var pool []string
	switch fi {
	case fAcct1, fAcct2:
		pool = m.knownAccounts
	case fCurr:
		pool = m.knownCurrencies
	default:
		m.showSugg = false
		m.suggestions = nil
		return
	}

	if val == "" {
		m.showSugg = false
		m.suggestions = nil
		return
	}

	var matches []string
	lower := strings.ToLower(val)
	for _, a := range pool {
		if strings.HasPrefix(strings.ToLower(a), lower) && a != val {
			matches = append(matches, a)
			if len(matches) == 8 {
				break
			}
		}
	}

	m.suggestions = matches
	m.showSugg = len(matches) > 0
	if m.suggIdx >= len(matches) {
		m.suggIdx = 0
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m InsertModel) View() string {
	if m.done || m.aborted {
		return ""
	}

	var b strings.Builder

	b.WriteString(insStyleHeader.Render("  DoubleBook — Add Transaction"))
	b.WriteByte('\n')
	b.WriteString(insStyleSep.Render("  " + strings.Repeat("─", 50)))
	b.WriteByte('\n')
	b.WriteByte('\n')

	for i := 0; i < numFields; i++ {
		// Render the input field.
		b.WriteString(m.inputs[i].View())
		b.WriteByte('\n')

		// Render suggestions right after the focused account/currency field.
		if i == m.focusIndex && m.showSugg && len(m.suggestions) > 0 {
			for j, s := range m.suggestions {
				prefix := "    "
				if j == m.suggIdx {
					b.WriteString(insStyleSuggSel.Render(prefix + "▸ " + s))
				} else {
					b.WriteString(insStyleSugg.Render(prefix + "  " + s))
				}
				b.WriteByte('\n')
			}
			b.WriteString(insStyleSugg.Render("    (↑↓ to navigate, Enter to select, Esc to dismiss)"))
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')

	// Error or empty line.
	if m.fieldError != "" {
		b.WriteString(insStyleError.Render("  ✗ " + m.fieldError))
	} else {
		b.WriteString("  ")
	}
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(insStyleFooter.Render(
		"  Tab/↓ next  Shift+Tab/↑ prev  Enter confirm  Esc abort",
	))
	b.WriteByte('\n')

	return b.String()
}

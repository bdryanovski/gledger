package rules

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Interactive Mapper TUI
// ---------------------------------------------------------------------------

// MapperMode represents the current view/mode of the mapper.
type MapperMode int

const (
	ModePreview MapperMode = iota
	ModeMapping
	ModeTransform
	ModeSettings
	ModeSave
)

// MapperModel is the Bubbletea model for the interactive column mapper.
type MapperModel struct {
	// File data
	preview   *FilePreview
	filePath  string

	// UI state
	mode         MapperMode
	width        int
	height       int
	selectedCol  int
	cursor       int
	message      string
	messageIsErr bool

	// Table for column display
	columnTable table.Model

	// The ruleset being built
	ruleSet *RuleSet

	// Mapping state
	mappings map[int]*FieldMapping // column index -> mapping
	
	// Text inputs for various fields
	nameInput          textinput.Model
	sourceAccountInput textinput.Model
	currencyInput      textinput.Model
	delimiterInput     textinput.Model
	skipLinesInput     textinput.Model
	encodingInput      textinput.Model
	
	// Settings focus index (which input is focused)
	settingsFocus int
	
	// Available fields to map
	availableFields []string
	fieldCursor     int
	
	// Quit flag
	quitting bool
	saved    bool
}

// Available transaction fields
var transactionFields = []string{
	"date",
	"description",
	"amount",
	"debit_amount",
	"credit_amount",
	"debit_account",
	"credit_account",
	"currency",
	"reference",
	"tag:category",
	"tag:merchant",
	"(skip)",
}

// Styles
var (
	mapperHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#7C3AED")).
		Padding(0, 2)

	mapperTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	mapperSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#22c55e"))

	mapperDimStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	mapperErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444")).
		Bold(true)

	mapperSuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22c55e")).
		Bold(true)

	mapperBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	mapperHighlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("57")).
		Foreground(lipgloss.Color("229"))
)

// NewMapperModel creates a new mapper model for the given file.
func NewMapperModel(filePath string) (*MapperModel, error) {
	preview, err := PreviewFile(filePath, DefaultPreviewOptions())
	if err != nil {
		return nil, fmt.Errorf("previewing file: %w", err)
	}

	// Create column table
	cols := []table.Column{
		{Title: "#", Width: 3},
		{Title: "Column Name", Width: 20},
		{Title: "Samples", Width: 40},
		{Title: "Mapped To", Width: 20},
	}
	
	rows := make([]table.Row, len(preview.Columns))
	for i, col := range preview.Columns {
		samples := strings.Join(col.Samples, " | ")
		if len(samples) > 38 {
			samples = samples[:35] + "..."
		}
		rows[i] = table.Row{
			fmt.Sprintf("%d", i),
			truncate(col.Name, 18),
			samples,
			"(unmapped)",
		}
	}
	
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	ts.Selected = ts.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(ts)

	// Create text inputs
	nameInput := textinput.New()
	nameInput.Placeholder = "my-bank"
	nameInput.Prompt = "Rule name:      "
	nameInput.Width = 30

	sourceInput := textinput.New()
	sourceInput.Placeholder = "assets:checking:mybank"
	sourceInput.Prompt = "Source account: "
	sourceInput.Width = 40

	currencyInput := textinput.New()
	currencyInput.Placeholder = "USD"
	currencyInput.Prompt = "Currency:       "
	currencyInput.Width = 10
	currencyInput.SetValue("USD")

	delimiterInput := textinput.New()
	delimiterInput.Placeholder = ","
	delimiterInput.Prompt = "Delimiter:      "
	delimiterInput.Width = 5
	delimiterInput.CharLimit = 1
	delimiterInput.SetValue(",")

	skipLinesInput := textinput.New()
	skipLinesInput.Placeholder = "1"
	skipLinesInput.Prompt = "Skip lines:     "
	skipLinesInput.Width = 5
	skipLinesInput.CharLimit = 2
	skipLinesInput.SetValue("1")

	encodingInput := textinput.New()
	encodingInput.Placeholder = "utf-8"
	encodingInput.Prompt = "Encoding:       "
	encodingInput.Width = 15
	encodingInput.SetValue("utf-8")

	// Create initial ruleset
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	rs := &RuleSet{
		Name:                 baseName,
		Version:              "1",
		SourceAccount:        "assets:checking",
		DefaultDebitAccount:  "expenses:unknown",
		DefaultCreditAccount: "income:unknown",
		Currency:             "USD",
		Format: FileFormat{
			Type:      preview.FileType,
			Delimiter: ",",
			Encoding:  "utf-8",
			SkipLines: 1,
		},
		Columns: preview.Columns,
	}
	nameInput.SetValue(baseName)
	sourceInput.SetValue("assets:checking")

	// Auto-suggest mappings
	mappings := make(map[int]*FieldMapping)
	for i, col := range preview.Columns {
		suggested := SuggestFieldMapping(col)
		if suggested != "" && suggested != "(skip)" {
			mappings[i] = &FieldMapping{
				Field:  suggested,
				Direct: &DirectMapping{Column: i},
			}
		}
	}

	return &MapperModel{
		preview:            preview,
		filePath:           filePath,
		mode:               ModePreview,
		width:              80,
		height:             24,
		columnTable:        t,
		ruleSet:            rs,
		mappings:           mappings,
		nameInput:          nameInput,
		sourceAccountInput: sourceInput,
		currencyInput:      currencyInput,
		delimiterInput:     delimiterInput,
		skipLinesInput:     skipLinesInput,
		encodingInput:      encodingInput,
		availableFields:    transactionFields,
	}, nil
}

// Init implements tea.Model.
func (m MapperModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m MapperModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// Global quit
		if key == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// Mode-specific handling
		switch m.mode {
		case ModePreview:
			return m.updatePreview(msg)
		case ModeMapping:
			return m.updateMapping(msg)
		case ModeSettings:
			return m.updateSettings(msg)
		case ModeSave:
			return m.updateSave(msg)
		}
	}

	return m, nil
}

// updatePreview handles input in preview mode.
func (m MapperModel) updatePreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "enter", " ":
		// Enter mapping mode for selected column
		m.selectedCol = m.columnTable.Cursor()
		m.mode = ModeMapping
		m.fieldCursor = 0
		m.message = ""
		return m, nil

	case "s":
		// Go to settings
		m.mode = ModeSettings
		m.settingsFocus = 0
		m.message = ""
		return m, m.focusSettingsInput(0)

	case "w":
		// Save and quit
		if err := m.saveRuleSet(); err != nil {
			m.message = err.Error()
			m.messageIsErr = true
		} else {
			m.saved = true
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case "r":
		// Reload preview with current settings
		opts := PreviewOptions{
			Delimiter:   m.ruleSet.Format.Delimiter,
			Encoding:    m.ruleSet.Format.Encoding,
			SkipLines:   0, // We show raw columns, skip_lines affects import
			SampleCount: 5,
		}
		newPreview, err := PreviewFile(m.filePath, opts)
		if err != nil {
			m.message = "Reload failed: " + err.Error()
			m.messageIsErr = true
		} else {
			m.preview = newPreview
			m.ruleSet.Columns = newPreview.Columns
			m.rebuildColumnTable()
			m.message = "Preview reloaded with new settings"
			m.messageIsErr = false
		}
		return m, nil
	}

	// Forward to table
	var cmd tea.Cmd
	m.columnTable, cmd = m.columnTable.Update(msg)
	return m, cmd
}

// updateMapping handles input in mapping mode.
func (m MapperModel) updateMapping(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.mode = ModePreview
		m.message = ""
		return m, nil

	case "up", "k":
		if m.fieldCursor > 0 {
			m.fieldCursor--
		}
		return m, nil

	case "down", "j":
		if m.fieldCursor < len(m.availableFields)-1 {
			m.fieldCursor++
		}
		return m, nil

	case "enter", " ":
		field := m.availableFields[m.fieldCursor]
		if field == "(skip)" {
			delete(m.mappings, m.selectedCol)
		} else {
			m.mappings[m.selectedCol] = &FieldMapping{
				Field:  field,
				Direct: &DirectMapping{Column: m.selectedCol},
			}
		}
		m.updateTableRow(m.selectedCol)
		m.mode = ModePreview
		m.message = fmt.Sprintf("Column %d mapped to %s", m.selectedCol, field)
		m.messageIsErr = false
		return m, nil
	}

	return m, nil
}

// settingsInputs returns all settings inputs in order.
func (m *MapperModel) settingsInputs() []*textinput.Model {
	return []*textinput.Model{
		&m.nameInput,
		&m.sourceAccountInput,
		&m.currencyInput,
		&m.delimiterInput,
		&m.skipLinesInput,
		&m.encodingInput,
	}
}

// focusSettingsInput focuses the input at the given index.
func (m *MapperModel) focusSettingsInput(idx int) tea.Cmd {
	inputs := m.settingsInputs()
	for i, inp := range inputs {
		if i == idx {
			return inp.Focus()
		} else {
			inp.Blur()
		}
	}
	return nil
}

// updateSettings handles input in settings mode.
func (m MapperModel) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	numInputs := 6

	switch key {
	case "esc":
		m.mode = ModePreview
		for _, inp := range m.settingsInputs() {
			inp.Blur()
		}
		return m, nil

	case "tab", "down":
		m.settingsFocus = (m.settingsFocus + 1) % numInputs
		return m, m.focusSettingsInput(m.settingsFocus)

	case "shift+tab", "up":
		m.settingsFocus = (m.settingsFocus - 1 + numInputs) % numInputs
		return m, m.focusSettingsInput(m.settingsFocus)

	case "enter":
		// Apply settings and go back
		m.ruleSet.Name = m.nameInput.Value()
		m.ruleSet.SourceAccount = m.sourceAccountInput.Value()
		m.ruleSet.Currency = m.currencyInput.Value()
		
		// Handle delimiter - convert \t to actual tab
		delimiter := m.delimiterInput.Value()
		if delimiter == "\\t" || delimiter == "t" {
			delimiter = "\t"
		}
		if delimiter == "" {
			delimiter = ","
		}
		m.ruleSet.Format.Delimiter = delimiter
		m.ruleSet.Format.Encoding = m.encodingInput.Value()
		
		// Parse skip lines
		skipLines := 1
		if s := m.skipLinesInput.Value(); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 {
				skipLines = n
			}
		}
		m.ruleSet.Format.SkipLines = skipLines
		
		m.mode = ModePreview
		m.message = "Settings updated"
		m.messageIsErr = false
		return m, nil
	}

	// Forward to focused input
	var cmd tea.Cmd
	inputs := m.settingsInputs()
	for i, inp := range inputs {
		if inp.Focused() {
			*inputs[i], cmd = inp.Update(msg)
			break
		}
	}
	return m, cmd
}

// updateSave handles input in save mode.
func (m MapperModel) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "n":
		m.mode = ModePreview
		return m, nil

	case "y", "enter":
		if err := m.saveRuleSet(); err != nil {
			m.message = err.Error()
			m.messageIsErr = true
			m.mode = ModePreview
		} else {
			m.saved = true
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

// rebuildColumnTable rebuilds the entire column table from the current preview.
func (m *MapperModel) rebuildColumnTable() {
	rows := make([]table.Row, len(m.preview.Columns))
	for i, col := range m.preview.Columns {
		samples := strings.Join(col.Samples, " | ")
		if len(samples) > 38 {
			samples = samples[:35] + "..."
		}
		mappedTo := "(unmapped)"
		if mapping, ok := m.mappings[i]; ok && mapping != nil {
			mappedTo = mapping.Field
		}
		rows[i] = table.Row{
			fmt.Sprintf("%d", i),
			truncate(col.Name, 18),
			samples,
			mappedTo,
		}
	}
	m.columnTable.SetRows(rows)
}

// updateTableRow updates a single row in the column table.
func (m *MapperModel) updateTableRow(colIdx int) {
	rows := m.columnTable.Rows()
	if colIdx >= len(rows) {
		return
	}

	mappedTo := "(unmapped)"
	if mapping, ok := m.mappings[colIdx]; ok {
		mappedTo = mapping.Field
	}

	col := m.preview.Columns[colIdx]
	samples := strings.Join(col.Samples, " | ")
	if len(samples) > 38 {
		samples = samples[:35] + "..."
	}

	rows[colIdx] = table.Row{
		fmt.Sprintf("%d", colIdx),
		truncate(col.Name, 18),
		samples,
		mappedTo,
	}
	m.columnTable.SetRows(rows)
}

// saveRuleSet builds and saves the ruleset.
func (m *MapperModel) saveRuleSet() error {
	// Build mappings
	m.ruleSet.Mappings = nil
	for colIdx, mapping := range m.mappings {
		if mapping != nil {
			mapping.Direct.Column = colIdx
			m.ruleSet.Mappings = append(m.ruleSet.Mappings, *mapping)
		}
	}

	// Validate
	if m.ruleSet.Name == "" {
		return fmt.Errorf("rule name is required")
	}
	if m.ruleSet.SourceAccount == "" {
		return fmt.Errorf("source account is required")
	}

	// Check required fields
	hasDate := false
	hasAmount := false
	for _, mapping := range m.ruleSet.Mappings {
		if mapping.Field == "date" {
			hasDate = true
		}
		if mapping.Field == "amount" || mapping.Field == "debit_amount" || mapping.Field == "credit_amount" {
			hasAmount = true
		}
	}
	if !hasDate {
		return fmt.Errorf("date mapping is required")
	}
	if !hasAmount {
		return fmt.Errorf("amount mapping is required")
	}

	// Save
	savePath := filepath.Join(DefaultRulesDir(), m.ruleSet.Name+".rules.yaml")
	return SaveRuleSet(m.ruleSet, savePath)
}

// View implements tea.Model.
func (m MapperModel) View() string {
	var b strings.Builder

	// Header
	headerText := fmt.Sprintf(" Import Mapper  ·  %s ", filepath.Base(m.filePath))
	b.WriteString(mapperHeaderStyle.Width(m.width).Render(headerText))
	b.WriteString("\n")

	// Message
	if m.message != "" {
		style := mapperSuccessStyle
		if m.messageIsErr {
			style = mapperErrorStyle
		}
		b.WriteString(style.Render("  " + m.message))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
	}

	// Mode-specific content
	switch m.mode {
	case ModePreview:
		b.WriteString(m.viewPreview())
	case ModeMapping:
		b.WriteString(m.viewMapping())
	case ModeSettings:
		b.WriteString(m.viewSettings())
	case ModeSave:
		b.WriteString(m.viewSave())
	}

	return b.String()
}

func (m MapperModel) viewPreview() string {
	var b strings.Builder

	// File info
	info := fmt.Sprintf("  File: %s (%s, %d rows)",
		filepath.Base(m.filePath),
		m.preview.FileType,
		m.preview.TotalRows)
	b.WriteString(mapperDimStyle.Render(info))
	b.WriteString("\n\n")

	// Column table
	b.WriteString(mapperTitleStyle.Render("  COLUMNS"))
	b.WriteString("\n")
	b.WriteString(m.columnTable.View())
	b.WriteString("\n\n")

	// Mapping summary
	mapped := 0
	for _, mapping := range m.mappings {
		if mapping != nil {
			mapped++
		}
	}
	summary := fmt.Sprintf("  Mapped: %d/%d columns", mapped, len(m.preview.Columns))
	b.WriteString(mapperDimStyle.Render(summary))
	b.WriteString("\n\n")

	// Footer
	footer := "  [↑↓] select  [enter] map  [s] settings  [r] reload  [w] save & quit  [q] cancel"
	b.WriteString(mapperDimStyle.Render(footer))

	return b.String()
}

func (m MapperModel) viewMapping() string {
	var b strings.Builder

	col := m.preview.Columns[m.selectedCol]
	
	// Column info
	b.WriteString(mapperTitleStyle.Render(fmt.Sprintf("  MAPPING COLUMN %d: %s", m.selectedCol, col.Name)))
	b.WriteString("\n\n")

	// Samples
	b.WriteString(mapperDimStyle.Render("  Sample values:"))
	b.WriteString("\n")
	for _, sample := range col.Samples {
		b.WriteString(fmt.Sprintf("    • %s\n", sample))
	}
	b.WriteString("\n")

	// Field selection
	b.WriteString(mapperTitleStyle.Render("  MAP TO FIELD:"))
	b.WriteString("\n")
	for i, field := range m.availableFields {
		prefix := "  "
		style := mapperDimStyle
		if i == m.fieldCursor {
			prefix = "▶ "
			style = mapperSelectedStyle
		}
		b.WriteString(style.Render(prefix + field))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Footer
	footer := "  [↑↓] select field  [enter] confirm  [esc] cancel"
	b.WriteString(mapperDimStyle.Render(footer))

	return b.String()
}

func (m MapperModel) viewSettings() string {
	var b strings.Builder

	b.WriteString(mapperTitleStyle.Render("  SETTINGS"))
	b.WriteString("\n\n")

	// Basic settings
	b.WriteString(mapperDimStyle.Render("  Basic:"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.nameInput.View())
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.sourceAccountInput.View())
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.currencyInput.View())
	b.WriteString("\n\n")

	// File format settings
	b.WriteString(mapperDimStyle.Render("  File Format:"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.delimiterInput.View())
	b.WriteString("  ")
	b.WriteString(mapperDimStyle.Render("(use \\t for tab)"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.skipLinesInput.View())
	b.WriteString("  ")
	b.WriteString(mapperDimStyle.Render("(header rows to skip)"))
	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(m.encodingInput.View())
	b.WriteString("  ")
	b.WriteString(mapperDimStyle.Render("(utf-8, windows-1252, iso-8859-1)"))
	b.WriteString("\n\n")

	// Footer
	footer := "  [tab/↑↓] next field  [enter] save settings  [esc] cancel"
	b.WriteString(mapperDimStyle.Render(footer))

	return b.String()
}

func (m MapperModel) viewSave() string {
	var b strings.Builder

	b.WriteString(mapperTitleStyle.Render("  SAVE RULES"))
	b.WriteString("\n\n")

	savePath := filepath.Join(DefaultRulesDir(), m.ruleSet.Name+".rules.yaml")
	b.WriteString(fmt.Sprintf("  Save to: %s\n\n", savePath))

	b.WriteString("  Save now? [y/n]\n\n")

	return b.String()
}

// Saved returns true if the ruleset was saved.
func (m MapperModel) Saved() bool {
	return m.saved
}

// RuleSetPath returns the path where the ruleset was saved.
func (m MapperModel) RuleSetPath() string {
	return filepath.Join(DefaultRulesDir(), m.ruleSet.Name+".rules.yaml")
}

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// RunMapper runs the interactive mapper TUI for the given file.
func RunMapper(filePath string) (*RuleSet, error) {
	model, err := NewMapperModel(filePath)
	if err != nil {
		return nil, err
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("running mapper: %w", err)
	}

	m := finalModel.(MapperModel)
	if !m.Saved() {
		return nil, fmt.Errorf("mapping cancelled")
	}

	// Load the saved ruleset
	return LoadRuleSet(m.RuleSetPath())
}

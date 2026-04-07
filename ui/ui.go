// Package UI implements the DoubleBook fullscreen terminal interface using
// Bubbletea.  The TUI has four views:
//
//	VIEW_LIST   — scrollable transaction table with a detail pane
//	VIEW_ADD    — form for adding a new transaction
//	VIEW_REPORT — balance / income-statement report
//	VIEW_HELP   — keyboard shortcut reference
package UI

import (
	"fmt"
	"strings"
	"time"

	AST "doublebook/ast"
	"doublebook/config"
	Interpreter "doublebook/interpreter"
	"doublebook/utils"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// View identifiers
// ---------------------------------------------------------------------------

type ViewMode int

const (
	VIEW_LIST ViewMode = iota
	VIEW_ADD
	VIEW_REPORT
	VIEW_HELP
)

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#7C3AED")).
			Padding(0, 2)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED"))

	styleSep = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleFooter = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)

	styleMsg = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22c55e")).
			Bold(true)

	styleErr = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef4444")).
			Bold(true)

	styleLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))

	styleValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	stylePositive = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#22c55e"))

	styleNegative = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ef4444"))
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

// Model is the Bubbletea application model.
type Model struct {
	interpreter *Interpreter.Interpreter
	config      *config.Config

	currentView ViewMode

	// Dimensions — updated on tea.WindowSizeMsg
	width  int
	height int

	// Transaction table (LIST view)
	table table.Model

	// Add-transaction form (ADD view)
	formInputs []textinput.Model
	formFocus  int

	// Status message shown in the header area
	message    string
	messageErr bool // true → show message in red, false → green
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

const (
	// Lines of chrome surrounding the table in VIEW_LIST.
	// 1 header bar + 1 blank + 1 "Transactions" title + 1 sep + 1 blank +
	// 1 blank after table + 1 detail-pane header + 3 detail lines + 1 footer
	tableChrome = 11

	// Minimum table height even on tiny terminals.
	minTableHeight = 5
)

// InitialModel constructs the initial TUI model and loads journal data.
func InitialModel() (Model, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return Model{}, fmt.Errorf("loading config: %w", err)
	}

	interp := Interpreter.NewInterpreter(cfg)
	// Non-fatal: may not exist yet.
	_ = interp.LoadFromFile(cfg.DataFile)

	// Table columns — widths are adjusted later on WindowSizeMsg.
	cols := []table.Column{
		{Title: "Date", Width: 12},
		{Title: "Description", Width: 28},
		{Title: "Amount", Width: 14},
		{Title: "Account", Width: 24},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(minTableHeight),
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

	// Add-transaction form inputs.
	inputs := make([]textinput.Model, 5)
	specs := []struct {
		placeholder, prompt string
		charLimit, width    int
	}{
		{"YYYY-MM-DD", "Date:          ", 10, 20},
		{"Grocery Store", "Description:   ", 100, 40},
		{"expenses:food", "Debit account: ", 50, 40},
		{"$45.32", "Amount:        ", 20, 20},
		{"assets:checking", "Credit account:", 50, 40},
	}
	for i, spec := range specs {
		inp := textinput.New()
		inp.Placeholder = spec.placeholder
		inp.Prompt = spec.prompt
		inp.CharLimit = spec.charLimit
		inp.Width = spec.width
		inputs[i] = inp
	}
	inputs[0].Focus()
	inputs[0].SetValue(time.Now().Format("2006-01-02"))

	m := Model{
		interpreter: interp,
		config:      cfg,
		currentView: VIEW_LIST,
		width:       80,
		height:      24,
		table:       t,
		formInputs:  inputs,
	}
	m.rebuildTable()

	return m, nil
}

// ---------------------------------------------------------------------------
// Bubbletea interface
// ---------------------------------------------------------------------------

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update is the central event handler.  Global navigation keys are handled
// first and return immediately so they cannot fall through to view handlers.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Window resize ─────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeTable()
		return m, nil

	// ── Keyboard ──────────────────────────────────────────────────────────
	case tea.KeyMsg:
		key := msg.String()

		// Global quit — save first.
		if key == "ctrl+c" {
			_ = m.interpreter.SaveToFile(m.config.DataFile)
			return m, tea.Quit
		}

		// Quit from any non-add view with 'q'.
		if key == "q" && m.currentView != VIEW_ADD {
			_ = m.interpreter.SaveToFile(m.config.DataFile)
			return m, tea.Quit
		}

		// Global navigation — handled here and returned immediately.
		// This prevents the key from also being passed to the active view.
		switch key {
		case "a":
			if m.currentView == VIEW_LIST {
				return m.enterAddView()
			}
		case "r":
			if m.currentView == VIEW_LIST || m.currentView == VIEW_HELP {
				m.currentView = VIEW_REPORT
				m.message = ""
				return m, nil
			}
		case "?", "h":
			if m.currentView == VIEW_LIST || m.currentView == VIEW_REPORT {
				m.currentView = VIEW_HELP
				m.message = ""
				return m, nil
			}
		case "esc":
			if m.currentView != VIEW_LIST {
				m.currentView = VIEW_LIST
				m.message = ""
				return m, nil
			}
		}

		// Dispatch to the active view's handler.
		switch m.currentView {
		case VIEW_LIST:
			return m.updateList(msg)
		case VIEW_ADD:
			return m.updateAdd(msg)
		case VIEW_REPORT:
			return m.updateReport(msg)
		case VIEW_HELP:
			return m, nil
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Per-view update handlers
// ---------------------------------------------------------------------------

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "tab", "down":
		m.formFocus = (m.formFocus + 1) % len(m.formInputs)
		return m, m.focusFormField()

	case "shift+tab", "up":
		m.formFocus = (m.formFocus - 1 + len(m.formInputs)) % len(m.formInputs)
		return m, m.focusFormField()

	case "enter":
		if err := m.submitTransaction(); err != nil {
			m.message = err.Error()
			m.messageErr = true
		} else {
			m.message = "Transaction added!"
			m.messageErr = false
			m.currentView = VIEW_LIST
			m.rebuildTable()
			m.resetForm()
		}
		return m, nil

	case "esc":
		m.currentView = VIEW_LIST
		m.message = ""
		m.resetForm()
		return m, nil
	}

	// Forward key to focused input.
	var cmd tea.Cmd
	m.formInputs[m.formFocus], cmd = m.formInputs[m.formFocus].Update(msg)
	return m, cmd
}

func (m Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// All navigation is handled globally; nothing extra needed here.
	return m, nil
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func (m Model) View() string {
	var b strings.Builder

	// ── Header bar ────────────────────────────────────────────────────────
	viewName := map[ViewMode]string{
		VIEW_LIST:   "Transactions",
		VIEW_ADD:    "Add Transaction",
		VIEW_REPORT: "Balance Report",
		VIEW_HELP:   "Help",
	}[m.currentView]

	headerText := fmt.Sprintf(" DoubleBook  ·  %s ", viewName)
	b.WriteString(styleHeader.Width(m.width).Render(headerText))
	b.WriteByte('\n')

	// ── Status message ────────────────────────────────────────────────────
	if m.message != "" {
		style := styleMsg
		if m.messageErr {
			style = styleErr
		}
		b.WriteString(style.Render("  " + m.message))
		b.WriteByte('\n')
	} else {
		b.WriteByte('\n')
	}

	// ── View content ──────────────────────────────────────────────────────
	switch m.currentView {
	case VIEW_LIST:
		b.WriteString(m.viewList())
	case VIEW_ADD:
		b.WriteString(m.viewAdd())
	case VIEW_REPORT:
		b.WriteString(m.viewReport())
	case VIEW_HELP:
		b.WriteString(m.viewHelp())
	}

	return b.String()
}

// viewList renders the transaction table and a detail pane for the selected row.
func (m Model) viewList() string {
	var b strings.Builder

	sep := styleSep.Render(strings.Repeat("─", m.width))
	b.WriteString(sep)
	b.WriteByte('\n')
	b.WriteString(m.table.View())
	b.WriteByte('\n')
	b.WriteString(sep)
	b.WriteByte('\n')

	// Detail pane: show selected posting's transaction info.
	b.WriteString(m.selectedDetail())
	b.WriteByte('\n')

	// Footer.
	b.WriteString(styleFooter.Render(
		"  [↑↓] scroll  [a] add  [r] report  [?] help  [q] quit",
	))

	return b.String()
}

// selectedDetail returns a one-line summary of the currently selected row.
func (m Model) selectedDetail() string {
	rows := m.table.Rows()
	cursor := m.table.Cursor()
	if len(rows) == 0 || cursor < 0 || cursor >= len(rows) {
		return styleLabel.Render("  No transaction selected")
	}
	row := rows[cursor]
	if len(row) < 4 {
		return ""
	}
	date, desc, amount, account := row[0], row[1], row[2], row[3]
	return fmt.Sprintf("  %s  %s  %s  %s",
		styleLabel.Render(date),
		styleValue.Render(desc),
		colorAmount(amount),
		styleLabel.Render(account),
	)
}

// colorAmount renders an amount string green if positive/zero, red if negative.
func colorAmount(s string) string {
	if strings.HasPrefix(s, "-") {
		return styleNegative.Render(s)
	}
	return stylePositive.Render(s)
}

// viewAdd renders the add-transaction form.
func (m Model) viewAdd() string {
	var b strings.Builder
	sep := styleSep.Render(strings.Repeat("─", m.width))

	b.WriteString(sep)
	b.WriteString("\n\n")

	for i, inp := range m.formInputs {
		b.WriteString("  ")
		b.WriteString(inp.View())
		if i < len(m.formInputs)-1 {
			b.WriteByte('\n')
		}
	}

	b.WriteString("\n\n")
	b.WriteString(sep)
	b.WriteByte('\n')
	b.WriteString(styleFooter.Render(
		"  [tab] next field  [shift+tab] prev  [enter] save  [esc] cancel",
	))

	return b.String()
}

// reportRow holds one line of data for the balance report display.
type reportRow struct {
	amt     string // formatted amount string, empty for section headers
	account string // account name (indented for children) or section heading
}

// viewReport renders the balance report.
func (m Model) viewReport() string {
	var b strings.Builder
	sep := styleSep.Render(strings.Repeat("─", m.width))

	b.WriteString(sep)
	b.WriteString("\n\n")

	// Build a sorted, colourised balance list.
	nodes := m.interpreter.CalculateBalancesTree(Interpreter.Filter{})
	groups := Interpreter.GroupAccountsByType(nodes)

	order := []string{"assets", "liabilities", "equity", "income", "expenses", "other"}

	// Collect all rows first so we can right-align amounts.
	var rows []reportRow
	for _, grp := range order {
		rootNodes := groups[grp]
		if len(rootNodes) == 0 {
			continue
		}
		rows = append(rows, reportRow{amt: "", account: strings.ToUpper(grp)})
		for _, root := range rootNodes {
			collectReportRows(root, 0, &rows)
		}
		rows = append(rows, reportRow{}) // blank separator between sections
	}

	// Find max amount width for alignment.
	maxW := 0
	for _, r := range rows {
		if len(r.amt) > maxW {
			maxW = len(r.amt)
		}
	}

	for _, r := range rows {
		if r.amt == "" {
			// Section header or blank line.
			if r.account != "" {
				b.WriteString(styleTitle.Render("  " + r.account))
			}
			b.WriteByte('\n')
			continue
		}
		amtStyled := colorAmount(fmt.Sprintf("%*s", maxW, r.amt))
		b.WriteString(fmt.Sprintf("  %s  %s\n", amtStyled, styleLabel.Render(r.account)))
	}

	// Footer.
	b.WriteString(sep)
	b.WriteByte('\n')
	b.WriteString(styleFooter.Render(
		"  [esc] back  [?] help  [q] quit",
	))

	return b.String()
}

// collectReportRows recursively appends AccountNodes to rows with indentation.
func collectReportRows(node *Interpreter.AccountNode, depth int, rows *[]reportRow) {
	if node.Amount.Value == 0 {
		return
	}
	indent := strings.Repeat("  ", depth)
	*rows = append(*rows, reportRow{
		amt:     node.Amount.String(),
		account: indent + node.Name,
	})
	for _, child := range node.Children {
		collectReportRows(child, depth+1, rows)
	}
}

// viewHelp renders the keyboard reference.
func (m Model) viewHelp() string {
	var b strings.Builder
	sep := styleSep.Render(strings.Repeat("─", m.width))

	b.WriteString(sep)
	b.WriteString("\n\n")

	type entry struct{ key, desc string }
	sections := []struct {
		heading string
		entries []entry
	}{
		{"Navigation", []entry{
			{"↑ / ↓ / j / k", "Move up/down in the transaction list"},
			{"a", "Open the add-transaction form"},
			{"r", "Open the balance report"},
			{"? / h", "Show this help screen"},
			{"esc", "Go back to the transaction list"},
			{"q / ctrl+c", "Save and quit"},
		}},
		{"Add Transaction Form", []entry{
			{"tab / ↓", "Move to the next field"},
			{"shift+tab / ↑", "Move to the previous field"},
			{"enter", "Save the transaction"},
			{"esc", "Cancel and go back"},
		}},
		{"Journal Files", []entry{
			{"~/.doublebook/data.journal", "Default journal location"},
			{"--journal NAME", "Use a different journal stem (CLI flag)"},
		}},
	}

	for _, sec := range sections {
		b.WriteString(styleTitle.Render("  " + sec.heading))
		b.WriteString("\n\n")
		for _, e := range sec.entries {
			b.WriteString(fmt.Sprintf("    %-30s %s\n",
				styleValue.Render(e.key),
				styleLabel.Render(e.desc),
			))
		}
		b.WriteByte('\n')
	}

	b.WriteString(sep)
	b.WriteByte('\n')
	b.WriteString(styleFooter.Render("  [esc] back  [q] quit"))

	return b.String()
}

// ---------------------------------------------------------------------------
// Table helpers
// ---------------------------------------------------------------------------

// rebuildTable repopulates the table rows from the in-memory transactions.
func (m *Model) rebuildTable() {
	txns := m.interpreter.GetTransactions()

	var rows []table.Row
	for _, txn := range txns {
		for _, p := range txn.Postings {
			rows = append(rows, table.Row{
				txn.Date.Format("2006-01-02"),
				txn.Description,
				p.Amount.String(),
				p.Account,
			})
		}
	}
	m.table.SetRows(rows)
}

// resizeTable updates the table height when the terminal is resized.
// It also widens the Description and Account columns to use available space.
func (m *Model) resizeTable() {
	h := m.height - tableChrome
	if h < minTableHeight {
		h = minTableHeight
	}
	m.table.SetHeight(h)

	// Distribute extra horizontal space.
	fixed := 12 + 14 + 4 + 3 // Date + Amount + separators
	remaining := m.width - fixed
	if remaining < 20 {
		remaining = 20
	}
	descW := remaining * 55 / 100
	acctW := remaining - descW
	if descW < 10 {
		descW = 10
	}
	if acctW < 10 {
		acctW = 10
	}
	m.table.SetColumns([]table.Column{
		{Title: "Date", Width: 12},
		{Title: "Description", Width: descW},
		{Title: "Amount", Width: 14},
		{Title: "Account", Width: acctW},
	})
}

// ---------------------------------------------------------------------------
// Form helpers
// ---------------------------------------------------------------------------

// enterAddView switches to VIEW_ADD cleanly: focuses the first field and
// returns without passing the triggering key to any input.
func (m Model) enterAddView() (tea.Model, tea.Cmd) {
	m.currentView = VIEW_ADD
	m.message = ""
	m.formFocus = 0
	m.formInputs[0].SetValue(time.Now().Format("2006-01-02"))
	return m, m.focusFormField()
}

// focusFormField focuses the current formFocus index and blurs all others.
func (m Model) focusFormField() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.formInputs))
	for i := range m.formInputs {
		if i == m.formFocus {
			cmds[i] = m.formInputs[i].Focus()
		} else {
			m.formInputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

// resetForm clears all form inputs and resets to the first field.
func (m *Model) resetForm() {
	m.formFocus = 0
	m.formInputs[0].SetValue(time.Now().Format("2006-01-02"))
	for i := 1; i < len(m.formInputs); i++ {
		m.formInputs[i].SetValue("")
		m.formInputs[i].Blur()
	}
	m.formInputs[0].Focus()
}

// submitTransaction reads the form fields and creates a new transaction.
func (m *Model) submitTransaction() error {
	date, err := time.Parse("2006-01-02", m.formInputs[0].Value())
	if err != nil {
		return fmt.Errorf("invalid date — use YYYY-MM-DD format")
	}

	desc := strings.TrimSpace(m.formInputs[1].Value())
	if desc == "" {
		return fmt.Errorf("description is required")
	}

	debitAcct := strings.TrimSpace(m.formInputs[2].Value())
	if debitAcct == "" {
		return fmt.Errorf("debit account is required")
	}

	amtStr := strings.TrimSpace(m.formInputs[3].Value())
	amount, err := utils.ParseAmount(amtStr)
	if err != nil {
		return fmt.Errorf("invalid amount — try $45.32 or 45.32")
	}

	creditAcct := strings.TrimSpace(m.formInputs[4].Value())
	if creditAcct == "" {
		return fmt.Errorf("credit account is required")
	}

	txn := AST.NewTransaction(date, desc)
	txn.Postings = append(txn.Postings,
		AST.NewPosting(debitAcct, amount),
		AST.NewPosting(creditAcct, amount.Negate()),
	)

	if err := m.interpreter.AddTransaction(txn); err != nil {
		return err
	}
	return m.interpreter.SaveToFile(m.config.DataFile)
}

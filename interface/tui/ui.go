// Package tui implements the DoubleBook fullscreen terminal interface using
// Bubbletea.  The TUI has four views:
//
//	VIEW_LIST   — scrollable transaction table with a detail pane
//	VIEW_ADD    — form for adding a new transaction
//	VIEW_REPORT — balance / income-statement report
//	VIEW_HELP   — keyboard shortcut reference
package tui

import (
	"fmt"
	"strings"
	"time"

	"doublebook/core/ast"
	"doublebook/infra/config"
	"doublebook/engine/dashboard"
	"doublebook/engine/interpreter"
	"doublebook/infra/utils"

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
	VIEW_DASHBOARD
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
	interpreter *interpreter.Interpreter
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

	// Dashboard view state
	dashData        *dashboard.DashboardData
	dashMonths      int    // Number of months to show (0 = custom range)
	dashBeginDate   string
	dashEndDate     string
	dashFocus       int                 // 0=presets, 1=begin date, 2=end date
	dashBeginInput  textinput.Model
	dashEndInput    textinput.Model
	dashCustomMode  bool                // true when editing custom dates

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

	interp := interpreter.NewInterpreter(cfg)
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

	// Dashboard date inputs
	beginInput := textinput.New()
	beginInput.Placeholder = "YYYY-MM-DD"
	beginInput.Prompt = "From: "
	beginInput.CharLimit = 10
	beginInput.Width = 12

	endInput := textinput.New()
	endInput.Placeholder = "YYYY-MM-DD"
	endInput.Prompt = "To: "
	endInput.CharLimit = 10
	endInput.Width = 12

	m := Model{
		interpreter:    interp,
		config:         cfg,
		currentView:    VIEW_LIST,
		width:          80,
		height:         24,
		table:          t,
		formInputs:     inputs,
		dashBeginInput: beginInput,
		dashEndInput:   endInput,
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
		case "d":
			if m.currentView == VIEW_LIST || m.currentView == VIEW_HELP || m.currentView == VIEW_REPORT {
				return m.enterDashboard()
			}
		case "?", "h":
			if m.currentView == VIEW_LIST || m.currentView == VIEW_REPORT {
				m.currentView = VIEW_HELP
				m.message = ""
				return m, nil
			}
		case "esc":
			// In dashboard custom mode, esc cancels custom mode first
			if m.currentView == VIEW_DASHBOARD && m.dashCustomMode {
				m.dashCustomMode = false
				m.dashBeginInput.Blur()
				m.dashEndInput.Blur()
				m.message = ""
				return m, nil
			}
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
		case VIEW_DASHBOARD:
			return m.updateDashboard(msg)
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
		VIEW_LIST:      "Transactions",
		VIEW_ADD:       "Add Transaction",
		VIEW_REPORT:    "Balance Report",
		VIEW_DASHBOARD: "Dashboard",
		VIEW_HELP:      "Help",
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
	case VIEW_DASHBOARD:
		b.WriteString(m.viewDashboard())
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
		"  [↑↓] scroll  [a] add  [r] report  [d] dashboard  [?] help  [q] quit",
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
		colorAmountForAccount(amount, account, m.config.CreditNormalPrefixes),
		styleLabel.Render(account),
	)
}

// colorAmountForAccount colors an amount based on account type using the
// credit-normal prefixes from config — no hardcoded account names.
func colorAmountForAccount(s, account string, creditPrefixes []string) string {
	creditNormal := utils.IsAccountCreditNormal(account, creditPrefixes)
	isNeg := strings.HasPrefix(s, "-")
	healthy := (!isNeg && !creditNormal) || (isNeg && creditNormal)
	if healthy {
		return stylePositive.Render(s)
	}
	return styleNegative.Render(s)
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
	nodes := m.interpreter.CalculateBalancesTree(interpreter.Filter{})
	groups := interpreter.GroupAccountsByType(nodes)

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
		amtStyled := colorAmountForAccount(fmt.Sprintf("%*s", maxW, r.amt), r.account, m.config.CreditNormalPrefixes)
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
func collectReportRows(node *interpreter.AccountNode, depth int, rows *[]reportRow) {
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
			{"d", "Open the dashboard"},
			{"? / h", "Show this help screen"},
			{"esc", "Go back to the transaction list"},
			{"q / ctrl+c", "Save and quit"},
		}},
		{"Dashboard", []entry{
			{"← / →", "Decrease / increase time period"},
			{"1 / 3 / 6 / y", "Quick select 1M / 3M / 6M / 1Y"},
			{"c", "Enter custom date range mode"},
			{"tab", "Switch between date fields (in custom mode)"},
			{"enter", "Apply custom date range"},
			{"esc", "Cancel custom mode / return to list"},
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

	txn := ast.NewTransaction(date, desc)
	txn.Postings = append(txn.Postings,
		ast.NewPosting(debitAcct, amount),
		ast.NewPosting(creditAcct, amount.Negate()),
	)

	if err := m.interpreter.AddTransaction(txn); err != nil {
		return err
	}
	return m.interpreter.SaveToFile(m.config.DataFile)
}

// ---------------------------------------------------------------------------
// Dashboard helpers
// ---------------------------------------------------------------------------

// enterDashboard switches to the dashboard view and computes data.
func (m Model) enterDashboard() (tea.Model, tea.Cmd) {
	m.currentView = VIEW_DASHBOARD
	m.message = ""
	m.dashMonths = 6
	m.dashFocus = 0
	m.dashCustomMode = false
	m.dashBeginInput.Blur()
	m.dashEndInput.Blur()
	m.refreshDashboard()
	return m, nil
}

// refreshDashboard recomputes dashboard data based on current settings.
func (m *Model) refreshDashboard() {
	var beginDate, endDate string

	if m.dashMonths == 0 {
		// Custom date range — use stored values
		beginDate = m.dashBeginDate
		endDate = m.dashEndDate
	} else {
		// Preset range
		endDate = time.Now().Format("2006-01-02")
		beginDate = time.Now().AddDate(0, -m.dashMonths, 0).Format("2006-01-02")
		m.dashBeginDate = beginDate
		m.dashEndDate = endDate
	}

	filter := interpreter.Filter{
		BeginDate: beginDate,
		EndDate:   endDate,
	}
	txns := m.interpreter.FilteredTransactions(filter)
	m.dashData = dashboard.ComputeDashboard(txns, beginDate, endDate)
}

// updateDashboard handles keyboard input for the dashboard view.
func (m Model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Custom date mode input handling
	if m.dashCustomMode {
		switch key {
		case "tab":
			// Switch focus between begin and end inputs
			if m.dashFocus == 1 {
				m.dashFocus = 2
				m.dashBeginInput.Blur()
				return m, m.dashEndInput.Focus()
			} else {
				m.dashFocus = 1
				m.dashEndInput.Blur()
				return m, m.dashBeginInput.Focus()
			}

		case "shift+tab":
			// Switch focus in reverse
			if m.dashFocus == 2 {
				m.dashFocus = 1
				m.dashEndInput.Blur()
				return m, m.dashBeginInput.Focus()
			} else {
				m.dashFocus = 2
				m.dashBeginInput.Blur()
				return m, m.dashEndInput.Focus()
			}

		case "enter":
			// Validate and apply custom dates
			beginStr := m.dashBeginInput.Value()
			endStr := m.dashEndInput.Value()

			_, errBegin := time.Parse("2006-01-02", beginStr)
			_, errEnd := time.Parse("2006-01-02", endStr)

			if errBegin != nil {
				m.message = "Invalid start date — use YYYY-MM-DD"
				m.messageErr = true
				return m, nil
			}
			if errEnd != nil {
				m.message = "Invalid end date — use YYYY-MM-DD"
				m.messageErr = true
				return m, nil
			}

			// Apply custom range
			m.dashMonths = 0 // 0 = custom range
			m.dashBeginDate = beginStr
			m.dashEndDate = endStr
			m.dashCustomMode = false
			m.dashBeginInput.Blur()
			m.dashEndInput.Blur()
			m.message = ""
			m.refreshDashboard()
			return m, nil
		}

		// Forward key to focused input
		var cmd tea.Cmd
		if m.dashFocus == 1 {
			m.dashBeginInput, cmd = m.dashBeginInput.Update(msg)
		} else {
			m.dashEndInput, cmd = m.dashEndInput.Update(msg)
		}
		return m, cmd
	}

	// Normal dashboard navigation
	switch key {
	case "c":
		// Enter custom date mode
		m.dashCustomMode = true
		m.dashFocus = 1
		m.dashBeginInput.SetValue(m.dashBeginDate)
		m.dashEndInput.SetValue(m.dashEndDate)
		return m, m.dashBeginInput.Focus()

	case "left", "h":
		if m.dashMonths > 1 {
			m.dashMonths--
			m.refreshDashboard()
		} else if m.dashMonths == 0 {
			// Switch from custom to preset
			m.dashMonths = 1
			m.refreshDashboard()
		}
		return m, nil

	case "right", "l":
		if m.dashMonths > 0 && m.dashMonths < 24 {
			m.dashMonths++
			m.refreshDashboard()
		}
		return m, nil

	case "1":
		m.dashMonths = 1
		m.refreshDashboard()
		return m, nil

	case "3":
		m.dashMonths = 3
		m.refreshDashboard()
		return m, nil

	case "6":
		m.dashMonths = 6
		m.refreshDashboard()
		return m, nil

	case "y":
		m.dashMonths = 12
		m.refreshDashboard()
		return m, nil
	}

	return m, nil
}

// viewDashboard renders the dashboard view.
func (m Model) viewDashboard() string {
	var b strings.Builder
	sep := styleSep.Render(strings.Repeat("─", m.width))

	if m.dashData == nil {
		b.WriteString("\n  Loading dashboard...\n")
		return b.String()
	}

	// Period selector
	b.WriteString(sep)
	b.WriteString("\n")
	periodStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	customStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	periods := []struct {
		months int
		label  string
	}{
		{1, "1M"},
		{3, "3M"},
		{6, "6M"},
		{12, "1Y"},
	}

	if m.dashCustomMode {
		// Show date input fields
		b.WriteString("  Custom range: ")
		b.WriteString(m.dashBeginInput.View())
		b.WriteString("  ")
		b.WriteString(m.dashEndInput.View())
		b.WriteString("  ")
		b.WriteString(styleLabel.Render("[tab] switch  [enter] apply  [esc] cancel"))
		b.WriteString("\n\n")
	} else {
		b.WriteString("  Period: ")
		for _, p := range periods {
			label := p.label
			if m.dashMonths == p.months {
				label = "[" + label + "]"
				b.WriteString(periodStyle.Render(label))
			} else {
				b.WriteString(styleLabel.Render(label))
			}
			b.WriteString("  ")
		}
		// Show custom as selected when dashMonths is 0
		if m.dashMonths == 0 {
			b.WriteString(customStyle.Render("[Custom]"))
		} else {
			b.WriteString(styleLabel.Render("Custom"))
		}
		b.WriteString("  ")
		b.WriteString(styleLabel.Render(fmt.Sprintf("(%s to %s)", m.dashBeginDate, m.dashEndDate)))
		b.WriteString("\n\n")
	}

	// Summary cards
	b.WriteString(styleTitle.Render("  SUMMARY"))
	b.WriteString("\n")
	s := m.dashData.Summary
	b.WriteString(fmt.Sprintf("  %-12s %s\n",
		styleLabel.Render("Income:"),
		stylePositive.Render(dashboard.FormatAmount(s.TotalIncome, "$"))))
	b.WriteString(fmt.Sprintf("  %-12s %s\n",
		styleLabel.Render("Expenses:"),
		styleNegative.Render(dashboard.FormatAmount(s.TotalExpenses, "$"))))

	netStyle := stylePositive
	if s.NetIncome < 0 {
		netStyle = styleNegative
	}
	b.WriteString(fmt.Sprintf("  %-12s %s\n",
		styleLabel.Render("Net:"),
		netStyle.Render(dashboard.FormatAmount(s.NetIncome, "$"))))

	savingsStyle := stylePositive
	if s.SavingsRate < 0 {
		savingsStyle = styleNegative
	}
	b.WriteString(fmt.Sprintf("  %-12s %s\n",
		styleLabel.Render("Savings:"),
		savingsStyle.Render(fmt.Sprintf("%.1f%%", s.SavingsRate))))
	b.WriteString("\n")

	// Monthly breakdown
	b.WriteString(styleTitle.Render("  MONTHLY BREAKDOWN"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %-7s  %12s  %12s  %12s\n", "Month", "Income", "Expenses", "Gain"))
	b.WriteString(styleSep.Render("  " + strings.Repeat("─", 50)))
	b.WriteString("\n")

	maxGain := 0.0
	for _, md := range m.dashData.Monthly {
		if md.Gain > maxGain {
			maxGain = md.Gain
		}
		if -md.Gain > maxGain {
			maxGain = -md.Gain
		}
	}

	for _, md := range m.dashData.Monthly {
		incomeStr := dashboard.FormatAmount(md.Income, "$")
		expenseStr := dashboard.FormatAmount(md.Expenses, "$")
		gainStr := dashboard.FormatAmount(md.Gain, "$")

		gainStyle := stylePositive
		if md.Gain < 0 {
			gainStyle = styleNegative
		}

		// Mini bar
		barWidth := 12
		bar := ""
		if maxGain > 0 {
			barLen := int((md.Gain / maxGain) * float64(barWidth))
			if barLen < 0 {
				barLen = -barLen
			}
			if barLen > barWidth {
				barLen = barWidth
			}
			if barLen < 1 && md.Gain != 0 {
				barLen = 1
			}
			if md.Gain >= 0 {
				bar = stylePositive.Render(strings.Repeat("█", barLen))
			} else {
				bar = styleNegative.Render(strings.Repeat("█", barLen))
			}
		}

		b.WriteString(fmt.Sprintf("  %-7s  %s  %s  %s  %s\n",
			md.Label,
			stylePositive.Render(fmt.Sprintf("%12s", incomeStr)),
			styleNegative.Render(fmt.Sprintf("%12s", expenseStr)),
			gainStyle.Render(fmt.Sprintf("%12s", gainStr)),
			bar))
	}
	b.WriteString("\n")

	// Top expenses
	b.WriteString(styleTitle.Render("  TOP EXPENSES"))
	b.WriteString("\n")
	maxExpense := 0.0
	for _, c := range m.dashData.TopExpenses {
		if c.Amount > maxExpense {
			maxExpense = c.Amount
		}
	}
	for _, c := range m.dashData.TopExpenses {
		barLen := 0
		if maxExpense > 0 {
			barLen = int((c.Amount / maxExpense) * 20)
		}
		if barLen < 1 {
			barLen = 1
		}
		bar := styleNegative.Render(strings.Repeat("█", barLen))
		b.WriteString(fmt.Sprintf("  %-15s %s %s\n",
			styleLabel.Render(truncateStr(c.Category, 15)),
			bar,
			styleValue.Render(dashboard.FormatAmount(c.Amount, "$"))))
	}
	b.WriteString("\n")

	// Footer
	b.WriteString(sep)
	b.WriteString("\n")
	if m.dashCustomMode {
		b.WriteString(styleFooter.Render(
			"  [tab] switch field  [enter] apply  [esc] cancel",
		))
	} else {
		b.WriteString(styleFooter.Render(
			"  [←→] change period  [1/3/6/y] quick select  [c] custom range  [esc] back  [q] quit",
		))
	}

	return b.String()
}

// truncateStr truncates a string to maxLen characters.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

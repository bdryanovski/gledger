package UI

import (
	"fmt"
	AST "gledger/ast"
	"gledger/config"
	Interpreter "gledger/interpreter"
	"gledger/utils"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Different mode of the UI
type ViewMode int

const (
	VIEW_LIST ViewMode = iota
	VIEW_ADD
	VIEW_REPORT
	VIEW_HELP
)

type Model struct {
	interpreter *Interpreter.Interpreter
	config      *config.Config
	currentView ViewMode
	table       table.Model
	formInputs  []textinput.Model
	formFocus   int
	message     string
	err         error
}

func InitialModel() (Model, error) {
	config, err := config.LoadConfig()
	if err != nil {
		return Model{}, fmt.Errorf("Error loading config: %v", err)
	}

	interpreter := Interpreter.NewInterpreter(config)

	if err := interpreter.LoadFromFile(config.DataFile); err != nil {
		fmt.Printf("Error loading transactions: %v\n", err)
	}

	columns := []table.Column{
		{Title: "Date", Width: 12},
		{Title: "Description", Width: 30},
		{Title: "Amount", Width: 10},
		{Title: "Account", Width: 20},
	}

	t := table.New(table.WithColumns(columns), table.WithFocused(true), table.WithHeight(15))

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)

	inputs := make([]textinput.Model, 5)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Date (YYYY-MM-DD)"
	inputs[0].Prompt = "Date: "
	inputs[0].CharLimit = 10
	inputs[0].Width = 30
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Description"
	inputs[1].Prompt = "Description: "
	inputs[1].CharLimit = 100
	inputs[1].Width = 50

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "account:name"
	inputs[2].Prompt = "Acount 1: "
	inputs[2].CharLimit = 50
	inputs[2].Width = 40

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "$0.00"
	inputs[3].Prompt = "Amount: "
	inputs[3].CharLimit = 50
	inputs[3].Width = 40

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "account:name"
	inputs[4].Prompt = "Account 2: "
	inputs[4].CharLimit = 50
	inputs[4].Width = 40

	m := Model{
		interpreter: interpreter,
		config:      config,
		currentView: VIEW_LIST,
		table:       t,
		formInputs:  inputs,
		formFocus:   0,
	}

	m.updateTableRows()

	return m, nil
}

func (model Model) Init() tea.Cmd {
	return textinput.Blink
}

func (model Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			/**
			* Save before you go go go
			 */
			if err := model.interpreter.SaveToFile(model.config.DataFile); err != nil {
				fmt.Printf("Error saving transactions: %v\n", err)
				model.err = err
			}
			return model, tea.Quit
		case "?":
			model.currentView = VIEW_HELP
			return model, nil
		case "a":
			model.currentView = VIEW_ADD
		case "r":
			model.currentView = VIEW_REPORT
		case "h":
			model.currentView = VIEW_HELP
		}

		switch model.currentView {
		case VIEW_LIST:
			return model.updateList(msg)
		case VIEW_ADD:
			return model.updateAdd(msg)
		case VIEW_REPORT:
			return model.updateReport(msg)
		}

	case tea.WindowSizeMsg:
		model.table.SetHeight(msg.Height - 10)
	}

	return model, cmd

}

func (model Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "a":
		model.currentView = VIEW_ADD
		model.formFocus = 0
		model.formInputs[0].Focus()

		model.formInputs[0].SetValue(time.Now().Format("2006-01-02"))
		return model, textinput.Blink

	case "r":
		model.currentView = VIEW_REPORT
	case "enter":
		return model, nil
	}

	model.table, cmd = model.table.Update(msg)
	return model, cmd

}

func (model Model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab", "up", "down":
		if msg.String() == "up" || msg.String() == "shift+tab" {
			model.formFocus--
		} else {
			model.formFocus++
		}

		if model.formFocus > len(model.formInputs)-1 {
			model.formFocus = 0
		} else if model.formFocus < 0 {
			model.formFocus = len(model.formInputs) - 1
		}

		cmds := make([]tea.Cmd, len(model.formInputs))
		for i := range model.formInputs {
			if i == model.formFocus {
				cmds[i] = model.formInputs[i].Focus()
			} else {
				model.formInputs[i].Blur()
			}
		}

		return model, tea.Batch(cmds...)
	case "enter":
		if err := model.submitTransaction(); err != nil {
			model.message = fmt.Sprintf("Error adding transaction: %v", err)
		} else {
			model.message = "Transaction added successfully!"
			model.currentView = VIEW_LIST
			model.updateTableRows()

			for i := range model.formInputs {
				if i == 0 {
					model.formInputs[i].SetValue(time.Now().Format("2006-01-02"))
				} else {
					model.formInputs[i].SetValue("")
				}
			}
		}
		return model, nil
	}

	var cmd tea.Cmd
	model.formInputs[model.formFocus], cmd = model.formInputs[model.formFocus].Update(msg)
	return model, cmd
}

func (model Model) updateReport(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Just wait for ESC to go back
	return model, nil
}

func (m *Model) submitTransaction() error {
	// Parse date
	date, err := time.Parse("2006-01-02", m.formInputs[0].Value())
	if err != nil {
		return fmt.Errorf("invalid date format")
	}

	description := m.formInputs[1].Value()
	if description == "" {
		return fmt.Errorf("description is required")
	}

	account1 := m.formInputs[2].Value()
	if account1 == "" {
		return fmt.Errorf("account 1 is required")
	}

	amount1Str := m.formInputs[3].Value()
	amount1, err := utils.ParseAmount(amount1Str)
	if err != nil {
		return fmt.Errorf("invalid amount 1")
	}

	account2 := m.formInputs[4].Value()
	if account2 == "" {
		return fmt.Errorf("account 2 is required")
	}

	// Create transaction
	txn := &AST.Transaction{
		Date:        date,
		Description: description,
		Postings: []AST.Posting{
			{Account: account1, Amount: amount1},
			{Account: account2, Amount: AST.Amount{Value: -amount1.Value, Currency: amount1.Currency}},
		},
	}

	// Add to engine
	if err := m.interpreter.AddTransaction(txn); err != nil {
		return err
	}

	if err := m.interpreter.SaveToFile(m.config.DataFile); err != nil {
		return fmt.Errorf("Error saving transactions: %v", err)
	}

	return nil
}

func (m *Model) updateTableRows() {
	txns := m.interpreter.GetTransactions()

	var rows []table.Row
	for _, txn := range txns {
		for _, posting := range txn.Postings {
			rows = append(rows, table.Row{
				txn.Date.Format("2006-01-02"),
				txn.Description,
				posting.Account,
				posting.Amount.String(),
			})
		}
	}

	m.table.SetRows(rows)
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00")).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 1)

	s.WriteString(headerStyle.Render("GLedger"))
	s.WriteString("\n\n")

	// Show any messages
	if m.message != "" {
		msgStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffff00")).
			Padding(0, 1)
		s.WriteString(msgStyle.Render(m.message))
		s.WriteString("\n\n")
	}

	// Render current view
	switch m.currentView {
	case VIEW_LIST:
		s.WriteString(m.viewList())
	case VIEW_ADD:
		s.WriteString(m.viewAdd())
	case VIEW_REPORT:
		s.WriteString(m.viewReport())
	case VIEW_HELP:
		s.WriteString(m.viewHelp())
	}

	return s.String()
}

func (m Model) viewList() string {
	var s strings.Builder

	s.WriteString("Transactions\n")
	s.WriteString("────────────────────────────────────────────────────────────────────────────\n\n")
	s.WriteString(m.table.View())
	s.WriteString("\n\n")

	// Show summary
	balances := m.interpreter.CalculateBalances()
	s.WriteString("Account Balances:\n")
	for account, balance := range balances {
		s.WriteString(fmt.Sprintf("  %-40s %10.2f\n", account, balance))
	}

	s.WriteString("\n")
	s.WriteString("Commands: [a]dd  [r]eport  [?]help  [q]uit\n")

	return s.String()
}

func (m Model) viewAdd() string {
	var s strings.Builder

	s.WriteString("Add New Transaction\n")
	s.WriteString("────────────────────────────────────────────────────────────────────────────\n\n")

	for i, input := range m.formInputs {
		s.WriteString(input.View())
		if i < len(m.formInputs)-1 {
			s.WriteString("\n")
		}
	}

	s.WriteString("\n\n")
	s.WriteString("Commands: [tab]next  [enter]save  [esc]cancel\n")

	return s.String()
}

func (m Model) viewReport() string {
	var s strings.Builder

	s.WriteString("Financial Reports\n")
	s.WriteString("────────────────────────────────────────────────────────────────────────────\n\n")

	report := m.interpreter.GenerateBalanceReport()
	s.WriteString(report)

	// Plugin reportss
	pluginReports := m.interpreter.GetPluginReports()
	for _, report := range pluginReports {
		s.WriteString("\n")
		s.WriteString(report)
	}

	s.WriteString("\nCommands: [esc]back\n")

	return s.String()
}

func (m Model) viewHelp() string {
	var s strings.Builder

	s.WriteString("Help\n")
	s.WriteString("────────────────────────────────────────────────────────────────────────────\n\n")

	s.WriteString("FinTrack is a double-entry accounting system.\n\n")

	s.WriteString("Key Concepts:\n")
	s.WriteString("  • Every transaction has at least 2 postings that balance to zero\n")
	s.WriteString("  • Positive amounts are debits, negative amounts are credits\n")
	s.WriteString("  • Transactions are stored in plain text files\n\n")

	s.WriteString("Keyboard Shortcuts:\n")
	s.WriteString("  a       - Add new transaction\n")
	s.WriteString("  r       - View reports\n")
	s.WriteString("  ?       - Show this help\n")
	s.WriteString("  q       - Quit (and save)\n")
	s.WriteString("  esc     - Go back\n")
	s.WriteString("  tab     - Navigate form fields\n\n")

	s.WriteString("File Format:\n")
	s.WriteString("  2024-01-15 Grocery Store\n")
	s.WriteString("      expenses:groceries        $45.32\n")
	s.WriteString("      assets:checking          -$45.32\n\n")

	s.WriteString("Config: ~/.fintrack/config.yaml\n")

	s.WriteString("Commands: [esc]back\n")

	return s.String()
}

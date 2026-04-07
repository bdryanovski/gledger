package UI

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"doublebook/db"
	"doublebook/fql"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// Styles
// ---------------------------------------------------------------------------

var (
	fqlStyleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	fqlStylePrompt  = lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Bold(true)
	fqlStyleQuery   = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
	fqlStyleResult  = lipgloss.NewStyle()
	fqlStyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	fqlStyleStatus  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	fqlStyleSep     = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	fqlStyleTime    = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	fqlStyleSugg    = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	fqlStyleWelcome = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// ---------------------------------------------------------------------------
// FQLModel
// ---------------------------------------------------------------------------

// FQLModel is the Bubbletea model for the fullscreen FQL REPL.
type FQLModel struct {
	// Database
	db *db.DB

	// Input line state
	input  []rune // current query being typed
	cursor int    // cursor position in input (rune index)

	// Query history
	history []string
	histIdx int // -1 = not browsing history

	// Output
	output   []string // accumulated output lines
	viewport viewport.Model

	// Completion
	completions []string
	compIdx     int

	// Status
	status   string // "Ready" | "Error: ..." | ""
	quitting bool

	// Dimensions
	width  int
	height int
}

// NewFQLModel creates a new FQL REPL model ready to run.
func NewFQLModel(database *db.DB, width, height int) FQLModel {
	vp := viewport.New(width, height-6) // reserve lines for header + prompt + status
	vp.SetContent(buildWelcome(width))

	return FQLModel{
		db:       database,
		viewport: vp,
		histIdx:  -1,
		status:   "Ready",
		width:    width,
		height:   height,
	}
}

// ---------------------------------------------------------------------------
// Bubbletea interface
// ---------------------------------------------------------------------------

func (m FQLModel) Init() tea.Cmd { return nil }

func (m FQLModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 6
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m FQLModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// ── Global quit ───────────────────────────────────────────────────────
	if key == "ctrl+c" || (key == "q" && len(m.input) == 0) {
		m.quitting = true
		return m, tea.Quit
	}

	// ── Clear completions on most keys ────────────────────────────────────
	if key != "tab" {
		m.completions = nil
		m.compIdx = 0
	}

	switch key {

	// ── Execute ───────────────────────────────────────────────────────────
	case "enter":
		query := strings.TrimSpace(string(m.input))
		if query == "" {
			return m, nil
		}
		m.input = nil
		m.cursor = 0
		m.histIdx = -1
		m.completions = nil
		return m.runQuery(query)

	// ── History navigation ────────────────────────────────────────────────
	case "up", "ctrl+p":
		if len(m.history) == 0 {
			return m, nil
		}
		if m.histIdx == -1 {
			m.histIdx = len(m.history) - 1
		} else if m.histIdx > 0 {
			m.histIdx--
		}
		m.input = []rune(m.history[m.histIdx])
		m.cursor = len(m.input)
		m.completions = nil

	case "down", "ctrl+n":
		if m.histIdx == -1 {
			return m, nil
		}
		m.histIdx++
		if m.histIdx >= len(m.history) {
			m.histIdx = -1
			m.input = nil
			m.cursor = 0
		} else {
			m.input = []rune(m.history[m.histIdx])
			m.cursor = len(m.input)
		}
		m.completions = nil

	// ── Output scrolling ──────────────────────────────────────────────────
	case "pgup":
		m.viewport.LineUp(m.viewport.Height / 2)
	case "pgdown":
		m.viewport.LineDown(m.viewport.Height / 2)

	// ── Clear ─────────────────────────────────────────────────────────────
	case "ctrl+l":
		m.output = nil
		m.viewport.SetContent("")
		m.status = "Ready"

	// ── Cursor movement ───────────────────────────────────────────────────
	case "ctrl+a", "home":
		m.cursor = 0
	case "ctrl+e", "end":
		m.cursor = len(m.input)
	case "left":
		if m.cursor > 0 {
			m.cursor--
		}
	case "right":
		if m.cursor < len(m.input) {
			m.cursor++
		}

	// ── Editing ───────────────────────────────────────────────────────────
	case "backspace":
		if m.cursor > 0 {
			m.input = append(m.input[:m.cursor-1], m.input[m.cursor:]...)
			m.cursor--
		}
	case "delete":
		if m.cursor < len(m.input) {
			m.input = append(m.input[:m.cursor], m.input[m.cursor+1:]...)
		}

	// ── Tab completion ────────────────────────────────────────────────────
	case "tab":
		m.handleTab()

	// ── Regular typing ────────────────────────────────────────────────────
	default:
		if len(msg.Runes) > 0 {
			for _, r := range msg.Runes {
				if unicode.IsPrint(r) {
					m.input = append(m.input[:m.cursor], append([]rune{r}, m.input[m.cursor:]...)...)
					m.cursor++
				}
			}
			m.status = "Ready"
		}
	}

	return m, nil
}

// ---------------------------------------------------------------------------
// Query execution
// ---------------------------------------------------------------------------

func (m FQLModel) runQuery(query string) (tea.Model, tea.Cmd) {
	m.status = "Running…"

	// Add to history (deduplicate).
	if len(m.history) == 0 || m.history[len(m.history)-1] != query {
		m.history = append(m.history, query)
		if len(m.history) > 100 {
			m.history = m.history[1:]
		}
	}

	start := time.Now()
	output, err := fql.Execute(query, m.db, m.width-2)
	elapsed := time.Since(start)

	// Build output block.
	var lines []string
	lines = append(lines, fqlStyleQuery.Render("FQL> "+query))
	if err != nil {
		lines = append(lines, fqlStyleError.Render("Error: "+err.Error()))
		m.status = "Error"
	} else {
		for _, l := range strings.Split(strings.TrimRight(output, "\n"), "\n") {
			lines = append(lines, l)
		}
		m.status = fmt.Sprintf("Done in %s", elapsed.Round(time.Millisecond))
	}
	lines = append(lines, fqlStyleTime.Render("("+elapsed.Round(100*time.Microsecond).String()+")"))
	lines = append(lines, fqlStyleSep.Render(strings.Repeat("─", m.width)))

	m.output = append(m.output, lines...)
	m.viewport.SetContent(strings.Join(m.output, "\n"))
	m.viewport.GotoBottom()

	return m, nil
}

// ---------------------------------------------------------------------------
// Tab completion
// ---------------------------------------------------------------------------

func (m *FQLModel) handleTab() {
	word := currentWord(m.input, m.cursor)
	if word == "" {
		return
	}

	// Build candidate list: table names + column names from all tables.
	var candidates []string
	lower := strings.ToLower(word)
	for name := range fql.Tables {
		if strings.HasPrefix(name, lower) {
			candidates = append(candidates, name)
		}
	}
	for _, tbl := range fql.Tables {
		for _, col := range tbl.Columns {
			if strings.HasPrefix(col.Name, lower) && !contains(candidates, col.Name) {
				candidates = append(candidates, col.Name)
			}
		}
	}

	if len(candidates) == 0 {
		return
	}

	if len(m.completions) == 0 {
		m.completions = candidates
		m.compIdx = 0
	} else {
		m.compIdx = (m.compIdx + 1) % len(m.completions)
	}

	completion := m.completions[m.compIdx]
	// Replace the current word with the completion.
	wordStart := m.cursor - len([]rune(word))
	m.input = append(m.input[:wordStart], append([]rune(completion), m.input[m.cursor:]...)...)
	m.cursor = wordStart + len([]rune(completion))
}

// currentWord returns the word immediately to the left of the cursor.
func currentWord(input []rune, cursor int) string {
	if cursor == 0 {
		return ""
	}
	end := cursor
	start := end
	for start > 0 && (unicode.IsLetter(input[start-1]) || unicode.IsDigit(input[start-1]) || input[start-1] == '_') {
		start--
	}
	return string(input[start:end])
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m FQLModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	sep := fqlStyleSep.Render(strings.Repeat("─", m.width))

	// ── Header ───────────────────────────────────────────────────────────
	header := fqlStyleHeader.Width(m.width).Render(" DoubleBook FQL — Financial Query Language")
	b.WriteString(header)
	b.WriteByte('\n')
	b.WriteString(sep)
	b.WriteByte('\n')

	// ── Output viewport ───────────────────────────────────────────────────
	b.WriteString(m.viewport.View())
	b.WriteByte('\n')

	// ── Prompt ────────────────────────────────────────────────────────────
	b.WriteString(sep)
	b.WriteByte('\n')
	b.WriteString(fqlStylePrompt.Render("FQL> "))
	b.WriteString(renderInput(m.input, m.cursor))
	b.WriteByte('\n')

	// ── Completions ───────────────────────────────────────────────────────
	if len(m.completions) > 0 {
		shown := m.completions
		if len(shown) > 5 {
			shown = shown[:5]
		}
		b.WriteString(fqlStyleSugg.Render("  [tab] " + strings.Join(shown, " · ")))
		b.WriteByte('\n')
	}

	// ── Status bar ────────────────────────────────────────────────────────
	statusLine := m.status
	hints := "Enter=run  ↑↓=history  PgUp/PgDn=scroll  Ctrl+L=clear  q=quit"
	pad := m.width - len(statusLine) - len(hints) - 2
	if pad < 1 {
		pad = 1
	}
	b.WriteString(fqlStyleStatus.Render(statusLine + strings.Repeat(" ", pad) + hints))

	return b.String()
}

// renderInput renders the input rune slice with a visible cursor block.
func renderInput(input []rune, cursor int) string {
	if len(input) == 0 {
		return lipgloss.NewStyle().Background(lipgloss.Color("57")).Render(" ")
	}
	var b strings.Builder
	for i, r := range input {
		if i == cursor {
			b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("57")).Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	if cursor == len(input) {
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color("57")).Render(" "))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Welcome message
// ---------------------------------------------------------------------------

func buildWelcome(width int) string {
	var b strings.Builder

	b.WriteString(fqlStyleWelcome.Render(strings.Repeat("─", width)))
	b.WriteByte('\n')
	b.WriteString(fqlStyleWelcome.Render("  Available virtual tables:"))
	b.WriteByte('\n')
	b.WriteByte('\n')
	b.WriteString(fql.AvailableTables())
	b.WriteByte('\n')
	b.WriteString(fqlStyleWelcome.Render("  Examples:"))
	b.WriteByte('\n')
	examples := []string{
		"  SELECT account, SUM(amount) AS total FROM accounts ORDER BY total DESC",
		"  SELECT date, description, amount FROM transactions WHERE amount < 0 LIMIT 20",
		"  SELECT month, SUM(total_amount) AS monthly FROM spending GROUP BY month",
		"  SELECT account, COUNT(*) AS cnt, AVG(amount) AS avg FROM transactions GROUP BY account",
	}
	for _, e := range examples {
		b.WriteString(fqlStyleQuery.Render(e))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(fqlStyleWelcome.Render(strings.Repeat("─", width)))

	return b.String()
}

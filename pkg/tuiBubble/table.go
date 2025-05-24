package tuiBubble

import (
	"fmt"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ = log.Fatal

// TableData is the data to be displayed in a table.
type TableData [][]string

// Get returns the value of the cell at the given row and column.
func (td TableData) Get(r, c int) string {
	if r >= 0 && r < len(td) && c >= 0 && c < len(td[r]) {
		return td[r][c]
	}
	return ""
}

// GetIndex returns the index of the specified column.
func (td TableData) GetIndex(key string) int {
	if len(td) == 0 {
		return -1
	}
	for i, v := range td[0] {
		if strings.EqualFold(v, key) {
			return i
		}
	}
	return -1
}

// Update updates the data at given row and column.
func (td TableData) Update(r, c int, val string) {
	if r >= 0 && r < len(td) && c >= 0 && c < len(td[r]) {
		td[r][c] = val
	}
}

// TableStyle sets the style of the table.
type TableStyle struct {
	SelectionBackground string
	SelectionForeground string
	SelectionTextIsBold bool
}

// SelectedFunc is fired when a user press enter key in the table cell.
type SelectedFunc func(row, column int, data interface{})

// CopyFunc is fired when a user press 'c' character in the table cell.
type CopyFunc func(row, column int, data interface{})

// Table is a bubble tea model for rendering tables.
type Table struct {
	table        table.Model
	tableData    TableData
	style        TableStyle
	footerText   string
	helpText     string
	selectedFunc SelectedFunc
	copyFunc     CopyFunc
	showHelp     bool
	baseStyle    lipgloss.Style
	helpStyle    lipgloss.Style
	footerStyle  lipgloss.Style

	rawWidth       int
	rawHeight      int
	viewportWidth  int
	viewportHeight int

	footerHeight int
	helpHeight   int

	err error
}

type WidgetSizeMsg struct {
	Width  int
	Height int
}

// TableOption is a functional option to wrap table properties.
type TableOption func(*Table)

// NewTable constructs a new table model.
func NewTable(opts ...TableOption) *Table {
	baseStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	footerStyle := lipgloss.NewStyle().
		Padding(0, 0, 1, 2).
		Foreground(lipgloss.Color("240"))

	helpStyle := lipgloss.NewStyle().
		Padding(1, 0, 0, 2).
		Foreground(lipgloss.Color("240"))

	t := &Table{
		baseStyle:   baseStyle,
		footerStyle: footerStyle,
		helpStyle:   helpStyle,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// WithTableStyle sets the style of the table.
func WithTableStyle(style TableStyle) TableOption {
	return func(t *Table) {
		t.style = style
	}
}

// WithTableFooterText sets footer text that is displayed after the table.
func WithTableFooterText(text string) TableOption {
	return func(t *Table) {
		t.footerText = text
	}
}

// WithTableHelpText sets the help text for the view.
func WithTableHelpText(text string) TableOption {
	return func(t *Table) {
		t.helpText = text
	}
}

// WithSelectedFunc sets a func that is triggered when table row is selected.
func WithSelectedFunc(fn SelectedFunc) TableOption {
	return func(t *Table) {
		t.selectedFunc = fn
	}
}

// WithCopyFunc sets a func that is triggered when a user press 'c'.
func WithCopyFunc(fn CopyFunc) TableOption {
	return func(t *Table) {
		t.copyFunc = fn
	}
}

// Init initializes the table model.
func (t *Table) Init() tea.Cmd {
	return nil
}

func (t *Table) columnWidth() (int, int) {
	numColumns := len(t.tableData[0])

	availableSpace := t.viewportWidth

	availableSpace -= 2 * numColumns // this was the most difficult part. Each column is really ' ' + width + ' ', there is an implicit padding of 2 per column

	colWidth := availableSpace / numColumns
	if colWidth < 10 {
		colWidth = 10 // Minimum column width
	}

	remainder := availableSpace - colWidth*numColumns
	return colWidth, remainder
}

// Update handles user input and updates the table model state.
func (t *Table) Update(msg tea.Msg) (*Table, tea.Cmd) {
	switch msg := msg.(type) {
	case WidgetSizeMsg:
		t.rawWidth = msg.Width
		t.rawHeight = msg.Height

		t.footerHeight = 4
		t.helpHeight = 4

		t.viewportWidth = msg.Width - 2 // table external border
		t.viewportHeight = msg.Height - t.footerHeight - t.helpHeight
	}
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return t, cmd
}

func (t *Table) SetData(data TableData) {
	t.tableData = data
}

func (t *Table) SetUnderlyingTable() {
	t.table = table.New(
		table.WithFocused(true),
	)

	// Set up table styles
	st := table.DefaultStyles()
	st.Header = st.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(240)).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("24"))

	// Set selection colors based on provided style
	if t.style.SelectionBackground != "" {
		bg := lipgloss.Color(t.style.SelectionBackground)
		st.Selected = st.Selected.Background(bg)
	} else {
		st.Selected = st.Selected.Background(lipgloss.Color("57"))
	}

	if t.style.SelectionForeground != "" {
		fg := lipgloss.Color(t.style.SelectionForeground)
		st.Selected = st.Selected.Foreground(fg)
	} else {
		st.Selected = st.Selected.Foreground(lipgloss.Color("229"))
	}

	st.Selected = st.Selected.Bold(t.style.SelectionTextIsBold)
	t.table.SetStyles(st)
}

// View renders the table.
func (t *Table) View() string {
	var s strings.Builder

	data := t.tableData
	columns := make([]table.Column, len(data[0]))
	for i, col := range data[0] {
		oneWidth, rem := t.columnWidth()
		columns[i] = table.Column{
			Title: col,
			Width: oneWidth,
		}
		if i == len(data[0])-1 {
			columns[i].Width = oneWidth + rem
		}
	}

	rows := make([]table.Row, len(data)-1)
	for i := 1; i < len(data); i++ {
		row := make(table.Row, len(data[i]))
		for j, cell := range data[i] {
			row[j] = cell
		}
		rows[i-1] = row
	}

	t.table.SetColumns(columns)
	t.table.SetRows(rows)
	t.table.SetHeight(t.viewportHeight)
	t.table.SetWidth(t.viewportWidth)

	// Render the table
	tableView := t.baseStyle.Render(t.table.View())
	s.WriteString(tableView)

	// Render the footer
	if t.footerText != "" {
		s.WriteString("\n")
		s.WriteString(t.footerStyle.Render(t.footerText))
	}

	// Render the help text if visible
	if t.helpText != "" {
		s.WriteString(t.helpStyle.Render(t.helpText))
	}

	// Render error if there is one
	if t.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Padding(0, 0, 1, 2)
		s.WriteString("\n")
		s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", t.err)))

		// Clear the error after showing it once
		t.err = nil
	}

	return s.String()
}

// Accessor methods for IssueList to use
// GetCursorRow returns the current cursor row index
func (t *Table) GetCursorRow() int {
	return t.table.Cursor()
}

// GetSelectedFunc returns the selectedFunc
func (t *Table) GetSelectedFunc() SelectedFunc {
	return t.selectedFunc
}

// GetCopyFunc returns the copyFunc
func (t *Table) GetCopyFunc() CopyFunc {
	return t.copyFunc
}

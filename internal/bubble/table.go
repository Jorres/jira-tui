package bubble

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/jorres/jira-tui/api"
	forkedTable "github.com/jorres/jira-tui/internal/bubble/table"
	"github.com/jorres/jira-tui/internal/debug"
	"github.com/jorres/jira-tui/pkg/jira"
	"github.com/jorres/jira-tui/pkg/jira/filter/issue"
)

var _ = debug.Debug

const (
	SorterInactive int = iota
	SorterFiltering
	SorterActive
)

const (
	sorterHeight = 3
)

// TableData is the data to be displayed in a table.
type TableData [][]string

// Table is a bubble tea model for rendering tables.
type Table struct {
	table       forkedTable.Model
	footerText  string
	helpText    string
	showHelp    bool
	baseStyle   lipgloss.Style
	helpStyle   lipgloss.Style
	footerStyle lipgloss.Style

	rawWidth       int
	rawHeight      int
	viewportWidth  int
	viewportHeight int

	SorterState  int
	sorterHeight int
	sorterText   string
	sorterStyle  lipgloss.Style

	footerHeight int
	helpHeight   int

	err error

	columns  []string
	timezone string

	allIssues      []*jira.Issue
	filteredIssues []*jira.Issue
	issueCache     map[string]*jira.Issue

	// Data provider for getting table data
	dataProvider DataProvider

	// Background color resolver function
	backgroundColorResolver func(issueKey string) *color.Color

	// Spinner for loading state
	spinner spinner.Model
}

// TableOption is a functional option to wrap table properties.
type TableOption func(*Table)

// NewTable constructs a new table model.
func NewTable(opts ...TableOption) *Table {
	baseStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(getPaleColor()))

	footerStyle := lipgloss.NewStyle().
		Padding(0, 0, 1, 2).
		Foreground(lipgloss.Color(getPaleColor()))

	helpStyle := lipgloss.NewStyle().
		Padding(1, 0, 0, 2).
		Foreground(lipgloss.Color(getPaleColor()))

	sorterStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(getPaleColor())).
		Padding(0, 1).
		Height(1)

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(getAccentColor()))

	t := &Table{
		baseStyle:    baseStyle,
		footerStyle:  footerStyle,
		helpStyle:    helpStyle,
		sorterStyle:  sorterStyle,
		sorterHeight: sorterHeight,
		spinner:      s,
	}

	t.table = forkedTable.New(
		forkedTable.WithFocused(true),
	)

	// Set up table styles
	st := forkedTable.DefaultStyles()
	st.Header = st.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(getPaleColor())).
		BorderBottom(true).
		Bold(true).
		Background(lipgloss.Color(getPaleColor()))

	st.Selected = st.Selected.Background(lipgloss.Color(getAccentColor()))
	st.Selected = st.Selected.Foreground(lipgloss.Color("229"))

	t.table.SetStyles(st)

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// WithTableHelpText sets the help text for the view.
func WithTableHelpText(text string) TableOption {
	return func(t *Table) {
		t.helpText = text
	}
}

// Init initializes the table model.
func (t *Table) Init() tea.Cmd {
	return nil
}

func (t *Table) columnWidth(columnName string, data TableData) int {
	if len(data) == 0 || len(data[0]) == 0 {
		return 10 // fallback
	}

	numColumns := len(data[0])

	additionalSpaceForSummary := 10

	availableSpace := t.viewportWidth - additionalSpaceForSummary

	availableSpace -= 2 * numColumns // Implicitly, bubbletea's table's columns are really ' ' + width + ' '. There is an implicit padding of 2 per column

	colWidth := availableSpace / numColumns
	if colWidth < 10 {
		colWidth = 10 // Minimum column width
	}

	defaultWidth := colWidth
	remainder := availableSpace - colWidth*numColumns

	if columnName == FieldSummary {
		return defaultWidth + remainder + additionalSpaceForSummary
	}

	return defaultWidth
}

func (t *Table) filterTableData(filterText string) {
	t.filteredIssues = []*jira.Issue{}

	// Special case: when just entered search, we should not
	// immediately yank all content from under user's nose
	if filterText == "" {
		t.filteredIssues = t.allIssues
		return
	}

	for _, iss := range t.allIssues {
		if strings.Contains(iss.Key, filterText) || strings.Contains(
			strings.ToLower(iss.Fields.Summary),
			strings.ToLower(filterText),
		) {
			t.filteredIssues = append(t.filteredIssues, iss)
		}
	}
}

// Update handles user input and updates the table model state.
func (t *Table) Update(msg tea.Msg) (*Table, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case WidgetSizeMsg:
		t.rawWidth = msg.Width
		t.rawHeight = msg.Height

		t.footerHeight = 2
		t.helpHeight = 2

		t.viewportWidth = msg.Width - 2                                   // table external border
		t.viewportHeight = msg.Height - t.footerHeight - t.helpHeight - 2 // table external border
	case tea.KeyMsg:

		if t.SorterState == SorterFiltering {
			switch msg.String() {
			case "enter":
				t.SorterState = SorterActive
				t.filterTableData(t.sorterText)
				t.viewportHeight += sorterHeight
				return t, cmd
			case "esc", "ctrl+c":
				t.SorterState = SorterInactive
				t.viewportHeight += sorterHeight
				return t, cmd
			case "backspace":
				if len(t.sorterText) > 0 {
					t.sorterText = t.sorterText[:len(t.sorterText)-1]
				}
				t.filterTableData(t.sorterText)
				return t, cmd
			default:
				t.sorterText = t.sorterText + msg.String()
				t.filterTableData(t.sorterText)
				if len(t.filteredIssues) > 0 {
					t.table.SetCursor(0)
				}
				return t, cmd
			}
		}

		switch msg.String() {
		case "/":
			t.viewportHeight -= sorterHeight
			t.sorterText = ""
			t.SorterState = SorterFiltering
			t.filterTableData(t.sorterText)
			return t, cmd
		}
	}

	// Update spinner if we don't have data yet
	if t.allIssues == nil {
		t.spinner, cmd = t.spinner.Update(msg)
		return t, cmd
	}

	t.table, cmd = t.table.Update(msg)
	return t, cmd
}

// SetIssueData sets the issue data for the table
func (t *Table) SetIssueData(issues []*jira.Issue) {
	t.allIssues = issues
	if t.issueCache == nil {
		t.issueCache = make(map[string]*jira.Issue)
	}
}

func (t *Table) SetBacklightResolver(resolver func(string) *color.Color) {
	t.backgroundColorResolver = resolver
}

// GetIssueData returns the current issue data
func (t *Table) GetIssueData() []*jira.Issue {
	return t.allIssues
}

// GetDetailedCache returns the detailed issue cache
func (t *Table) GetDetailedCache() map[string]*jira.Issue {
	return t.issueCache
}

// SetDetailedCache sets the detailed issue cache
func (t *Table) SetDetailedCache(cache map[string]*jira.Issue) {
	t.issueCache = cache
}

// DataProvider interface allows external components to provide table data
type DataProvider interface {
	GetTableData() TableData
}

// SetDataProvider sets the data provider for the table
func (t *Table) SetDataProvider(provider DataProvider) {
	t.dataProvider = provider
}

func (t *Table) setInnerTableColumnsRows() {
	var data TableData
	var issues []*jira.Issue
	if t.SorterState == SorterInactive {
		data = t.makeTableData(t.allIssues)
		issues = t.allIssues
	} else {
		data = t.makeTableData(t.filteredIssues)
		issues = t.filteredIssues
	}

	columns := make([]forkedTable.Column, len(data[0]))
	for i, col := range data[0] {
		oneWidth := t.columnWidth(col, data)
		columns[i] = forkedTable.Column{
			Title: col,
			Width: oneWidth,
		}
	}

	rows := make([]forkedTable.Row, len(data)-1)
	for i := 1; i < len(data); i++ {
		row := make(forkedTable.Row, len(data[i]))
		for j, cell := range data[i] {
			row[j] = cell
		}
		rows[i-1] = row
	}

	t.table.SetColumns(columns)
	t.table.SetRows(rows)

	for i, issue := range issues {
		if i < len(rows) {
			backgroundColor := t.backgroundColorResolver(issue.Key)

			if backgroundColor == nil {
				continue
			}

			// Apply custom background color only to the first column
			rowStyles := make([]lipgloss.Style, len(columns))
			for j := range rowStyles {
				if j == 0 {
					// Apply background color only to first column
					rowStyles[j] = forkedTable.DefaultStyles().Cell.Background(*backgroundColor)
				} else {
					// Use default cell style for other columns
					rowStyles[j] = forkedTable.DefaultStyles().Cell
				}
			}
			t.table.SetRowStyles(i, rowStyles)
		}
	}
}

// View renders the table.
func (t *Table) View() string {
	// Show spinner if no issues loaded yet
	if t.allIssues == nil {
		spinnerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(getAccentColor())).
			Align(lipgloss.Center).
			Width(t.viewportWidth).
			Height(t.viewportHeight)

		spinnerContent := fmt.Sprintf("%s Loading issues...", t.spinner.View())
		return t.baseStyle.Render(spinnerStyle.Render(spinnerContent))
	}

	if len(t.allIssues) == 0 {
		// Show centered "No issues found" message when issue list is empty
		emptyStyle := lipgloss.NewStyle().
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Width(t.viewportWidth).
			Height(t.viewportHeight)

		emptyContent := emptyStyle.Render("No issues found")
		return t.baseStyle.Render(emptyContent)
	}

	var s strings.Builder
	var viewComponents []string

	if t.SorterState == SorterFiltering {
		headerContent := t.sorterStyle.Width(t.viewportWidth).Render("/" + t.sorterText)
		viewComponents = append(viewComponents, headerContent)
	}

	t.setInnerTableColumnsRows()

	t.table.SetHeight(t.viewportHeight)
	t.table.SetWidth(t.viewportWidth)

	// Render the table
	tableView := t.baseStyle.Render(t.table.View())
	viewComponents = append(viewComponents, tableView)

	// Join header and table vertically
	if len(viewComponents) > 1 {
		s.WriteString(lipgloss.JoinVertical(lipgloss.Left, viewComponents...))
	} else {
		s.WriteString(tableView)
	}

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

// SetFooterText updates the footer text dynamically
func (t *Table) SetFooterText(text string) {
	t.footerText = text
}

func (t *Table) SetDefaultFooterText() {
	t.footerText = fmt.Sprintf("")
}

func (t *Table) SetColumns(columns []string) {
	t.columns = columns
}

func (t *Table) SetTimezone(timezone string) {
	t.timezone = timezone
}

// data prepares the data for table view.
func (t *Table) makeTableData(issues []*jira.Issue) TableData {
	var data TableData

	headers := t.header()
	data = append(data, headers)
	for _, iss := range issues {
		data = append(data, t.assignColumns(headers, iss))
	}

	return data
}

// header prepares table headers.
func (t *Table) header() []string {
	headers := []string{}
	for _, c := range t.columns {
		c = strings.ToUpper(c)
		if slices.Contains(ValidIssueColumns(), c) {
			headers = append(headers, c)
		}
	}

	return headers
}

// assignColumns assigns columns for the issue.
func (t *Table) assignColumns(columns []string, issue *jira.Issue) []string {
	var bucket []string

	for _, column := range columns {
		switch column {
		case FieldType:
			bucket = append(bucket, issue.Fields.IssueType.Name)
		case FieldParent:
			if issue.Fields.Parent != nil {
				bucket = append(bucket, issue.Fields.Parent.Key)
			} else {
				bucket = append(bucket, "")
			}
		case FieldKey:
			bucket = append(bucket, issue.Key)
		case FieldSummary:
			bucket = append(bucket, prepareTitle(issue.Fields.Summary))
		case FieldStatus:
			bucket = append(bucket, issue.Fields.Status.Name)
		case FieldAssignee:
			bucket = append(bucket, issue.Fields.Assignee.Name)
		case FieldReporter:
			bucket = append(bucket, issue.Fields.Reporter.Name)
		case FieldPriority:
			bucket = append(bucket, issue.Fields.Priority.Name)
		case FieldResolution:
			bucket = append(bucket, issue.Fields.Resolution.Name)
		case FieldCreated:
			bucket = append(bucket, FormatDateTime(issue.Fields.Created, jira.RFC3339, t.timezone))
		case FieldUpdated:
			bucket = append(bucket, FormatDateTime(issue.Fields.Updated, jira.RFC3339, t.timezone))
		case FieldLabels:
			bucket = append(bucket, strings.Join(issue.Fields.Labels, ","))
		}
	}
	return bucket
}

func (t *Table) GetIssueSync(shift int) *jira.Issue {
	key := t.getKeyUnderCursorWithShift(shift)

	if iss, ok := t.issueCache[key]; ok {
		return iss
	}

	iss, err := api.ProxyGetIssue(api.DefaultClient(false), key, issue.NewNumCommentsFilter(10))
	if err != nil {
		panic(err)
	}

	t.issueCache[key] = iss

	return iss
}

func (t *Table) getKeyUnderCursorWithShift(shift int) string {
	row := t.GetCursorRow()
	var issuePool []*jira.Issue
	if t.SorterState == SorterInactive {
		issuePool = t.allIssues
	} else {
		issuePool = t.filteredIssues
	}
	pos := row + shift
	if pos < 0 {
		pos = 0
	}
	if pos >= len(issuePool) {
		pos = len(issuePool) - 1
	}

	if pos == -1 {
		return ""
	}

	return issuePool[pos].Key
}

func (t *Table) GetIssueAsync(i int, shift int) tea.Cmd {
	key := t.getKeyUnderCursorWithShift(shift)
	return func() tea.Msg {
		if key == "" {
			return NopMsg{}
		}

		if iss, ok := t.issueCache[key]; ok {
			return IncomingIssueMsg{index: i, issue: iss}
		}

		iss, err := api.ProxyGetIssue(api.DefaultClient(false), key, issue.NewNumCommentsFilter(10))
		if err != nil {
			panic(err)
		}

		t.issueCache[key] = iss
		return IncomingIssueMsg{index: i, issue: iss}
	}
}

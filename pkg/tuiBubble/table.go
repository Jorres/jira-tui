package tuiBubble

import (
	"fmt"
	"strings"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/jira/filter/issue"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	SorterInactive int = iota
	SorterFiltering
	SorterActive
)

const (
	sorterHeight = 3
)

// DisplayFormat is a issue display type.
type DisplayFormat struct {
	Plain        bool
	NoHeaders    bool
	NoTruncate   bool
	FixedColumns uint
	Columns      []string
	Timezone     string
}

// TableData is the data to be displayed in a table.
type TableData [][]string

// TableStyle sets the style of the table.
type TableStyle struct {
	SelectionBackground string
	SelectionForeground string
	SelectionTextIsBold bool
}

// Table is a bubble tea model for rendering tables.
type Table struct {
	table       table.Model
	style       TableStyle
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

	displayFormat DisplayFormat

	allIssues      []*jira.Issue
	filteredIssues []*jira.Issue
	issueCache     map[string]*jira.Issue

	// Data provider for getting table data
	dataProvider DataProvider
}

type WidgetSizeMsg struct {
	Width  int
	Height int
}

type CurrentIssueReceivedMsg struct {
	Table *Table
	Issue *jira.Issue
	Pos   int
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

	sorterStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1).
		Height(1)

	t := &Table{
		baseStyle:    baseStyle,
		footerStyle:  footerStyle,
		helpStyle:    helpStyle,
		sorterStyle:  sorterStyle,
		sorterHeight: sorterHeight,
	}

	t.table = table.New(
		table.WithFocused(true),
	)

	// Set up table styles
	st := table.DefaultStyles()
	st.Header = st.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
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

func (t *Table) columnWidth(data TableData) (int, int) {
	if len(data) == 0 || len(data[0]) == 0 {
		return 10, 0 // fallback
	}
	numColumns := len(data[0])

	availableSpace := t.viewportWidth

	availableSpace -= 2 * numColumns // this was the most difficult part. Each column is really ' ' + width + ' ', there is an implicit padding of 2 per column

	colWidth := availableSpace / numColumns
	if colWidth < 10 {
		colWidth = 10 // Minimum column width
	}

	remainder := availableSpace - colWidth*numColumns
	return colWidth, remainder
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

// View renders the table.
func (t *Table) View() string {
	var s strings.Builder
	var viewComponents []string

	if t.SorterState == SorterFiltering {
		headerContent := t.sorterStyle.Width(t.viewportWidth).Render("/" + t.sorterText)
		viewComponents = append(viewComponents, headerContent)
	}

	var data TableData
	if t.SorterState == SorterInactive {
		data = t.makeTableData(t.allIssues)
	} else {
		data = t.makeTableData(t.filteredIssues)
	}

	if len(data) == 0 || len(data[0]) == 0 {
		// Return empty view if no data
		return ""
	}

	columns := make([]table.Column, len(data[0]))
	for i, col := range data[0] {
		oneWidth, rem := t.columnWidth(data)
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

func (t *Table) SetDisplayFormat(displayFormat DisplayFormat) {
	t.displayFormat = displayFormat
}

// data prepares the data for table view.
func (t *Table) makeTableData(issues []*jira.Issue) TableData {
	var data TableData

	headers := t.header()
	if !(t.displayFormat.Plain && t.displayFormat.NoHeaders) {
		data = append(data, headers)
	}
	for _, iss := range issues {
		data = append(data, t.assignColumns(headers, iss))
	}

	return data
}

// header prepares table headers.
func (t *Table) header() []string {
	if len(t.displayFormat.Columns) > 0 {
		headers := []string{}
		columnsMap := t.validColumnsMap()
		for _, c := range t.displayFormat.Columns {
			c = strings.ToUpper(c)
			if _, ok := columnsMap[c]; ok {
				headers = append(headers, strings.ToUpper(c))
			}
		}

		return headers
	}

	return validIssueColumns()
}

// validColumnsMap returns a map of valid columns.
func (*Table) validColumnsMap() map[string]struct{} {
	columns := validIssueColumns()
	out := make(map[string]struct{}, len(columns))

	for _, c := range columns {
		out[c] = struct{}{}
	}

	return out
}

// validIssueColumns returns valid columns for issue list.
func validIssueColumns() []string {
	return []string{
		FieldKey,
		FieldType,
		FieldParent,
		FieldSummary,
		FieldStatus,
		FieldAssignee,
		FieldReporter,
		// FieldResolution,
		FieldCreated,
		FieldPriority,
		FieldUpdated,
		// FieldLabels,
	}
}

// assignColumns assigns columns for the issue.
func (t *Table) assignColumns(columns []string, issue *jira.Issue) []string {
	var bucket []string

	for _, column := range columns {
		switch column {
		case FieldType:
			bucket = append(bucket, issue.Fields.IssueType.Name)
		case FieldKey:
			bucket = append(bucket, issue.Key)
		case FieldParent:
			if issue.Fields.Parent != nil {
				bucket = append(bucket, issue.Fields.Parent.Key)
			} else {
				bucket = append(bucket, "")
			}
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
			bucket = append(bucket, formatDateTime(issue.Fields.Created, jira.RFC3339, t.displayFormat.Timezone))
		case FieldUpdated:
			bucket = append(bucket, formatDateTime(issue.Fields.Updated, jira.RFC3339, t.displayFormat.Timezone))
		case FieldLabels:
			bucket = append(bucket, strings.Join(issue.Fields.Labels, ","))
		}
	}
	return bucket
}

func (t *Table) GetIssueSync(shift int) *jira.Issue {
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

	key := issuePool[pos].Key

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

func (t *Table) ScheduleIssueUpdateMessage(shift int) tea.Cmd {
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

	key := issuePool[pos].Key

	return func() tea.Msg {
		if iss, ok := t.issueCache[key]; ok {
			return CurrentIssueReceivedMsg{
				Table: t,
				Issue: iss,
				Pos:   pos,
			}
		}

		iss, err := api.ProxyGetIssue(api.DefaultClient(false), key, issue.NewNumCommentsFilter(10))
		if err != nil {
			panic(err)
		}

		t.issueCache[key] = iss
		return CurrentIssueReceivedMsg{
			Table: t,
			Issue: iss,
			Pos:   pos,
		}
	}
}

func (t *Table) RefreshCache(issues []*jira.Issue) {
	t.allIssues = issues
	t.issueCache = make(map[string]*jira.Issue)
}

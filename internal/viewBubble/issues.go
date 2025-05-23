package viewBubble

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/jira/filter/issue"
	"github.com/ankitpokhrel/jira-cli/pkg/tuiBubble"
	"github.com/atotto/clipboard"
	"github.com/spf13/viper"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ = log.Fatal

// DisplayFormat is a issue display type.
type DisplayFormat struct {
	Plain        bool
	NoHeaders    bool
	NoTruncate   bool
	Columns      []string
	FixedColumns uint
	Timezone     string
}

// IssueList is a list view for issues.
type IssueList struct {
	Total          int
	Project        string
	Server         string
	Data           []*jira.Issue
	Display        DisplayFormat
	DetailedCache  map[string]*jira.Issue
	FetchAllIssues func() ([]*jira.Issue, int)
	FooterText     string

	table *tuiBubble.Table
	err   error

	width  int
	height int

	// Split view related fields
	showSplitView   bool
	issueDetailView *Issue
	activeView      int
}

// Init initializes the IssueList model.
func (l *IssueList) Init() tea.Cmd {
	// Enable split view by default
	l.showSplitView = true
	l.activeView = issueListMode

	// Wait for window size before loading issues
	return nil
}

// fetchSelectedIssueCmd creates a command to fetch the currently selected issue
func (l *IssueList) fetchSelectedIssueCmd() tea.Cmd {
	return func() tea.Msg {
		return selectedIssueMsg{issue: l.GetSelectedIssueShift(0)}
	}
}

type editorFinishedMsg struct{ err error }
type issueMovedMsg struct{ err error }
type selectedIssueMsg struct{ issue *jira.Issue }

// View mode constants
const (
	issueListMode int = iota
	issueDetailMode
)

func (l *IssueList) editIssue(issue *jira.Issue) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	args := []string{}

	config := viper.GetString("config")
	if config != "" {
		args = append(args,
			"-c",
			config,
		)
	}

	args = append(args,
		"issue",
		"edit",
		issue.Key,
	)

	c := exec.Command("jira", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

func (l *IssueList) moveIssue(issue *jira.Issue) tea.Cmd {
	args := []string{}

	config := viper.GetString("config")
	if config != "" {
		args = append(args,
			"-c",
			config,
		)
	}

	args = append(args,
		"issue",
		"move",
		issue.Key,
	)

	c := exec.Command("jira", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return issueMovedMsg{err}
	})
}

func (l *IssueList) GetSelectedIssueShift(shift int) *jira.Issue {
	row := l.table.GetCursorRow()
	tableData := l.data()
	totalIssues := len(tableData) - 1 // because of headers
	pos := row + shift
	if pos < 0 {
		pos = 0
	}
	if pos >= totalIssues {
		pos = totalIssues - 1
	}

	pos = pos + 1 // because of headers

	ci := tableData.GetIndex(fieldKey)
	key := tableData.Get(pos, ci)

	// check if in cache
	if iss, ok := l.DetailedCache[key]; ok {
		return iss
	}

	// fetch
	iss, err := api.ProxyGetIssue(api.DefaultClient(false), key, issue.NewNumCommentsFilter(10))
	if err != nil {
		panic(err)
	}

	// store in cache
	l.DetailedCache[key] = iss
	return iss
}

func (l *IssueList) safeIssueUpdate(msg tea.Msg) (*Issue, tea.Cmd) {
	m, cmd := l.issueDetailView.Update(msg)
	if v, ok := m.(*Issue); ok {
		return v, cmd
	} else {
		panic("expected *Issue from some of the issue model updates")
	}
}

// Update handles user input and updates the model state.
func (l *IssueList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Store the full window size
		l.width = msg.Width
		l.height = msg.Height

		// First time we get a window size, fetch the first issue
		var fetchCmd tea.Cmd

		var cmds []tea.Cmd

		if l.showSplitView {
			tableHeight := l.height / 2
			previewHeight := l.height - tableHeight

			l.table, cmd = l.table.Update(tuiBubble.WidgetSizeMsg{
				Height: tableHeight,
				Width:  l.width,
			})
			cmds = append(cmds, cmd)

			l.issueDetailView, cmd = l.safeIssueUpdate(tuiBubble.WidgetSizeMsg{
				Height: previewHeight,
				Width:  l.width,
			})
			cmds = append(cmds, cmd)
		} else {
			l.table, cmd = l.table.Update(tuiBubble.WidgetSizeMsg{
				Height: l.height,
				Width:  l.width,
			})
			cmds = append(cmds, cmd)
		}

		cmds = append(cmds, fetchCmd)
		return l, tea.Batch(cmds...)
	case selectedIssueMsg:
		l.issueDetailView, cmd = l.safeIssueUpdate(msg)
		return l, cmd
	case editorFinishedMsg, issueMovedMsg:
		l.FetchAndRefreshCache()
		l.table.SetData(l.data())
		if l.showSplitView {
			return l, l.fetchSelectedIssueCmd()
		}
		return l, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			var cmd1, cmd2 tea.Cmd
			l.issueDetailView, cmd1 = l.safeIssueUpdate(l.GetSelectedIssueShift(-1))
			l.table, cmd2 = l.table.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "down", "j":
			var cmd1, cmd2 tea.Cmd
			l.issueDetailView, cmd1 = l.safeIssueUpdate(l.GetSelectedIssueShift(+1))
			l.table, cmd2 = l.table.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "ctrl+c", "q", "esc":
			return l, tea.Quit
		case "tab":
			if l.showSplitView {
				if l.activeView == issueListMode {
					l.activeView = issueDetailMode
				} else {
					l.activeView = issueListMode
				}
			}
			return l, nil
		case "m":
			return l, l.moveIssue(l.GetSelectedIssueShift(0))
		case "v":
			// If we're in split view, just toggle to full-screen detail view
			if l.showSplitView {
				l.showSplitView = false
				iss := NewIssueFromSelected(l, l.width, l.height)
				return iss, nil
			} else {
				iss := NewIssueFromSelected(l, l.width, l.height)
				return iss, nil
			}
		case "e":
			return l, l.editIssue(l.GetSelectedIssueShift(0))
		case "c":
			row := l.table.GetCursorRow()
			tableData := l.data()
			l.table.GetCopyFunc()(row+1, 0, tableData)
		case "enter":
			row := l.table.GetCursorRow()
			tableData := l.data()
			if selectedFunc := l.table.GetSelectedFunc(); selectedFunc != nil {
				selectedFunc(row+1, 0, tableData)
			}
			return l, nil
		case "ctrl+r", "f5":
			l.table.SetData(l.data())
			l.table, cmd = l.table.Update(msg)
			// Also refresh the selected issue if we're in split view
			if l.showSplitView {
				return l, l.fetchSelectedIssueCmd()
			}
			return l, cmd
		}
	}

	// If we're in issue detail mode, pass the key message to the issue detail view
	var cmd1, cmd2 tea.Cmd
	l.table, cmd1 = l.table.Update(msg)
	l.issueDetailView, cmd2 = l.safeIssueUpdate(msg)

	return l, tea.Batch(cmd1, cmd2)
}

func (l *IssueList) FetchAndRefreshCache() {
	l.Data, _ = l.FetchAllIssues()
	l.DetailedCache = make(map[string]*jira.Issue)
}

// View renders the IssueList.
func (l *IssueList) View() string {
	if !l.showSplitView {
		return l.table.View()
	}

	// Get the raw table view
	tableView := l.table.View()
	detailView := l.issueDetailView.View()

	// Create styles for both views to highlight the active one
	// tableStyle := lipgloss.NewStyle().
	// 	Border(lipgloss.RoundedBorder()).
	// 	BorderForeground(lipgloss.Color("240")).
	// 	Padding(0, 0). // Minimal padding
	// 	Width(l.width - 2).
	// 	MaxHeight(l.height/2 - 2)

	// detailStyle := lipgloss.NewStyle().
	// 	Border(lipgloss.RoundedBorder()).
	// 	BorderForeground(lipgloss.Color("240")).
	// 	Padding(0, 1). // A bit of horizontal padding for readability
	// 	Width(l.width - 4).
	// 	MaxHeight(l.height/2 - 2)

	// // Highlight the active view with a different border color
	// if l.activeView == issueListMode {
	// 	tableStyle = tableStyle.BorderForeground(lipgloss.Color("62"))
	// } else {
	// 	detailStyle = detailStyle.BorderForeground(lipgloss.Color("62"))
	// }

	// // Wrap both views in their respective styles
	// tableWrapped := tableStyle.Render(tableView)
	// detailWrapped := detailStyle.Render(detailView)

	// Add a visual separator between views
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", l.width))

	// Join everything vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		tableView,
		separator,
		detailView,
	)
}

// RunView runs the view with the data.
func (l *IssueList) RunView() error {
	if l.Display.Plain {
		return l.renderPlain()
	}

	// Create table data
	tableData := l.data()

	// If no footer text, generate one
	if l.FooterText == "" {
		l.FooterText = fmt.Sprintf("Showing %d of %d results for project %q", len(tableData)-1, l.Total, l.Project)
	}

	// Set up table
	l.table = l.setupTable()

	// Enable split view by default
	l.showSplitView = true
	l.activeView = issueListMode

	if len(l.Data) == 0 {
		panic("test data should not be 0, there should be some issues already on startup")
	}

	l.issueDetailView = NewIssueFromSelected(l, l.width, l.height/2)
	if _, err := tea.NewProgram(l, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	return nil
}

// setupTable creates and configures the table
func (l *IssueList) setupTable() *tuiBubble.Table {
	// Updated help text to include tab for split view
	splitViewHelpText := tableHelpText

	table := tuiBubble.NewTable(
		tuiBubble.WithTableFooterText(l.FooterText),
		tuiBubble.WithTableHelpText(splitViewHelpText),
		tuiBubble.WithSelectedFunc(navigate(l.Server)),
		tuiBubble.WithCopyFunc(copyURL(l.Server)),
	)

	table.SetUnderlyingTable()
	table.SetData(l.data())

	return table
}

// renderPlain renders the issue in plain view.
func (l *IssueList) renderPlain() error {
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	return renderPlain(w, l.data())
}

// renderPlain renders the data in plain view.
func renderPlain(w *tabwriter.Writer, data tuiBubble.TableData) error {
	defer w.Flush()

	for i, d := range data {
		if i == 0 {
			continue
		}
		fmt.Fprintln(w, strings.Join(d, "\t"))
	}

	return nil
}

// data prepares the data for table view.
func (l *IssueList) data() tuiBubble.TableData {
	var data tuiBubble.TableData

	headers := l.header()
	if !(l.Display.Plain && l.Display.NoHeaders) {
		data = append(data, headers)
	}
	for _, iss := range l.Data {
		data = append(data, l.assignColumns(headers, iss))
	}

	return data
}

// header prepares table headers.
func (l *IssueList) header() []string {
	if len(l.Display.Columns) > 0 {
		headers := []string{}
		columnsMap := l.validColumnsMap()
		for _, c := range l.Display.Columns {
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
func (*IssueList) validColumnsMap() map[string]struct{} {
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
		fieldKey,
		fieldType,
		fieldSummary,
		fieldStatus,
		fieldAssignee,
		fieldReporter,
		fieldResolution,
		fieldCreated,
		fieldPriority,
		fieldUpdated,
		fieldLabels,
	}
}

// assignColumns assigns columns for the issue.
func (l *IssueList) assignColumns(columns []string, issue *jira.Issue) []string {
	var bucket []string

	for _, column := range columns {
		switch column {
		case fieldType:
			bucket = append(bucket, issue.Fields.IssueType.Name)
		case fieldKey:
			bucket = append(bucket, issue.Key)
		case fieldSummary:
			bucket = append(bucket, prepareTitle(issue.Fields.Summary))
		case fieldStatus:
			bucket = append(bucket, issue.Fields.Status.Name)
		case fieldAssignee:
			bucket = append(bucket, issue.Fields.Assignee.Name)
		case fieldReporter:
			bucket = append(bucket, issue.Fields.Reporter.Name)
		case fieldPriority:
			bucket = append(bucket, issue.Fields.Priority.Name)
		case fieldResolution:
			bucket = append(bucket, issue.Fields.Resolution.Name)
		case fieldCreated:
			bucket = append(bucket, formatDateTime(issue.Fields.Created, jira.RFC3339, l.Display.Timezone))
		case fieldUpdated:
			bucket = append(bucket, formatDateTime(issue.Fields.Updated, jira.RFC3339, l.Display.Timezone))
		case fieldLabels:
			bucket = append(bucket, strings.Join(issue.Fields.Labels, ","))
		}
	}

	return bucket
}

// Utility functions to support rendering
const tableHelpText = "j/↓: Down • k/↑: Up • v: View • c: Copy URL • CTRL+r/F5: Refresh • Enter: Select/Open • Tab: Switch Focus • q/ESC/CTRL+c: Quit"

// navigate opens the issue in browser.
func navigate(server string) tuiBubble.SelectedFunc {
	return func(row, _ int, data interface{}) {
		d := data.(tuiBubble.TableData)
		if row <= 0 || row >= len(d) {
			return
		}

		keyCol := d.GetIndex(fieldKey)
		cmdutil.Navigate(server, d.Get(row, keyCol))
	}
}

// copyURL copies issue URL to clipboard.
func copyURL(server string) tuiBubble.CopyFunc {
	return func(row, _ int, data interface{}) {
		d := data.(tuiBubble.TableData)
		if row <= 0 || row >= len(d) {
			return
		}

		keyCol := d.GetIndex(fieldKey)
		key := d.Get(row, keyCol)
		copyToClipboard(fmt.Sprintf("%s/browse/%s", server, key))
	}
}

// copyToClipboard copies text to clipboard.
func copyToClipboard(text string) {
	_ = clipboard.WriteAll(text)
}

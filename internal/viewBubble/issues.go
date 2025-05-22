package viewBubble

import (
	"fmt"
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
)

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
	Total       int
	Project     string
	Server      string
	Data        []*jira.Issue
	Display     DisplayFormat
	RefreshFunc func() ([]*jira.Issue, int)
	FooterText  string

	table *tuiBubble.Table
	err   error

	width  int
	height int
}

// Init initializes the IssueList model.
func (l *IssueList) Init() tea.Cmd {
	return nil
}

type editorFinishedMsg struct{ err error }
type issueMovedMsg struct{ err error }

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

func (l *IssueList) GetSelectedIssue() *jira.Issue {
	row := l.table.GetCursorRow()
	tableData := l.data()

	ci := tableData.GetIndex(fieldKey)
	issData, err := api.ProxyGetIssue(api.DefaultClient(false), tableData.Get(row+1, ci), issue.NewNumCommentsFilter(10))
	if err != nil {
		panic(err)
	}

	return issData
}

// Update handles user input and updates the model state.
func (l *IssueList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
	case editorFinishedMsg, issueMovedMsg:
		l.Data, _ = l.RefreshFunc()
		l.table.SetData(l.data())
		return l, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return l, tea.Quit
		case "m":
			return l, l.moveIssue(l.GetSelectedIssue())
		case "v":
			iss := Issue{
				Server:   l.Server,
				Data:     l.GetSelectedIssue(),
				Options:  IssueOption{NumComments: 1},
				ListView: l,
				width:    l.width,
				height:   l.height,
			}

			return iss, nil
		case "e":
			return l, l.editIssue(l.GetSelectedIssue())
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
			return l, cmd
		}
	}

	l.table, cmd = l.table.Update(msg)

	return l, cmd
}

// View renders the IssueList.
func (l *IssueList) View() string {
	return l.table.View()
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
	l.table = tuiBubble.NewTable(
		tuiBubble.WithTableFooterText(l.FooterText),
		tuiBubble.WithTableHelpText(tableHelpText),
		tuiBubble.WithSelectedFunc(navigate(l.Server)),
		tuiBubble.WithCopyFunc(copyURL(l.Server)),
	)

	l.table.SetUnderlyingTable()

	l.table.SetData(tableData)

	// Run the program
	if _, err := tea.NewProgram(l, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	return nil
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
const tableHelpText = "j/↓: Down • k/↑: Up • h/←: Left • l/→: Right • v: View • c: Copy URL • CTRL+k: Copy Key • CTRL+r/F5: Refresh • Enter: Open in Browser • ?: Help • q/ESC/CTRL+c: Quit"

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

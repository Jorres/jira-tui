package viewBubble

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/jira/filter/issue"
	"github.com/ankitpokhrel/jira-cli/pkg/tuiBubble"
	"github.com/atotto/clipboard"
	"github.com/spf13/viper"

	"github.com/charmbracelet/bubbles/list"
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
	FetchAllEpics  func() ([]*jira.Issue, int)
	FooterText     string

	table *tuiBubble.Table
	err   error

	width  int
	height int

	fuzzy *FuzzySelector

	// Split view related fields
	showSplitView   bool
	issueDetailView *Issue

	// Status message fields
	statusMessage string
	statusTimer   *time.Timer
}

// statusClearMsg is sent when the status message should be cleared
type statusClearMsg struct{}

// setStatusMessage sets a temporary status message that will be cleared after 1 second
func (l *IssueList) setStatusMessage(message string) tea.Cmd {
	l.statusMessage = message

	// Clear any existing timer
	if l.statusTimer != nil {
		l.statusTimer.Stop()
	}

	// Set a new timer to clear the message after 1 second
	l.statusTimer = time.NewTimer(time.Second)

	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

type fuzzySelectorResult struct {
	item list.Item
}

// Init initializes the IssueList model.
func (l *IssueList) Init() tea.Cmd {
	// Enable split view by default
	l.showSplitView = true

	// Initialize cache if not already done
	if l.DetailedCache == nil {
		l.DetailedCache = make(map[string]*jira.Issue)
	}

	return l.prefetchIssuesCmd()
}

// fetchSelectedIssueCmd creates a command to fetch the currently selected issue
func (l *IssueList) fetchSelectedIssueCmd() tea.Cmd {
	return func() tea.Msg {
		return selectedIssueMsg{issue: l.GetSelectedIssueShift(0)}
	}
}

// prefetchIssuesCmd creates a command to pre-fetch the first N issues
func (l *IssueList) prefetchIssuesCmd() tea.Cmd {
	return func() tea.Msg {
		// Pre-fetch first N issues in the background
		go l.prefetchTopIssues()
		return nil
	}
}

// prefetchTopIssues fetches the first N issues asynchronously and caches them
func (l *IssueList) prefetchTopIssues() {
	if len(l.Data) == 0 {
		return
	}

	// Get prefetch count from config
	prefetchCount := viper.GetInt("bubble.list.prefetch_from_top")
	if prefetchCount <= 0 {
		// If not configured or 0, disable prefetching
		return
	}

	// Determine how many issues to prefetch (config value or total available)
	maxPrefetch := prefetchCount
	if len(l.Data) < maxPrefetch {
		maxPrefetch = len(l.Data)
	}

	// Use a WaitGroup for controlled concurrency
	var wg sync.WaitGroup
	concurrencyLimit := 3 // Limit concurrent requests to avoid overwhelming the API
	sem := make(chan struct{}, concurrencyLimit)

	for i := 0; i < maxPrefetch; i++ {
		iss := l.Data[i]
		key := iss.Key

		// Skip if already cached
		if _, exists := l.DetailedCache[key]; exists {
			continue
		}

		wg.Add(1)
		go func(issueKey string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore
			defer func() { <-sem }() // Release semaphore

			// Fetch detailed issue
			detailedIssue, err := api.ProxyGetIssue(api.DefaultClient(false), issueKey, issue.NewNumCommentsFilter(10))
			if err != nil {
				// Log error but don't panic - just skip this issue
				log.Printf("Failed to prefetch issue %s: %v", issueKey, err)
				return
			}

			// Cache the detailed issue
			l.DetailedCache[issueKey] = detailedIssue
		}(key)
	}

	wg.Wait()
}

type editorFinishedMsg struct{ err error }
type issueMovedMsg struct{ err error }
type issueAssignedToEpic struct{ err error }
type selectedIssueMsg struct{ issue *jira.Issue }
type issueCachedMsg struct {
	key   string
	issue *jira.Issue
}

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

func (l *IssueList) createIssue() tea.Cmd {
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
		"create",
	)

	c := exec.Command("jira", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

func (l *IssueList) addComment(iss *jira.Issue) tea.Cmd {
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
		"comment",
		"add",
		iss.Key,
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

func (l *IssueList) assignToEpic(epicKey string, issue *jira.Issue) tea.Cmd {
	args := []string{}

	config := viper.GetString("config")
	if config != "" {
		args = append(args,
			"-c",
			config,
		)
	}

	args = append(args,
		"epic",
		"add",
		epicKey,
		issue.Key,
	)

	c := exec.Command("jira", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return issueAssignedToEpic{err}
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
			tableHeight := int(0.4 * float32(l.height))
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
	case editorFinishedMsg, issueMovedMsg, issueAssignedToEpic:
		l.FetchAndRefreshCache()
		l.table.SetData(l.data())
		if l.showSplitView {
			return l, l.fetchSelectedIssueCmd()
		}
		return l, nil
	case statusClearMsg:
		l.statusMessage = ""
		if l.statusTimer != nil {
			l.statusTimer.Stop()
			l.statusTimer = nil
		}
		return l, nil
	case fuzzySelectorResult:
		switch item := msg.item.(type) {
		case *jira.Issue:
			epic := item
			return l, l.assignToEpic(epic.Key, l.GetSelectedIssueShift(0))
		}
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
		case "ctrl+p":
			// I hate golang, why tf []concrete -> []interface is invalid when concrete satisfies interface...
			epics, _ := l.FetchAllEpics()
			listItems := []list.Item{}
			for _, epic := range epics {
				listItems = append(listItems, epic)
			}
			fz := NewFuzzySelectorFrom(l, l.width, l.height, listItems)
			return fz, nil
		case "ctrl+c", "q", "esc":
			return l, tea.Quit
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
		case "u":
			// Copy URL and show confirmation
			row := l.table.GetCursorRow()
			tableData := l.data()

			// Get the issue key to build the URL
			if row >= 0 && row+1 < len(tableData) {
				keyCol := tableData.GetIndex("KEY")
				if keyCol >= 0 {
					key := tableData.Get(row+1, keyCol)
					url := fmt.Sprintf("%s/browse/%s", l.Server, key)
					copyToClipboard(url)
					return l, l.setStatusMessage(fmt.Sprintf("Current issue FQDN copied: %s", url))
				}
			}
			return l, nil
		case "enter":
			row := l.table.GetCursorRow()
			tableData := l.data()
			if selectedFunc := l.table.GetSelectedFunc(); selectedFunc != nil {
				selectedFunc(row+1, 0, tableData)
			}
			return l, nil
		case "n":
			return l, l.createIssue()
		case "c":
			return l, l.addComment(l.GetSelectedIssueShift(0))
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
	// Update footer text based on status message
	if l.statusMessage != "" {
		l.table.SetFooterText(l.statusMessage)
	} else {
		// Use the default footer text
		if l.FooterText == "" {
			tableData := l.data()
			l.FooterText = fmt.Sprintf("Showing %d of %d results for project %q", len(tableData)-1, l.Total, l.Project)
		}
		l.table.SetFooterText(l.FooterText)
	}

	if !l.showSplitView {
		return l.table.View()
	}

	// Get the raw table view
	tableView := l.table.View()
	detailView := l.issueDetailView.View()

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
		tuiBubble.WithCopyFunc(copyURLToClipboard(l.Server)),
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
		fieldParent,
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
		case fieldParent:
			if issue.Fields.Parent != nil {
				bucket = append(bucket, issue.Fields.Parent.Key)
			} else {
				bucket = append(bucket, "")
			}
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

const tableHelpText = "j/↓ k/↑: down up, CTRL+e/y scroll  •  v: view  •  n: new issue  •  u: copy URL  •  c: add comment  •  CTRL+r/F5: refresh  •  CTRL+p: assign to epic  •  enter: select/Open  •  q/ESC/CTRL+c: quit"

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

func copyURLToClipboard(server string) tuiBubble.CopyFunc {
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

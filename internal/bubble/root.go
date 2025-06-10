package bubble

import (
	"bytes"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/jorres/jira-tui/api"
	"github.com/jorres/jira-tui/internal/cmdutil"
	"github.com/jorres/jira-tui/internal/debug"
	"github.com/jorres/jira-tui/internal/exp"
	"github.com/jorres/jira-tui/internal/query"
	"github.com/jorres/jira-tui/pkg/jira"
	"github.com/jorres/jira-tui/pkg/jira/filter/issue"
	"github.com/spf13/viper"

	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/charmbracelet/bubbles/v2/spinner"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

var _ = debug.Debug

// getDefaultIssueColumns returns the default columns for issue list.
func getDefaultIssueColumns() []string {
	return []string{
		FieldKey,
		FieldType,
		FieldParent,
		FieldSummary,
		FieldStatus,
		FieldAssignee,
		FieldReporter,
		FieldCreated,
		FieldPriority,
		FieldResolution,
		FieldUpdated,
		FieldLabels,
	}
}

// TabConfig holds configuration for a single tab
type TabConfig struct {
	Name        string
	Project     string
	Columns     []string
	BoardId     int
	QueryParams *query.IssueParams
	FetchIssues func() ([]*jira.Issue, int)
	FetchEpics  func() ([]*jira.Issue, int)

	BoardStateResolver *exp.BoardStateResolver
}

func (tc *TabConfig) getColumns() []string {
	if len(tc.Columns) > 0 {
		return tc.Columns
	}
	return getDefaultIssueColumns()
}

// IssueList is a list view for issues.
type IssueList struct {
	Total   int
	Project string
	Server  string

	// Tab management
	tabs      []*TabConfig
	activeTab int

	// Per-tab state
	tables           []*Table
	issueDetailViews []IssueModel

	err error

	rawWidth      int
	rawHeight     int
	tableHeight   int
	previewHeight int

	fuzzy *FuzzySelector

	// Status message fields
	statusMessage string
	statusTimer   *time.Timer

	c *jira.Client

	cachedAllUsers []*jira.User
}

func RunMainUI(project, server string, total int, tabs []*TabConfig, timezone string, debugMode bool) {
	l := &IssueList{
		Project: project,
		Server:  server,
		Total:   total,

		c:                api.DefaultClient(debugMode),
		tabs:             tabs,
		activeTab:        0,
		tables:           make([]*Table, len(tabs)),
		issueDetailViews: make([]IssueModel, len(tabs)),
	}

	detect := tea.NewProgram(DetectColorModel{})
	_, _ = detect.Run()

	p := tea.NewProgram(l, tea.WithAltScreen())

	_, err := p.Run()
	if err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func (l *IssueList) reinitTable(index int) tea.Cmd {
	const tableHelpText = "?: toggle help"
	tabConfig := l.tabs[index]
	table := NewTable(WithTableHelpText(tableHelpText))
	table.SetColumns(tabConfig.getColumns())
	table.SetTimezone("Local")
	l.tables[index] = table

	var tableUpdateCmd tea.Cmd
	if l.tableHeight != 0 {
		l.tables[index], tableUpdateCmd = table.Update(WidgetSizeMsg{
			Height: l.tableHeight,
			Width:  l.rawWidth,
		})
	}

	cmd2 := table.spinner.Tick

	return tea.Batch(tableUpdateCmd, cmd2, func() tea.Msg {
		tabConfig.BoardStateResolver = exp.CreateBoardStateResolver(l.c, tabConfig.BoardId, tabConfig.QueryParams)

		issues, _ := tabConfig.FetchIssues()
		return IncomingIssueListMsg{issues: issues, index: index, resolver: tabConfig.BoardStateResolver}
	})
}

func (l *IssueList) reinitOnlyOneIssue(index int, issueKey string) tea.Cmd {
	newIssue, err := api.ProxyGetIssue(api.DefaultClient(false), issueKey, issue.NewNumCommentsFilter(10))
	if err != nil {
		panic(err)
	}

	delete(l.tables[index].issueCache, issueKey)

	for i, oldIssue := range l.tables[index].allIssues {
		if oldIssue.Key == newIssue.Key {
			l.tables[index].allIssues[i] = newIssue
		}
	}

	return func() tea.Msg {
		return IncomingIssueListMsg{
			issues:   l.tables[index].allIssues,
			index:    index,
			resolver: l.tabs[index].BoardStateResolver,
		}
	}
}

func (l *IssueList) reinitIssue(index int) tea.Cmd {
	var issueUpdateCmd tea.Cmd
	cmds := []tea.Cmd{}
	l.issueDetailViews[index] = NewIssueModel(l.Server)
	l.issueDetailViews[index], issueUpdateCmd = l.issueDetailViews[index].Update(WidgetSizeMsg{
		Height: l.previewHeight,
		Width:  l.rawWidth,
	})
	cmds = append(cmds, issueUpdateCmd)
	cmds = append(cmds, l.issueDetailViews[index].spinner.Tick)
	return tea.Batch(cmds...)
}

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
		return StatusClearMsg{}
	})
}

// Init initializes the IssueList model.
func (l *IssueList) Init() tea.Cmd {
	var cmds []tea.Cmd
	for i := range l.tabs {
		cmds = append(cmds, l.reinitTable(i))
		cmds = append(cmds, l.reinitIssue(i))
	}
	return tea.Batch(cmds...)
}

// getCurrentTable returns the table for the active tab
func (l *IssueList) getCurrentTable() *Table {
	return l.tables[l.activeTab]
}

// getCurrentIssueDetailView returns the issue detail view for the active tab
func (l *IssueList) getCurrentIssueDetailView() IssueModel {
	return l.issueDetailViews[l.activeTab]
}

// getCurrentTabConfig returns the tab config for the active tab
func (l *IssueList) getCurrentTabConfig() *TabConfig {
	return l.tabs[l.activeTab]
}

// View mode constants
const (
	issueListMode int = iota
	issueDetailMode
)

// execCommandWithStderr executes a command and captures both stdout and stderr
func execCommandWithStderr(args []string, msgConstructor func(error, string) tea.Msg) tea.Cmd {
	c := exec.Command("jira", args...)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		stderrOutput := stderr.String()
		return msgConstructor(err, stderrOutput)
	})
}

func (l *IssueList) editIssue(issue *jira.Issue) tea.Cmd {
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

	return execCommandWithStderr(args, func(err error, stderr string) tea.Msg {
		return IssueEditedMsg{issueKey: issue.Key, err: err, stderr: stderr}
	})
}

func (l *IssueList) createIssue(project string) tea.Cmd {
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
		fmt.Sprintf("-p%s", project),
	)

	return execCommandWithStderr(args, func(err error, stderr string) tea.Msg {
		return IssueCreatedMsg{err: err, stderr: stderr}
	})
}

func (l *IssueList) addComment(iss *jira.Issue) tea.Cmd {
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

	return execCommandWithStderr(args, func(err error, stderr string) tea.Msg {
		return IssueEditedMsg{issueKey: iss.Key, err: err, stderr: stderr}
	})
}

func (l *IssueList) toggleBacklogState(issue *jira.Issue) tea.Cmd {
	return func() tea.Msg {
		tabConfig := l.getCurrentTabConfig()
		newState, err := exp.ToggleIssueBacklogState(l.c, tabConfig.BoardId, issue, tabConfig.BoardStateResolver)
		if err != nil {
			return IssueBacklogToggleMsg{issueKey: issue.Key, err: err, stderr: err.Error()}
		} else {
			tabConfig.BoardStateResolver.SetBacklogState(issue.Key, newState)
		}
		return IssueBacklogToggleMsg{issueKey: issue.Key, err: nil, stderr: ""}
	}
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

	return execCommandWithStderr(args, func(err error, stderr string) tea.Msg {
		return IssueMovedMsg{issueKey: issue.Key, err: err, stderr: stderr}
	})
}

func (l *IssueList) processError(err error, stderr string) (tea.Model, tea.Cmd) {
	// we don't want to draw the error message border if user just pressed ctrl+c,
	// this is not an "error" that user expects
	if err != nil && !strings.Contains(stderr, "interrupt") {
		errorModel := NewErrorModel(l, err.Error(), stderr, l.rawWidth, l.rawHeight)
		return errorModel, nil
	} else {
		return l, l.reinitTable(l.activeTab)
	}
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

	return execCommandWithStderr(args, func(err error, stderr string) tea.Msg {
		return IssueAssignedToEpicMsg{err: err, stderr: stderr}
	})
}

func (l *IssueList) assignToUser(user *jira.User, issue *jira.Issue) {
	var err error
	if viper.GetString("installation") == jira.InstallationTypeLocal {
		err = l.c.AssignIssueV2(issue.Key, user.Name)
	} else {
		err = l.c.AssignIssue(issue.Key, user.AccountID)
	}

	if err != nil {
		cmdutil.ExitIfError(err)
	}
}

func (l *IssueList) SafelyGetAssignableUsers(issueKey string) ([]*jira.User, error) {
	var err error
	if l.cachedAllUsers == nil {
		l.cachedAllUsers, err = l.c.GetAssignableToIssue(issueKey)
		if err != nil {
			return nil, err
		}
	}
	return l.cachedAllUsers, nil
}

// Update handles user input and updates the model state.
func (l *IssueList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Store the full window size
		l.rawWidth = msg.Width
		l.rawHeight = msg.Height

		// Reserve 2 rows for tabs only if there are multiple tabs
		tabHeight := 0
		if len(l.tabs) > 1 {
			tabHeight = 2
		}
		l.tableHeight = int(0.4 * float32(l.rawHeight-tabHeight))
		l.previewHeight = l.rawHeight - l.tableHeight - tabHeight

		var cmds []tea.Cmd

		for key := range l.tables {
			l.tables[key], cmd = l.tables[key].Update(WidgetSizeMsg{
				Height: l.tableHeight,
				Width:  l.rawWidth,
			})
			cmds = append(cmds, cmd)

			l.issueDetailViews[key], cmd = l.issueDetailViews[key].Update(WidgetSizeMsg{
				Height: l.previewHeight,
				Width:  l.rawWidth,
			})
			cmds = append(cmds, cmd)
		}

		return l, tea.Batch(cmds...)
	case spinner.TickMsg:
		var cmd1, cmd2 tea.Cmd
		l.tables[l.activeTab], cmd1 = l.tables[l.activeTab].Update(msg)
		l.issueDetailViews[l.activeTab], cmd2 = l.issueDetailViews[l.activeTab].Update(msg)
		return l, tea.Batch(cmd1, cmd2)
	case IncomingIssueMsg:
		m, _ := l.issueDetailViews[msg.index].Update(msg.issue)
		l.tables[msg.index], cmd = l.tables[msg.index].Update(msg.issue)
		l.issueDetailViews[msg.index] = m
		return l, cmd
	case IncomingIssueListMsg:
		var cmd tea.Cmd
		thisTable := l.tables[msg.index]

		thisTable.SetIssueData(msg.issues)
		thisTable.SetBacklightResolver(func(issueKey string) *color.Color {
			if msg.resolver == nil {
				return nil
			}

			if msg.resolver.IsOnBoard(issueKey) {
				color := lipgloss.Color(viper.GetString("ui.theme.onBoard"))
				return &color
			}

			color := lipgloss.Color(viper.GetString("ui.theme.onBacklog"))
			return &color
		})

		if len(msg.issues) > 0 {
			cmd = thisTable.GetIssueAsync(msg.index, 0)
		}
		return l, cmd
	// Can't combine the next 4 into one switch clause due to Go's type system
	case IssueEditedMsg:
		if msg.err != nil {
			return l.processError(msg.err, msg.stderr)
		}
		return l, l.reinitOnlyOneIssue(l.activeTab, msg.issueKey)
	case IssueMovedMsg:
		if msg.err != nil {
			return l.processError(msg.err, msg.stderr)
		}
		return l, l.reinitOnlyOneIssue(l.activeTab, msg.issueKey)
	case IssueAssignedToEpicMsg:
		if msg.err != nil {
			return l.processError(msg.err, msg.stderr)
		}
		return l, l.reinitOnlyOneIssue(l.activeTab, msg.issueKey)
	case IssueCreatedMsg:
		if msg.err != nil {
			return l.processError(msg.err, msg.stderr)
		}
		return l, l.reinitTable(l.activeTab)
	case IssueBacklogToggleMsg:
		if msg.err != nil {
			return l.processError(msg.err, msg.stderr)
		}
		return l, l.reinitOnlyOneIssue(l.activeTab, msg.issueKey)
	case StatusClearMsg:
		l.statusMessage = ""
		if l.statusTimer != nil {
			l.statusTimer.Stop()
			l.statusTimer = nil
		}
		return l, nil
	case FuzzySelectorResultMsg:
		switch msg.selectorType {
		case FuzzySelectorEpic:
			epic := msg.item.(*jira.Issue)
			return l, l.assignToEpic(epic.Key, l.getCurrentTable().GetIssueSync(0))
		case FuzzySelectorUser:
			user := msg.item.(*jira.User)
			issue := l.getCurrentTable().GetIssueSync(0)
			l.assignToUser(user, issue)
			return l, l.reinitOnlyOneIssue(l.activeTab, issue.Key)
		}
	case tea.KeyMsg:
		currentTable := l.getCurrentTable()
		if currentTable != nil {
			if currentTable.SorterState == SorterFiltering {
				var cmd1, cmd2 tea.Cmd
				l.tables[l.activeTab], cmd1 = currentTable.Update(msg)
				cmd2 = l.tables[l.activeTab].GetIssueAsync(l.activeTab, 0)
				return l, tea.Batch(cmd1, cmd2)
			}

			if currentTable.SorterState == SorterActive && msg.String() == "esc" {
				l.tables[l.activeTab], cmd = currentTable.Update(msg)
				return l, cmd
			}
		}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return l, tea.Quit
		case "right", "l":
			if len(l.tabs) > 1 {
				l.activeTab = (l.activeTab + 1) % len(l.tabs)
				tableSpinner := l.getCurrentTable().spinner.Tick
				issueSpinner := l.getCurrentIssueDetailView().spinner.Tick
				return l, tea.Batch(tableSpinner, issueSpinner)
			}
		case "left", "h":
			if len(l.tabs) > 1 {
				l.activeTab = (l.activeTab - 1 + len(l.tabs)) % len(l.tabs)
				tableSpinner := l.getCurrentTable().spinner.Tick
				issueSpinner := l.getCurrentIssueDetailView().spinner.Tick
				return l, tea.Batch(tableSpinner, issueSpinner)
			}
		case "up", "k":
			currentTable := l.getCurrentTable()
			var cmd1, cmd2 tea.Cmd
			cmd1 = currentTable.GetIssueAsync(l.activeTab, -1)
			l.tables[l.activeTab], cmd = currentTable.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "down", "j":
			currentTable := l.getCurrentTable()
			var cmd1, cmd2 tea.Cmd
			cmd1 = currentTable.GetIssueAsync(l.activeTab, +1)
			l.tables[l.activeTab], cmd = currentTable.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "a":
			iss := l.getCurrentTable().GetIssueSync(0)
			users, err := l.SafelyGetAssignableUsers(iss.Key)

			if err != nil {
				return l.processError(err, "")
			}

			listItems := []list.Item{}
			for _, user := range users {
				listItems = append(listItems, user)
			}
			fz := NewFuzzySelectorFrom(l, l.rawWidth, l.rawHeight, listItems, FuzzySelectorUser)
			return fz, nil
		case "ctrl+p":
			// I hate golang, why tf []concrete -> []interface is invalid when concrete satisfies interface...
			tabConfig := l.getCurrentTabConfig()
			epics, _ := tabConfig.FetchEpics()
			listItems := []list.Item{}
			for _, epic := range epics {
				listItems = append(listItems, epic)
			}
			fz := NewFuzzySelectorFrom(l, l.rawWidth, l.rawHeight, listItems, FuzzySelectorEpic)
			return fz, nil
		case "m":
			return l, l.moveIssue(l.getCurrentTable().GetIssueSync(0))
		case "e":
			return l, l.editIssue(l.getCurrentTable().GetIssueSync(0))
		case "u":
			key := l.getCurrentTable().getKeyUnderCursorWithShift(0)
			url := fmt.Sprintf("%s/browse/%s", l.Server, key)
			copyToClipboard(url)
			return l, l.setStatusMessage(fmt.Sprintf("Current issue FQDN copied: %s", url))
		case "enter":
			iss := l.getCurrentTable().GetIssueSync(0)
			cmdutil.Navigate(l.Server, iss.Key)
			return l, nil
		case "n":
			return l, l.createIssue(l.getCurrentTabConfig().Project)
		case "c":
			return l, l.addComment(l.getCurrentTable().GetIssueSync(0))
		case "b":
			return l, l.toggleBacklogState(l.getCurrentTable().GetIssueSync(0))
		case "ctrl+r":
			return l, l.reinitTable(l.activeTab)
		case "?":
			helpView := NewHelpView(l, l.rawWidth, l.rawHeight)
			return helpView, nil

		// Forwarding to issue:
		case "ctrl+e", "ctrl+y", "tab":
			m, cmd := l.getCurrentIssueDetailView().Update(msg)
			l.issueDetailViews[l.activeTab] = m
			return l, cmd
		// Forwarding straight to table:
		case "/":
			l.tables[l.activeTab], cmd = l.getCurrentTable().Update(msg)
		}
	}

	return l, cmd
}

// View renders the IssueList.
func (l *IssueList) View() string {
	if len(l.tabs) == 0 {
		return "No tabs configured"
	}

	currentTable := l.getCurrentTable()
	if currentTable == nil {
		return ""
	}

	currentView := l.getCurrentIssueDetailView()

	// Update footer text based on status message
	if l.statusMessage != "" {
		currentTable.SetFooterText(l.statusMessage)
	} else {
		currentTable.SetDefaultFooterText()
	}

	// Get the raw table view
	tableView := currentTable.View()
	detailView := currentView.View()

	// Add a visual separator between views
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color(getPaleColor())).
		Render(strings.Repeat("─", l.rawWidth))

	// Only render tabs if there's more than one
	if len(l.tabs) > 1 {
		tabView := l.renderTabs()
		// Join everything vertically with tabs
		return lipgloss.JoinVertical(
			lipgloss.Left,
			tabView,
			tableView,
			separator,
			detailView,
		)
	} else {
		// Join everything vertically without tabs
		return lipgloss.JoinVertical(
			lipgloss.Left,
			tableView,
			separator,
			detailView,
		)
	}
}

func activeTabStyle() lipgloss.Style {
	return lipgloss.
		NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(getHighlightColor())).
		Padding(0, 1).
		Margin(0, 2).
		Bold(true)
}

func inactiveTabStyle() lipgloss.Style {
	return lipgloss.
		NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(getPaleColor())).
		Padding(0, 1).
		Bold(false)
}

// renderTabs renders the tab bar
func (l *IssueList) renderTabs() string {
	if len(l.tabs) == 0 {
		return ""
	}

	var renderedTabs []string

	for i, tabConfig := range l.tabs {
		var style lipgloss.Style
		isActive := i == l.activeTab
		if isActive {
			style = activeTabStyle()
		} else {
			style = inactiveTabStyle()
		}
		border, _, _, _, _ := style.GetBorder()
		style = style.Border(border).BorderBottom(false)
		renderedTabs = append(renderedTabs, style.Render(tabConfig.Name))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
}

// copyToClipboard copies text to clipboard.
func copyToClipboard(text string) {
	_ = clipboard.WriteAll(text)
}

package viewBubble

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/internal/debug"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/atotto/clipboard"
	"github.com/spf13/viper"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ = debug.Debug

// TabConfig holds configuration for a single tab
type TabConfig struct {
	Name        string
	Project     string
	FetchIssues func() ([]*jira.Issue, int)
	FetchEpics  func() ([]*jira.Issue, int)
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

	rawWidth  int
	rawHeight int

	fuzzy *FuzzySelector

	// Status message fields
	statusMessage string
	statusTimer   *time.Timer

	c *jira.Client

	users []*jira.User
}

func NewIssueList(
	project, server string,
	total int,
	tabs []*TabConfig,
	displayFormat DisplayFormat,
	debug bool,
) *IssueList {
	const tableHelpText = "j/↓ k/↑: down up, CTRL+e/y scroll  •  n: new issue  •  u: copy URL  •  c: add comment  •  CTRL+r: refresh  •  CTRL+p: assign to epic  •  enter: select/Open  •  q/ESC/CTRL+c: quit   •  a: change assignee"

	splitViewHelpText := tableHelpText

	l := &IssueList{
		Project: project,
		Server:  server,
		Total:   total,

		c:                api.DefaultClient(debug),
		tabs:             tabs,
		activeTab:        0,
		tables:           make([]*Table, len(tabs)),
		issueDetailViews: make([]IssueModel, len(tabs)),
	}

	wg := sync.WaitGroup{}

	for i, tabConfig := range tabs {
		wg.Add(1)
		go func(index int, config *TabConfig) {
			defer wg.Done()
			table := NewTable(
				WithTableHelpText(splitViewHelpText),
			)
			table.SetDisplayFormat(displayFormat)

			issues, _ := config.FetchIssues()
			table.SetIssueData(issues)

			l.tables[index] = table
			l.issueDetailViews[index] = NewIssueModel(l.Server)
			if len(issues) > 0 {
				m, _ := l.issueDetailViews[index].Update(table.GetIssueSync(0))
				l.issueDetailViews[index] = m
			}
		}(i, tabConfig)
	}

	wg.Wait()

	return l
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
	return nil
}

func (l *IssueList) forceRedrawCmd() tea.Cmd {
	return func() tea.Msg {
		return SelectedIssueMsg{issue: l.getCurrentTable().GetIssueSync(0)}
	}
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
		return EditorFinishedMsg{err}
	})
}

func (l *IssueList) createIssue(project string) tea.Cmd {
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
		fmt.Sprintf("-p%s", project),
	)

	c := exec.Command("jira", args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return EditorFinishedMsg{err}
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
		return EditorFinishedMsg{err}
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
		return IssueMovedMsg{err}
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
		return IssueAssignedToEpicMsg{err}
	})
}

func (l *IssueList) assignToUser(user *jira.User, issue *jira.Issue) tea.Cmd {
	err := l.c.AssignIssue(issue.Key, user.AccountID)
	if err != nil {
		cmdutil.ExitIfError(err)
	}
	return l.forceRedrawCmd()
}

func (l *IssueList) updateCurrentIssue(msg tea.Msg) tea.Cmd {
	m, cmd := l.getCurrentIssueDetailView().Update(msg)
	l.issueDetailViews[l.activeTab] = m
	return cmd
}

func (l *IssueList) SafelyGetAssignableUsers(issueKey string) []*jira.User {
	if l.users == nil {
		var err error
		l.users, err = l.c.GetAssignableToIssue(issueKey)
		if err != nil {
			cmdutil.ExitIfError(err)
		}
	}
	return l.users
}

// Update handles user input and updates the model state.
func (l *IssueList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Store the full window size
		l.rawWidth = msg.Width
		l.rawHeight = msg.Height

		var cmds []tea.Cmd

		// Reserve 2 rows for tabs only if there are multiple tabs
		tabHeight := 0
		if len(l.tabs) > 1 {
			tabHeight = 2
		}
		tableHeight := int(0.4 * float32(l.rawHeight-tabHeight))
		previewHeight := l.rawHeight - tableHeight - tabHeight

		// Update all tables and issue detail views
		for key := range l.tables {
			l.tables[key], cmd = l.tables[key].Update(WidgetSizeMsg{
				Height: tableHeight,
				Width:  l.rawWidth,
			})
			cmds = append(cmds, cmd)

			l.issueDetailViews[key], cmd = l.issueDetailViews[key].Update(WidgetSizeMsg{
				Height: previewHeight,
				Width:  l.rawWidth,
			})
			cmds = append(cmds, cmd)
		}

		return l, tea.Batch(cmds...)
	case SelectedIssueMsg:
		cmd := l.updateCurrentIssue(msg.issue)
		return l, cmd
	case EditorFinishedMsg, IssueMovedMsg, IssueAssignedToEpicMsg:
		l.FetchAndRefreshCache()
		return l, cmd
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
			return l, l.assignToUser(user, l.getCurrentTable().GetIssueSync(0))
		}
	case CurrentIssueReceivedMsg:
		currentTable := l.getCurrentTable()

		if msg.Table == currentTable && msg.Pos == currentTable.GetCursorRow() {
			cmd = l.updateCurrentIssue(msg.Issue)
			return l, cmd
		}
	case tea.KeyMsg:
		currentTable := l.getCurrentTable()
		if currentTable != nil {
			if currentTable.SorterState == SorterFiltering {
				var cmd1, cmd2 tea.Cmd
				l.tables[l.activeTab], cmd1 = currentTable.Update(msg)
				cmd2 = l.tables[l.activeTab].ScheduleIssueUpdateMessage(0)
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
				return l, l.forceRedrawCmd()
			}
		case "left", "h":
			if len(l.tabs) > 1 {
				l.activeTab = (l.activeTab - 1 + len(l.tabs)) % len(l.tabs)
				return l, l.forceRedrawCmd()
			}
		case "up", "k":
			currentTable := l.getCurrentTable()
			var cmd1, cmd2 tea.Cmd
			cmd1 = currentTable.ScheduleIssueUpdateMessage(-1)
			l.tables[l.activeTab], cmd = currentTable.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "down", "j":
			currentTable := l.getCurrentTable()
			var cmd1, cmd2 tea.Cmd
			cmd1 = currentTable.ScheduleIssueUpdateMessage(+1)
			l.tables[l.activeTab], cmd = currentTable.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "a":
			iss := l.getCurrentTable().GetIssueSync(0)
			users := l.SafelyGetAssignableUsers(iss.Key)

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
			iss := l.getCurrentTable().GetIssueSync(0)
			url := fmt.Sprintf("%s/browse/%s", l.Server, iss.Key)
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
		case "ctrl+r":
			currentTable := l.getCurrentTable()
			cmd1 := l.updateCurrentIssue(currentTable.GetIssueSync(0))
			var cmd2 tea.Cmd
			l.tables[l.activeTab], cmd2 = currentTable.Update(msg)
			return l, tea.Batch(cmd1, cmd2)

		// Forwarding to issue:
		case "ctrl+e", "ctrl+y", "tab":
			cmd := l.updateCurrentIssue(msg)
			return l, cmd
		// Forwarding straight to table:
		case "/":
			l.tables[l.activeTab], cmd = l.getCurrentTable().Update(msg)
			return l, l.forceRedrawCmd()
		}
	}

	return l, cmd
}

func (l *IssueList) FetchAndRefreshCache() {
	tabConfig := l.getCurrentTabConfig()
	issues, _ := tabConfig.FetchIssues()
	currentTable := l.getCurrentTable()
	currentTable.RefreshCache(issues)
}

// View renders the IssueList.
func (l *IssueList) View() string {
	if len(l.tabs) == 0 {
		return "No tabs configured"
	}

	currentTable := l.getCurrentTable()
	if currentTable == nil {
		panic("no current table configured, should not happen")
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
		Foreground(lipgloss.Color("240")).
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

func (l *IssueList) RunView() error {
	if _, err := tea.NewProgram(l, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	return nil
}

// Tab styling (based on tabs.go example)
func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┬", "─", "┬")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	highlightColor    = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	grayColor         = lipgloss.Color("240")
	inactiveTabStyle  = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(grayColor).Padding(0, 1)
	activeTabStyle    = lipgloss.NewStyle().Border(activeTabBorder, true).BorderForeground(highlightColor).Padding(0, 1)
)

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
			style = activeTabStyle
		} else {
			style = inactiveTabStyle
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

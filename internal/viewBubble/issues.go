package viewBubble

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/tuiBubble"
	"github.com/atotto/clipboard"
	"github.com/spf13/viper"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ = log.Fatal

// IssueList is a list view for issues.
type IssueList struct {
	Total   int
	Project string
	Server  string

	FetchAllIssues func() ([]*jira.Issue, int)
	FetchAllEpics  func() ([]*jira.Issue, int)

	table *tuiBubble.Table
	err   error

	rawWidth  int
	rawHeight int

	fuzzy *FuzzySelector

	issueDetailView IssueModel

	// Status message fields
	statusMessage string
	statusTimer   *time.Timer
}

func NewIssueList(
	project, server string,
	total int,
	issues []*jira.Issue,
	fetchIssuesWithArgs func() ([]*jira.Issue, int),
	fetchAllEpics func() ([]*jira.Issue, int),
	displayFormat tuiBubble.DisplayFormat,
) *IssueList {
	const tableHelpText = "j/↓ k/↑: down up, CTRL+e/y scroll  •  n: new issue  •  u: copy URL  •  c: add comment  •  CTRL+r: refresh  •  CTRL+p: assign to epic  •  enter: select/Open  •  q/ESC/CTRL+c: quit"

	splitViewHelpText := tableHelpText

	l := &IssueList{
		Project:        project,
		Server:         server,
		Total:          total,
		FetchAllIssues: fetchIssuesWithArgs,
		FetchAllEpics:  fetchAllEpics,
	}

	table := tuiBubble.NewTable(
		tuiBubble.WithTableHelpText(splitViewHelpText),
	)
	table.SetDisplayFormat(displayFormat)
	table.SetIssueData(issues)
	l.table = table

	l.issueDetailView = NewIssueFromSelected(l)

	return l
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
	return l.prefetchIssuesCmd()
}

func (l *IssueList) forceRedrawCmd() tea.Cmd {
	return func() tea.Msg {
		return selectedIssueMsg{issue: l.table.GetSelectedIssueShift(0)}
	}
}

// prefetchIssuesCmd creates a command to pre-fetch the first N issues
func (l *IssueList) prefetchIssuesCmd() tea.Cmd {
	return func() tea.Msg {
		// Pre-fetch first N issues in the background
		go l.table.PrefetchTopIssues()
		return nil
	}
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

func (l *IssueList) safeIssueUpdate(msg tea.Msg) (IssueModel, tea.Cmd) {
	m, cmd := l.issueDetailView.Update(msg)
	if v, ok := m.(IssueModel); ok {
		return v, cmd
	} else {
		panic("expected IssueModel from some of the issue model updates")
	}
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

		tableHeight := int(0.4 * float32(l.rawHeight))
		previewHeight := l.rawHeight - tableHeight

		l.table, cmd = l.table.Update(tuiBubble.WidgetSizeMsg{
			Height: tableHeight,
			Width:  l.rawWidth,
		})
		cmds = append(cmds, cmd)

		l.issueDetailView, cmd = l.safeIssueUpdate(tuiBubble.WidgetSizeMsg{
			Height: previewHeight,
			Width:  l.rawWidth,
		})
		cmds = append(cmds, cmd)
		return l, tea.Batch(cmds...)
	case selectedIssueMsg:
		l.issueDetailView, cmd = l.safeIssueUpdate(msg)
		return l, cmd
	case editorFinishedMsg, issueMovedMsg, issueAssignedToEpic:
		l.FetchAndRefreshCache()
		return l, cmd
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
			return l, l.assignToEpic(epic.Key, l.table.GetSelectedIssueShift(0))
		}
	case tea.KeyMsg:

		if l.table.SorterState == tuiBubble.SorterFiltering {
			l.table, cmd = l.table.Update(msg)
			return l, cmd
		}

		if l.table.SorterState == tuiBubble.SorterActive && msg.String() == "esc" {
			l.table, cmd = l.table.Update(msg)
			return l, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return l, tea.Quit
		case "up", "k":
			var cmd1, cmd2 tea.Cmd
			l.issueDetailView, cmd1 = l.safeIssueUpdate(l.table.GetSelectedIssueShift(-1))
			l.table, cmd2 = l.table.Update(msg)
			return l, tea.Batch(cmd1, cmd2)
		case "down", "j":
			var cmd1, cmd2 tea.Cmd
			l.issueDetailView, cmd1 = l.safeIssueUpdate(l.table.GetSelectedIssueShift(+1))
			l.table, cmd2 = l.table.Update(msg)
			return l, tea.Batch(cmd1, cmd2)

		case "ctrl+p":
			// I hate golang, why tf []concrete -> []interface is invalid when concrete satisfies interface...
			epics, _ := l.FetchAllEpics()
			listItems := []list.Item{}
			for _, epic := range epics {
				listItems = append(listItems, epic)
			}
			fz := NewFuzzySelectorFrom(l, l.rawWidth, l.rawHeight, listItems)
			return fz, nil
		case "m":
			return l, l.moveIssue(l.table.GetSelectedIssueShift(0))
		case "e":
			return l, l.editIssue(l.table.GetSelectedIssueShift(0))
		case "u":
			iss := l.table.GetSelectedIssueShift(0)
			url := fmt.Sprintf("%s/browse/%s", l.Server, iss.Key)
			copyToClipboard(url)
			return l, l.setStatusMessage(fmt.Sprintf("Current issue FQDN copied: %s", url))
		case "enter":
			iss := l.table.GetSelectedIssueShift(0)
			cmdutil.Navigate(l.Server, iss.Key)
			return l, nil
		case "n":
			return l, l.createIssue()
		case "c":
			return l, l.addComment(l.table.GetSelectedIssueShift(0))
		case "ctrl+r":
			var cmd1, cmd2 tea.Cmd
			l.issueDetailView, cmd1 = l.safeIssueUpdate(l.table.GetSelectedIssueShift(0))
			l.table, cmd2 = l.table.Update(msg)
			return l, tea.Batch(cmd1, cmd2)

		// Forwarding to issue:
		case "tab", "ctrl+e", "ctrl+y":
			l.issueDetailView, cmd = l.safeIssueUpdate(msg)
			return l, cmd

		// Forwarding straight to table:
		case "/":
			l.table, cmd = l.table.Update(msg)
			return l, l.forceRedrawCmd()
		}
	}

	return l, cmd
}

func (l *IssueList) FetchAndRefreshCache() {
	issues, _ := l.FetchAllIssues()
	l.table.RefreshCache(issues)
}

// View renders the IssueList.
func (l *IssueList) View() string {
	// Update footer text based on status message
	if l.statusMessage != "" {
		l.table.SetFooterText(l.statusMessage)
	} else {
		l.table.SetDefaultFooterText()
	}

	// Get the raw table view
	tableView := l.table.View()
	detailView := l.issueDetailView.View()

	// Add a visual separator between views
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", l.rawWidth))

	// Join everything vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		tableView,
		separator,
		detailView,
	)
}

func (l *IssueList) RunView() error {
	if _, err := tea.NewProgram(l, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	return nil
}

// copyToClipboard copies text to clipboard.
func copyToClipboard(text string) {
	_ = clipboard.WriteAll(text)
}

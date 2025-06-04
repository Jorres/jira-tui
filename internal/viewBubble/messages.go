package viewBubble

import (
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/charmbracelet/bubbles/list"
)

type StatusClearMsg struct{}

type WidgetSizeMsg struct {
	Width  int
	Height int
}

type NopMsg struct{}

type CurrentIssueReceivedMsg struct {
	Table *Table
	Issue *jira.Issue
	Pos   int
}

type EditorFinishedMsg struct{ err error }

type IssueCreatedMsg struct{ err error }

type IssueMovedMsg struct{ err error }

type IssueAssignedToEpicMsg struct{ err error }

type SelectedIssueMsg struct{ issue *jira.Issue }

type IssueCachedMsg struct {
	key   string
	issue *jira.Issue
}

type FuzzySelectorResultMsg struct {
	item         list.Item
	selectorType FuzzySelectorType
}

type IncomingIssueListMsg struct {
	issues []*jira.Issue
	index  int
}

type IncomingIssueMsg struct {
	issue *jira.Issue
	index int
}

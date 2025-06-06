package viewBubble

import (
	"github.com/jorres/jira-tui/pkg/jira"
	"github.com/charmbracelet/bubbles/v2/list"
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

type IssueEditedMsg struct {
	err    error
	stderr string
}

type IssueCreatedMsg struct {
	err    error
	stderr string
}

type IssueMovedMsg struct {
	err    error
	stderr string
}

type IssueAssignedToEpicMsg struct {
	err    error
	stderr string
}

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

type SetRenderStyleMsg struct {
	style string
}

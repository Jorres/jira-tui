package bubble

import (
	"github.com/charmbracelet/bubbles/v2/list"
	"github.com/jorres/jira-tui/internal/exp"
	"github.com/jorres/jira-tui/pkg/jira"
)

type StatusClearMsg struct{}

type WidgetSizeMsg struct {
	Width  int
	Height int
}

type NopMsg struct{}

type IssueEditedMsg struct {
	issueKey string
	err      error
	stderr   string
}

type IssueCreatedMsg struct {
	err    error
	stderr string
}

type IssueMovedMsg struct {
	issueKey string
	err      error
	stderr   string
}

type IssueAssignedToEpicMsg struct {
	issueKey string
	err      error
	stderr   string
}

type IssueBacklogToggleMsg struct {
	issueKey string
	err      error
	stderr   string
}

type SelectedIssueMsg struct{ issue *jira.Issue }

type FuzzySelectorResultMsg struct {
	item         list.Item
	selectorType FuzzySelectorType
}

type IncomingIssueListMsg struct {
	issues   []*jira.Issue
	index    int
	resolver *exp.BoardStateResolver
}

type IncomingIssueMsg struct {
	issue *jira.Issue
	index int
}

type SetRenderStyleMsg struct {
	style string
}

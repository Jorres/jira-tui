package exp

import (
	"fmt"

	"github.com/jorres/jira-tui/internal/debug"
	"github.com/jorres/jira-tui/internal/query"
	"github.com/jorres/jira-tui/pkg/jira"
)

type BoardStateResolver struct {
	backlogIssueKeys map[string]bool // Keys of issues currently in backlog
}

func (r *BoardStateResolver) IsOnBoard(issueKey string) bool {
	return !r.backlogIssueKeys[issueKey]
}

func (r *BoardStateResolver) SetBacklogState(issueKey string, newState BacklogState) {
	if newState == InBacklog {
		r.backlogIssueKeys[issueKey] = true
	} else {
		delete(r.backlogIssueKeys, issueKey)
	}
}

// fetchBacklogIssueKeys fetches all issue keys from the configured board's backlog
func fetchBacklogIssueKeys(client *jira.Client, boardID string, queryParams *query.IssueParams) (map[string]bool, error) {
	var jqlQuery string
	if queryParams != nil {
		q := &query.Issue{Flags: nil}
		q.SetParams(queryParams)
		jqlQuery = q.Get()
	}

	backlogResult, err := client.BacklogIssuesWithJQL(boardID, jqlQuery)
	if err != nil {
		return nil, err
	}

	issueKeys := make(map[string]bool)
	for _, issue := range backlogResult.Issues {
		issueKeys[issue.Key] = true
	}

	return issueKeys, nil
}

func CreateBoardStateResolver(client *jira.Client, boardID int, queryParams *query.IssueParams) *BoardStateResolver {
	if boardID == 0 {
		return nil
	}

	debug.Debug("Tab has board ID %d, fetching backlog issues", boardID)
	backlogIssueKeys, fetchErr := fetchBacklogIssueKeys(client, fmt.Sprintf("%d", boardID), queryParams)
	if fetchErr != nil {
		debug.Debug("Failed to fetch backlog issues: %v", fetchErr)
		return nil
	}

	return &BoardStateResolver{backlogIssueKeys: backlogIssueKeys}
}

type BacklogState int

const (
	Unknown = iota
	InBacklog
	OnBoard
)

// ToggleIssueBacklogState toggles an issue between board and backlog state using cached board state
func ToggleIssueBacklogState(client *jira.Client, boardID int, issue *jira.Issue, stateChecker *BoardStateResolver) (BacklogState, error) {
	if boardID == 0 {
		return Unknown, fmt.Errorf("no board ID configured for this tab")
	}

	if stateChecker == nil {
		return Unknown, fmt.Errorf("no board state information available")
	}

	boardIDStr := fmt.Sprintf("%d", boardID)
	isOnBoard := stateChecker.IsOnBoard(issue.Key)

	var err error
	if isOnBoard {
		err = client.MoveIssueToBacklog(boardIDStr, issue.Key)
		if err != nil {
			return OnBoard, fmt.Errorf("failed to move issue to backlog: %v", err)
		}
		return InBacklog, nil
	} else {
		err = client.MoveIssueToBoard(boardIDStr, issue.Key)
		if err != nil {
			return InBacklog, fmt.Errorf("failed to move issue to board: %v", err)
		}
		return OnBoard, nil
	}
}

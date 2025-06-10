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

// createBackgroundColorResolver creates a resolver function that uses backlog issue data
// func createBackgroundColorResolver(backlogIssueKeys map[string]bool) BacklightResolver {
// 	var backlogKeys []string
// 	for key := range backlogIssueKeys {
// 		backlogKeys = append(backlogKeys, key)
// 	}
// 	debug.Debug("Backlog issues: %v", backlogKeys)

// 	return func(issue *jira.Issue) *color.Color {
// 		isInBacklog := backlogIssueKeys[issue.Key]
// 		if isInBacklog {
// 			color := lipgloss.Color("67") // Issue is in backlog
// 			return &color
// 		}
// 		color := lipgloss.Color("62") // Issue is on board
// 		return &color
// 	}
// }

// createNilResolver creates a resolver that always returns nil (no custom coloring)
// func createNilResolver() BacklightResolver {
// 	return func(issue *jira.Issue) *color.Color {
// 		return nil // Use default table styling
// 	}
// }

// CreateBacklightResolver creates a background color resolver based on board configuration
// Returns both the resolver function and a board state checker
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

	stateResolver := &BoardStateResolver{backlogIssueKeys: backlogIssueKeys}

	// colorResolver := func(issue *jira.Issue) *color.Color {
	// 	if stateResolver.IsOnBoard(issue.Key) {
	// 		color := lipgloss.Color("62")
	// 		return &color
	// 	}
	// 	color := lipgloss.Color("67")
	// 	return &color
	// }

	return stateResolver
}

// ToggleIssueBacklogState toggles an issue between board and backlog state using cached board state
func ToggleIssueBacklogState(client *jira.Client, boardID int, issue *jira.Issue, stateChecker *BoardStateResolver) error {
	if boardID == 0 {
		return fmt.Errorf("no board ID configured for this tab")
	}

	if stateChecker == nil {
		return fmt.Errorf("no board state information available")
	}

	boardIDStr := fmt.Sprintf("%d", boardID)
	isOnBoard := stateChecker.IsOnBoard(issue.Key)

	var err error
	if isOnBoard {
		// Issue is on board, move to backlog
		debug.Debug("Issue %s is on board, moving to backlog", issue.Key)
		err = client.MoveIssueToBacklog(boardIDStr, issue.Key)
		if err != nil {
			return fmt.Errorf("failed to move issue to backlog: %v", err)
		}
		debug.Debug("Successfully moved issue %s to backlog", issue.Key)
	} else {
		// Issue is in backlog, move to board
		debug.Debug("Issue %s is in backlog, moving to board", issue.Key)
		err = client.MoveIssueToBoard(boardIDStr, issue.Key)
		if err != nil {
			return fmt.Errorf("failed to move issue to board: %v", err)
		}
		debug.Debug("Successfully moved issue %s to board", issue.Key)
	}
	return nil
}

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/jorres/jira-tui/internal/debug"
)

const (
	// BoardTypeScrum represents a scrum board type.
	BoardTypeScrum = "scrum"
	// BoardTypeAll represents all board types.
	BoardTypeAll = ""
)

// BoardResult holds response from /board endpoint.
type BoardResult struct {
	MaxResults int      `json:"maxResults"`
	Total      int      `json:"total"`
	Boards     []*Board `json:"values"`
}

// BoardIssueResult holds response from /board/{boardId}/issue endpoint.
type BoardIssueResult struct {
	MaxResults int      `json:"maxResults"`
	StartAt    int      `json:"startAt"`
	Total      int      `json:"total"`
	Issues     []*Issue `json:"issues"`
}

// Boards gets all boards of a given type in a project.
func (c *Client) Boards(project, boardType string) (*BoardResult, error) {
	path := fmt.Sprintf("/board?projectKeyOrId=%s", project)
	if boardType != "" {
		path += fmt.Sprintf("&type=%s", boardType)
	}

	return c.board(path)
}

// BoardSearch fetches boards with the given name in a project.
func (c *Client) BoardSearch(project, name string) (*BoardResult, error) {
	path := fmt.Sprintf("/board?projectKeyOrId=%s&name=%s", project, name)

	return c.board(path)
}

func (c *Client) board(path string) (*BoardResult, error) {
	res, err := c.GetV1Agile(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out BoardResult

	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}

// BacklogIssues gets all backlog issues for a specific board.
func (c *Client) BacklogIssues(boardID string) (*BoardIssueResult, error) {
	return c.BacklogIssuesWithJQL(boardID, "")
}

// BacklogIssuesWithJQL gets all backlog issues for a specific board with optional JQL filtering.
func (c *Client) BacklogIssuesWithJQL(boardID, jql string) (*BoardIssueResult, error) {
	path := fmt.Sprintf("/board/%s/backlog?maxResults=100", boardID)

	if jql != "" {
		path += "&jql=" + url.QueryEscape(jql)
	}

	res, err := c.GetV1Agile(context.Background(), path, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, formatUnexpectedResponse(res)
	}

	var out BoardIssueResult
	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}

// MoveIssueToBacklog moves an issue to the backlog for a specific board.
func (c *Client) MoveIssueToBacklog(boardID, issueKey string) error {
	path := fmt.Sprintf("/backlog/%s/issue", boardID)

	body := map[string]interface{}{
		"issues": []string{issueKey},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	debug.Debug("Moving issue %s to backlog for board %s", issueKey, boardID)
	debug.Debug("API Path: %s", path)
	debug.Debug("Request body: %s", string(bodyBytes))

	headers := Header{"Content-Type": "application/json"}
	res, err := c.PostV1(context.Background(), path, bodyBytes, headers)
	if err != nil {
		debug.Debug("HTTP request failed: %v", err)
		return err
	}
	if res == nil {
		debug.Debug("Empty response received")
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	debug.Debug("HTTP Status Code: %d", res.StatusCode)

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		debug.Debug("Unexpected status code, returning error")
		return formatUnexpectedResponse(res)
	}

	debug.Debug("Successfully moved issue %s to backlog", issueKey)
	return nil
}

// MoveIssueToBoard moves an issue from backlog to the board.
func (c *Client) MoveIssueToBoard(boardID, issueKey string) error {
	path := fmt.Sprintf("/board/%s/issue", boardID)

	body := map[string]interface{}{
		"issues": []string{issueKey},
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %v", err)
	}

	debug.Debug("Moving issue %s to board %s", issueKey, boardID)
	debug.Debug("API Path: %s", path)
	debug.Debug("Request body: %s", string(bodyBytes))

	headers := Header{"Content-Type": "application/json"}
	res, err := c.PostV1(context.Background(), path, bodyBytes, headers)
	if err != nil {
		debug.Debug("HTTP request failed: %v", err)
		return err
	}
	if res == nil {
		debug.Debug("Empty response received")
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	debug.Debug("HTTP Status Code: %d", res.StatusCode)

	if res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusOK {
		debug.Debug("Unexpected status code, returning error")
		return formatUnexpectedResponse(res)
	}

	debug.Debug("Successfully moved issue %s to board", issueKey)
	return nil
}

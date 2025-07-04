package ui

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jorres/jira-tui/api"
	"github.com/jorres/jira-tui/internal/bubble"
	"github.com/jorres/jira-tui/internal/cmdutil"
	D "github.com/jorres/jira-tui/internal/debug"
	"github.com/jorres/jira-tui/internal/query"
	"github.com/jorres/jira-tui/pkg/jira"
)

var _ = D.Debug

const helpText = `UI opens up a comprehensive UI. Press ? for help right after ui opens.`

// NewCmdUI is an issue command.
func NewCmdUI() *cobra.Command {
	cmd := cobra.Command{
		Use:         "ui",
		Short:       "UI opens up a comprehensive UI",
		Long:        helpText,
		Aliases:     []string{},
		Annotations: map[string]string{"cmd:main": "true"},
		Args:        cobra.NoArgs,
		Run:         ui,
	}

	SetFlags(&cmd)

	return &cmd
}

func ui(cmd *cobra.Command, args []string) {
	server := viper.GetString("server")
	project := viper.GetString("project.key")

	debug, err := cmd.Flags().GetBool("debug")
	cmdutil.ExitIfError(err)

	// Read tab configuration from viper
	var tabConfigs []ListTabConfig
	err = viper.UnmarshalKey("ui.list.tabs", &tabConfigs)
	if err != nil {
		cmdutil.ExitIfError(err)
	}

	columns, err := cmd.Flags().GetString("columns")
	cmdutil.ExitIfError(err)

	columnsList := func() []string {
		if columns != "" {
			return strings.Split(columns, ",")
		}
		return []string{}
	}()
	timezone := viper.GetString("timezone")

	projectType := viper.GetString("project.type")
	epicQ := query.NewDefaultIssue(project, cmd.Flags())
	if projectType == jira.ProjectTypeNextGen {
		epicQ.Params().IssueType = viper.GetString("next_gen.epic_task_name")
	}
	epicQ.Params().Status = []string{}
	epicQ.Params().Assignee = ""
	fetchAllEpics := MakeFetcherFromQuery(epicQ, debug)

	var tabs []*bubble.TabConfig
	var total int

	if len(tabConfigs) <= 1 {
		q := query.NewDefaultIssue(project, cmd.Flags())
		fetchIssuesWithArgs := MakeFetcherFromQuery(q, debug)

		_, total = fetchIssuesWithArgs()

		if total == 0 {
			fmt.Println()
			cmdutil.Failed("No result found for given query in project %q", project)
			return
		}

		// Use the default board ID from config for single tab
		defaultBoardId := viper.GetInt("board.id")

		tabs = []*bubble.TabConfig{
			{
				Project:     project,
				Name:        "Issues",
				Columns:     columnsList,
				BoardId:     defaultBoardId,
				QueryParams: &query.IssueParams{},
				FetchIssues: fetchIssuesWithArgs,
				FetchEpics:  fetchAllEpics,
			},
		}
	} else {
		tabs = make([]*bubble.TabConfig, len(tabConfigs))
		total = 0

		for i, tabConfig := range tabConfigs {
			tabProject := project
			if tabConfig.Project != "" {
				tabProject = tabConfig.Project
			}

			fetchIssues := MakeFetcherFromTabConfig(tabProject, cmd.Flags(), tabConfig, debug)

			tabs[i] = &bubble.TabConfig{
				Project:     tabProject,
				Name:        tabConfig.Name,
				Columns:     tabConfig.Columns,
				BoardId:     tabConfig.BoardId,
				QueryParams: &tabConfig.IssueParams,
				FetchIssues: fetchIssues,
				FetchEpics:  fetchAllEpics,
			}
		}
	}

	bubble.RunMainUI(project, server, total, tabs, timezone, debug)
}

type ListTabConfig struct {
	Name              string   `mapstructure:"name"`
	Project           string   `mapstructure:"project"`
	Columns           []string `mapstructure:"columns"`
	BoardId           int      `mapstructure:"boardId"`
	query.IssueParams `mapstructure:",squash"`
}

// MakeFetcherFromTabConfig creates a fetcher function from a tab configuration
func MakeFetcherFromTabConfig(project string, baseFlags query.FlagParser, tabConfig ListTabConfig, debug bool) func() ([]*jira.Issue, int) {
	return func() ([]*jira.Issue, int) {
		// Replace the entire params with our config, but preserve defaults
		params := tabConfig.IssueParams
		if params.OrderBy == "" {
			params.OrderBy = "created"
		}
		if params.Limit == 0 {
			params.Limit = 300
		}

		q := &query.Issue{
			Flags: baseFlags,
		}

		params.Project = project
		q.SetParams(&params)

		issues, total, err := func() ([]*jira.Issue, int, error) {
			resp, err := api.ProxySearch(api.DefaultClient(debug), q.Get(), q.Params().From, q.Params().Limit)
			if err != nil {
				return nil, 0, err
			}

			return resp.Issues, resp.Total, nil
		}()

		cmdutil.ExitIfError(err)
		return issues, total
	}
}

func MakeFetcherFromQuery(q *query.Issue, debug bool) func() ([]*jira.Issue, int) {
	return func() ([]*jira.Issue, int) {
		issues, total, err := func() ([]*jira.Issue, int, error) {
			D.Debug("limit", q.Params().Limit)
			resp, err := api.ProxySearch(api.DefaultClient(debug), q.Get(), q.Params().From, q.Params().Limit)
			if err != nil {
				return nil, 0, err
			}

			// TODO @jorres we lost an ability to query epics here, see `epic list` command, it would fail in case of non-next-gen project

			// 	var resp *jira.SearchResult
			// 	if projectType == jira.ProjectTypeNextGen {
			// 		q.Params().Parent = key
			// 		q.Params().IssueType = viper.GetString("next_gen.epic_task_name")

			// 		resp, err = client.Search(q.Get(), q.Params().From, q.Params().Limit)
			// 	} else {
			// 		resp, err = client.EpicIssues(key, q.Get(), q.Params().From, q.Params().Limit)
			// 	}

			return resp.Issues, resp.Total, nil
		}()

		cmdutil.ExitIfError(err)

		return issues, total
	}
}

// SetFlags sets flags supported by a list command.
func SetFlags(cmd *cobra.Command) {
	cmd.Flags().SortFlags = false

	cmd.Flags().String("columns", "", "Comma separated list of columns to display in the plain mode.\n"+
		fmt.Sprintf("Accepts: %s", strings.Join(bubble.ValidIssueColumns(), ", ")))
	cmd.Flags().Uint("fixed-columns", 1, "Number of fixed columns in the interactive mode")
}

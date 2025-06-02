package list

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ankitpokhrel/jira-cli/api"
	"github.com/ankitpokhrel/jira-cli/internal/cmd/issue/list"
	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/internal/query"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"

	"github.com/ankitpokhrel/jira-cli/internal/viewBubble"
)

var _ = log.Fatal

const (
	helpText = `List lists top 100 epics.

By default epics are displayed in an explorer view. You can use --table
and --plain flags to display output in different modes.`

	examples = `# Display epics in an explorer view
$ jira epic list

# Display epics or epic issues in an interactive table view
$ jira epic list --table
$ jira epic list <KEY>

# Display epics or epic issues in a plain table view
$ jira epic list --table --plain
$ jira epic list <KEY> --plain

# Display epics or epic issues in a plain table view without headers
$ jira epic list --table --plain --no-headers
$ jira epic list <KEY> --plain --no-headers

# Display some columns of epic or epic issues in a plain table view
$ jira epic list --table --plain --columns key,summary,status
$ jira epic list <KEY> --plain --columns type,key,summary`
)

// NewCmdList is a list command.
func NewCmdList() *cobra.Command {
	return &cobra.Command{
		Use:     "list [EPIC-KEY]",
		Short:   "List lists issues in a project",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"lists", "ls"},
		Annotations: map[string]string{
			"help:args": "[EPIC-KEY]\tKey for the issue of type epic, eg: ISSUE-1",
		},
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.Flags().Set("type", "Epic")
			cmdutil.ExitIfError(err)

			epicList(cmd, args)
		},
	}
}

// SetFlags sets flags supported by an epic list command.
func SetFlags(cmd *cobra.Command) {
	setFlags(cmd)
	hideFlags(cmd)
}

func epicList(cmd *cobra.Command, args []string) {
	server := viper.GetString("server")
	project := viper.GetString("project.key")
	projectType := viper.GetString("project.type")

	debug, err := cmd.Flags().GetBool("debug")
	cmdutil.ExitIfError(err)

	client := api.DefaultClient(debug)

	if len(args) == 0 {
		epicExplorerView(cmd, cmd.Flags(), project, projectType, server, client)
	} else {
		key := cmdutil.GetJiraIssueKey(project, args[0])
		singleEpicView(cmd, cmd.Flags(), key, project, projectType, server, client)
	}
}

func singleEpicView(cmd *cobra.Command, flags query.FlagParser, key, project, projectType, server string, client *jira.Client) {
	err := flags.Set("type", "") // Unset issue type.
	cmdutil.ExitIfError(err)
	debug, err := cmd.Flags().GetBool("debug")
	cmdutil.ExitIfError(err)

	q, err := query.NewIssue(project, flags)
	cmdutil.ExitIfError(err)
	if projectType == jira.ProjectTypeNextGen {
		q.Params().IssueType = ""
	}
	q.Params().Parent = key
	fetchAllIssuesOfEpic := list.MakeFetcherFromQuery(q, debug)

	_, total := fetchAllIssuesOfEpic()
	if total == 0 {
		fmt.Println()
		cmdutil.Failed("No result found for given query in project %q", project)
		return
	}

	plain, err := flags.GetBool("plain")
	cmdutil.ExitIfError(err)

	noHeaders, err := flags.GetBool("no-headers")
	cmdutil.ExitIfError(err)

	noTruncate, err := flags.GetBool("no-truncate")
	cmdutil.ExitIfError(err)

	fixedColumns, err := flags.GetUint("fixed-columns")
	cmdutil.ExitIfError(err)

	columns, err := flags.GetString("columns")
	cmdutil.ExitIfError(err)

	displayFormat := viewBubble.DisplayFormat{
		Plain:        plain,
		NoHeaders:    noHeaders,
		NoTruncate:   noTruncate,
		FixedColumns: fixedColumns,
		Columns: func() []string {
			if columns != "" {
				return strings.Split(columns, ",")
			}
			return []string{}
		}(),
		Timezone: viper.GetString("timezone"),
	}

	tabs := []*viewBubble.TabConfig{
		{
			Name:        "Epics",
			Project:     project,
			FetchIssues: fetchAllIssuesOfEpic,
			FetchEpics:  func() ([]*jira.Issue, int) { return []*jira.Issue{}, 0 },
		},
	}
	v := viewBubble.NewIssueList(project, server, total, tabs, displayFormat, debug)

	cmdutil.ExitIfError(v.RunView())
}

func epicExplorerView(cmd *cobra.Command, flags query.FlagParser, project, projectType, server string, client *jira.Client) {
	debug, err := cmd.Flags().GetBool("debug")
	cmdutil.ExitIfError(err)

	q, err := query.NewIssue(project, flags)
	cmdutil.ExitIfError(err)
	if projectType == jira.ProjectTypeNextGen {
		q.Params().IssueType = viper.GetString("next_gen.epic_task_name")
	}
	fetchAllEpics := list.MakeFetcherFromQuery(q, debug)
	if err != nil {
		cmdutil.ExitIfError(err)
	}

	_, total := fetchAllEpics()
	if total == 0 {
		fmt.Println()
		return
	}

	fixedColumns, err := flags.GetUint("fixed-columns")
	cmdutil.ExitIfError(err)

	plain, err := flags.GetBool("plain")
	cmdutil.ExitIfError(err)

	noHeaders, err := flags.GetBool("no-headers")
	cmdutil.ExitIfError(err)

	noTruncate, err := flags.GetBool("no-truncate")
	cmdutil.ExitIfError(err)

	if err != nil {
		cmdutil.ExitIfError(err)
	}

	displayFormat := viewBubble.DisplayFormat{
		Plain:        plain,
		NoHeaders:    noHeaders,
		NoTruncate:   noTruncate,
		FixedColumns: fixedColumns,
		Timezone:     viper.GetString("timezone"),
	}

	tabs := []*viewBubble.TabConfig{
		{
			Name:        "Epics",
			Project:     project,
			FetchIssues: fetchAllEpics,
			FetchEpics:  fetchAllEpics,
		},
	}
	v := viewBubble.NewIssueList(project, server, total, tabs, displayFormat, debug)
	cmdutil.ExitIfError(v.RunView())
}

func setFlags(cmd *cobra.Command) {
	list.SetFlags(cmd)
	cmd.Flags().Bool("table", false, "Display epics in table view")
}

func hideFlags(cmd *cobra.Command) {
	cmdutil.ExitIfError(cmd.Flags().MarkHidden("type"))
	cmdutil.ExitIfError(cmd.Flags().MarkHidden("parent"))
}

package sprint

import (
	"github.com/spf13/cobra"

	"github.com/jorres/jira-tui/internal/cmd/sprint/add"
	"github.com/jorres/jira-tui/internal/cmd/sprint/close"
	"github.com/jorres/jira-tui/internal/cmd/sprint/list"
)

const helpText = `Sprint manage sprints in a project board. See available commands below.`

// NewCmdSprint is a sprint command.
func NewCmdSprint() *cobra.Command {
	cmd := cobra.Command{
		Use:         "sprint",
		Short:       "Sprint manage sprints in a project board",
		Long:        helpText,
		Aliases:     []string{"sprints"},
		Annotations: map[string]string{"cmd:main": "true"},
		RunE:        sprint,
	}

	lc := list.NewCmdList()
	ac := add.NewCmdAdd()
	cc := close.NewCmdClose()

	cmd.AddCommand(lc, ac, cc)

	list.SetFlags(lc)

	return &cmd
}

func sprint(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}

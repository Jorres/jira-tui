package edit

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/jorres/md2adf-translator/adf"
	"github.com/jorres/md2adf-translator/adf2md"
	"github.com/jorres/md2adf-translator/md2adf"

	"github.com/jorres/jira-tui/api"
	"github.com/jorres/jira-tui/internal/cmdcommon"
	"github.com/jorres/jira-tui/internal/cmdutil"
	"github.com/jorres/jira-tui/internal/debug"
	"github.com/jorres/jira-tui/internal/editing"
	"github.com/jorres/jira-tui/internal/query"
	"github.com/jorres/jira-tui/internal/viewBubble"
	"github.com/jorres/jira-tui/pkg/jira"
	"github.com/jorres/jira-tui/pkg/jira/filter/issue"
	"github.com/jorres/jira-tui/pkg/md"
	"github.com/jorres/jira-tui/pkg/surveyext"
)

var _ = debug.Debug

const (
	helpText = `Edit an issue in a given project with minimal information.`
	examples = `$ jira issue edit ISSUE-1

# Edit issue in the configured project
$ jira issue edit ISSUE-1 -s"New Bug" -yHigh -lbug -lurgent -CBackend -b"Bug description"

# Use --no-input option to disable interactive prompt
$ jira issue edit ISSUE-1 -s"New updated summary" --no-input

# Use pipe to read the description body directly from standard input
$ echo "Description from stdin" | jira issue edit ISSUE-1 -s"New updated summary"  --no-input

# Use minus (-) to remove label, component or fixVersion
$ jira issue edit ISSUE-1 --label -urgent --component -BE --fix-version -v1.0`
)

// NewCmdEdit is an edit command.
func NewCmdEdit() *cobra.Command {
	cmd := cobra.Command{
		Use:     "edit ISSUE-KEY",
		Short:   "Edit an issue in a project",
		Long:    helpText,
		Example: examples,
		Aliases: []string{"update", "modify"},
		Annotations: map[string]string{
			"help:args": `ISSUE-KEY	Issue key, eg: ISSUE-1`,
		},
		Args: cobra.MinimumNArgs(1),
		Run:  edit,
	}

	setFlags(&cmd)

	return &cmd
}

func checkForUnsupportedNodeTypes(translator *adf2md.Translator, doc *adf.ADFNode) {
	unsupported := translator.CheckSupport(doc)
	if len(unsupported) > 0 {
		keys := make([]adf.NodeType, 0, len(unsupported))
		for nt := range unsupported {
			keys = append(keys, nt)
		}
		cmdutil.ExitIfError(fmt.Errorf(
			"You are trying to edit an issue which contains following item types: %v.\n"+
				"If edited, sending this to Jira server will corrupt your issue.\n"+
				"Please wait for md -> adf serialization support for these types",
			keys,
		))
	}
}

func edit(cmd *cobra.Command, args []string) {
	server := viper.GetString("server")
	project := viper.GetString("project.key")

	params := parseArgsAndFlags(cmd.Flags(), args, project)
	client := api.DefaultClient(params.debug)
	ec := editCmd{
		client: client,
		params: params,
	}

	issue, err := func() (*jira.Issue, error) {
		issue, err := api.ProxyGetIssue(client, params.issueKey, issue.NewNumCommentsFilter(10))
		if err != nil {
			return nil, err
		}

		return issue, nil
	}()
	cmdutil.ExitIfError(err)

	var originalBody string

	var adf2mdTranslator *adf2md.Translator
	// Create a user email resolver function
	emailResolver := func(userID string) string {
		return editing.ResolveUserIDToEmail(userID, client, project)
	}

	adf2mdTranslator = adf2md.NewTranslator(adf2md.NewJiraMarkdownTranslator(
		adf2md.WithUserEmailResolver(emailResolver),
	))

	if issue.Fields.Description != nil {
		if adfBody, ok := issue.Fields.Description.(*adf.ADFNode); ok {
			checkForUnsupportedNodeTypes(adf2mdTranslator, adfBody)
			originalBody = adf2mdTranslator.Translate(adfBody)
		} else {
			originalBody = issue.Fields.Description.(string)
		}
	}

	// Prepare content with comments separated by DO NOT EDIT lines
	contentWithComments := originalBody

	// Add comments if they exist
	if issue.Fields.Comment.Total > 0 {
		for _, comment := range issue.Fields.Comment.Comments {

			at := viewBubble.FormatDateTime(comment.Created, jira.RFC3339, "Local")
			contentWithComments += fmt.Sprintf(
				"\n\n# DO NOT EDIT THIS LINE - Comment by %s (at %s)\n\n",
				comment.Author.GetDisplayableName(),
				at,
			)

			// Convert comment body from ADF to markdown if needed
			var commentBody string
			if adfBody, ok := comment.Body.(*adf.ADFNode); ok {
				checkForUnsupportedNodeTypes(adf2mdTranslator, adfBody)
				commentBody = adf2mdTranslator.Translate(adfBody)
			} else {
				commentBody = comment.Body.(string)
			}

			contentWithComments += commentBody
		}
	}

	// Update originalBody to include comments for the editor
	originalBody = contentWithComments

	// HACK. TODO think of a better solution
	// otherwise if we insert inline node into panel, '{/panel}' would
	// literally become part of a paragraph
	strings.ReplaceAll(originalBody, "{/panel}", "\n{/panel}")

	cmdutil.ExitIfError(ec.askQuestions(issue, originalBody))

	if !params.noInput {
		getAnswers(client, params, issue)
	}

	md2adfTranslator, err := editing.PrepareMD2AdfTranslator(params.body, client, issue.Key, adf2mdTranslator)
	if err != nil {
		cmdutil.ExitIfError(err)
	}

	// Parse the edited content back into body and comments
	if params.body != "" {
		separatorPattern := regexp.MustCompile(`(?m)^# DO NOT EDIT THIS LINE - Comment by .* \(.*\)$`)
		segments := separatorPattern.Split(params.body, -1)

		// First segment is the body
		params.body = strings.TrimSpace(segments[0])

		// Remaining segments are comments
		expectedComments := len(issue.Fields.Comment.Comments)
		actualComments := len(segments) - 1

		if actualComments != expectedComments {
			cmdutil.ExitIfError(fmt.Errorf(
				"Comment count mismatch: expected %d comments, got %d. DO NOT EDIT separator lines must not be modified.",
				expectedComments,
				actualComments,
			))
		}

		// Parse comments back
		for i, commentText := range segments[1:] {
			params.comments = append(params.comments, editComment{
				id:   issue.Fields.Comment.Comments[i].ID,
				body: strings.TrimSpace(commentText),
			})
		}
	}

	// Use stdin only if nothing is passed to --body
	if params.body == "" && cmdutil.StdinHasData() {
		b, err := cmdutil.ReadFile("-")
		if err != nil {
			cmdutil.Failed("Error: %s", err)
		}
		params.body = string(b)
	}

	// Keep body as is if there were no changes.
	if params.body != "" && params.body == originalBody {
		// TODO there are some bugs, like stray (or missing) '\n' after links
		// which cause the content in jira to update all the time, and this edit avoidance
		// does not work because strings are technically different
		params.body = ""
	}

	// TODO remove from editComments all the comments that are not edited (to prevent extra queries)

	labels := params.labels
	labels = append(labels, issue.Fields.Labels...)

	components := make([]string, 0, len(issue.Fields.Components)+len(params.components))
	for _, c := range issue.Fields.Components {
		components = append(components, c.Name)
	}
	components = append(components, params.components...)

	fixVersions := make([]string, 0, len(issue.Fields.FixVersions)+len(params.fixVersions))
	for _, fv := range issue.Fields.FixVersions {
		fixVersions = append(fixVersions, fv.Name)
	}
	fixVersions = append(fixVersions, params.fixVersions...)

	affectsVersions := make([]string, 0, len(issue.Fields.AffectsVersions)+len(params.affectsVersions))
	for _, fv := range issue.Fields.AffectsVersions {
		affectsVersions = append(affectsVersions, fv.Name)
	}
	affectsVersions = append(affectsVersions, params.affectsVersions...)

	err = func() error {
		s := cmdutil.Info("Updating an issue...")
		defer s.Stop()

		useV3API := true
		if viper.GetString("installation") == jira.InstallationTypeLocal {
			err := checkV2Safe(params.body, params.comments, md2adfTranslator)

			if err != nil {
				cmdutil.ExitIfError(jira.V3ContentToV2EndpointError(err))
			}

			// if there are no unsafe elements, we have to resort to V2 api...
			useV3API = false
		}

		debug.Debug("useV3API: ", useV3API)
		// Now process body and comments based on the chosen API version
		debug.Debug("markdown body", string(params.body))
		body, bodyIsRawADF := processBodyForAPI(params.body, useV3API, md2adfTranslator)
		debug.Debug("adf body", string(body))

		for _, comment := range params.comments {
			debug.Debug("markdown comment", comment.id, comment.body)
		}
		editComments := processCommentsForAPI(params.comments, useV3API, md2adfTranslator)
		for _, comment := range editComments {
			debug.Debug("adf comment", comment.ID, comment.BodyIsRawADF, comment.Body)
		}

		parent := cmdutil.GetJiraIssueKey(project, params.parentIssueKey)
		if parent == "" && issue.Fields.Parent != nil {
			parent = issue.Fields.Parent.Key
		}

		// Create EditRequest with comments
		edr := jira.EditRequest{
			ParentIssueKey:  parent,
			Summary:         params.summary,
			Body:            body,
			BodyIsRawADF:    bodyIsRawADF,
			Comments:        editComments,
			Priority:        params.priority,
			Labels:          labels,
			Components:      components,
			FixVersions:     fixVersions,
			AffectsVersions: affectsVersions,
			CustomFields:    params.customFields,
		}
		if configuredCustomFields, err := cmdcommon.GetConfiguredCustomFields(); err == nil {
			cmdcommon.ValidateCustomFields(edr.CustomFields, configuredCustomFields)
			edr.WithCustomFields(configuredCustomFields)
		}

		// Choose API version based on content safety
		if useV3API {
			return client.Edit(params.issueKey, &edr)
		} else {
			return client.EditV2(params.issueKey, &edr)
		}
	}()

	cmdutil.ExitIfError(err)

	cmdutil.Success("Issue updated\n%s", cmdutil.GenerateServerBrowseURL(server, params.issueKey))

	handleUserAssign(project, params.issueKey, params.assignee, client)

	if web, _ := cmd.Flags().GetBool("web"); web {
		err := cmdutil.Navigate(server, params.issueKey)
		cmdutil.ExitIfError(err)
	}
}

func defaultSurveyOptions() []survey.AskOpt {
	_, height, _ := term.GetSize(int(os.Stdout.Fd()))
	return []survey.AskOpt{
		survey.WithRemoveSelectNone(),
		survey.WithRemoveSelectAll(),
		survey.WithKeepFilter(true),
		survey.WithPageSize(height - 10),
	}
}

func checkV2Safe(body string, comments []editComment, translator *md2adf.Translator) error {
	if err := translator.CheckSafeForV2(body); err != nil {
		return fmt.Errorf("body is not suitable for translation: %w", err)
	}

	for _, comment := range comments {
		if err := translator.CheckSafeForV2(comment.body); err != nil {
			return fmt.Errorf("comment is not suitable for translation: %w", err)
		}
	}

	return nil
}

func getAnswers(client *jira.Client, params *editParams, issue *jira.Issue) {
	answer := struct{ Action string }{}
	for answer.Action != cmdcommon.ActionSubmit {
		err := survey.Ask(
			[]*survey.Question{cmdcommon.GetNextAction()},
			&answer,
			defaultSurveyOptions()...,
		)
		cmdutil.ExitIfError(err)

		switch answer.Action {
		case cmdcommon.ActionCancel:
			cmdutil.Failed("Action aborted")
		case cmdcommon.ActionMetadata:
			ans := struct{ Metadata []string }{}
			editMetadata, err := client.GetEditMetadata(params.issueKey)
			if err != nil {
				cmdutil.ExitIfError(fmt.Errorf("Failed to get issue edit metadata: %w", err))
			}

			// Convert EditMetadata to []*Field format for compatibility
			var customFields []*jira.Field
			for _, fieldMeta := range editMetadata.Fields {
				if fieldMeta.Schema.Custom != "" { // Only include custom fields
					customFields = append(customFields, &jira.Field{
						ID:     fieldMeta.Key,
						Name:   fieldMeta.Name,
						Custom: true,
						Schema: struct {
							DataType string `json:"type"`
							Items    string `json:"items,omitempty"`
							FieldID  int    `json:"customId,omitempty"`
						}{
							DataType: fieldMeta.Schema.Type,
							Items:    fieldMeta.Schema.Items,
							FieldID:  fieldMeta.Schema.CustomId,
						},
					})
				}
			}

			// Create a custom metadata question that only includes editable fields
			var metadataOptions []string
			if _, exists := editMetadata.Fields["priority"]; exists {
				metadataOptions = append(metadataOptions, "Priority")
			}
			if _, exists := editMetadata.Fields["components"]; exists {
				metadataOptions = append(metadataOptions, "Components")
			}
			if _, exists := editMetadata.Fields["labels"]; exists {
				metadataOptions = append(metadataOptions, "Labels")
			}
			if _, exists := editMetadata.Fields["fixVersions"]; exists {
				metadataOptions = append(metadataOptions, "FixVersions")
			}
			if _, exists := editMetadata.Fields["versions"]; exists {
				metadataOptions = append(metadataOptions, "AffectsVersions")
			}

			// Add custom fields to options
			for _, field := range customFields {
				metadataOptions = append(metadataOptions, field.Name)
			}

			metadataQuestion := []*survey.Question{
				{
					Name: "metadata",
					Prompt: &survey.MultiSelect{
						Message: "What would you like to add?",
						Options: metadataOptions,
					},
				},
			}

			err = survey.Ask(metadataQuestion, &ans, defaultSurveyOptions()...)

			cmdutil.ExitIfError(err)

			if len(ans.Metadata) > 0 {
				keys := []string{}
				for _, v := range ans.Metadata {
					keys = append(keys, v)
				}
				qs := getEditMetadataQuestions(keys, customFields, issue, editMetadata, client, params.issueKey)
				ans := make(map[string]any)

				err := survey.Ask(qs, &ans, defaultSurveyOptions()...)
				cmdutil.ExitIfError(err)

				if priority, ok := ans["Priority"].(string); ok && priority != "" {
					params.priority = priority
				}
				if labels, ok := ans["Labels"].(string); ok && labels != "" {
					params.labels = strings.Split(labels, ",")
				}
				if components, ok := ans["Components"].(string); ok && components != "" {
					params.components = strings.Split(components, ",")
				}
				if fixVers, ok := ans["FixVersions"].(string); ok && fixVers != "" {
					params.fixVersions = strings.Split(fixVers, ",")
				}
				if affVers, ok := ans["AffectsVersions"].(string); ok && affVers != "" {
					params.affectsVersions = strings.Split(affVers, ",")
				}

				for k, v := range ans {
					// customfield_12... -> channel
					debug.Debug(k, v)
					params.customFields[k] = v.(string)
				}
			}
		}
	}
}

func handleUserAssign(project, key, assignee string, client *jira.Client) {
	if assignee == "" {
		return
	}
	if assignee == "x" {
		if err := api.ProxyAssignIssue(client, key, nil, jira.AssigneeNone); err != nil {
			cmdutil.Failed("Unable to unassign user: %s", err.Error())
		}
		return
	}
	user, err := api.ProxyUserSearch(client, &jira.UserSearchOptions{
		Query:   assignee,
		Project: project,
	})
	if err != nil || len(user) == 0 {
		cmdutil.Failed("Unable to find assignee")
	}
	if err = api.ProxyAssignIssue(client, key, user[0], assignee); err != nil {
		cmdutil.Failed("Unable to set assignee: %s", err.Error())
	}
}

type editCmd struct {
	client *jira.Client
	params *editParams
}

func (ec *editCmd) askQuestions(issue *jira.Issue, originalBody string) error {
	if ec.params.noInput {
		return nil
	}

	var qs []*survey.Question

	if ec.params.summary == "" {
		qs = append(qs, &survey.Question{
			Name: "summary",
			Prompt: &survey.Input{
				Message: "Summary",
				Default: issue.Fields.Summary,
			},
			Validate: survey.Required,
		})
	}

	if ec.params.body == "" {
		qs = append(qs, &survey.Question{
			Name: "body",
			Prompt: &surveyext.JiraEditor{
				Editor: &survey.Editor{
					Message:       "Description",
					Default:       originalBody,
					HideDefault:   true,
					AppendDefault: true,
				},
				BlankAllowed: true,
			},
		})
	}

	ans := struct{ Summary, Body string }{}
	err := survey.Ask(qs, &ans, defaultSurveyOptions()...)
	if err != nil {
		return err
	}

	if ec.params.summary == "" {
		ec.params.summary = ans.Summary
	}
	if ec.params.body == "" {
		ec.params.body = ans.Body
	}

	return nil
}

type editComment struct {
	id   string
	body string
}

type editParams struct {
	issueKey       string
	parentIssueKey string
	summary        string
	body           string
	comments       []editComment
	assignee       string

	priority        string
	labels          []string
	components      []string
	fixVersions     []string
	affectsVersions []string

	customFields map[string]string
	noInput      bool
	debug        bool
}

func parseArgsAndFlags(flags query.FlagParser, args []string, project string) *editParams {
	parentIssueKey, err := flags.GetString("parent")
	cmdutil.ExitIfError(err)

	summary, err := flags.GetString("summary")
	cmdutil.ExitIfError(err)

	body, err := flags.GetString("body")
	cmdutil.ExitIfError(err)

	priority, err := flags.GetString("priority")
	cmdutil.ExitIfError(err)

	assignee, err := flags.GetString("assignee")
	cmdutil.ExitIfError(err)

	labels, err := flags.GetStringArray("label")
	cmdutil.ExitIfError(err)

	components, err := flags.GetStringArray("component")
	cmdutil.ExitIfError(err)

	fixVersions, err := flags.GetStringArray("fix-version")
	cmdutil.ExitIfError(err)

	affectsVersions, err := flags.GetStringArray("affects-version")
	cmdutil.ExitIfError(err)

	custom, err := flags.GetStringToString("custom")
	cmdutil.ExitIfError(err)

	noInput, err := flags.GetBool("no-input")
	cmdutil.ExitIfError(err)

	debug, err := flags.GetBool("debug")
	cmdutil.ExitIfError(err)

	return &editParams{
		issueKey:        cmdutil.GetJiraIssueKey(project, args[0]),
		parentIssueKey:  parentIssueKey,
		summary:         summary,
		body:            body,
		priority:        priority,
		assignee:        assignee,
		labels:          labels,
		components:      components,
		fixVersions:     fixVersions,
		affectsVersions: affectsVersions,
		customFields:    custom,
		noInput:         noInput,
		debug:           debug,
	}
}

func getEditMetadataQuestions(meta []string, customFields []*jira.Field, issue *jira.Issue, editMetadata *jira.EditMetadata, client *jira.Client, issueKey string) []*survey.Question {
	var qs []*survey.Question

	fixVersions := make([]string, 0, len(issue.Fields.FixVersions))
	for _, fv := range issue.Fields.FixVersions {
		fixVersions = append(fixVersions, fv.Name)
	}

	affectsVersions := make([]string, 0, len(issue.Fields.AffectsVersions))
	for _, fv := range issue.Fields.AffectsVersions {
		affectsVersions = append(affectsVersions, fv.Name)
	}

	customFieldMap := make(map[string]*jira.Field)
	for _, field := range customFields {
		customFieldMap[field.Name] = field
	}

	// Get autocomplete URLs for custom fields
	var customFieldIds []string
	for _, name := range meta {
		if customField, ok := customFieldMap[name]; ok {
			customFieldIds = append(customFieldIds, customField.ID)
		}
	}

	// Fetch autocomplete URLs if we have custom fields
	var autocompleteUrls map[string]string
	if len(customFieldIds) > 0 {
		autocompleteMetadata, err := client.GetEditMetadataWithFields(issueKey, customFieldIds)
		if err == nil && autocompleteMetadata != nil {
			autocompleteUrls = make(map[string]string)
			for fieldId, fieldMeta := range autocompleteMetadata.Fields {
				if fieldMeta.AutoCompleteUrl != "" {
					autocompleteUrls[fieldId] = fieldMeta.AutoCompleteUrl
				}
			}
		}
	}

	for _, name := range meta {
		switch name {
		case "Priority":
			qs = append(qs, &survey.Question{
				Name:   "priority",
				Prompt: &survey.Input{Message: "Priority", Default: issue.Fields.Priority.Name},
			})
		case "Components":
			qs = append(qs, &survey.Question{
				Name: "components",
				Prompt: &survey.Input{
					Message: "Components",
					Help:    "Comma separated list of valid components. For eg: BE,FE",
				},
			})
		case "Labels":
			qs = append(qs, &survey.Question{
				Name: "labels",
				Prompt: &survey.Input{
					Message: "Labels",
					Help:    "Comma separated list of labels. For eg: backend,urgent",
					Default: strings.Join(issue.Fields.Labels, ","),
				},
			})
		case "FixVersions":
			qs = append(qs, &survey.Question{
				Name: "fixversions",
				Prompt: &survey.Input{
					Message: "Fix Versions",
					Help:    "Comma separated list of fixVersions. For eg: v1.0-beta,v2.0",
					Default: strings.Join(fixVersions, ","),
				},
			})
		case "AffectsVersions":
			qs = append(qs, &survey.Question{
				Name: "affectsversions",
				Prompt: &survey.Input{
					Message: "Affects Versions",
					Help:    "Comma separated list of affectsVersions. For eg: v1.0-beta,v2.0",
					Default: strings.Join(affectsVersions, ","),
				},
			})
		default:
			if customField, ok := customFieldMap[name]; ok {
				inputPrompt := &survey.Input{
					Message: customField.Name,
					Help:    "Sorry, no help for custom fields",
					Default: "",
					// Maybe I don't even need it, if I'm not going to send it? :)
					// Default: issue.Fields.CustomFields[customField.ID],
				}

				// Add autocomplete if available
				if autocompleteUrl, hasAutocomplete := autocompleteUrls[customField.ID]; hasAutocomplete {
					inputPrompt.Suggest = func(toComplete string) []string {
						suggestions, err := client.GetAutocompleteSuggestions(autocompleteUrl, toComplete)
						if err != nil {
							// If autocomplete fails, return empty suggestions
							return []string{}
						}
						return suggestions
					}
				}

				qs = append(qs, &survey.Question{
					Name:   customField.ID,
					Prompt: inputPrompt,
				})
			}
		}
	}

	return qs
}

// processBodyForAPI processes the body based on the chosen API version
func processBodyForAPI(body string, useV3API bool, translator *md2adf.Translator) (string, bool) {
	bodyIsRawADF := false

	if body == "" {
		return "", false
	}

	if useV3API {
		// Use ADF for v3 API
		adfBody, convErr := editing.ConvertMarkdownToADF(body, translator)
		if convErr != nil {
			cmdutil.ExitIfError(fmt.Errorf("Failed to convert markdown to adf, this may be a translator bug; %w", convErr))
		} else {
			body = adfBody
			bodyIsRawADF = true
		}
	} else {
		// Use simple markdown for v2 API
		body = md.ToJiraMD(body)
	}

	return body, bodyIsRawADF
}

// processCommentsForAPI processes comments based on the chosen API version
func processCommentsForAPI(comments []editComment, useV3API bool, translator *md2adf.Translator) []jira.EditComment {
	var editComments []jira.EditComment

	for _, comment := range comments {
		body, isRaw := processBodyForAPI(comment.body, useV3API, translator)

		editComments = append(editComments, jira.EditComment{
			ID:           comment.id,
			Body:         body,
			BodyIsRawADF: isRaw,
		})
	}
	return editComments
}

func setFlags(cmd *cobra.Command) {
	custom := make(map[string]string)

	cmd.Flags().SortFlags = false

	cmd.Flags().StringP("parent", "P", "", `Link to a parent key`)
	cmd.Flags().StringP("summary", "s", "", "Edit summary or title")
	cmd.Flags().StringP("body", "b", "", "Edit description")
	cmd.Flags().StringP("priority", "y", "", "Edit priority")
	cmd.Flags().StringP("assignee", "a", "", "Edit assignee (email or display name)")
	cmd.Flags().StringArrayP("label", "l", []string{}, "Append labels")
	cmd.Flags().StringArrayP("component", "C", []string{}, "Replace components")
	cmd.Flags().StringArray("fix-version", []string{}, "Add/Append release info (fixVersions)")
	cmd.Flags().StringArray("affects-version", []string{}, "Add/Append release info (affectsVersions)")
	cmd.Flags().StringToString("custom", custom, "Edit custom fields")
	cmd.Flags().Bool("web", false, "Open in web browser after successful update")
	cmd.Flags().Bool("no-input", false, "Disable prompt for non-required fields")
}

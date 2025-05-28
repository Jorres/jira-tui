package viewBubble

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"github.com/spf13/viper"

	"github.com/ankitpokhrel/jira-cli/internal/cmdutil"
	"github.com/ankitpokhrel/jira-cli/pkg/adf"
	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/md"
	"github.com/ankitpokhrel/jira-cli/pkg/tuiBubble"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ = log.Fatal

const defaultSummaryLength = 73 // +1 to take ellipsis '…' into account.

type fragment struct {
	Body  string
	Parse bool
}

func newBlankFragment(n int) fragment {
	var buf strings.Builder
	for i := 0; i < n; i++ {
		buf.WriteRune('\n')
	}
	return fragment{
		Body:  buf.String(),
		Parse: false,
	}
}

type issueComment struct {
	meta string
	body string
}

// IssueOption is filtering options for an issue.
type IssueOption struct {
	NumComments uint
}

type IssueModel struct {
	Server  string
	Data    *jira.Issue
	Display tuiBubble.DisplayFormat
	Options IssueOption

	ListView *IssueList

	// Original window dimensions
	RawWidth  int
	RawHeight int

	// Calculated viewport dimensions (with margins and borders)
	viewportWidth  int
	viewportHeight int

	marginWidth  int
	marginHeight int

	contentHeight int // Content height (viewport minus border/padding)

	// Scrolling state
	firstVisibleLine int
	renderedLines    []string

	currentlyHighlightedLinkPos       int
	currentlyHighlightedLinkCountdown int

	currentlyHighlightedLinkText string
	currentlyHighlightedLinkURL  string

	uniqueLinkTitleReplacement string
	uniqueLinkTextReplacement  string
	nLinks                     int
}

// RenderedOut translates raw data to the format we want to display in.
func (i *IssueModel) RenderedOut(renderer *glamour.TermRenderer) (string, error) {
	var res strings.Builder

	i.currentlyHighlightedLinkCountdown = i.currentlyHighlightedLinkPos

	for _, p := range i.fragments() {
		if p.Parse {
			out, err := renderer.Render(p.Body)
			if err != nil {
				return "", err
			}
			res.WriteString(out)
		} else {
			res.WriteString(p.Body)
		}
	}

	return res.String(), nil
}

func (i *IssueModel) fragments() []fragment {
	scraps := []fragment{
		{Body: i.header(), Parse: true},
	}

	desc := i.description()
	if desc != "" {
		scraps = append(
			scraps,
			newBlankFragment(1),
			fragment{Body: i.separator("Description")},
			newBlankFragment(2),
			fragment{Body: desc, Parse: true},
		)
	}

	if len(i.Data.Fields.Subtasks) > 0 {
		scraps = append(
			scraps,
			newBlankFragment(1),
			fragment{Body: i.separator(fmt.Sprintf("%d Subtasks", len(i.Data.Fields.Subtasks)))},
			newBlankFragment(2),
			fragment{Body: i.subtasks()},
			newBlankFragment(1),
		)
	}

	if len(i.Data.Fields.IssueLinks) > 0 {
		scraps = append(
			scraps,
			newBlankFragment(1),
			fragment{Body: i.separator("Linked Issues")},
			newBlankFragment(2),
			fragment{Body: i.linkedIssues()},
			newBlankFragment(1),
		)
	}

	if i.Data.Fields.Comment.Total > 0 && i.Options.NumComments > 0 {
		scraps = append(
			scraps,
			newBlankFragment(1),
			fragment{Body: i.separator(fmt.Sprintf("%d Comments", i.Data.Fields.Comment.Total))},
			newBlankFragment(2),
		)
		for _, comment := range i.comments() {
			scraps = append(
				scraps,
				fragment{Body: comment.meta},
				newBlankFragment(1),
				fragment{Body: comment.body, Parse: true},
			)
		}
	}

	return append(scraps, newBlankFragment(1), fragment{Body: i.footer()}, newBlankFragment(2))
}

func (i *IssueModel) separator(msg string) string {
	pad := func(m string) string {
		if m != "" {
			return fmt.Sprintf(" %s ", m)
		}
		return m
	}

	if i.Display.Plain {
		sep := "------------------------"
		return fmt.Sprintf("%s%s%s", sep, pad(msg), sep)
	}
	sep := "————————————————————————"
	if msg == "" {
		return gray(fmt.Sprintf("%s%s", sep, sep))
	}
	return gray(fmt.Sprintf("%s%s%s", sep, pad(msg), sep))
}

func (i *IssueModel) header() string {
	as := i.Data.Fields.Assignee.Name
	if as == "" {
		as = "Unassigned"
	}
	st, sti := i.Data.Fields.Status.Name, "🚧"
	if st == "Done" {
		sti = "✅"
	}
	lbl := "None"
	if len(i.Data.Fields.Labels) > 0 {
		lbl = strings.Join(i.Data.Fields.Labels, ", ")
	}
	components := make([]string, 0, len(i.Data.Fields.Components))
	for _, c := range i.Data.Fields.Components {
		components = append(components, c.Name)
	}
	cmpt := "None"
	if len(components) > 0 {
		cmpt = strings.Join(components, ", ")
	}
	it, iti := i.Data.Fields.IssueType.Name, "⭐"
	if it == "Bug" {
		iti = "🐞"
	}
	wch := fmt.Sprintf("%d watchers", i.Data.Fields.Watches.WatchCount)
	if i.Data.Fields.Watches.WatchCount == 1 && i.Data.Fields.Watches.IsWatching {
		wch = "You are watching"
	} else if i.Data.Fields.Watches.IsWatching {
		wch = fmt.Sprintf("You + %d watchers", i.Data.Fields.Watches.WatchCount-1)
	}
	return fmt.Sprintf(
		"%s %s  %s %s  ⌛ %s  👷 %s  🔑️ %s  💭 %d comments  \U0001F9F5 %d linked\n# %s\n⏱️  %s  🔎 %s  🚀 %s  📦 %s  🏷️  %s  👀 %s",
		iti, it, sti, st, cmdutil.FormatDateTimeHuman(i.Data.Fields.Updated, jira.RFC3339), as, i.Data.Key,
		i.Data.Fields.Comment.Total, len(i.Data.Fields.IssueLinks),
		i.Data.Fields.Summary,
		cmdutil.FormatDateTimeHuman(i.Data.Fields.Created, jira.RFC3339), i.Data.Fields.Reporter.Name,
		i.Data.Fields.Priority.Name, cmpt, lbl, wch,
	)
}

func (i *IssueModel) description() string {
	if i.Data.Fields.Description == nil {
		return ""
	}

	var desc string

	if adfNode, ok := i.Data.Fields.Description.(*adf.ADF); ok {
		desc = adf.NewTranslator(adfNode, adf.NewMarkdownTranslator()).Translate()
	} else {
		desc = i.Data.Fields.Description.(string)
		desc = md.FromJiraMD(desc)
	}

	// Apply view-only link text replacement for better readability
	desc = replaceRedundantLinkText(desc)
	desc = i.colorizeSelected(desc)

	return desc
}

func debug(v ...any) {
	f, _ := os.OpenFile("/home/jorres/hobbies/jira-cli/debug.log", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	for _, val := range v {
		fmt.Fprintln(f, val)
	}
	f.Close()
}

func (i *IssueModel) colorizeSelected(input string) string {
	re := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	matches := re.FindAllStringSubmatchIndex(input, -1)

	var out strings.Builder
	last := 0
	for _, m := range matches {
		fullStart, fullEnd := m[0], m[1]
		textStart, textEnd := m[2], m[3]
		urlStart, urlEnd := m[4], m[5]

		orig := input[fullStart:fullEnd]
		linkText := input[textStart:textEnd]
		linkURL := input[urlStart:urlEnd]

		var newChunk string
		if i.currentlyHighlightedLinkCountdown == 0 {
			replacement := strings.Repeat("X", len(linkText))
			replacementLink := "https://" + strings.Repeat("Y", len(linkURL)-len("https://"))

			i.currentlyHighlightedLinkText = linkText
			i.currentlyHighlightedLinkURL = linkURL
			go func() {
				// can take a while (hundred ms) so I'd like it copied async
				copyToClipboard(linkURL)
			}()
			i.uniqueLinkTitleReplacement = replacement
			i.uniqueLinkTextReplacement = replacementLink

			newChunk = fmt.Sprintf("[%s](%s)", replacement, replacementLink)
		} else {
			newChunk = orig
		}

		i.currentlyHighlightedLinkCountdown--

		out.WriteString(input[last:fullStart])
		out.WriteString(newChunk)
		last = fullEnd
	}

	out.WriteString(input[last:])
	return out.String()
}

// replaceRedundantLinkText replaces link text with "link" when text equals URL (view-only)
// This is only for display purposes and doesn't affect the original content for editing
func replaceRedundantLinkText(text string) string {
	// Match full markdown links where text equals URL
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	return re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the link text and URL
		submatch := re.FindStringSubmatch(match)
		if len(submatch) == 3 {
			linkText := submatch[1]
			linkURL := submatch[2]

			// Check if link text equals URL (duplicate case)
			if strings.TrimSpace(linkText) == strings.TrimSpace(linkURL) {
				// Replace with [link](URL) for cleaner display
				return fmt.Sprintf("[link](%s)", linkURL)
			}
		}
		// Otherwise return the original match
		return match
	})
}

func (i *IssueModel) subtasks() string {
	if len(i.Data.Fields.Subtasks) == 0 {
		return ""
	}

	var (
		subtasks       strings.Builder
		summaryLen     = defaultSummaryLength
		maxKeyLen      int
		maxSummaryLen  int
		maxStatusLen   int
		maxPriorityLen int
	)

	for idx := range i.Data.Fields.Subtasks {
		task := i.Data.Fields.Subtasks[idx]

		maxKeyLen = max(len(task.Key), maxKeyLen)
		maxSummaryLen = max(len(task.Fields.Summary), maxSummaryLen)
		maxStatusLen = max(len(task.Fields.Status.Name), maxStatusLen)
		maxPriorityLen = max(len(task.Fields.Priority.Name), maxPriorityLen)
	}

	if maxSummaryLen < summaryLen {
		summaryLen = maxSummaryLen
	}

	subtasks.WriteString(
		fmt.Sprintf("\n %s\n\n", coloredOut("SUBTASKS", color.FgWhite, color.Bold)),
	)
	for idx := range i.Data.Fields.Subtasks {
		task := i.Data.Fields.Subtasks[idx]
		subtasks.WriteString(
			fmt.Sprintf(
				"  %s %s • %s • %s\n",
				coloredOut(pad(task.Key, maxKeyLen), color.FgGreen, color.Bold),
				shortenAndPad(task.Fields.Summary, summaryLen),
				pad(task.Fields.Priority.Name, maxPriorityLen),
				pad(task.Fields.Status.Name, maxStatusLen),
			),
		)
	}

	return subtasks.String()
}

func (i *IssueModel) linkedIssues() string {
	if len(i.Data.Fields.IssueLinks) == 0 {
		return ""
	}

	var (
		linked         strings.Builder
		keys           = make([]string, 0)
		linkMap        = make(map[string][]*jira.Issue, len(i.Data.Fields.IssueLinks))
		summaryLen     = defaultSummaryLength
		maxKeyLen      int
		maxSummaryLen  int
		maxTypeLen     int
		maxStatusLen   int
		maxPriorityLen int
	)

	for _, link := range i.Data.Fields.IssueLinks {
		var (
			linkType    string
			linkedIssue *jira.Issue
		)

		if link.InwardIssue != nil {
			linkType = link.LinkType.Inward
			linkedIssue = link.InwardIssue
		} else if link.OutwardIssue != nil {
			linkType = link.LinkType.Outward
			linkedIssue = link.OutwardIssue
		}

		if linkedIssue == nil {
			continue
		}

		if _, ok := linkMap[linkType]; !ok {
			keys = append(keys, linkType)
		}
		linkMap[linkType] = append(linkMap[linkType], linkedIssue)

		maxKeyLen = max(len(linkedIssue.Key), maxKeyLen)
		maxSummaryLen = max(len(linkedIssue.Fields.Summary), maxSummaryLen)
		maxTypeLen = max(len(linkedIssue.Fields.IssueType.Name), maxTypeLen)
		maxStatusLen = max(len(linkedIssue.Fields.Status.Name), maxStatusLen)
		maxPriorityLen = max(len(linkedIssue.Fields.Priority.Name), maxPriorityLen)
	}

	if maxSummaryLen < summaryLen {
		summaryLen = maxSummaryLen
	}

	// We are sorting keys to respect the order we see in the UI.
	sort.Strings(keys)

	for _, k := range keys {
		linked.WriteString(
			fmt.Sprintf("\n %s\n\n", coloredOut(strings.ToUpper(k), color.FgWhite, color.Bold)),
		)
		for _, iss := range linkMap[k] {
			linked.WriteString(
				fmt.Sprintf(
					"  %s %s • %s • %s • %s\n",
					coloredOut(pad(iss.Key, maxKeyLen), color.FgGreen, color.Bold),
					shortenAndPad(iss.Fields.Summary, summaryLen),
					pad(iss.Fields.IssueType.Name, maxTypeLen),
					pad(iss.Fields.Priority.Name, maxPriorityLen),
					pad(iss.Fields.Status.Name, maxStatusLen),
				),
			)
		}
	}

	return linked.String()
}

func (i *IssueModel) comments() []issueComment {
	total := i.Data.Fields.Comment.Total
	comments := make([]issueComment, 0, total)

	if total == 0 {
		return comments
	}

	limit := int(i.Options.NumComments)
	if limit > total {
		limit = total
	}

	for idx := total - 1; idx >= total-limit; idx-- {
		c := i.Data.Fields.Comment.Comments[idx]
		var body string
		if adfNode, ok := c.Body.(*adf.ADF); ok {
			body = adf.NewTranslator(adfNode, adf.NewMarkdownTranslator()).Translate()
		} else {
			body = c.Body.(string)
			body = md.FromJiraMD(body)
		}
		// Apply view-only link text replacement for better readability
		body = replaceRedundantLinkText(body)
		body = i.colorizeSelected(body)
		authorName := func() string {
			if c.Author.DisplayName != "" {
				return c.Author.DisplayName
			}
			return c.Author.Name
		}
		meta := fmt.Sprintf(
			"\n %s • %s",
			coloredOut(authorName(), color.FgWhite, color.Bold),
			coloredOut(cmdutil.FormatDateTimeHuman(c.Created, jira.RFC3339), color.FgWhite, color.Bold),
		)
		if idx == total-1 {
			meta += fmt.Sprintf(" • %s", coloredOut("Latest comment", color.FgCyan, color.Bold))
		}
		comments = append(comments, issueComment{
			meta: meta,
			body: body,
		})
	}

	return comments
}

func (i *IssueModel) footer() string {
	var out strings.Builder

	nc := int(i.Options.NumComments)
	if i.Data.Fields.Comment.Total > 0 && nc > 0 && nc < i.Data.Fields.Comment.Total {
		if i.Display.Plain {
			out.WriteString("\n")
		}
		out.WriteString(fmt.Sprintf("%s\n", gray("Use --comments <limit> with `jira issue view` to load more comments")))
	}
	if i.Display.Plain {
		out.WriteString("\n")
	}
	out.WriteString(gray(fmt.Sprintf("View this issue on Jira: %s", cmdutil.GenerateServerBrowseURL(i.Server, i.Data.Key))))

	return out.String()
}

// Init initializes the IssueList model.
func (iss IssueModel) Init() tea.Cmd {
	return nil
}

// Update handles user input and updates the model state.
func (iss IssueModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case *jira.Issue:
		iss.Data = msg
		// Reset scroll when new issue is loaded
		iss.ResetResetables()
	case tuiBubble.WidgetSizeMsg:
		iss.RawWidth = msg.Width
		iss.RawHeight = msg.Height
		iss.calculateViewportDimensions()
		// Reset rendered lines when size changes
		iss.renderedLines = nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return iss.ListView, cmd
		case "ctrl+e":
			iss.scrollDown()
		case "ctrl+y":
			iss.scrollUp()
		case "tab":
			if iss.currentlyHighlightedLinkPos == iss.nLinks-1 {
				// set to "no links selected"
				iss.currentlyHighlightedLinkPos = -1
				// scroll back up all the way
				iss.firstVisibleLine = 0
			} else {
				iss.currentlyHighlightedLinkPos++

				// scroll down until the link is visible
				for {
					iss.prepareRenderedLines()
					out := iss.getVisibleLines()

					if len(iss.uniqueLinkTitleReplacement) > 0 && strings.Contains(out, iss.uniqueLinkTitleReplacement) {
						break
					}

					iss.scrollDown()
				}
			}
		}
	}

	return iss, cmd
}

func (iss *IssueModel) calculateViewportDimensions() {
	// Calculate viewport with 10% margins
	iss.viewportWidth = int(float32(iss.RawWidth) * 0.9)
	// iss.viewportHeight = int(float32(iss.RawHeight) * 0.9)
	iss.viewportHeight = iss.RawHeight - 2
	iss.marginWidth = (iss.RawWidth - iss.viewportWidth) / 2
	iss.marginHeight = (iss.RawHeight - iss.viewportHeight) / 2
	// Available content height (subtract 2 for border)
	iss.contentHeight = iss.viewportHeight - 2
}

// scrollDown scrolls the content down by configured scroll size
func (iss *IssueModel) scrollDown() {
	iss.prepareRenderedLines()

	maxScroll := len(iss.renderedLines) - iss.contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	scrollSize := viper.GetInt("bubble.issue.scroll_size")
	if scrollSize <= 0 {
		scrollSize = 1 // fallback to 1 if not configured or invalid
	}

	// Calculate new scroll position
	newScrollPos := iss.firstVisibleLine + scrollSize
	if newScrollPos > maxScroll {
		newScrollPos = maxScroll
	}

	// Only allow scrolling if it won't go beyond content
	if newScrollPos > iss.firstVisibleLine {
		iss.firstVisibleLine = newScrollPos
	}
}

// scrollUp scrolls the content up by configured scroll size
func (iss *IssueModel) scrollUp() {
	scrollSize := viper.GetInt("bubble.issue.scroll_size")
	if scrollSize <= 0 {
		scrollSize = 1 // fallback to 1 if not configured or invalid
	}

	// Calculate new scroll position
	newScrollPos := iss.firstVisibleLine - scrollSize
	if newScrollPos < 0 {
		newScrollPos = 0
	}

	iss.firstVisibleLine = newScrollPos
}

// prepareRenderedLines renders the full content and splits it into lines
func (iss *IssueModel) prepareRenderedLines() {
	r, err := MDRenderer()
	if err != nil {
		panic(err)
	}
	out, err := iss.RenderedOut(r)
	if err != nil {
		panic(err)
	}

	iss.renderedLines = strings.Split(out, "\n")
}

func NewIssueFromSelected(l *IssueList) IssueModel {
	iss := IssueModel{
		Server:                            l.Server,
		Data:                              l.table.GetSelectedIssueShift(0),
		Options:                           IssueOption{NumComments: 10},
		ListView:                          l,
		currentlyHighlightedLinkPos:       -1,
		currentlyHighlightedLinkCountdown: -1,
	}
	iss.countLinks()
	iss.calculateViewportDimensions()
	return iss
}

func (iss *IssueModel) countLinks() {
	re := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)
	linkCount := 0

	for _, p := range iss.fragments() {
		matches := re.FindAllString(p.Body, -1)
		linkCount += len(matches)
	}

	iss.nLinks = linkCount
}

func (iss *IssueModel) getVisibleLines() string {
	var visibleLines []string
	if len(iss.renderedLines) <= iss.contentHeight {
		visibleLines = iss.renderedLines
	} else {
		startLine := iss.firstVisibleLine
		endLine := startLine + iss.contentHeight
		visibleLines = iss.renderedLines[startLine:endLine]
	}

	return strings.Join(visibleLines, "\n")
}

// View renders the IssueList.
func (iss IssueModel) View() string {
	iss.prepareRenderedLines()

	if iss.contentHeight <= 0 {
		return "Sorry, no issues yet"
	}

	out := iss.getVisibleLines()

	if len(iss.uniqueLinkTitleReplacement) > 0 && strings.Contains(out, iss.uniqueLinkTitleReplacement) {
		coloredText := coloredOut(iss.currentlyHighlightedLinkText, color.BgYellow)
		out = strings.ReplaceAll(out, iss.uniqueLinkTitleReplacement, coloredText)
	}

	if len(iss.uniqueLinkTextReplacement) > 0 && strings.Contains(out, iss.uniqueLinkTextReplacement) {
		coloredText := coloredOut(iss.currentlyHighlightedLinkURL, color.BgYellow)
		out = strings.ReplaceAll(out, iss.uniqueLinkTextReplacement, coloredText)
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Width(iss.viewportWidth).
		Height(iss.viewportHeight).
		Margin(iss.marginHeight, iss.marginWidth).
		Align(lipgloss.Center, lipgloss.Top) // Change alignment to show content from top

	return boxStyle.Render(out)
}

func (iss *IssueModel) ResetResetables() {
	iss.currentlyHighlightedLinkCountdown = -1
	iss.currentlyHighlightedLinkPos = -1
	iss.currentlyHighlightedLinkText = ""
	iss.currentlyHighlightedLinkURL = ""

	iss.firstVisibleLine = 0
	iss.renderedLines = nil
	iss.calculateViewportDimensions()
	iss.countLinks()
}

// currently highlighted link url feature:
// proof of concept works
// 1. you need to correctly loop over, not do %3. Count the number of links beforehand
// 2. scrolling is not done
// - Some nicer coloring and visual indication that link has been copied would be nice.
// - The whole feature feels like fighting against the system, to be honest. Coloring BEFORE calling glamour should work and none of this
// would be necessary.

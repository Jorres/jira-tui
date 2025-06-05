package viewBubble

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/spf13/viper"
)

type HelpView struct {
	RawWidth  int
	RawHeight int

	viewportWidth  int
	viewportHeight int
	marginWidth    int
	marginHeight   int
	contentHeight  int

	// Scrolling state
	firstVisibleLine int
	renderedLines    []string

	PreviousModel tea.Model
}

func NewHelpView(prev tea.Model, width, height int) *HelpView {
	h := &HelpView{
		PreviousModel: prev,
		RawWidth:      width,
		RawHeight:     height,
	}
	h.calculateViewportDimensions()
	h.prepareRenderedLines()
	return h
}

func (h *HelpView) calculateViewportDimensions() {
	// Calculate viewport with 10% margins
	h.viewportWidth = int(float32(h.RawWidth) * 0.8)
	h.viewportHeight = int(float32(h.RawHeight) * 0.8)
	h.marginWidth = (h.RawWidth - h.viewportWidth) / 2
	h.marginHeight = (h.RawHeight - h.viewportHeight) / 2
	// Available content height (subtract 4 for border + padding)
	h.contentHeight = h.viewportHeight - 4
}

func (h *HelpView) Init() tea.Cmd {
	return nil
}

func (h *HelpView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h.RawWidth = msg.Width
		h.RawHeight = msg.Height
		h.calculateViewportDimensions()
		// Reset rendered lines when size changes
		h.renderedLines = nil
		h.prepareRenderedLines()
	case tea.KeyMsg:
		switch msg.String() {
		case "?", "esc", "q", "ctrl+c":
			return h.PreviousModel, func() tea.Msg {
				return tea.WindowSizeMsg{Width: h.RawWidth, Height: h.RawHeight}
			}
		case "ctrl+e", "j", "down":
			h.scrollDown()
		case "ctrl+y", "k", "up":
			h.scrollUp()
		}
	}
	return h, nil
}

// scrollDown scrolls the content down by configured scroll size
func (h *HelpView) scrollDown() {
	h.prepareRenderedLines()

	maxScroll := len(h.renderedLines) - h.contentHeight
	if maxScroll < 0 {
		maxScroll = 0
	}

	scrollSize := viper.GetInt("bubble.issue.scroll_size")
	if scrollSize <= 0 {
		scrollSize = 1 // fallback to 1 if not configured or invalid
	}

	// Calculate new scroll position
	newScrollPos := h.firstVisibleLine + scrollSize
	if newScrollPos > maxScroll {
		newScrollPos = maxScroll
	}

	// Only allow scrolling if it won't go beyond content
	if newScrollPos > h.firstVisibleLine {
		h.firstVisibleLine = newScrollPos
	}
}

// scrollUp scrolls the content up by configured scroll size
func (h *HelpView) scrollUp() {
	scrollSize := viper.GetInt("bubble.issue.scroll_size")
	if scrollSize <= 0 {
		scrollSize = 1 // fallback to 1 if not configured or invalid
	}

	// Calculate new scroll position
	newScrollPos := h.firstVisibleLine - scrollSize
	if newScrollPos < 0 {
		newScrollPos = 0
	}

	h.firstVisibleLine = newScrollPos
}

// prepareRenderedLines renders the full content and splits it into lines
func (h *HelpView) prepareRenderedLines() {
	if h.renderedLines != nil {
		return // Already prepared
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15"))

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		MarginTop(1).
		MarginBottom(0)

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Italic(true).
		MarginTop(1)

	title := titleStyle.Render("🎯 JIRA CLI Help")

	navigation := sectionTitleStyle.Render("Navigation:")
	navItems := []string{
		"  " + keyStyle.Render("j/↓ k/↑") + "           " + descStyle.Render("Move cursor down/up"),
		"  " + keyStyle.Render("CTRL+e/y") + "          " + descStyle.Render("Scroll content up/down"),
		"  " + keyStyle.Render("left/h right/l") + "    " + descStyle.Render("Switch between tabs (if multiple)"),
	}

	issueActions := sectionTitleStyle.Render("Issue Actions:")
	issueItems := []string{
		"  " + keyStyle.Render("enter") + "             " + descStyle.Render("open issue in browser"),
		"  " + keyStyle.Render("n") + "                 " + descStyle.Render("create 'n'ew issue"),
		"  " + keyStyle.Render("e") + "                 " + descStyle.Render("'e'dit current issue"),
		"  " + keyStyle.Render("m") + "                 " + descStyle.Render("'m'ove issue to different status"),
		"  " + keyStyle.Render("c") + "                 " + descStyle.Render("add 'c'omment to issue"),
		"  " + keyStyle.Render("u") + "                 " + descStyle.Render("copy issue 'u'rl to clipboard"),
	}

	assignment := sectionTitleStyle.Render("Assignment:")
	assignItems := []string{
		"  " + keyStyle.Render("a") + "                 " + descStyle.Render("change 'a'ssignee"),
		"  " + keyStyle.Render("CTRL+p") + "            " + descStyle.Render("assign to e'p'ic"),
	}

	other := sectionTitleStyle.Render("Other:")
	otherItems := []string{
		"  " + keyStyle.Render("/") + "                 " + descStyle.Render("Filter/search issues"),
		"  " + keyStyle.Render("CTRL+r") + "            " + descStyle.Render("Refresh current view"),
		"  " + keyStyle.Render("?") + "                 " + descStyle.Render("Toggle this help"),
		"  " + keyStyle.Render("q/ESC/CTRL+c") + "      " + descStyle.Render("Quit"),
	}

	exitTip := footerStyle.Render("Press ? or ESC to return to issues view")

	var content []string
	content = append(content, title)
	content = append(content, exitTip, "")
	content = append(content, navigation)
	content = append(content, navItems...)
	content = append(content, "", issueActions)
	content = append(content, issueItems...)
	content = append(content, "", assignment)
	content = append(content, assignItems...)
	content = append(content, "", other)
	content = append(content, otherItems...)

	helpText := lipgloss.JoinVertical(lipgloss.Left, content...)
	h.renderedLines = strings.Split(helpText, "\n")
}

func (h *HelpView) getVisibleLines() string {
	var visibleLines []string
	if len(h.renderedLines) <= h.contentHeight {
		visibleLines = h.renderedLines
	} else {
		startLine := h.firstVisibleLine
		endLine := startLine + h.contentHeight
		if endLine > len(h.renderedLines) {
			endLine = len(h.renderedLines)
		}
		visibleLines = h.renderedLines[startLine:endLine]
	}

	return strings.Join(visibleLines, "\n")
}

// generateScrollbar creates a vertical scrollbar representation using the scrollbar module
func (h *HelpView) generateScrollbar() (string, bool) {
	config := DefaultScrollbarConfig(h.contentHeight)
	return GenerateScrollbar(len(h.renderedLines), h.contentHeight, h.firstVisibleLine, config)
}

func (h *HelpView) View() string {
	h.prepareRenderedLines()

	if h.contentHeight <= 0 {
		return "Help view too small"
	}

	out := h.getVisibleLines()

	// Generate scrollbar
	scrollbar, needsScrollbar := h.generateScrollbar()

	// Create content with scrollbar if needed
	var contentWithScrollbar string
	if needsScrollbar {
		// Calculate available width for content (subtract scrollbar width and padding)
		contentWidth := h.viewportWidth - 6 - 1 // 6 for padding (3 each side), 1 for scrollbar
		paddedContent := lipgloss.NewStyle().Width(contentWidth).Render(out)

		contentWithScrollbar = lipgloss.JoinHorizontal(
			lipgloss.Top,
			paddedContent,
			scrollbar,
		)
	} else {
		contentWithScrollbar = out
	}

	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(getAccentColor())).
		Padding(2, 3).
		Width(h.viewportWidth).
		Height(h.viewportHeight)

	return lipgloss.Place(
		h.RawWidth,
		h.RawHeight,
		lipgloss.Center,
		lipgloss.Center,
		helpStyle.Render(contentWithScrollbar),
	)
}

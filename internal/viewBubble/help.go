package viewBubble

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HelpView struct {
	RawWidth  int
	RawHeight int

	viewportWidth  int
	viewportHeight int

	PreviousModel tea.Model
}

func NewHelpView(prev tea.Model, width, height int) *HelpView {
	h := &HelpView{
		PreviousModel: prev,
		RawWidth:      width,
		RawHeight:     height,
	}
	h.calculateViewportDimensions()
	return h
}

func (h *HelpView) calculateViewportDimensions() {
	h.viewportWidth = int(float32(h.RawWidth) * 0.8)
	h.viewportHeight = int(float32(h.RawHeight) * 0.8)
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
	case tea.KeyMsg:
		switch msg.String() {
		case "?", "esc", "q", "ctrl+c":
			return h.PreviousModel, nil
		}
	}
	return h, nil
}

func (h *HelpView) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")).
		MarginBottom(1)

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
		"  " + keyStyle.Render("CTRL+e/y") + "          " + descStyle.Render("Scroll issue preview up/down"),
		"  " + keyStyle.Render("left/h right/l") + "    " + descStyle.Render("Switch between tabs (if multiple)"),
	}

	issueActions := sectionTitleStyle.Render("Issue Actions:")
	issueItems := []string{
		"  " + keyStyle.Render("enter") + "             " + descStyle.Render("Open issue in browser"),
		"  " + keyStyle.Render("n") + "                 " + descStyle.Render("Create new issue"),
		"  " + keyStyle.Render("e") + "                 " + descStyle.Render("Edit current issue"),
		"  " + keyStyle.Render("m") + "                 " + descStyle.Render("Move issue to different status"),
		"  " + keyStyle.Render("c") + "                 " + descStyle.Render("Add comment to issue"),
		"  " + keyStyle.Render("u") + "                 " + descStyle.Render("Copy issue URL to clipboard"),
	}

	assignment := sectionTitleStyle.Render("Assignment:")
	assignItems := []string{
		"  " + keyStyle.Render("a") + "                 " + descStyle.Render("Change assignee"),
		"  " + keyStyle.Render("CTRL+p") + "            " + descStyle.Render("Assign to epic"),
	}

	other := sectionTitleStyle.Render("Other:")
	otherItems := []string{
		"  " + keyStyle.Render("/") + "                 " + descStyle.Render("Filter/search issues"),
		"  " + keyStyle.Render("CTRL+r") + "            " + descStyle.Render("Refresh current view"),
		"  " + keyStyle.Render("?") + "                 " + descStyle.Render("Toggle this help"),
		"  " + keyStyle.Render("q/ESC/CTRL+c") + "      " + descStyle.Render("Quit"),
	}

	footer := footerStyle.Render("Press ? or ESC to return to issues view")

	var content []string
	content = append(content, title, "")
	content = append(content, navigation)
	content = append(content, navItems...)
	content = append(content, "", issueActions)
	content = append(content, issueItems...)
	content = append(content, "", assignment)
	content = append(content, assignItems...)
	content = append(content, "", other)
	content = append(content, otherItems...)
	content = append(content, "", footer)

	helpText := lipgloss.JoinVertical(lipgloss.Left, content...)

	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(2, 3).
		Width(h.viewportWidth).
		Height(h.viewportHeight)

	return lipgloss.Place(
		h.RawWidth,
		h.RawHeight,
		lipgloss.Center,
		lipgloss.Center,
		helpStyle.Render(helpText),
	)
}


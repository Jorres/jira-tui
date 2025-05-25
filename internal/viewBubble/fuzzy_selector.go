package viewBubble

import (
	"log"

	"github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/ankitpokhrel/jira-cli/pkg/tuiBubble"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var _ = log.Fatal

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type FuzzySelector struct {
	list      list.Model
	RawWidth  int
	RawHeight int

	viewportWidth  int
	viewportHeight int

	marginWidth   int
	marginHeight  int
	contentHeight int

	PreviousModel tea.Model
}

func (m FuzzySelector) Init() tea.Cmd {
	return nil
}

func (m *FuzzySelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tuiBubble.WidgetSizeMsg:
		m.RawWidth = msg.Width
		m.RawHeight = msg.Height
		m.calculateViewportDimensions()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m.PreviousModel, cmd
		case "enter":
			// if we are currently filtering, first "enter" should apply
			// filtering to the underlying list model and only subsequent "enter"
			// should return selected issue to previous view
			if m.list.FilterState() != list.Filtering {
				item := (m.list.Items()[m.list.Cursor()]).(*jira.Issue)
				return m.PreviousModel, func() tea.Msg {
					return fuzzySelectorResult{item: item}
				}
			}
		}
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *FuzzySelector) calculateViewportDimensions() {
	// Calculate viewport with 10% margins
	m.viewportWidth = int(float32(m.RawWidth) * 0.9)
	m.viewportHeight = m.RawHeight - 2
	m.marginWidth = (m.RawWidth - m.viewportWidth) / 2
	m.marginHeight = (m.RawHeight - m.viewportHeight) / 2
	// Available content height (subtract 2 for border)
	m.contentHeight = m.viewportHeight - 2
	m.list.SetSize(m.viewportWidth, m.viewportHeight)
}

func NewFuzzySelectorFrom(prev tea.Model, width, height int, items []list.Item) *FuzzySelector {
	fz := &FuzzySelector{
		PreviousModel: prev,
		RawWidth:      width,
		RawHeight:     height,
		list:          list.New(items, list.NewDefaultDelegate(), 0, 0),
	}

	fz.list.Title = "Select an epic to assign to:"
	fz.calculateViewportDimensions()

	return fz
}

func (m *FuzzySelector) View() string {
	return docStyle.Render(m.list.View())
}

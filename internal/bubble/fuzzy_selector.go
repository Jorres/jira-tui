package bubble

import (
	"log"

	"github.com/charmbracelet/bubbles/v2/list"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

var _ = log.Fatal

var docStyle = lipgloss.NewStyle().Margin(1, 2)

type FuzzySelectorType int

const (
	FuzzySelectorEpic FuzzySelectorType = iota
	FuzzySelectorUser
)

type FuzzySelector struct {
	list      list.Model
	RawWidth  int
	RawHeight int

	viewportWidth  int
	viewportHeight int

	marginWidth   int
	marginHeight  int
	contentHeight int
	selectorType  FuzzySelectorType

	PreviousModel tea.Model
}

func (m FuzzySelector) Init() tea.Cmd {
	return nil
}

func (m *FuzzySelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case WidgetSizeMsg:
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
				return m.PreviousModel, func() tea.Msg {
					return FuzzySelectorResultMsg{
						item:         m.list.SelectedItem(),
						selectorType: m.selectorType,
					}
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

func NewFuzzySelectorFrom(prev tea.Model, width, height int, items []list.Item, fuzzySelectorType FuzzySelectorType) *FuzzySelector {
	// Create a themed delegate with accent color
	delegate := list.NewDefaultDelegate()

	// Apply accent color theming to selected items
	accentColor := lipgloss.Color(getAccentColor())

	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(accentColor).BorderForeground(accentColor)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(accentColor).BorderForeground(accentColor)

	fz := &FuzzySelector{
		PreviousModel: prev,
		RawWidth:      width,
		RawHeight:     height,

		list:         list.New(items, delegate, 0, 0),
		selectorType: fuzzySelectorType,
	}

	fz.list.Styles.Title = fz.list.Styles.Title.Background(accentColor)

	switch fuzzySelectorType {
	case FuzzySelectorEpic:
		fz.list.Title = "Select an epic to assign to:"
	case FuzzySelectorUser:
		fz.list.Title = "Assign this issue to:"
	}
	fz.calculateViewportDimensions()

	return fz
}

func (m *FuzzySelector) View() string {
	return docStyle.Render(m.list.View())
}

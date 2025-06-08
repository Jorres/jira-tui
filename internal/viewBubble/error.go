package viewBubble

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/lipgloss/v2"
)

// ErrorModel displays error messages with stderr output in a bordered modal window
type ErrorModel struct {
	errorMessage string
	stderrOutput string
	width        int
	height       int
	parentModel  tea.Model
}

// NewErrorModel creates a new error model
func NewErrorModel(parentModel tea.Model, errorMessage, stderrOutput string, width, height int) ErrorModel {
	return ErrorModel{
		errorMessage: errorMessage,
		stderrOutput: stderrOutput,
		width:        width,
		height:       height,
		parentModel:  parentModel,
	}
}

// Init initializes the error model
func (m ErrorModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the error model
func (m ErrorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "esc", "q":
			return m.parentModel, func() tea.Msg {
				return tea.WindowSizeMsg{
					Width:  m.width,
					Height: m.height,
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View renders the error modal
func (m ErrorModel) View() string {
	// Modal dimensions - make it smaller than full screen
	modalWidth := min(100, m.width-4)
	modalHeight := min(20, m.height-4)

	// Content area inside the border
	contentWidth := modalWidth - 4 // Account for border and padding

	// Prepare content
	var content strings.Builder
	content.WriteString("Error occurred:\n")
	content.WriteString(m.errorMessage)
	content.WriteString("\n\n")

	if m.stderrOutput != "" {
		content.WriteString("Command output:\n")
		content.WriteString(wrapText(m.stderrOutput, contentWidth-4)) // -4? I forgot why it is here
	}

	content.WriteString("\n\nPress Enter, Esc, or 'q' to close")

	// Style the modal content
	contentStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Padding(1, 2).
		Foreground(lipgloss.Color("15"))

	// Style the modal border
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")). // Red border for error
		Width(modalWidth).
		Height(modalHeight).
		Align(lipgloss.Center, lipgloss.Center).
		Background(lipgloss.Color("235")) // Dark background

	modalContent := contentStyle.Render(content.String())
	modal := modalStyle.Render(modalContent)

	// Center the modal on screen
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modal,
	)
}

// wrapText wraps text to fit within specified width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return text
	}

	var result strings.Builder
	lineLength := 0

	for i, word := range words {
		if i > 0 {
			if lineLength+len(word)+1 > width {
				result.WriteString("\n")
				lineLength = 0
			} else {
				result.WriteString(" ")
				lineLength++
			}
		}

		result.WriteString(word)
		lineLength += len(word)
	}

	return result.String()
}

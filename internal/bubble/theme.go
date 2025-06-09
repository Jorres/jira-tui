package bubble

import (
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/spf13/viper"
)

var (
	currentTheme          string
	globalBackgroundColor string
)

// getAccentColor returns the configured accent color or default fallback
func getAccentColor() string {
	color := viper.GetString("ui.theme.accent")
	if color != "" {
		return color
	}

	if currentTheme == "dark" {
		return "62"
	} else {
		return "62"
	}
}

func getPaleColor() string {
	color := viper.GetString("ui.theme.pale")
	if color != "" {
		return color
	}

	if currentTheme == "dark" {
		return "240"
	} else {
		return "#bbbbbb"
	}
}

// getHighlightColor returns a lipgloss color for highlighting
func getHighlightColor() string {
	return getAccentColor()
}

func setGlobalRenderingStyle(backgroundColor string) string {
	globalBackgroundColor = backgroundColor
	color, _ := colorful.Hex(backgroundColor)
	_, _, lum := color.Hsl()

	if lum < 0.5 {
		currentTheme = "dark"
	} else {
		currentTheme = "light"
	}

	return currentTheme
}

func getCurrentTheme() string {
	return currentTheme
}

type DetectColorModel struct{}

func (m DetectColorModel) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

func (m DetectColorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		setGlobalRenderingStyle(msg.String())
		return m, tea.Quit
	}

	return m, nil
}

func (m DetectColorModel) View() string {
	return ""
}

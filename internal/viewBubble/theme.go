package viewBubble

import (
	"github.com/spf13/viper"
)

// getAccentColor returns the configured accent color or default fallback
func getAccentColor() string {
	color := viper.GetString("bubble.theme.accent")
	if color == "" {
		return "62"
	}
	return color
}

func getPaleColor() string {
	color := viper.GetString("bubble.theme.pale")
	if color == "" {
		return "240"
	}
	return color
}

// getHighlightColor returns a lipgloss color for highlighting
func getHighlightColor() string {
	return getAccentColor()
}


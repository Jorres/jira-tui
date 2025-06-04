package viewBubble

import (
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
)

// ScrollbarConfig holds configuration for scrollbar appearance
type ScrollbarConfig struct {
	Height            int    // Height of the scrollbar (viewport height)
	ThumbColor        string // Color for the visible portion (thumb)
	TrackColor        string // Color for the non-visible portion (track)
	ShowWhenNotNeeded bool   // Show empty space when scrollbar not needed
}

// GenerateScrollbar creates a vertical scrollbar representation
// Parameters:
//
//	totalLines: Total number of lines in the content
//	viewportHeight: Number of lines visible at once
//	firstVisibleLine: Index of the first visible line (0-based)
//	config: Scrollbar appearance configuration
//
// Returns:
//
//	scrollbar: Rendered scrollbar string
//	needed: Whether scrollbar is actually needed
func GenerateScrollbar(totalLines, viewportHeight, firstVisibleLine int, config ScrollbarConfig) (string, bool) {
	needsScrollbar := totalLines > viewportHeight

	var scrollbar strings.Builder
	for i := 0; i < config.Height; i++ {
		if needsScrollbar {
			// Calculate the proportion of content that is visible
			visibleProportion := float64(viewportHeight) / float64(totalLines)

			// Calculate the size of the bright section (thumb)
			thumbSize := int(visibleProportion * float64(config.Height))
			if thumbSize < 1 {
				thumbSize = 1
			}

			// Calculate the position of the thumb
			scrollProgress := float64(firstVisibleLine) / float64(totalLines-viewportHeight)
			thumbPosition := int(scrollProgress * float64(config.Height-thumbSize))

			if i >= thumbPosition && i < thumbPosition+thumbSize {
				scrollbar.WriteString("█") // Bright block for visible portion
			} else {
				scrollbar.WriteString("▓") // Dim block for non-visible portion
			}
		} else if config.ShowWhenNotNeeded {
			scrollbar.WriteString(" ") // Empty space when no scrollbar needed
		}

		if i < config.Height-1 {
			scrollbar.WriteString("\n")
		}
	}

	// Apply colors if scrollbar is needed
	if needsScrollbar {
		scrollbarContent := scrollbar.String()

		// Create styles for thumb and track
		thumbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(config.ThumbColor))
		trackStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(config.TrackColor))

		// Replace characters with colored versions
		scrollbarContent = strings.ReplaceAll(scrollbarContent, "█", thumbStyle.Render("█"))
		scrollbarContent = strings.ReplaceAll(scrollbarContent, "▓", trackStyle.Render("▓"))

		return scrollbarContent, true
	}

	return scrollbar.String(), false
}

// DefaultScrollbarConfig returns a default configuration for scrollbars
func DefaultScrollbarConfig(height int) ScrollbarConfig {
	return ScrollbarConfig{
		Height:            height,
		ThumbColor:        "62",  // Gray for thumb
		TrackColor:        "240", // Gray for track
		ShowWhenNotNeeded: true,
	}
}

package viewBubble

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"github.com/mgutz/ansi"
)

const (
	wordWrap = 120
	tabWidth = 8
	helpText = `USAGE
	-----
	
	The layout contains 2 sections, viz: Sidebar and Contents screen.  
	
	You can use up and down arrow keys or 'j' and 'k' letters to navigate through the sidebar.
	Press 'w' or Tab to toggle focus between the sidebar and the contents screen.
	
	On contents screen:
	  - Use arrow keys or 'j', 'k', 'h', and 'l' letters to navigate through the issue list.
	  - Use 'g' and 'SHIFT+G' to quickly navigate to the top and bottom respectively.
	  - Press 'v' to view selected issue details.
	  - Press 'c' to copy issue URL to the system clipboard.
	  - Press 'CTRL+K' to copy issue key to the system clipboard.
	  - Hit ENTER to open the selected issue in a browser.
	
	Press 'q' / ESC / CTRL+C to quit.`
)

// MDRenderer constructs markdown renderer.
func MDRenderer() (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(wordWrap),
	)
}

func unescape(s string) string {
	pattern := regexp.MustCompile(`(\[[a-zA-Z0-9_,;: \-\."#]+\[*)\[\]`)
	return pattern.ReplaceAllString(s, "$1]")
}

func coloredOut(msg string, clr color.Attribute, attrs ...color.Attribute) string {
	c := color.New(clr).Add(attrs...)
	return c.Sprint(msg)
}

func xterm256() bool {
	term := os.Getenv("TERM")
	return strings.Contains(term, "-256color")
}

func gray(msg string) string {
	if xterm256() {
		return gray256(msg)
	}
	return ansi.ColorFunc("black+h")(msg)
}

func gray256(msg string) string {
	return fmt.Sprintf("\x1b[38;5;242m%s\x1b[m", msg)
}

func shortenAndPad(msg string, limit int) string {
	if limit > 1 && len(msg) > limit {
		return msg[0:limit-1] + "…"
	}
	return pad(msg, limit)
}

func pad(msg string, limit int) string {
	var out strings.Builder
	out.WriteString(msg)
	for i := len(msg); i < limit; i++ {
		out.WriteRune(' ')
	}
	return out.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

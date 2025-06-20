package bubble

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"github.com/mgutz/ansi"
)

func FormatDateTime(dt, format, tz string) string {
	t, err := time.Parse(format, dt)
	if err != nil {
		return dt
	}
	if tz == "" {
		return t.Format("2006-01-02 15:04")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return dt
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

func prepareTitle(text string) string {
	text = strings.TrimSpace(text)
	return text
}

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
func MDRenderer(lightOrDark string) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle(lightOrDark),
		glamour.WithWordWrap(wordWrap),
	)
}

// MDRendererWithWidth constructs markdown renderer with custom width.
func MDRendererWithWidth(lightOrDark string, width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStandardStyle(lightOrDark),
		glamour.WithWordWrap(width),
	)
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

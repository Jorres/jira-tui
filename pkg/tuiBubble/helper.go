package tuiBubble

import (
	"strings"
	"time"
)

// ValidIssueColumns returns valid columns for issue list.
func ValidIssueColumns() []string {
	return []string{
		FieldType,
		FieldKey,
		FieldSummary,
		FieldStatus,
		FieldAssignee,
		FieldReporter,
		FieldPriority,
		FieldResolution,
		FieldCreated,
		FieldUpdated,
		FieldLabels,
	}
}

func FormatDateTime(dt, format, tz string) string {
	t, err := time.Parse(format, dt)
	if err != nil {
		return dt
	}
	if tz == "" {
		return t.Format("2006-01-02 15:04:05")
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return dt
	}
	return t.In(loc).Format("2006-01-02 15:04:05")
}

func prepareTitle(text string) string {
	text = strings.TrimSpace(text)
	return text
}

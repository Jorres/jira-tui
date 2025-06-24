package bubble

const (
	FieldType       = "TYPE"
	FieldParent     = "PARENT"
	FieldKey        = "KEY"
	FieldSummary    = "SUMMARY"
	FieldStatus     = "STATUS"
	FieldAssignee   = "ASSIGNEE"
	FieldReporter   = "REPORTER"
	FieldPriority   = "PRIORITY"
	FieldResolution = "RESOLUTION"
	FieldCreated    = "CREATED"
	FieldUpdated    = "UPDATED"
	FieldLabels     = "LABELS"
	FieldIsOnBoard  = "IS ON BOARD"
)

// ValidIssueColumns returns the list of valid column names for help text
func ValidIssueColumns() []string {
	return []string{
		FieldType, FieldParent, FieldKey, FieldSummary, FieldStatus,
		FieldAssignee, FieldReporter, FieldPriority, FieldResolution,
		FieldCreated, FieldUpdated, FieldLabels, FieldIsOnBoard,
	}
}

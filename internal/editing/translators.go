package editing

import (
	"fmt"

	"github.com/jorres/jira-tui/pkg/jira"
	"github.com/jorres/md2adf-translator/adf2md"
	"github.com/jorres/md2adf-translator/md2adf"
)

func PrepareMD2AdfTranslator(body string, client *jira.Client, issueKey string, reverseTranslator *adf2md.Translator) (*md2adf.Translator, error) {
	var userMapping map[string]string

	emails := extractEmailsFromMarkdown(body)
	if len(emails) != 0 {
		var err error
		userMapping, err = resolveUserIDs(emails, client, issueKey)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve user IDs: %w", err)
		}
		// If no users were resolved, fall back to standard conversion
		if len(userMapping) == 0 {
			return nil, fmt.Errorf("no users resolved from mentions")
		}
	}

	opts := []md2adf.TranslatorOption{}

	opts = append(opts, md2adf.WithUserEmailMapping(userMapping))
	if reverseTranslator != nil {
		opts = append(opts, md2adf.WithAdf2MdTranslator(reverseTranslator))
	}

	// Convert markdown to ADF using the translator
	return md2adf.NewTranslator(opts...), nil
}

// ConvertMarkdownToADF converts markdown to ADF JSON string if mentions are found
func ConvertMarkdownToADF(body string, translator *md2adf.Translator) (string, error) {
	adfDoc, err := translator.TranslateToADF([]byte(body))
	if err != nil {
		return "", err
	}

	// Convert ADF document to JSON string
	jsonBytes, err := adfDoc.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal ADF to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

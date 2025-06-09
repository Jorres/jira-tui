package editing

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jorres/jira-tui/api"
	"github.com/jorres/jira-tui/pkg/jira"
	"github.com/spf13/viper"
)

// extractEmailsFromMarkdown extracts all @email patterns from markdown text
func extractEmailsFromMarkdown(markdown string) []string {
	// Pattern matches @word@domain.tld
	emailPattern := regexp.MustCompile(`@[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	matches := emailPattern.FindAllString(markdown, -1)

	// Remove duplicates
	emailSet := make(map[string]bool)
	var emails []string
	for _, match := range matches {
		if !emailSet[match] {
			emailSet[match] = true
			emails = append(emails, match)
		}
	}

	return emails
}

// resolveUserIDs takes a list of @emails and returns a mapping of email -> userID
func resolveUserIDs(emails []string, client *jira.Client, issueKey string) (map[string]string, error) {
	userMapping := make(map[string]string)

	users, err := client.GetAssignableToIssue(issueKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get all issues for email matching: %w", err)
	}

	userMap := make(map[string]*jira.User)
	for _, user := range users {
		userMap[user.Email] = user
	}

	for _, email := range emails {
		// Remove @ prefix for user search
		cleanEmail := strings.TrimPrefix(email, "@")

		if user, exists := userMap[cleanEmail]; exists {
			// Use AccountID for cloud installations, Name for server
			it := viper.GetString("installation")
			var userID string
			if it == jira.InstallationTypeLocal {
				userID = user.Name
			} else {
				userID = user.AccountID
			}
			userMapping[email] = userID
			fmt.Fprintf(os.Stderr, "Info: Resolved %s to user ID %s\n", email, userID)
		} else {
			fmt.Fprintf(os.Stderr, "Error: failed to create mention from email, user not found: %s\n", cleanEmail)
		}
	}

	return userMapping, nil
}

// ResolveUserIDToEmail resolves a Jira user ID to their email address
func ResolveUserIDToEmail(userID string, client *jira.Client, project string) string {
	// Try to get user info by ID
	user, err := api.ProxyUserGet(client, &jira.UserGetOptions{
		AccountID: userID,
	})

	if err != nil {
		return ""
	}

	// Check if we have an email field
	if user.Email != "" {
		return user.Email
	}
	// Some installations might use different field names
	if user.Name != "" && strings.Contains(user.Name, "@") {
		return user.Name
	}

	return ""
}

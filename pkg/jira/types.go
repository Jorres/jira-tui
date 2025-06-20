package jira

import (
	"encoding/json"
	"fmt"
)

const (
	// AuthTypeBasic is a basic auth.
	AuthTypeBasic AuthType = "basic"
	// AuthTypeBearer is a bearer auth.
	AuthTypeBearer AuthType = "bearer"
	// AuthTypeMTLS is a mTLS auth.
	AuthTypeMTLS AuthType = "mtls"
)

// AuthType is a jira authentication type.
// Currently supports basic and bearer (PAT).
// Defaults to basic for empty or invalid value.
type AuthType string

// String implements stringer interface.
func (at AuthType) String() string {
	if at == "" {
		return string(AuthTypeBasic)
	}
	return string(at)
}

// Project holds project info.
type Project struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Lead struct {
		Name string `json:"displayName"`
	} `json:"lead"`
	Type string `json:"style"`
}

// Board holds board info.
type Board struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Epic holds epic info.
type Epic struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

// Issue holds issue info.
type Issue struct {
	Key    string      `json:"key"`
	Fields IssueFields `json:"fields"`
}

// This allows for `Issue` type to be passed to FuzzySelector
func (i Issue) FilterValue() string { return fmt.Sprintf("%s %s", i.Key, i.Fields.Summary) }
func (i Issue) Description() string { return i.Fields.Summary }
func (i Issue) Title() string       { return i.Key }

type Comments []struct {
	ID      string      `json:"id"`
	Author  User        `json:"author"`
	Body    interface{} `json:"body"` // string in v1/v2, adf.ADF in v3
	Created string      `json:"created"`
}

// IssueFields holds issue fields.
type IssueFields struct {
	Summary     string      `json:"summary"`
	Description interface{} `json:"description"` // string in v1/v2, adf.ADF in v3
	Labels      []string    `json:"labels"`
	Resolution  struct {
		Name string `json:"name"`
	} `json:"resolution"`
	IssueType IssueType `json:"issueType"`
	Parent    *struct {
		Key string `json:"key"`
	} `json:"parent,omitempty"`
	Assignee struct {
		Name string `json:"displayName"`
	} `json:"assignee"`
	Priority struct {
		Name string `json:"name"`
	} `json:"priority"`
	Reporter struct {
		Name string `json:"displayName"`
	} `json:"reporter"`
	Watches struct {
		IsWatching bool `json:"isWatching"`
		WatchCount int  `json:"watchCount"`
	} `json:"watches"`
	Status struct {
		Name string `json:"name"`
	} `json:"status"`
	Components []struct {
		Name string `json:"name"`
	} `json:"components"`
	FixVersions []struct {
		Name string `json:"name"`
	} `json:"fixVersions"`
	AffectsVersions []struct {
		Name string `json:"name"`
	} `json:"versions"`
	Comment struct {
		Comments Comments `json:"comments"`
		Total    int      `json:"total"`
	} `json:"comment"`
	Subtasks   []Issue
	IssueLinks []struct {
		ID       string `json:"id"`
		LinkType struct {
			Name    string `json:"name"`
			Inward  string `json:"inward"`
			Outward string `json:"outward"`
		} `json:"type"`
		InwardIssue  *Issue `json:"inwardIssue,omitempty"`
		OutwardIssue *Issue `json:"outwardIssue,omitempty"`
	} `json:"issueLinks"`
	Created      string            `json:"created"`
	Updated      string            `json:"updated"`
	CustomFields map[string]string `json:"-"`
}

// Field holds field info.
type Field struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
	Schema struct {
		DataType string `json:"type"`
		Items    string `json:"items,omitempty"`
		FieldID  int    `json:"customId,omitempty"`
	} `json:"schema"`
}

// IssueTypeField holds issue field info.
type IssueTypeField struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	Schema struct {
		DataType string `json:"type"`
		Items    string `json:"items,omitempty"`
	} `json:"schema"`
	FieldID string `json:"fieldId,omitempty"`
}

// IssueType holds issue type info.
type IssueType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Handle  string `json:"untranslatedName,omitempty"` // This field may not exist in older version of the API.
	Subtask bool   `json:"subtask"`
}

// IssueLinkType holds issue link type info.
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// Sprint holds sprint info.
type Sprint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"state"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	CompleteDate string `json:"completeDate,omitempty"`
	BoardID      int    `json:"originBoardId,omitempty"`
}

// Transition holds issue transition info.
type Transition struct {
	ID          json.Number `json:"id"`
	Name        string      `json:"name"`
	IsAvailable bool        `json:"isAvailable"`
}

// This allows for `User` type to be passed to FuzzySelector
func (u User) FilterValue() string {
	return fmt.Sprintf("%s %s", u.GetDisplayableName(), u.Email)
}

func (u User) Description() string { return u.Email }
func (u User) Title() string       { return u.GetDisplayableName() }

// User holds user info.
type User struct {
	AccountID   string `json:"accountId,omitempty"`
	Email       string `json:"emailAddress"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName"`
	Active      bool   `json:"active"`
}

// EditMetadata holds edit metadata response from Jira API.
type EditMetadata struct {
	Fields map[string]FieldMetadata `json:"fields"`
}

// FieldMetadata holds metadata about a field that can be edited.
type FieldMetadata struct {
	Key             string        `json:"key"`
	Name            string        `json:"name"`
	Operations      []string      `json:"operations"`
	Required        bool          `json:"required"`
	Schema          FieldSchema   `json:"schema"`
	AllowedValues   []interface{} `json:"allowedValues"`
	AutoCompleteUrl string        `json:"autoCompleteUrl,omitempty"`
	Configuration   interface{}   `json:"configuration,omitempty"`
	DefaultValue    interface{}   `json:"defaultValue,omitempty"`
	HasDefaultValue bool          `json:"hasDefaultValue,omitempty"`
}

// FieldSchema holds schema information for a field.
type FieldSchema struct {
	Type          string      `json:"type"`
	Custom        string      `json:"custom,omitempty"`
	CustomId      int         `json:"customId,omitempty"`
	Items         string      `json:"items,omitempty"`
	System        string      `json:"system,omitempty"`
	Configuration interface{} `json:"configuration,omitempty"`
}

// AutocompleteSuggestion holds a single autocomplete suggestion.
type AutocompleteSuggestion struct {
	Label string `json:"label"`
	HTML  string `json:"html"`
}

// AutocompleteResponse holds the response from autocomplete API.
type AutocompleteResponse struct {
	Token       string                   `json:"token"`
	Suggestions []AutocompleteSuggestion `json:"suggestions"`
}

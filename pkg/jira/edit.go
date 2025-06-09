package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/jorres/md2adf-translator/md2adf"

	"github.com/jorres/jira-tui/internal/debug"
)

var _ = debug.Debug

const separatorMinus = "-"

// EditResponse struct holds response from POST /issue endpoint.
type EditResponse struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// EditComment holds comment data for editing
type EditComment struct {
	ID   string
	Body string
	// BodyIsRawADF indicates that Body contains raw ADF JSON that should be embedded directly
	BodyIsRawADF bool
}

// EditRequest struct holds request data for edit request.
// Setting an Assignee requires an account ID.
type EditRequest struct {
	IssueType      string
	ParentIssueKey string
	Summary        string
	Body           string
	// BodyIsRawADF indicates that Body contains raw ADF JSON that should be embedded directly
	BodyIsRawADF    bool
	Comments        []EditComment
	Priority        string
	Labels          []string
	Components      []string
	FixVersions     []string
	AffectsVersions []string
	// CustomFields holds all custom fields passed
	// while editing the issue.
	CustomFields map[string]string

	configuredCustomFields []IssueTypeField
}

// WithCustomFields sets valid custom fields for the issue.
func (er *EditRequest) WithCustomFields(cf []IssueTypeField) {
	er.configuredCustomFields = cf
}

// Edit updates an issue using PUT /issue endpoint (v3 API).
func (c *Client) Edit(key string, req *EditRequest) error {
	data := getRequestDataForEdit(req)
	if data == nil {
		return fmt.Errorf("jira: invalid request - failed to parse ADF JSON")
	}

	body, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	res, err := c.Put(context.Background(), "/issue/"+key, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return formatUnexpectedResponse(res)
	}

	// Update comments separately
	for _, comment := range req.Comments {
		err := c.updateComment(key, comment)
		if err != nil {
			return fmt.Errorf("failed to update comment %s: %w", comment.ID, err)
		}
	}

	return nil
}

// EditV2 updates an issue using PUT /issue endpoint (v2 API).
func (c *Client) EditV2(key string, req *EditRequest) error {
	data := getRequestDataForEdit(req)
	if data == nil {
		return fmt.Errorf("jira: invalid request - failed to parse ADF JSON")
	}

	body, err := json.Marshal(&data)
	if err != nil {
		return err
	}

	res, err := c.PutV2(context.Background(), "/issue/"+key, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusNoContent {
		return formatUnexpectedResponse(res)
	}

	// Update comments separately
	for _, comment := range req.Comments {
		err := c.updateCommentV2(key, comment)
		if err != nil {
			return fmt.Errorf("failed to update comment %s: %w", comment.ID, err)
		}
	}

	return nil
}

func V3ContentToV2EndpointError(err error) error {
	return fmt.Errorf(
		"You are trying to edit an issue which contains Jira markdown elements, only supported in jira v3 api (your Jira only supports v2). "+
			"Sending this content as is to Jira will CORRUPT the issue content for everybody else, thus it is forbidden. %w",
		err,
	)
}

// updateComment updates a single comment using PUT /issue/{key}/comment/{commentId} endpoint.
func (c *Client) updateComment(issueKey string, comment EditComment) error {
	path := fmt.Sprintf("/issue/%s/comment/%s", issueKey, comment.ID)

	// This little dance is a dirty hack required to push the json into a struct
	// should be rewritten ASAP
	var requestBody any
	// Parse the ADF JSON string into a map for direct embedding
	var adfMap any
	if err := json.Unmarshal([]byte(comment.Body), &adfMap); err != nil {
		return fmt.Errorf("failed to parse ADF JSON: %w", err)
	}
	requestBody = map[string]any{
		"body": adfMap,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal comment request: %w", err)
	}

	res, err := c.Put(context.Background(), path, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return formatUnexpectedResponse(res)
	}

	return nil
}

// updateCommentV2 updates a single comment using PUT /issue/{key}/comment/{commentId} endpoint (v2 API).
func (c *Client) updateCommentV2(issueKey string, comment EditComment) error {
	path := fmt.Sprintf("/issue/%s/comment/%s", issueKey, comment.ID)

	requestBody := map[string]any{
		"body": comment.Body,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal comment request: %w", err)
	}

	res, err := c.PutV2(context.Background(), path, body, Header{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})
	if err != nil {
		return err
	}
	if res == nil {
		return ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return formatUnexpectedResponse(res)
	}

	return nil
}

type editUpdate struct {
	Summary []struct {
		Set string `json:"set,omitempty"`
	} `json:"summary,omitempty"`
	Description []struct {
		Set interface{} `json:"set,omitempty"`
	} `json:"description,omitempty"`
	Priority []struct {
		Set struct {
			Name string `json:"name,omitempty"`
		} `json:"set,omitempty"`
	} `json:"priority,omitempty"`
	Labels []struct {
		Add    string `json:"add,omitempty"`
		Remove string `json:"remove,omitempty"`
	} `json:"labels,omitempty"`
	Components []struct {
		Add *struct {
			Name string `json:"name,omitempty"`
		} `json:"add,omitempty"`
		Remove *struct {
			Name string `json:"name,omitempty"`
		} `json:"remove,omitempty"`
	} `json:"components,omitempty"`
	FixVersions []struct {
		Add *struct {
			Name string `json:"name,omitempty"`
		} `json:"add,omitempty"`
		Remove *struct {
			Name string `json:"name,omitempty"`
		} `json:"remove,omitempty"`
	} `json:"fixVersions,omitempty"`
	AffectsVersions []struct {
		Add *struct {
			Name string `json:"name,omitempty"`
		} `json:"add,omitempty"`
		Remove *struct {
			Name string `json:"name,omitempty"`
		} `json:"remove,omitempty"`
	} `json:"versions,omitempty"`
}

type editUpdateMarshaler struct {
	M editUpdate
}

// MarshalJSON is a custom marshaler to handle empty fields.
func (cfm *editUpdateMarshaler) MarshalJSON() ([]byte, error) {
	if len(cfm.M.Summary) == 0 || cfm.M.Summary[0].Set == "" {
		cfm.M.Summary = nil
	}
	if len(cfm.M.Description) == 0 || cfm.M.Description[0].Set == nil || cfm.M.Description[0].Set == "" {
		cfm.M.Description = nil
	} else {
	}
	if len(cfm.M.Priority) == 0 || cfm.M.Priority[0].Set.Name == "" {
		cfm.M.Priority = nil
	}
	if len(cfm.M.Components) == 0 || (cfm.M.Components[0].Add != nil && cfm.M.Components[0].Remove != nil) {
		cfm.M.Components = nil
	}
	if len(cfm.M.Labels) == 0 || (cfm.M.Labels[0].Add == "" && cfm.M.Labels[0].Remove == "") {
		cfm.M.Labels = nil
	}

	m, err := json.Marshal(cfm.M)
	if err != nil {
		return m, err
	}

	var temp interface{}
	if err := json.Unmarshal(m, &temp); err != nil {
		return nil, err
	}
	dm := temp.(map[string]interface{})

	return json.Marshal(dm)
}

// MarshalJSON is a custom marshaler to handle empty fields.
func (cfm *editFieldsMarshaler) MarshalJSON() ([]byte, error) {
	m, err := json.Marshal(cfm.M)
	if err != nil {
		return m, err
	}

	var temp interface{}
	if err := json.Unmarshal(m, &temp); err != nil {
		return nil, err
	}
	dm := temp.(map[string]interface{})

	for key, val := range cfm.M.customFields {
		dm[key] = val
	}

	return json.Marshal(dm)
}

type Parent struct {
	Key string `json:"key,omitempty"`
	Set string `json:"set,omitempty"`
}

type editFields struct {
	Parent       Parent `json:"parent,omitempty"`
	customFields customField
}

type editFieldsMarshaler struct {
	M editFields
}

type editRequest struct {
	Update editUpdateMarshaler `json:"update"`
	Fields editFieldsMarshaler `json:"fields"`
}

func getRequestDataForEdit(req *EditRequest) *editRequest {
	if req.Labels == nil {
		req.Labels = []string{}
	}

	var descriptionContent interface{}
	if req.BodyIsRawADF && req.Body != "" {
		// Parse the ADF JSON string into a map for direct embedding
		var adfMap interface{}
		if err := json.Unmarshal([]byte(req.Body), &adfMap); err != nil {
			return nil // Return nil to indicate error, should be handled by caller
		}
		descriptionContent = adfMap
	} else {
		descriptionContent = req.Body
	}

	log.Printf("%v\n", descriptionContent)

	update := editUpdateMarshaler{editUpdate{
		Summary: []struct {
			Set string `json:"set,omitempty"`
		}{{Set: req.Summary}},

		Description: []struct {
			Set interface{} `json:"set,omitempty"`
		}{{Set: descriptionContent}},

		Priority: []struct {
			Set struct {
				Name string `json:"name,omitempty"`
			} `json:"set,omitempty"`
		}{{Set: struct {
			Name string `json:"name,omitempty"`
		}{Name: req.Priority}}},
	}}

	if len(req.Labels) > 0 {
		add, sub := splitAddAndRemove(req.Labels)

		labels := make([]struct {
			Add    string `json:"add,omitempty"`
			Remove string `json:"remove,omitempty"`
		}, 0, len(req.Labels))

		for _, l := range sub {
			labels = append(labels, struct {
				Add    string `json:"add,omitempty"`
				Remove string `json:"remove,omitempty"`
			}{Remove: l})
		}
		for _, l := range add {
			labels = append(labels, struct {
				Add    string `json:"add,omitempty"`
				Remove string `json:"remove,omitempty"`
			}{Add: l})
		}

		update.M.Labels = labels
	}
	if len(req.Components) > 0 {
		add, sub := splitAddAndRemove(req.Components)

		cmp := make([]struct {
			Add *struct {
				Name string `json:"name,omitempty"`
			} `json:"add,omitempty"`
			Remove *struct {
				Name string `json:"name,omitempty"`
			} `json:"remove,omitempty"`
		}, 0, len(req.Components))

		for _, c := range sub {
			cmp = append(cmp, struct {
				Add *struct {
					Name string `json:"name,omitempty"`
				} `json:"add,omitempty"`
				Remove *struct {
					Name string `json:"name,omitempty"`
				} `json:"remove,omitempty"`
			}{Remove: &struct {
				Name string `json:"name,omitempty"`
			}{Name: c}})
		}
		for _, c := range add {
			cmp = append(cmp, struct {
				Add *struct {
					Name string `json:"name,omitempty"`
				} `json:"add,omitempty"`
				Remove *struct {
					Name string `json:"name,omitempty"`
				} `json:"remove,omitempty"`
			}{Add: &struct {
				Name string `json:"name,omitempty"`
			}{Name: c}})
		}

		update.M.Components = cmp
	}
	if len(req.FixVersions) > 0 {
		add, sub := splitAddAndRemove(req.FixVersions)

		versions := make([]struct {
			Add *struct {
				Name string `json:"name,omitempty"`
			} `json:"add,omitempty"`
			Remove *struct {
				Name string `json:"name,omitempty"`
			} `json:"remove,omitempty"`
		}, 0, len(req.FixVersions))

		for _, v := range sub {
			versions = append(versions, struct {
				Add *struct {
					Name string `json:"name,omitempty"`
				} `json:"add,omitempty"`
				Remove *struct {
					Name string `json:"name,omitempty"`
				} `json:"remove,omitempty"`
			}{Remove: &struct {
				Name string `json:"name,omitempty"`
			}{Name: v}})
		}
		for _, v := range add {
			versions = append(versions, struct {
				Add *struct {
					Name string `json:"name,omitempty"`
				} `json:"add,omitempty"`
				Remove *struct {
					Name string `json:"name,omitempty"`
				} `json:"remove,omitempty"`
			}{Add: &struct {
				Name string `json:"name,omitempty"`
			}{Name: v}})
		}

		update.M.FixVersions = versions
	}

	if len(req.AffectsVersions) > 0 {
		add, sub := splitAddAndRemove(req.AffectsVersions)

		versions := make([]struct {
			Add *struct {
				Name string `json:"name,omitempty"`
			} `json:"add,omitempty"`
			Remove *struct {
				Name string `json:"name,omitempty"`
			} `json:"remove,omitempty"`
		}, 0, len(req.AffectsVersions))

		for _, v := range sub {
			versions = append(versions, struct {
				Add *struct {
					Name string `json:"name,omitempty"`
				} `json:"add,omitempty"`
				Remove *struct {
					Name string `json:"name,omitempty"`
				} `json:"remove,omitempty"`
			}{Remove: &struct {
				Name string `json:"name,omitempty"`
			}{Name: v}})
		}
		for _, v := range add {
			versions = append(versions, struct {
				Add *struct {
					Name string `json:"name,omitempty"`
				} `json:"add,omitempty"`
				Remove *struct {
					Name string `json:"name,omitempty"`
				} `json:"remove,omitempty"`
			}{Add: &struct {
				Name string `json:"name,omitempty"`
			}{Name: v}})
		}

		update.M.AffectsVersions = versions
	}

	fields := editFieldsMarshaler{
		M: editFields{
			Parent: Parent{},
		},
	}

	if req.ParentIssueKey != "" {
		if req.ParentIssueKey == AssigneeNone {
			fields.M.Parent.Set = AssigneeNone
		} else {
			fields.M.Parent.Key = req.ParentIssueKey
		}
	}

	data := editRequest{
		Update: update,
		Fields: fields,
	}
	constructCustomFieldsForEdit(req.CustomFields, req.configuredCustomFields, &data)

	return &data
}

func constructCustomFieldsForEdit(fields map[string]string, configuredFields []IssueTypeField, data *editRequest) {
	if len(fields) == 0 || len(configuredFields) == 0 {
		return
	}

	data.Fields.M.customFields = make(customField)

	for key, val := range fields {
		for _, configured := range configuredFields {
			identifier := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(configured.Name)), " ", "-")
			if identifier != strings.ToLower(key) && key != configured.Key {
				continue
			}

			switch configured.Schema.DataType {
			case customFieldFormatOption:
				data.Fields.M.customFields[configured.Key] = []customFieldTypeOptionSet{{Set: customFieldTypeOption{Value: val}}}
			case customFieldFormatProject:
				data.Fields.M.customFields[configured.Key] = []customFieldTypeProjectSet{{Set: customFieldTypeProject{Value: val}}}
			case customFieldFormatArray:
				pieces := strings.Split(strings.TrimSpace(val), ",")
				if configured.Schema.Items == customFieldFormatOption {
					items := make([]customFieldTypeOptionAddRemove, 0)
					for _, p := range pieces {
						if strings.HasPrefix(p, separatorMinus) {
							items = append(items, customFieldTypeOptionAddRemove{Remove: &customFieldTypeOption{Value: strings.TrimPrefix(p, separatorMinus)}})
						} else {
							items = append(items, customFieldTypeOptionAddRemove{Add: &customFieldTypeOption{Value: p}})
						}
					}
					data.Fields.M.customFields[configured.Key] = items
				} else {
					data.Fields.M.customFields[configured.Key] = pieces
				}
			case customFieldFormatNumber:
				num, err := strconv.ParseFloat(val, 64) //nolint:gomnd
				if err != nil {
					// Let Jira API handle data type error for now.
					data.Fields.M.customFields[configured.Key] = []customFieldTypeStringSet{{Set: val}}
				} else {
					data.Fields.M.customFields[configured.Key] = []customFieldTypeNumberSet{{Set: customFieldTypeNumber(num)}}
				}
			default:
				val, _ := md2adf.NewTranslator().TranslateToADF([]byte(val))
				data.Fields.M.customFields[configured.Key] = val
				// TODO we lost compatibility with v2 api here
			}
		}
	}
}

func splitAddAndRemove(input []string) ([]string, []string) {
	add := make([]string, 0, len(input))
	sub := make([]string, 0, len(input))

	for _, inp := range input {
		if strings.HasPrefix(inp, separatorMinus) {
			sub = append(sub, strings.TrimPrefix(inp, separatorMinus))
		}
	}
	for _, inp := range input {
		if !strings.HasPrefix(inp, separatorMinus) && !slices.Contains(sub, inp) {
			add = append(add, inp)
		}
	}

	return add, sub
}

// EditMetadata returns the metadata about fields visible to the user on issue editing screen
// using GET /issue/{issueId}/editmeta handler.
func (c *Client) GetEditMetadata(key string) (*EditMetadata, error) {
	res, err := c.GetV2(context.Background(), "/issue/"+key+"/editmeta", nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	var out EditMetadata

	err = json.NewDecoder(res.Body).Decode(&out)

	return &out, err
}

// GetEditMetadataWithFields returns the metadata about specified fields for issue editing
// using GET /issue/{issueId}?expand=editmeta&fields={fieldIds} handler.
func (c *Client) GetEditMetadataWithFields(key string, fieldIds []string) (*EditMetadata, error) {
	queryParams := "?expand=editmeta&fields=" + strings.Join(fieldIds, ",")

	res, err := c.GetV2(context.Background(), "/issue/"+key+queryParams, nil)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrEmptyResponse
	}
	defer func() { _ = res.Body.Close() }()

	var response struct {
		EditMeta EditMetadata `json:"editmeta"`
	}

	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response.EditMeta, nil
}

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/guionardo/workitems"
)

const projectMaxResponseBytes = 4 << 20

// ProjectHTTPClient implements the GitHub Projects V2 GraphQL API.
type ProjectHTTPClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// NewProjectHTTPClient creates a GitHub Projects V2 client.
func NewProjectHTTPClient(baseURL, token string) ProjectHTTPClient {
	return ProjectHTTPClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:   strings.TrimSpace(token),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// GetProjectItem retrieves one project item and its configured status value.
func (c ProjectHTTPClient) GetProjectItem(ctx context.Context, projectID, itemID, statusFieldID string) (ProjectItem, error) {
	var data struct {
		Node projectItemResponse `json:"node"`
	}
	if err := c.query(ctx, projectItemQuery, map[string]any{
		"itemID": itemID,
	}, &data); err != nil {
		return ProjectItem{}, err
	}
	return data.Node.item(statusFieldID), nil
}

// ListProjectItems lists one page of project items.
func (c ProjectHTTPClient) ListProjectItems(ctx context.Context, projectID, statusFieldID, cursor string) (ProjectPage, error) {
	var data struct {
		Node struct {
			Items struct {
				Nodes    []projectItemResponse `json:"nodes"`
				PageInfo pageInfo              `json:"pageInfo"`
			} `json:"items"`
		} `json:"node"`
	}
	variables := map[string]any{"projectID": projectID}
	if cursor != "" {
		variables["cursor"] = cursor
	}
	if err := c.query(ctx, projectListQuery, variables, &data); err != nil {
		return ProjectPage{}, err
	}
	items := make([]ProjectItem, 0, len(data.Node.Items.Nodes))
	for _, response := range data.Node.Items.Nodes {
		items = append(items, response.item(statusFieldID))
	}
	page := ProjectPage{Items: items}
	if data.Node.Items.PageInfo.HasNextPage {
		page.NextPageToken = data.Node.Items.PageInfo.EndCursor
	}
	return page, nil
}

// SetProjectItemStatus updates a project's single-select status option.
func (c ProjectHTTPClient) SetProjectItemStatus(ctx context.Context, projectID, itemID, fieldID, optionID string) (ProjectItem, error) {
	var data struct {
		Mutation struct {
			Item projectItemResponse `json:"projectV2Item"`
		} `json:"updateProjectV2ItemFieldValue"`
	}
	if err := c.query(ctx, projectStatusMutation, map[string]any{
		"projectID": projectID, "itemID": itemID, "fieldID": fieldID, "optionID": optionID,
	}, &data); err != nil {
		return ProjectItem{}, err
	}
	return data.Mutation.Item.item(fieldID), nil
}

// AddProjectItem adds an issue to the configured project and returns its current item data.
func (c ProjectHTTPClient) AddProjectItem(ctx context.Context, projectID, contentID, statusFieldID string) (ProjectItem, error) {
	var data struct {
		Mutation struct {
			Item projectItemResponse `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	if err := c.query(ctx, projectAddItemMutation, map[string]any{
		"projectID": projectID, "contentID": contentID,
	}, &data); err != nil {
		return ProjectItem{}, err
	}
	if data.Mutation.Item.ID == "" {
		return ProjectItem{}, errors.New("GitHub project item was not returned after adding issue")
	}
	// shortcut: GitHub may expose the new item before its default Status value; retry briefly for consistency.
	for attempt := 0; attempt < 5; attempt++ {
		item, err := c.GetProjectItem(ctx, projectID, data.Mutation.Item.ID, statusFieldID)
		if err != nil || item.ProviderStatus.ID != "" || attempt == 4 {
			return item, err
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ProjectItem{}, ctx.Err()
		case <-timer.C:
		}
	}
	return ProjectItem{}, errors.New("GitHub project item status was not available")
}

func (c ProjectHTTPClient) query(ctx context.Context, query string, variables map[string]any, target any) error {
	if c.BaseURL == "" || c.Token == "" {
		return errors.New("github projects client credentials are required")
	}
	payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("marshal GitHub GraphQL request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create GitHub GraphQL request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("GitHub GraphQL request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return &APIError{Status: response.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, projectMaxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read GitHub GraphQL response: %w", err)
	}
	if len(data) > projectMaxResponseBytes {
		return errors.New("GitHub GraphQL response exceeds size limit")
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode GitHub GraphQL response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return errors.New("GitHub GraphQL request returned errors")
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		return fmt.Errorf("decode GitHub GraphQL data: %w", err)
	}
	return nil
}

type projectItemResponse struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Content     projectContent `json:"content"`
	FieldValues struct {
		Nodes []projectFieldValue `json:"nodes"`
	} `json:"fieldValues"`
}

type projectContent struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

type projectFieldValue struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Field    struct {
		ID string `json:"id"`
	} `json:"field"`
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

func (response projectItemResponse) item(statusFieldID string) ProjectItem {
	labels := make([]string, 0, len(response.Content.Labels.Nodes))
	for _, label := range response.Content.Labels.Nodes {
		labels = append(labels, label.Name)
	}
	status := workitem.ProviderStatus{}
	for _, value := range response.FieldValues.Nodes {
		if value.Field.ID == statusFieldID {
			status = workitem.ProviderStatus{ID: value.OptionID, Name: value.Name}
			break
		}
	}
	return ProjectItem{
		ID: response.ID, URL: response.Content.URL, IssueNumber: response.Content.Number,
		Title: response.Content.Title, Description: response.Content.Body, Tags: labels,
		Type: response.Type, ProviderStatus: status,
	}
}

const projectItemFields = `
id
type
content {
  ... on Issue {
    number
    url
    title
    body
    labels(first: 100) { nodes { name } }
  }
}
fieldValues(first: 100) {
  nodes {
    ... on ProjectV2ItemFieldSingleSelectValue {
      optionId
      name
      field { ... on ProjectV2SingleSelectField { id } }
    }
  }
}`

const projectItemQuery = `query($itemID: ID!) {
  node(id: $itemID) {
    ... on ProjectV2Item {
      ` + projectItemFields + `
    }
  }
}`

const projectListQuery = `query($projectID: ID!, $cursor: String) {
  node(id: $projectID) {
    ... on ProjectV2 {
      items(first: 100, after: $cursor) {
        nodes {
          ` + projectItemFields + `
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}`

const projectStatusMutation = `mutation($projectID: ID!, $itemID: ID!, $fieldID: ID!, $optionID: String!) {
  updateProjectV2ItemFieldValue(input: {
    projectId: $projectID
    itemId: $itemID
    fieldId: $fieldID
    value: { singleSelectOptionId: $optionID }
  }) {
    projectV2Item {
      ` + projectItemFields + `
    }
  }
}`

const projectAddItemMutation = `mutation($projectID: ID!, $contentID: ID!) {
  addProjectV2ItemById(input: { projectId: $projectID, contentId: $contentID }) {
    item {
      ` + projectItemFields + `
    }
  }
}`

var _ ProjectClient = ProjectHTTPClient{}

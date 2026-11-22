package linear

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

const maxResponseBytes = 4 << 20

// APIError is a safe Linear HTTP error that does not expose response bodies.
type APIError struct{ Status int }

func (e *APIError) Error() string {
	return fmt.Sprintf("Linear request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with the Linear GraphQL API.
type HTTPClient struct {
	BaseURL string
	APIKey  string
	TeamID  string
	HTTP    *http.Client
}

// NewHTTPClient creates a Linear client for one team.
func NewHTTPClient(baseURL, apiKey, teamID string) HTTPClient {
	return HTTPClient{BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), APIKey: strings.TrimSpace(apiKey), TeamID: strings.TrimSpace(teamID), HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// GetIssue retrieves a Linear issue by UUID or identifier.
func (c HTTPClient) GetIssue(ctx context.Context, id string) (Issue, error) {
	var data struct {
		Issue issueResponse `json:"issue"`
	}
	if err := c.query(ctx, `query($id:String!) { issue(id:$id) { `+issueFields+` } }`, map[string]any{"id": id}, &data); err != nil {
		return Issue{}, err
	}
	return data.Issue.issue(), nil
}

// ListIssues lists one page of issues for the configured team.
func (c HTTPClient) ListIssues(ctx context.Context, cursor string) (IssuePage, error) {
	var data struct {
		Team struct {
			Issues struct {
				Nodes    []issueResponse `json:"nodes"`
				PageInfo pageInfo        `json:"pageInfo"`
			} `json:"issues"`
		} `json:"team"`
	}
	variables := map[string]any{"teamID": c.TeamID}
	if cursor != "" {
		variables["cursor"] = cursor
	}
	if err := c.query(ctx, `query($teamID:ID!, $cursor:String) { team(id:$teamID) { issues(first:100, after:$cursor) { nodes { `+issueFields+` } pageInfo { hasNextPage endCursor } } } }`, variables, &data); err != nil {
		return IssuePage{}, err
	}
	issues := make([]Issue, 0, len(data.Team.Issues.Nodes))
	for _, response := range data.Team.Issues.Nodes {
		issues = append(issues, response.issue())
	}
	page := IssuePage{Issues: issues}
	if data.Team.Issues.PageInfo.HasNextPage {
		page.NextPageToken = data.Team.Issues.PageInfo.EndCursor
	}
	return page, nil
}

// CreateIssue creates a Linear issue with the configured team.
func (c HTTPClient) CreateIssue(ctx context.Context, input workitem.CreateTaskInput) (Issue, error) {
	var data struct {
		Mutation struct {
			Success bool          `json:"success"`
			Issue   issueResponse `json:"issue"`
		} `json:"issueCreate"`
	}
	issueInput := map[string]any{
		"teamId":      c.TeamID,
		"title":       input.Title,
		"description": input.Description,
	}
	if len(input.Tags) > 0 {
		labelIDs, err := c.labelIDs(ctx, input.Tags)
		if err != nil {
			return Issue{}, err
		}
		issueInput["labelIds"] = labelIDs
	}
	variables := map[string]any{"input": issueInput}
	if err := c.query(ctx, `mutation($input:IssueCreateInput!) { issueCreate(input:$input) { success issue { `+issueFields+` } } }`, variables, &data); err != nil {
		return Issue{}, err
	}
	if !data.Mutation.Success || data.Mutation.Issue.ID == "" {
		return Issue{}, errors.New("Linear issue creation was not successful")
	}
	return data.Mutation.Issue.issue(), nil
}

func (c HTTPClient) labelIDs(ctx context.Context, names []string) ([]string, error) {
	var data struct {
		Team struct {
			Labels struct {
				Nodes []Label `json:"nodes"`
			} `json:"labels"`
		} `json:"team"`
	}
	if err := c.query(ctx, `query($teamID:ID!) { team(id:$teamID) { labels(first:100) { nodes { id name } } } }`, map[string]any{"teamID": c.TeamID}, &data); err != nil {
		return nil, err
	}

	byName := make(map[string]string, len(data.Team.Labels.Nodes))
	for _, label := range data.Team.Labels.Nodes {
		byName[label.Name] = label.ID
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("Linear issue label %q was not found", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// SetIssueState updates a Linear issue's workflow state.
func (c HTTPClient) SetIssueState(ctx context.Context, id, stateID string) (Issue, error) {
	var data struct {
		Mutation struct {
			Success bool          `json:"success"`
			Issue   issueResponse `json:"issue"`
		} `json:"issueUpdate"`
	}
	variables := map[string]any{"id": id, "input": map[string]any{"stateId": stateID}}
	if err := c.query(ctx, `mutation($id:String!, $input:IssueUpdateInput!) { issueUpdate(id:$id, input:$input) { success issue { `+issueFields+` } } }`, variables, &data); err != nil {
		return Issue{}, err
	}
	if !data.Mutation.Success {
		return Issue{}, errors.New("Linear issue update was not successful")
	}
	return data.Mutation.Issue.issue(), nil
}

func (c HTTPClient) query(ctx context.Context, query string, variables map[string]any, target any) error {
	if c.BaseURL == "" || c.APIKey == "" || c.TeamID == "" {
		return errors.New("Linear client configuration is required")
	}
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("marshal Linear request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", c.APIKey)
	request.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Linear request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return &APIError{Status: response.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return errors.New("Linear response exceeds size limit")
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return errors.New("Linear GraphQL request returned errors")
	}
	return json.Unmarshal(envelope.Data, target)
}

type issueResponse struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"state"`
	Labels struct {
		Nodes []Label `json:"nodes"`
	} `json:"labels"`
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

func (response issueResponse) issue() Issue {
	return Issue{ID: response.ID, Identifier: response.Identifier, URL: response.URL, Title: response.Title, Description: response.Description, Labels: append([]Label(nil), response.Labels.Nodes...), State: workitem.ProviderStatus{ID: response.State.ID, Name: response.State.Name}}
}

const issueFields = `id identifier url title description state { id name } labels(first:50) { nodes { id name } }`

var _ Client = HTTPClient{}

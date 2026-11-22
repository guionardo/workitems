package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/guionardo/workitems"
)

const maxResponseBytes = 4 << 20

// APIError is a safe Jira HTTP error that does not expose response bodies.
type APIError struct {
	Status int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Jira request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with Jira Cloud REST API v3.
type HTTPClient struct {
	BaseURL   string
	Email     string
	APIToken  string
	Project   string
	IssueType string
	HTTP      *http.Client
}

// NewHTTPClient creates a Jira REST client configured for one project.
func NewHTTPClient(baseURL, email, apiToken, project string) HTTPClient {
	return HTTPClient{
		BaseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Email:     strings.TrimSpace(email),
		APIToken:  strings.TrimSpace(apiToken),
		Project:   strings.TrimSpace(project),
		IssueType: "Task",
		HTTP:      &http.Client{Timeout: 10 * time.Second},
	}
}

// GetIssue retrieves a Jira issue by key or ID.
func (c HTTPClient) GetIssue(ctx context.Context, key string) (Issue, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Issue{}, errors.New("jira issue key is required")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(key)+"?fields=summary,description,labels,status", nil, &response); err != nil {
		return Issue{}, err
	}
	return c.issue(response), nil
}

// SearchIssues searches issues using Jira's enhanced JQL endpoint.
func (c HTTPClient) SearchIssues(ctx context.Context, options SearchOptions) (IssuePage, error) {
	if c.Project == "" {
		return IssuePage{}, errors.New("jira project is required")
	}
	jql := c.jql(options)
	maxResults := options.PageSize
	if maxResults <= 0 || maxResults > 100 {
		maxResults = 50
	}
	body := map[string]any{
		"jql":        jql,
		"fields":     []string{"summary", "description", "labels", "status"},
		"maxResults": maxResults,
	}
	if options.PageToken != "" {
		body["nextPageToken"] = options.PageToken
	}
	var response searchResponse
	if err := c.do(ctx, http.MethodPost, "/rest/api/3/search/jql", body, &response); err != nil {
		return IssuePage{}, err
	}
	page := IssuePage{NextPageToken: response.NextPageToken, Issues: make([]Issue, 0, len(response.Issues))}
	for _, issue := range response.Issues {
		page.Issues = append(page.Issues, c.issue(issue))
	}
	return page, nil
}

var _ Client = HTTPClient{}

// CreateIssue creates a Jira issue with the configured project and issue type.
func (c HTTPClient) CreateIssue(ctx context.Context, input workitem.CreateTaskInput) (Issue, error) {
	if c.Project == "" {
		return Issue{}, errors.New("jira project is required")
	}
	issueType := c.IssueType
	if issueType == "" {
		issueType = "Task"
	}
	fields := map[string]any{
		"project":   map[string]string{"key": c.Project},
		"summary":   input.Title,
		"issuetype": map[string]string{"name": issueType},
	}
	if input.Description != "" {
		fields["description"] = adfDocument(input.Description)
	}
	if input.Tags != nil {
		fields["labels"] = append([]string(nil), input.Tags...)
	}
	var response struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := c.do(ctx, http.MethodPost, "/rest/api/3/issue", map[string]any{"fields": fields}, &response); err != nil {
		return Issue{}, err
	}
	if response.Key == "" && response.ID == "" {
		return Issue{}, errors.New("jira create issue returned no reference")
	}
	key := firstNonEmpty(response.Key, response.ID)
	return c.GetIssue(ctx, key)
}

// ListTransitions returns transitions currently available for an issue.
func (c HTTPClient) ListTransitions(ctx context.Context, key string) ([]Transition, error) {
	var response struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/issue/"+url.PathEscape(strings.TrimSpace(key))+"/transitions", nil, &response); err != nil {
		return nil, err
	}
	transitions := make([]Transition, 0, len(response.Transitions))
	for _, item := range response.Transitions {
		transitions = append(transitions, Transition{
			ID: item.ID, Name: item.Name,
			To: workitem.ProviderStatus{ID: item.To.ID, Name: item.To.Name},
		})
	}
	return transitions, nil
}

// TransitionIssue executes a Jira transition and returns the updated issue.
func (c HTTPClient) TransitionIssue(ctx context.Context, key, transitionID string) (Issue, error) {
	key = strings.TrimSpace(key)
	transitionID = strings.TrimSpace(transitionID)
	if key == "" || transitionID == "" {
		return Issue{}, errors.New("jira issue key and transition ID are required")
	}
	if err := c.do(ctx, http.MethodPost, "/rest/api/3/issue/"+url.PathEscape(key)+"/transitions", map[string]any{
		"transition": map[string]string{"id": transitionID},
	}, nil); err != nil {
		return Issue{}, err
	}
	return c.GetIssue(ctx, key)
}

func (c HTTPClient) do(ctx context.Context, method, endpoint string, body any, target any) error {
	if c.BaseURL == "" || c.Email == "" || c.APIToken == "" {
		return errors.New("jira client credentials and base URL are required")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return errors.New("jira base URL is invalid")
	}
	relative, err := url.Parse(endpoint)
	if err != nil || !strings.HasPrefix(relative.Path, "/") {
		return errors.New("jira endpoint is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + relative.Path
	base.RawQuery = relative.RawQuery
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal Jira request: %w", err)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, base.String(), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create Jira request: %w", err)
	}
	request.SetBasicAuth(c.Email, c.APIToken)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Jira request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return &APIError{Status: response.StatusCode}
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Jira response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return errors.New("Jira response exceeds size limit")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode Jira response: %w", err)
	}
	return nil
}

func (c HTTPClient) jql(options SearchOptions) string {
	parts := make([]string, 0, 3)
	if c.Project != "" {
		parts = append(parts, `project = `+quote(c.Project))
	}
	if len(options.Statuses) > 0 {
		parts = append(parts, `status in (`+joinQuoted(statusNames(options.Statuses))+`)`)
	}
	if len(options.Labels) > 0 {
		parts = append(parts, `labels in (`+joinQuoted(options.Labels)+`)`)
	}
	return strings.Join(parts, " AND ") + " ORDER BY updated DESC"
}

type issueResponse struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Labels      []string        `json:"labels"`
		Status      struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

type searchResponse struct {
	Issues        []issueResponse `json:"issues"`
	NextPageToken string          `json:"nextPageToken"`
}

func (c HTTPClient) issue(response issueResponse) Issue {
	key := firstNonEmpty(response.Key, response.ID)
	return Issue{
		ID: response.ID, Key: key,
		URL:     strings.TrimRight(c.BaseURL, "/") + "/browse/" + url.PathEscape(key),
		Summary: response.Fields.Summary, Description: adfText(response.Fields.Description),
		Status: workitem.ProviderStatus{ID: response.Fields.Status.ID, Name: response.Fields.Status.Name},
		Labels: append([]string(nil), response.Fields.Labels...),
	}
}

func adfDocument(text string) map[string]any {
	return map[string]any{
		"version": 1, "type": "doc",
		"content": []any{map[string]any{
			"type":    "paragraph",
			"content": []any{map[string]string{"type": "text", "text": text}},
		}},
	}
}

func adfText(data json.RawMessage) string {
	if len(data) == 0 || string(data) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		return text
	}
	var node struct {
		Type    string            `json:"type"`
		Text    string            `json:"text"`
		Content []json.RawMessage `json:"content"`
	}
	if json.Unmarshal(data, &node) != nil {
		return ""
	}
	parts := make([]string, 0, len(node.Content)+1)
	if node.Text != "" {
		parts = append(parts, node.Text)
	}
	for _, child := range node.Content {
		if value := adfText(child); value != "" {
			parts = append(parts, value)
		}
	}
	separator := ""
	if node.Type == "paragraph" || node.Type == "heading" || node.Type == "listItem" {
		separator = "\n"
	}
	return strings.TrimSpace(strings.Join(parts, separator))
}

func statusNames(statuses []workitem.ProviderStatus) []string {
	names := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status.Name != "" {
			names = append(names, status.Name)
			continue
		}
		names = append(names, status.ID)
	}
	return names
}

func joinQuoted(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, quote(value))
	}
	return strings.Join(quoted, ", ")
}

func quote(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

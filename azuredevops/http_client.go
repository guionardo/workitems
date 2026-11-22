package azuredevops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/guionardo/workitems"
)

const maxResponseBytes = 4 << 20

// APIError is a safe Azure DevOps HTTP error that does not expose response bodies.
type APIError struct{ Status int }

func (e *APIError) Error() string {
	return fmt.Sprintf("Azure DevOps request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with Azure DevOps Work Item Tracking REST APIs.
type HTTPClient struct {
	BaseURL      string
	Organization string
	Project      string
	Token        string
	WorkItemType string
	HTTP         *http.Client
}

// NewHTTPClient creates an Azure DevOps work item client.
func NewHTTPClient(baseURL, organization, project, token, workItemType string) HTTPClient {
	if strings.TrimSpace(workItemType) == "" {
		workItemType = "Task"
	}
	return HTTPClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), Organization: strings.TrimSpace(organization),
		Project: strings.TrimSpace(project), Token: strings.TrimSpace(token), WorkItemType: strings.TrimSpace(workItemType),
		HTTP: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetWorkItem retrieves a work item by ID.
func (c HTTPClient) GetWorkItem(ctx context.Context, id int) (WorkItem, error) {
	if id < 1 {
		return WorkItem{}, errors.New("Azure DevOps work item ID is required")
	}
	var response workItemResponse
	if err := c.do(ctx, http.MethodGet, "/_apis/wit/workitems/"+strconv.Itoa(id), url.Values{"$expand": {"Fields"}}, nil, "application/json", &response); err != nil {
		return WorkItem{}, err
	}
	return response.item(), nil
}

// ListWorkItems executes a flat WIQL query and loads its work items.
func (c HTTPClient) ListWorkItems(ctx context.Context, options ListOptions) (WorkItemPage, error) {
	clauses := []string{"[System.TeamProject] = @project", "[System.WorkItemType] = '" + escapeWIQL(c.WorkItemType) + "'"}
	if len(options.States) > 0 {
		states := make([]string, 0, len(options.States))
		for _, state := range options.States {
			states = append(states, "'"+escapeWIQL(state.Name)+"'")
		}
		clauses = append(clauses, "[System.State] IN ("+strings.Join(states, ",")+")")
	}
	for _, tag := range options.Tags {
		clauses = append(clauses, "[System.Tags] CONTAINS '"+escapeWIQL(tag)+"'")
	}
	wiql := "SELECT [System.Id] FROM WorkItems WHERE " + strings.Join(clauses, " AND ") + " ORDER BY [System.Id]"
	var query wiqlResponse
	if err := c.do(ctx, http.MethodPost, "/_apis/wit/wiql", nil, map[string]string{"query": wiql}, "application/json", &query); err != nil {
		return WorkItemPage{}, err
	}
	offset := 0
	if options.PageToken != "" {
		var err error
		offset, err = strconv.Atoi(options.PageToken)
		if err != nil || offset < 0 {
			return WorkItemPage{}, errors.New("Azure DevOps page token is invalid")
		}
	}
	pageSize := options.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	if offset >= len(query.WorkItems) {
		return WorkItemPage{}, nil
	}
	end := offset + pageSize
	if end > len(query.WorkItems) {
		end = len(query.WorkItems)
	}
	ids := make([]string, 0, end-offset)
	for _, reference := range query.WorkItems[offset:end] {
		ids = append(ids, strconv.Itoa(reference.ID))
	}
	var response workItemsResponse
	if err := c.do(ctx, http.MethodGet, "/_apis/wit/workitems", url.Values{"ids": {strings.Join(ids, ",")}, "$expand": {"Fields"}}, nil, "application/json", &response); err != nil {
		return WorkItemPage{}, err
	}
	result := WorkItemPage{Items: make([]WorkItem, 0, len(response.Value))}
	for _, item := range response.Value {
		result.Items = append(result.Items, item.item())
	}
	if end < len(query.WorkItems) {
		result.NextPageToken = strconv.Itoa(end)
	}
	return result, nil
}

// CreateWorkItem creates a work item of the configured type.
func (c HTTPClient) CreateWorkItem(ctx context.Context, input workitem.CreateTaskInput) (WorkItem, error) {
	patch := []patchOperation{{Op: "add", Path: "/fields/System.Title", Value: input.Title}, {Op: "add", Path: "/fields/System.Description", Value: input.Description}, {Op: "add", Path: "/fields/System.Tags", Value: strings.Join(input.Tags, "; ")}}
	var response workItemResponse
	path := "/_apis/wit/workitems/$" + url.PathEscape(c.WorkItemType)
	if err := c.do(ctx, http.MethodPost, path, nil, patch, "application/json-patch+json", &response); err != nil {
		return WorkItem{}, err
	}
	return response.item(), nil
}

// SetWorkItemState updates the System.State field.
func (c HTTPClient) SetWorkItemState(ctx context.Context, id int, state string) (WorkItem, error) {
	if id < 1 || strings.TrimSpace(state) == "" {
		return WorkItem{}, errors.New("Azure DevOps work item state is required")
	}
	var response workItemResponse
	patch := []patchOperation{{Op: "add", Path: "/fields/System.State", Value: state}}
	if err := c.do(ctx, http.MethodPatch, "/_apis/wit/workitems/"+strconv.Itoa(id), nil, patch, "application/json-patch+json", &response); err != nil {
		return WorkItem{}, err
	}
	return response.item(), nil
}

func (c HTTPClient) do(ctx context.Context, method, endpoint string, query url.Values, body any, contentType string, target any) error {
	if c.BaseURL == "" || c.Organization == "" || c.Project == "" || c.Token == "" {
		return errors.New("Azure DevOps client configuration is required")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return errors.New("Azure DevOps base URL is invalid")
	}
	basePath := strings.TrimRight(base.Path, "/") + "/" + url.PathEscape(c.Organization) + "/" + url.PathEscape(c.Project) + endpoint
	base.Path = basePath
	if query == nil {
		query = url.Values{}
	}
	query.Set("api-version", "7.1")
	base.RawQuery = query.Encode()
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal Azure DevOps request: %w", err)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, base.String(), bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create Azure DevOps request: %w", err)
	}
	auth := base64.StdEncoding.EncodeToString([]byte(":" + c.Token))
	request.Header.Set("Authorization", "Basic "+auth)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", contentType)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Azure DevOps request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return &APIError{Status: response.StatusCode}
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Azure DevOps response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return errors.New("Azure DevOps response exceeds size limit")
	}
	if target != nil && len(data) > 0 {
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("decode Azure DevOps response: %w", err)
		}
	}
	return nil
}

func escapeWIQL(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

type patchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

type wiqlResponse struct {
	WorkItems []struct {
		ID int `json:"id"`
	} `json:"workItems"`
}

type workItemsResponse struct {
	Value []workItemResponse `json:"value"`
}

type workItemResponse struct {
	ID     int            `json:"id"`
	URL    string         `json:"url"`
	Fields map[string]any `json:"fields"`
}

func (response workItemResponse) item() WorkItem {
	state := stringField(response.Fields, "System.State")
	tags := strings.FieldsFunc(stringField(response.Fields, "System.Tags"), func(r rune) bool { return r == ';' || r == ',' })
	for i := range tags {
		tags[i] = strings.TrimSpace(tags[i])
	}
	return WorkItem{
		ID: response.ID, URL: response.URL, Type: stringField(response.Fields, "System.WorkItemType"),
		Title: stringField(response.Fields, "System.Title"), Description: stringField(response.Fields, "System.Description"),
		Tags: tags, State: workitem.ProviderStatus{ID: state, Name: state},
	}
}

func stringField(fields map[string]any, name string) string {
	value, _ := fields[name].(string)
	return value
}

var _ Client = HTTPClient{}

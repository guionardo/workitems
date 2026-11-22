package gitlab

import (
	"context"
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

// APIError is a safe GitLab HTTP error that does not expose response bodies.
type APIError struct {
	Status int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("GitLab request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with the GitLab REST API.
type HTTPClient struct {
	BaseURL string
	Token   string
	Project string
	HTTP    *http.Client
}

// NewHTTPClient creates a GitLab issue client for one project.
func NewHTTPClient(baseURL, token, project string) HTTPClient {
	return HTTPClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:   strings.TrimSpace(token), Project: strings.TrimSpace(project),
		HTTP: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetIssue retrieves a GitLab issue by project-local IID.
func (c HTTPClient) GetIssue(ctx context.Context, iid int) (Issue, error) {
	if iid < 1 {
		return Issue{}, errors.New("gitlab issue IID is required")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodGet, "/issues/"+strconv.Itoa(iid), nil, nil, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// ListIssues lists issues in the configured GitLab project.
func (c HTTPClient) ListIssues(ctx context.Context, options ListOptions) (IssuePage, error) {
	page := options.Page
	if page < 1 {
		page = 1
	}
	query := url.Values{"state": {options.State}, "per_page": {"100"}, "page": {strconv.Itoa(page)}}
	if query.Get("state") == "" {
		query.Set("state", "all")
	}
	if len(options.Labels) > 0 {
		query.Set("labels", strings.Join(options.Labels, ","))
	}
	var response []issueResponse
	header, err := c.doWithHeader(ctx, http.MethodGet, "/issues", query, nil, &response)
	if err != nil {
		return IssuePage{}, err
	}
	pageResult := IssuePage{Issues: make([]Issue, 0, len(response))}
	for _, item := range response {
		pageResult.Issues = append(pageResult.Issues, item.issue())
	}
	pageResult.NextPageToken = header.Get("X-Next-Page")
	return pageResult, nil
}

// CreateIssue creates a GitLab issue with the configured project.
func (c HTTPClient) CreateIssue(ctx context.Context, input workitem.CreateTaskInput) (Issue, error) {
	var response issueResponse
	form := url.Values{"title": {input.Title}, "description": {input.Description}, "labels": {strings.Join(input.Tags, ",")}}
	if err := c.do(ctx, http.MethodPost, "/issues", nil, form, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// SetIssueState opens or closes a GitLab issue.
func (c HTTPClient) SetIssueState(ctx context.Context, iid int, stateEvent string) (Issue, error) {
	if iid < 1 || (stateEvent != "close" && stateEvent != "reopen") {
		return Issue{}, errors.New("gitlab issue state event is invalid")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodPut, "/issues/"+strconv.Itoa(iid), nil, url.Values{"state_event": {stateEvent}}, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

func (c HTTPClient) do(ctx context.Context, method, endpoint string, query url.Values, body url.Values, target any) error {
	_, err := c.doWithHeader(ctx, method, endpoint, query, body, target)
	return err
}

func (c HTTPClient) doWithHeader(ctx context.Context, method, endpoint string, query url.Values, body url.Values, target any) (http.Header, error) {
	if c.BaseURL == "" || c.Token == "" || c.Project == "" {
		return nil, errors.New("gitlab client credentials and project are required")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("gitlab base URL is invalid")
	}
	basePath := strings.TrimRight(base.Path, "/")
	baseEscapedPath := strings.TrimRight(base.EscapedPath(), "/")
	base.Path = basePath + "/api/v4/projects/" + c.Project + endpoint
	base.RawPath = baseEscapedPath + "/api/v4/projects/" + url.PathEscape(c.Project) + endpoint
	base.RawQuery = query.Encode()
	var reader io.Reader
	if body != nil {
		reader = strings.NewReader(body.Encode())
	}
	request, err := http.NewRequestWithContext(ctx, method, base.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("create GitLab request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("PRIVATE-TOKEN", c.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GitLab request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return response.Header, &APIError{Status: response.StatusCode}
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return response.Header, nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return response.Header, fmt.Errorf("read GitLab response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return response.Header, errors.New("GitLab response exceeds size limit")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return response.Header, fmt.Errorf("decode GitLab response: %w", err)
	}
	return response.Header, nil
}

type issueResponse struct {
	IID         int      `json:"iid"`
	WebURL      string   `json:"web_url"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	State       string   `json:"state"`
	Labels      []string `json:"labels"`
}

func (response issueResponse) issue() Issue {
	return Issue{IID: response.IID, URL: response.WebURL, Title: response.Title, Description: response.Description, State: response.State, Labels: append([]string(nil), response.Labels...)}
}

var _ Client = HTTPClient{}

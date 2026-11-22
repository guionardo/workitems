package gitea

import (
	"bytes"
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

// APIError is a safe Gitea HTTP error that does not expose response bodies.
type APIError struct{ Status int }

func (e *APIError) Error() string {
	return fmt.Sprintf("Gitea request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with the Gitea REST API.
type HTTPClient struct {
	BaseURL string
	Token   string
	Owner   string
	Repo    string
	HTTP    *http.Client
}

// NewHTTPClient creates a Gitea issue client for one repository.
func NewHTTPClient(baseURL, token, owner, repo string) HTTPClient {
	return HTTPClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:   strings.TrimSpace(token),
		Owner:   strings.TrimSpace(owner),
		Repo:    strings.TrimSpace(repo),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// GetIssue retrieves a Gitea issue by repository-local index.
func (c HTTPClient) GetIssue(ctx context.Context, index int) (Issue, error) {
	if index < 1 {
		return Issue{}, errors.New("gitea issue index is required")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodGet, "/issues/"+strconv.Itoa(index), nil, nil, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// ListIssues lists issues in the configured Gitea repository.
func (c HTTPClient) ListIssues(ctx context.Context, options ListOptions) (IssuePage, error) {
	page := options.Page
	if page < 1 {
		page = 1
	}
	query := url.Values{
		"state": {options.State},
		"page":  {strconv.Itoa(page)},
		"limit": {"100"},
		"type":  {"issues"},
	}
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
	issues := make([]Issue, 0, len(response))
	for _, item := range response {
		issues = append(issues, item.issue())
	}
	result := IssuePage{Issues: issues}
	if strings.Contains(header.Get("Link"), `rel="next"`) {
		result.NextPageToken = strconv.Itoa(page + 1)
	}
	return result, nil
}

// CreateIssue creates a Gitea issue with the configured repository.
func (c HTTPClient) CreateIssue(ctx context.Context, input workitem.CreateTaskInput) (Issue, error) {
	labels, err := c.labelIDs(ctx, input.Tags)
	if err != nil {
		return Issue{}, err
	}
	var response issueResponse
	body := map[string]any{
		"title":  input.Title,
		"body":   input.Description,
		"labels": labels,
	}
	if err := c.do(ctx, http.MethodPost, "/issues", nil, body, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// SetIssueState opens or closes a Gitea issue.
func (c HTTPClient) SetIssueState(ctx context.Context, index int, state string) (Issue, error) {
	if index < 1 || (state != "open" && state != "closed") {
		return Issue{}, errors.New("gitea issue state is invalid")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodPatch, "/issues/"+strconv.Itoa(index), nil, map[string]any{"closed": state == "closed"}, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

func (c HTTPClient) labelIDs(ctx context.Context, names []string) ([]int64, error) {
	if len(names) == 0 {
		return []int64{}, nil
	}
	var labels []labelResponse
	for page := 1; ; page++ {
		var pageLabels []labelResponse
		query := url.Values{"page": {strconv.Itoa(page)}, "limit": {"100"}}
		if err := c.do(ctx, http.MethodGet, "/labels", query, nil, &pageLabels); err != nil {
			return nil, err
		}
		labels = append(labels, pageLabels...)
		if len(pageLabels) < 100 {
			break
		}
	}
	byName := make(map[string]int64, len(labels))
	for _, label := range labels {
		byName[label.Name] = label.ID
	}
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("Gitea issue label %q was not found", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c HTTPClient) do(ctx context.Context, method, endpoint string, query url.Values, body any, target any) error {
	_, err := c.doWithHeader(ctx, method, endpoint, query, body, target)
	return err
}

func (c HTTPClient) doWithHeader(ctx context.Context, method, endpoint string, query url.Values, body any, target any) (http.Header, error) {
	if c.BaseURL == "" || c.Token == "" || c.Owner == "" || c.Repo == "" {
		return nil, errors.New("gitea client credentials and repository are required")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("gitea base URL is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/v1/repos/" + url.PathEscape(c.Owner) + "/" + url.PathEscape(c.Repo) + endpoint
	base.RawQuery = query.Encode()
	var reader io.Reader
	if body != nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return nil, fmt.Errorf("marshal Gitea request: %w", marshalErr)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, base.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("create Gitea request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "token "+c.Token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("Gitea request: %w", err)
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
		return response.Header, fmt.Errorf("read Gitea response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return response.Header, errors.New("Gitea response exceeds size limit")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return response.Header, fmt.Errorf("decode Gitea response: %w", err)
	}
	return response.Header, nil
}

type issueResponse struct {
	Number  int             `json:"number"`
	HTMLURL string          `json:"html_url"`
	Title   string          `json:"title"`
	Body    string          `json:"body"`
	State   string          `json:"state"`
	Labels  []labelResponse `json:"labels"`
}

type labelResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (response issueResponse) issue() Issue {
	labels := make([]string, 0, len(response.Labels))
	for _, label := range response.Labels {
		labels = append(labels, label.Name)
	}
	return Issue{
		Index: response.Number, URL: response.HTMLURL, Title: response.Title,
		Description: response.Body, State: response.State, Labels: labels,
	}
}

var _ Client = HTTPClient{}

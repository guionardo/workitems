package github

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

// APIError is a safe GitHub HTTP error that does not expose response bodies.
type APIError struct {
	Status int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("GitHub request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with the GitHub REST API.
type HTTPClient struct {
	BaseURL string
	Token   string
	Owner   string
	Repo    string
	HTTP    *http.Client
}

// NewHTTPClient creates a GitHub issue client for one repository.
func NewHTTPClient(baseURL, token, owner, repo string) HTTPClient {
	return HTTPClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:   strings.TrimSpace(token),
		Owner:   strings.TrimSpace(owner),
		Repo:    strings.TrimSpace(repo),
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// GetIssue retrieves a GitHub issue by number.
func (c HTTPClient) GetIssue(ctx context.Context, number int) (Issue, error) {
	if number < 1 {
		return Issue{}, errors.New("github issue number is required")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodGet, "/issues/"+strconv.Itoa(number), nil, nil, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// ListIssues lists issues in the configured repository.
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
	if strings.Contains(header.Get("Link"), `rel="next"`) {
		pageResult.NextPageToken = strconv.Itoa(page + 1)
	}
	return pageResult, nil
}

// CreateIssue creates a GitHub issue with the configured repository.
func (c HTTPClient) CreateIssue(ctx context.Context, input workitem.CreateTaskInput) (Issue, error) {
	var response issueResponse
	if err := c.do(ctx, http.MethodPost, "/issues", nil, map[string]any{
		"title": input.Title, "body": input.Description, "labels": input.Tags,
	}, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

// SetIssueState opens or closes a GitHub issue.
func (c HTTPClient) SetIssueState(ctx context.Context, number int, state string) (Issue, error) {
	if state != "open" && state != "closed" {
		return Issue{}, errors.New("github issue state must be open or closed")
	}
	var response issueResponse
	if err := c.do(ctx, http.MethodPatch, "/issues/"+strconv.Itoa(number), nil, map[string]string{"state": state}, &response); err != nil {
		return Issue{}, err
	}
	return response.issue(), nil
}

func (c HTTPClient) do(ctx context.Context, method, endpoint string, query url.Values, body, target any) error {
	_, err := c.doWithHeader(ctx, method, endpoint, query, body, target)
	return err
}

func (c HTTPClient) doWithHeader(ctx context.Context, method, endpoint string, query url.Values, body, target any) (http.Header, error) {
	if c.BaseURL == "" || c.Token == "" || c.Owner == "" || c.Repo == "" {
		return nil, errors.New("github client credentials and repository are required")
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("github base URL is invalid")
	}
	relative, err := url.Parse("/repos/" + url.PathEscape(c.Owner) + "/" + url.PathEscape(c.Repo) + endpoint)
	if err != nil {
		return nil, errors.New("github endpoint is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + relative.Path
	base.RawQuery = query.Encode()
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal GitHub request: %w", err)
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, base.String(), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create GitHub request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GitHub request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return response.Header, &APIError{Status: response.StatusCode}
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return response.Header, nil
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return response.Header, fmt.Errorf("read GitHub response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return response.Header, errors.New("GitHub response exceeds size limit")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return response.Header, fmt.Errorf("decode GitHub response: %w", err)
	}
	return response.Header, nil
}

type issueResponse struct {
	NodeID      string `json:"node_id"`
	Number      int    `json:"number"`
	HTMLURL     string `json:"html_url"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
	PullRequest *any   `json:"pull_request"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func (response issueResponse) issue() Issue {
	labels := make([]string, 0, len(response.Labels))
	for _, label := range response.Labels {
		labels = append(labels, label.Name)
	}
	return Issue{
		NodeID: response.NodeID, Number: response.Number, URL: response.HTMLURL, Title: response.Title,
		Body: response.Body, State: response.State, Labels: labels,
		PullRequest: response.PullRequest != nil,
	}
}

var _ Client = HTTPClient{}

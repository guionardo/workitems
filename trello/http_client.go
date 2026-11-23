package trello

import (
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

// APIError is a safe Trello HTTP error that does not expose response bodies.
type APIError struct{ Status int }

func (e *APIError) Error() string {
	return fmt.Sprintf("Trello request failed with HTTP status %d", e.Status)
}

// HTTPClient implements Client with the Trello REST API.
type HTTPClient struct {
	BaseURL string
	APIKey  string
	Token   string
	BoardID string
	HTTP    *http.Client
}

// NewHTTPClient creates a Trello client for one board.
func NewHTTPClient(baseURL, apiKey, token, boardID string) HTTPClient {
	return HTTPClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		APIKey:  strings.TrimSpace(apiKey), Token: strings.TrimSpace(token),
		BoardID: strings.TrimSpace(boardID), HTTP: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetCard retrieves a Trello card by ID or short link.
func (c HTTPClient) GetCard(ctx context.Context, id string) (Card, error) {
	if strings.TrimSpace(id) == "" {
		return Card{}, errors.New("trello card ID is required")
	}
	var response cardResponse
	if err := c.do(ctx, http.MethodGet, "/cards/"+url.PathEscape(id), url.Values{"fields": {"id,name,desc,idList,url,labels"}}, &response); err != nil {
		return Card{}, err
	}
	return c.card(ctx, response)
}

// ListCards lists all active cards on the configured Trello board.
func (c HTTPClient) ListCards(ctx context.Context) ([]Card, error) {
	var responses []cardResponse
	if err := c.do(ctx, http.MethodGet, "/boards/"+url.PathEscape(c.BoardID)+"/cards", url.Values{"fields": {"id,name,desc,idList,url,labels"}}, &responses); err != nil {
		return nil, err
	}
	return c.cards(ctx, responses)
}

// CreateCard creates a Trello card in a list.
func (c HTTPClient) CreateCard(ctx context.Context, listID string, input workitem.CreateTaskInput) (Card, error) {
	if strings.TrimSpace(listID) == "" {
		return Card{}, errors.New("trello list ID is required")
	}
	labels, err := c.labelIDs(ctx, input.Tags)
	if err != nil {
		return Card{}, err
	}
	query := url.Values{
		"idList": {listID}, "name": {input.Title}, "desc": {input.Description},
	}
	if len(labels) > 0 {
		query.Set("idLabels", strings.Join(labels, ","))
	}
	var response cardResponse
	if err := c.do(ctx, http.MethodPost, "/cards", query, &response); err != nil {
		return Card{}, err
	}
	return c.card(ctx, response)
}

// MoveCard moves a Trello card to another list.
func (c HTTPClient) MoveCard(ctx context.Context, id, listID string) (Card, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(listID) == "" {
		return Card{}, errors.New("trello card and list IDs are required")
	}
	var response cardResponse
	if err := c.do(ctx, http.MethodPut, "/cards/"+url.PathEscape(id), url.Values{"idList": {listID}, "fields": {"id,name,desc,idList,url,labels"}}, &response); err != nil {
		return Card{}, err
	}
	return c.card(ctx, response)
}

func (c HTTPClient) cards(ctx context.Context, responses []cardResponse) ([]Card, error) {
	listNames, err := c.listNames(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Card, 0, len(responses))
	for _, response := range responses {
		result = append(result, response.card(listNames))
	}
	return result, nil
}

func (c HTTPClient) card(ctx context.Context, response cardResponse) (Card, error) {
	listNames, err := c.listNames(ctx)
	if err != nil {
		return Card{}, err
	}
	return response.card(listNames), nil
}

func (c HTTPClient) listNames(ctx context.Context) (map[string]string, error) {
	var lists []listResponse
	if err := c.do(ctx, http.MethodGet, "/boards/"+url.PathEscape(c.BoardID)+"/lists", url.Values{"fields": {"id,name"}}, &lists); err != nil {
		return nil, err
	}
	names := make(map[string]string, len(lists))
	for _, list := range lists {
		names[list.ID] = list.Name
	}
	return names, nil
}

func (c HTTPClient) labelIDs(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	var labels []labelResponse
	if err := c.do(ctx, http.MethodGet, "/boards/"+url.PathEscape(c.BoardID)+"/labels", url.Values{"limit": {"100"}}, &labels); err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(labels))
	for _, label := range labels {
		byName[label.Name] = label.ID
	}
	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("Trello card label %q was not found", name)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c HTTPClient) do(ctx context.Context, method, endpoint string, query url.Values, target any) error {
	if c.BaseURL == "" || c.APIKey == "" || c.Token == "" || c.BoardID == "" {
		return errors.New("trello client credentials and board are required")
	}
	base, err := url.Parse(c.BaseURL + endpoint)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return errors.New("trello base URL is invalid")
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("key", c.APIKey)
	query.Set("token", c.Token)
	base.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, method, base.String(), nil)
	if err != nil {
		return fmt.Errorf("create Trello request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Trello request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.CopyN(io.Discard, response.Body, 4096)
		return &APIError{Status: response.StatusCode}
	}
	if target == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read Trello response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return errors.New("Trello response exceeds size limit")
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("decode Trello response: %w", err)
	}
	return nil
}

type cardResponse struct {
	ID     string          `json:"id"`
	URL    string          `json:"url"`
	Name   string          `json:"name"`
	Desc   string          `json:"desc"`
	ListID string          `json:"idList"`
	Labels []labelResponse `json:"labels"`
}

type labelResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type listResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (response cardResponse) card(listNames map[string]string) Card {
	return Card{
		ID: response.ID, URL: response.URL, Title: response.Name, Description: response.Desc,
		List:   workitem.ProviderStatus{ID: response.ListID, Name: listNames[response.ListID]},
		Labels: labels(response.Labels),
	}
}

func labels(items []labelResponse) []Label {
	result := make([]Label, 0, len(items))
	for _, item := range items {
		result = append(result, Label{ID: item.ID, Name: item.Name})
	}
	return result
}

var _ Client = HTTPClient{}

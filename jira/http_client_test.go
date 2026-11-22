package jira

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/guionardo/workitems"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func testHTTPClient(fn roundTripFunc) HTTPClient {
	client := NewHTTPClient("https://jira.example.test", "user@example.com", "token", "APP")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestHTTPClientGetIssue(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/rest/api/3/issue/APP-1" || request.URL.Query().Get("fields") == "" {
			t.Fatalf("request path/query = %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		user, token, ok := request.BasicAuth()
		if !ok || user != "user@example.com" || token != "token" {
			t.Fatalf("unexpected authorization")
		}
		return response(http.StatusOK, `{"id":"1","key":"APP-1","fields":{"summary":"Test","description":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Details"}]}]},"labels":["bug"],"status":{"id":"10001","name":"In Progress"}}}`), nil
	})

	issue, err := client.GetIssue(context.Background(), "APP-1")
	if err != nil {
		t.Fatalf("HTTPClient.GetIssue() error = %v", err)
	}
	if issue.Key != "APP-1" || issue.Description != "Details" || issue.Status.Name != "In Progress" {
		t.Errorf("HTTPClient.GetIssue() = %+v", issue)
	}
}

func TestHTTPClientSearchIssuesUsesEnhancedJQL(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/rest/api/3/search/jql" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		var body struct {
			JQL string `json:"jql"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body.JQL, `project = "APP"`) || !strings.Contains(body.JQL, `labels in ("bug")`) {
			t.Errorf("JQL = %q", body.JQL)
		}
		return response(http.StatusOK, `{"issues":[],"nextPageToken":"next"}`), nil
	})

	page, err := client.SearchIssues(context.Background(), SearchOptions{Labels: []string{"bug"}})
	if err != nil {
		t.Fatalf("HTTPClient.SearchIssues() error = %v", err)
	}
	if page.NextPageToken != "next" {
		t.Errorf("HTTPClient.SearchIssues() next token = %q, want %q", page.NextPageToken, "next")
	}
}

func TestHTTPClientTransitionIssue(t *testing.T) {
	t.Parallel()
	requests := 0
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		requests++
		switch request.URL.Path {
		case "/rest/api/3/issue/APP-1/transitions":
			if request.Method != http.MethodPost {
				t.Fatalf("transition method = %s", request.Method)
			}
			return response(http.StatusNoContent, ""), nil
		case "/rest/api/3/issue/APP-1":
			return response(http.StatusOK, `{"id":"1","key":"APP-1","fields":{"summary":"Test","labels":[],"status":{"id":"10002","name":"Done"}}}`), nil
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
			return nil, errors.New("unreachable")
		}
	})

	issue, err := client.TransitionIssue(context.Background(), "APP-1", "31")
	if err != nil {
		t.Fatalf("HTTPClient.TransitionIssue() error = %v", err)
	}
	if issue.Status.ID != "10002" || requests != 2 {
		t.Errorf("HTTPClient.TransitionIssue() = %+v, requests=%d", issue, requests)
	}
}

func TestHTTPClientErrorsDoNotExposeResponseBody(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, "secret response body"), nil
	})

	_, err := client.GetIssue(context.Background(), "APP-1")
	if err == nil || strings.Contains(err.Error(), "secret response body") {
		t.Errorf("HTTPClient.GetIssue() error = %v, must not expose response body", err)
	}
}

var _ workitem.TaskReader = Adapter{}

package gitea

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/guionardo/workitems"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestHTTPClientCreateIssueResolvesLabels(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "token gitea-token" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		if request.URL.Path == "/api/v1/repos/acme/app/labels" {
			return response(http.StatusOK, `[{"id":7,"name":"bug"}]`), nil
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		labels, ok := body["labels"].([]any)
		if !ok || len(labels) != 1 || labels[0] != float64(7) {
			t.Fatalf("labels = %#v", body["labels"])
		}
		return response(http.StatusCreated, `{"number":4,"html_url":"https://gitea.test/acme/app/issues/4","title":"Created","state":"open","labels":[{"id":7,"name":"bug"}]}`), nil
	})
	issue, err := client.CreateIssue(context.Background(), workitem.CreateTaskInput{Title: "Created", Tags: []string{"bug"}})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if issue.Index != 4 || issue.Labels[0] != "bug" {
		t.Fatalf("CreateIssue() = %+v", issue)
	}
}

func TestHTTPClientListIssues(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("state") != "open" || request.URL.Query().Get("labels") != "bug" {
			t.Fatalf("query = %s", request.URL.RawQuery)
		}
		return response(http.StatusOK, `[{"number":4,"title":"Task","state":"open","labels":[]}]`), nil
	})
	page, err := client.ListIssues(context.Background(), ListOptions{State: "open", Labels: []string{"bug"}, Page: 1})
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if len(page.Issues) != 1 || page.Issues[0].Index != 4 {
		t.Fatalf("ListIssues() = %+v", page)
	}
}

func TestHTTPClientSetIssueState(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["state"] != "closed" {
			t.Fatalf("state = %#v", body["state"])
		}
		return response(http.StatusCreated, `{"number":4,"title":"Task","state":"closed","labels":[]}`), nil
	})
	issue, err := client.SetIssueState(context.Background(), 4, "closed")
	if err != nil {
		t.Fatalf("SetIssueState() error = %v", err)
	}
	if issue.State != "closed" {
		t.Fatalf("SetIssueState() = %+v", issue)
	}
}

func testHTTPClient(fn roundTripFunc) HTTPClient {
	client := NewHTTPClient("https://gitea.test", "gitea-token", "acme", "app")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

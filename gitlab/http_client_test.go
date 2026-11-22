package gitlab

import (
	"context"
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

func TestHTTPClientCreateIssue(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.EscapedPath() != "/api/v4/projects/acme%2Fapp/issues" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("PRIVATE-TOKEN") != "token" {
			t.Fatalf("missing GitLab token header")
		}
		return response(http.StatusCreated, `{"iid":8,"web_url":"https://gitlab.test/acme/app/-/issues/8","title":"Created","state":"opened","labels":["test"]}`), nil
	})

	issue, err := client.CreateIssue(context.Background(), testInput())
	if err != nil {
		t.Fatalf("HTTPClient.CreateIssue() error = %v", err)
	}
	if issue.IID != 8 || issue.State != "opened" || len(issue.Labels) != 1 {
		t.Errorf("HTTPClient.CreateIssue() = %+v", issue)
	}
}

func TestHTTPClientListIssues(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Query().Get("state") != "opened" || request.URL.Query().Get("labels") != "bug" {
			t.Fatalf("unexpected query: %s", request.URL.RawQuery)
		}
		response := response(http.StatusOK, `[{"iid":1,"title":"Bug","state":"opened"}]`)
		response.Header.Set("X-Next-Page", "2")
		return response, nil
	})

	page, err := client.ListIssues(context.Background(), ListOptions{State: "opened", Labels: []string{"bug"}, Page: 1})
	if err != nil {
		t.Fatalf("HTTPClient.ListIssues() error = %v", err)
	}
	if len(page.Issues) != 1 || page.NextPageToken != "2" {
		t.Errorf("HTTPClient.ListIssues() = %+v", page)
	}
}

func TestHTTPClientDoesNotExposeResponseBody(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(*http.Request) (*http.Response, error) {
		return response(http.StatusForbidden, "secret response"), nil
	})

	_, err := client.GetIssue(context.Background(), 1)
	if err == nil || strings.Contains(err.Error(), "secret response") {
		t.Errorf("HTTPClient.GetIssue() error = %v", err)
	}
}

func testHTTPClient(fn roundTripFunc) HTTPClient {
	client := NewHTTPClient("https://gitlab.test", "token", "acme/app")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func testInput() workitem.CreateTaskInput {
	return workitem.CreateTaskInput{Title: "Created", Description: "Details", Tags: []string{"test"}}
}

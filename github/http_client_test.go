package github

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

func TestHTTPClientGetIssue(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/repos/acme/app/issues/7" || request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		return response(http.StatusOK, `{"number":7,"html_url":"https://github.com/acme/app/issues/7","title":"Bug","body":"Details","state":"open","labels":[{"name":"bug"}]}`), nil
	})

	issue, err := client.GetIssue(context.Background(), 7)
	if err != nil {
		t.Fatalf("HTTPClient.GetIssue() error = %v", err)
	}
	if issue.Number != 7 || issue.Title != "Bug" || len(issue.Labels) != 1 {
		t.Errorf("HTTPClient.GetIssue() = %+v", issue)
	}
}

func TestHTTPClientCreateIssue(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/repos/acme/app/issues" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		return response(http.StatusCreated, `{"number":8,"title":"Created","state":"open"}`), nil
	})

	issue, err := client.CreateIssue(context.Background(), workitem.CreateTaskInput{Title: "Created", Tags: []string{"test"}})
	if err != nil {
		t.Fatalf("HTTPClient.CreateIssue() error = %v", err)
	}
	if issue.Number != 8 || issue.State != "open" {
		t.Errorf("HTTPClient.CreateIssue() = %+v", issue)
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
	client := NewHTTPClient("https://api.github.test", "token", "acme", "app")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

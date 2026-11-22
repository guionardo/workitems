package linear

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/guionardo/workitems"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestHTTPClientCreateIssue(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "linear-token" {
			t.Fatalf("missing Linear authorization")
		}
		return response(http.StatusOK, `{"data":{"issueCreate":{"success":true,"issue":{"id":"id-1","identifier":"ENG-1","title":"Created","state":{"id":"todo","name":"Todo"},"labels":{"nodes":[]}}}}}`), nil
	})
	issue, err := client.CreateIssue(context.Background(), workitem.CreateTaskInput{Title: "Created"})
	if err != nil {
		t.Fatalf("HTTPClient.CreateIssue() error = %v", err)
	}
	if issue.Identifier != "ENG-1" || issue.State.ID != "todo" {
		t.Errorf("HTTPClient.CreateIssue() = %+v", issue)
	}
}

func TestHTTPClientListIssues(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"data":{"team":{"issues":{"nodes":[{"id":"id-1","identifier":"ENG-1","title":"Task","state":{"id":"doing","name":"In Progress"},"labels":{"nodes":[{"id":"label-1","name":"bug"}]}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`), nil
	})
	page, err := client.ListIssues(context.Background(), "")
	if err != nil {
		t.Fatalf("HTTPClient.ListIssues() error = %v", err)
	}
	if len(page.Issues) != 1 || page.Issues[0].State.ID != "doing" {
		t.Errorf("HTTPClient.ListIssues() = %+v", page)
	}
}

func testHTTPClient(fn roundTripFunc) HTTPClient {
	client := NewHTTPClient("https://api.linear.test/graphql", "linear-token", "team-1")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

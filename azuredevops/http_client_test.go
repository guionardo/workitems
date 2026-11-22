package azuredevops

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/guionardo/workitems"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestHTTPClientCreateWorkItem(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/org/project/_apis/wit/workitems/$Task" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(":token"))
		if request.Header.Get("Authorization") != wantAuth || request.URL.Query().Get("api-version") != "7.1" {
			t.Fatalf("unexpected authorization or version")
		}
		return response(http.StatusOK, `{"id":8,"url":"https://dev.azure.com/org/project/_apis/wit/workItems/8","fields":{"System.Title":"Created","System.State":"New","System.Tags":"test; docs"}}`), nil
	})

	item, err := client.CreateWorkItem(context.Background(), workitem.CreateTaskInput{Title: "Created", Tags: []string{"test", "docs"}})
	if err != nil {
		t.Fatalf("HTTPClient.CreateWorkItem() error = %v", err)
	}
	if item.ID != 8 || len(item.Tags) != 2 {
		t.Errorf("HTTPClient.CreateWorkItem() = %+v", item)
	}
}

func TestHTTPClientListWorkItems(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/org/project/_apis/wit/wiql":
			return response(http.StatusOK, `{"workItems":[{"id":7}]}`), nil
		case "/org/project/_apis/wit/workitems":
			return response(http.StatusOK, `{"value":[{"id":7,"fields":{"System.Title":"Task","System.State":"Active"}}]}`), nil
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
			return nil, nil
		}
	})

	page, err := client.ListWorkItems(context.Background(), ListOptions{PageSize: 10})
	if err != nil {
		t.Fatalf("HTTPClient.ListWorkItems() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].State.ID != "Active" {
		t.Errorf("HTTPClient.ListWorkItems() = %+v", page)
	}
}

func testHTTPClient(fn roundTripFunc) HTTPClient {
	client := NewHTTPClient("https://dev.azure.com", "org", "project", "token", "Task")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

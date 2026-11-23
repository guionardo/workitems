package trello

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

func TestHTTPClientCreateCard(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		query := request.URL.Query()
		if query.Get("key") != "api-key" || query.Get("token") != "token" {
			t.Fatalf("credentials missing from query")
		}
		if request.URL.Path == "/1/boards/board-1/labels" {
			return response(http.StatusOK, `[{"id":"label-1","name":"bug"}]`), nil
		}
		if request.URL.Path == "/1/boards/board-1/lists" {
			return response(http.StatusOK, `[{"id":"list-todo","name":"To Do"}]`), nil
		}
		if request.URL.Path != "/1/cards" || query.Get("idList") != "list-todo" || query.Get("idLabels") != "label-1" {
			t.Fatalf("create query = %s", request.URL.RawQuery)
		}
		return response(http.StatusOK, `{"id":"card-1","url":"https://trello.test/c/card-1","name":"Created","desc":"Details","idList":"list-todo","labels":[{"id":"label-1","name":"bug"}]}`), nil
	})
	card, err := client.CreateCard(context.Background(), "list-todo", workitem.CreateTaskInput{Title: "Created", Description: "Details", Tags: []string{"bug"}})
	if err != nil {
		t.Fatalf("CreateCard() error = %v", err)
	}
	if card.ID != "card-1" || card.List.ID != "list-todo" || card.Labels[0].Name != "bug" {
		t.Fatalf("CreateCard() = %+v", card)
	}
}

func TestHTTPClientMoveCard(t *testing.T) {
	t.Parallel()
	client := testHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/1/boards/board-1/lists" {
			return response(http.StatusOK, `[{"id":"list-done","name":"Done"}]`), nil
		}
		if request.URL.Path != "/1/cards/card-1" || request.URL.Query().Get("idList") != "list-done" {
			t.Fatalf("move query = %s", request.URL.RawQuery)
		}
		return response(http.StatusOK, `{"id":"card-1","name":"Task","idList":"list-done","labels":[]}`), nil
	})
	card, err := client.MoveCard(context.Background(), "card-1", "list-done")
	if err != nil {
		t.Fatalf("MoveCard() error = %v", err)
	}
	if card.List.Name != "Done" {
		t.Fatalf("MoveCard() = %+v", card)
	}
}

func testHTTPClient(fn roundTripFunc) HTTPClient {
	client := NewHTTPClient("https://trello.test/1", "api-key", "token", "board-1")
	client.HTTP = &http.Client{Transport: fn}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

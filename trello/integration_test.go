package trello

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestTrelloIntegrationCreateAndMove(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("TRELLO_API_KEY"))
	token := strings.TrimSpace(os.Getenv("TRELLO_TOKEN"))
	boardID := strings.TrimSpace(os.Getenv("TRELLO_BOARD_ID"))
	todoList := strings.TrimSpace(os.Getenv("TRELLO_LIST_TODO"))
	doneList := strings.TrimSpace(os.Getenv("TRELLO_LIST_DONE"))
	if apiKey == "" || token == "" || boardID == "" || todoList == "" || doneList == "" {
		t.Skip("TRELLO_API_KEY, TRELLO_TOKEN, TRELLO_BOARD_ID, TRELLO_LIST_TODO, and TRELLO_LIST_DONE are required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("TRELLO_HOST")), "/")
	if baseURL == "" {
		baseURL = "https://api.trello.com/1"
	}
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: todoList, Name: "To Do"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: doneList, Name: "Done"}, Status: workitem.StatusDone},
	})
	adapter := NewAdapter(NewHTTPClient(baseURL, apiKey, token, boardID), mapper)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	task, err := adapter.CreateTask(ctx, workitem.CreateTaskInput{
		Title:       fmt.Sprintf("workitems integration %s", time.Now().UTC().Format(time.RFC3339Nano)),
		Description: "Created by the workitems Trello integration test.",
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if task.Ref.ID == "" || task.Status != workitem.StatusTodo {
		t.Fatalf("CreateTask() = %+v", task)
	}

	moved, err := adapter.MoveTask(ctx, task.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("MoveTask() error = %v", err)
	}
	if moved.Status != workitem.StatusDone || moved.ProviderStatus.ID != doneList {
		t.Fatalf("MoveTask() = %+v", moved)
	}
}

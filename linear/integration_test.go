package linear

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestLinearIntegrationCreateAndMove(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("LINEAR_API_KEY"))
	teamID := strings.TrimSpace(os.Getenv("LINEAR_TEAM_ID"))
	todoID := strings.TrimSpace(os.Getenv("LINEAR_STATE_TODO"))
	doneID := strings.TrimSpace(os.Getenv("LINEAR_STATE_DONE"))
	if apiKey == "" || teamID == "" || todoID == "" || doneID == "" {
		t.Skip("LINEAR_API_KEY, LINEAR_TEAM_ID, LINEAR_STATE_TODO, and LINEAR_STATE_DONE are required")
	}
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: todoID, Name: "Todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: doneID, Name: "Done"}, Status: workitem.StatusDone},
	})
	adapter := NewAdapter(NewHTTPClient("https://api.linear.app/graphql", apiKey, teamID), mapper)
	created, err := adapter.CreateTask(context.Background(), workitem.CreateTaskInput{Title: fmt.Sprintf("[workitems integration test] %s", time.Now().UTC().Format(time.RFC3339)), Description: "Created by the workitems Linear integration test."})
	if err != nil {
		t.Fatalf("Linear create failed: %v", err)
	}
	moved, err := adapter.MoveTask(context.Background(), created.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Linear move failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("Linear move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}
}

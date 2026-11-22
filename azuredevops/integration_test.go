package azuredevops

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestAzureDevOpsIntegrationCreateAndMove(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("AZDO_PAT"))
	organization := strings.TrimSpace(os.Getenv("AZDO_ORGANIZATION"))
	project := strings.TrimSpace(os.Getenv("AZDO_PROJECT"))
	if token == "" || organization == "" || project == "" {
		t.Skip("AZDO_PAT, AZDO_ORGANIZATION, and AZDO_PROJECT are required")
	}
	baseURL := strings.TrimSpace(os.Getenv("AZDO_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://dev.azure.com"
	}
	stateTodo := valueOr("AZDO_STATE_TODO", "New")
	stateInProgress := valueOr("AZDO_STATE_IN_PROGRESS", "Active")
	stateDone := valueOr("AZDO_STATE_DONE", "Done")
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: stateTodo, Name: stateTodo}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: stateInProgress, Name: stateInProgress}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: stateDone, Name: stateDone}, Status: workitem.StatusDone},
	})
	client := NewHTTPClient(baseURL, organization, project, token, os.Getenv("AZDO_WORK_ITEM_TYPE"))
	adapter := NewAdapter(client, mapper)
	created, err := adapter.CreateTask(context.Background(), workitem.CreateTaskInput{
		Title:       fmt.Sprintf("[workitems integration test] %s", time.Now().UTC().Format(time.RFC3339)),
		Description: "Created by the workitems Azure DevOps integration test.",
		Tags:        []string{"workitems-integration-test"},
	})
	if err != nil {
		t.Fatalf("Azure DevOps create failed: %v", err)
	}
	moved, err := adapter.MoveTask(context.Background(), created.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Azure DevOps move failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("Azure DevOps move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}
}

func valueOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

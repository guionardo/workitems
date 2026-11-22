package jira

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestJiraIntegrationCreateAndMove(t *testing.T) {
	host := strings.TrimSpace(os.Getenv("JIRA_HOST"))
	email := strings.TrimSpace(os.Getenv("JIRA_EMAIL"))
	token := strings.TrimSpace(os.Getenv("JIRA_API_TOKEN"))
	project := strings.TrimSpace(os.Getenv("JIRA_PROJECT"))
	if host == "" || email == "" || token == "" || project == "" {
		t.Skip("JIRA_HOST, JIRA_EMAIL, JIRA_API_TOKEN, and JIRA_PROJECT are required")
	}

	client := NewHTTPClient(host, email, token, project)
	created, err := client.CreateIssue(context.Background(), workitem.CreateTaskInput{
		Title:       fmt.Sprintf("[workitems integration test] %s", time.Now().UTC().Format(time.RFC3339)),
		Description: "Created by the workitems Jira integration test.",
		Tags:        []string{"workitems-integration-test"},
	})
	if err != nil {
		t.Fatalf("Jira create failed: %v", err)
	}

	transitions, err := client.ListTransitions(context.Background(), created.Key)
	if err != nil {
		t.Fatalf("Jira transition lookup failed: %v", err)
	}
	if len(transitions) == 0 {
		t.Log("Jira issue was created, but no transition is available for this account")
		return
	}

	mappings := []workitem.StatusMapping{{ProviderStatus: created.Status, Status: workitem.StatusTodo}}
	var target workitem.ProviderStatus
	for _, transition := range transitions {
		if transition.To.ID != "" && transition.To.ID != created.Status.ID {
			target = transition.To
			mappings = append(mappings, workitem.StatusMapping{ProviderStatus: target, Status: workitem.StatusDone})
			break
		}
	}
	if target.ID == "" {
		t.Log("Jira issue was created, but all available transitions target the current status")
		return
	}

	adapter := NewAdapter(client, workitem.NewStatusMapper(mappings))
	moved, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: "jira", ID: created.Key}, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Jira move failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("Jira move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}
}

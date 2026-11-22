package github

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestGitHubProjectsIntegrationList(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	projectID := strings.TrimSpace(os.Getenv("GITHUB_PROJECT_ID"))
	statusFieldID := strings.TrimSpace(os.Getenv("GITHUB_STATUS_FIELD_ID"))
	if token == "" || projectID == "" || statusFieldID == "" {
		t.Skip("GITHUB_API_TOKEN, GITHUB_PROJECT_ID, and GITHUB_STATUS_FIELD_ID are required")
	}

	baseURL := strings.TrimSpace(os.Getenv("GITHUB_GRAPHQL_URL"))
	if baseURL == "" {
		baseURL = "https://api.github.com/graphql"
	}
	client := NewProjectHTTPClient(baseURL, token)
	page, err := client.ListProjectItems(context.Background(), projectID, statusFieldID, "")
	if err != nil {
		t.Fatalf("GitHub Projects list failed: %v", err)
	}
	t.Logf("GitHub Projects returned %d item(s); next page token present: %t", len(page.Items), page.NextPageToken != "")
}

func TestGitHubProjectsIntegrationMove(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	projectID := strings.TrimSpace(os.Getenv("GITHUB_PROJECT_ID"))
	statusFieldID := strings.TrimSpace(os.Getenv("GITHUB_STATUS_FIELD_ID"))
	doneOptionID := strings.TrimSpace(os.Getenv("GITHUB_STATUS_DONE_OPTION_ID"))
	if token == "" || projectID == "" || statusFieldID == "" || doneOptionID == "" {
		t.Skip("GitHub Projects credentials and status IDs are required")
	}

	baseURL := strings.TrimSpace(os.Getenv("GITHUB_GRAPHQL_URL"))
	if baseURL == "" {
		baseURL = "https://api.github.com/graphql"
	}
	client := NewProjectHTTPClient(baseURL, token)
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: strings.TrimSpace(os.Getenv("GITHUB_STATUS_TODO_OPTION_ID")), Name: "Todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: strings.TrimSpace(os.Getenv("GITHUB_STATUS_IN_PROGRESS_OPTION_ID")), Name: "In Progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: doneOptionID, Name: "Done"}, Status: workitem.StatusDone},
	})
	adapter := NewProjectAdapter(client, projectID, statusFieldID, mapper)
	page, err := adapter.ListTasks(context.Background(), workitem.ListOptions{Tags: []string{"workitems-integration-test"}})
	if err != nil {
		t.Fatalf("GitHub Projects list through adapter failed: %v", err)
	}
	if len(page.Tasks) == 0 {
		t.Skip("no workitems-integration-test project item is available")
	}

	moved, err := adapter.MoveTask(context.Background(), page.Tasks[0].Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("GitHub Projects move through adapter failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("GitHub Projects move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}

	fetched, err := adapter.GetTask(context.Background(), page.Tasks[0].Ref)
	if err != nil {
		t.Fatalf("GitHub Projects get after move failed: %v", err)
	}
	if fetched.Status != workitem.StatusDone {
		t.Fatalf("GitHub Projects get returned status %q, want %q", fetched.Status, workitem.StatusDone)
	}
}

func TestGitHubProjectsIntegrationCreate(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	owner := strings.TrimSpace(os.Getenv("GITHUB_OWNER"))
	repo := strings.TrimSpace(os.Getenv("GITHUB_REPO"))
	projectID := strings.TrimSpace(os.Getenv("GITHUB_PROJECT_ID"))
	statusFieldID := strings.TrimSpace(os.Getenv("GITHUB_STATUS_FIELD_ID"))
	doneOptionID := strings.TrimSpace(os.Getenv("GITHUB_STATUS_DONE_OPTION_ID"))
	if token == "" || owner == "" || repo == "" || projectID == "" || statusFieldID == "" || doneOptionID == "" {
		t.Skip("GitHub Issues and Projects credentials and status IDs are required")
	}

	graphqlURL := strings.TrimSpace(os.Getenv("GITHUB_GRAPHQL_URL"))
	if graphqlURL == "" {
		graphqlURL = "https://api.github.com/graphql"
	}
	apiURL := strings.TrimSpace(os.Getenv("GITHUB_API_URL"))
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	projectClient := NewProjectHTTPClient(graphqlURL, token)
	issueClient := NewHTTPClient(apiURL, token, owner, repo)
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: strings.TrimSpace(os.Getenv("GITHUB_STATUS_TODO_OPTION_ID")), Name: "Todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: strings.TrimSpace(os.Getenv("GITHUB_STATUS_IN_PROGRESS_OPTION_ID")), Name: "In Progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: doneOptionID, Name: "Done"}, Status: workitem.StatusDone},
	})
	adapter := NewProjectAdapterWithIssueClient(projectClient, issueClient, projectID, statusFieldID, mapper)
	created, err := adapter.CreateTask(context.Background(), workitem.CreateTaskInput{
		Title:       fmt.Sprintf("[workitems Projects adapter test] %s", time.Now().UTC().Format(time.RFC3339)),
		Description: "Created by the workitems GitHub Projects adapter integration test.",
		Tags:        []string{"workitems-integration-test"},
	})
	if err != nil {
		t.Fatalf("GitHub Projects create failed: %v", err)
	}

	moved, err := adapter.MoveTask(context.Background(), created.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("GitHub Projects move after create failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("GitHub Projects move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}
}

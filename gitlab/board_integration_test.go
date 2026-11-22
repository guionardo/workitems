package gitlab

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestGitLabBoardIntegrationCreateAndMove(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITLAB_API_TOKEN"))
	project := strings.TrimSpace(os.Getenv("GITLAB_PROJECT"))
	boardID, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("GITLAB_BOARD_ID")))
	if token == "" || project == "" || boardID < 1 {
		t.Skip("GITLAB_API_TOKEN, GITLAB_PROJECT, and GITLAB_BOARD_ID are required")
	}
	host := strings.TrimSpace(os.Getenv("GITLAB_HOST"))
	if host == "" {
		host = "https://gitlab.com"
	}
	client := NewHTTPClient(host, token, project)
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "label:todo", Name: "todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "label:in-progress", Name: "in-progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: "label:done", Name: "done"}, Status: workitem.StatusDone},
	})
	adapter := NewBoardAdapter(client, client, client, boardID, mapper)
	created, err := client.CreateIssue(context.Background(), workitem.CreateTaskInput{
		Title:       fmt.Sprintf("[workitems board integration test] %s", time.Now().UTC().Format(time.RFC3339)),
		Description: "Created by the workitems GitLab board integration test.",
		Tags:        []string{"todo", "workitems-integration-test"},
	})
	if err != nil {
		t.Fatalf("GitLab board create failed: %v", err)
	}
	ref := workitem.TaskRef{Provider: Provider, ID: strconv.Itoa(created.IID), URL: created.URL}
	for _, target := range []workitem.Status{workitem.StatusInProgress, workitem.StatusDone} {
		task, moveErr := adapter.MoveTask(context.Background(), ref, target)
		if moveErr != nil {
			t.Fatalf("GitLab board move to %q failed: %v", target, moveErr)
		}
		if task.Status != target {
			t.Fatalf("GitLab board move returned status %q, want %q", task.Status, target)
		}
	}
	readMapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "label:todo", Name: "todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "label:in-progress", Name: "in-progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: "label:done", Name: "done"}, Status: workitem.StatusDone},
		{ProviderStatus: workitem.ProviderStatus{ID: "open", Name: "Open"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "closed", Name: "Closed"}, Status: workitem.StatusDone},
	})
	readAdapter := NewBoardAdapter(client, client, client, boardID, readMapper)
	page, err := readAdapter.ListTasks(context.Background(), workitem.ListOptions{Tags: []string{"workitems-integration-test"}})
	if err != nil {
		t.Fatalf("GitLab board list after move failed: %v", err)
	}
	found := false
	for _, task := range page.Tasks {
		if task.Ref.ID == ref.ID && task.Status == workitem.StatusDone {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("GitLab board list did not return moved issue %s with done status", ref.ID)
	}
}

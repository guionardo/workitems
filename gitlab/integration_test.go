package gitlab

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestGitLabIntegrationCreateAndMove(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITLAB_API_TOKEN"))
	project := strings.TrimSpace(os.Getenv("GITLAB_PROJECT"))
	if token == "" || project == "" {
		t.Skip("GITLAB_API_TOKEN and GITLAB_PROJECT are required")
	}
	host := strings.TrimSpace(os.Getenv("GITLAB_HOST"))
	if host == "" {
		host = "https://gitlab.com"
	}
	client := NewHTTPClient(host, token, project)
	adapter := NewAdapter(client)
	created, err := adapter.CreateTask(context.Background(), workitem.CreateTaskInput{
		Title:       fmt.Sprintf("[workitems integration test] %s", time.Now().UTC().Format(time.RFC3339)),
		Description: "Created by the workitems GitLab integration test.",
		Tags:        []string{"workitems-integration-test"},
	})
	if err != nil {
		t.Fatalf("GitLab create failed: %v", err)
	}
	moved, err := adapter.MoveTask(context.Background(), created.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("GitLab move failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("GitLab move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}
}

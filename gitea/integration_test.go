package gitea

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guionardo/workitems"
)

func TestGiteaIntegrationCreateAndMove(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITEA_API_TOKEN"))
	owner := strings.TrimSpace(os.Getenv("GITEA_OWNER"))
	repo := strings.TrimSpace(os.Getenv("GITEA_REPO"))
	if token == "" || owner == "" || repo == "" {
		t.Skip("GITEA_API_TOKEN, GITEA_OWNER, and GITEA_REPO are required")
	}
	host := strings.TrimRight(strings.TrimSpace(os.Getenv("GITEA_HOST")), "/")
	if host == "" {
		host = "https://gitea.com"
	}
	client := NewHTTPClient(host, token, owner, repo)
	adapter := NewAdapter(client)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	issue, err := adapter.CreateTask(ctx, workitem.CreateTaskInput{
		Title:       fmt.Sprintf("workitems integration %s", time.Now().UTC().Format(time.RFC3339Nano)),
		Description: "Created by the workitems Gitea integration test.",
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if issue.Ref.ID == "" || issue.Status != workitem.StatusTodo {
		t.Fatalf("CreateTask() = %+v", issue)
	}

	moved, err := adapter.MoveTask(ctx, issue.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("MoveTask() error = %v", err)
	}
	if moved.Status != workitem.StatusDone || moved.ProviderStatus.ID != "closed" {
		t.Fatalf("MoveTask() = %+v", moved)
	}
}

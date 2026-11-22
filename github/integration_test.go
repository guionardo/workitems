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

func TestGitHubIntegrationCreateAndMove(t *testing.T) {
	token := strings.TrimSpace(os.Getenv("GITHUB_API_TOKEN"))
	owner := strings.TrimSpace(os.Getenv("GITHUB_OWNER"))
	repo := strings.TrimSpace(os.Getenv("GITHUB_REPO"))
	if owner == "" && strings.Count(repo, "/") == 1 {
		parts := strings.SplitN(repo, "/", 2)
		owner, repo = parts[0], parts[1]
	}
	if token == "" || owner == "" || repo == "" {
		t.Skip("GITHUB_API_TOKEN and GITHUB_REPO are required; use owner/repo or set GITHUB_OWNER separately")
	}

	baseURL := strings.TrimSpace(os.Getenv("GITHUB_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	client := NewHTTPClient(baseURL, token, owner, repo)
	adapter := NewAdapter(client)
	created, err := adapter.CreateTask(context.Background(), workitem.CreateTaskInput{
		Title:       fmt.Sprintf("[workitems integration test] %s", time.Now().UTC().Format(time.RFC3339)),
		Description: "Created by the workitems GitHub integration test.",
		Tags:        []string{"workitems-integration-test"},
	})
	if err != nil {
		t.Fatalf("GitHub create failed: %v", err)
	}

	fetched, err := adapter.GetTask(context.Background(), created.Ref)
	if err != nil {
		t.Fatalf("GitHub get failed: %v", err)
	}
	if fetched.Ref.ID != created.Ref.ID {
		t.Fatalf("GitHub get returned ID %q, want %q", fetched.Ref.ID, created.Ref.ID)
	}

	moved, err := adapter.MoveTask(context.Background(), created.Ref, workitem.StatusDone)
	if err != nil {
		t.Fatalf("GitHub move failed: %v", err)
	}
	if moved.Status != workitem.StatusDone {
		t.Fatalf("GitHub move returned status %q, want %q", moved.Status, workitem.StatusDone)
	}
}

package github

import (
	"context"
	"os"
	"strings"
	"testing"
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

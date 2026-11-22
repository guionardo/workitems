package gitea

import (
	"context"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeClient struct {
	issue Issue
}

func (f fakeClient) GetIssue(context.Context, int) (Issue, error) { return f.issue, nil }

func (f fakeClient) ListIssues(context.Context, ListOptions) (IssuePage, error) {
	return IssuePage{Issues: []Issue{f.issue}}, nil
}

func (f fakeClient) CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error) {
	return f.issue, nil
}

func (f fakeClient) SetIssueState(context.Context, int, string) (Issue, error) {
	return f.issue, nil
}

func TestAdapterMapsIssue(t *testing.T) {
	adapter := NewAdapter(fakeClient{issue: Issue{
		Index: 12, URL: "https://gitea.test/acme/app/issues/12", Title: "Fix bug",
		Description: "Details", State: "open", Labels: []string{"bug"},
	}})
	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "12"})
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.Status != workitem.StatusTodo || task.ProviderStatus.ID != "open" || len(task.Tags) != 1 {
		t.Fatalf("GetTask() = %+v", task)
	}
}

func TestAdapterRejectsInProgress(t *testing.T) {
	adapter := NewAdapter(fakeClient{issue: Issue{Index: 1, State: "open"}})
	_, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "1"}, workitem.StatusInProgress)
	if err != ErrStatusUnsupported {
		t.Fatalf("MoveTask() error = %v, want %v", err, ErrStatusUnsupported)
	}
}

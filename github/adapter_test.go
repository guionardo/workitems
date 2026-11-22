package github

import (
	"context"
	"errors"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeClient struct {
	issue Issue
	page  IssuePage
	state string
}

func (f *fakeClient) GetIssue(context.Context, int) (Issue, error)               { return f.issue, nil }
func (f *fakeClient) ListIssues(context.Context, ListOptions) (IssuePage, error) { return f.page, nil }
func (f *fakeClient) CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error) {
	return f.issue, nil
}
func (f *fakeClient) SetIssueState(_ context.Context, _ int, state string) (Issue, error) {
	f.state = state
	f.issue.State = state
	return f.issue, nil
}

func TestAdapterGetTask(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{issue: Issue{Number: 12, Title: "Fix bug", State: "open", Labels: []string{"bug"}}})

	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: "github", ID: "12"})
	if err != nil {
		t.Fatalf("Adapter.GetTask() error = %v", err)
	}
	if task.Ref.ID != "12" || task.Status != workitem.StatusTodo || len(task.Tags) != 1 {
		t.Errorf("Adapter.GetTask() = %+v", task)
	}
}

func TestAdapterListTasksExcludesPullRequests(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{page: IssuePage{Issues: []Issue{
		{Number: 1, Title: "Issue", State: "open"},
		{Number: 2, Title: "Pull request", State: "open", PullRequest: true},
	}}})

	page, err := adapter.ListTasks(context.Background(), workitem.ListOptions{})
	if err != nil {
		t.Fatalf("Adapter.ListTasks() error = %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].Ref.ID != "1" {
		t.Errorf("Adapter.ListTasks() = %+v, want only issue 1", page.Tasks)
	}
}

func TestAdapterMoveTask(t *testing.T) {
	t.Parallel()
	client := &fakeClient{issue: Issue{Number: 3, State: "closed"}}
	adapter := NewAdapter(client)

	task, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: "github", ID: "3"}, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Adapter.MoveTask() error = %v", err)
	}
	if client.state != "closed" || task.Status != workitem.StatusDone {
		t.Errorf("Adapter.MoveTask() state=%q task=%+v", client.state, task)
	}
}

func TestAdapterRejectsPullRequests(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{issue: Issue{Number: 4, State: "open", PullRequest: true}})
	_, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: "github", ID: "4"})
	if !errors.Is(err, ErrPullRequestUnsupported) {
		t.Errorf("Adapter.GetTask() error = %v, want %v", err, ErrPullRequestUnsupported)
	}
}

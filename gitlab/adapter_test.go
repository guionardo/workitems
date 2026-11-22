package gitlab

import (
	"context"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeClient struct {
	issue Issue
	page  IssuePage
	state string
}

func (f *fakeClient) GetIssue(context.Context, int) (Issue, error) {
	return f.issue, nil
}

func (f *fakeClient) ListIssues(context.Context, ListOptions) (IssuePage, error) {
	return f.page, nil
}

func (f *fakeClient) CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error) {
	return f.issue, nil
}

func (f *fakeClient) SetIssueState(_ context.Context, _ int, state string) (Issue, error) {
	f.state = state
	f.issue.State = map[string]string{"reopen": "opened", "close": "closed"}[state]
	return f.issue, nil
}

func TestAdapterGetTask(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{issue: Issue{IID: 12, Title: "Fix bug", State: "opened", Labels: []string{"bug"}}})

	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "12"})
	if err != nil {
		t.Fatalf("Adapter.GetTask() error = %v", err)
	}
	if task.Ref.ID != "12" || task.Status != workitem.StatusTodo || len(task.Tags) != 1 {
		t.Errorf("Adapter.GetTask() = %+v", task)
	}
}

func TestAdapterListTasks(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{page: IssuePage{Issues: []Issue{
		{IID: 1, Title: "Issue", State: "opened", Labels: []string{"bug"}},
		{IID: 2, Title: "Closed", State: "closed"},
	}}})

	status := workitem.StatusTodo
	page, err := adapter.ListTasks(context.Background(), workitem.ListOptions{Status: &status})
	if err != nil {
		t.Fatalf("Adapter.ListTasks() error = %v", err)
	}
	if len(page.Tasks) != 2 {
		t.Errorf("Adapter.ListTasks() returned %d tasks, want 2", len(page.Tasks))
	}
}

func TestAdapterMoveTask(t *testing.T) {
	t.Parallel()
	client := &fakeClient{issue: Issue{IID: 3, State: "opened"}}
	adapter := NewAdapter(client)

	task, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "3"}, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Adapter.MoveTask() error = %v", err)
	}
	if client.state != "close" || task.Status != workitem.StatusDone {
		t.Errorf("Adapter.MoveTask() state=%q task=%+v", client.state, task)
	}
}

func TestAdapterRejectsInProgress(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{issue: Issue{IID: 4, State: "opened"}})
	_, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "4"}, workitem.StatusInProgress)
	if err != ErrStatusUnsupported {
		t.Errorf("Adapter.MoveTask() error = %v, want %v", err, ErrStatusUnsupported)
	}
}

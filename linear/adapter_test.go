package linear

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

func (f *fakeClient) GetIssue(context.Context, string) (Issue, error)       { return f.issue, nil }
func (f *fakeClient) ListIssues(context.Context, string) (IssuePage, error) { return f.page, nil }
func (f *fakeClient) CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error) {
	return f.issue, nil
}
func (f *fakeClient) SetIssueState(_ context.Context, _, stateID string) (Issue, error) {
	f.state = stateID
	f.issue.State.ID = stateID
	return f.issue, nil
}

func linearMapper() workitem.StatusMapper {
	return workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "todo", Name: "Todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "doing", Name: "In Progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: "done", Name: "Done"}, Status: workitem.StatusDone},
	})
}

func TestAdapterGetTask(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{issue: Issue{Identifier: "ENG-1", Title: "Task", State: workitem.ProviderStatus{ID: "todo", Name: "Todo"}}}, linearMapper())
	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "ENG-1"})
	if err != nil {
		t.Fatalf("Adapter.GetTask() error = %v", err)
	}
	if task.Status != workitem.StatusTodo || task.Ref.ID != "ENG-1" {
		t.Errorf("Adapter.GetTask() = %+v", task)
	}
}

func TestAdapterMoveTask(t *testing.T) {
	t.Parallel()
	client := &fakeClient{issue: Issue{Identifier: "ENG-2", State: workitem.ProviderStatus{ID: "todo", Name: "Todo"}}}
	adapter := NewAdapter(client, linearMapper())
	task, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "ENG-2"}, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Adapter.MoveTask() error = %v", err)
	}
	if client.state != "done" || task.Status != workitem.StatusDone {
		t.Errorf("Adapter.MoveTask() state=%q task=%+v", client.state, task)
	}
}

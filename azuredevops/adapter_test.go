package azuredevops

import (
	"context"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeClient struct {
	item  WorkItem
	page  WorkItemPage
	state string
}

func (f *fakeClient) GetWorkItem(context.Context, int) (WorkItem, error) { return f.item, nil }
func (f *fakeClient) ListWorkItems(context.Context, ListOptions) (WorkItemPage, error) {
	return f.page, nil
}
func (f *fakeClient) CreateWorkItem(context.Context, workitem.CreateTaskInput) (WorkItem, error) {
	return f.item, nil
}
func (f *fakeClient) SetWorkItemState(_ context.Context, _ int, state string) (WorkItem, error) {
	f.state = state
	f.item.State = workitem.ProviderStatus{ID: state, Name: state}
	return f.item, nil
}

func mapper() workitem.StatusMapper {
	return workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "New", Name: "New"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "Active", Name: "Active"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: "Done", Name: "Done"}, Status: workitem.StatusDone},
	})
}

func TestAdapterGetTask(t *testing.T) {
	t.Parallel()
	adapter := NewAdapter(&fakeClient{item: WorkItem{ID: 12, Title: "Task", State: workitem.ProviderStatus{ID: "New", Name: "New"}}}, mapper())
	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "12"})
	if err != nil {
		t.Fatalf("Adapter.GetTask() error = %v", err)
	}
	if task.Status != workitem.StatusTodo || task.Ref.ID != "12" {
		t.Errorf("Adapter.GetTask() = %+v", task)
	}
}

func TestAdapterMoveTask(t *testing.T) {
	t.Parallel()
	client := &fakeClient{item: WorkItem{ID: 3, State: workitem.ProviderStatus{ID: "New", Name: "New"}}}
	adapter := NewAdapter(client, mapper())
	task, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "3"}, workitem.StatusInProgress)
	if err != nil {
		t.Fatalf("Adapter.MoveTask() error = %v", err)
	}
	if client.state != "Active" || task.Status != workitem.StatusInProgress {
		t.Errorf("Adapter.MoveTask() state=%q task=%+v", client.state, task)
	}
}

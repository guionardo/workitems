package trello

import (
	"context"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeClient struct {
	card Card
}

func (f fakeClient) GetCard(context.Context, string) (Card, error) { return f.card, nil }

func (f fakeClient) ListCards(context.Context) ([]Card, error) { return []Card{f.card}, nil }

func (f fakeClient) CreateCard(context.Context, string, workitem.CreateTaskInput) (Card, error) {
	return f.card, nil
}

func (f fakeClient) MoveCard(context.Context, string, string) (Card, error) { return f.card, nil }

func trelloMapper() workitem.StatusMapper {
	return workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "list-todo", Name: "To Do"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "list-done", Name: "Done"}, Status: workitem.StatusDone},
	})
}

func TestAdapterMapsCard(t *testing.T) {
	adapter := NewAdapter(fakeClient{card: Card{
		ID: "card-1", URL: "https://trello.test/c/card-1", Title: "Fix bug",
		Description: "Details", List: workitem.ProviderStatus{ID: "list-todo", Name: "To Do"},
		Labels: []Label{{ID: "label-1", Name: "bug"}},
	}}, trelloMapper())
	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "card-1"})
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if task.Status != workitem.StatusTodo || task.ProviderStatus.Name != "To Do" || len(task.Tags) != 1 {
		t.Fatalf("GetTask() = %+v", task)
	}
}

func TestAdapterRejectsAmbiguousMove(t *testing.T) {
	mapper := workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "list-a"}, Status: workitem.StatusDone},
		{ProviderStatus: workitem.ProviderStatus{ID: "list-b"}, Status: workitem.StatusDone},
	})
	adapter := NewAdapter(fakeClient{card: Card{List: workitem.ProviderStatus{ID: "list-a"}}}, mapper)
	_, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: Provider, ID: "card-1"}, workitem.StatusDone)
	if err != ErrStatusUnsupported {
		t.Fatalf("MoveTask() error = %v, want %v", err, ErrStatusUnsupported)
	}
}

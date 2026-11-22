package jira

import (
	"context"
	"errors"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeClient struct {
	issue       Issue
	page        IssuePage
	transitions []Transition
	created     workitem.CreateTaskInput
	transition  string
}

func (f *fakeClient) GetIssue(context.Context, string) (Issue, error) { return f.issue, nil }

func (f *fakeClient) SearchIssues(context.Context, SearchOptions) (IssuePage, error) {
	return f.page, nil
}

func (f *fakeClient) CreateIssue(_ context.Context, input workitem.CreateTaskInput) (Issue, error) {
	f.created = input
	return f.issue, nil
}

func (f *fakeClient) ListTransitions(context.Context, string) ([]Transition, error) {
	return f.transitions, nil
}

func (f *fakeClient) TransitionIssue(_ context.Context, _, transitionID string) (Issue, error) {
	f.transition = transitionID
	return f.issue, nil
}

func testAdapter(client Client) Adapter {
	return NewAdapter(client, workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "10000", Name: "To Do"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "10001", Name: "In Progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: "10002", Name: "Done"}, Status: workitem.StatusDone},
	}))
}

func TestAdapterGetTask(t *testing.T) {
	t.Parallel()
	adapter := testAdapter(&fakeClient{issue: Issue{
		Key: "APP-1", Summary: "Document API", Description: "Add examples.",
		Status: workitem.ProviderStatus{ID: "10001", Name: "In Progress"}, Labels: []string{"docs"},
	}})

	task, err := adapter.GetTask(context.Background(), workitem.TaskRef{Provider: "jira", ID: "APP-1"})
	if err != nil {
		t.Fatalf("Adapter.GetTask() error = %v", err)
	}
	if task.Ref.ID != "APP-1" || task.Title != "Document API" || task.Status != workitem.StatusInProgress || len(task.Tags) != 1 {
		t.Errorf("Adapter.GetTask() = %+v, want normalized Jira task", task)
	}
}

func TestAdapterCreateTask(t *testing.T) {
	t.Parallel()
	client := &fakeClient{issue: Issue{Key: "APP-2", Status: workitem.ProviderStatus{ID: "10000", Name: "To Do"}}}
	adapter := testAdapter(client)
	input := workitem.CreateTaskInput{Title: "Add tests", Description: "Cover the adapter.", Tags: []string{"testing"}}

	_, err := adapter.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("Adapter.CreateTask() error = %v", err)
	}
	if client.created.Title != input.Title || client.created.Description != input.Description {
		t.Errorf("Adapter.CreateTask() input = %+v, want %+v", client.created, input)
	}
}

func TestAdapterMoveTaskSelectsMappedTransition(t *testing.T) {
	t.Parallel()
	client := &fakeClient{
		issue:       Issue{Key: "APP-3", Status: workitem.ProviderStatus{ID: "10002", Name: "Done"}},
		transitions: []Transition{{ID: "31", Name: "Close issue", To: workitem.ProviderStatus{ID: "10002", Name: "Done"}}},
	}

	_, err := testAdapter(client).MoveTask(context.Background(), workitem.TaskRef{Provider: "jira", ID: "APP-3"}, workitem.StatusDone)
	if err != nil {
		t.Fatalf("Adapter.MoveTask() error = %v", err)
	}
	if client.transition != "31" {
		t.Errorf("Adapter.MoveTask() selected transition %q, want %q", client.transition, "31")
	}
}

func TestAdapterMoveTaskRejectsAmbiguousTransitions(t *testing.T) {
	t.Parallel()
	client := &fakeClient{transitions: []Transition{
		{ID: "31", To: workitem.ProviderStatus{ID: "10002", Name: "Done"}},
		{ID: "32", To: workitem.ProviderStatus{ID: "10002", Name: "Done"}},
	}}

	_, err := testAdapter(client).MoveTask(context.Background(), workitem.TaskRef{Provider: "jira", ID: "APP-4"}, workitem.StatusDone)
	if !errors.Is(err, ErrAmbiguousTransition) {
		t.Errorf("Adapter.MoveTask() error = %v, want %v", err, ErrAmbiguousTransition)
	}
}

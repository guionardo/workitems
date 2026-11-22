package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/guionardo/workitems"
)

type fakeProjectClient struct {
	item      ProjectItem
	page      ProjectPage
	projectID string
	itemID    string
	fieldID   string
	optionID  string
}

func (f *fakeProjectClient) GetProjectItem(context.Context, string, string, string) (ProjectItem, error) {
	return f.item, nil
}

func (f *fakeProjectClient) ListProjectItems(context.Context, string, string, string) (ProjectPage, error) {
	return f.page, nil
}

func (f *fakeProjectClient) SetProjectItemStatus(_ context.Context, projectID, itemID, fieldID, optionID string) (ProjectItem, error) {
	f.projectID, f.itemID, f.fieldID, f.optionID = projectID, itemID, fieldID, optionID
	f.item.ProviderStatus = workitem.ProviderStatus{ID: optionID, Name: "Done"}
	return f.item, nil
}

func TestProjectAdapterListTasks(t *testing.T) {
	t.Parallel()
	mapper := projectMapper()
	adapter := NewProjectAdapter(&fakeProjectClient{page: ProjectPage{Items: []ProjectItem{
		{ID: "item-1", Type: "ISSUE", Title: "Bug", Tags: []string{"bug"}, ProviderStatus: workitem.ProviderStatus{ID: "todo", Name: "Todo"}},
		{ID: "item-2", Type: "PULL_REQUEST", Title: "PR", ProviderStatus: workitem.ProviderStatus{ID: "done", Name: "Done"}},
	}}}, "project", "status", mapper)

	page, err := adapter.ListTasks(context.Background(), workitem.ListOptions{Tags: []string{"bug"}})
	if err != nil {
		t.Fatalf("ProjectAdapter.ListTasks() error = %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].Ref.ID != "item-1" {
		t.Errorf("ProjectAdapter.ListTasks() = %+v, want issue item-1", page.Tasks)
	}
}

func TestProjectAdapterMoveTask(t *testing.T) {
	t.Parallel()
	client := &fakeProjectClient{item: ProjectItem{ID: "item-1", Type: "ISSUE", Title: "Bug"}}
	adapter := NewProjectAdapter(client, "project", "status", projectMapper())

	task, err := adapter.MoveTask(context.Background(), workitem.TaskRef{Provider: ProjectsProvider, ID: "item-1"}, workitem.StatusDone)
	if err != nil {
		t.Fatalf("ProjectAdapter.MoveTask() error = %v", err)
	}
	if client.projectID != "project" || client.itemID != "item-1" || client.fieldID != "status" || client.optionID != "done" {
		t.Errorf("ProjectAdapter.MoveTask() request = project %q, item %q, field %q, option %q", client.projectID, client.itemID, client.fieldID, client.optionID)
	}
	if task.Status != workitem.StatusDone {
		t.Errorf("ProjectAdapter.MoveTask() status = %q, want %q", task.Status, workitem.StatusDone)
	}
}

func TestProjectHTTPClientListProjectItems(t *testing.T) {
	t.Parallel()
	client := NewProjectHTTPClient("https://api.github.test/graphql", "token")
	client.HTTP = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/graphql" || request.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		return response(http.StatusOK, `{"data":{"node":{"items":{"nodes":[{"id":"item-1","type":"ISSUE","content":{"number":7,"url":"https://github.com/acme/app/issues/7","title":"Bug","body":"Details","labels":{"nodes":[{"name":"bug"}]}},"fieldValues":{"nodes":[{"optionId":"done","name":"Done","field":{"id":"status"}}]}}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-2"}}}}}`), nil
	})}

	page, err := client.ListProjectItems(context.Background(), "project", "status", "")
	if err != nil {
		t.Fatalf("ProjectHTTPClient.ListProjectItems() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ProviderStatus.ID != "done" || page.NextPageToken != "cursor-2" {
		t.Errorf("ProjectHTTPClient.ListProjectItems() = %+v", page)
	}
}

func projectMapper() workitem.StatusMapper {
	return workitem.NewStatusMapper([]workitem.StatusMapping{
		{ProviderStatus: workitem.ProviderStatus{ID: "todo", Name: "Todo"}, Status: workitem.StatusTodo},
		{ProviderStatus: workitem.ProviderStatus{ID: "in-progress", Name: "In Progress"}, Status: workitem.StatusInProgress},
		{ProviderStatus: workitem.ProviderStatus{ID: "done", Name: "Done"}, Status: workitem.StatusDone},
	})
}

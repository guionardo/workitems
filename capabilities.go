package workitem

import "context"

// TaskReader reads tasks from a provider context configured by the adapter.
type TaskReader interface {
	GetTask(context.Context, TaskRef) (Task, error)
	ListTasks(context.Context, ListOptions) (TaskPage, error)
}

// TaskCreator creates tasks in a provider context configured by the adapter.
type TaskCreator interface {
	CreateTask(context.Context, CreateTaskInput) (Task, error)
}

// TaskMover changes a task to a canonical status when the provider supports it.
type TaskMover interface {
	MoveTask(context.Context, TaskRef, Status) (Task, error)
}

// CreateTaskInput contains the provider-independent fields accepted on creation.
type CreateTaskInput struct {
	Title       string
	Description string
	Tags        []string
}

// ListOptions controls a provider task listing.
type ListOptions struct {
	Status    *Status
	Tags      []string
	PageToken string
	PageSize  int
}

// TaskPage contains one provider task listing page.
type TaskPage struct {
	Tasks         []Task
	NextPageToken string
}

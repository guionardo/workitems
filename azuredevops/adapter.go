package azuredevops

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guionardo/workitems"
)

const Provider = "azure-devops"

var (
	ErrClientRequired    = errors.New("azure devops client is required")
	ErrInvalidReference  = errors.New("azure devops work item reference is invalid")
	ErrStatusUnsupported = errors.New("azure devops work item status is not supported")
)

// WorkItem is the Azure DevOps data needed to build a canonical task.
type WorkItem struct {
	ID          int
	URL         string
	Type        string
	Title       string
	Description string
	Tags        []string
	State       workitem.ProviderStatus
}

// ListOptions contains Azure DevOps-specific listing values.
type ListOptions struct {
	States    []workitem.ProviderStatus
	Tags      []string
	PageToken string
	PageSize  int
}

// WorkItemPage is one page returned by an Azure DevOps query.
type WorkItemPage struct {
	Items         []WorkItem
	NextPageToken string
}

// Client is the Azure DevOps API boundary used by Adapter.
type Client interface {
	GetWorkItem(context.Context, int) (WorkItem, error)
	ListWorkItems(context.Context, ListOptions) (WorkItemPage, error)
	CreateWorkItem(context.Context, workitem.CreateTaskInput) (WorkItem, error)
	SetWorkItemState(context.Context, int, string) (WorkItem, error)
}

// Adapter translates Azure DevOps work items into the canonical model.
type Adapter struct {
	client Client
	mapper workitem.StatusMapper
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates an Azure DevOps adapter with a workflow mapper.
func NewAdapter(client Client, mapper workitem.StatusMapper) Adapter {
	return Adapter{client: client, mapper: mapper}
}

// GetTask retrieves and normalizes one Azure DevOps work item.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	id, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	item, err := a.client.GetWorkItem(ctx, id)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get Azure DevOps work item: %w", err)
	}
	return a.task(item)
}

// ListTasks queries and normalizes Azure DevOps work items.
func (a Adapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if a.client == nil {
		return workitem.TaskPage{}, ErrClientRequired
	}
	query := ListOptions{Tags: append([]string(nil), options.Tags...), PageToken: options.PageToken, PageSize: options.PageSize}
	if options.Status != nil {
		states, err := a.mapper.ToProvider(*options.Status)
		if err != nil {
			return workitem.TaskPage{}, ErrStatusUnsupported
		}
		query.States = states
	}
	page, err := a.client.ListWorkItems(ctx, query)
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list Azure DevOps work items: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Items))}
	for _, item := range page.Items {
		task, taskErr := a.task(item)
		if taskErr != nil {
			return workitem.TaskPage{}, taskErr
		}
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

// CreateTask creates an Azure DevOps work item.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	item, err := a.client.CreateWorkItem(ctx, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create Azure DevOps work item: %w", err)
	}
	return a.task(item)
}

// MoveTask changes a work item to the uniquely mapped provider state.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	id, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	states, err := a.mapper.ToProvider(target)
	if err != nil || len(states) != 1 {
		return workitem.Task{}, ErrStatusUnsupported
	}
	item, err := a.client.SetWorkItemState(ctx, id, states[0].Name)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move Azure DevOps work item: %w", err)
	}
	return a.task(item)
}

func (a Adapter) validate(ref workitem.TaskRef) (int, error) {
	if a.client == nil {
		return 0, ErrClientRequired
	}
	if ref.Provider != Provider {
		return 0, ErrInvalidReference
	}
	id, err := strconv.Atoi(strings.TrimSpace(ref.ID))
	if err != nil || id < 1 {
		return 0, ErrInvalidReference
	}
	return id, nil
}

func (a Adapter) task(item WorkItem) (workitem.Task, error) {
	status, err := a.mapper.ToCanonical(item.State)
	if err != nil {
		return workitem.Task{}, ErrStatusUnsupported
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: Provider, ID: strconv.Itoa(item.ID), URL: item.URL},
		Title: item.Title, Description: item.Description,
		Tags: append([]string(nil), item.Tags...), Status: status,
		ProviderStatus: item.State,
	}, nil
}

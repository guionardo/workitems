package linear

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/guionardo/workitems"
)

const Provider = "linear"

var (
	ErrClientRequired    = errors.New("linear client is required")
	ErrInvalidReference  = errors.New("linear issue reference is invalid")
	ErrStatusUnsupported = errors.New("linear issue status is not supported")
)

// Label is a Linear issue label.
type Label struct {
	ID   string
	Name string
}

// Issue is the Linear issue data needed to build a canonical task.
type Issue struct {
	ID          string
	Identifier  string
	URL         string
	Title       string
	Description string
	Labels      []Label
	State       workitem.ProviderStatus
}

// IssuePage is one page returned by a Linear team issue listing.
type IssuePage struct {
	Issues        []Issue
	NextPageToken string
}

// Client is the Linear GraphQL boundary used by Adapter.
type Client interface {
	GetIssue(context.Context, string) (Issue, error)
	ListIssues(context.Context, string) (IssuePage, error)
	CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error)
	SetIssueState(context.Context, string, string) (Issue, error)
}

// Adapter translates Linear issues into the canonical workitem model.
type Adapter struct {
	client Client
	mapper workitem.StatusMapper
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates a Linear adapter with a workflow-state mapper.
func NewAdapter(client Client, mapper workitem.StatusMapper) Adapter {
	return Adapter{client: client, mapper: mapper}
}

// GetTask retrieves and normalizes a Linear issue.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	issue, err := a.client.GetIssue(ctx, ref.ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get Linear issue: %w", err)
	}
	return a.task(issue)
}

// ListTasks lists and normalizes Linear team issues.
func (a Adapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if a.client == nil {
		return workitem.TaskPage{}, ErrClientRequired
	}
	page, err := a.client.ListIssues(ctx, options.PageToken)
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list Linear issues: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Issues))}
	for _, issue := range page.Issues {
		task, taskErr := a.task(issue)
		if taskErr != nil {
			return workitem.TaskPage{}, taskErr
		}
		if options.Status != nil && task.Status != *options.Status {
			continue
		}
		if !hasLabels(issue.Labels, options.Tags) {
			continue
		}
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

// CreateTask creates a Linear issue in the configured team.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	issue, err := a.client.CreateIssue(ctx, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create Linear issue: %w", err)
	}
	return a.task(issue)
}

// MoveTask changes a Linear issue to the uniquely mapped workflow state.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	states, err := a.mapper.ToProvider(target)
	if err != nil || len(states) != 1 {
		return workitem.Task{}, ErrStatusUnsupported
	}
	issue, err := a.client.SetIssueState(ctx, ref.ID, states[0].ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move Linear issue: %w", err)
	}
	return a.task(issue)
}

func (a Adapter) validate(ref workitem.TaskRef) error {
	if a.client == nil {
		return ErrClientRequired
	}
	if ref.Provider != Provider || strings.TrimSpace(ref.ID) == "" {
		return ErrInvalidReference
	}
	return nil
}

func (a Adapter) task(issue Issue) (workitem.Task, error) {
	status, err := a.mapper.ToCanonical(issue.State)
	if err != nil {
		return workitem.Task{}, ErrStatusUnsupported
	}
	tags := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		tags = append(tags, label.Name)
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: Provider, ID: issue.Identifier, URL: issue.URL},
		Title: issue.Title, Description: issue.Description, Tags: tags,
		Status: status, ProviderStatus: issue.State,
	}, nil
}

func hasLabels(labels []Label, wanted []string) bool {
	for _, tag := range wanted {
		found := false
		for _, label := range labels {
			if label.Name == tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

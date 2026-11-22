package gitea

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guionardo/workitems"
)

const Provider = "gitea"

var (
	ErrClientRequired    = errors.New("gitea client is required")
	ErrInvalidReference  = errors.New("gitea task reference is invalid")
	ErrStatusUnsupported = errors.New("gitea issue status is not supported")
)

// Issue is the Gitea issue data needed to build a canonical task.
type Issue struct {
	Index       int
	URL         string
	Title       string
	Description string
	State       string
	Labels      []string
}

// ListOptions contains Gitea-specific listing values translated by Adapter.
type ListOptions struct {
	State  string
	Labels []string
	Page   int
}

// IssuePage is one page returned by a Gitea issue listing.
type IssuePage struct {
	Issues        []Issue
	NextPageToken string
}

// Client is the Gitea API boundary used by Adapter.
type Client interface {
	GetIssue(context.Context, int) (Issue, error)
	ListIssues(context.Context, ListOptions) (IssuePage, error)
	CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error)
	SetIssueState(context.Context, int, string) (Issue, error)
}

// Adapter translates Gitea issues into the canonical workitem model.
type Adapter struct {
	client Client
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates a Gitea issue adapter.
func NewAdapter(client Client) Adapter {
	return Adapter{client: client}
}

// GetTask retrieves and normalizes a Gitea issue.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	index, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	issue, err := a.client.GetIssue(ctx, index)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get Gitea issue: %w", err)
	}
	return task(issue)
}

// ListTasks lists and normalizes Gitea issues.
func (a Adapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if a.client == nil {
		return workitem.TaskPage{}, ErrClientRequired
	}
	state, err := issueState(options.Status)
	if err != nil {
		return workitem.TaskPage{}, err
	}
	page := 1
	if options.PageToken != "" {
		page, err = strconv.Atoi(options.PageToken)
		if err != nil || page < 1 {
			return workitem.TaskPage{}, errors.New("gitea page token is invalid")
		}
	}
	result, err := a.client.ListIssues(ctx, ListOptions{
		State:  state,
		Labels: append([]string(nil), options.Tags...),
		Page:   page,
	})
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list Gitea issues: %w", err)
	}
	tasks := make([]workitem.Task, 0, len(result.Issues))
	for _, issue := range result.Issues {
		item, taskErr := task(issue)
		if taskErr != nil {
			return workitem.TaskPage{}, taskErr
		}
		tasks = append(tasks, item)
	}
	return workitem.TaskPage{Tasks: tasks, NextPageToken: result.NextPageToken}, nil
}

// CreateTask creates a Gitea issue.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	issue, err := a.client.CreateIssue(ctx, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create Gitea issue: %w", err)
	}
	return task(issue)
}

// MoveTask opens or closes a Gitea issue for supported canonical statuses.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	index, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	state := ""
	switch target {
	case workitem.StatusTodo:
		state = "open"
	case workitem.StatusDone:
		state = "closed"
	default:
		return workitem.Task{}, ErrStatusUnsupported
	}
	issue, err := a.client.SetIssueState(ctx, index, state)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move Gitea issue: %w", err)
	}
	return task(issue)
}

func (a Adapter) validate(ref workitem.TaskRef) (int, error) {
	if a.client == nil {
		return 0, ErrClientRequired
	}
	if ref.Provider != Provider {
		return 0, ErrInvalidReference
	}
	index, err := strconv.Atoi(strings.TrimSpace(ref.ID))
	if err != nil || index < 1 {
		return 0, ErrInvalidReference
	}
	return index, nil
}

func task(issue Issue) (workitem.Task, error) {
	state := strings.ToLower(strings.TrimSpace(issue.State))
	canonical := workitem.StatusUnknown
	switch state {
	case "open":
		canonical = workitem.StatusTodo
	case "closed":
		canonical = workitem.StatusDone
	default:
		return workitem.Task{}, ErrStatusUnsupported
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: Provider, ID: strconv.Itoa(issue.Index), URL: issue.URL},
		Title: issue.Title, Description: issue.Description,
		Tags: append([]string(nil), issue.Labels...), Status: canonical,
		ProviderStatus: workitem.ProviderStatus{ID: state, Name: state},
	}, nil
}

func issueState(status *workitem.Status) (string, error) {
	if status == nil {
		return "all", nil
	}
	switch *status {
	case workitem.StatusTodo:
		return "open", nil
	case workitem.StatusDone:
		return "closed", nil
	case workitem.StatusInProgress:
		return "", ErrStatusUnsupported
	default:
		return "", ErrStatusUnsupported
	}
}

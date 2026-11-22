package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guionardo/workitems"
)

const Provider = "gitlab"

var (
	ErrClientRequired    = errors.New("gitlab client is required")
	ErrInvalidReference  = errors.New("gitlab task reference is invalid")
	ErrStatusUnsupported = errors.New("gitlab issue status is not supported")
)

// Issue is the GitLab issue data needed to build a canonical task.
type Issue struct {
	IID         int
	URL         string
	Title       string
	Description string
	State       string
	Labels      []string
}

// ListOptions contains GitLab-specific listing values translated by Adapter.
type ListOptions struct {
	State  string
	Labels []string
	Page   int
}

// IssuePage is one page returned by a GitLab issue listing.
type IssuePage struct {
	Issues        []Issue
	NextPageToken string
}

// Client is the GitLab API boundary used by Adapter.
type Client interface {
	GetIssue(context.Context, int) (Issue, error)
	ListIssues(context.Context, ListOptions) (IssuePage, error)
	CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error)
	SetIssueState(context.Context, int, string) (Issue, error)
}

// Adapter translates GitLab issues into the canonical workitem model.
type Adapter struct {
	client Client
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates a GitLab issue adapter.
func NewAdapter(client Client) Adapter {
	return Adapter{client: client}
}

// GetTask retrieves and normalizes a GitLab issue.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	iid, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	issue, err := a.client.GetIssue(ctx, iid)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get GitLab issue: %w", err)
	}
	return task(issue)
}

// ListTasks lists and normalizes GitLab issues.
func (a Adapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if a.client == nil {
		return workitem.TaskPage{}, ErrClientRequired
	}
	state, err := issueState(options.Status)
	if err != nil {
		return workitem.TaskPage{}, err
	}
	pageNumber := 1
	if options.PageToken != "" {
		pageNumber, err = strconv.Atoi(options.PageToken)
		if err != nil || pageNumber < 1 {
			return workitem.TaskPage{}, errors.New("gitlab page token is invalid")
		}
	}
	page, err := a.client.ListIssues(ctx, ListOptions{State: state, Labels: append([]string(nil), options.Tags...), Page: pageNumber})
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list GitLab issues: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Issues))}
	for _, issue := range page.Issues {
		item, err := task(issue)
		if err != nil {
			return workitem.TaskPage{}, err
		}
		result.Tasks = append(result.Tasks, item)
	}
	return result, nil
}

// CreateTask creates a GitLab issue.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	issue, err := a.client.CreateIssue(ctx, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create GitLab issue: %w", err)
	}
	return task(issue)
}

// MoveTask opens or closes a GitLab issue for the supported canonical statuses.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	iid, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	stateEvent := ""
	switch target {
	case workitem.StatusTodo:
		stateEvent = "reopen"
	case workitem.StatusDone:
		stateEvent = "close"
	default:
		return workitem.Task{}, ErrStatusUnsupported
	}
	issue, err := a.client.SetIssueState(ctx, iid, stateEvent)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move GitLab issue: %w", err)
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
	iid, err := strconv.Atoi(strings.TrimSpace(ref.ID))
	if err != nil || iid < 1 {
		return 0, ErrInvalidReference
	}
	return iid, nil
}

func task(issue Issue) (workitem.Task, error) {
	state := strings.ToLower(strings.TrimSpace(issue.State))
	canonical := workitem.StatusUnknown
	switch state {
	case "opened":
		canonical = workitem.StatusTodo
	case "closed":
		canonical = workitem.StatusDone
	default:
		return workitem.Task{}, ErrStatusUnsupported
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: Provider, ID: strconv.Itoa(issue.IID), URL: issue.URL},
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
		return "opened", nil
	case workitem.StatusDone:
		return "closed", nil
	case workitem.StatusInProgress:
		return "", ErrStatusUnsupported
	default:
		return "", ErrStatusUnsupported
	}
}

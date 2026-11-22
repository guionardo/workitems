package github

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guionardo/workitems"
)

var (
	ErrClientRequired         = errors.New("github client is required")
	ErrInvalidReference       = errors.New("github task reference is invalid")
	ErrPullRequestUnsupported = errors.New("github pull requests are not supported as tasks")
	ErrStatusUnsupported      = errors.New("github issue status is not supported")
)

// Issue is the GitHub issue data needed to build a canonical task.
type Issue struct {
	Number      int
	URL         string
	Title       string
	Body        string
	State       string
	Labels      []string
	PullRequest bool
}

// ListOptions contains GitHub-specific listing values translated by Adapter.
type ListOptions struct {
	State  string
	Labels []string
	Page   int
}

// IssuePage is one page returned by a GitHub issue listing.
type IssuePage struct {
	Issues        []Issue
	NextPageToken string
}

// Client is the GitHub API boundary used by Adapter.
type Client interface {
	GetIssue(context.Context, int) (Issue, error)
	ListIssues(context.Context, ListOptions) (IssuePage, error)
	CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error)
	SetIssueState(context.Context, int, string) (Issue, error)
}

// Adapter translates GitHub issues into the canonical workitem model.
type Adapter struct {
	client Client
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates a GitHub issue adapter.
func NewAdapter(client Client) Adapter {
	return Adapter{client: client}
}

// GetTask retrieves and normalizes a GitHub issue.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	number, err := a.validate(ref)
	if err != nil {
		return workitem.Task{}, err
	}
	issue, err := a.client.GetIssue(ctx, number)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get GitHub issue: %w", err)
	}
	return task(issue)
}

// ListTasks lists and normalizes GitHub issues, excluding pull requests.
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
			return workitem.TaskPage{}, errors.New("github page token is invalid")
		}
	}
	page, err := a.client.ListIssues(ctx, ListOptions{State: state, Labels: append([]string(nil), options.Tags...), Page: pageNumber})
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list GitHub issues: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Issues))}
	for _, issue := range page.Issues {
		if issue.PullRequest {
			continue
		}
		item, err := task(issue)
		if err != nil {
			return workitem.TaskPage{}, err
		}
		result.Tasks = append(result.Tasks, item)
	}
	return result, nil
}

// CreateTask creates a GitHub issue.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	issue, err := a.client.CreateIssue(ctx, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create GitHub issue: %w", err)
	}
	return task(issue)
}

// MoveTask opens or closes a GitHub issue for the supported canonical statuses.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	number, err := a.validate(ref)
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
	issue, err := a.client.SetIssueState(ctx, number, state)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move GitHub issue: %w", err)
	}
	return task(issue)
}

func (a Adapter) validate(ref workitem.TaskRef) (int, error) {
	if a.client == nil {
		return 0, ErrClientRequired
	}
	if ref.Provider != "github" {
		return 0, ErrInvalidReference
	}
	number, err := strconv.Atoi(strings.TrimSpace(ref.ID))
	if err != nil || number < 1 {
		return 0, ErrInvalidReference
	}
	return number, nil
}

func task(issue Issue) (workitem.Task, error) {
	if issue.PullRequest {
		return workitem.Task{}, ErrPullRequestUnsupported
	}
	status := strings.ToLower(strings.TrimSpace(issue.State))
	canonical := workitem.StatusUnknown
	switch status {
	case "open":
		canonical = workitem.StatusTodo
	case "closed":
		canonical = workitem.StatusDone
	default:
		return workitem.Task{}, ErrStatusUnsupported
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: "github", ID: strconv.Itoa(issue.Number), URL: issue.URL},
		Title: issue.Title, Description: issue.Body,
		Tags: append([]string(nil), issue.Labels...), Status: canonical,
		ProviderStatus: workitem.ProviderStatus{ID: status, Name: status},
	}, nil
}

func issueState(status *workitem.Status) (string, error) {
	if status == nil {
		return "all", nil
	}
	switch *status {
	case workitem.StatusTodo, workitem.StatusInProgress:
		return "open", nil
	case workitem.StatusDone:
		return "closed", nil
	default:
		return "", ErrStatusUnsupported
	}
}

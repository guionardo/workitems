package jira

import (
	"context"
	"errors"
	"fmt"

	"github.com/guionardo/workitems"
)

var (
	// ErrClientRequired indicates that the adapter has no Jira client.
	ErrClientRequired      = errors.New("jira client is required")
	// ErrInvalidReference indicates that a task reference is not a Jira reference.
	ErrInvalidReference    = errors.New("jira task reference is invalid")
	// ErrAmbiguousTransition indicates that multiple transitions reach the target status.
	ErrAmbiguousTransition = errors.New("jira transition is ambiguous")
)

// Issue is the Jira data needed to build a canonical task.
type Issue struct {
	ID          string
	Key         string
	URL         string
	Summary     string
	Description string
	Status      workitem.ProviderStatus
	Labels      []string
}

// Transition is a Jira transition and its destination status.
type Transition struct {
	ID   string
	Name string
	To   workitem.ProviderStatus
}

// SearchOptions contains Jira-specific search values translated by Adapter.
type SearchOptions struct {
	Statuses  []workitem.ProviderStatus
	Labels    []string
	PageToken string
	PageSize  int
}

// IssuePage is one page returned by a Jira search.
type IssuePage struct {
	Issues        []Issue
	NextPageToken string
}

// Client is the Jira API boundary used by Adapter.
type Client interface {
	GetIssue(context.Context, string) (Issue, error)
	SearchIssues(context.Context, SearchOptions) (IssuePage, error)
	CreateIssue(context.Context, workitem.CreateTaskInput) (Issue, error)
	ListTransitions(context.Context, string) ([]Transition, error)
	TransitionIssue(context.Context, string, string) (Issue, error)
}

// Adapter translates Jira issues into the canonical workitem model.
type Adapter struct {
	client Client
	mapper workitem.StatusMapper
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates a Jira adapter with a status mapper scoped to its Jira context.
func NewAdapter(client Client, mapper workitem.StatusMapper) Adapter {
	return Adapter{client: client, mapper: mapper}
}

// GetTask retrieves and normalizes a Jira issue.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	issue, err := a.client.GetIssue(ctx, ref.ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get Jira issue: %w", err)
	}
	return a.task(issue)
}

// ListTasks searches and normalizes Jira issues.
func (a Adapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if a.client == nil {
		return workitem.TaskPage{}, ErrClientRequired
	}
	search := SearchOptions{Labels: append([]string(nil), options.Tags...), PageToken: options.PageToken, PageSize: options.PageSize}
	if options.Status != nil {
		statuses, err := a.mapper.ToProvider(*options.Status)
		if err != nil {
			return workitem.TaskPage{}, err
		}
		search.Statuses = statuses
	}
	page, err := a.client.SearchIssues(ctx, search)
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("search Jira issues: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Issues))}
	for _, issue := range page.Issues {
		task, err := a.task(issue)
		if err != nil {
			return workitem.TaskPage{}, err
		}
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

// CreateTask creates a Jira issue from canonical task fields.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	issue, err := a.client.CreateIssue(ctx, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create Jira issue: %w", err)
	}
	return a.task(issue)
}

// MoveTask applies the unique Jira transition mapped to the target status.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	transitions, err := a.client.ListTransitions(ctx, ref.ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("list Jira transitions: %w", err)
	}
	var selected *Transition
	for _, transition := range transitions {
		status, mapErr := a.mapper.ToCanonical(transition.To)
		if mapErr != nil || status != target {
			continue
		}
		if selected != nil {
			return workitem.Task{}, ErrAmbiguousTransition
		}
		candidate := transition
		selected = &candidate
	}
	if selected == nil {
		return workitem.Task{}, workitem.ErrNoProviderStatus
	}
	issue, err := a.client.TransitionIssue(ctx, ref.ID, selected.ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move Jira issue: %w", err)
	}
	return a.task(issue)
}

func (a Adapter) task(issue Issue) (workitem.Task, error) {
	status, err := a.mapper.ToCanonical(issue.Status)
	if err != nil {
		return workitem.Task{}, err
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: "jira", ID: firstNonEmpty(issue.Key, issue.ID), URL: issue.URL},
		Title: issue.Summary, Description: issue.Description,
		Tags: append([]string(nil), issue.Labels...), Status: status, ProviderStatus: issue.Status,
	}, nil
}

func (a Adapter) validate(ref workitem.TaskRef) error {
	if a.client == nil {
		return ErrClientRequired
	}
	if ref.Provider != "jira" || ref.ID == "" {
		return ErrInvalidReference
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

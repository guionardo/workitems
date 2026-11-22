package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/guionardo/workitems"
)

const ProjectsProvider = "github-projects"

var (
	ErrProjectClientRequired    = errors.New("github projects client is required")
	ErrInvalidProjectReference  = errors.New("github projects task reference is invalid")
	ErrProjectItemUnsupported   = errors.New("github project item is not an issue")
	ErrProjectStatusUnsupported = errors.New("github project status is not supported")
	ErrAmbiguousProjectStatus   = errors.New("github project status mapping is ambiguous")
)

// ProjectItem is the GitHub Projects V2 data needed to build a canonical task.
type ProjectItem struct {
	ID             string
	URL            string
	IssueNumber    int
	Title          string
	Description    string
	Tags           []string
	Type           string
	ProviderStatus workitem.ProviderStatus
}

// ProjectPage is one page returned by a GitHub Projects V2 listing.
type ProjectPage struct {
	Items         []ProjectItem
	NextPageToken string
}

// ProjectClient is the GitHub Projects V2 API boundary used by ProjectAdapter.
type ProjectClient interface {
	GetProjectItem(context.Context, string, string, string) (ProjectItem, error)
	ListProjectItems(context.Context, string, string, string) (ProjectPage, error)
	SetProjectItemStatus(context.Context, string, string, string, string) (ProjectItem, error)
}

// ProjectAdapter translates GitHub Projects V2 issue items into workitems.
type ProjectAdapter struct {
	client        ProjectClient
	projectID     string
	statusFieldID string
	mapper        workitem.StatusMapper
}

var (
	_ workitem.TaskReader = ProjectAdapter{}
	_ workitem.TaskMover  = ProjectAdapter{}
)

// NewProjectAdapter creates an adapter for one GitHub Projects V2 project.
func NewProjectAdapter(client ProjectClient, projectID, statusFieldID string, mapper workitem.StatusMapper) ProjectAdapter {
	return ProjectAdapter{
		client: client, projectID: strings.TrimSpace(projectID),
		statusFieldID: strings.TrimSpace(statusFieldID), mapper: mapper,
	}
}

// GetTask retrieves and normalizes one GitHub Projects V2 issue item.
func (a ProjectAdapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	item, err := a.client.GetProjectItem(ctx, a.projectID, ref.ID, a.statusFieldID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get GitHub project item: %w", err)
	}
	return a.task(item)
}

// ListTasks lists issue-backed GitHub Projects V2 items and applies canonical filters.
func (a ProjectAdapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if err := a.validateConfiguration(); err != nil {
		return workitem.TaskPage{}, err
	}
	page, err := a.client.ListProjectItems(ctx, a.projectID, a.statusFieldID, options.PageToken)
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list GitHub project items: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Items))}
	for _, item := range page.Items {
		task, taskErr := a.task(item)
		if errors.Is(taskErr, ErrProjectItemUnsupported) {
			continue
		}
		if taskErr != nil {
			return workitem.TaskPage{}, taskErr
		}
		if options.Status != nil && task.Status != *options.Status {
			continue
		}
		if !hasAllTags(task.Tags, options.Tags) {
			continue
		}
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

// MoveTask changes the project status of an issue item using the configured mapping.
func (a ProjectAdapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	statuses, err := a.mapper.ToProvider(target)
	if err != nil {
		return workitem.Task{}, ErrProjectStatusUnsupported
	}
	if len(statuses) != 1 {
		return workitem.Task{}, ErrAmbiguousProjectStatus
	}
	item, err := a.client.SetProjectItemStatus(ctx, a.projectID, ref.ID, a.statusFieldID, statuses[0].ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move GitHub project item: %w", err)
	}
	if item.ProviderStatus.ID == "" {
		item.ProviderStatus = statuses[0]
	}
	return a.task(item)
}

func (a ProjectAdapter) validate(ref workitem.TaskRef) error {
	if err := a.validateConfiguration(); err != nil {
		return err
	}
	if ref.Provider != ProjectsProvider || strings.TrimSpace(ref.ID) == "" {
		return ErrInvalidProjectReference
	}
	return nil
}

func (a ProjectAdapter) validateConfiguration() error {
	if a.client == nil {
		return ErrProjectClientRequired
	}
	if a.projectID == "" || a.statusFieldID == "" {
		return ErrInvalidProjectReference
	}
	return nil
}

func (a ProjectAdapter) task(item ProjectItem) (workitem.Task, error) {
	if item.Type != "" && item.Type != "ISSUE" {
		return workitem.Task{}, ErrProjectItemUnsupported
	}
	status, err := a.mapper.ToCanonical(item.ProviderStatus)
	if err != nil {
		return workitem.Task{}, ErrProjectStatusUnsupported
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: ProjectsProvider, ID: item.ID, URL: item.URL},
		Title: item.Title, Description: item.Description,
		Tags: append([]string(nil), item.Tags...), Status: status,
		ProviderStatus: item.ProviderStatus,
	}, nil
}

func hasAllTags(have, wanted []string) bool {
	for _, tag := range wanted {
		found := false
		for _, candidate := range have {
			if candidate == tag {
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

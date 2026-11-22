package gitlab

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guionardo/workitems"
)

var (
	ErrBoardClientRequired    = errors.New("gitlab board client is required")
	ErrInvalidBoard           = errors.New("gitlab board configuration is invalid")
	ErrBoardStatusUnsupported = errors.New("gitlab board status is not supported")
	ErrAmbiguousBoardStatus   = errors.New("gitlab issue matches multiple board label lists")
)

// BoardList is a label-backed GitLab issue board list.
type BoardList struct {
	ID       int
	Position int
	LabelID  int
	Label    string
	Type     string
}

// BoardClient reads the board configuration used by BoardAdapter.
type BoardClient interface {
	ListBoardLists(context.Context, int) ([]BoardList, error)
}

// IssueUpdater updates labels and state on an existing GitLab issue.
type IssueUpdater interface {
	UpdateIssue(context.Context, int, []string, string) (Issue, error)
}

// BoardAdapter maps label-backed GitLab board lists to canonical statuses.
type BoardAdapter struct {
	issues  Client
	updater IssueUpdater
	board   BoardClient
	boardID int
	mapper  workitem.StatusMapper
}

var (
	_ workitem.TaskReader = BoardAdapter{}
	_ workitem.TaskMover  = BoardAdapter{}
)

// NewBoardAdapter creates an adapter for one GitLab issue board.
func NewBoardAdapter(issues Client, updater IssueUpdater, board BoardClient, boardID int, mapper workitem.StatusMapper) BoardAdapter {
	return BoardAdapter{issues: issues, updater: updater, board: board, boardID: boardID, mapper: mapper}
}

// GetTask retrieves and normalizes an issue using board-list status.
func (a BoardAdapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	lists, err := a.board.ListBoardLists(ctx, a.boardID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("list GitLab board lists: %w", err)
	}
	iid, _ := strconv.Atoi(ref.ID)
	issue, err := a.issues.GetIssue(ctx, iid)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get GitLab board issue: %w", err)
	}
	return a.task(issue, lists)
}

// ListTasks lists issues and maps their status from the configured board lists.
func (a BoardAdapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if err := a.validateConfiguration(); err != nil {
		return workitem.TaskPage{}, err
	}
	lists, err := a.board.ListBoardLists(ctx, a.boardID)
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list GitLab board lists: %w", err)
	}
	pageNumber := 1
	if options.PageToken != "" {
		pageNumber, err = strconv.Atoi(options.PageToken)
		if err != nil || pageNumber < 1 {
			return workitem.TaskPage{}, errors.New("gitlab board page token is invalid")
		}
	}
	page, err := a.issues.ListIssues(ctx, ListOptions{State: "all", Labels: append([]string(nil), options.Tags...), Page: pageNumber})
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list GitLab board issues: %w", err)
	}
	result := workitem.TaskPage{NextPageToken: page.NextPageToken, Tasks: make([]workitem.Task, 0, len(page.Issues))}
	for _, issue := range page.Issues {
		task, taskErr := a.task(issue, lists)
		if taskErr != nil {
			return workitem.TaskPage{}, taskErr
		}
		if options.Status != nil && task.Status != *options.Status {
			continue
		}
		result.Tasks = append(result.Tasks, task)
	}
	return result, nil
}

// MoveTask moves an issue to the label-backed board list mapped to target.
func (a BoardAdapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	if a.updater == nil {
		return workitem.Task{}, ErrBoardClientRequired
	}
	lists, err := a.board.ListBoardLists(ctx, a.boardID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("list GitLab board lists: %w", err)
	}
	statuses, err := a.mapper.ToProvider(target)
	if err != nil || len(statuses) != 1 {
		return workitem.Task{}, ErrBoardStatusUnsupported
	}
	iid, _ := strconv.Atoi(ref.ID)
	issue, err := a.issues.GetIssue(ctx, iid)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get GitLab board issue: %w", err)
	}
	labels := boardLabels(issue.Labels, lists)
	stateEvent := ""
	targetID := statuses[0].ID
	switch {
	case targetID == "open":
		stateEvent = "reopen"
	case targetID == "closed":
		stateEvent = "close"
	case strings.HasPrefix(targetID, "label:"):
		label := strings.TrimPrefix(targetID, "label:")
		labels = append(labels, label)
	default:
		return workitem.Task{}, ErrBoardStatusUnsupported
	}
	updated, err := a.updater.UpdateIssue(ctx, iid, labels, stateEvent)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move GitLab board issue: %w", err)
	}
	return a.task(updated, lists)
}

func (a BoardAdapter) validate(ref workitem.TaskRef) error {
	if err := a.validateConfiguration(); err != nil {
		return err
	}
	if ref.Provider != Provider || strings.TrimSpace(ref.ID) == "" {
		return ErrInvalidReference
	}
	if _, err := strconv.Atoi(ref.ID); err != nil {
		return ErrInvalidReference
	}
	return nil
}

func (a BoardAdapter) validateConfiguration() error {
	if a.issues == nil || a.board == nil {
		return ErrBoardClientRequired
	}
	if a.boardID < 1 {
		return ErrInvalidBoard
	}
	return nil
}

func (a BoardAdapter) task(issue Issue, lists []BoardList) (workitem.Task, error) {
	providerStatus, err := boardStatus(issue, lists)
	if err != nil {
		return workitem.Task{}, err
	}
	status, err := a.mapper.ToCanonical(providerStatus)
	if err != nil {
		return workitem.Task{}, ErrBoardStatusUnsupported
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: Provider, ID: strconv.Itoa(issue.IID), URL: issue.URL},
		Title: issue.Title, Description: issue.Description,
		Tags: append([]string(nil), issue.Labels...), Status: status,
		ProviderStatus: providerStatus,
	}, nil
}

func boardStatus(issue Issue, lists []BoardList) (workitem.ProviderStatus, error) {
	if strings.EqualFold(issue.State, "closed") {
		return workitem.ProviderStatus{ID: "closed", Name: "Closed"}, nil
	}
	matched := make([]BoardList, 0, len(lists))
	for _, list := range lists {
		if list.Type == "label" && list.Label != "" && hasLabel(issue.Labels, list.Label) {
			matched = append(matched, list)
		}
	}
	if len(matched) > 1 {
		return workitem.ProviderStatus{}, ErrAmbiguousBoardStatus
	}
	if len(matched) == 1 {
		return workitem.ProviderStatus{ID: "label:" + matched[0].Label, Name: matched[0].Label}, nil
	}
	return workitem.ProviderStatus{ID: "open", Name: "Open"}, nil
}

func boardLabels(labels []string, lists []BoardList) []string {
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		isBoardLabel := false
		for _, list := range lists {
			if list.Type == "label" && list.Label == label {
				isBoardLabel = true
				break
			}
		}
		if !isBoardLabel {
			result = append(result, label)
		}
	}
	return result
}

func hasLabel(labels []string, wanted string) bool {
	for _, label := range labels {
		if label == wanted {
			return true
		}
	}
	return false
}

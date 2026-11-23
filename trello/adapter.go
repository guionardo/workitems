package trello

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/guionardo/workitems"
)

const Provider = "trello"

var (
	ErrClientRequired    = errors.New("trello client is required")
	ErrInvalidReference  = errors.New("trello card reference is invalid")
	ErrStatusUnsupported = errors.New("trello card status is not supported")
)

// Label is a Trello card label.
type Label struct {
	ID   string
	Name string
}

// Card is the Trello card data needed to build a canonical task.
type Card struct {
	ID          string
	URL         string
	Title       string
	Description string
	List        workitem.ProviderStatus
	Labels      []Label
}

// Client is the Trello API boundary used by Adapter.
type Client interface {
	GetCard(context.Context, string) (Card, error)
	ListCards(context.Context) ([]Card, error)
	CreateCard(context.Context, string, workitem.CreateTaskInput) (Card, error)
	MoveCard(context.Context, string, string) (Card, error)
}

// Adapter translates Trello cards into the canonical workitem model.
type Adapter struct {
	client Client
	mapper workitem.StatusMapper
}

var (
	_ workitem.TaskReader  = Adapter{}
	_ workitem.TaskCreator = Adapter{}
	_ workitem.TaskMover   = Adapter{}
)

// NewAdapter creates a Trello adapter with a list-to-status mapper.
func NewAdapter(client Client, mapper workitem.StatusMapper) Adapter {
	return Adapter{client: client, mapper: mapper}
}

// GetTask retrieves and normalizes a Trello card.
func (a Adapter) GetTask(ctx context.Context, ref workitem.TaskRef) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	card, err := a.client.GetCard(ctx, ref.ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("get Trello card: %w", err)
	}
	return a.task(card)
}

// ListTasks lists and normalizes cards on the configured Trello board.
func (a Adapter) ListTasks(ctx context.Context, options workitem.ListOptions) (workitem.TaskPage, error) {
	if a.client == nil {
		return workitem.TaskPage{}, ErrClientRequired
	}
	cards, err := a.client.ListCards(ctx)
	if err != nil {
		return workitem.TaskPage{}, fmt.Errorf("list Trello cards: %w", err)
	}
	tasks := make([]workitem.Task, 0, len(cards))
	for _, card := range cards {
		task, taskErr := a.task(card)
		if taskErr != nil {
			return workitem.TaskPage{}, taskErr
		}
		if options.Status != nil && task.Status != *options.Status {
			continue
		}
		if !hasLabels(card.Labels, options.Tags) {
			continue
		}
		tasks = append(tasks, task)
	}
	return workitem.TaskPage{Tasks: tasks}, nil
}

// CreateTask creates a Trello card in the list mapped to todo.
func (a Adapter) CreateTask(ctx context.Context, input workitem.CreateTaskInput) (workitem.Task, error) {
	if a.client == nil {
		return workitem.Task{}, ErrClientRequired
	}
	lists, err := a.mapper.ToProvider(workitem.StatusTodo)
	if err != nil || len(lists) != 1 {
		return workitem.Task{}, ErrStatusUnsupported
	}
	card, err := a.client.CreateCard(ctx, lists[0].ID, input)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("create Trello card: %w", err)
	}
	return a.task(card)
}

// MoveTask moves a Trello card to the list mapped to a canonical status.
func (a Adapter) MoveTask(ctx context.Context, ref workitem.TaskRef, target workitem.Status) (workitem.Task, error) {
	if err := a.validate(ref); err != nil {
		return workitem.Task{}, err
	}
	lists, err := a.mapper.ToProvider(target)
	if err != nil || len(lists) != 1 {
		return workitem.Task{}, ErrStatusUnsupported
	}
	card, err := a.client.MoveCard(ctx, ref.ID, lists[0].ID)
	if err != nil {
		return workitem.Task{}, fmt.Errorf("move Trello card: %w", err)
	}
	return a.task(card)
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

func (a Adapter) task(card Card) (workitem.Task, error) {
	status, err := a.mapper.ToCanonical(card.List)
	if err != nil {
		return workitem.Task{}, ErrStatusUnsupported
	}
	tags := make([]string, 0, len(card.Labels))
	for _, label := range card.Labels {
		tags = append(tags, label.Name)
	}
	return workitem.Task{
		Ref:   workitem.TaskRef{Provider: Provider, ID: card.ID, URL: card.URL},
		Title: card.Title, Description: card.Description, Tags: tags,
		Status: status, ProviderStatus: card.List,
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

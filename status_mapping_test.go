package workitem

import (
	"errors"
	"testing"
)

func TestStatusMapper(t *testing.T) {
	t.Parallel()

	mapper := NewStatusMapper([]StatusMapping{
		{ProviderStatus: ProviderStatus{ID: "10000", Name: "To Do"}, Status: StatusTodo},
		{ProviderStatus: ProviderStatus{ID: "10001", Name: "In Progress"}, Status: StatusInProgress},
		{ProviderStatus: ProviderStatus{ID: "10002", Name: "Code Review"}, Status: StatusInProgress},
		{ProviderStatus: ProviderStatus{ID: "10003", Name: "Done"}, Status: StatusDone},
	})

	t.Run("maps provider status to canonical status", func(t *testing.T) {
		t.Parallel()

		got, err := mapper.ToCanonical(ProviderStatus{ID: "10002", Name: "Code Review"})
		if err != nil {
			t.Fatalf("StatusMapper.ToCanonical() error = %v", err)
		}
		if got != StatusInProgress {
			t.Errorf("StatusMapper.ToCanonical() = %q, want %q", got, StatusInProgress)
		}
	})

	t.Run("rejects an unknown provider status", func(t *testing.T) {
		t.Parallel()

		_, err := mapper.ToCanonical(ProviderStatus{ID: "missing"})
		if !errors.Is(err, ErrUnknownProviderStatus) {
			t.Errorf("StatusMapper.ToCanonical() error = %v, want %v", err, ErrUnknownProviderStatus)
		}
	})

	t.Run("returns all provider statuses for a canonical status", func(t *testing.T) {
		t.Parallel()

		got, err := mapper.ToProvider(StatusInProgress)
		if err != nil {
			t.Fatalf("StatusMapper.ToProvider() error = %v", err)
		}
		if len(got) != 2 || got[0].ID != "10001" || got[1].ID != "10002" {
			t.Errorf("StatusMapper.ToProvider() = %+v, want IDs [10001 10002]", got)
		}
	})

	t.Run("rejects a canonical status without a mapping", func(t *testing.T) {
		t.Parallel()

		_, err := mapper.ToProvider(StatusUnknown)
		if !errors.Is(err, ErrNoProviderStatus) {
			t.Errorf("StatusMapper.ToProvider() error = %v, want %v", err, ErrNoProviderStatus)
		}
	})
}

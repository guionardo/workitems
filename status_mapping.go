package workitem

import (
	"errors"
	"sort"
)

var (
	// ErrUnknownProviderStatus indicates that no mapping exists for a provider status.
	ErrUnknownProviderStatus = errors.New("unknown provider status")
	// ErrNoProviderStatus indicates that no provider status represents a canonical status.
	ErrNoProviderStatus = errors.New("no provider status mapped")
)

// StatusMapping associates one provider status with a normalized status.
type StatusMapping struct {
	ProviderStatus ProviderStatus
	Status         Status
}

// StatusMapper converts statuses for one provider board or workflow.
type StatusMapper struct {
	byID map[string]StatusMapping
}

// NewStatusMapper creates a mapper from provider status IDs to normalized statuses.
func NewStatusMapper(mappings []StatusMapping) StatusMapper {
	byID := make(map[string]StatusMapping, len(mappings))
	for _, mapping := range mappings {
		byID[mapping.ProviderStatus.ID] = mapping
	}
	return StatusMapper{byID: byID}
}

// ToCanonical maps a provider status to a normalized status.
func (m StatusMapper) ToCanonical(providerStatus ProviderStatus) (Status, error) {
	mapping, ok := m.byID[providerStatus.ID]
	if !ok {
		return StatusUnknown, ErrUnknownProviderStatus
	}
	return mapping.Status, nil
}

// ToProvider returns the provider statuses that represent a normalized status.
func (m StatusMapper) ToProvider(status Status) ([]ProviderStatus, error) {
	providerStatuses := make([]ProviderStatus, 0)
	for _, mapping := range m.byID {
		if mapping.Status == status {
			providerStatuses = append(providerStatuses, mapping.ProviderStatus)
		}
	}
	if len(providerStatuses) == 0 {
		return nil, ErrNoProviderStatus
	}
	sort.Slice(providerStatuses, func(i, j int) bool {
		return providerStatuses[i].ID < providerStatuses[j].ID
	})
	return providerStatuses, nil
}

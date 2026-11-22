package workitem

// Task is the provider-independent representation of a work item.
type Task struct {
	Ref            TaskRef
	Title          string
	Description    string
	Tags           []string
	Status         Status
	ProviderStatus ProviderStatus
}

// TaskRef identifies a task in an external provider.
type TaskRef struct {
	Provider string
	ID       string
	URL      string
}

// ProviderStatus preserves the status reported by the external provider.
type ProviderStatus struct {
	ID   string
	Name string
}

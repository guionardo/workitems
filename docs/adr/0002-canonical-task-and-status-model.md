# ADR-0002: Canonical task and status model

- Status: Accepted
- Date: 2026-11-22

## Context

GitHub, Jira, and GitLab represent states, columns, and transitions differently. Jira uses workflows and transitions; GitHub Projects uses configurable fields; GitLab can use lists based on labels or statuses. The library needs to provide a common view without losing the original information.

## Decision

The canonical `Task` model contains a title, description, tags, a normalized status, and the provider's original status:

```go
type Task struct {
    Title          string
    Description    string
    Tags           []string
    Status         Status
    ProviderStatus ProviderStatus
}
```

The initial canonical statuses are `todo`, `in-progress`, `done`, and `unknown`. The original status preserves at least the ID and name supplied by the provider.

Mapping is configurable for the provider, board, or workflow context. Reverse mapping may produce more than one provider status; the adapter must treat that situation as ambiguous before moving the task.

## Consequences

- Consumers can work with common statuses across providers.
- Native provider information is not lost during normalization.
- Custom statuses and workflows do not need to be forced into a universal enum.
- Adapter interfaces must distinguish status conversion from transition execution.

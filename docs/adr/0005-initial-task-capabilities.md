# ADR-0005: Initial task capabilities

- Status: Accepted
- Date: 2026-11-22

## Context

Providers differ in the operations they support. Reading tasks is broadly available, while creation, updates, status movement, board access, and pagination vary by provider and configuration. A single interface would force adapters to expose unsupported operations or return meaningless implementations.

## Decision

The initial public contracts are split into three optional capabilities:

- `TaskReader` for getting and listing tasks;
- `TaskCreator` for creating tasks from title, description, and tags;
- `TaskMover` for changing a task to a canonical status.

Adapters configure their provider, project, repository, board, or workflow context when they are constructed. Calls use `TaskRef` for existing tasks and `ListOptions` for provider-neutral filters and pagination tokens.

`TaskMover` receives a canonical status. The adapter is responsible for resolving the provider status or transition. Ambiguous or unsupported transitions must return an error instead of silently choosing a result.

## Consequences

- Consumers can depend only on the capabilities they need.
- Provider-specific scope and authentication stay outside the core call signatures.
- Pagination remains opaque to the caller through `NextPageToken`.
- Provider-specific filters and fields are deferred until a concrete adapter requires them.

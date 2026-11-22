# ADR-0008: GitHub Projects V2 provider boundary

- Status: Accepted
- Date: 2026-11-22

## Context

GitHub Issues expose only `open` and `closed` states. GitHub Projects V2 stores workflow state in a project-owned, customizable single-select field, commonly named `Status`. A project item can reference an issue or pull request, and the same issue may appear in multiple projects with different statuses.

## Decision

Implement GitHub Projects V2 as a separate adapter boundary using the provider identifier `github-projects`.

- A task reference identifies the project item; its ID is distinct from the Issue number.
- The adapter is configured for one Project V2 node ID and one status field.
- Provider status IDs are the Project V2 single-select option IDs; their names are preserved in `ProviderStatus`.
- Status mappings remain explicit through `StatusMapper`, allowing custom project workflows.
- Only Issue-backed project items are tasks in the initial implementation; pull requests and draft issues are excluded.
- Project reads and status changes use the GitHub GraphQL API. Issue creation and project-item insertion are separate capabilities and are deferred until the project-item lifecycle is defined.

## Consequences

Consumers must configure the project node ID and status mapping. A task can have different canonical statuses in different projects, which matches GitHub's project-scoped workflow model. The adapter requires a token with project access in addition to repository access.

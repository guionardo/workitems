# ADR-0016: Gitea Issues provider boundary

- Status: Accepted
- Date: 2026-11-22

## Context

Gitea issues use repository-local numeric indexes, repository labels, and a simple open/closed lifecycle. The REST API supports issue listing, creation, and updates, but does not provide a universal in-progress state for issues.

## Decision

Implement a Gitea adapter for reading, listing, creating, and moving issues between the canonical `todo` and `done` statuses. Preserve the native `open` or `closed` state in `ProviderStatus`. Resolve canonical tag names to existing repository label IDs during creation; missing labels return an error instead of being silently discarded.

## Consequences

Gitea issues do not support canonical `in-progress` without a separate project-board adapter. Pull requests, milestones, projects, and label creation remain outside this provider boundary.

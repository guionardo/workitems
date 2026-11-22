# ADR-0010: GitLab Issues provider boundary

- Status: Accepted
- Date: 2026-11-22

## Context

GitLab Issues provide title, description, labels, project-local IIDs, and `opened`/`closed` states. GitLab issue boards can add custom list semantics, but they are a separate project-level concern.

## Decision

The initial GitLab adapter targets project Issues through the REST API. It supports reading, listing, creating, and moving issues between `opened` and `closed`. These states map to canonical `todo` and `done`; `in-progress` and custom board lists are not inferred.

## Consequences

GitLab project paths and numeric IDs are accepted by the HTTP client. The adapter preserves GitLab's project-local IID and native state. A later board integration may add explicit status mapping without changing the issue client boundary.

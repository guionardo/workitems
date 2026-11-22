# ADR-0003: Pull request scope

- Status: Accepted
- Date: 2026-11-22

## Context

Pull requests look similar to tasks because they have a title, description, labels, and a lifecycle. However, creating a pull request requires source-control information such as source and target branches, commits, repository references, and provider-specific review settings.

Those requirements do not belong in the initial provider-independent task creation model.

## Decision

Pull request creation is out of scope for the initial `workitems` library. The generic task creation contract will target provider work items that can be created from the canonical task fields.

Pull request reading may be considered as a future, read-only capability, but it will not be required by the generic task interface. Pull request-specific data must remain provider-specific until a separate abstraction is justified.

## Consequences

- The initial API remains focused on creating and managing tasks/issues.
- GitHub, Gitea, and Bitbucket adapters do not need to invent source-control inputs for generic task creation.
- Pull request status and review state will not be forced into the initial task status model.
- If pull request support is added later, it should use explicit capabilities or a dedicated model rather than expanding `CreateTask` with provider-specific fields.

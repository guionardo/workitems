# ADR-0009: GitHub Projects issue-backed item creation

- Status: Accepted
- Date: 2026-11-22

## Context

The Projects V2 adapter initially deferred creation because a project item can be a draft issue, an issue, or a pull request. The canonical task model and the provider boundary already exclude draft issues and pull requests.

## Decision

`ProjectAdapter.CreateTask` creates a GitHub Issue through the existing Issues client and then adds that Issue to the configured Project V2 project with `addProjectV2ItemById`. The returned task uses the Project V2 item ID as its reference and reads the project's default status after insertion.

Creation requires both an Issues client and a Projects V2 client. If the provider returns the new item before its default status is visible, the GraphQL client performs a short, context-aware retry.

## Consequences

Created tasks are issue-backed and retain the canonical title, description, and tags. The operation can leave an Issue created in GitHub if project insertion fails; callers must treat creation as a two-step remote operation. Draft issue creation remains unsupported.

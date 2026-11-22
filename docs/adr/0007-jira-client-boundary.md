# ADR-0007: Jira client boundary

- Status: Accepted
- Date: 2026-11-22

## Context

The Jira adapter needs an HTTP client for issues, searches, and transitions. The project evaluated the existing `gh-eva` implementation and the `go-atlassian` SDK. The former is coupled to `gh-eva` authentication and logging; the latter provides provider API models that should not become part of the `workitems` public API.

## Decision

The `workitems/jira` adapter depends on a local `jira.Client` interface containing only the operations it needs. The adapter translates those results into the canonical `workitem` model.

A concrete client may be implemented with `go-atlassian` later, but the initial implementation uses the standard library. Any replacement remains behind the local interface and must preserve context handling, bounded responses, safe errors, and mutation retry rules.

## Consequences

- The core adapter is testable without network access or a Jira account.
- SDK-specific models and authentication remain outside the public API.
- Existing `gh-eva` transport and security behavior can be ported selectively.
- The standard-library client keeps the first integration dependency-free.
- An SDK bridge can be added later without changing the adapter contract.

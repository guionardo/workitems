# ADR-0014: Linear Issues provider boundary

- Status: Accepted
- Date: 2026-11-22

## Context

Linear issues belong to a team and use team-specific workflow states. The public API is GraphQL, and state identifiers are not portable across teams.

## Decision

The initial Linear adapter supports reading, listing, creating, and moving issues through the GraphQL API. It requires a configured team ID and maps workflow state IDs explicitly through `StatusMapper`. Labels are exposed as canonical tags.

## Consequences

Consumers must configure state mappings for each Linear team. The adapter does not infer state semantics from names, and projects, cycles, assignees, and webhooks remain outside this first boundary.

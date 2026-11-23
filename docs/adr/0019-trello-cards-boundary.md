# ADR-0019: Trello Cards provider boundary

- Status: Accepted
- Date: 2026-11-23

## Context

Trello represents work items as cards placed in lists. Boards can have arbitrary list names and IDs, while labels are board-scoped. Cards support the canonical title, description, tags, and movement between lists.

## Decision

Implement a Trello adapter for listing, reading, creating, and moving cards. Use Trello list IDs as provider statuses and require an explicit `StatusMapper`; resolve tag names to existing board label IDs during creation. Creation uses the list mapped to canonical `todo`.

## Consequences

Trello list IDs and label IDs are board-specific and must be configured by consumers. Card archiving, checklists, custom fields, members, and Power-Ups remain outside this provider boundary.

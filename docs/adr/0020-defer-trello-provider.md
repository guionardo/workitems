# ADR-0020: Defer Trello provider maintenance

- Status: Accepted
- Date: 2026-11-23
- Supersedes: [ADR-0019](0019-trello-cards-boundary.md)

## Context

Trello API access requires an API key tied to a Power-Up and a separately authorized user token. Obtaining the token was not practical for the current integration workflow, so the provider cannot be validated against the intended board.

## Decision

Defer Trello from the maintained provider roadmap. Keep the existing adapter as experimental code, but remove Trello from current setup guidance and do not promise compatibility stability.

## Consequences

The roadmap remains focused on providers with practical integration validation. Reintroducing Trello as a maintained provider requires a new roadmap decision and a successful authenticated board test.

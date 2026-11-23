# ADR-0017: Defer YouTrack provider

- Status: Accepted
- Date: 2026-11-22

## Context

YouTrack was listed as a candidate provider, but it is not currently a priority for the intended use cases. There is no implementation or integration test to maintain at this stage.

## Decision

Defer YouTrack from the maintained provider roadmap. Keep it as a future candidate and do not begin adapter implementation until user demand justifies API research, status-model analysis, and integration validation.

## Consequences

The active roadmap stays focused on providers with higher expected value. Supporting YouTrack later requires a new provider-boundary decision before implementation.

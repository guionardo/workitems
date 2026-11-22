# ADR-0015: Defer Linear provider maintenance

- Status: Accepted
- Date: 2026-11-22
- Supersedes: [ADR-0014](0014-linear-issues-boundary.md)

## Context

The repository contains an exploratory Linear adapter, but Linear does not currently have enough adoption in the intended use case to justify treating it as a maintained provider. Maintaining the integration would also require ongoing provider API validation and dedicated integration credentials.

## Decision

Defer Linear from the maintained provider roadmap. Keep the existing implementation as experimental code for possible future adoption, but do not include it in the current provider setup guidance or promise compatibility stability.

## Consequences

The current roadmap remains focused on providers with higher expected value. The Linear package may evolve or be removed in a future breaking change if demand does not emerge. Reintroducing it as a maintained provider requires a new roadmap decision and renewed integration validation.

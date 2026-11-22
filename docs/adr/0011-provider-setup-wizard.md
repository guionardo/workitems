# ADR-0011: Provider setup wizard as a separate CLI

- Status: Proposed
- Date: 2026-11-22

## Context

Provider setup requires identifiers that are not stable or portable across accounts: GitHub Project node IDs and status option IDs, GitLab project/board/list and label IDs, and Jira project/status identifiers. Asking users to discover these values manually makes integration setup error-prone.

## Decision

Create a separate setup wizard CLI rather than adding interactive behavior to the library packages. The wizard will:

- authenticate using environment variables or an interactive secret prompt;
- discover provider-specific identifiers through the provider APIs;
- validate read/write access needed by the selected capabilities;
- write a local, ignored environment file with configuration values;
- never print token values or commit generated secrets.

The wizard will generate configuration only; runtime adapters remain non-interactive and receive explicit configuration.

## Consequences

Provider onboarding becomes repeatable while the library remains suitable for services and automation. The CLI adds a small distribution surface and must keep discovery logic aligned with provider API changes.

# ADR-0013: Azure DevOps Work Items provider boundary

- Status: Accepted
- Date: 2026-11-22

## Context

Azure DevOps work items expose provider-defined types and workflow states. Queries use WIQL, while creation and updates use JSON Patch. State names vary by process template and work item type.

## Decision

The initial adapter targets one configured work item type, defaulting to `Task`. It supports reading, WIQL-backed listing, creation, and state updates. Provider states are preserved and mapped explicitly through `StatusMapper`; the adapter does not assume universal state names beyond example configuration.

## Consequences

Consumers must provide mappings for the selected Azure DevOps process and work item type. The client uses the `vso.work_write`-equivalent PAT/API permission for the read/write integration. Board columns and team-specific query semantics remain outside this first boundary.

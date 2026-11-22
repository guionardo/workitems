# ADR-0012: GitLab label-backed board status mapping

- Status: Accepted
- Date: 2026-11-22

## Context

GitLab issue boards commonly use labels as workflow lists. Moving an issue between label lists adds the destination label and removes the source list label. An issue can match multiple label lists, so a board context does not always identify one status.

## Decision

Add a separate `BoardAdapter` for label-backed lists. It derives provider statuses as `label:<label-name>`, with `open` and `closed` reserved for default lists. It removes other board labels and adds the mapped destination label when moving an issue.

The adapter rejects ambiguous issues and ambiguous target mappings instead of choosing a status silently. Assignee, milestone, iteration, and native status lists are deferred.

## Consequences

GitLab board workflows can represent `in-progress` without changing the canonical task model. Consumers must provide a status mapper scoped to the selected board, and board labels should map one-to-one to canonical statuses.

# ADR-0001: Module and package name

- Status: Accepted
- Date: 2026-11-22

## Context

The project abstracts tasks from different providers such as GitHub, Jira, and GitLab. The name `boards` suggests a specific Kanban board, although the product also handles tasks without a board and different workflows.

## Decision

The module is named `workitems` and the public Go package is named `workitem`:

```go
import "github.com/guionardo/workitems"
```

## Consequences

- The name represents tasks, issues, and other types of work items.
- The library is not limited to the concept of a board.
- The module path depends on the public repository `github.com/guionardo/workitems`.

Alternatives considered: `boards`, `taskbridge`, and `taskflow`. `boards` was considered too specific; `taskflow` too broad and associated with workflow engines; `taskbridge` more implementation-oriented than domain-oriented.

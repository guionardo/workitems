# Provider Capability Matrix

This matrix describes the maintained provider boundaries. `Yes` means the capability is implemented; `Pending` means implementation exists but integration validation is still missing; `Experimental` means the package is retained for future evaluation.

| Provider | Read/list | Create | Move status | Native status mapping | Integration |
| --- | --- | --- | --- | --- | --- |
| Jira | Yes | Yes | Yes | Configurable transitions | Validated |
| GitHub Issues | Yes | Yes | Yes | `open` / `closed` | Validated |
| GitHub Projects V2 | Yes | Yes (issue-backed) | Yes | Project status options | Validated |
| GitLab Issues | Yes | Yes | Yes | `opened` / `closed` | Validated |
| GitLab Boards | Yes | Yes | Yes | Board labels/lists | Validated |
| Azure DevOps | Yes | Yes | Yes | Configurable work item states | Pending |
| Gitea Issues | Yes | Yes | Yes | `open` / `closed` | Validated |
| Codeberg | Via Gitea adapter | Via Gitea adapter | Via Gitea adapter | Forgejo-compatible | Validated |
| Linear | Experimental | Experimental | Experimental | Workflow states | Deferred |
| Trello | Experimental | Experimental | Experimental | Board list IDs | Deferred |

## Generic task fields

All maintained issue/card adapters target the canonical title, description, tags, normalized status, provider reference, and original provider status. Provider-specific fields such as milestones, cycles, projects, custom fields, members, and pull-request metadata remain outside the generic model unless a dedicated capability is introduced.

## Current setup wizard coverage

The setup wizard currently discovers identifiers for GitHub Projects V2 and GitLab Boards. Jira, Azure DevOps, Gitea/Codeberg, and Bitbucket setup still require provider-specific configuration documented by each integration.

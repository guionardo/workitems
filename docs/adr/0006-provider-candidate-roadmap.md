# ADR-0006: Provider candidate roadmap

- Status: Proposed
- Date: 2026-11-22

## Context

`workitems` is intended to normalize tasks from issue trackers, project boards, and work-item systems. Providers differ in their APIs, status models, board concepts, and supported mutations. The project needs an explicit implementation order so that each adapter validates the core model without expanding it prematurely.

## Decision

The candidate providers and initial order are:

| Order | Provider | Initial scope |
| --- | --- | --- |
| 1 | Jira | Issues, labels, configurable status mappings, transitions |
| 2 | GitHub | Issues and Projects; no pull-request creation |
| 3 | GitLab | Issues, labels, issue-board lists, status mapping |
| 4 | Azure DevOps | Work items, tags, configurable states, queries |
| 5 | Gitea | Issues, labels, milestones, status mapping |
| 6 | YouTrack | Issues, tags, custom state fields, agile-board context |
| 7 | Trello | Cards, labels, lists, and list-based status |
| 8 | Bitbucket | Pull-request read-only support first; issue support only if demand justifies it |
| 9 | Forgejo | Evaluate after Gitea based on API compatibility and demand |

Linear is deferred. An exploratory implementation remains available in the repository, but it is not part of the maintained provider order until user demand justifies integration validation and compatibility maintenance.

Pull requests and merge requests are not part of the generic task-creation contract. They may be added later as explicit read-only or change-request capabilities.

## Prioritization criteria

Adapters should be prioritized by:

1. coverage of the canonical title, description, tags, and status model;
2. ability to list, get, create, and move work items;
3. quality and stability of the official API;
4. expected user value;
5. reuse of existing implementation knowledge and tests.

## Consequences

- Jira is first because `gh-eva` already contains integration knowledge and security-tested HTTP behavior.
- GitHub and GitLab validate repository-centric issue boards and project-specific state mapping.
- Azure DevOps is prioritized for its relevance and broad work-item, tag, state, and query model.
- Bitbucket PR support remains separate from generic task creation.
- The order can change through a new ADR when demand or provider constraints change.

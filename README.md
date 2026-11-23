# workitems

`workitems` is a Go library for working with tasks from different providers through a common model. It is intended to support providers such as GitHub Issues, GitHub Projects, Jira, and GitLab Issues without exposing provider-specific workflow details to consumers.

## Current status

The project currently provides the canonical task model, external task references, configurable provider-status mapping, separate task capability contracts, Jira, GitHub Issues, GitHub Projects V2, GitLab Issues, label-backed GitLab Issue Board adapters, Azure DevOps Work Items, and Gitea Issues. Pull requests, Linear support as a maintained provider, advanced credential stores, retries, and observability are not part of the current scope.

The canonical model preserves both normalized and provider-native state:

```go
task := workitem.Task{
    Title:       "Document the API",
    Description: "Add usage examples and provider notes.",
    Tags:        []string{"documentation"},
    Status:      workitem.StatusTodo,
    ProviderStatus: workitem.ProviderStatus{
        ID:   "10000",
        Name: "To Do",
    },
}
```

Provider-specific statuses can be mapped to the common `todo`, `in-progress`, `done`, and `unknown` statuses. Mappings are expected to be scoped to the relevant provider, board, or workflow.

## Future improvements

GitHub Projects V2 draft issues are a provider-specific lifecycle capability, not a canonical task status. A future optional `DraftTaskCreator` capability may create and publish draft items where the provider supports them; Jira, GitLab, and Azure DevOps should not be required to emulate this concept.

The provider setup wizard is a separate CLI tool that discovers provider-specific project, board, field, label, status, and option IDs; validates access; and generates an ignored environment file without printing tokens. Interactive secret prompts remain future work.

Linear support has been explored and remains in the repository as an experimental implementation. It is deferred from the maintained provider roadmap until there is enough demand to justify integration validation and ongoing compatibility work.

The Jira adapter can be configured with an injected status mapper and HTTP client:

```go
client := jira.NewHTTPClient(siteURL, email, apiToken, "APP")
adapter := jira.NewAdapter(client, mapper)
task, err := adapter.GetTask(ctx, workitem.TaskRef{
    Provider: "jira",
    ID:       "APP-123",
})
```

## Development

The project requires Go 1.27 or later.

```sh
go test ./...
go vet ./...
gofmt -w .
```

The setup wizard discovers provider-specific identifiers and writes a local environment file without copying the token:

```sh
GITHUB_API_TOKEN="$GITHUB_API_TOKEN" go run ./cmd/workitems-setup github-projects \
  --owner guionardo --owner-type auto --project-number 9 --output github.env

GITLAB_API_TOKEN="$GITLAB_API_TOKEN" go run ./cmd/workitems-setup gitlab-board \
  --project guionardo/workitems-testing --output gitlab.env
```

GitHub `--owner-type` accepts `user`, `organization`, or `auto` (the default).
Use `--force` only when intentionally replacing an existing file. The current wizard reads tokens from environment variables; interactive secret prompts are future work.

## Integration tests

Integration tests are opt-in. They are skipped when their required environment variables are absent, so the default `go test ./...` remains safe for local and CI runs without provider credentials.

Create local, ignored environment files with named variables. Never commit real tokens:

```sh
# github.env
GITHUB_API_TOKEN=replace-with-a-test-token
GITHUB_OWNER=guionardo
GITHUB_REPO=workitems
```

The GitHub test creates an Issue, reads it back, and closes it through the adapter. It leaves the Issue in the repository with the `workitems-integration-test` label. Run it explicitly after loading the variables:

```sh
set -a; source github.env; set +a
go test ./github -run TestGitHubIntegrationCreateAndMove -count=1 -v
```

Set `GITHUB_API_TOKEN` and `GITHUB_REPO=owner/repo` in `github.env`. Alternatively, use `GITHUB_OWNER` and a repository name separately. The test creates and closes a clearly labeled issue.

### GitHub Projects V2 identifiers

Projects V2 uses a project node ID, a status field node ID, and single-select option IDs. The visible project number (for example, `9`) is not the project ID used by GraphQL. Add these values to the local environment when the Projects V2 adapter is enabled:

```sh
GITHUB_PROJECT_NUMBER=9
GITHUB_PROJECT_ID=PVT_...
GITHUB_STATUS_FIELD_ID=PVTSSF_...
GITHUB_STATUS_TODO_OPTION_ID=...
GITHUB_STATUS_IN_PROGRESS_OPTION_ID=...
GITHUB_STATUS_DONE_OPTION_ID=...
```

Use the GitHub CLI to discover them without exposing the token:

```sh
set -a; source github.env; set +a
GH_TOKEN="$GITHUB_API_TOKEN" gh api graphql \
  -f query='query($login:String!, $number:Int!) { user(login:$login) { projectV2(number:$number) { id number title fields(first:100) { nodes { ... on ProjectV2SingleSelectField { id name options { id name } } } } } } }' \
  -F login="$GITHUB_OWNER" \
  -F number="$GITHUB_PROJECT_NUMBER"
```

Copy `projectV2.id` to `GITHUB_PROJECT_ID`, copy the `id` of the field named `Status` to `GITHUB_STATUS_FIELD_ID`, and copy the option IDs by name. For an organization-owned project, replace `user(login:$login)` with `organization(login:$login)` in the query.

The Projects V2 integration checks use these values:

```sh
go test ./github -run TestGitHubProjectsIntegrationList -count=1 -v
go test ./github -run TestGitHubProjectsIntegrationMove -count=1 -v
go test ./github -run TestGitHubProjectsIntegrationCreate -count=1 -v
```

The first test reads project items. The second finds an existing item labeled `workitems-integration-test`, moves it to `Done`, and reads it again. The third creates an Issue, adds it to the project, and moves it to `Done`. The adapter supports creating, listing, reading, and moving issue-backed project items. Pull requests and draft issues are excluded.

For Jira, provide `JIRA_HOST`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, and `JIRA_PROJECT` in `jira.env`, then run:

```sh
set -a; source jira.env; set +a
go test ./jira -run TestJiraIntegrationCreateAndMove -count=1 -v
```

The Jira test creates a clearly labeled test issue and moves it through one available transition. Both tests perform write operations and should use dedicated test projects or repositories.

For GitLab, provide `GITLAB_HOST` (optional), `GITLAB_API_TOKEN`, and `GITLAB_PROJECT` in `gitlab.env`, then run:

```sh
set -a; source gitlab.env; set +a
go test ./gitlab -run TestGitLabIntegrationCreateAndMove -count=1 -v
```

The GitLab test creates a clearly labeled issue and closes it. The plain Issues adapter supports `todo` and `done`; the BoardAdapter adds label-backed custom statuses.

With `GITLAB_BOARD_ID` and the board labels configured, the board integration test creates an issue with `todo`, moves it through `in-progress` and `done`, and verifies the canonical status:

```sh
go test ./gitlab -run TestGitLabBoardIntegrationCreateAndMove -count=1 -v
```

For Azure DevOps, provide `AZDO_PAT`, `AZDO_ORGANIZATION`, and `AZDO_PROJECT` in `azuredevops.env`. The PAT needs work-item read/write access:

```sh
set -a; source azuredevops.env; set +a
go test ./azuredevops -run TestAzureDevOpsIntegrationCreateAndMove -count=1 -v
```

Set `AZDO_STATE_TODO`, `AZDO_STATE_IN_PROGRESS`, and `AZDO_STATE_DONE` when the selected process uses custom state names. The test creates a Task and moves it to the configured done state.

For Gitea, provide `GITEA_HOST` (optional), `GITEA_API_TOKEN`, `GITEA_OWNER`, and `GITEA_REPO` in `gitea.env`, then run:

```sh
set -a; source gitea.env; set +a
go test ./gitea -run TestGiteaIntegrationCreateAndMove -count=1 -v
```

The Gitea test creates an issue and closes it. Tags must already exist in the repository because Gitea issue creation uses repository label IDs.

The same adapter was validated against Codeberg by setting `GITEA_HOST=https://codeberg.org`; use the same `GITEA_API_TOKEN`, `GITEA_OWNER`, and `GITEA_REPO` variables for a disposable Codeberg repository.

Architecture decisions are recorded in [`docs/adr`](docs/adr/README.md).

The current provider candidate order is documented in [ADR-0006](docs/adr/0006-provider-candidate-roadmap.md).

## License

This project is licensed under the MIT License. See [LICENSE.md](LICENSE.md).

# workitems

`workitems` is a Go library for working with tasks from different providers through a common model. It is intended to support providers such as GitHub Issues, GitHub Projects, Jira, and GitLab Issues without exposing provider-specific workflow details to consumers.

## Current status

The project currently provides the canonical task model, external task references, configurable provider-status mapping, separate task capability contracts, and Jira and GitHub Issues adapters with standard-library HTTP clients. GitHub Projects, pull requests, advanced credential stores, retries, and observability are not implemented yet.

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

For Jira, provide `JIRA_HOST`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, and `JIRA_PROJECT` in `jira.env`, then run:

```sh
set -a; source jira.env; set +a
go test ./jira -run TestJiraIntegrationCreateAndMove -count=1 -v
```

The Jira test creates a clearly labeled test issue and moves it through one available transition. Both tests perform write operations and should use dedicated test projects or repositories.

Architecture decisions are recorded in [`docs/adr`](docs/adr/README.md).

The current provider candidate order is documented in [ADR-0006](docs/adr/0006-provider-candidate-roadmap.md).

## License

This project is licensed under the MIT License. See [LICENSE.md](LICENSE.md).

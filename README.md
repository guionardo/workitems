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

When Jira or GitHub integration variables are available, `go test ./...` also runs the corresponding read/write integration test. Without them, the test is skipped. To run the GitHub test explicitly:

```sh
set -a; source github.env; set +a
go test ./github -run TestGitHubIntegrationCreateAndMove -count=1 -v
```

Set `GITHUB_API_TOKEN` and `GITHUB_REPO=owner/repo` in `github.env`. Alternatively, use `GITHUB_OWNER` and a repository name separately. The test creates and closes a clearly labeled issue.

To run the Jira test explicitly:

```sh
set -a; source jira.env; set +a
go test ./jira -run TestJiraIntegrationCreateAndMove -count=1 -v
```

The integration test creates a clearly labeled test issue and moves it through one available transition.

Architecture decisions are recorded in [`docs/adr`](docs/adr/README.md).

The current provider candidate order is documented in [ADR-0006](docs/adr/0006-provider-candidate-roadmap.md).

## License

This project is licensed under the MIT License. See [LICENSE.md](LICENSE.md).

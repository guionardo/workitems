# Repository Guidelines

## Project Structure & Module Organization

This is the `workitems` Go library. The public `workitem` package currently lives at the repository root, with implementation files such as `task.go` and `status_mapping.go`; co-located tests use the `*_test.go` convention. Architecture Decision Records live in `docs/adr/`. Use English for source comments, documentation, and ADRs. Keep provider adapters in focused packages or files as they are introduced, and avoid adding layers until the provider integrations require them.

## Build, Test, and Development Commands

Run these commands from the repository root:

- `go test ./...` — run the package and unit tests.
- `gofmt -w .` — format Go source files.
- `go vet ./...` — run the standard static checks.

Keep dependencies in `go.mod` and prefer the standard library unless an integration genuinely needs another package.

## Coding Style & Naming Conventions

Use `gofmt`, tabs for Go indentation, lowercase package names, and exported identifiers with GoDoc comments. Prefer small value types, explicit error handling, early returns, and descriptive names. Keep provider-specific behavior out of the canonical task model where possible.

## Testing Guidelines

Add deterministic, table-driven unit tests beside new behavior. Test observable mapping and error behavior, and run `go test ./...` before submitting changes. No coverage threshold is configured yet.

## Commit & Pull Request Guidelines

There is no Git history yet, so no existing commit convention can be inferred. Use concise imperative commit subjects (for example, `Add initial project structure`). Pull requests should explain the change, note validation performed, link any issue, and include screenshots or recordings when the change affects a user interface.

## Security & Configuration Tips

Do not commit credentials, tokens, private keys, generated secrets, or local environment files. Provide safe example configuration such as `.env.example` when runtime settings are introduced, and document required variables without including real values.

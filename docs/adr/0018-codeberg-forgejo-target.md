# ADR-0018: Codeberg as a Forgejo-compatible target

- Status: Accepted
- Date: 2026-11-23

## Context

Codeberg is a hosted Forgejo service with repository issues, labels, and the same general REST API family targeted by the Gitea adapter. A separate Codeberg package would duplicate provider behavior before compatibility differences are known.

## Decision

Treat Codeberg as a Forgejo-compatible deployment target of the Gitea adapter. Validate the existing adapter against a disposable Codeberg repository before adding any provider-specific code. Create a separate boundary only if Codeberg exposes material API or behavior differences.

## Consequences

Users can configure the existing Gitea client with `https://codeberg.org` when the API behavior is compatible. Codeberg-specific integration coverage remains future work, and the adapter must not assume compatibility without an actual test.

The Gitea integration test was validated successfully against `https://codeberg.org` using a disposable repository, confirming the initial issue create-and-close flow without Codeberg-specific code.

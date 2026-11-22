# ADR-0004: Provider bindings and optional capabilities

- Status: Accepted
- Date: 2026-11-22

## Context

The same work item may be represented by different providers, and providers do not expose the same operations. Some support creation, movement, boards, or custom fields while others support only a subset. Provider-specific identifiers and URLs must remain available without becoming the canonical task identity.

## Decision

The canonical task carries a provider reference containing the provider name, external ID, and URL. Provider-native status remains an observation through `ProviderStatus`; it does not replace the normalized `Status`.

Provider integrations will be capability-oriented. Reading, creating, updating, moving, and board/project access may be implemented as separate capabilities rather than one interface that every provider must satisfy.

Provider adapters own translation between provider payloads and the `workitem` model. Provider SDK models, authentication stores, persistence, and observability implementations must not leak into the public core model.

## Consequences

- A task can be traced back to its provider without making the provider identity canonical.
- Unsupported operations can be reported explicitly instead of being simulated.
- Future adapters can expose only the operations their provider supports.
- The public operation interfaces remain to be defined after board/project context and pagination are settled.

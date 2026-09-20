# Workflow Harness

Workflow Harness is a private, local-first development workspace for coordinating GitHub work, Codex Agent Roles, isolated Project execution, quality gates, delivery through Coolify, and durable project knowledge.

The repository currently contains the agreed architecture and implementation roadmap. Runtime code has not been started.

## Core decisions

- GitHub is authoritative for repositories, Issues, Projects, pull requests, checks, and releases.
- A single Harness Workspace links Projects as Git submodules under `projects/`.
- Each Work Item runs in an isolated Project worktree.
- The daemon and CLI are implemented in Go and run locally as a user service.
- Codex CLI uses local authentication and isolated Agent Threads; untrusted Project commands run through Docker.
- SQLite stores reconstructible operational state, Commands, Events, projections, leases, and telemetry summaries.
- Project knowledge is stored natively as Open Knowledge Format bundles under `.knowledge/`.
- Production is the only deployed acceptance environment; unaccepted Features require a bounded Exposure Strategy.
- Coolify access, GitHub mutation, Docker execution, and other privileged effects cross narrow brokers.
- Human Gates protect merge, deployment, destructive data changes, credentials, policy, and self-modification.

## Documentation

- [Domain language](./CONTEXT.md)
- [Architecture](./docs/architecture.md)
- [Workflow](./docs/workflow.md)
- [Security model](./docs/security.md)
- [Quality gates](./docs/quality-gates.md)
- [Model routing](./docs/model-routing.md)
- [Structured contracts](./docs/contracts.md)
- [Implementation roadmap](./docs/implementation-plan.md)
- [Architecture decisions](./docs/adr/)
- [Feature flag provider research](./docs/research/feature-flag-providers.md)

# Workflow Harness

Workflow Harness is a private, local-first development workspace for coordinating GitHub work, Codex Agent Roles, isolated Project execution, quality gates, delivery through Coolify, and durable project knowledge.

The repository ships the Go runtime module by module: Workflow core, operational journal, CLI and registry, GitHub intake, workspace isolation, containerized Project Runner, Codex threads, the Product → Implementer → Reviewer loop, PR and release policy, the Coolify broker, flag exposure, Telegram commands, the loopback dashboard, OKF knowledge, telemetry with routing evals, and the `go-service`, `python-service`, and `nextjs-web` Stack Adapters. The `harness` CLI (`cmd/harness`) currently registers, validates, and lists Projects.

## Verification

Run the full suite with the race detector, vet, and build:

```bash
go test -race ./...
go vet ./...
go build ./...
```

Docker-gated tests run against the local daemon with `HARNESS_RUNNER_DOCKER=1`; the Codex fixture stays double-gated behind `HARNESS_CODEX_FIXTURE=1` plus `HARNESS_CODEX_LIVE=1` so no model budget is spent by default; the live Coolify broker needs `HARNESS_COOLIFY_URL` plus `HARNESS_COOLIFY_TOKEN`. Per-increment evidence lives in [the implementation roadmap](./docs/implementation-plan.md).

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

# Workflow Harness v1 roadmap

The v1 is delivered as vertical, independently demonstrable increments. Each increment receives its own executable TDD plan immediately before implementation so later plans incorporate evidence from earlier slices.

| Increment | Demonstrable outcome |
|---|---|
| 1. Workflow core | Commands deterministically produce Events and rebuild State |
| 2. Operational journal | SQLite persistence, projections, leases, migrations, recovery |
| 3. CLI and registry | Projects can be registered, validated, and inspected |
| 4. GitHub intake | Issues and Project state reconcile into Work Items |
| 5. Workspace isolation | Submodule Project worktree and safe Git configuration |
| 6. Go Project Runner | Go Gates execute in restricted Docker Tasks |
| 7. Codex integration | Structured local Agent Thread runs against one fixture |
| 8. Role loop | Product → Implementer → Reviewer with bounded fix cycles |
| 9. Pull request and release | Preserved commits, PR evidence, merge, immutable Release |
| 10. Coolify delivery | Brokered deploy, health evidence, rollback |
| 11. Controlled exposure | GO Feature Flag spike, targeting, acceptance, rollout, cleanup |
| 12. Telegram | Authenticated notifications and version-bound Commands |
| 13. Dashboard | Loopback operator UI over projections and Commands |
| 14. Knowledge | OKF retrieval, FTS index, proposals, verification |
| 15. Observability and evals | OpenTelemetry, cost attribution, model-routing baseline |
| 16. Additional stacks | Python service and Next.js web adapters |

## v1 acceptance scenario

The release is complete only when a Go fixture Project passes the full scenario documented in [workflow.md](./workflow.md): external Issue through Triage, Spec, Codex implementation, containerized Gates, independent review, preserved commits, PR approval, immutable release, Coolify deploy, controlled client exposure, acceptance, rollout, scheduled flag removal, and OKF proposal—with restart recovery and no secret leakage.

## Plan index

- [Increment 1: Workflow core](./superpowers/plans/2026-09-20-workflow-core.md)

Later plans are intentionally created after the preceding increment is verified. This avoids fixing database, integration, or adapter details before the domain interface has executable evidence.

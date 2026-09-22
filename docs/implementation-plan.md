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
- [Increment 2: Operational journal](./superpowers/plans/2026-09-21-operational-journal.md)
- [Increment 3: CLI and registry](./superpowers/plans/2026-09-21-cli-registry.md)
- [Increment 4: GitHub intake](./superpowers/plans/2026-09-22-github-intake.md)
- [Increment 5: Workspace isolation](./superpowers/plans/2026-09-22-workspace-isolation.md)

## Increment 1 verification

Verified 2026-09-21 in the `feat/workflow-core` Project Worktree with Go 1.27.1.
Commands deterministically produce Events and Work Item State rebuilds only by folding Events.

- [x] `go test ./internal/workflow -run TestHappyPathReplay -count=1` — PASS (Gatekeeper + Workflow happy path SubmitWork through CompleteRollout ends in `done`, Version equals Event count, replay identical)
- [x] `go test ./internal/workflow -run '^$' -fuzz FuzzFoldNeverReturnsInvalidState -fuzztime 10s` — PASS (~9.6M execs, no panic, no failing corpus entry)
- [x] `go test -race ./...` — PASS (317 specs, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings

## Increment 2 verification

Verified 2026-09-21 in the `feat/operational-journal` Project Worktree with Go 1.27.1.
Persisted Events rebuild identical Work Item State after close/reopen and stale Commands never mutate State.

- [x] `go test ./internal/journal -run TestRestartRecoversHappyPath -count=1` — PASS (Gatekeeper + Apply loop SubmitWork through CompleteRollout ends in `done`, Version equals Command count, reopened Load identical, stale Apply errors without mutation)
- [x] `go test -race ./...` — PASS (324 specs, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings

## Increment 3 verification

Verified 2026-09-21 in the `feat-cli` Project Worktree with Go 1.27.1.
Register then validate then list agree on one fixture Project and escaping paths never persist a registry.

- [x] `go test ./internal/cli -run TestRegister -count=1` — PASS (register→manifest+gitmodules fixture→validate lists OK demo, list agrees, escape rejected without persisting)
- [x] `go test -race ./...` — PASS (351 passed in 6 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

## Increment 4 verification

Verified 2026-09-22 on `master` with Go 1.27.1.
Open Issues submit new Work Items and Project status advances them one step; closed Issues record intent without mutating State and terminal items never resurrect by intake.

- [x] `go test ./internal/github -run 'TestClosed|TestBlocked|TestTerminal|TestIntake' -count=1` — PASS (closed intent without commands, blocked never jumps, terminal never resurrects, stable command ids)
- [x] `go test -race ./...` — PASS (375 passed in 7 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

## Increment 5 verification

Verified 2026-09-22 on `master` with Go 1.27.1 and git 2.53.0.
Project Worktrees prepare detached at the submodule HEAD with generated Git configuration (empty hooks path, cleared credential helper, deny-by-default environment); local Execution commits never move the registered submodule reference and Close removes the worktree registration.

- [x] `go test ./internal/workspace -run 'TestPrepareSanitizes|TestPreparedHooks' -count=1` — PASS (secret-bearing variables absent, hardening vars present, hooksPath enforced, no credential helper)
- [x] `go test -race ./...` — PASS (390 passed in 8 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

Later plans are intentionally created after the preceding increment is verified. This avoids fixing database, integration, or adapter details before the domain interface has executable evidence.

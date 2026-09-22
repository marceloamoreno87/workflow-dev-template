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
- [Increment 6: Go Project Runner](./superpowers/plans/2026-09-22-go-runner.md)
- [Increment 7: Codex integration](./superpowers/plans/2026-09-22-codex-integration.md)
- [Increment 8: Role loop](./superpowers/plans/2026-09-22-role-loop.md)
- [Increment 9: Pull request and release](./superpowers/plans/2026-09-22-pr-release.md)
- [Increment 10: Coolify delivery](./superpowers/plans/2026-09-22-coolify-delivery.md)
- [Increment 11: Controlled exposure](./superpowers/plans/2026-09-22-controlled-exposure.md)

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

## Increment 6 verification

Verified 2026-09-22 on `master` with Go 1.27.1 and Docker 29.8.0.
Declared Tasks run in restricted containers with runner-owned flags only: digest-pinned image, no shell, detached-equivalent hardening (`--network none`, numeric user, `cap-drop ALL`, `no-new-privileges`, pids limit, equal memory/swap, read-only rootfs, single worktree bind mount), bounded timeout with best-effort removal, capped output, and secrets never echoed in errors.

- [x] `go test ./internal/runner -run 'TestRunTruncatesOutput|TestRunRealDocker' -count=1` — PASS (truncation flagged at 64 KiB cap, gate closed skips)
- [x] `HARNESS_RUNNER_DOCKER=1 go test ./internal/runner -run 'TestRunRealDocker' -count=1` — PASS (digest-pinned `pgvector/pgvector` fixture runs `/bin/echo` for real, exit 0)
- [x] `go test -race ./...` — PASS (419 passed in 9 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

## Increment 7 verification

Verified 2026-09-22 on `master` with Go 1.27.1 against codex-cli 0.155.0 flags.
One Agent Thread runs through the real `codex exec` interface with validated envelopes: classification `confidential`/`restricted` never spawns, sandbox is always `workspace-write` on the fixture worktree, auth rides on `CODEX_HOME` with user config ignored, transcripts are capped and parsed, and the final message must match the embedded output schema. Model output stays inert data.

- [x] `go test ./internal/codex -run 'TestTranscriptIsCapped|TestClassifiedSpecNeverSpawns|TestFixturePathIsGated' -count=1` — PASS (2500-event transcript capped at 2000 with flag, classified spec never reaches the binary, fixture path skips without gates)
- [x] `go test -race ./...` — PASS (447 passed in 10 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

Live-model fixture run is operator-gated (external model cost is never spent by default): with local auth present, `HARNESS_CODEX_FIXTURE=1 HARNESS_CODEX_LIVE=1 go test ./internal/codex -run TestFixturePathIsGated -count=1 -v` exercises the Task 3 `Run` path against a fixture worktree.

## Increment 8 verification

Verified 2026-09-22 on `master` with Go 1.27.1.
Product → Implementer → Reviewer runs as a deterministic loop over explicit events: complete Specs select only required Agent Roles, green Gates advance, approvals complete, and every blocking rule from the development loop holds (three fix cycles, repeated failure, policy conflict, scope expansion, budget breach, deadline expiry).

- [x] `go test ./internal/loop -run 'TestHappyPathLoop|TestBlockedLoop' -count=1` — PASS (happy path ends done with cost 6, exhausted loop records reason and rejects further events)
- [x] `go test -race ./...` — PASS (466 passed in 11 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

## Increment 9 verification

Verified 2026-09-22 on `master` with Go 1.27.1.
Pull requests and releases are governed as pure policy: own Execution commits reorganize only before creation, third-party ranges never rewrite, merges require operator approval with green checks through history-preserving merge commits, releases are strict semver with digest-pinned artifacts, and every Gate leaves validated Evidence.

- [x] `go test ./internal/delivery -run 'TestSeparateFactsFlow|TestMergeEvaluatesWithoutMutating' -count=1` — PASS (approval/merge/release stay separate facts, merge evaluates without mutating)
- [x] `go test -race ./...` — PASS (505 passed in 12 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

## Increment 10 verification

Verified 2026-09-22 on `master` with Go 1.27.1 against `httptest` stub servers.
Releases deploy through a typed Capability Broker with the Feature disabled, health arrives as structured evidence, and rollback carries an auditable reason; credentials travel in one header and never surface in errors, responses are capped, and every call is deadline-bound.

- [x] `go test ./internal/coolify -run 'TestDeliverFlow|TestOversized|TestDeadline|TestLiveBroker' -count=1` — PASS (deploy→red health→rollback flow, oversized rejection, deadline enforcement, live gate skips)
- [x] `go test -race ./...` — PASS (527 passed in 13 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

## Increment 11 verification

Verified 2026-09-22 on `master` with Go 1.27.1.
Features expose through validated declarations with deterministic offline evaluation (FNV-1a bucketing, kill switch always off), staged rollouts ending at 100, contributor-only acceptance (reject disables), and removal deadlines that become tracked debt after 14 days.

- [x] `go test ./internal/flags -run 'TestExposureFlow|TestRejectedAcceptanceDisables' -count=1` — PASS (targeted→accept→rollout→debt flow, reject disables and stays off)
- [x] `go test -race ./...` — PASS (546 passed in 14 packages, zero failures, zero race reports)
- [x] `go vet ./...` — exit 0, no findings
- [x] `go build ./...` — clean build of `cmd/harness`

Later plans are intentionally created after the preceding increment is verified. This avoids fixing database, integration, or adapter details before the domain interface has executable evidence.

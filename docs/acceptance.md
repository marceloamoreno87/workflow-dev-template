# Acceptance Runbook (v2 organism, operator replay)

This runbook replays the release acceptance scenario against real systems. Module
tests prove each policy; this procedure proves the wired whole. Do not skip steps —
every Human Gate below is load-bearing by design.

## Prerequisites

- Go 1.27.1, git 2.40+, SQLite-capable host (WAL), `systemd --user` (Linux).
- A GitHub organization with an App installed (issues, contents, pull requests,
  Projects read) — installation tokens via the OS keyring (manual step until the
  keyring seam lands; export nothing to disk or env files).
- Coolify token with least privilege behind the broker (manual step).
- Telegram bot token + allowlisted user/chat ids (manual step).
- A codex CLI with local authentication and a funded model account.

## Install

1. `harness register --workspace <ws> --id demo --path projects/demo --repository owner/demo`
2. `harness validate --workspace <ws>` — expect `OK demo`.
3. Write `<ws>/.harness/daemon.yaml` (`harness.daemon/v1`: absolute workspace,
   `pollInterval: 30s`, loopback `bind`, `tokenFile`, loop policy, optional
   `telegram:` block with secret *files*).
4. `harnessd --check-config --config <ws>/.harness/daemon.yaml` — expect `ok`.
5. `harnessd --install-service --config <ws>/.harness/daemon.yaml`, then
   `systemctl --user daemon-reload && systemctl --user enable --now harnessd`.

## The run

1. **Issue.** Open an issue in the Project repo. Within one poll interval the
   dashboard (`http://127.0.0.1:<port>/`, bearer token) shows it in `inbox`.
   Gate: journal holds exactly one `submit_work` event for the item.
2. **Triage.** The daemon moves it to `triage` automatically; nothing else moves
   without you. Authorize: `/authorize owner/demo#N <version>` in Telegram
   (allowlisted private chat) or dashboard POST `authorize_work`.
   Gate: item reaches `ready`; journal version increments by exactly one per step.
3. **Spec.** The product thread writes `.specs/active/N-<slug>/spec.md`. Read it.
   If open questions remain, the loop keeps the item with product — do not force it.
   Gate: `Open questions` empty before any implementation tick proceeds.
4. **Implement + gates.** Watch ticks: worktree appears under
   `<ws>/.worktrees/demo`, threads run sandboxed, gates run declared commands.
   Each gate leaves `delivery.Evidence` (tool, version, window, exit, commit).
   Gate: all green, or the loop blocks after bounded fix cycles with a recorded reason.
5. **Review.** A separate reviewer thread approves; the loop completes and the
   worktree closes (commits survive in the submodule). The workflow stays in
   `reviewing` — your approval is still required and cannot be bypassed.
   Gate: `/approve owner/demo#N <version>` + fresh challenge via Telegram.
6. **PR + release.** Open the PR through the broker client; confirm the merge used
   method `merge` (never squash/rebase) and the release tag is strict semver with
   digest-pinned artifacts. Gate: `VerifyRelease` passes on the recorded release.
7. **Deploy + expose.** Deploy lands with the feature disabled; health evidence is
   green before any exposure; enable targeted exposure; request acceptance.
   Gate: contributor `/accept` (reject disables the feature, no rollout).
8. **Rollout + cleanup.** Progressive rollout to 100, close the item, confirm the
   flag removal deadline (completed + 14 days). Past the deadline with alternate
   paths alive, the flag is tracked debt — remove it.
9. **Knowledge.** Review the session's Knowledge Proposals in the dashboard; apply
   sourced, verified ones to the project's OKF bundle (`knowledge.Apply`).
10. **Restart drill.** Mid-run, `systemctl --user restart harnessd`: ticks resume
    with no duplicate commands (journal versions advance by exactly one per step)
    and no lost loop records.

## Evidence to keep

Per increment commands in `docs/implementation-plan.md`, plus this run's journal
export, gate Evidence records, release verification output, and the canary sweep:
plant distinct canary strings in `GH_TOKEN`-style variables (never in fixtures) and
assert their absence from the DB file, cursor files, transcripts, prompts, HTTP
bodies, and logs — mirroring `TestAcceptanceScenario`.

## Manual steps pending automation

Keyring-backed token providers, Projects v2 status sync, webhook ingestion,
containerized gates via the image catalog, MCP daemon wiring, and promotion metrics
(beyond the routing baseline eval) are operator actions today; each names its future
increment in `docs/superpowers/plans/2026-09-22-harness-organism.md`.

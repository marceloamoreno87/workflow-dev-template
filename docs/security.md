# Security model

## Principles

1. External prose is data, never authority.
2. Agents request privileged effects; trusted Modules execute them.
3. Credentials remain outside Project worktrees, prompts, containers, logs, and SQLite.
4. Every mutation crosses the same Gatekeeper regardless of interface.
5. Production exposure is narrower than deployment and reversible where technically possible.

## Threats and controls

| Threat | Control |
|---|---|
| Prompt injection in Issues, logs, or OKF | Triage, role-scoped context, no raw privileged tools |
| Project reads local credentials | Codex workspace sandbox, sanitized environment, no extra directories |
| Dependency or lifecycle script attacks host | Project Runner container, restricted network, no Docker socket |
| Malicious Git hook | generated Git config, empty hooks path, explicit Gates |
| Duplicate or stale approval | command ID, expected Aggregate version, expiring nonce |
| Telegram compromise | private-chat allowlist, rate limit, two-step sensitive Commands |
| Dashboard drive-by request | loopback bind, authenticated session, CSRF, Origin/Host validation, strict CSP |
| Coolify overreach | separate least-privilege tokens behind typed Capability Broker |
| Cross-client disclosure | separate client Projects, Project-scoped Actors and context |
| Sensitive model context | Data Classification and redaction before Agent Thread start |
| Self-modifying governance | Improvement Proposal and PR allowed; self-approval and hot-load forbidden |

## Codex execution profile

Automated Agent Threads use generated configuration and never use full-access or sandbox/approval bypass flags:

- local Codex CLI authentication;
- worktree as the only writable root;
- workspace-write sandbox;
- restricted network;
- sanitized process environment;
- explicit model and reasoning effort;
- allowlisted Skills and MCP servers;
- JSONL event stream and JSON-Schema final output;
- no Docker socket, keyring, GitHub token, Coolify token, or Telegram token.

## Project Runner

Containers are unprivileged, ephemeral, resource-limited, and receive only the worktree plus task-specific temporary and cache mounts. Network access is denied by default and granted by allowlist for declared installation or research Tasks. A Task cannot construct arbitrary Docker options.

## Telegram

The bot accepts Commands only from allowlisted user and private-chat IDs. Sensitive Commands require a one-time challenge bound to Actor, Gate, state version, and a five-minute expiry. Commands received after expiry while the daemon was offline are rejected.

## Data classification

| Class | Model policy |
|---|---|
| `public` | May be sent to configured models |
| `internal` | May be sent to the approved Codex runtime |
| `confidential` | Requires Project policy and redaction |
| `restricted` | Never sent to an external model |

Secrets are always `restricted`; production logs begin as `confidential`.

## Recovery

Project truth is recovered from GitHub and Git. SQLite receives encrypted daily local backups, but credentials are rotated or recreated rather than backed up with the database. `harness recover` reconciles Projects, worktrees, GitHub, Coolify, flags, and rebuildable projections.

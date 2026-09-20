# Structured contracts

All documents below carry `schemaVersion`. Schemas live under `schemas/` and are validated at every trust seam.

## Project manifest

Path: `.harness/project.yaml`

```yaml
schemaVersion: harness/v1
project:
  id: example
  repository: owner/repo
  defaultBranch: main
stack:
  adapter: go-service
quality:
  profile: standard
data:
  classification: internal
knowledge:
  root: .knowledge
specs:
  root: .specs
delivery:
  coolifyResource: resource-id
  productionUrl: https://example.com
flags:
  provider: go-feature-flag
  declarationRoot: .flags
budgets:
  maxDuration: 2h
  maxCostUSD: 10
  maxFixCycles: 3
```

The manifest contains policy and non-secret references only. Executable commands belong to versioned Stack Adapters.

## Workspace registry

Path: `.harness/registry.yaml`

```yaml
schemaVersion: harness.registry/v1
projects:
  - id: example
    path: projects/example
    github:
      repository: owner/repo
      clientProject: PVT_client
      portfolioProject: PVT_private
    actors:
      operator: actor/operator
      clients: [actor/client-a]
```

Paths must resolve below `projects/`, match a registered submodule, and agree with the Project manifest.

## Spec

Path: `.specs/active/<issue>-<slug>/spec.md`

```yaml
---
schemaVersion: harness.spec/v1
workItem: owner/repo#123
qualityProfile: standard
dataClassification: internal
requiredRoles: [product, backend, qa, reviewer]
humanGates: [merge, production-deploy, client-acceptance]
---
```

Required sections are Problem and desired outcome, Scope and non-scope, Verifiable scenarios and acceptance criteria, Constraints and risks, Affected interfaces, Exposure/rollout/rollback strategy, and Open questions. Open questions must be empty before implementation.

## Agent Role envelope

Input contains role, objective, Spec reference, selected OKF context, writable paths, tools, network policy, classification, budget, deadline, and completion criteria. Output contains status, summary, changes, Evidence references, blockers, usage, Knowledge Proposals, and privileged action requests. Free text cannot advance Workflow state.

## Command envelope

```json
{
  "schemaVersion": "harness.command/v1",
  "commandId": "uuid",
  "aggregateId": "work-item-id",
  "expectedVersion": 12,
  "actorId": "actor/operator",
  "type": "ApproveGate",
  "payload": {},
  "issuedAt": "RFC3339 timestamp"
}
```

Commands are idempotent. A stale `expectedVersion` returns a conflict and the current Actions.

## Event envelope

Events include event ID, aggregate ID and version, command/causation/correlation IDs, Actor, type, versioned payload, timestamp, and redacted telemetry references. Events are immutable; projections are disposable.

# Development workflow

## Work Item state machine

```mermaid
stateDiagram-v2
    [*] --> Inbox
    Inbox --> Triage
    Triage --> Ready: operator authorizes
    Triage --> Cancelled: reject
    Ready --> Specifying
    Specifying --> Ready: Spec approved
    Ready --> Implementing
    Implementing --> Reviewing: implementation gates pass
    Reviewing --> ChangesRequested: review rejects
    ChangesRequested --> Implementing
    Reviewing --> ReadyToDeploy: PR approved and merged
    ReadyToDeploy --> Deploying
    Deploying --> ClientQA: release healthy and exposure bounded
    ClientQA --> ChangesRequested: contributor rejects
    ClientQA --> RollingOut: contributor accepts
    RollingOut --> Done: rollout complete
    Done --> [*]
    Inbox --> Blocked
    Triage --> Blocked
    Specifying --> Blocked
    Implementing --> Blocked
    Reviewing --> Blocked
    Deploying --> Failed
    ClientQA --> Blocked
    Blocked --> Ready: blocking decision resolved
    Failed --> ReadyToDeploy: retry authorized
```

GitHub Project status is a projection and command surface. The Workflow Module owns valid transitions. Closing an Issue during an open Acceptance Gate records contributor intent but cannot bypass pending technical Gates.

## Delivery sequence

```mermaid
sequenceDiagram
    participant O as Operator
    participant H as Harness
    participant G as GitHub
    participant C as Coolify
    participant F as Flag provider
    participant U as Client contributor
    H->>G: Open PR with evidence
    O->>G: Approve PR
    G->>G: Merge preserving commits
    H->>G: Create immutable Release
    H->>C: Deploy Feature disabled
    C-->>H: Health and smoke evidence
    H->>F: Enable targeted exposure
    H->>U: Request Acceptance Gate
    U-->>H: Approve or request changes
    alt accepted
        H->>F: Progressive rollout
        H->>G: Close Work Item
        H->>G: Schedule flag removal within 14 days
    else rejected
        H->>F: Disable Feature
        H->>G: Record Changes Requested
    end
```

## Development loop

1. Triage treats all external content as untrusted.
2. Product produces a complete Spec with no implementation-blocking open questions.
3. The orchestrator selects only required Agent Roles.
4. Independent work may run concurrently in isolated worktrees.
5. Quality Gates produce structured Evidence.
6. Reviewer and Security use independent Agent Threads.
7. Fix cycles resume the Implementer while measurable progress continues.
8. The loop blocks after three cycles without progress, two repetitions of the same failure, a budget breach, policy conflict, or scope expansion.
9. Execution commits may be reorganized before PR creation; third-party commits are never rewritten.
10. Human approval, release, deploy, exposure, client acceptance, rollout, and flag cleanup remain separate facts.

## Quality Profile inheritance

`global default → Project minimum → Work Item override`

A Work Item may increase rigor. Lowering below the Project minimum requires a recorded Human Gate. An Agent Role cannot reduce its own Gates.

## Human Gates

Human approval is mandatory for:

- merge into a protected branch according to profile;
- production deployment according to profile;
- first infrastructure creation or structural infrastructure change;
- destructive or irreversible data migration;
- permission, secret, authentication, or security policy changes;
- rollback with possible data loss;
- configured cost overruns;
- self-modification of Harness controls;
- reduction or bypass of any Gate.

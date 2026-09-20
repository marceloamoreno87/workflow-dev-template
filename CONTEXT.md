# Workflow Harness

This context defines the language of the local workspace that coordinates agent-assisted development across linked Projects.

## Workspace and work

**Harness Workspace**:
The single operational workspace that contains shared coordination, policy, and tooling for linked Projects.
_Avoid_: platform, main project, template repository

**Project**:
An independently versioned software repository linked to the Harness Workspace and treated as one product and history boundary.
_Avoid_: folder, package, module

**Project Worktree**:
An isolated, disposable checkout in which an Execution may change a Project without moving the Project revision registered by the Harness Workspace.
_Avoid_: clone, sandbox, branch

**Work Item**:
The unit of intent and tracking represented by a GitHub Issue and projected into one or more GitHub Projects.
_Avoid_: task, card, ticket

**Spec**:
The versioned description of a Work Item's expected behavior, scope, constraints, and acceptance criteria.
_Avoid_: brief, prompt, plan

**Execution**:
One traceable attempt to move a Work Item through the development workflow using authorized Agent Roles and tools.
_Avoid_: session, conversation, job

**Triage**:
The operator decision that converts an untrusted external Work Item into authorized work ready for specification and execution.
_Avoid_: classification, refinement, final approval

## People and agents

**Actor**:
A human or automated identity recognized by the Harness as the origin of Commands and Events.
_Avoid_: user, client, Agent Role

**Contributor**:
A person outside development who may submit needs and participate in explicitly assigned Gates.
_Avoid_: client, system user, developer

**Agent Role**:
A specialized set of responsibilities, capabilities, and limits exercised during an Execution.
_Avoid_: agent, persona, bot

**Agent Thread**:
An isolated Codex session that exercises one Agent Role for one Work Item and may resume only within those limits.
_Avoid_: Agent Role, Execution, conversation

## Workflow

**Command**:
An authenticated intent to attempt a state change under the current workflow rules.
_Avoid_: event, message, shell command

**Event**:
An immutable fact recorded after processing a Command or reconciling an external observation.
_Avoid_: log, Command, notification

**Action**:
An intent currently permitted by state and Gates and suitable for presentation to an Actor or integration.
_Avoid_: Command, transition, button

**Gate**:
A verifiable condition that must be satisfied before a workflow transition may occur.
_Avoid_: step, checklist, approval

**Human Gate**:
A Gate whose decision requires explicit operator authorization.
_Avoid_: manual review, pause

**Acceptance Gate**:
A Human Gate in which a Contributor confirms that delivered behavior satisfies observable acceptance criteria.
_Avoid_: technical QA, closing a task, code approval

**Quality Profile**:
The declared rigor applied to a Project or Work Item: `prototype`, `standard`, or `critical`.
_Avoid_: mode, environment, tier

**Orchestration State**:
Reconstructible operational state used to queue, resume, and coordinate Executions without becoming permanent project truth.
_Avoid_: memory, knowledge base, project state

## Knowledge

**Knowledge Base**:
The searchable projection of authoritative Project knowledge and derived operational history.
_Avoid_: source of truth, independent wiki

**Knowledge Bundle**:
A Project's Open Knowledge Format corpus with explicit provenance, trust, and freshness.
_Avoid_: documentation folder, wiki, vector database

**Knowledge Concept**:
One permanent, versioned unit of knowledge in a Knowledge Bundle.
_Avoid_: document, page, memory

**Observation**:
A provisional operational fact captured during an Execution and subject to expiry, with no permanent authority.
_Avoid_: memory, learning, knowledge

**Knowledge Proposal**:
A candidate Knowledge Bundle change with sources that awaits impact-appropriate verification.
_Avoid_: Observation, automatic annotation, decision

**Improvement Proposal**:
A Work Item opened by the Harness to suggest an evidence-backed improvement without authority to approve or activate its own change.
_Avoid_: self-update, Knowledge Proposal, automatic fix

## Delivery and infrastructure

**Stack Adapter**:
The contract that translates abstract Gates and operations into tools for a supported development stack.
_Avoid_: project template, plugin, preset

**Project Runner**:
The Module that executes declared Project Gates and Tasks inside controlled Docker environments.
_Avoid_: shell, Docker wrapper, Agent Role

**Capability Broker**:
A controlled seam that offers validated infrastructure operations without exposing credentials or raw administrative tools.
_Avoid_: proxy, direct MCP, secret manager

**Feature**:
An identifiable behavior change that can be exposed to a defined audience and evaluated independently from its deploy.
_Avoid_: branch, deploy, Work Item

**Feature Flag**:
A temporary operational control that determines Feature availability for a defined audience and context.
_Avoid_: permanent configuration, permission, environment variable

**Exposure Strategy**:
The mechanism that limits who can observe a Feature and which effects it may produce before full rollout.
_Avoid_: deploy, Feature Flag, environment

**Flag Debt**:
A Feature Flag that has completed rollout but still preserves alternate paths after its removal deadline.
_Avoid_: backlog, legacy configuration, generic technical debt

**Data Classification**:
The category controlling whether content may be sent to agent runtimes: `public`, `internal`, `confidential`, or `restricted`.
_Avoid_: visibility, permission, Quality Profile

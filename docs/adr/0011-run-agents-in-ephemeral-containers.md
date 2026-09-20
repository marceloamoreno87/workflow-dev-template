---
status: superseded by ADR-0028
---

# Run agents in ephemeral containers

Each Agent Role runs inside an unprivileged ephemeral Docker container with only its Project Worktree and task-specific temporary storage mounted. The container receives bounded compute and network capabilities but no host keyring, Docker socket, infrastructure credentials, or unrestricted access to sibling Projects; privileged operations cross typed daemon brokers.

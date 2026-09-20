# Isolate Git effects and network credentials

Agent Threads may create and reorganize only their Execution's local commits using generated repository configuration with Project hooks and global includes disabled. The daemon verifies ancestry and scope, authenticates network pushes through the GitHub App, and enforces branch and force-push policy. Commit-time checks are explicit Quality Gates, so repository-controlled hooks and lifecycle scripts never execute on the host merely because an agent invokes Git.

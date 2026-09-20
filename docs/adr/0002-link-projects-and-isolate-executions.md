# Link Projects and isolate Executions

The single Harness Workspace records each Project as a Git submodule, while mutable work for an Execution occurs in an isolated worktree of that Project. This separates the Project revision intentionally registered by the workspace from temporary branches and concurrent agent changes, preventing routine execution from silently moving submodule references.

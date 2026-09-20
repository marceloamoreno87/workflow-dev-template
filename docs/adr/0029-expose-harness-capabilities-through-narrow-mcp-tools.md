# Expose Harness capabilities through narrow MCP tools

Agent Threads communicate with the daemon through per-Execution authenticated MCP tools for reading assigned work, retrieving Specs and knowledge, running declared Quality Gates, requesting Human Gates or delivery actions, and proposing knowledge. The MCP server does not expose generic shell, SQL, Docker, Telegram, arbitrary GitHub mutation, unrestricted Coolify MCP, or credentials; every mutating request becomes a validated Command processed by the same Gatekeeper used by human interfaces.

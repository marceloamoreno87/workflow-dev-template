# Broker Coolify capabilities with separate tokens

The daemon mediates Coolify operations through a Capability Broker backed by separate least-privilege tokens for observation, deployment, and configuration. Agent runtimes never receive tokens or unrestricted MCP access; the broker validates typed operations, enforces Human Gates, and records an audit event before invoking Coolify. This limits the impact of untrusted Issue, log, and documentation content while retaining automated infrastructure workflows.

# Run orchestration in a local daemon

The Harness Workspace uses a CLI as its human interface and a persistent local daemon as its orchestration runtime. This keeps operation local and single-user while allowing queued work, resumable executions, parallel agents, and durable telemetry that a synchronous CLI process could not reliably provide.

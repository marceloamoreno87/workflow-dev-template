# Create only proven Module seams

The Go design introduces seams only where behavior already varies: Stack Adapters, Flag Providers, Secret Stores, clocks, process runners, and telemetry exporters. GitHub, Codex, and Coolify remain deep Modules without speculative generic interfaces until a second implementation exists. Workflow, Gatekeeper, Execution, Workspace, Knowledge, and Delivery hide policy and orchestration behind small test surfaces instead of exposing infrastructure steps to callers.

# Capture local telemetry with OpenTelemetry

The Harness emits OpenTelemetry traces and metrics, keeps queryable execution summaries in SQLite, and rotates detailed structured logs after 30 days. Export to an external observability backend remains optional, preserving local-first operation without coupling telemetry semantics to the first user interface.

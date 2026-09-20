# Drive workflow with commands and events

The daemon processes authenticated Commands through a deterministic Workflow module and records resulting Events in an append-only SQLite operational journal. Projections serve the CLI and dashboard, external effects use idempotency keys, and GitHub state is reconciled into the journal. The journal supports recovery and audit but remains reconstructible operational state rather than a competing permanent source of project truth.

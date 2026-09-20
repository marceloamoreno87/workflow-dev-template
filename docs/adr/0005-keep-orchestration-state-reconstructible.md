# Keep orchestration state reconstructible

The local daemon persists queues, leases, checkpoints, locks, budgets, and telemetry in SQLite so concurrent Executions can survive process interruption. GitHub, Git, and each Project's Knowledge Bundle remain authoritative; the local database is operational state that must be reconcilable or rebuildable rather than a second source of permanent project truth.

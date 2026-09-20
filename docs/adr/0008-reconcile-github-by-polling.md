# Reconcile GitHub by polling

The local daemon discovers GitHub changes through cursor-based polling at startup and on a configured interval. This avoids exposing a local webhook endpoint and lets the single-user system recover deterministically after being offline; event-driven GitHub Actions can be added later for workflows that must run while the daemon is unavailable.

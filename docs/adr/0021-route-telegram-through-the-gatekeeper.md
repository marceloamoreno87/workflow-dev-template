# Route Telegram through the Gatekeeper

The Telegram bot accepts allowlisted private-chat Commands but has no direct execution path. Read and lifecycle Commands may proceed normally, while approvals, deployment, and rollback require an expiring one-time challenge bound to the Actor, Gate, and expected Aggregate version. Long polling supports the local daemon, and delayed privileged Commands expire rather than executing after an offline period.

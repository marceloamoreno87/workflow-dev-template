# Enforce idempotent optimistic Commands

Every Command carries a stable command ID and the Aggregate version it expects. The Workflow records a Command once, returns its prior result on duplication, and rejects stale intent for reconciliation instead of allowing the last interface to write to win. External effects begin only after their causative Event is durable and use correlation, causation, and idempotency identifiers.

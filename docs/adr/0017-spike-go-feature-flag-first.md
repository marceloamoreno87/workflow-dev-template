# Spike GO Feature Flag first

The first vertical slice evaluates GO Feature Flag as the default OpenFeature provider because it is self-hosted, configuration-first, lightweight, and supports the required Go, Python, and JavaScript stacks. The spike must prove identity targeting, offline defaults, Git rollback, OpenTelemetry exposure tracking, kill-switch latency, backup, browser confidentiality, and Coolify operation; Flipt is evaluated with the same fixture if GO Feature Flag fails those criteria.

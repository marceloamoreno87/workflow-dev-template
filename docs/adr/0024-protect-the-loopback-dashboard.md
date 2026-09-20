# Protect the loopback dashboard

The local dashboard binds only to loopback but still requires an ephemeral authenticated session, strict same-site cookies, CSRF protection, Origin and Host validation, disabled CORS, a restrictive content security policy, state-version-aware confirmations, and inactivity expiry. Loopback location is treated as network placement, not as authentication or authorization.

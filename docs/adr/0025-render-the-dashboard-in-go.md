# Render the dashboard in Go

The v1 daemon renders the loopback dashboard with Go templates, embedded static assets, and lightweight server-sent updates. This preserves single-binary installation and avoids adding a Node.js runtime to the Harness itself; a richer client application is deferred until demonstrated interaction complexity justifies it.

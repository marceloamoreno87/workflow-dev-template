# Quality gates

## Profile matrix

| Gate | Prototype | Standard | Critical |
|---|---:|---:|---:|
| Format/lint | required | required | required |
| Compile/typecheck | required | required | required |
| Unit tests | when present | required | required |
| Integration tests | optional with reason | when affected | required |
| E2E | optional with reason | when affected | critical flows |
| Dependency and secret scan | required | required | required |
| SAST | optional with reason | required | required |
| Race/concurrency | optional with reason | when supported | required |
| Performance | baseline | regression check | explicit budget |
| Independent review | optional | required | required |
| Client acceptance | visible changes | visible changes | visible changes |

“Not applicable” is structured Evidence with a reason; it is never silent.

## Go Stack Adapter baseline

- `gofmt`
- `go vet`
- `staticcheck`
- pinned `golangci-lint`
- `go test ./...`
- `go test -race ./...` for standard and critical profiles
- integration tests selected by build tag
- `govulncheck`
- reproducible build
- module, license, and secret checks
- binary or container smoke test
- benchmark comparison for declared critical paths

Every tool version is pinned by the Harness catalog and updated only after adapter evals pass. Project module files remain authoritative for each build.

## Evidence

Each Gate returns a schema-validated Evidence record containing command identity, tool version, start/end time, exit status, redacted output reference, affected commit, cache status, and deterministic/not-applicable metadata. Passing output is evidence, not merely an agent assertion.

## Harness verification levels

1. Table-driven and property tests for Workflow and Gatekeeper.
2. Contract tests for real seams.
3. Integration tests with real SQLite, Git, and disposable containers.
4. E2E fixture repositories with protocol-faithful GitHub and Coolify fakes.
5. Controlled smoke tests against real external systems.

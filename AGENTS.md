# FocalSpan repository rules

- Build: `go build ./cmd/focalspan`
- Test: `go test ./...`; race: `go test -race ./...`; lint: `go vet ./...`
- Cross-build with `CGO_ENABLED=0` for Windows amd64, Linux amd64, and Darwin arm64.
- Production code stays inside `cmd/focalspan` and `internal/*`; acceptance fixtures stay under `testdata/`.
- CLI output belongs on stdout; errors and `slog` belong on stderr.
- MCP stdout is protocol-only; never mix logs or diagnostics into MCP messages.
- Evidence Packet source fidelity is a public contract.
- Normal MCP context responses must not expose ranking or token-savings debug fields.
- MCP source appears once in structured content and never in text summaries.
- Every Evidence Packet change requires wire-budget and invariant tests.
- Write the failing test before each behavior change and keep tests deterministic.
- No network, external LLM, repository-code execution, build/package restore, or CRG code copying.
- Fixture and end-to-end acceptance tests live under `testdata/` and package integration tests.

# ExecPlans

For work spanning multiple packages, a public contract, or more than one
session, read `PLANS.md` and execute the repository-root `PLAN.md`.
`PLAN.md` is the sole active plan. Keep its Progress, discoveries, decisions,
and outcomes current; archive it only when introducing its successor.

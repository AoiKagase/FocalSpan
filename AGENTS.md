# FocalSpan repository rules

- Build: `go build ./cmd/focalspan`
- Test: `go test ./...`; race: `go test -race ./...`; lint: `go vet ./...`
- Cross-build with `CGO_ENABLED=0` for Windows amd64, Linux amd64, and Darwin arm64.
- Production code stays inside `cmd/focalspan` and `internal/*`; acceptance fixtures stay under `testdata/`.
- CLI output belongs on stdout; errors and `slog` belong on stderr.
- MCP stdout is protocol-only; never mix logs or diagnostics into MCP messages.
- Write the failing test before each behavior change and keep tests deterministic.
- No network, external LLM, repository-code execution, build/package restore, or CRG code copying.
- Fixture and end-to-end acceptance tests live under `testdata/` and package integration tests.

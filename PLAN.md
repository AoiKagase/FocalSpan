# MCP worker restart through a persistent supervisor

## Purpose / Big Picture
Keep the host's MCP stdio session alive while reloading the installed FocalSpan executable. Existing code_restart continues to reopen configuration/index; new code_restart_process replaces only the worker and reports success after initialize, compatibility checks and health checks.

## Progress
- [x] 2026-10-17 UTC: Inspected SDK subprocess transport, MCP schema negotiation, registration and active work. Created isolated managed worktree codex/mcp-worker-restart from 7e30ed1 so another session's root PLAN.md remains untouched.
- [ ] Add subprocess-backed supervisor regressions and observe RED.
- [ ] Implement supervisor, CLI worker mode and registration of the restart API.
- [ ] Verify protocol fidelity, cancellation, compatibility failure, process death and cleanup; review.
- [ ] Complete full tests, race, vet, native and three CGO-disabled builds.

## Surprises & Discoveries
- The root checkout has a different active Git-impact plan. Only this worktree's PLAN.md is owned here.
- A default-capability worker can return v1 Evidence; the supervisor can apply the existing v2 negotiation function on behalf of the actual frontend client without changing source placement or budget logic.
- Windows UCRT GCC at C:/msys64/ucrt64/bin/gcc.exe is needed for amd64 race checks.

## Decision Log
- 2026-10-17 UTC: User approved the persistent launcher architecture. Keep serve as the registered entry point; it supervises by default and internally launches serve --worker with an absolute, fixed executable path and startup root/options.
- 2026-10-17 UTC: Stage a candidate worker before switching. Reject incompatible tool catalog, capabilities or Instructions; preserve the old worker on startup, health, cancellation or drain failure. No arbitrary executable/root input is exposed by the restart API.
- 2026-10-17 UTC: Restart requests serialize; regular tool calls share a read lease and finish before swap. Gate waits are cancellable and bounded to five seconds; candidate startup has a 30-second ceiling. Never replay failed requests automatically.
- 2026-10-17 UTC: No registration outside this checkout, binary deployment, host reconnect, push or merge is part of implementation. One initial host reconnect to the new supervisor is necessary; subsequent compatible worker replacements retain the session.

## Outcomes & Retrospective
Pending implementation and verification.

## Context and Orientation
internal/mcpserver owns the SDK server, context schema negotiation and five existing tools. internal/cli runServe currently hosts this service directly. internal/integration/codex defines the enabled-tools allowlist. Existing raw MCP forwarding preserves structured evidence and summary separation.

## Plan of Work
1. Add an independent subprocess test helper and a frontend in-memory MCP session. Require six supervisor tools, distinct worker PIDs across restart, preserved root/options, v1/v2 output, no source in summaries, and successful later calls on the same host session.
2. Add supervisor.go in mcpserver using the SDK CommandTransport. Capture the initial five-tool catalog/capabilities/instructions, publish them plus code_restart_process, and forward requests under a cancellable read lease. Reuse negotiatedContextOutput for packet tools.
3. Launch/health-check a candidate with startup timeout, compare the captured contract, drain current calls, swap, then close/reap the prior child. Failed launch/compatibility/cancellation leaves the old worker serving. Worker crashes surface errors; explicit restart recovers without duplicating requests. Host shutdown closes its worker.
4. Make CLI serve resolve root once, supervise by default, and use --worker only for the original direct server. Preserve auto-update and legacy no-auto-update options. Include code_restart_process in managed registrations and document lifecycle/limitations.
5. Run focused and complete verification and a read-only review; record real results.

## Validation and Acceptance
Use only test-binary helpers or built FocalSpan processes, not repository fixture execution; no network or restore. Verify fresh-process restart, failed candidate retaining old service, malformed/unknown args, cancelled restart and tool calls, stable host session, concurrent calls/drain, process cleanup, and v1/v2 Evidence invariants/wire budgets. Run go test ./..., go test -race ./..., go vet ./..., native go build ./cmd/focalspan; cross-build windows/amd64, linux/amd64, darwin/arm64 with CGO_ENABLED=0. Target execution is Windows only.

## Idempotence and Recovery
Keep persistent supervisor executable mapped while a candidate is loaded from the captured installation path; replacing a Windows binary uses rename/backup then copy, never overwrite a mapped executable. API input is empty. Repeated successful restarts create one current worker; failed ones never masquerade as success. Incompatible public contracts require host reconnect. Schema migrations requiring exclusive DB access can be rejected while the old worker is live; perform those through a planned outage.

## Interfaces and Dependencies
New internal NewSupervisor(ctx, commandFactory) lifecycle plus Run/Close. New MCP code_restart_process input {} and result {restarted, previous_pid, worker_pid}; six enabled tools only for the supervisor, five direct worker tools. Existing CLI registration command and Evidence Packet formats remain compatible. Reuse go-sdk v1.7.0; no new modules.

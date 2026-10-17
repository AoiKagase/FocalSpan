# Reloading the FocalSpan worker without disconnecting MCP

The registered command remains `focalspan serve`. It now runs a persistent
supervisor which owns the host's MCP session and starts a direct worker with
the internal `serve --worker` option. The repository root is resolved once;
the absolute executable path, root and auto-update options are retained
across restarts.

Two different tools are available:

- `code_restart {}` reopens configuration and the index in the current worker.
- `code_restart_process {}` loads the installed worker executable again.
  It returns `restarted`, `previous_pid` and `worker_pid` only after the
  candidate has initialized, exposed a compatible tool contract, responded
  to ping and passed a read-only index health check.

The process restart API accepts no executable path or repository root.
The supervisor stages a candidate before switching. Startup failures,
incompatible contracts, a changed repository root and cancellation leave the original worker in place.
Current tool calls finish before the switch. Admission waits have a five-second
cap and startup a 30-second cap; caller cancellation takes priority. Failed
worker calls are not replayed automatically. If a worker crashes, explicitly
call `code_restart_process` to recover.

Evidence structured content, text summaries, error results and negotiation of
v1/v2 remain the worker's responsibility. The supervisor forwards the host's
capabilities, including initialize-time capabilities of older clients.
Diagnostics go to stderr; stdout remains MCP JSON-RPC only.

## First activation and later binary updates

The first upgrade to this architecture requires one host reconnect to launch
the supervisor. If a managed registration has an enabled-tools allowlist,
refresh it with the existing `focalspan mcp install` command (add `--project`
for project scope) so it includes `code_restart_process`.

For a later Windows update, rename the mapped executable to a backup and
place the new verified binary at the original installation path. Then call
`code_restart_process {}` on the existing MCP connection. Do not remove the
backup while a process is still using it.

Tool catalog/schema, capabilities or initialization-instruction changes are
rejected for hot replacement and require a host reconnect. Updating the
supervisor itself also requires reconnecting the host. A database migration
which needs exclusive lifecycle access can be blocked by the live worker;
perform that migration during a planned stop instead of bypassing its lock.

# Parallel FocalSpan sessions

CLI and MCP processes may share a local index directory. Each open database
retains a shared lifecycle lock. Index updates acquire a separate exclusive
writer lock before scanning and keep it through linking and completion metadata.
A waiting updater scans the current files after it acquires the lock.

Contended operations wait at most five seconds; a caller's earlier deadline or
cancellation takes priority. Update failures propagate to CLI/MCP callers instead
of silently returning stale results. Normal MCP output remains protocol-only.

Database creation, interrupted-upgrade recovery and schema migration require an
exclusive lifecycle lock. An upgrade cannot replace a database another session
has open. If an error asks you to close sessions and restart, close the other
FocalSpan processes and retry the update. A completed migration reopens and
rechecks the database with shared access.

The index directory contains persistent .index-lifecycle.lock and
.index-writer.lock files. Do not remove them to resolve contention: another
process may be waiting on the same file. OS locks are released automatically
when the owning process exits, including abnormal termination.

Supported targets are Windows, Linux and macOS on local filesystems. All
concurrent processes must use a version with this locking protocol; older
binaries do not honor the locks. Network shares are not covered. Locks
coordinate FocalSpan operations, not editor writes or a whole-repository
filesystem snapshot.

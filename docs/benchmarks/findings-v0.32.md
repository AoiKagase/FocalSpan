# v0.32 negotiated compact context findings

## Verdict

Accepted. Commit `cd3556ff3944eea2896f86a6294db937cd23dcb6` adds
`focalspan.context.v2` as an explicitly negotiated MCP representation while
keeping `focalspan.context.v1` as the default.

The single candidate history run passed the adoption gates and materially
reduced model-visible wire cost without changing selected Evidence.

## Implementation

- The server advertises `io.focalspan/context-encoding` with v1 as the default.
- A client must explicitly advertise an `accept` list containing
  `focalspan.context.v2`; missing or malformed capability settings return v1.
- v2 uses path, evidence, relation, segment, and next-action tables. Stable
  handles and all source, metadata, line ranges, fidelity, and provenance are
  retained.
- A strict decoder reconstructs the canonical v1 `evidence.Packet` and applies
  the existing validator.
- Each v2 response re-settles `Budget.Used`. If v2 is not strictly smaller than
  the corresponding v1 response, the server returns v1 even after opt-in.
- The three Evidence tools advertise a v1/v2 `oneOf` output schema. The CLI and
  normal benchmark profiles remain on v1.

## RED and GREEN evidence

The first codec/MCP test run failed because `EncodeContextV2`,
`DecodeContextV2`, `SchemaContextV2`, and the negotiation extension did not
exist. The benchmark RED then failed because `ContextEncoding`, explicit v2
profiles, and `measurePacketForProfile` did not exist.

After implementation:

- `go test ./internal/evidence ./internal/mcpserver ./internal/benchmark -count=1`:
  229 passed.
- `go test ./... -count=1`: 725 passed in 46 packages.
- `go vet ./...`: no issues.
- `go build ./cmd/focalspan`: passed.
- `CGO_ENABLED=0` cross-builds passed for Windows amd64, Linux amd64, and
  Darwin arm64.
- `git diff --check` passed.
- race testing remained unavailable under the existing local MinGW limitation.

## Single candidate measurement

Command profile set:

```text
full-evidence-focused-v2,fts-evidence-focused-v2,no-relations-evidence-focused-v2
```

The run used the eight-case `focalspan-history-v0.5` suite, five matching
profile/budget configurations, repeat 3, and produced 40 quality rows.

| Metric | v1 accepted baseline | v0.32 v2 | Result |
|---|---:|---:|---:|
| useful Evidence | 5 | 5 | unchanged |
| cumulative estimated wire | 11,693 | 8,652 | -3,041 (-26.0%) |
| useful Evidence / 1,000 tokens | 0.4276 | 0.5779 | +0.1503 |
| UTF-8 model-visible bytes | 32,494 | 23,752 | -8,742 (-26.9%) |
| packet JSON bytes | 31,090 | 22,348 | -8,742 |
| summary bytes | 1,404 | 1,404 | unchanged |
| Evidence content value bytes | 3,459 | 3,459 | unchanged |
| guidance value bytes | 4,010 | 4,010 | unchanged |
| envelope/metadata bytes | 23,621 | 14,879 | -8,742 |
| selected fidelity | 10 / 15 / 30 / 0 | 10 / 15 / 30 / 0 | unchanged |

All rows had budget compliance, deterministic output, and relation validity 1.
Forbidden violations and known-handle resend were 0. Canonical packet selection
is performed before encoding, and round-trip equivalence tests cover every
fidelity, relation, guidance, and known-handle field, so comparison regression
is 0 by the enforced conversion boundary.

## Decision

Adopt v0.32. It satisfies the long-term v2 requirement without replacing v1,
and it gives the largest measured token reduction in the v0.20+ sequence.

The remaining cache, retriever-latency, and SQL re-optimization item is closed
outside the token roadmap. It has no model-visible token mechanism, and prior
bounded retriever/SQL candidates supplied no new byte-identical all-profile
latency opportunity that would justify another milestone.

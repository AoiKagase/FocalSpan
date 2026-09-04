# v0.31 `focalspan.context.v2` design gate findings

## Verdict

The design gate passes. The repository can add an explicitly negotiated compact
encoding without changing the default `focalspan.context.v1` response.

This milestone makes no product change and runs no candidate benchmark.

## Evidence

- `github.com/modelcontextprotocol/go-sdk/mcp` v1.7.0 exposes
  `ServerCapabilities.AddExtension` and `ClientCapabilities.AddExtension`.
- `mcp.CallToolRequest.ClientCapabilities()` exposes the calling client's
  session or per-request capabilities to each tool handler.
- FocalSpan's `code_context`, `code_expand`, and `code_impact` handlers already
  receive `*mcp.CallToolRequest`; they currently discard it.
- `mcp.AddTool` accepts an explicit draft-2020-12 output schema and validates
  the marshalled output against it. An `Out=any` handler plus an explicit
  `oneOf` output schema can therefore validate both v1 and v2 objects.
- `evidence.Validate` already provides the canonical v1 invariant boundary for
  identity, fidelity, line ranges, relations, ordering, and budget fields.

## Negotiation contract

The extension identifier is `io.focalspan/context-encoding`.

The server advertises:

```json
{
  "extensions": {
    "io.focalspan/context-encoding": {
      "schemas": ["focalspan.context.v1", "focalspan.context.v2"],
      "default": "focalspan.context.v1"
    }
  }
}
```

A client opts in by advertising:

```json
{
  "extensions": {
    "io.focalspan/context-encoding": {
      "accept": ["focalspan.context.v2"]
    }
  }
}
```

Missing, malformed, or differently named settings select v1. No public tool
input is added, and the selection is made independently for each request from
the request's effective client capabilities.

## Compact representation

The v2 object keeps `schema` readable and uses fixed-position tables elsewhere:

- `r`, `i`, `m`: revision, intent, and mode.
- `b`: `[limit, used, truncated, omitted]`.
- `p`: a de-duplicated path table.
- `e`: evidence rows
  `[handle, role, path-index, start, end, fidelity, payload, attributes]`.
- `x`: relation rows `[from-index, to-index, kind, certainty]` using zero-based
  evidence indexes rather than repeated local IDs.
- `l`: limitations.
- `n`: next-action rows `[handle, relation, reason]`.
- `k`: skipped-known count.

`payload` is the exact source, signature, or outline string for those fidelity
modes. For excerpt fidelity it is the ordered segment table
`[kind, start, end, text]`; omitted segments have no text. Optional evidence
metadata is carried by short keys in `attributes`: `a` language, `k` kind,
`s` symbol, and `w` why. Local IDs remain derivable as `e1`, `e2`, ... and are
restored by the decoder.

## Equivalence and budget rules

The encoder accepts only an `evidence.Packet` that passes `evidence.Validate`.
The v2 decoder reconstructs a v1 `evidence.Packet` and validates it through the
same function. Tests compare every canonical field except `Budget.Used`, which
is intentionally encoding-specific. `Budget.Limit`, `Truncated`, `Omitted`,
all evidence content and metadata, relation endpoints and provenance,
limitations, next actions, ordering, and `SkippedKnown` must match exactly.

For a negotiated request, v2 is emitted only when its settled model-visible
wire cost is strictly below the already compiled v1 packet. Otherwise the
handler falls back to v1. This preserves the hard budget and makes the compact
encoding monotonic per response. The text summary remains the existing fixed
numeric form and never contains source.

## Next milestone acceptance fixture

The implementation milestone must begin with failing tests for:

1. absent, malformed, and non-v2 capabilities returning byte-identical v1;
2. v2 discovery in server initialize capabilities;
3. all three Evidence tools negotiating v2;
4. v1-to-v2-to-v1 canonical equivalence across verbatim, excerpt, signature,
   synthetic, relation, limitation, next-action, and known-handle fields;
5. invalid tables being rejected without panic;
6. deterministic encoding and per-response v1 fallback when v2 is not smaller;
7. hard-budget compliance and source appearing exactly once in structured
   content and never in the text summary.

Only after these tests pass may the history candidate benchmark run once.
Adoption additionally requires comparison regression 0, relation validity 1,
forbidden violation 0, known resend 0, cumulative wire not above 11,693, and
strictly fewer UTF-8 model-visible bytes than 32,494 on at least one negotiated
v2 profile.

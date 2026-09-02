# Known-handle delta guidance v0.15 findings

## Scope and execution

v0.15 adds a private final-guidance pass for Evidence expansions that contain
`known_handles`. It removes only envelope text that describes an already-known
anchor when no other relation edge or relation next-action is present. A
known-handle self action is removed under the same condition. In a known-only
empty packet, `no_relevant_source_found` is also omitted because the absence is
intentional; `budget_limited`, `skipped_known`, source omission, unresolved
relations, and omitted-relation actions remain. Initial queries and
known-handles-empty control expansions are unchanged.

The public MCP/CLI surfaces, `focalspan.context.v1`, JSON key names, source
fidelity, relation endpoints, budgets, and deterministic ordering are
unchanged. The product change and regression tests are in commit `b07203a`.

Static verification completed before the candidate gate:

- `go test ./... -count=1`: passed;
- `go vet ./...`: passed;
- `git diff --check`: passed;
- native build: passed;
- CGO-free Windows amd64, Linux amd64, and Darwin arm64 builds: passed; and
- `go test -race ./...`: **UNVERIFIED** because the local MinGW compiler
  reports `cc1.exe: sorry, unimplemented: 64-bit mode not compiled in`.

The fixture contract evaluation passed all eight cases with coverage, role,
fidelity, relation, budget, and deterministic scores equal to 1. Its
`code_context -> code_expand` delta ratio improved from
`0.5578351609480015` to `0.5550053059780686`; known resend stayed at 0.

## Candidate gate status

The historical `focalspan-history-v0.5` suite was run exactly once with the
`default` profile, repeat 1, attribution enabled, and diagnosis enabled. It
completed with 48 quality rows, 40 attribution results, and 40 diagnosis
results. Comparing the candidate report with the v0.14 baseline returned
`compatible=true` and `regressions=0`.

| Metric | v0.14 baseline | v0.15 candidate | Gate |
|---|---:|---:|---|
| `packing_dropped` labels (focused/2048) | 7 | 7 | no increase |
| packed labels (all candidate rows) | 5 | 5 | retain all |
| cumulative estimated wire tokens | 12,304 | 12,304 | `<= 12,304` |
| useful evidence | 5 | 5 | `>= 5` |
| useful evidence / 1,000 tokens | 0.4064 | 0.4064 | `>= 0.4064` |
| median metadata overhead | 0.9222 | 0.9222 | no regression |

The candidate therefore **passed the strict gate**. The v0.15 quality report
is recorded as `docs/benchmarks/results-v0.15.json` and
`docs/benchmarks/results-v0.15.md`.

## Artifact hashes

| Artifact | SHA-256 |
|---|---|
| v0.15 quality JSON | `04ce61fcd08cf88feea729f9b8aec2b9237a33b8b4c49dee4b5e2bbf9e4d3b9c` |
| v0.15 quality Markdown | `47af731c3512e417a968c8f92b616fb11a820fe09e7c3424f59ebe0c767b3088` |
| candidate attribution JSON | `425dd767bcdef04e791c31065616a94468be49b4d54d576f82fb9232f7826da2` |
| candidate attribution Markdown | `7b82cf947316c5c6f1aeb32dbe47f773d3d228433af260701ed2cbe948d2567c` |
| candidate diagnosis JSON | `1dea51b6ea2a6593da2771d3aeee7c3393b5484efeb5d1195b58e3923b794287` |
| candidate diagnosis Markdown | `5c7f22e42ea5245d9701ec2d6de43ec73c45b9ddda432967a6ec3a7f14d131bc` |

Privacy scanning found no absolute local paths, usernames, source/content
sentinels, secrets, `NaN`, or `Infinity` in the generated reports. Temporary
benchmark reports, snapshots, binaries, and writable Go caches are removed
after this finding is committed. User-owned `AGENTS.md`, `.focalspan.json`, and
`TASKS.md` changes are not staged.

## Retrospective

The measured historical suite did not exercise a valid expansion anchor, so its
quality and cumulative-wire baseline remained unchanged. The dedicated
evidence fixture did exercise the stateless self expansion and showed the
intended reduction without changing Evidence selection or fidelity. The
remaining race verification requires a compatible 64-bit MinGW toolchain.

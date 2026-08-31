# FocalSpan Candidate Attribution and Coverage Findings v0.6

## Frozen starting state

Task 1 was measured on 2026-08-31 UTC from commit
`6582dc56b3f1a972f60b8671c8c5718c80588d22`. The branch was `master`, two
documentation commits ahead of fetched `origin/master`; no tracked or staged
change existed before this record. The pre-existing untracked
`.focalspan.json` was not read, modified, staged, or committed.

- Environment: Go 1.27.0, Windows amd64, `CGO_ENABLED=1`.
- Frozen v0.5 quality object:
  `f914facbfbf55c450fd26769bdc7bd6a992112dc`.
- Byte-identical v0.5 plan archive object:
  `281211f6754bd3b1e45a7b321d8aaab1a1a27094`.
- `go test ./... -count=1`: exit 0, 657 tests in 46 packages.
- `go vet ./...`: exit 0, no issues reported.
- `git diff --check`: exit 0.
- Public suite validation: 8 cases, 0 invalid.
- Development smoke: `php-extractor-integration` and
  `cpp-extractor-registry`, default profiles, repeat 1, 2 cases and 12 quality
  results.
- v0.5 comparison: exit 0, compatible true, 0 regressions.

The temporary candidate JSON, Markdown, indexes, and workspace were removed
after comparison. No eight-case quality run was performed in Task 1.

## Attribution measurement

This section will receive source-free stage counts from the single eight-case
repeat-1 diagnostic after the attribution schema and adapter pass their privacy
and determinism gates. It is not a placeholder for a chosen optimization: the
selection rule is already frozen in root `PLAN.md`, and no production change is
allowed before the measured Decision Log commit.

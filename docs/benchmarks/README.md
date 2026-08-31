# FocalSpan Real-Repository Benchmarks

## What the benchmark measures

The benchmark measures whether the current FocalSpan implementation retrieves human-labeled paths and symbols from historical repository snapshots, assigns useful Evidence roles, respects wire budgets, and avoids retransmitting known evidence.

## What it does not prove

A target commit's diff is diagnostic evidence, not automatic ground truth. Results from one repository do not prove general retrieval quality, and timing results are not quality thresholds.

## Privacy model

Public suites use logical repository IDs, commit IDs, and repository-relative paths. Reports contain no source text, absolute local paths, usernames, environment values, or secrets. The benchmark never executes repository code or accesses the network.

## Historical task labeling

Authors describe a realistic question against a base revision and manually label required, optional, and forbidden evidence that existed at that base. Added target files cannot be required base evidence.

## Running the public self-history suite

Validate the eight human-reviewed historical cases before running them:

    go run ./cmd/focalspan-bench validate --suite testdata/benchmark/focalspan-history.json

Labels and their rationale are recorded in `testdata/benchmark/focalspan-history-labels.md`.

The full eight-case repeat-3 run is the release measurement. It is intentionally
manual because it materializes and indexes every historical snapshot:

    go run ./cmd/focalspan-bench run --suite testdata/benchmark/focalspan-history.json --profile default --repeat 3 --json-out .focalspan-bench/candidate.json --markdown-out .focalspan-bench/candidate.md --force
    go run ./cmd/focalspan-bench compare --baseline docs/benchmarks/results-v0.5.json --candidate .focalspan-bench/candidate.json

For a quick two-case regression smoke, filter both the run and comparison to
the same IDs:

    go run ./cmd/focalspan-bench run --suite testdata/benchmark/focalspan-history.json --case php-extractor-integration --case cpp-extractor-registry --profile default --repeat 1 --json-out .focalspan-bench/smoke.json --markdown-out .focalspan-bench/smoke.md --force
    go run ./cmd/focalspan-bench compare --baseline docs/benchmarks/results-v0.5.json --candidate .focalspan-bench/smoke.json --case php-extractor-integration --case cpp-extractor-registry

Within a run, each historical case is indexed once and that index is shared by
all profiles and budgets. Retrieval mode remains query-local. There is no
persistent cross-run cache, so separate invocations always materialize and
measure fresh snapshots.

The development command writes temporary snapshots and generated reports only;
`.focalspan-bench/` is ignored and should be removed after local verification.

## Running private local suites

Private repository paths belong in the ignored `.focalspan-bench.json` registry or explicit `--repo ID=PATH` arguments. Suite files retain only logical IDs.

## Interpreting quality and timing reports

Quality results are deterministic for identical inputs. Wall-clock snapshot, indexing, and query timings are volatile and are serialized separately from deterministic comparison data.

## Choosing the next optimization milestone

Choose one production area only after failure attribution shows whether evidence was never retrieved, ranked but not packed, expensive in metadata, or concentrated in one language or artifact type.

## Continuous verification

GitHub Actions configures Linux unit tests, vet, Linux race tests, CGO-free
Windows/Linux/Darwin builds, and a two-case repeat-1 public benchmark comparison
for pushes and pull requests. The full eight-case repeat-3 comparison is
manual-dispatch only. The workflow
has read-only repository permission, uses no private registry, uploads no
snapshot or binary, and writes benchmark outputs only below the runner temporary
directory. A configured workflow is not proof of a pass; remote Linux race and
benchmark status remain unverified until an actual GitHub Actions run reports
success.

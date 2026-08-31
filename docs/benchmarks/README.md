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

## Running private local suites

Private repository paths belong in the ignored `.focalspan-bench.json` registry or explicit `--repo ID=PATH` arguments. Suite files retain only logical IDs.

## Interpreting quality and timing reports

Quality results are deterministic for identical inputs. Wall-clock snapshot, indexing, and query timings are volatile and are serialized separately from deterministic comparison data.

## Choosing the next optimization milestone

Choose one production area only after failure attribution shows whether evidence was never retrieved, ranked but not packed, expensive in metadata, or concentrated in one language or artifact type.

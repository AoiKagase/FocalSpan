# FocalSpan MVP Evaluation

## Dataset

The checked-in fixture is `testdata/repos/authsample`. It contains an
authentication service with `ValidateToken`, an expired-token branch, a
middleware caller, an expired-token test, configuration, documentation, and a
larger unrelated report file. Cases are in `testdata/eval/cases.jsonl`.

The fixture is intentionally ordinary Go code. Production ranking contains no
fixture-specific names or query branches.

## Metrics

For every case, `focalspan eval` runs the same query twice and records:

- hit@1, hit@3, hit@5 for expected symbols;
- expected symbol recall and expected path recall;
- forbidden-path violations;
- final token-budget compliance;
- median estimated tokens;
- reduction ratio, where the baseline is the estimated token count of the full
  candidate files returned by retrieval;
- deterministic output equality between repeated runs.

The MVP acceptance thresholds are:

| Metric | Threshold |
| --- | ---: |
| Budget compliance | 100% |
| Fixture expected symbol hit@5 | 100% |
| Forbidden path violations | 0 |
| Deterministic result | 100% |
| Fixture median reduction ratio | <= 0.25 |
| Source result path and line range | present for every item |
| Unrelated full-file return | none |

## Reproduction

From the repository root:

```text
focalspan init
focalspan index --root testdata/repos/authsample
focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
```

The evaluation output is the evidence for the thresholds. A failed or
unexecuted command remains explicitly unverified; it is not converted into a
pass by documentation.

## Interpretation

The fixture measures retrieval, deduplication, and packing, not semantic call
resolution. Go relations are syntax-only and unresolved calls are labeled with
their lexical target and confidence. Future semantic providers may improve
recall without changing the packer or output contract.

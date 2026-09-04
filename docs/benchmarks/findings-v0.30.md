# TokenEstimator oracle gate v0.30 findings

## Read-only investigation

Repository source, tests, dependencies, benchmark documentation, and active
constraints were searched for an approved real-token oracle. No tokenizer
implementation, tokenizer fixture, oracle output, or approved local dependency
exists. Existing design and plan documents explicitly place model-specific or
external tokenizers outside the current offline contract.

The v0.21 estimator-independent UTF-8 byte metrics already prevent a change to
the fixed 1.12 margin from being mistaken for payload reduction, but bytes are
not a tokenizer oracle and cannot prove zero underestimation.

## Decision

The prerequisite is absent. No RED/GREEN product work, coefficient change,
dependency addition, network access, or candidate benchmark was performed.
`internal/budget` and the accepted baseline remain byte-identical: history wire
11,693, model-visible bytes 32,494, useful Evidence 5, efficiency 0.4276.

Calibration may be reopened only when the user approves a concrete tokenizer
oracle and its model/version, reproducible corpus, error tolerance, and
zero-underestimation gate are specified.

# FocalSpan ExecPlan Policy

`PLAN.md` is the repository's only active ExecPlan. It remains at the repository root while work is active and after completion, until the next plan transition. Completed plans are immutable snapshots under `docs/superpowers/plans/completed/`; plans whose goal or architecture was replaced before completion belong under `docs/superpowers/plans/superseded/`.

## Required sections

Every ExecPlan must contain:

- Purpose / Big Picture
- Progress
- Surprises & Discoveries
- Decision Log
- Outcomes & Retrospective
- Context and Orientation
- A task-oriented Plan of Work with concrete steps
- Validation and Acceptance
- Idempotence and Recovery
- Interfaces and Dependencies

## Lifecycle

1. Keep at most one active root `PLAN.md`.
2. Before replacing a completed plan, archive its final checked state, decisions, discoveries, validation evidence, and retrospective byte-for-byte.
3. Archive a superseded plan with its replacement reason and successor plan ID.
4. Make a plan transition one reviewable documentation commit; do not mix product code into it.
5. Update the active plan throughout execution. Progress timestamps use UTC. Record actual command results, not predicted counts.
6. Do not weaken acceptance criteria silently. Record every change with date, evidence, and consequence in the Decision Log.
7. Do not rewrite completed archives. Add a clearly marked correction in a later commit if historical evidence needs amendment.
8. Keep future ideas in `docs/design.md` or a later specification rather than growing the active plan.

## Authoring rules

One plan covers one measurable milestone and must be executable from the current tree without conversation history. It names repository-relative files, interfaces, commands, observable results, exclusions, recovery behavior, and completion conditions. Checkboxes track execution; prose explains intent and design. A plan is not complete if it leaves only scaffolding or unverified behavior.

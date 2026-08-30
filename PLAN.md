# FocalSpan LLM Evidence Contract v0.4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace FocalSpan's MCP-facing flat ranked bundle with a versioned, role-aware, source-faithful Evidence Packet that gives coding LLMs the smallest useful set of code evidence within the actual model-visible token budget.

**Architecture:** Preserve the existing retrieval, ranking, legacy `model.ContextBundle`, CLI defaults, extractors, and SQLite schema. Add an `internal/evidence` presentation-and-packing layer that converts ranked candidates plus query intent and relation provenance into a compact transport contract with explicit roles, fidelity, source segments, local evidence relations, limitations, follow-up actions, and stateless delta suppression. MCP tools use the new contract; legacy CLI/evaluation remain available for regression and A/B comparison.

**Tech Stack:** Go 1.26+, `github.com/modelcontextprotocol/go-sdk/mcp` v1.7.0, standard `encoding/json`, existing SQLite/FTS5 store, existing deterministic token estimator, table-driven tests, process-level MCP JSON-RPC tests.

**Spec:** This root `PLAN.md` is the executable specification. Record the validated design decisions in `docs/design.md` before changing the public MCP result contract.

## Global Constraints

- Treat the current checkout as the only source of truth. This plan was authored against public `master` at commit `ec6f86e`, but newer local or merged work takes precedence.
- Inspect `AGENTS.md`, `PLAN.md`, `README.md`, `docs/design.md`, `docs/evaluation.md`, `docs/implementation-plan.md`, `git status`, and `git diff` before editing.
- Never run `git reset`, `git restore`, `git checkout --`, `git clean`, or `git stash` against user work.
- Preserve all current language extractors and in-progress polyglot work. This milestone does not redesign language parsing.
- Preserve existing tool names: `code_context`, `code_expand`, `code_impact`, `code_restart`, and `code_status`.
- Preserve the current SQLite schema. No database migration is part of this milestone.
- Preserve current retrieval ranking and evaluation baselines unless relation provenance requires a narrowly scoped internal change.
- Keep the legacy `model.ContextBundle` and existing default CLI output available during v0.4.
- MCP `code_context`, `code_expand`, and `code_impact` switch to the versioned Evidence Packet. `code_status` and `code_restart` keep their existing output contracts.
- The canonical source text must appear once in MCP structured output and must not be duplicated in text content.
- Do not expose rank scores, BM25 values, RRF contributions, reason weights, token-savings baselines, or debug traces in the normal Evidence Packet.
- Preserve `focalspan explain` as the place for retrieval and ranking diagnostics.
- Build with `CGO_ENABLED=0` for Windows amd64, Linux amd64, and macOS arm64.
- Do not add network access, external LLM calls, embeddings, a model-specific tokenizer, HTTP MCP transport, session storage, or compiler/runtime execution.
- All output ordering, local evidence IDs, omission decisions, and follow-up actions must be deterministic.
- All source text labeled `verbatim` or carried in a source segment must exactly match bytes from the indexed source span.
- The model-visible payload—the compact serialized Evidence Packet plus its canonical short MCP text summary—must fit the requested token budget. JSON-RPC framing and CLI indentation are transport overhead outside this contract.
- Use TDD. Every behavior starts with a failing test, then minimal implementation, then local and full verification.
- Do not leave incomplete production branches, placeholder implementations, ignored test failures, or panic-based unimplemented paths.

---

## Product Contract

### Why this milestone exists

The current internal bundle is optimized for implementation and debugging. It contains flat `items`, floating-point scores, weighted reasons, per-item token estimates, savings data, and source text. Coding LLMs instead need a small packet that answers four questions without reinterpreting ranking internals:

1. What is the primary target?
2. What role does each additional span play?
3. Which relations are exact, scoped, or lexical?
4. What useful evidence was omitted, and how can it be expanded?

The output must present evidence rather than a natural-language conclusion. FocalSpan selects and organizes source; the consuming LLM draws the conclusion.

### Public schema identifier

Use this exact schema identifier:

```text
focalspan.context.v1
```

### Public modes

```go
type Mode string

const (
    ModeOutline Mode = "outline"
    ModeFocused Mode = "focused"
    ModeSource  Mode = "source"
)
```

Semantics:

- `outline`: locations, symbols, signatures, roles, relations, and concise synthetic outlines; no full source bodies.
- `focused`: the primary target may be verbatim when compact; large or supporting items use query-relevant source segments or signatures. This is the MCP default.
- `source`: include complete selected source spans when they fit; degrade large spans to explicit excerpts, never to an opaque tail-truncated string.

### Public roles

Use these exact serialized values:

```go
type Role string

const (
    RoleTarget         Role = "target"
    RoleDefinition     Role = "definition"
    RoleDeclaration    Role = "declaration"
    RoleImplementation Role = "implementation"
    RoleCaller         Role = "caller"
    RoleCallee         Role = "callee"
    RoleTest           Role = "test"
    RoleType           Role = "type"
    RoleImport         Role = "import"
    RoleExport         Role = "export"
    RoleReference      Role = "reference"
    RoleConfig         Role = "config"
    RoleTemplate       Role = "template"
    RoleDocumentation  Role = "documentation"
    RoleChange         Role = "change"
    RoleDependent      Role = "dependent"
    RoleContext        Role = "context"
)
```

### Fidelity values

```go
type Fidelity string

const (
    FidelityVerbatim  Fidelity = "verbatim"
    FidelityExcerpt   Fidelity = "excerpt"
    FidelitySignature Fidelity = "signature"
    FidelitySynthetic Fidelity = "synthetic"
)
```

Rules:

- `verbatim`: `source` is populated; `segments` and `outline` are empty.
- `excerpt`: `segments` is populated; `source` and `outline` are empty.
- `signature`: `signature` is populated; `source`, `segments`, and `outline` are empty.
- `synthetic`: `outline` is populated; `source` and `segments` are empty.
- An item must use exactly one content representation.

### Certainty values

```go
type Certainty string

const (
    CertaintyExact   Certainty = "exact"
    CertaintyScoped  Certainty = "scoped"
    CertaintyLexical Certainty = "lexical"
)
```

### Required public types

Implement these public transport types in `internal/evidence/model.go`. Additional private fields are allowed, but the JSON contract below must remain compact.

```go
package evidence

type Location struct {
    Path  string `json:"path"`
    Lines [2]int `json:"lines"`
}

type Segment struct {
    Kind  string `json:"kind"` // "source" or "omitted"
    Lines [2]int `json:"lines"`
    Text  string `json:"text,omitempty"`
}

type Item struct {
    ID        string     `json:"id"`
    Handle    string     `json:"handle"`
    Role      Role       `json:"role"`
    Location  Location   `json:"location"`
    Language  string     `json:"language,omitempty"`
    Kind      string     `json:"kind,omitempty"`
    Symbol    string     `json:"symbol,omitempty"`
    Signature string     `json:"signature,omitempty"`
    Fidelity  Fidelity   `json:"fidelity"`
    Why       []string   `json:"why,omitempty"`
    Source    string     `json:"source,omitempty"`
    Segments  []Segment  `json:"segments,omitempty"`
    Outline   string     `json:"outline,omitempty"`
}

type Edge struct {
    From      string    `json:"from"`
    To        string    `json:"to"`
    Kind      string    `json:"kind"`
    Certainty Certainty `json:"certainty"`
}

type Budget struct {
    Limit     int  `json:"limit"`
    Used      int  `json:"used"`
    Truncated bool `json:"truncated,omitempty"`
    Omitted   int  `json:"omitted,omitempty"`
}

type NextAction struct {
    Handle   string `json:"handle"`
    Relation string `json:"relation"`
    Reason   string `json:"reason"`
}

type Packet struct {
    Schema       string       `json:"schema"`
    Revision     string       `json:"revision,omitempty"`
    Intent       string       `json:"intent,omitempty"`
    Mode         Mode         `json:"mode"`
    Budget       Budget       `json:"budget"`
    Evidence     []Item       `json:"evidence"`
    Relations    []Edge       `json:"relations,omitempty"`
    Limitations  []string     `json:"limitations,omitempty"`
    Next         []NextAction `json:"next,omitempty"`
    SkippedKnown int          `json:"skipped_known,omitempty"`
}
```

### Packet invariants

- `schema` is always `focalspan.context.v1`.
- `evidence` is always present as a JSON array, including when empty.
- Final local IDs are sequential `e1`, `e2`, ... in presentation order.
- Every `location.path` is repository-relative and slash-normalized.
- Every line pair is one-based and satisfies `1 <= start <= end`.
- Every edge references IDs present in the same packet.
- No self-edge is emitted unless the relation itself is explicitly `self`; normal packets omit `self` edges.
- Duplicate edges are removed by `(from, to, kind, certainty)`.
- `why` contains stable short codes, not prose and not numeric values; maximum four codes per item.
- `limitations` contains stable codes; maximum eight entries.
- `next` contains at most four deterministic actions.
- `budget.used <= budget.limit` after measuring compact packet JSON plus the canonical short summary returned alongside structured content.
- `budget.omitted` counts candidates excluded by budget, duplicate suppression, or known-handle suppression only when they otherwise qualified for presentation.
- The original query is not repeated in the packet because it already exists in the MCP tool input.
- Normal packets never serialize `score`, `retrieval_score`, `weight`, `detail`, `token_savings`, `baseline_tokens`, `saved_tokens`, or `savings_ratio`.

---

## File Map

Create or modify files according to this responsibility map. Follow current repository conventions when filenames have moved, but preserve these boundaries.

```text
internal/evidence/
    model.go              public Evidence Packet types and enum constants
    validate.go           contract invariant validation
    roles.go              role classification and intent presentation profiles
    fidelity.go           fidelity selection and candidate content variants
    segments.go           query-aware, source-faithful excerpt construction
    compiler.go           candidate selection and packet assembly
    wire.go               model-visible JSON/summary token accounting and final hard-cap loop
    next.go               limitations and follow-up action generation

internal/evidence/*_test.go

internal/model/model.go    add relation provenance used only internally
internal/store/store.go    return relation hits with provenance
internal/store/store_test.go
internal/search/retrieval.go
internal/search/retrieval_test.go
internal/app/service.go
internal/app/service_test.go
internal/render/evidence.go
internal/render/evidence_test.go
internal/cli/run.go
internal/cli/run_test.go
internal/mcpserver/server.go
internal/mcpserver/mcp_test.go
internal/eval/evidence.go
internal/eval/evidence_test.go

testdata/repos/evidencesample/
testdata/eval/evidence-cases.jsonl

docs/design.md
docs/evaluation.md
docs/implementation-plan.md
README.md
PLAN.md
```

Do not move current extractors, split the store schema, or replace the existing budget package in this milestone.

---

### Task 0: Protect the Current Baseline and Record the Contract Decision

**Files:**
- Modify: `docs/design.md`
- Modify: `docs/implementation-plan.md`
- Modify: `PLAN.md`
- Create: `testdata/eval/baseline-v0.4.json` only if the repository does not already have an equivalent checked-in baseline artifact

**Interfaces:**
- Consumes: current `model.ContextBundle`, existing CLI/MCP behavior, current fixture evaluations
- Produces: a documented baseline and a decision record that later tasks must preserve

- [x] **Step 1: Record the exact starting state without modifying it**

Run:

```bash
git status --short
git diff --stat
git rev-parse HEAD
go version
go env GOOS GOARCH CGO_ENABLED
git log -8 --oneline
```

Copy the command results into a temporary work note outside tracked source or into the final implementation report. Do not mark a dirty checkout as clean.

- [x] **Step 2: Run baseline unit and vet checks**

Run:

```bash
go test ./...
go vet ./...
```

Expected: both commands pass. If either already fails, preserve the exact failure before making changes and distinguish it from v0.4 regressions.

- [x] **Step 3: Run every checked-in retrieval evaluation**

Discover files first:

```bash
find testdata/eval -maxdepth 1 -type f -name '*cases.jsonl' -print
```

On Windows without a POSIX shell, list the same directory with PowerShell and run each file through the existing `focalspan eval` command using its matching fixture root. At minimum run all currently checked-in Go, PHP, C/C++, C#, JS/TS, and Smarty/template cases.

Expected: save the exact hit, recall, forbidden-path, budget, reduction, relation, intent, and determinism metrics. Do not round stored baselines more aggressively than the current evaluator output.

- [x] **Step 4: Add the v0.4 decision to `docs/design.md`**

Add a decision section with these exact decisions:

```text
- Keep model.ContextBundle as an internal and legacy CLI representation.
- Introduce internal/evidence as the LLM-facing presentation boundary.
- Switch code_context, code_expand, and code_impact to focalspan.context.v1.
- Keep code_status and code_restart unchanged.
- Keep normal ranking diagnostics out of the Evidence Packet.
- Budget the final serialized packet rather than source text alone.
- Make MCP default mode focused.
- Use stateless known_handles instead of server-side conversation state.
- Preserve the SQLite schema and all extractors in this milestone.
```

- [x] **Step 5: Update `docs/implementation-plan.md` with milestone boundaries**

Document that v0.4 covers transport modeling, role assignment, excerpt fidelity, wire budgeting, delta suppression, MCP integration, and evidence evaluation. Explicitly place learned reranking, embeddings, model tokenizers, semantic-provider work, repository linker redesign, and HTTP transport outside v0.4.

- [x] **Step 6: Verify documentation-only changes**

Run:

```bash
git diff --check
go test ./...
```

Expected: no whitespace errors and no test regression.

- [x] **Step 7: Commit the baseline and design decision**

```bash
git add docs/design.md docs/implementation-plan.md PLAN.md testdata/eval/baseline-v0.4.json
git commit -m "docs: define LLM evidence contract v0.4"
```

If `baseline-v0.4.json` was not needed, omit it from `git add`. If the checkout was already dirty, stage only files changed by this task.

---

### Task 1: Define and Validate the Versioned Evidence Packet

**Files:**
- Create: `internal/evidence/model.go`
- Create: `internal/evidence/validate.go`
- Create: `internal/evidence/model_test.go`
- Create: `internal/evidence/validate_test.go`

**Interfaces:**
- Consumes: only standard library types
- Produces: `evidence.Packet`, enums, `Validate(Packet) error`, and deterministic local-ID helpers used by every later task

- [x] **Step 1: Write contract serialization tests**

Add table-driven tests that marshal a minimal packet and assert:

```go
func TestPacketJSONContract(t *testing.T) {
    packet := evidence.Packet{
        Schema: evidence.SchemaContextV1,
        Intent: "callers",
        Mode:   evidence.ModeFocused,
        Budget: evidence.Budget{Limit: 1200, Used: 380},
        Evidence: []evidence.Item{
            {
                ID:       "e1",
                Handle:   "sym_target",
                Role:     evidence.RoleTarget,
                Location: evidence.Location{Path: "auth/service.go", Lines: [2]int{44, 51}},
                Language: "go",
                Kind:     "method",
                Symbol:   "Service.ValidateToken",
                Fidelity: evidence.FidelitySignature,
                Signature: "func (s *Service) ValidateToken(token string) error",
                Why:      []string{"exact_symbol"},
            },
        },
    }

    data, err := json.Marshal(packet)
    if err != nil {
        t.Fatal(err)
    }
    text := string(data)
    for _, required := range []string{`"schema":"focalspan.context.v1"`, `"role":"target"`, `"fidelity":"signature"`} {
        if !strings.Contains(text, required) {
            t.Fatalf("missing %s in %s", required, text)
        }
    }
    for _, forbidden := range []string{`"score"`, `"weight"`, `"token_savings"`, `"query"`} {
        if strings.Contains(text, forbidden) {
            t.Fatalf("forbidden field %s in %s", forbidden, text)
        }
    }
}
```

Expected before implementation: compilation fails because `internal/evidence` does not exist.

- [x] **Step 2: Implement exact enums and public structs**

Create `model.go` with the exact schema identifier, modes, roles, fidelity values, certainty values, and public structs defined in the Product Contract section. Use constants for segment kinds:

```go
const (
    SegmentSource  = "source"
    SegmentOmitted = "omitted"
)
```

- [x] **Step 3: Write validation tests for every invariant**

Cover at least:

```text
wrong schema
unsupported mode
empty evidence array is valid
non-sequential local ID
blank stable handle
absolute path
backslash path
zero or reversed line range
unknown role
unknown fidelity
verbatim with missing source
verbatim with segments
excerpt with no source segment
excerpt with top-level source
signature with source
synthetic with no outline
edge to missing local ID
duplicate edge
more than four why codes
more than eight limitations
more than four next actions
used greater than limit
negative skipped_known
```

Use exact error substrings so callers can diagnose malformed packet assembly.

- [x] **Step 4: Implement `Validate`**

Required signature:

```go
func Validate(packet Packet) error
```

Validation must be read-only, deterministic, and must not repair malformed packets silently. Use joined or wrapped errors only when the resulting message remains stable enough for tests.

- [x] **Step 5: Add deterministic local-ID assignment**

Required helper:

```go
func AssignLocalIDs(items []Item) map[string]string
```

Behavior:

- assign `e1`, `e2`, ... in slice order;
- return a stable-handle-to-local-ID map;
- reject or surface duplicate non-empty handles through validation rather than silently mapping the last duplicate;
- do not derive IDs from scores, line numbers, map iteration, or random values.

- [x] **Step 6: Run focused tests**

```bash
go test ./internal/evidence -run 'TestPacket|TestValidate|TestAssignLocalIDs' -count=1
```

Expected: PASS.

- [x] **Step 7: Run full tests and commit**

```bash
go test ./...
git add internal/evidence
git commit -m "feat: define versioned evidence packet contract"
```

---

### Task 2: Preserve Relation Provenance from Store to Ranked Candidates

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: `internal/search/retrieval.go`
- Modify: `internal/search/retrieval_test.go`

**Interfaces:**
- Consumes: stored `relations` rows and existing `RelatedCandidates` behavior
- Produces: provenance-rich relation hits and `RankedCandidate.RelationContext` without changing the database schema or legacy public JSON

- [x] **Step 1: Add failing model and store tests**

Define the expected internal types in tests:

```go
type RelationDirection string

const (
    RelationIncoming RelationDirection = "incoming"
    RelationOutgoing RelationDirection = "outgoing"
    RelationRelated  RelationDirection = "related"
)

type RelationContext struct {
    AnchorHandle string
    Kind         string
    Direction    RelationDirection
    Confidence   float64
    Source       string
    Resolved     bool
}

type RelationHit struct {
    Candidate RankedCandidate
    Context   RelationContext
}
```

Store tests must create a small graph and assert exact provenance for:

```text
caller candidate -> anchor: incoming
anchor -> callee candidate: outgoing
parent -> child candidate: outgoing when anchor is parent
parent candidate -> child anchor: incoming when requesting parent
importer -> imported target: outgoing
importer candidate -> imported anchor: incoming for reverse import/export lookup
resolved ToHandle relation: Resolved=true
UnresolvedTo lexical relation: Resolved=false
confidence and source preserved from the relation row
```

Expected before implementation: the types and API are absent.

- [x] **Step 2: Add internal relation types to `model.go`**

Add the types above without JSON tags. Extend `RankedCandidate`:

```go
RelationContext *RelationContext
```

Keep the existing `Relation string` field for legacy ranking and output compatibility during v0.4.

- [x] **Step 3: Add a provenance-rich store API**

Required API:

```go
func (s *Store) RelatedCandidateHits(
    ctx context.Context,
    handles []string,
    relation string,
) ([]model.RelationHit, error)
```

Implementation requirements:

- use parameter binding;
- preserve current supported relation names;
- return actual edge direction for each SQL branch;
- return relation confidence and source;
- mark exact `ToHandle` joins resolved;
- mark simple-name or qualified-name fallback through `UnresolvedTo` unresolved;
- avoid N+1 queries by joining candidate symbol/chunk data in bounded queries;
- preserve deterministic ordering by relation class, confidence descending, path, start line, and handle;
- deduplicate by candidate handle plus anchor, kind, direction, source, and resolved state.

- [x] **Step 4: Keep the old store API as a compatibility wrapper**

Keep:

```go
func (s *Store) RelatedCandidates(
    ctx context.Context,
    handles []string,
    relation string,
) ([]model.RankedCandidate, error)
```

Implement it by calling `RelatedCandidateHits`, copying `Context.Kind` into legacy `Candidate.Relation`, and deduplicating candidates exactly as the old caller expects. Existing tests must continue to pass.

- [x] **Step 5: Update the search store interface and relation retriever**

Make the search candidate-store interface consume `RelatedCandidateHits`. For each relation hit:

```go
candidate := hit.Candidate
candidate.Relation = hit.Context.Kind
contextCopy := hit.Context
candidate.RelationContext = &contextCopy
```

Do not let a later duplicate with weaker lexical provenance replace an earlier resolved hit. Merge rules:

```text
resolved beats unresolved
higher confidence beats lower confidence
exact direction beats related
stable lexical order breaks remaining ties
```

- [x] **Step 6: Test relation provenance through `SearchDetailed`**

Add a search test for `what calls ValidateToken?` that verifies:

```text
candidate.Relation == "callers"
candidate.RelationContext.AnchorHandle is the ValidateToken anchor
candidate.RelationContext.Direction == incoming
candidate.RelationContext.Resolved matches the fixture relation
```

Also test the unresolved lexical path.

- [x] **Step 7: Run local and full verification**

```bash
go test ./internal/store ./internal/search -count=1
go test ./...
go vet ./...
```

Expected: all existing relation behavior remains compatible.

- [x] **Step 8: Commit**

```bash
git add internal/model/model.go internal/store/store.go internal/store/store_test.go internal/search/retrieval.go internal/search/retrieval_test.go
git commit -m "feat: preserve relation provenance for evidence output"
```

---

### Task 3: Classify Evidence Roles and Define Intent-Specific Presentation Order

**Files:**
- Create: `internal/evidence/roles.go`
- Create: `internal/evidence/roles_test.go`

**Interfaces:**
- Consumes: `query.Plan`, ranked candidate kind/path/language, changed flag, relation provenance, original rank
- Produces: `ClassifiedCandidate`, role, certainty, semantic reason codes, and deterministic presentation priority

- [x] **Step 1: Write role-classification tests**

Required internal type:

```go
type ClassifiedCandidate struct {
    Candidate       model.RankedCandidate
    OriginalRank    int
    Role            Role
    Certainty       Certainty
    Why             []string
    PresentationKey PresentationKey
}
```

Test at least these mappings:

```text
first exact anchor -> target
callers relation, incoming -> caller
callees relation, outgoing -> callee
tests relation -> test
imports outgoing -> import
imports incoming -> dependent
references -> reference
changed candidate under impact intent -> change
non-changed reverse dependency under impact -> dependent
kind test or test path -> test
kind class/interface/struct/record/trait/type -> type
kind template/block/template-function -> template
config/project/manifest path -> config
Markdown/documentation kind -> documentation
ordinary function/method target -> implementation
prototype/interface/ambient declaration -> declaration
unclassified supporting span -> context
```

- [x] **Step 2: Implement stable reason-code mapping**

Normal `why` codes may include only this bounded vocabulary in v1:

```text
exact_symbol
qualified_symbol
path_match
lexical_match
changed_span
direct_caller
direct_callee
related_test
imports_target
imported_by
references_target
contains_target
parent_context
same_symbol
same_file
```

Map current score/retriever reasons to these codes without copying weights or details. Remove duplicates and cap at four in this priority order:

```text
exact or qualified identity
relation
change
lexical or path
contextual fallback
```

- [x] **Step 3: Implement relation certainty mapping**

Rules:

```text
Resolved=true and confidence >= 0.90 -> exact
Resolved=true otherwise             -> scoped
Resolved=false                      -> lexical
No relation provenance              -> lexical only when an edge is not emitted
```

Do not emit an edge solely from a candidate role when there is no identifiable anchor.

- [x] **Step 4: Define intent presentation profiles**

Use exact role order tables:

```text
definition/default:
  target, implementation, definition, declaration, type, caller, callee, test,
  import, reference, config, template, change, dependent, context, documentation

callers:
  target, caller, implementation, test, type, import, reference, dependent,
  context, documentation

callees:
  target, callee, type, import, implementation, test, reference, context,
  documentation

tests:
  target, test, implementation, type, config, context, documentation

imports:
  target, import, export, dependent, implementation, type, config, context,
  documentation

references:
  target, reference, type, implementation, dependent, test, context,
  documentation

impact:
  change, target, dependent, caller, reference, test, implementation, type,
  config, context, documentation

template:
  target, template, import, implementation, caller, test, config, context,
  documentation
```

Within the same role, preserve original retrieval rank, then path, start line, and handle.

- [x] **Step 5: Implement classifier and ordering API**

Required signatures:

```go
func Classify(plan query.Plan, candidates []model.RankedCandidate) []ClassifiedCandidate
func SortForPresentation(plan query.Plan, candidates []ClassifiedCandidate)
```

The input candidate slice must not be mutated.

- [x] **Step 6: Test determinism across repeated runs and shuffled maps**

Run classification 100 times with the same candidates and assert byte-identical JSON after converting only stable fields. Include equal-score and duplicate-reason cases.

- [x] **Step 7: Verify and commit**

```bash
go test ./internal/evidence -run 'TestClassify|TestPresentation|TestWhy|TestCertainty' -count=1
go test ./...
git add internal/evidence/roles.go internal/evidence/roles_test.go
git commit -m "feat: classify and order LLM evidence by intent"
```

---

### Task 4: Build Source-Faithful Focused Excerpts

**Files:**
- Create: `internal/evidence/fidelity.go`
- Create: `internal/evidence/fidelity_test.go`
- Create: `internal/evidence/segments.go`
- Create: `internal/evidence/segments_test.go`
- Create: `testdata/repos/evidencesample/auth/service.go`
- Create: `testdata/repos/evidencesample/auth/service_test.go`
- Create: `testdata/repos/evidencesample/http/middleware.go`
- Create: `testdata/repos/evidencesample/config/auth.json`
- Create: `testdata/repos/evidencesample/unrelated/report.go`

**Interfaces:**
- Consumes: classified candidate, query plan, selected mode, item token allowance
- Produces: exactly one valid content representation with explicit source or omitted segments

- [x] **Step 1: Add a late-hit fixture that exposes current tail truncation**

Create `auth/service.go` with a `ValidateToken` function longer than 120 lines. Place routine normalization and logging near the beginning and the decisive branch near the end:

```go
if token.ExpiresAt.Before(now) {
    return ErrExpiredToken
}
```

The exact source line containing `ErrExpiredToken` must be expected evidence for the query:

```text
where is an expired authentication token rejected?
```

Add a caller, a test, a compact JSON config file, and a large unrelated Go file.

- [x] **Step 2: Write fidelity invariant tests**

Test `verbatim`, `excerpt`, `signature`, and `synthetic` item construction. For every source segment, assert:

```go
want := linesFromOriginal(candidate.Content, segment.Lines, candidate.StartLine)
if segment.Text != want {
    t.Fatalf("segment differs from indexed source")
}
```

Also assert no generated line-number prefixes and no literal `[...]` marker is inserted into source text.

- [x] **Step 3: Define internal content variants**

Required private type:

```go
type ContentVariant struct {
    Fidelity Fidelity
    Source   string
    Segments []Segment
    Outline  string
    Signature string
    EvidenceTokens int
}
```

Required API:

```go
func BuildVariants(
    candidate ClassifiedCandidate,
    plan query.Plan,
    mode Mode,
    estimator budget.TokenEstimator,
) []ContentVariant
```

Return variants from richest to cheapest. Every candidate must have a signature fallback; when the original signature is blank, build a compact fallback from symbol, kind, and location without pretending it is source.

- [x] **Step 4: Implement line indexing without rescanning per hit**

Create a line table once per candidate content. Preserve line endings in source segments. Calculate absolute source lines as:

```text
candidate.StartLine + local zero-based line index
```

Do not split a UTF-8 code point or alter CRLF/LF bytes in verbatim text.

- [x] **Step 5: Implement focused hit detection**

Use `query.Plan` terms and anchors. Match:

```text
case-sensitive exact identifiers first
ASCII case-insensitive identifiers second
qualified terminal symbol names
quoted phrases
Unicode lexical terms of length >= 2
relation anchor symbol terminal names when available
```

Do not treat every punctuation token as a hit.

- [x] **Step 6: Implement deterministic source windows**

For each hit line, start with:

```text
2 lines before
4 lines after
```

Always attempt to include a declaration prefix from candidate line 1 through the first opening body delimiter or a maximum of six lines. Merge windows whose gap is two lines or fewer. Keep at most three source windows, selected in this order:

```text
declaration prefix
window with the most distinct query terms
remaining windows by distinct-term count descending, then source order
```

Represent every excluded gap between selected windows as:

```go
Segment{Kind: SegmentOmitted, Lines: [2]int{start, end}}
```

Omitted segments have no `text` field.

- [x] **Step 7: Implement mode rules**

Exact behavior:

```text
outline:
  signature for source symbols;
  synthetic outline for extractor-generated outline chunks.

focused:
  target/implementation under 40 lines -> verbatim;
  large target or evidence with query hits -> excerpt;
  direct caller/callee/test under 24 lines -> verbatim;
  remaining supporting items -> signature or synthetic outline.

source:
  full verbatim candidate when it fits the item allowance;
  otherwise focused excerpt;
  otherwise signature.
```

Recognize synthetic outline chunks through existing kinds/signals rather than path-specific rules. Record the helper in one place so new languages can extend it.

- [x] **Step 8: Test that the late decisive branch survives**

At 512, 1200, and 4000 token allowances, `focused` must include the exact `ErrExpiredToken` branch or its containing source lines. It must not return only the beginning of the function.

- [x] **Step 9: Test hard cases**

Cover:

```text
single-line function
content with no trailing newline
CRLF
Japanese comment before a hit
UTF-8 identifier
multiple distant hits
more than three candidate windows
hit on first line
hit on last line
no lexical hit
synthetic outline
blank signature
source shorter than allowance
source larger than allowance
```

- [x] **Step 10: Verify and commit**

```bash
go test ./internal/evidence -run 'TestBuildVariants|TestFocused|TestSegments|TestFidelity' -count=1
go test ./...
git add internal/evidence testdata/repos/evidencesample
git commit -m "feat: add source-faithful focused evidence excerpts"
```

---

### Task 5: Compile Ranked Candidates into a Wire-Budgeted Packet

**Files:**
- Create: `internal/evidence/compiler.go`
- Create: `internal/evidence/compiler_test.go`
- Create: `internal/evidence/wire.go`
- Create: `internal/evidence/wire_test.go`

**Interfaces:**
- Consumes: query plan, revision, mode, token budget, ranked candidates, known handles
- Produces: `CompileResult` containing a valid `Packet`, canonical short summary measurement, and internal-only accounting statistics

- [x] **Step 1: Define compiler input and internal statistics**

Implement:

```go
type CompileRequest struct {
    Plan             query.Plan
    Revision         string
    TokenBudget      int
    Mode             Mode
    Candidates       []model.RankedCandidate
    KnownHandles     []string
    ExpansionAnchors []string
}

type Stats struct {
    WireTokens          int
    EvidenceTokens      int
    MetadataTokens      int
    DuplicateSourceBytes int
    Selected            int
    Omitted             int
    SkippedKnown        int
}

type CompileResult struct {
    Packet Packet
    Stats  Stats
}

type Compiler struct {
    estimator budget.TokenEstimator
}

func NewCompiler(estimator budget.TokenEstimator) *Compiler
func (c *Compiler) Compile(req CompileRequest) (CompileResult, error)
```

If estimator is nil, use `budget.NewEstimator()`.

- [x] **Step 2: Write failing tests for model-visible budget compliance**

For budgets 256, 512, 1200, 4000, and 64000, measure the same compact JSON plus canonical summary that the MCP handler will expose:

```go
used := evidence.MeasureModelVisible(result.Packet, estimator)
if used > result.Packet.Budget.Limit {
    t.Fatalf("wire packet uses %d > %d", used, result.Packet.Budget.Limit)
}
if used != result.Packet.Budget.Used {
    t.Fatalf("reported %d, measured %d", result.Packet.Budget.Used, used)
}
```

Also test clamping to existing `budget.MinBudget` and `budget.MaxBudget`.

- [x] **Step 3: Implement candidate preprocessing**

Before selection:

```text
remove candidates with blank handles only when no stable fallback can be generated
remove exact duplicate handles, preserving strongest provenance
remove duplicate content hashes, preserving the higher presentation priority
remove identical path/start/end spans
filter known handles and increment skipped_known
classify roles
build content variants
```

Do not count candidates rejected before ranking as omitted evidence.

- [x] **Step 4: Implement marginal utility per wire cost**

Use deterministic internal utility. Define named constants with these initial values:

```go
const (
    rankBaseUtility       = 100.0
    newRoleBonus          = 18.0
    newPathBonus          = 10.0
    directRelationBonus   = 16.0
    resolvedRelationBonus = 8.0
    exactIdentityBonus    = 14.0
    changedSpanBonus      = 12.0
    repeatedPathPenalty   = 7.0
)
```

Base utility:

```text
rankBaseUtility / (1 + original rank)
+ intent role weight
+ bonuses
- repeated path penalty
```

Use these role weights by intent:

```text
default target=45 implementation=34 definition=30 declaration=24 type=18
callers target=40 caller=46 test=20 implementation=18
callees target=40 callee=46 type=20 import=18
tests target=36 test=50 implementation=18 config=12
imports target=34 import=46 export=38 dependent=24 config=14
references target=38 reference=44 type=30 dependent=20
impact change=50 dependent=46 caller=30 test=28 target=26
template target=42 template=44 import=28 implementation=22
```

Unlisted roles receive 8. Divide the resulting utility by the incremental serialized token cost of adding the selected content variant. Use original rank, path, start line, and handle as deterministic tie breakers.

- [x] **Step 5: Guarantee a compact anchor before greedy selection**

When candidates exist, include the highest-priority target/change item at least as a signature if the minimal valid packet can fit. For callers, callees, tests, imports, and references intents, preserve a compact target signature even when the richest relation candidate scores above it.

Do not violate the wire budget to force an anchor. A 256-token packet may contain one signature item or an empty evidence array plus limitations.

- [x] **Step 6: Add variants using incremental serialized cost**

For each candidate, try richest-to-cheapest allowed variants. Measure each trial packet with `MeasureModelVisible`, including the canonical summary. Select the variant with the highest utility per incremental model-visible token that fits. Recompute role/path diversity after each selection.

Do not estimate candidate source alone as the hard-cap decision.

- [x] **Step 7: Assemble edges only after final item selection**

Use relation provenance and stable-handle-to-local-ID mapping. Edge orientation:

```text
incoming candidate relation: candidate local ID -> anchor local ID
outgoing candidate relation: anchor local ID -> candidate local ID
related/ambiguous relation: omit edge; retain a lexical why code and limitation
```

Map certainty through Task 3. If the anchor was suppressed by `known_handles`, do not emit a dangling edge. The item may retain role and why; add `known_anchor_not_repeated` to limitations once per packet.

- [x] **Step 8: Implement canonical summary and fixed-point model-visible token reporting**

Add these functions in `wire.go`:

```go
func Summary(packet Packet) string
func MeasureModelVisible(packet Packet, estimator budget.TokenEstimator) int
```

`Summary` must produce exactly the same short sentence later used by MCP and must contain no source, path, handle, query, score, or savings data. `MeasureModelVisible` estimates compact `json.Marshal(packet)` plus one newline plus `Summary(packet)`. Because changing `budget.used` can change both serialized length and summary digits, use at most four iterations:

```go
for i := 0; i < 4; i++ {
    measured := MeasureModelVisible(packet, estimator)
    if measured == packet.Budget.Used {
        break
    }
    packet.Budget.Used = measured
}
```

After the loop, remeasure. If the packet exceeds the limit, degrade or remove the lowest-utility non-anchor item and repeat. If only the anchor remains, degrade verbatim to excerpt to signature before removing it. Return an error only when even the empty valid packet cannot fit after budget clamping.

- [x] **Step 9: Implement internal accounting**

Definitions:

```text
WireTokens: estimator over compact final packet JSON plus the canonical short summary.
EvidenceTokens: estimator over concatenated signature/source/segment text/outline values only.
MetadataTokens: max(0, WireTokens - EvidenceTokens).
DuplicateSourceBytes: bytes repeated by overlapping source line ranges on the same path plus identical verbatim text reused across items.
```

Do not serialize `Stats` into normal MCP output.

- [x] **Step 10: Validate every compiled packet**

Call `evidence.Validate` before returning success. Compiler tests must fail if a future change produces a dangling edge, mixed content representation, invalid line range, duplicate handle, or wrong `budget.used`.

- [x] **Step 11: Test deterministic packing**

Run the same request 100 times, including candidates with tied scores and tied utility. Assert byte-identical `json.Marshal(result.Packet)` output.

- [x] **Step 12: Verify and commit**

```bash
go test ./internal/evidence -run 'TestCompiler|TestWire|TestBudget|TestUtility|TestDeterministic' -count=1
go test ./...
git add internal/evidence/compiler.go internal/evidence/compiler_test.go internal/evidence/wire.go internal/evidence/wire_test.go
git commit -m "feat: compile evidence within serialized token budgets"
```

---

### Task 6: Generate Compact Limitations and Follow-Up Actions

**Files:**
- Create: `internal/evidence/next.go`
- Create: `internal/evidence/next_test.go`
- Modify: `internal/evidence/compiler.go`
- Modify: `internal/evidence/compiler_test.go`

**Interfaces:**
- Consumes: query plan, selected/omitted classified candidates, selected variants, known handles, packet budget state
- Produces: bounded stable `limitations` and `next` entries

- [ ] **Step 1: Define the v1 limitation vocabulary**

Use these exact codes only:

```text
budget_limited
source_reduced_to_excerpt
source_reduced_to_signature
additional_callers_omitted
additional_callees_omitted
additional_tests_omitted
additional_imports_omitted
additional_references_omitted
dynamic_dispatch_unresolved
lexical_relation_only
known_anchor_not_repeated
no_relevant_source_found
syntax_only_impact
```

Remove duplicates and order by the list above.

- [ ] **Step 2: Define the v1 next-action reasons**

Use these exact values:

```text
more_callers_omitted
more_callees_omitted
more_tests_omitted
more_imports_omitted
more_references_omitted
source_body_omitted
parent_context_available
children_available
```

Supported next relations remain the current stable relation vocabulary.

- [ ] **Step 3: Write action-generation tests**

Cover:

```text
callers omitted -> anchor/callers action
callee source reduced to signature -> callee/self action
related test omitted -> target/tests action
parent context exists -> item/parent action
no duplicate handle/relation action
known target still usable as action anchor
no more than four actions
actions stable across candidate input ordering
```

- [ ] **Step 4: Implement `BuildGuidance`**

Required signature:

```go
func BuildGuidance(input GuidanceInput) (limitations []string, next []NextAction)
```

Define `GuidanceInput` with explicit selected, omitted, known, plan, and truncation fields. Do not let it inspect serialized JSON or global mutable state.

- [ ] **Step 5: Integrate guidance after final packet selection**

Guidance must be part of wire budgeting. If adding all guidance exceeds budget:

1. retain `budget_limited` when true;
2. retain the most intent-relevant next action;
3. remove lower-priority next actions;
4. remove lower-priority limitations;
5. never remove source evidence solely to preserve optional guidance.

Re-run the fixed-point token loop afterward.

- [ ] **Step 6: Verify and commit**

```bash
go test ./internal/evidence -run 'TestGuidance|TestLimitations|TestNext' -count=1
go test ./...
git add internal/evidence/next.go internal/evidence/next_test.go internal/evidence/compiler.go internal/evidence/compiler_test.go
git commit -m "feat: add compact evidence limitations and follow-ups"
```

---

### Task 7: Add Stateless `known_handles` Delta Suppression

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/mcp_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `internal/evidence/compiler_test.go`

**Interfaces:**
- Consumes: caller-supplied stable handles
- Produces: no retransmission of known evidence while preserving those handles as relation anchors

- [ ] **Step 1: Add validation tests for known handles**

Exact validation rules:

```text
maximum 512 input entries
trim surrounding ASCII/Unicode whitespace
ignore empty entries after trimming
deduplicate while preserving first occurrence
maximum 256 bytes per handle
reject NUL
reject control characters below U+0020 except no character is permitted after trimming
```

Do not require a specific prefix because existing handles vary by symbol/chunk type.

- [ ] **Step 2: Implement a shared validator**

Place validation in `internal/evidence` or an existing shared request-validation package, not separately in CLI and MCP. Required signature:

```go
func NormalizeKnownHandles(values []string) ([]string, error)
```

- [ ] **Step 3: Extend MCP tool inputs**

Add:

```go
KnownHandles []string `json:"known_handles,omitempty" jsonschema:"stable handles already present in the model context and not to be retransmitted"`
```

To:

```text
CodeContextInput
CodeExpandInput
CodeImpactInput
```

Do not add server-side session IDs or hidden state.

- [ ] **Step 4: Add CLI flags for evidence-format testing**

For `query`, `expand`, and `impact`, support repeatable:

```text
--known-handle HANDLE
```

Use a small `flag.Value` implementation in the CLI package. Do not parse comma-separated values because handles may evolve to contain punctuation.

- [ ] **Step 5: Preserve known handles as expansion anchors**

Filtering occurs only during packet compilation. Retrieval and relation expansion must still receive requested handles. Example:

```text
code_expand(handles=[A], relation=callers, known_handles=[A,B])
```

must expand from `A`, omit `A` and `B` from returned evidence, and return only new caller evidence.

- [ ] **Step 6: Test exact retransmission behavior**

Run two calls:

1. query returning handles A, B, C;
2. expand with known A, B, C.

Assert:

```text
none of A/B/C appears in packet.evidence
packet.skipped_known equals the number of otherwise selected known items
new relations do not reference missing local IDs
known_anchor_not_repeated appears when relevant
next actions may still name stable handle A
```

- [ ] **Step 7: Test deterministic normalization and limits**

Include duplicate handles, whitespace, Unicode, 256-byte boundary, 257-byte rejection, 512-entry boundary, 513-entry rejection, NUL, and cancellation through the enclosing request.

- [ ] **Step 8: Verify and commit**

```bash
go test ./internal/evidence ./internal/app ./internal/cli ./internal/mcpserver -run 'Known|Delta|Retransmit' -count=1
go test ./...
git add internal/evidence internal/app/service.go internal/app/service_test.go internal/mcpserver/server.go internal/mcpserver/mcp_test.go internal/cli/run.go internal/cli/run_test.go
git commit -m "feat: suppress previously delivered evidence by handle"
```

---

### Task 8: Add Evidence APIs to the Application Service Without Breaking Legacy Bundles

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Create: `internal/app/evidence.go`
- Create: `internal/app/evidence_test.go`

**Interfaces:**
- Consumes: existing query/expand/impact retrieval and new `evidence.Compiler`
- Produces: `QueryEvidence`, `ExpandEvidence`, and `ImpactEvidence` while preserving existing `Query`, `Expand`, and `Impact`

- [ ] **Step 1: Refactor candidate retrieval behind private result types**

Create private types:

```go
type candidateResult struct {
    Plan       query.Plan
    Revision   string
    Candidates []model.RankedCandidate
}
```

Create private methods that perform validation, optional index update, changed-range calculation, search, and revision lookup exactly once. Both legacy and evidence paths consume these methods.

Do not make `QueryEvidence` call legacy `Query` and reconstruct candidates from a packed bundle; that would lose omitted candidates and provenance.

- [ ] **Step 2: Preserve public legacy methods**

These signatures and defaults remain unchanged:

```go
func (s *Service) Query(ctx context.Context, req QueryRequest) (model.ContextBundle, error)
func (s *Service) Expand(ctx context.Context, req ExpandRequest) (model.ContextBundle, error)
func (s *Service) Impact(ctx context.Context, req ImpactRequest) (model.ContextBundle, error)
```

Existing CLI behavior and evaluator tests must pass byte-for-byte where they currently assert output.

- [ ] **Step 3: Add Evidence request types**

```go
type EvidenceQueryRequest struct {
    Query         string
    TokenBudget   int
    Mode          evidence.Mode
    ChangedOnly   bool
    Paths         []string
    NoUpdate      bool
    RetrievalMode search.RetrievalMode
    KnownHandles  []string
}

type EvidenceExpandRequest struct {
    Handles      []string
    Relation     string
    TokenBudget  int
    Mode         evidence.Mode
    KnownHandles []string
}

type EvidenceImpactRequest struct {
    BaseRef      string
    HeadRef      string
    TokenBudget  int
    Mode         evidence.Mode
    KnownHandles []string
}
```

- [ ] **Step 4: Add service Evidence methods**

Required signatures:

```go
func (s *Service) QueryEvidence(ctx context.Context, req EvidenceQueryRequest) (evidence.CompileResult, error)
func (s *Service) ExpandEvidence(ctx context.Context, req EvidenceExpandRequest) (evidence.CompileResult, error)
func (s *Service) ImpactEvidence(ctx context.Context, req EvidenceImpactRequest) (evidence.CompileResult, error)
```

Defaults:

```text
TokenBudget: service configuration default
Mode: focused
Query auto-update: same rule as legacy Query
Expand relation: self when blank, preserving current behavior
Impact limitation: syntax_only_impact
```

- [ ] **Step 5: Create deterministic plans for expand and impact**

Do not synthesize plan intent through loose English query text. Implement explicit helpers:

```go
func planForRelation(relation string) query.Plan
func planForImpact() query.Plan
```

Map relation to intent:

```text
callers -> callers
callees -> callees
tests -> tests
imports/exports -> imports
references -> references
parent/children/neighbors/self -> definition/default
```

- [ ] **Step 6: Construct the compiler once per service**

Extend `Service`:

```go
evidenceCompiler *evidence.Compiler
```

Initialize it with the same deterministic estimator family used by the legacy packer. Do not create compiler state per MCP request.

- [ ] **Step 7: Test shared retrieval parity**

For the same query:

```text
legacy and evidence paths receive the same ranked candidate handles before packing
evidence result contains the expected target path/symbol
legacy ContextBundle remains unchanged
auto-update runs once, not once per output representation
cancellation is propagated
evidence mode default is focused
```

Add an internal test seam only if needed; do not expose ranked candidates publicly.

- [ ] **Step 8: Verify and commit**

```bash
go test ./internal/app -run 'Test.*Evidence|TestQueryLegacy|TestExpandLegacy|TestImpactLegacy' -count=1
go test ./...
git add internal/app/service.go internal/app/service_test.go internal/app/evidence.go internal/app/evidence_test.go
git commit -m "feat: expose evidence compilation through application service"
```

---

### Task 9: Add a Human and JSON Evidence CLI Format

**Files:**
- Create: `internal/render/evidence.go`
- Create: `internal/render/evidence_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `README.md`

**Interfaces:**
- Consumes: `evidence.Packet`
- Produces: compact human-readable evidence and exact JSON while keeping current CLI output as default

- [ ] **Step 1: Add CLI format flags**

For `query`, `expand`, and `impact`, add:

```text
--format legacy|evidence
```

Default remains `legacy` in v0.4.

Mode behavior:

```text
legacy format: outline|source
Evidence format: outline|focused|source
Evidence default when --format evidence is selected: focused
```

Reject `--format legacy --mode focused` with a clear validation error.

- [ ] **Step 2: Implement JSON rendering**

Required function:

```go
func EvidenceJSON(packet evidence.Packet) ([]byte, error)
```

Use indented JSON for CLI output. Validate the packet before marshaling. Do not add a CLI-only envelope that changes the contract.

- [ ] **Step 3: Implement compact human rendering**

Required function:

```go
func EvidenceCompact(packet evidence.Packet) string
```

Expected structure:

```text
schema: focalspan.context.v1
intent: callers
mode: focused
budget: 934/1200

[e1 target] auth/service.go:44-87
Service.ValidateToken
------------------------------------------------
<source or segment text>

[e2 caller] http/middleware.go:21-38
Authenticate
why: direct_caller
------------------------------------------------
<source>

relations:
  e2 -calls/exact-> e1

next:
  callers sym_Q7K... (more_callers_omitted)
```

Rules:

- no floating-point scores;
- no per-line number prefix;
- omitted segments render as `--- lines 53-113 omitted ---` outside code text;
- source text remains unchanged;
- no token-savings section;
- no debug reasons/weights;
- omit empty sections;
- deterministic newline behavior.

- [ ] **Step 4: Keep debug ranking separate**

`--debug-scores` remains valid only for legacy output or existing `explain`. Reject `--format evidence --debug-scores` and direct the user to `focalspan explain`.

- [ ] **Step 5: Wire CLI commands to the correct service method**

Dispatch:

```text
legacy -> Query/Expand/Impact
Evidence -> QueryEvidence/ExpandEvidence/ImpactEvidence
```

Pass repeatable `--known-handle` values only to Evidence methods. `--json` selects JSON versus human rendering, not the underlying contract.

- [ ] **Step 6: Add CLI tests**

Cover:

```text
legacy output unchanged by default
evidence JSON validates and has schema
evidence human output includes roles
evidence human output excludes score and savings
focused accepted only for Evidence
known handles passed through
debug-scores rejected for Evidence
invalid format rejected
query shortcut still uses legacy default
stdout/stderr separation
```

- [ ] **Step 7: Document preview commands**

Add to README:

```bash
focalspan query --format evidence --mode focused --query "ValidateToken の呼び出し元" --budget 1200
focalspan query --format evidence --mode focused --query "ValidateToken の呼び出し元" --budget 1200 --json
focalspan expand --format evidence --handle sym_... --relation callers --known-handle sym_...
```

State clearly that MCP always uses Evidence Packet v1 while CLI defaults to legacy during v0.4.

- [ ] **Step 8: Verify and commit**

```bash
go test ./internal/render ./internal/cli -count=1
go test ./...
git add internal/render/evidence.go internal/render/evidence_test.go internal/cli/run.go internal/cli/run_test.go README.md
git commit -m "feat: add evidence packet CLI rendering"
```

---

### Task 10: Switch MCP Context Tools to Evidence Packet v1

**Files:**
- Modify: `internal/mcpserver/server.go`
- Modify: `internal/mcpserver/mcp_test.go`
- Modify: `README.md`
- Modify: `docs/design.md`

**Interfaces:**
- Consumes: service Evidence APIs
- Produces: versioned typed structured output for context, expand, and impact with source appearing once

- [ ] **Step 1: Write failing typed-output tests**

For `code_context`, assert the typed handler returns `evidence.Packet` and that the SDK-generated output schema requires:

```text
schema
mode
budget
evidence
```

Assert `schema` serializes as `focalspan.context.v1` and each evidence item schema includes `id`, `handle`, `role`, `location`, and `fidelity`.

- [ ] **Step 2: Change the three handler output types**

Change only:

```text
code_context
code_expand
code_impact
```

To return `evidence.Packet` as structured output. Keep status and restart outputs unchanged.

- [ ] **Step 3: Make MCP mode default `focused`**

Input accepts:

```text
outline
focused
source
```

Blank mode becomes `focused`. Invalid mode returns a typed tool validation error with no stack trace.

- [ ] **Step 4: Pass normalized known handles**

Normalize and validate `known_handles` once in each handler before calling the service. Return a compact user-correctable error when invalid.

- [ ] **Step 5: Improve tool descriptions for model use**

Use these descriptions or text with identical semantics:

```text
code_context:
  Find and return a role-labeled packet of repository evidence for a code question.
  Call this before broad file reads. Use handles and next actions for follow-up expansion.

code_expand:
  Return new evidence related to stable handles. Pass known_handles to avoid retransmitting
  context already present in the conversation.

code_impact:
  Return syntax-based changed spans, dependents, and related tests for Git changes within
  a token budget. Results may omit unresolved dynamic relationships.
```

Do not turn descriptions into a long tutorial.

- [ ] **Step 6: Keep text content to one short summary**

Text content must be produced by `evidence.Summary(packet)` and have this exact format:

```text
FocalSpan evidence: <n> items, <used>/<limit> tokens, <omitted> omitted.
```

Maximum 160 Unicode code points. It must not contain query text, source code, signatures, paths, handles, score details, or savings values.

- [ ] **Step 7: Add in-memory MCP integration tests**

Using the SDK client/session, verify:

```text
initialize succeeds
tools/list still returns exactly the five existing tool names
code_context returns structuredContent with schema v1
code_expand accepts known_handles
code_impact includes syntax_only_impact limitation
invalid mode returns tool error
invalid known handle returns tool error
cancellation propagates
code_status contract unchanged
code_restart contract unchanged
```

- [ ] **Step 8: Add raw stdio JSON-RPC duplication test**

Start the actual `focalspan serve` subprocess against the evidence fixture. Send initialize, tools/list, and tools/call requests. Place a unique source marker in the fixture:

```text
FOCALSPAN_UNIQUE_EVIDENCE_MARKER_9F2A
```

Assert the raw tools/call response line contains that marker exactly once. Also assert:

```text
structuredContent contains the marker
content[0].text does not contain the marker
raw response has no "score" key
raw response has no "weight" key
raw response has no "token_savings" key
stdout contains JSON-RPC only
stderr may contain logs but no source marker
```

Count escaped JSON occurrences carefully by parsing first and use raw counting only for the unique marker.

- [ ] **Step 9: Bump the MCP implementation version**

Change:

```go
mcp.Implementation{Name: "focalspan", Version: "0.1.0"}
```

To:

```go
mcp.Implementation{Name: "focalspan", Version: "0.4.0"}
```

Do not infer the executable release version from this field elsewhere.

- [ ] **Step 10: Document the intentional pre-1.0 output change**

README and design docs must say:

```text
- Tool names and inputs remain compatible except for the additive known_handles and focused mode.
- code_context/code_expand/code_impact structured outputs now use focalspan.context.v1.
- Consumers must read structuredContent rather than parse the short text summary.
- Numeric ranking diagnostics remain available through focalspan explain, not MCP context responses.
```

- [ ] **Step 11: Verify and commit**

```bash
go test ./internal/mcpserver -count=1
go test ./...
go vet ./...
git add internal/mcpserver/server.go internal/mcpserver/mcp_test.go README.md docs/design.md
git commit -m "feat: return evidence packets from MCP context tools"
```

---

### Task 11: Add Evidence Contract Evaluation and Legacy A/B Comparison

**Files:**
- Create: `internal/eval/evidence.go`
- Create: `internal/eval/evidence_test.go`
- Modify: `internal/eval/eval.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Create: `testdata/eval/evidence-cases.jsonl`
- Modify: `docs/evaluation.md`

**Interfaces:**
- Consumes: legacy and Evidence service methods, evidence fixture, token estimator
- Produces: measurable wire/quality comparisons without weakening existing retrieval evaluation

- [ ] **Step 1: Define evidence evaluation cases**

Implement:

```go
type EvidenceExpectation struct {
    Path       string          `json:"path"`
    Symbol     string          `json:"symbol,omitempty"`
    Roles      []evidence.Role `json:"roles,omitempty"`
    Contains   []string        `json:"contains,omitempty"`
    Fidelity   []evidence.Fidelity `json:"fidelity,omitempty"`
}

type EvidenceCase struct {
    Name             string                `json:"name"`
    Query            string                `json:"query"`
    TokenBudget      int                   `json:"token_budget"`
    Mode             evidence.Mode         `json:"mode"`
    Expected         []EvidenceExpectation `json:"expected"`
    ForbiddenPaths   []string              `json:"forbidden_paths,omitempty"`
    FollowUpRelation string                `json:"follow_up_relation,omitempty"`
}
```

- [ ] **Step 2: Create cross-language evidence cases**

At minimum include:

```text
Go late expired-token branch
Go caller relationship
PHP method plus PHPUnit test
C++ declaration plus implementation/caller
C# method plus test
TypeScript function plus importer/caller
Smarty block plus embedded JavaScript
```

Use current checked-in fixtures where suitable and the dedicated `evidencesample` for late-hit and delta scenarios. Do not add production hard-coding for case names.

- [ ] **Step 3: Define evidence metrics**

Implement per-case and aggregate metrics:

```go
type EvidenceCaseResult struct {
    Name                  string  `json:"name"`
    ExpectedCoverage      float64 `json:"expected_coverage"`
    RoleAccuracy          float64 `json:"role_accuracy"`
    FidelityValid         int     `json:"fidelity_valid"`
    RelationValid         int     `json:"relation_valid"`
    WireBudgetCompliant   int     `json:"wire_budget_compliant"`
    WireTokens            int     `json:"wire_tokens"`
    EvidenceTokens        int     `json:"evidence_tokens"`
    MetadataOverheadRatio float64 `json:"metadata_overhead_ratio"`
    DuplicateSourceRatio  float64 `json:"duplicate_source_ratio"`
    KnownResendCount      int     `json:"known_resend_count"`
    Deterministic         int     `json:"deterministic"`
    LegacyWireTokens      int     `json:"legacy_wire_tokens"`
    EvidenceVsLegacyRatio float64 `json:"evidence_vs_legacy_ratio"`
}
```

Aggregate medians and compliance rates in an `EvidenceReport`.

Definitions:

```text
ExpectedCoverage: expected path/symbol/content expectations found / total expectations.
RoleAccuracy: matched expectations whose returned role is allowed / matched expectations.
FidelityValid: all item fidelity invariants pass.
RelationValid: all edges reference local IDs and expected direct relations are present.
MetadataOverheadRatio: metadata tokens / wire tokens; zero when wire tokens are zero.
DuplicateSourceRatio: duplicate source bytes / total returned source bytes; zero when no source.
KnownResendCount: number of known stable handles incorrectly retransmitted.
EvidenceVsLegacyRatio: Evidence Packet serialized tokens / legacy ContextBundle serialized tokens.
```

- [ ] **Step 4: Add evaluator contract modes**

Extend `focalspan eval` with:

```text
--contract legacy|evidence|compare
```

Default remains `legacy`.

Behavior:

```text
legacy: current evaluator and report unchanged
Evidence: load EvidenceCase and emit EvidenceReport
compare: run both output paths for compatible evidence cases and include A/B fields
```

Do not reinterpret old case files as Evidence cases without an explicit contract flag.

- [ ] **Step 5: Add a two-step delta evaluation**

For cases with `follow_up_relation`:

1. run `QueryEvidence`;
2. collect all returned stable handles;
3. expand the best target handle with those handles in `known_handles`;
4. assert no known handle is retransmitted;
5. compare cumulative wire tokens with and without `known_handles`.

Add aggregate `delta_token_ratio`.

- [ ] **Step 6: Add A/B acceptance tests**

The checked-in evidence suite must meet:

```text
wire budget compliance                 = 1.0
fidelity validity                      = 1.0
relation validity                      = 1.0
deterministic output                   = 1.0
forbidden path violations              = 0
known resend count                     = 0
expected evidence coverage             = 1.0
role accuracy                          = 1.0
focused late-hit preservation          = 1.0
median duplicate source ratio          <= 0.05
median metadata overhead ratio         <= 0.35 for budgets >= 1200
median Evidence-vs-legacy wire ratio   <= 1.00 for focused cases
median two-step delta token ratio      <= 0.70
```

Do not weaken current legacy hit@5, recall, forbidden-path, budget, reduction, relation, intent, or determinism thresholds.

- [ ] **Step 7: Test forbidden output fields**

Every evaluated serialized packet must be recursively inspected and fail if any object key equals:

```text
score
retrieval_score
weight
detail
token_savings
baseline_tokens
saved_tokens
savings_ratio
```

This must inspect keys, not naive substring matches inside source code.

- [ ] **Step 8: Document metric meaning and limitations**

Update `docs/evaluation.md` with:

```text
wire budget versus evidence token count
metadata overhead
source duplication
role and relation correctness
focused late-hit preservation
stateless delta savings
legacy A/B comparison
why one-response size is not enough without cumulative tool-result tokens
```

Record actual measured results after implementation; do not copy acceptance thresholds into the results section as if they were measurements.

- [ ] **Step 9: Verify and commit**

```bash
go test ./internal/eval ./internal/cli -count=1
go test ./...
go build -o ./focalspan-eval ./cmd/focalspan
./focalspan-eval eval --root testdata/repos/evidencesample --cases testdata/eval/evidence-cases.jsonl --contract compare --json
rm -f ./focalspan-eval ./focalspan-eval.exe
git add internal/eval internal/cli/run.go internal/cli/run_test.go testdata/eval/evidence-cases.jsonl docs/evaluation.md
git commit -m "test: evaluate LLM evidence packets and delta savings"
```

On Windows, use an output path under a temporary directory and remove the `.exe` afterward.

---

### Task 12: Harden Edge Cases, Fuzz Invariants, and Compatibility

**Files:**
- Create: `internal/evidence/fuzz_test.go`
- Modify: `internal/evidence/*_test.go`
- Modify: `internal/mcpserver/mcp_test.go`
- Modify: `internal/app/evidence_test.go`

**Interfaces:**
- Consumes: complete Evidence implementation
- Produces: robust invariants for malformed source content, tiny budgets, repeated relations, and protocol output

- [ ] **Step 1: Add packet/compiler fuzz seeds**

Seed with:

```text
empty candidate list
single compact signature
long Go function
C++ raw string
C# interpolated raw string
JavaScript template literal and JSX
PHP heredoc
Smarty block with embedded script
CRLF
Japanese identifiers/comments
invalid UTF-8 bytes already converted to indexed replacement text only where current scanner permits
```

- [ ] **Step 2: Enforce fuzz invariants**

For every successful compile:

```text
no panic
Validate succeeds
serialized packet fits budget
budget.used matches remeasurement
local IDs sequential
all edges local and valid
one content representation per item
source/segment UTF-8 valid
same request produces identical JSON
known handles absent from evidence
no forbidden debug key
```

Fuzz tests may discard impossible malformed model candidates, but must not hide panics.

- [ ] **Step 3: Add tiny-budget regression matrix**

Test budgets before clamping and at exact boundaries:

```text
0
1
255
256
257
511
512
1199
1200
63999
64000
64001
```

Expected: clamped budget is reported, packet remains valid, no over-budget output.

- [ ] **Step 4: Add relation ambiguity tests**

Construct multiple `Validate` methods. Assert lexical unresolved provenance does not produce an `exact` edge. It may return multiple caller/reference items with `lexical_relation_only`, but must not invent a resolved target.

- [ ] **Step 5: Add mixed known-anchor relation tests**

Cover:

```text
anchor known, candidate new
anchor new, candidate known
both known
neither known
multiple anchors with one known
```

No dangling edge is allowed.

- [ ] **Step 6: Add protocol compatibility tests**

Assert:

```text
five MCP tool names unchanged
project/user MCP registration still lists all five enabled tools
code_status JSON unchanged
code_restart behavior unchanged
legacy CLI query output remains available
focalspan explain remains source-free
```

- [ ] **Step 7: Run bounded fuzzing locally**

```bash
go test ./internal/evidence -run '^$' -fuzz FuzzCompile -fuzztime 20s
go test ./internal/evidence -run '^$' -fuzz FuzzValidate -fuzztime 20s
```

If the Go toolchain requires one fuzz target per command, run them separately exactly as above.

- [ ] **Step 8: Run full regression and commit**

```bash
go test ./...
go vet ./...
git add internal/evidence internal/mcpserver/mcp_test.go internal/app/evidence_test.go
git commit -m "test: harden evidence packet invariants"
```

---

### Task 13: Complete Documentation, Full Verification, and Release Readiness

**Files:**
- Modify: `README.md`
- Modify: `docs/design.md`
- Modify: `docs/evaluation.md`
- Modify: `docs/implementation-plan.md`
- Modify: `AGENTS.md` only when a durable project rule is missing
- Modify: `PLAN.md`

**Interfaces:**
- Consumes: all completed v0.4 behavior and measured results
- Produces: truthful documentation and a reproducible verification report

- [ ] **Step 1: Document the Evidence Packet contract**

README must include a compact valid example:

```json
{
  "schema": "focalspan.context.v1",
  "intent": "callers",
  "mode": "focused",
  "budget": {"limit": 1200, "used": 934, "truncated": true, "omitted": 2},
  "evidence": [
    {
      "id": "e1",
      "handle": "sym_target",
      "role": "target",
      "location": {"path": "auth/service.go", "lines": [44, 51]},
      "language": "go",
      "kind": "method",
      "symbol": "Service.ValidateToken",
      "signature": "func (s *Service) ValidateToken(token string) error",
      "fidelity": "signature",
      "why": ["exact_symbol"]
    }
  ]
}
```

The example values must fit the schema and must not claim they are measured output.

- [ ] **Step 2: Document how LLM consumers should use it**

State:

```text
- Start with code_context in focused mode.
- Treat evidence source and source segments as verbatim indexed code.
- Treat synthetic outlines as generated navigation aids, not source code.
- Use role and relations to distinguish target, caller, test, and dependency evidence.
- Use next actions with code_expand.
- Pass stable handles through known_handles to avoid repeated context.
- Do not parse the short MCP text summary for source.
```

- [ ] **Step 3: Document compatibility and migration**

Explain:

```text
CLI default remains legacy in v0.4.
CLI Evidence preview uses --format evidence.
MCP context tools return focalspan.context.v1.
Tool names remain unchanged.
code_status and code_restart outputs remain unchanged.
Normal MCP packets no longer expose ranking scores and token-savings diagnostics.
focalspan explain remains the debugging interface.
```

- [ ] **Step 4: Document fidelity and omission semantics**

Define `verbatim`, `excerpt`, `signature`, `synthetic`, source and omitted segments, line ranges, and the guarantee that omitted markers are metadata rather than injected code.

- [ ] **Step 5: Update architecture diagrams**

Use this pipeline:

```text
repository -> extraction -> SQLite/FTS5 -> query plan -> retrievers -> RRF/ranking
           -> ranked candidates + relation provenance
           -> Evidence Compiler
              -> role classifier
              -> fidelity/segment builder
              -> utility-per-wire-token selection
              -> local relations and guidance
              -> serialized hard-cap verification
           -> CLI Evidence renderer or MCP structuredContent
```

Keep the legacy packer shown as a compatibility branch, not the future canonical MCP path.

- [ ] **Step 6: Update durable contributor rules only when needed**

If `AGENTS.md` does not already cover them, add concise rules:

```text
Evidence Packet source fidelity is a public contract.
Normal MCP context responses must not expose ranking/debug fields.
MCP source must occur once in structuredContent and never in text summaries.
Every Evidence change requires wire-budget and invariant tests.
```

Do not paste task-specific checklists into `AGENTS.md`.

- [ ] **Step 7: Run formatting and static checks**

```bash
gofmt -w .
git diff --check
go test ./...
go vet ./...
```

Expected: PASS.

- [ ] **Step 8: Run race tests where supported**

```bash
go test -race ./...
```

If the current Windows environment lacks a compatible C compiler/linker, report it as unverified rather than passed. Run it in Linux CI or another supported environment before a release claim when available.

- [ ] **Step 9: Run CGO-free native and cross-builds**

```bash
CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/focalspan
```

Direct artifacts to a temporary directory or remove generated binaries after verification.

- [ ] **Step 10: Run every legacy evaluation**

Run all checked-in legacy case files using their matching fixture roots. Compare against Task 0 baseline. Requirements:

```text
no lower hit@5
no lower symbol/path recall
no new forbidden-path violation
budget compliance remains 1.0
no lower determinism
no lower expected relation/intent/kind recall
```

- [ ] **Step 11: Run Evidence comparison evaluation**

```bash
focalspan eval \
  --root testdata/repos/evidencesample \
  --cases testdata/eval/evidence-cases.jsonl \
  --contract compare \
  --json
```

Run any additional language-specific Evidence case roots documented by Task 11. Confirm all v0.4 acceptance thresholds from Task 11.

- [ ] **Step 12: Run manual MCP smoke tests**

Against an indexed fixture or safe local repository:

```text
code_context(query="ValidateToken の呼び出し元", token_budget=1200)
code_expand(handles=[target], relation="callers", known_handles=[all prior handles], token_budget=1200)
code_impact(token_budget=1600)
code_status()
code_restart()
```

Inspect the actual payload and confirm:

```text
roles make sense
source appears once
late relevant lines survive focused mode
known evidence is not resent
relations reference valid local IDs
no debug weights or savings fields appear
```

- [ ] **Step 13: Self-review the full diff**

Check every item:

```text
internal ContextBundle still works
MCP output uses only Evidence Packet for three context tools
focused is MCP default
source/segments are faithful
wire budget uses final JSON
budget.used is truthful
no dangling relation IDs
known handles are stateless
normal packet excludes numeric debug data
text content is short and source-free
legacy CLI/eval remain available
no SQLite migration
no extractor regression
no fixture-specific production branch
no incomplete production path
README matches implementation
```

Fix discovered issues and rerun affected tests.

- [ ] **Step 14: Mark this plan with actual completion evidence**

Check a box only after its command or assertion has been verified. Add a short final section to `docs/evaluation.md` containing actual command results, dates, platform, Go version, and any unverified race coverage.

- [ ] **Step 15: Commit final docs and verification updates**

```bash
git add README.md docs/design.md docs/evaluation.md docs/implementation-plan.md AGENTS.md PLAN.md
git commit -m "docs: complete LLM evidence contract v0.4"
```

Stage `AGENTS.md` only if it changed.

---

## Definition of Done

The milestone is complete only when all statements below are true:

```text
focalspan.context.v1 exists as a validated typed contract.
MCP code_context returns role-labeled Evidence Packet structured output.
MCP code_expand returns delta-friendly Evidence Packet structured output.
MCP code_impact returns Evidence Packet with syntax-only limitation.
code_status and code_restart contracts are unchanged.
MCP default output mode is focused.
CLI legacy output remains the default during v0.4.
CLI can preview Evidence format in human and JSON forms.
Each item has a stable role and exactly one fidelity representation.
Verbatim source and source segments match indexed source exactly.
Large functions preserve query-relevant late branches.
Omitted source ranges are metadata, not fake code lines.
Relations use packet-local IDs and never dangle.
Relation certainty distinguishes exact/scoped/lexical evidence.
Normal packets omit floating-point ranking and savings diagnostics.
Compact packet JSON plus the canonical short summary respects the token budget.
Reported budget.used matches final serialized remeasurement.
Known handles are not retransmitted but remain usable as expansion anchors.
No conversation/session state is stored on the MCP server.
Text content is a short source-free summary.
Source appears exactly once in raw MCP tools/call output.
Evidence evaluation measures wire cost, metadata, duplication, fidelity, roles, relations, and delta savings.
All v0.4 Evidence acceptance thresholds pass.
All existing retrieval and language evaluations meet or exceed the recorded baseline.
All unit and integration tests pass.
go vet passes.
CGO-free native and required cross-builds pass.
Race results are reported truthfully.
Documentation matches the implemented contract.
No database schema change or extractor redesign was introduced.
No user changes were reset, restored, cleaned, or stashed.
```

---

## Required Final Report from Codex

Report in this exact order:

1. Starting commit and whether the working tree was dirty.
2. Evidence Packet architecture and why the legacy bundle remains.
3. Public `focalspan.context.v1` fields and compatibility impact.
4. Role-classification and presentation-order behavior.
5. Verbatim/excerpt/signature/synthetic fidelity behavior.
6. Focused excerpt algorithm and late-hit proof.
7. Relation provenance, local edge IDs, and certainty mapping.
8. Serialized wire-budget algorithm and measured accuracy.
9. `known_handles` behavior and cumulative token reduction.
10. MCP text-versus-structured output and source-duplication result.
11. CLI Evidence preview commands.
12. New or modified major files.
13. Unit/integration/fuzz commands and results.
14. Legacy evaluation comparison against the recorded baseline.
15. Evidence evaluation metrics and acceptance results.
16. Native and cross-build results.
17. Race-test result or exact reason it remains unverified.
18. Git commits created.
19. Known limitations that remain after v0.4.
20. Confirmation that user changes were not reset, restored, cleaned, or stashed.

Do not claim completion when any required test, evaluation threshold, schema invariant, wire budget, or cross-build remains unsuccessful. State the exact failing command, observed result, likely impact, and remaining work instead.

# FocalSpan Design

## Goal

`FocalSpan` is a token-first code context compiler. It scans one repository, extracts
source spans and lightweight relations, searches them for a natural-language
question, and packs a deterministic context bundle within an explicit token
budget. It is an independent implementation and is not compatible with or
copied from Code Review Graph.

The MVP is deliberately syntax-oriented. It must be useful without running
repository code, installing dependencies, contacting a network, calling an
LLM, or requiring CGO.

## Constraints and selected dependencies

- Go language version: 1.26 or newer; the development verification uses Go 1.27.
- Module path: `github.com/focalspan/focalspan`.
- SQLite driver: `modernc.org/sqlite v1.57.0`.
- MCP SDK: `github.com/modelcontextprotocol/go-sdk v1.7.0`.
- TOML validation: `github.com/pelletier/go-toml/v2 v2.2.4`; existing config is
  never reserialized.
- SQLite access goes through `database/sql`; migrations are embedded with
  `go:embed`.
- Logging uses `log/slog` and is directed to stderr by the CLI and MCP server.
- The default index is `.focalspan/index.db`; source content is stored in that DB.

The dependency versions were selected by a module-version and minimal-driver
spike. The implementation must verify CGO-free compilation, an FTS5 virtual
table, `MATCH`, `bm25`, and a Windows cross-build before the dependency choice
is considered complete.

## System boundary

```text
repository + config + git state
        |
        v
Repository Scanner -> Language Detection -> Extractor Registry
        -> Indexer / single DB writer -> SQLite tables + FTS5
        -> Candidate Retrieval -> Deterministic Reranker
        -> Context Packer -> Compact/JSON Renderer
        -> CLI and MCP stdio adapters
```

The core packages do not know about CLI flags or MCP wire messages. CLI and MCP
adapters build the same application service and therefore cannot accidentally
diverge in ranking or budget behavior.

## Package responsibilities

- `internal/model`: source, symbol, chunk, relation, diagnostic, ranked result,
  bundle, status, and evaluation types.
- `internal/config`: strict JSON decoding with unknown-key warnings, defaults,
  bounded values, root-relative include/exclude patterns, and config hashing.
- `internal/repository`: root detection, path containment, safe file walking,
  language detection, binary/UTF-8/BOM/line handling, and secret exclusion.
- `internal/gitx`: separated-argument Git commands, file listing, diff modes,
  and zero-context hunk parsing.
- `internal/extract`: extractor interface and registry.
- `internal/extract/goast`: syntax-only Go extraction using `go/parser`,
  `go/ast`, and `go/token`.
- `internal/extract/php`: stateful PHP lexer plus namespace-aware structural
  extraction for declarations, members, relations, includes, PHPUnit tests,
  and partial recovery.
- `internal/extract/template`: stateful Smarty/HTML composite scanning,
  block/function/capture extraction, bounded embedded regions, and conservative
  include/extends/script relations. It never renders a template.
- `internal/extract/cpp`: pure-Go C/C++ lexer/parser for C and C++ source and
  header extensions, declarations, namespaces, types, methods, calls,
  references, includes, tests, and partial recovery.
- `internal/extract/csharp`: pure-Go C# lexer/parser for namespaces, types,
  methods, properties/events, calls, references, `using`, tests, and partial
  recovery.
- `internal/extract/jsts`: pure-Go JavaScript/TypeScript lexer/parser for
  ESM/CommonJS modules, namespaces, classes/interfaces/types, functions,
  methods/arrows, JSX/TSX, calls, references, tests, and partial recovery.
- `internal/extract/generic`: C-like lexer, Python-like indentation chunks,
  Markdown headings, and bounded line-window fallback.
- `internal/indexer`: bounded parse workers and a single transactional writer;
  SHA-256 equality, not mtime, decides whether a file is reparsed.
- `internal/store`: SQLite setup, migrations, CRUD, FTS5 synchronization,
  revision metadata, and read-only status/query operations.
- `internal/search`: query normalization, exact/lexical/FTS/path candidates,
  relation expansion, and changed-line candidates.
- `internal/rank`: named score weights, reasons, deterministic tie-breaking,
  and overlap/content-hash deduplication.
- `internal/budget`: conservative token estimator, elision, and budget packer.
- `internal/render`: compact human output and typed JSON/MCP payloads.
- `internal/mcpserver`: official SDK server setup, five typed tools including
  the in-process restart API, and protocol-safe stdio behavior.
- `internal/integration/codex`: Codex project-config managed blocks, user-scope
  `codex mcp` subprocess adapter, registration status, and safe executable
  resolution. It never reserializes an existing TOML document.
- `internal/eval`: JSON/JSONL cases and repeatable retrieval metrics.
- `internal/cli`: flag parsing and command orchestration only.

Interfaces are kept at boundaries that have an actual second implementation:
`Extractor`, `TokenEstimator`, and the store-facing service interfaces. A future
semantic provider enters through extractor/provider registration and does not
change search, packing, storage, or MCP contracts.

## Domain model and stable handles

The public domain model includes `SourceFile`, `Symbol`, `Chunk`, `Relation`,
`Diagnostic`, `RankedCandidate`, `ScoreReason`, `ContextBundle`, and
`ContextItem`. Spans carry both byte offsets and one-based line ranges.

Handles are never SQLite auto-increment IDs. A symbol handle is derived from
repository-relative slash-normalized path, language, kind, qualified name, and
normalized signature. A chunk handle additionally incorporates symbol identity,
kind, and content span identity. SHA-256 is encoded with a URL-safe base64
prefix. The store checks shortened-hash collisions and extends the digest for
the colliding values. Line movement therefore does not change a symbol handle
unless its identity/signature changes.

## Repository and Git safety

Git repositories use `git ls-files -co --exclude-standard -z`, executed with
`exec.CommandContext` and separate arguments. Non-Git directories use a bounded
filesystem walk. Canonical internal paths are repository-relative with `/`
separators. Every read resolves symlinks and verifies containment under the
bound root; directory symlinks are not recursively followed.

The scanner rejects binary or invalid UTF-8 files, files over 2 MiB by default,
and generated/build/vendor/index directories. `.env`, `.env.*`, `*.pem`,
`*.key`, SSH keys, `credentials.json`, and `secrets.json` are skipped by
default. An explicit include pattern is the only way to opt a secret-shaped
path back in. The scanner never logs file content or stores an outside-root
path.

The Git adapter distinguishes unstaged, staged, base..head, and untracked
states. Diff commands use `--unified=0 --no-ext-diff`; hunks become one-based
changed-line ranges. Rename, delete, binary, empty, and CRLF cases are handled
as data, not shell text.

## Extraction

Go files use standard-library AST syntax only. The extractor records package,
imports, functions, methods and receivers, structs/interfaces/named types,
const/var declarations, doc comments, calls, selector expressions, identifier
references, and test/benchmark/example functions. Unresolvable calls remain
lexical relations with `UnresolvedTo` and confidence; a receiver call is never
asserted to target an arbitrary same-named method.

PHP files use a stateful lexer so PHP/HTML transitions, comments, quoted and
heredoc/nowdoc strings, attributes, and malformed tails do not turn braces into
false declaration boundaries. The PHP extractor records namespace/use aliases,
class/interface/trait/enum declarations, functions/methods, properties,
constants, type/attribute references, includes, calls, and PHPUnit test
relations. Local canonical matches receive handles; unresolved names and safe
repository-relative include paths remain lexical relations with confidence.
`.inc` is selected as PHP only when content contains `<?php`, `<?=`, or `<?`;
XML or tagless `.inc` remains text. Composer and PHP are never run.

Smarty `.tpl` and `.smarty` files use a pure-Go stateful scanner. `.tpl` is
classified as `smarty` when a default-delimiter Smarty marker is present, as
`php` when it is PHP-like without a Smarty marker, and otherwise as `template`.
`.smarty` is always classified as `smarty`. The template extractor creates a
template outline plus `template-block`, `template-function`, and
`template-capture` symbols; full named bodies, static markup, style bodies,
data scripts, and embedded PHP are bounded source chunks. JavaScript and
TypeScript bodies delegate to the existing generic structural extractor, then
remap its line/byte spans and handles into a template namespace. Static
`include`/`extends` and external script targets are unresolved imports until a
later repository-aware lookup can prove a unique path. Dynamic targets and
remote URLs are never resolved. Smarty comments and literal blocks are opaque;
malformed input produces diagnostics and bounded recovery chunks.

Double-curly `{{...}}` tags are treated as opaque template tags. They remain
in searchable source and do not create Smarty symbols; quoted `}}` content is
kept inside the tag when its closing boundary is found.

This is not a complete Smarty, HTML, JavaScript, CSS, or PHP semantic parser.
Smarty semantic parsing uses only the default `{}` delimiters; custom
delimiters, plugin semantics, rendering, variable data-flow, and full
DOM/semantic resolution are out of scope. Other template syntaxes can still be
retained as opaque source, including the double-curly form above. Mixed PHP inside a Smarty file remains searchable as an
`embedded-php` bounded region rather than being executed or re-parsed by a
second PHP implementation.

The C/C++, C#, and JavaScript/TypeScript profiles are also first-class
structural extractors, but remain syntax-oriented rather than compiler
frontends. Their stateful lexers ignore strings and comments; the C/C++ and C#
lexers also track inactive preprocessor branches when locating declarations.
Their parsers produce exact
UTF-8-safe byte/line spans and one source chunk per concrete function/method;
class, namespace, module, and type chunks are bounded outlines so a container
does not duplicate every child body. The symbol identity includes normalized
path, language, kind, qualified name, and signature, while chunk identity also
includes its span/content identity.

- C/C++ covers C and common C/C++ header/module extensions, namespaces,
  class/struct/union/enum declarations, free functions, methods,
  constructors/destructors/operators, macros, `#include`/module imports,
  calls, type references, and test functions.
- C# covers namespace blocks and file-scoped namespaces, class/struct/
  interface/enum/record declarations, methods, constructors, properties,
  events, `using` imports, calls, type references, partial source, and test
  methods.
- JavaScript/TypeScript covers `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, and
  `.cjs`, ESM imports/exports, CommonJS `require`/exports, classes,
  interfaces, enums, type aliases, functions, methods, arrow functions,
  JSX/TSX bodies, and nested test suites.

All three profiles add `contains`, `imports`/`exports`, `calls`/`tests`, and
`references` relations. Unique same-file qualified/name matches become
resolved handles. Ambiguous or external targets remain confidence-labeled
`UnresolvedTo` lexical relations; relation expansion can still match those
targets without claiming semantic resolution. Malformed input yields the
valid prefix and a diagnostic instead of discarding the whole file. The
extractors never invoke Clang, Roslyn, the TypeScript Compiler API, a package
manager, or repository code.

Other profiles are intentionally approximate:

- C-like files use a small stateful lexer that ignores braces in strings,
  character literals, line comments, and block comments, then finds bounded
  top-level declarations.
- Python-like files use indentation and top-level/class-child `def`, `class`,
  and function-like declarations.
- Markdown is split at headings.
- All other text uses 80-line windows with 10-line overlap, boundary preference,
  a 160-line maximum, and no mid-line cuts.

Generic symbols have lower confidence and are labeled as approximate in
diagnostics. They do not pretend to provide semantic resolution.

### Language selection and project linking

Language selection has one deterministic precedence order:

1. A matching `language_overrides` entry from `.focalspan.json`.
2. Content-aware handling for ambiguous `.inc` files: PHP markers first,
   then decisive Pawn markers, otherwise text.
3. The normalized file extension and the first matching registered extractor.
4. Generic fallback for Markdown, unknown extensions, and unregistered source.

The registry order is Go AST, PHP, C/C++, C#, JavaScript/TypeScript, Lua,
Pawn, Rust, Python, Ruby, VB6, VB.NET, Nim, Zig, template, XAML, RESX, then
generic. Every extraction has an owner symbol for the source file or
compilation unit. Container declarations generally produce outline chunks;
concrete members produce source chunks, so owner and child bodies are not
stored twice without a structural reason.

The dedicated parsers are intentionally bounded. They recognize lexical
declarations and conservative local relations, but do not run a compiler,
macro expander, type checker, package manager, template renderer, or runtime.
In particular, dynamic Python/JavaScript/Ruby/Lua/PHP loading, C/C++ macro
semantics, C# generated partial code, Rust/Nim/Zig compile-time evaluation,
overload resolution, and virtual or dynamic dispatch remain unresolved.

`internal/projectmeta` reads static project metadata without executing it.
Supported facts include Go module/workspace and local replace/use paths,
Cargo package/workspace/path dependencies, Node package/module/exports/
workspaces and static TS path aliases, .NET project references and item
paths, Composer PSR mappings, Python package paths, literal Ruby and Lua
manifest paths, VB6 components, Nim package/source paths, and Zig package/
path declarations. Invalid or unsupported metadata yields diagnostics and
does not become executable behavior.

`internal/linker` resolves unresolved relations using this precedence:
exact static path, exact qualified symbol, metadata-constrained module plus
exported name, unique owner/scope name, then a simple unique repository-wide
name. A candidate must remain unique after each narrowing step; ambiguity is
left as `UnresolvedTo` with its confidence rather than assigning an arbitrary
`ToHandle`. Rust `crate/self/super` module paths and Python relative module
paths are normalized using the importing file's directory.

Linking runs only after a successful index transaction. It updates existing
relation rows deterministically and uses no schema migration. Relation
resolution never changes source content, and source duplication is avoided by
keeping one concrete span per path/symbol while preserving distinct files
even when their content hashes happen to be equal.

## SQLite schema and transaction model

The embedded migration creates `meta`, `files`, `symbols`, `chunks`,
`relations`, `diagnostics`, `index_runs`, and the FTS5 `chunk_fts` table over
path, symbol name, signature, and content. Foreign keys are enabled; the
connection sets a busy timeout, WAL, and `synchronous=NORMAL`. Read-only
operations use a separate query-only connection where supported.

FTS rows are synchronized explicitly in the same transaction as chunk writes.
An update removes the old file's symbols, chunks, relations, diagnostics, and
FTS rows before inserting the new extraction. A stale file is removed in the
same transaction. Migration runs in a transaction and a failed migration leaves
the prior DB untouched; a corrupt DB error explains the rebuild procedure.

Parse workers only return extraction results. One writer owns the transaction
and records `index_runs`, including seen/added/changed/unchanged/deleted and
parse-failure counts. Cancellation rolls back an incomplete run and never
publishes its revision as successful.

## Retrieval and ranking

The v0.2 query pipeline is deliberately local and deterministic:

```text
raw query
  -> Unicode normalization
  -> deterministic Query Plan
  -> independent retrievers
  -> weighted RRF
  -> intent-aware reranker
  -> existing deduplication
  -> existing token packer
```

`internal/query` performs the single normalization pass and produces lowercase
lexical terms, original-case identifiers, quoted phrases, path fragments,
snake/camel/Pascal splits, namespace and receiver variants, probable symbol
names, and Japanese Unicode runs. The planner preserves code identifiers in
mixed Japanese/code queries and selects ordered intents, anchors, relations,
and a ranking profile. FTS input is built from escaped terms and never receives
the raw query as syntax. Japanese support is intent and lexical normalization;
it is not translation, semantic embedding, or compiler-grade resolution.

Independent retrievers are exact qualified symbol, exact symbol, symbol prefix,
FTS5 BM25, path hints, and one-hop relations. Their caps are 50, 50, 50, 100,
50, and 100 respectively; fusion keeps at most 400 candidates before path and
changed-line filters. Exact symbol lookup does not depend on an FTS anchor.
Relation expansion starts from exact structural anchors and falls back to FTS
anchors only when needed. `fts-only` invokes only FTS; `no-relations` invokes
all base retrievers but skips relation expansion.

Weighted reciprocal-rank fusion combines lists using
`score(candidate) += weight / (60 + rank)`. Fixed profile weights are selected
once per query plan and the resulting contribution is retained for evaluation
and diagnostics.
The final candidate set is then passed to the intent-aware reranker.

The reranker uses named weights: qualified exact 120, symbol exact 100, prefix
70, lexical overlap 0-40, normalized BM25 0-40, path 0-20, changed-line 25,
changed-file 15, test/production pairing 15, resolved relation confidence
times 25, unresolved lexical relation confidence times 12, same package 10,
and a generated/low-density penalty. Relation queries first choose exact
structural symbol/path anchors, preferring concrete functions/methods over
generic text or unrelated same-token matches, then rank the one-hop relation
candidates. Short natural-language words are not treated as symbols merely
because they appear in documentation. Reasons are preserved as
`ScoreReason{Code, Weight, Detail}`. Ties sort by confidence descending, span
size ascending, path, start line, then handle.

Deduplication removes duplicate content hashes within the same path, contained
lower-score spans, duplicate outline/body choices, and excessive same-file
candidates. Equal content in different paths remains distinct so repository
identity is not lost. A raw text hit is not allowed to crowd out a source span
for the same symbol.

Template `imports` relations are expanded for natural-language include,
extends, layout, and partial queries, and receive the same explainable
relation score path as other relation candidates. Template outlines are kept
small so a full template body is not duplicated in every result.

## Token budget and progressive disclosure

`TokenEstimator` is deterministic and conservative: it considers UTF-8 byte
length, ASCII versus non-ASCII text, identifier/punctuation density, metadata,
and a 12% safety margin. It is not `len(text)/4` and does not depend on an
external tokenizer.

The packer reserves header/footer cost, prioritizes exact symbols, allows an
outline/signature when a body does not fit, caps ordinary items at 40% of the
budget, elides oversized symbols to signature plus query-hit windows, cuts only
at UTF-8 and line boundaries, and marks elision. It promotes appropriate
definition/test neighbors and file diversity, limits ordinary test queries to
the most useful test relation candidates, and shortens repeated relation
content to signatures. It re-renders and re-estimates the final payload,
including metadata, dropping the lowest-utility items until the serialized
estimate is at or below the budget. Reported `EstimatedTokens` is the
source-oriented estimate used for reduction comparisons; the hard packing
check includes the full structured output. Budgets are clamped to 256..64000,
with a default of 4000. An extreme budget or source request gracefully
degrades to outline content.

`outline` items contain handle, path, language, kind, symbol, signature, line
range, score, and reasons without source body. `source` includes bounded content.
Relation-expanded items also retain the relation name in structured output.
Relations supported by `expand` are `self`, `parent`, `children`, `callers`,
`callees`, `imports`, `references`, `tests`, and `neighbors`; unsupported
relations return an empty result.

## CLI and MCP

The public CLI has one query form plus `setup`, `update`, `status`, and
`mcp install|status|uninstall`. A question is supplied directly as positional
words; there is no query subcommand or alias. `setup` combines configuration
initialization with the first full index, while `update --rebuild` is the only
explicit rebuild path. `status` combines index statistics and health checks.
`update --if-repo --quiet` returns zero without stdout outside Git. Queries
auto-index unless `--auto-update=false` is set. Evaluation is isolated in the
development-only `focalspan-eval` binary. The internal `serve` entrypoint is
kept out of public help so existing MCP registrations remain runnable.

Codex project scope writes a deterministic marked block to the canonical
root's `.codex/config.toml` after validating both the old and new TOML. The
unmarked portion is preserved byte-for-byte, managed updates are idempotent,
and an unmanaged same-name table is a conflict. Global user scope is the MCP
default; project scope requires `--project`. User scope uses separated
arguments with the installed `codex mcp` CLI; FocalSpan never
edits the user Codex config directly. Project trust and existing Codex session
reload state are intentionally not changed or guessed.

The MCP server binds one startup root and exposes exactly `code_context`,
`code_expand`, `code_impact`, `code_restart`, and `code_status`. Handlers
validate typed input, pass cancellation through the service, and hide internal
stack traces. `code_restart` reloads configuration and atomically swaps the
application service after active calls finish; it does not terminate the MCP
process. Source appears once in canonical structured output; text content is
only a short summary. Logs go to stderr, and concurrent calls serialize DB
writes while allowing safe read transactions.

## Decision log

1. Use `modernc.org/sqlite` because it provides real SQLite/FTS5 without CGO;
   a fake FTS implementation is not acceptable.
2. Use the official MCP Go SDK v1.7.0 and stdio only; HTTP transport is outside
   the MVP.
3. Use syntax-only Go analysis and generic structural extraction so indexing
   never executes repository code or runs a build/package manager.
4. Keep source content in the local index DB and state this clearly in README;
   users can remove `.focalspan/` to rebuild or erase the cache.
5. Use explicit transactions and a single writer because deterministic index
   state is more important than unsafe parallel writes.
6. Keep all scoring deterministic and explainable; no network, embeddings, or
   learned reranker is needed to establish the MVP measurement loop.
7. Treat Smarty templates as composite source: scan them locally, delegate
   embedded JavaScript/TypeScript to the existing generic extractor, and remap
   spans/handles instead of copying a second JavaScript parser. Mixed PHP is a
   bounded searchable region because full PHP delegation would destabilize the
   existing PHP extractor and is not needed for first-class template coverage.
8. Promote C/C++, C#, and JavaScript/TypeScript to dedicated pure-Go
   structural extractors rather than extending `generic`: exact spans and
   language-specific recovery are required for useful relation anchors.
9. Keep container declarations as outline chunks and concrete methods/
   functions as source chunks; this prevents class-level and child-body
   duplication while preserving hierarchy through `contains` relations.
10. Treat unresolved calls, imports/exports/includes, and references as
    confidence-labeled lexical relations. This makes them searchable without
    presenting heuristic resolution as compiler semantics.
11. Keep the planner deterministic and local; Japanese handling is intent and
    lexical normalization rather than automatic translation or semantic search.
12. Let exact symbol retrieval operate independently of FTS, and let relation
    expansion anchor exact structural lookup before FTS fallback.
13. Use weighted RRF to combine ranks rather than comparing raw BM25 and
    heuristic score scales.
14. Keep the existing MCP structured contract and schema version 1; the
    current checkout's existing `code_restart` extension remains in its
    five-tool MCP surface.
15. Keep advanced retrieval operations on the MCP surface; the public CLI
    remains limited to a positional question and repository maintenance.

## Roadmap (design only)

1. SCIP semantic index importer.
2. Official Tree-sitter Go binding adapter.
3. C/C++ semantic provider beyond the syntax extractor.
4. C# semantic provider beyond the syntax extractor.
5. Rust semantic provider.
6. Python/TypeScript semantic provider beyond the syntax extractor.
7. Model-specific tokenizer.
8. MMR or a learned reranker.
9. Streamable HTTP transport.
10. Filesystem watcher.
11. Optional remote embedding provider.

These are extension points, not production stubs in the MVP.

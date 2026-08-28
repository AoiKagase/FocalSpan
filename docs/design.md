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
- `internal/mcpserver`: official SDK server setup, four typed tools, and
  protocol-safe stdio behavior.
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

Query normalization produces lowercase terms, original-case identifiers,
quoted phrases, path fragments, snake/camel/Pascal splits, namespace and
receiver variants, and probable symbol names. FTS input is built from escaped
tokens and never receives the raw query as syntax.

Retrieval stages are exact qualified symbol, exact symbol, prefix/substring,
identifier overlap, FTS5 BM25, path tokens, changed-line overlap, one-hop
relations, and same-file/package proximity. Each stage has an explicit cap and
the final candidate set is capped at 200.

The reranker uses named weights: qualified exact 120, symbol exact 100, prefix
70, lexical overlap 0-40, normalized BM25 0-40, path 0-20, changed-line 25,
changed-file 15, test/production pairing 15, resolved relation confidence
times 25, unresolved lexical relation confidence times 12, same package 10,
and a generated/low-density penalty. Reasons are preserved as
`ScoreReason{Code, Weight, Detail}`. Ties sort by confidence descending, span
size ascending, path, start line, then handle.

Deduplication removes duplicate content hashes, contained lower-score spans,
duplicate outline/body choices, and excessive same-file candidates. A raw text
hit is not allowed to crowd out a source span for the same symbol.

## Token budget and progressive disclosure

`TokenEstimator` is deterministic and conservative: it considers UTF-8 byte
length, ASCII versus non-ASCII text, identifier/punctuation density, metadata,
and a 12% safety margin. It is not `len(text)/4` and does not depend on an
external tokenizer.

The packer reserves header/footer cost, prioritizes exact symbols, allows an
outline/signature when a body does not fit, caps ordinary items at 40% of the
budget, elides oversized symbols to signature plus query-hit windows, cuts only
at UTF-8 and line boundaries, and marks elision. It promotes appropriate
definition/test neighbors and file diversity. It re-renders and re-estimates
the final payload, dropping the lowest-utility items until the estimate is at
or below the budget. Budgets are clamped to 256..64000, with a default of 4000.
An extreme budget or source request gracefully degrades to outline content.

`outline` items contain handle, path, language, kind, symbol, signature, line
range, score, and reasons without source body. `source` includes bounded content.
Relations supported by `expand` are `self`, `parent`, `children`, `callers`,
`callees`, `imports`, `references`, `tests`, and `neighbors`; unsupported
relations return an empty result.

## CLI and MCP

The CLI has `init`, `index`, `update`, `status`, `query`, `expand`, `impact`,
`eval`, `doctor`, and `serve`. `update --if-repo --quiet` returns zero without
stdout outside Git. `query` auto-indexes an absent index unless `--no-update`
is set. All user paths are root-relative or validated under the selected root.

The MCP server binds one startup root and exposes exactly `code_context`,
`code_expand`, `code_impact`, and `code_status`. Handlers validate typed input,
pass cancellation through the service, and hide internal stack traces. Source
appears once in canonical structured output; text content is only a short
summary. Logs go to stderr, and concurrent calls serialize DB writes while
allowing safe read transactions.

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

## Roadmap (design only)

1. SCIP semantic index importer.
2. Official Tree-sitter Go binding adapter.
3. C/C++ semantic provider.
4. C# semantic provider.
5. Rust semantic provider.
6. Python/TypeScript semantic provider.
7. Model-specific tokenizer.
8. MMR or a learned reranker.
9. Streamable HTTP transport.
10. Filesystem watcher.
11. Optional remote embedding provider.

These are extension points, not production stubs in the MVP.

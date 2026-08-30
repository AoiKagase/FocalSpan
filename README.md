# FocalSpan

FocalSpan is a local, deterministic code-context compiler for repositories. It indexes source code into SQLite/FTS5 and exposes compact context through a minimal CLI and MCP server.

It does not use a network service, external LLM, or repository-code execution.

## Build and test

```powershell
go build ./cmd/focalspan
go test ./...
go vet ./...
```

The production binary is CGO-free:

```powershell
$env:CGO_ENABLED = "0"
go build ./cmd/focalspan
```

## Minimal CLI

The public CLI has one query form and four maintenance command groups:

```text
focalspan [query flags] [--] <question...>
focalspan setup
focalspan update
focalspan status
focalspan mcp <install|status|uninstall>
```

Use `focalspan --help`, `focalspan help <command>`, or `<command> --help` for usage.

### Query

No query subcommand or alias is needed. Unquoted words are joined into one question:

```powershell
focalspan where is authentication configured
focalspan --root "C:\Work Spaces\BookStack" --token-budget 1200 where is authentication configured
focalspan --root . --mode outline --changed-only --path internal auth changes
focalspan --root . --json where is authentication configured
```

Query flags:

- `--root PATH`
- `--token-budget N`
- `--format legacy|evidence` (default: `legacy`)
- `--mode source|outline` for legacy; `source|focused|outline` for Evidence
- `--changed-only`
- repeatable `--path PREFIX`
- repeatable `--known-handle HANDLE` with `--format evidence`
- `--auto-update=false`
- `--json`

Use `--` when the question starts with a command word:

```powershell
focalspan -- status
```

### Repository setup and maintenance

`setup` creates the default configuration when absent, updates `.gitignore`, and builds the initial index. It is safe to run again and preserves an existing valid configuration.

```powershell
focalspan setup --root .
focalspan update --root .
focalspan update --root . --rebuild
focalspan status --root .
```

`status` combines repository statistics and health checks. JSON output includes configuration validity, database/FTS5 availability, path access, MCP readiness, index freshness, counts, revision, warnings, and diagnostics.

Hook-friendly update remains available:

```powershell
focalspan update --root . --if-repo --quiet
```

## Codex MCP registration

Global Codex registration is the default:

```powershell
focalspan mcp install
focalspan mcp status
focalspan mcp uninstall
```

Use project-local registration explicitly:

```powershell
focalspan mcp install --project --root .
focalspan mcp status --project --root .
focalspan mcp uninstall --project --root .
```

Preview installation without changing configuration:

```powershell
focalspan mcp install --dry-run --json
```

Use `--auto-update=false` at install time to disable automatic index updates.
The global-only options are `--codex PATH` and `--force`; both are rejected
with `--project` because project registration does not call the Codex CLI.
Global registration stores only `focalspan serve`; each Codex task resolves its
repository from the MCP process working directory. A global
`mcp install --root PATH` remains accepted only as a migration hint: after the
new fixed `focalspan` registration succeeds, FocalSpan removes that root's old
managed registration. The root path is not stored in the new registration.

FocalSpan keeps MCP protocol output isolated on stdout and writes diagnostics
only to stderr. Project-local registrations continue to launch the internal
`serve --root` entrypoint.

The MCP server exposes exactly:

- `code_context`
- `code_expand`
- `code_impact`
- `code_restart`
- `code_status`

`code_context`, `code_expand`, and `code_impact` return the versioned
`focalspan.context.v1` Evidence Packet. Their default mode is `focused`.
`code_status` and `code_restart` keep their existing output contracts.

## LLM Evidence Packet v0.4

Start with `code_context` in focused mode. Source strings and `source`
segments are verbatim indexed code; `synthetic` outlines are generated
navigation aids. Roles and local relations distinguish targets, callers,
tests, declarations, implementations, and dependencies. Follow `next` actions
with `code_expand`, passing stable handles through `known_handles` so evidence
already present in the conversation is not sent again. The short MCP text
content is only a source-free summary and must not be parsed as code.

This compact example illustrates the schema; its numbers are not measured
output:

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

Fidelity values are `verbatim`, `excerpt`, `signature`, and `synthetic`.
Excerpt `source` segments preserve source bytes and line endings. An `omitted`
segment is metadata describing a skipped line range; its marker is never
inserted into source text.

The public CLI remains legacy by default. Preview the same contract through
the sole positional query surface:

```powershell
focalspan --format evidence --mode focused --token-budget 1200 "callers of Service.ValidateToken"
focalspan --format evidence --mode focused --token-budget 1200 --json "callers of Service.ValidateToken"
focalspan --format evidence --known-handle sym_target --json "Service.ValidateToken"
```

Normal MCP Evidence packets exclude ranking scores, score details, and legacy
token-savings diagnostics. The current public CLI intentionally has no
`query`, `expand`, `impact`, or `explain` subcommands; Evidence preview and
debugging use the positional query surface without resurrecting retired
commands.

## Development evaluation

Evaluation is intentionally separated from the public binary:

```powershell
go run ./cmd/focalspan-eval --root . --cases testdata/eval/cases.jsonl --ablation all --json
go run ./cmd/focalspan-eval --root testdata/repos/evidencesample --cases testdata/eval/evidence-cases.jsonl --contract compare --json
```

Ambiguous extensions use content-aware detection. A `.inc` file containing a
PHP tag is indexed as PHP; Pawn markers such as `#include`, `public`,
`plugin_init`, or `register_plugin` select Pawn when their score is decisive;
otherwise the file remains text. Explicit `language_overrides` take precedence
over this content-aware rule.

## Language coverage

The matrix describes the parser boundary, not compiler or runtime
compatibility:

| Tier | Languages / inputs | What is indexed | Important limits |
| --- | --- | --- | --- |
| AST | Go | Standard-library AST packages, declarations, calls, tests, and source spans | No type checking, build execution, or compiler package graph |
| First-class structural | PHP, C/C++, C#, JavaScript/TypeScript, Rust, Python, Ruby, Lua, Pawn/AMX Mod X, VB6, VB.NET, Nim, Zig | Pure-Go lexer/parser declarations, owner symbols, bounded source chunks, and conservative relations | No compiler-grade type inference, overload/virtual dispatch, macro expansion, or runtime import/dispatch resolution |
| Composite structural | Smarty/template, XAML, RESX | Template/resource structure plus bounded embedded or related source regions | Template rendering, DOM semantics, generated-code semantics, and dynamic resource resolution are not executed |
| Metadata-assisted structural | Projects with static `go.mod`/`Cargo.toml`/`package.json`/`.csproj`/`pyproject.toml`/`Gemfile`/`*.rockspec`/`Project.vbp`/`.nimble`/`build.zig.zon` metadata | Read-only manifest facts constrain static file and symbol linking | Only literal, repository-local, unambiguous matches are linked; manifests are never evaluated |
| Generic fallback | Markdown and unregistered text/source | Bounded headings, declarations, or line windows with lower-confidence chunks | Syntax is approximate; dynamic semantics and unresolved language constructs remain lexical text |

All extractors use repository-relative normalized paths and half-open source
spans. `.inc` detection is content-aware: PHP markers win, decisive Pawn
markers select Pawn, and otherwise the file remains text unless an explicit
`language_overrides` entry says otherwise. Rust/Python module paths and the
other static project facts are used only when their resolution is unique;
ambiguous candidates remain unresolved.

The supported structural relations are conservative `contains`, imports or
exports, calls, tests, and references. JavaScript dynamic `require`, Python
dynamic imports, Ruby/Lua runtime loading, PHP dynamic includes, C/C++ macro
expansion, C# generated partial code, Rust macro expansion, Nim/Zig compile-
time evaluation, and runtime dispatch are not inferred.

## Generated state

Repository-local generated state lives under `.focalspan/`. Project MCP registration modifies only the FocalSpan-managed block in `.codex/config.toml`; global registration delegates to the installed Codex CLI.

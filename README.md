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
- `--mode source|outline`
- `--changed-only`
- repeatable `--path PREFIX`
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
focalspan mcp install --root .
focalspan mcp status --root .
focalspan mcp uninstall --root .
```

Use project-local registration explicitly:

```powershell
focalspan mcp install --project --root .
focalspan mcp status --project --root .
focalspan mcp uninstall --project --root .
```

Preview installation without changing configuration:

```powershell
focalspan mcp install --root . --dry-run --json
```

Use `--auto-update=false` at install time to disable automatic index updates.
The global-only options are `--codex PATH` and `--force`; both are rejected
with `--project` because project registration does not call the Codex CLI.

FocalSpan keeps MCP protocol output isolated on stdout and writes diagnostics only to stderr. Existing registrations continue to launch the internal `serve --root` entrypoint.

The MCP server exposes exactly:

- `code_context`
- `code_expand`
- `code_impact`
- `code_restart`
- `code_status`

## Development evaluation

Evaluation is intentionally separated from the public binary:

```powershell
go run ./cmd/focalspan-eval --root . --cases testdata/eval/cases.jsonl --ablation all --json
```

## Generated state

Repository-local generated state lives under `.focalspan/`. Project MCP registration modifies only the FocalSpan-managed block in `.codex/config.toml`; global registration delegates to the installed Codex CLI.

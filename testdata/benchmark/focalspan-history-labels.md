# FocalSpan history benchmark labels

These labels describe source locations that existed at each base revision. Target diffs were used to find candidate maintenance themes, never as automatic required evidence.

| Case | Why the question is realistic | Required evidence rationale | Deliberately excluded target-only evidence |
|---|---|---|---|
| `php-extractor-integration` | Adding a parser requires understanding the existing indexing boundary. | `internal/indexer/indexer.go` owns extraction result ingestion at the base. | The new PHP package and fixture did not exist at the base. |
| `cpp-extractor-registry` | A maintainer must locate construction before registering new extractors. | `internal/app/service.go` constructs the application and registry dependencies. | New C/C++ extractor files were not labeled. |
| `jsts-search-integration` | New language candidates must flow through the existing search entry point. | `internal/search/search.go` owns candidate search. | New JS/TS parser and fixtures were not labeled. |
| `rust-registry-integration` | Rust support must enter the established service construction path. | `internal/app/service.go` is the existing integration point. | Target-only Rust sources were not labeled. |
| `dotnet-structural-registry` | XAML/RESX support needs the pre-existing extractor wiring. | `internal/app/service.go` is required to understand registration. | New XAML, RESX, and fixture files were excluded. |
| `japanese-query-normalization` | Japanese support changes how existing identifiers become search terms. | `internal/search/query.go` owns the base normalization behavior. | New `internal/query` files did not exist at the base. |
| `project-metadata-indexing` | Metadata links must connect to the current indexing lifecycle. | `internal/indexer/indexer.go` is the existing lifecycle boundary. | New linker/projectmeta files were excluded. |
| `mcp-evidence-output` | Changing MCP output requires finding the established handler. | `internal/mcpserver/server.go` owns `codeContext`. | Later Evidence transport edits are target diagnostics only. |

All eight required paths were checked with `git cat-file -e <base>:<path>`. No optional or forbidden labels are needed for this first public corpus. The three expansion labels use distinct `callers` and `references` relations; their anchor path and symbol are present at the base revision. Coverage failures remain measurements rather than reasons to alter production ranking.

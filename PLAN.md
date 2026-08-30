# FocalSpan Polyglot Coverage v0.3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`（推奨）または `superpowers:executing-plans` で、この計画をTask単位に実装してください。各チェックボックスを更新し、Taskごとにテスト・レビュー・コミットを完了してから次へ進みます。

**Goal:** FocalSpanが、ユーザーが日常的に扱うC、C++、C#/.NET WinForms/WPF、PHP（`.inc`・Smarty `.tpl`を含む）、Go、Rust、Python、Node.js系JavaScript/TypeScript、Ruby、Nim、Zig、VB6/VB.NET、Lua、AMX Mod X/Pawnを、単なる全文検索ではなく、質問に必要なsymbol・source span・relationをtoken budget内で返せる第一級または実用的な構造解析プロファイルとして扱えるようにする。

**Architecture:** 現在の`Extractor -> SQLite/FTS5 -> independent retrievers -> weighted RRF -> intent-aware ranker -> token packer`を維持する。言語判定を`internal/language`へ集約し、各言語は独立したpure-Go lexer/parser/extractor packageとして実装する。曖昧なcross-file関係は確定せず`UnresolvedTo`とconfidenceを保持し、最後にread-onlyなproject metadataと安全なrepository linkerで候補を絞る。

**Tech Stack:** Go 1.26+、標準ライブラリ、`database/sql`、SQLite FTS5（`modernc.org/sqlite`）、既存MCP Go SDK。production pathから外部コンパイラ、言語runtime、package manager、language server、ネットワーク、外部LLMを起動しない。

**Spec:** `AGENTS.md`、`docs/design.md`、`docs/evaluation.md`、現行`README.md`、および本計画。

---

## Codexへの実行指示

このファイルを渡されたCodexは、次の順序で作業してください。

1. `AGENTS.md`、`PLAN.md`、`docs/design.md`、`docs/evaluation.md`、`README.md`を読む。
2. `git status --short`、`git diff --stat`、`git log -10 --oneline`を実行する。
3. 現在のcheckoutを唯一のsource of truthとして扱う。
4. Task 0から順番に実装する。言語Taskを飛ばして後段のlinkerへ進まない。
5. 各behaviorについて失敗するtestを先に追加し、失敗を確認してから最小実装を行う。
6. Task完了ごとに、局所テスト、`go test ./...`、該当fixtureの`focalspan eval`を実行する。
7. Task完了ごとに本ファイルのチェックボックスを更新する。
8. Gitへ書き込める場合はTask単位でcommitする。書き込めない場合は差分を保持し、最終報告に理由を書く。
9. 設計確認だけで停止せず、Task 17の全検証と最終報告まで進める。
10. 実行していないテスト、未達の評価値、未実装項目を成功扱いしない。

重大な仕様矛盾、既存データ破壊の危険、またはユーザー変更を失う危険がある場合だけ質問してください。

---

## Global Constraints

- 開始時から存在する未コミット差分を消さない。
- `git reset`、`git restore`、`git checkout --`、`git clean`、`git stash`を実行しない。
- Go 1.26以上を維持する。
- `CGO_ENABLED=0`でWindows amd64、Linux amd64、Darwin arm64へbuildできること。
- production pathからPython、Ruby、Node.js、PHP、.NET、Clang、rustc、nim、zig、Lua、Pawn compilerを起動しない。
- production pathからComposer、npm、pnpm、yarn、pip、cargo、go list、dotnet restore/build、MSBuildを起動しない。
- ネットワークアクセスを追加しない。
- repository内コードを実行しない。
- shell文字列を組み立てず、必要な既存Git呼び出しは引数分離した`exec.CommandContext`を維持する。
- SQLite schema version 1を原則維持する。schema変更が必要なら先に設計文書へ理由、migration、rebuild影響を書く。
- MCP tool名、structured output、stdout protocol-only規則を壊さない。
- CLIの既存command、flag、JSON出力を後方互換に保つ。
- source spanは元ファイル基準のhalf-open byte rangeと1-based line rangeを使う。
- source chunkの`Content`は、synthetic outlineを除き`file.Content[StartByte:EndByte]`と一致すること。
- stable handleへ行番号を直接含めない。
- 曖昧なrelationへ誤った`ToHandle`を設定しない。
- fixture固有のsymbol、path、queryをproduction codeへ埋め込まない。
- 同じ入力、index revision、設定では同じhandle、候補順、出力を返す。
- 最終serialized payloadのtoken budget complianceを100%維持する。
- production codeへ未完成stub、未実装marker、未実装panicを残さない。
- 既存Go、PHP、C/C++、C#、JS/TS、Smarty/templateの評価値を隠すためにcaseやthresholdを削除しない。
- 新しい外部parser dependencyは追加しない。pure-Goの軽量lexer/parserを用いる。
- 解析不能な新構文はfatalにせず、確定済みsymbolを保持し、diagnosticとbounded fallback chunkを返す。
- 大きなclass/module/file全体と全methodを無条件に二重保存しない。

---

## Current Baseline

本計画作成時の公開`master`基準:

- Latest commit: `ec6f86e` (`Fix global MCP install command runner`)
- Retrieval Quality v0.2実装済み。
- 専用Extractor登録済み:
  - Go: `go-ast`
  - PHP: `php-structural`
  - C/C++: `cpp-structural`
  - C#: `csharp-structural`
  - JavaScript/TypeScript: `jsts-structural`
  - Smarty/template: `template-structural`
  - その他: `generic-structural`
- 現在のglobal extractor version: `extractors-v4`
- 現在の専用fixture:
  - `authsample`
  - `phpsample`
  - `templatesample`
  - `cppsample`
  - `csharpsample`
  - `jstssample`

既存の受入下限:

| Profile | hit@5 | Budget | Forbidden | Deterministic | Path recall |
|---|---:|---:|---:|---:|---:|
| Go/auth | 1.00 | 1.00 | 0 | 1.00 | 0.875以上 |
| PHP | 1.00 | 1.00 | 0 | 1.00 | 1.00 |
| Smarty/template | 1.00 | 1.00 | 0 | 1.00 | 1.00 |
| C/C++ | 1.00 | 1.00 | 0 | 1.00 | 1.00 |
| C# | 1.00 | 1.00 | 0 | 1.00 | 1.00 |
| JavaScript/TypeScript | 1.00 | 1.00 | 0 | 1.00 | 1.00 |

既存profileのmedian reductionは原則`<= 0.25`を維持する。現在値より悪化する場合は、caseを削除せず原因を記録して修正する。

---

## Target Support Matrix

本計画完了時の能力目標:

| Profile | 目標 |
|---|---|
| C/C++ | 既存第一級Extractor強化、header/source pairing、compile metadata候補、より安全なpreprocessor/call relation |
| C# | 既存第一級Extractor強化、WinForms/WPF/XAML/partial/designer/resource/project関係 |
| PHP/.inc/.tpl | 既存第一級Extractor強化、曖昧拡張子override、Composer PSR-4、include/trait/template関係 |
| Go | AST抽出強化、partial parse recovery、interface/member/generics、go.mod/go.work package候補 |
| JS/TS/Node.js | 拡張子・ESM/CommonJS・tsconfig/package metadata・Node relative resolution強化 |
| Rust | 新規第一級Extractor |
| Python | 新規第一級Extractor |
| Ruby | 新規第一級Extractor |
| Lua | 新規第一級Extractor |
| AMX Mod X/Pawn | 新規第一級Extractor、`.inc` content-aware判定 |
| VB6 | 新規第一級line-oriented Extractor |
| VB.NET | 新規第一級line-oriented Extractor |
| Nim | 新規第一級indentation-aware Extractor |
| Zig | 新規第一級brace/token Extractor |
| WinForms/WPF | C#/VB.NETコード、Designer、XAML、resource/project metadataを複合的に検索可能 |

新規profileの最低評価基準:

- 5件以上のcase。
- hit@3 `>= 0.80`。
- hit@5 `= 1.00`。
- symbol recall `= 1.00`。
- path recall `= 1.00`。
- budget compliance `= 1.00`。
- forbidden path violations `= 0`。
- deterministic output `= 1.00`。
- median reduction ratio `<= 0.25`。
- 全resultに実在pathとvalid line rangeがある。
- malformed sourceでもpanicしない。

---

## Target File Map

### Create

```text
internal/language/
    detect.go
    profiles.go
    override.go
    detect_test.go
    override_test.go

internal/extract/testutil/
    conformance.go
    fixtures.go

internal/extract/rust/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go
    lexer_test.go

internal/extract/python/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go
    lexer_test.go

internal/extract/ruby/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go

internal/extract/lua/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go

internal/extract/pawn/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go

internal/extract/vb/
    common.go
    vb6.go
    vbnet.go
    builder.go
    extractor_test.go

internal/extract/nim/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go

internal/extract/zig/
    extractor.go
    lexer.go
    parser.go
    builder.go
    extractor_test.go

internal/extract/xaml/
    extractor.go
    scanner.go
    builder.go
    extractor_test.go

internal/extract/resx/
    extractor.go
    scanner.go
    builder.go
    extractor_test.go

internal/projectmeta/
    model.go
    discover.go
    go.go
    cargo.go
    node.go
    dotnet.go
    composer.go
    python.go
    ruby.go
    lua.go
    vb.go
    pawn.go
    nim.go
    zig.go
    projectmeta_test.go

internal/linker/
    linker.go
    paths.go
    symbols.go
    linker_test.go

testdata/repos/rustsample/
testdata/repos/pythonsample/
testdata/repos/rubysample/
testdata/repos/luasample/
testdata/repos/pawnsample/
testdata/repos/vb6sample/
testdata/repos/vbnetsample/
testdata/repos/nimsample/
testdata/repos/zigsample/
testdata/repos/dotnetsample/

testdata/eval/rust-cases.jsonl
testdata/eval/python-cases.jsonl
testdata/eval/ruby-cases.jsonl
testdata/eval/lua-cases.jsonl
testdata/eval/pawn-cases.jsonl
testdata/eval/vb6-cases.jsonl
testdata/eval/vbnet-cases.jsonl
testdata/eval/nim-cases.jsonl
testdata/eval/zig-cases.jsonl
testdata/eval/dotnet-cases.jsonl
```

### Modify

```text
internal/config/config.go
internal/config/config_test.go
internal/repository/scanner.go
internal/repository/scanner_test.go
internal/app/service.go
internal/app/service_test.go
internal/indexer/indexer.go
internal/indexer/indexer_test.go
internal/store/store.go
internal/store/store_test.go

internal/extract/sourceutil/*
internal/extract/goast/*
internal/extract/cpp/*
internal/extract/csharp/*
internal/extract/php/*
internal/extract/jsts/*
internal/extract/template/*
internal/extract/generic/*

internal/search/*
internal/rank/*
internal/eval/*
README.md
docs/design.md
docs/evaluation.md
docs/implementation-plan.md
PLAN.md
```

既存checkoutに同等packageやhelperが追加されている場合は重複作成せず、現在の境界を使う。

---

## Shared Extraction Contract

全専用Extractorは次を満たす。

```go
type Extractor interface {
    Name() string
    Supports(path string, language string) bool
    Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error)
}
```

各ファイルにowner symbolを一つ作る。推奨kind:

```text
C/C++: translation_unit
C#: compilation_unit
Go: package
PHP: php_file
JS/TS: module
Rust: crate_module
Python: module
Ruby: ruby_file
Lua: lua_module
Pawn: pawn_unit
VB6: vb6_component
VB.NET: vbnet_compilation_unit
Nim: nim_module
Zig: zig_module
XAML: xaml_document
```

全Extractorの共通invariant:

```text
symbol/chunk handleは空でない
handleはExtraction内で一意
StartByte <= EndByte
0 <= StartByte <= len(content)
0 <= EndByte <= len(content)
1 <= StartLine <= EndLine
source chunkは元source sliceと一致
synthetic outlineはStartByte=0/EndByte=0と明示signatureを持つ
ParentHandleは同じExtraction内のsymbolを指す
relation.FromHandleは同じExtraction内のsymbolを指す
ToHandleを設定する場合は同じExtraction内で一意に解決済み
ToHandleとUnresolvedToを同時に設定しない
relation confidenceは0..1
diagnosticへsource全文を入れない
同一relationを重複生成しない
結果順は決定的
context cancellationを伝播
```

---

### Task 0: Protect the Current Merge and Capture a New Baseline

**Files:**
- Modify: `docs/evaluation.md`
- Modify: `PLAN.md`

**Interfaces:**
- Consumes: current registry, all existing fixtures, current `extractorVersion`
- Produces: immutable pre-v0.3 baseline and worktree record

- [x] **Step 1: Record the starting checkout**

Run:

```text
git status --short
git diff --stat
git log -10 --oneline
```

最終報告用に保存する。差分をreset、restore、stash、cleanしない。

- [x] **Step 2: Run baseline tests**

```text
gofmt -w .
go test ./...
go vet ./...
```

`gofmt -w .`が開始時差分を大量変更する場合は停止せず、まず`gofmt -l .`で対象を確認し、今回変更するGo fileだけをformatする。

- [x] **Step 3: Build one temporary binary**

Windows PowerShell:

```text
go build -o .focalspan-v03-baseline.exe ./cmd/focalspan
```

Unix:

```text
go build -o .focalspan-v03-baseline ./cmd/focalspan
```

- [x] **Step 4: Run every checked-in evaluation**

既存case setを列挙し、対応するfixture rootで`index`直後に`eval --json`を実行する。最低限:

```text
authsample / cases.jsonl
phpsample / php-cases.jsonl
templatesample / template-cases.jsonl
cppsample / cpp-cases.jsonl
csharpsample / csharp-cases.jsonl
jstssample / jsts-cases.jsonl
authsample / ja-auth-cases.jsonl --ablation all
jstssample / ja-jsts-cases.jsonl --ablation all
```

- [x] **Step 5: Record the exact baseline**

`docs/evaluation.md`へ`Polyglot Coverage v0.3 pre-change baseline`を追加する。実行結果のみ記録し、値を推測しない。

- [x] **Step 6: Remove temporary binaries**

```text
rm -f .focalspan-v03-baseline .focalspan-v03-baseline.exe
```

PowerShellでは`Remove-Item -ErrorAction SilentlyContinue`を使う。

- [x] **Step 7: Commit**

```text
git add docs/evaluation.md PLAN.md
git commit -m "test: capture polyglot coverage baseline"
```

---

### Task 1: Centralize Language Detection and Add Explicit Overrides

**Files:**
- Create: `internal/language/profiles.go`
- Create: `internal/language/detect.go`
- Create: `internal/language/override.go`
- Create: `internal/language/detect_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/repository/scanner.go`
- Modify: `internal/repository/scanner_test.go`

**Interfaces:**
- Produces:

```go
type Detection struct {
    Language   string
    Reason     string
    Confidence float64
}

func Detect(path string, content []byte, overrides map[string]string) Detection
func IsKnown(language string) bool
func KnownLanguages() []string
```

- Config追加:

```go
LanguageOverrides map[string]string `json:"language_overrides"`
```

- [x] **Step 1: Add failing detection tests**

最低限:

```text
main.go                 -> go
main.rs                 -> rust
tool.py                 -> python
tool.pyw                -> python
types.pyi               -> python
script.rb               -> ruby
Rakefile                -> ruby
tool.nim                -> nim
build.nims              -> nim
package.nimble          -> nim
main.zig                -> zig
build.zig               -> zig
Form1.frm               -> vb6
Module1.bas             -> vb6
Class1.cls              -> vb6
App.vbp                 -> vb6-project
Module1.vb              -> vbnet
View.xaml               -> xaml
View.xaml.cs            -> csharp
View.xaml.vb            -> vbnet
Form1.resx              -> dotnet-resource
Settings.settings       -> dotnet-resource
main.lua                -> lua
plugin.sma              -> pawn
plugin.pwn              -> pawn
page.tpl + Smarty       -> smarty
page.tpl + HTML only    -> template
common.inc + PHP tag    -> php
common.inc + Pawn syntax -> pawn
plain.inc               -> text
main.mts                -> typescript
main.cts                -> typescript
types.d.ts              -> typescript
```

- [x] **Step 2: Add override tests**

設定例:

```json
{
  "language_overrides": {
    "**/*.inc": "pawn",
    "legacy/templates/**/*.tpl": "smarty",
    "scripts/**/*.bas": "vb6"
  }
}
```

Precedence:

```text
explicit language override
> content-aware ambiguous extension detection
> exact extension/profile detection
> text fallback
```

無効language、NUL、root外path、無効globは`Config.Validate`でerrorにする。mapの順序に依存せず、最も具体的なmatchを選ぶ。同じspecificityならkey辞書順で決定的に選ぶ。

- [x] **Step 3: Implement known profiles**

最低限のextension:

```text
go: .go
c: .c
cpp: .cc .cpp .cxx .c++ .h .hh .hpp .hxx .inl .ipp .tpp .ixx .cppm
csharp: .cs .csx
javascript: .js .jsx .mjs .cjs
typescript: .ts .tsx .mts .cts .d.ts .d.mts .d.cts
php: .php .phtml .php3 .php4 .php5 .php7 .php8 .phps
rust: .rs
python: .py .pyw .pyi
ruby: .rb .rake .gemspec and basename Gemfile/Rakefile
nim: .nim .nims .nimble
zig: .zig
vb6: .bas .cls .frm .ctl .pag
vb6-project: .vbp
vbnet: .vb
xaml: .xaml
dotnet-resource: .resx .settings
lua: .lua .rockspec
pawn: .sma .pwn
markdown: .md .markdown
config: existing config extensions
```

複合拡張子は`filepath.Ext`一回だけで判定しない。lowercase basename suffixで判定する。

- [x] **Step 4: Implement `.inc` scoring**

PHP:

```text
<?php
<?=
short PHP tag except <?xml
```

Pawn score:

```text
# include / #include
public
stock
native
forward
plugin_init
plugin_precache
register_plugin
new
enum
```

PHP markerがあればPHPを優先する。Pawn scoreが閾値以上ならPawn。それ以外はtext。単語がcomment/string内だけにある場合はscoreへ含めない軽量scanを行う。

- [x] **Step 5: Migrate scanner**

`repository.DetectLanguage`と`DetectLanguageContent`は互換wrapperにするか削除し、実処理を`language.Detect`へ一本化する。同じロジックを二か所に残さない。

- [x] **Step 6: Verify**

```text
go test ./internal/language ./internal/config ./internal/repository -v
go test ./...
```

- [x] **Step 7: Commit**

```text
git add internal/language internal/config internal/repository
git commit -m "feat: centralize polyglot language detection"
```

---

### Task 2: Add Cross-Extractor Conformance and Recovery Infrastructure

**Files:**
- Create: `internal/extract/testutil/conformance.go`
- Create: `internal/extract/testutil/fixtures.go`
- Modify: `internal/extract/sourceutil/*`
- Modify: `internal/app/service_test.go`

**Interfaces:**
- Produces test helpers:

```go
func AssertExtraction(t *testing.T, file model.SourceFile, got model.Extraction)
func AssertNoSourceDuplication(t *testing.T, file model.SourceFile, got model.Extraction, maxRatio float64)
func AssertDeterministic(t *testing.T, extractor extract.Extractor, file model.SourceFile)
```

- [x] **Step 1: Write conformance tests against existing extractors**

Go、PHP、C/C++、C#、JS/TS、templateで共通invariantを検証する。

- [x] **Step 2: Add interval helpers only where missing**

`sourceutil`へ既存実装と重複しない範囲で追加:

```go
func Merge(spans []Span) []Span
func Subtract(whole Span, covered []Span) []Span
func WindowByLines(source SourceMap, span Span, lines, overlap int) []Span
func ValidUTF8Boundary(content []byte, offset int) bool
```

- [x] **Step 3: Add registry selection test for all target languages**

期待するExtractor名はTask完了に応じて更新する。Task 2時点で未実装言語はgeneric、既存言語は専用名を期待する。後続Taskで専用名へ変更する。

- [x] **Step 4: Add fuzz invariant seeds**

少なくともC++ raw string、C# interpolated raw string、JS template literal、PHP heredoc、Smarty literalをseedにする。新規Extractorは各Taskでseedを追加する。

- [x] **Step 5: Verify**

```text
go test ./internal/extract/... -v
go test ./...
```

- [x] **Step 6: Commit**

```text
git add internal/extract internal/app/service_test.go
git commit -m "test: add extractor conformance infrastructure"
```

---

### Task 3: Strengthen Go AST Extraction

**Files:**
- Modify: `internal/extract/goast/go.go`
- Create or modify: `internal/extract/goast/go_test.go`
- Modify: `testdata/repos/authsample`
- Modify: `testdata/eval/cases.jsonl`

**Interfaces:**
- Retains `go-ast`
- Adds symbols/relations without public schema change

- [x] **Step 1: Add failing Go tests**

Cover:

```text
partial AST returned with parser error
type alias
generic type parameters
generic function
interface methods
embedded interface
struct fields
embedded struct field
const/var multiple names
method receiver normalization for pointer/generic receiver
function literal retained in parent chunk
go:build tags do not corrupt spans
import alias
dot import remains unresolved
Test/Benchmark/Fuzz/Example recognition
same-package call
selector call remains conservative
```

- [x] **Step 2: Preserve partial parse results**

`parser.ParseFile`がASTとerrorを同時に返した場合、ASTを捨てない。確定済みsymbol/chunk/relationを返し、`go_parse_partial` diagnosticを追加する。ASTがnilの場合だけfallbackまたはerror。

- [x] **Step 3: Add member symbols**

Interface method、struct field、embedded fieldを独立symbolまたは短いoutline memberとして保持する。大量のanonymous field occurrenceを作らない。

Kinds:

```text
interface_method
field
embedded_field
type_alias
```

- [x] **Step 4: Improve signatures and handles**

Generics、receiver、parameter、return typeを正規化signatureへ含める。同名overloadはGoにはないが、method ownerをqualified nameへ必ず含める。

- [x] **Step 5: Improve relations**

```text
contains
method_of
imports
calls
tests
references
```

Interface embedding、field type、parameter/return typeを`references`として保持する。selector receiver型を解決したふりをしない。

- [x] **Step 6: Add fixture cases**

Go fixtureへinterface、generics、Fuzz test、parse-recoverable fileを追加。既存caseを削除しない。

- [x] **Step 7: Verify**

```text
go test ./internal/extract/goast -v
go test ./...
focalspan index --root testdata/repos/authsample --quiet
focalspan eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json
```

- [x] **Step 8: Commit**

```text
git add internal/extract/goast testdata/repos/authsample testdata/eval/cases.jsonl
git commit -m "feat: strengthen Go structural extraction"
```

---

### Task 4: Strengthen C and C++ Extraction

**Files:**
- Modify: `internal/extract/cpp/*`
- Modify: `testdata/repos/cppsample`
- Modify: `testdata/eval/cpp-cases.jsonl`

**Interfaces:**
- Retains `cpp-structural`
- Adds more precise declaration/definition and callback relations

- [x] **Step 1: Add failing tests**

Cover:

```text
C designated initializer is not a function
function pointer declaration
function pointer typedef
C callback registration
C++ requires-expression
concept
friend function
friend class
lambda with braces
attributes
declspec
nested raw strings
preprocessor line continuation
#if 0 nested
#if defined(...) remains conservative
header declaration and source definition lexical pairing
GoogleTest/Catch2/doctest
Windows callback macros WINAPI/CALLBACK/APIENTRY
```

- [x] **Step 2: Improve parser disambiguation**

Control constructs、casts、initializers、function pointersをfunctionと誤認しない。C/C++ qualifier、trailing return、`noexcept`、`requires`、`= default/delete`をsignatureへ含める。

- [x] **Step 3: Add lexical declaration-definition hints**

同一file内では既存のresolved handleを使用。別fileのheader/source pairは`UnresolvedTo`へ完全qualified nameとnormalized signatureを保持し、`Source=cpp:declaration`または`cpp:definition`とする。ここではcross-file `ToHandle`を確定しない。

- [x] **Step 4: Add callback references**

次のような明示的なfunction pointer引数を低〜中confidenceの`references`として保持:

```text
register_callback(handler)
SetTimer(..., handler)
signal(..., handler)
```

一般関数呼び出しの任意引数をすべてcallback扱いしない。identifierが同一fileの一意なfunction symbolである場合だけ解決する。

- [x] **Step 5: Improve test recognition**

GoogleTest、Catch2、doctestのstatic title/macro spanをtest chunkとして正確に保持する。

- [x] **Step 6: Verify**

```text
go test ./internal/extract/cpp -v
go test ./...
focalspan index --root testdata/repos/cppsample --quiet
focalspan eval --root testdata/repos/cppsample --cases testdata/eval/cpp-cases.jsonl --json
```

- [x] **Step 7: Commit**

```text
git add internal/extract/cpp testdata/repos/cppsample testdata/eval/cpp-cases.jsonl
git commit -m "feat: strengthen C and C++ extraction"
```

---

### Task 5: Strengthen C# and Add .NET WinForms/WPF/XAML Coverage

**Files:**
- Modify: `internal/extract/csharp/*`
- Create: `internal/extract/xaml/*`
- Create: `internal/extract/resx/*`
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Create: `testdata/repos/dotnetsample/*`
- Create: `testdata/eval/dotnet-cases.jsonl`

**Interfaces:**
- Retains `csharp-structural`
- Adds `xaml-structural`
- Adds `resx-structural`

- [x] **Step 1: Add failing C# tests**

Cover:

```text
.csx
file-scoped and block namespace
primary constructor
record class/struct
partial method
extension method
local function
async iterator
required/init property
event/add/remove
indexer
operator/conversion
attributes
nameof
raw/interpolated raw strings
WinForms InitializeComponent
event assignment += Handler
WPF code-behind partial class
xUnit/NUnit/MSTest
```

- [x] **Step 2: Add XAML scanner tests**

Cover:

```text
x:Class
x:Name / Name
event attributes Click="OnClick"
Binding Path
x:Bind
StaticResource
DynamicResource
ResourceDictionary Source
MergedDictionaries
DataContext
UserControl/Window/Page
XML namespaces
CDATA
comments
UTF-8/CRLF
malformed tag recovery
```

- [x] **Step 3: Implement XAML symbols**

Kinds:

```text
xaml_document
xaml_element
xaml_resource
xaml_named_element
```

全elementをsymbol化しない。owner、root element、`x:Name`/`Name`、resource key、template/control、event-bearing elementを優先する。

Relations:

```text
xaml document -> x:Class          references
event element -> handler name     references
binding -> property path          references
resource use -> resource key      references
dictionary -> Source              imports
```

`ToHandle`は同一file内のresourceだけを確定し、code-behindや別resourceは`UnresolvedTo`にする。

- [x] **Step 4: Add RESX and .settings structural extraction**

`resx-structural`はXMLを実行せず、次を抽出する。

```text
resx_document owner
data/name resource key
metadata/name
ResXFileRef path
type/mimetype references
.settings Setting name/type/scope
```

Relations:

```text
resource file -> ResXFileRef target imports
resource key -> type references
settings document -> generated setting name references
```

Base64/blob本文はchunkへ複製しない。長いbinary-like valueはsignatureと短いpreviewだけを保持する。`.resources`と`.frx`はbinaryとしてskipする。

- [x] **Step 5: Improve C# WinForms relations**

Recognize:

```csharp
button.Click += OnButtonClick;
this.Load += Form_Load;
new Button();
Controls.Add(button);
resources.ApplyResources(...);
```

Handlerが同一partial declaration内で一意なら`references`または`calls`として解決する。Designer code全体とhandler sourceを重複返却しない。

- [x] **Step 6: Add dotnet fixture**

Minimum:

```text
DotNetSample.csproj
Views/MainWindow.xaml
Views/MainWindow.xaml.cs
Forms/MainForm.cs
Forms/MainForm.Designer.cs
Forms/MainForm.resx
ViewModels/MainViewModel.cs
Resources/Colors.xaml
Tests/MainViewModelTests.cs
unrelated/ReportWindow.xaml
unrelated/ReportService.cs
```

- [x] **Step 7: Add evaluation cases**

Queries:

```text
where is the WPF save button click handler?
which ViewModel property is bound to UserName?
which resource dictionary defines PrimaryBrush?
where is WinForms InitializeComponent defined?
what method handles the WinForms Load event?
what tests cover MainViewModel validation?
```

- [x] **Step 8: Verify**

```text
go test ./internal/extract/csharp ./internal/extract/xaml ./internal/extract/resx -v
go test ./...
focalspan index --root testdata/repos/dotnetsample --quiet
focalspan eval --root testdata/repos/dotnetsample --cases testdata/eval/dotnet-cases.jsonl --json
```

- [x] **Step 9: Commit**

```text
git add internal/extract/csharp internal/extract/xaml internal/extract/resx internal/app testdata/repos/dotnetsample testdata/eval/dotnet-cases.jsonl
git commit -m "feat: add WinForms and WPF structural coverage"
```

---

### Task 6: Strengthen PHP, `.inc`, and Smarty `.tpl`

**Files:**
- Modify: `internal/extract/php/*`
- Modify: `internal/extract/template/*`
- Modify: `testdata/repos/phpsample`
- Modify: `testdata/repos/templatesample`
- Modify: `testdata/eval/php-cases.jsonl`
- Modify: `testdata/eval/template-cases.jsonl`

- [x] **Step 1: Add failing PHP tests**

Cover:

```text
attributes
readonly class
enum cases
trait use with adaptations
anonymous class remains in parent chunk
closure/arrow function retained
namespace block and semicolon form
grouped use
use function/use const
include/require expression
PHPDoc type hints as low-confidence references
PHPUnit attributes
Pest test(...)
malformed heredoc recovery
```

- [x] **Step 2: Strengthen `.inc` behavior**

`language_overrides`を優先し、PHP marker付き`.inc`はPHP、Pawn score付き`.inc`はPawn、曖昧なものはtext。READMEへ規則を書く。

- [x] **Step 3: Improve PHP relations**

Trait adaptation、base/interface、attribute、parameter/return/property type、static include pathを保守的に保持。Container、magic method、dynamic includeを確定しない。

- [x] **Step 4: Add failing Smarty tests**

Cover:

```text
nested blocks
function/call
capture
extends/include relative paths
multiple script blocks
embedded TS
embedded PHP span
literal/verbatim
custom plugin tag as opaque
double-curly opaque tag
malformed close recovery
```

- [x] **Step 5: Reduce template duplication**

Template outline、named block/function、script/style、unclaimed fragmentのcoverageを測り、同一sourceの過剰な三重複を防ぐ。

- [x] **Step 6: Verify**

```text
go test ./internal/extract/php ./internal/extract/template -v
go test ./...
focalspan index --root testdata/repos/phpsample --quiet
focalspan eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json
focalspan index --root testdata/repos/templatesample --quiet
focalspan eval --root testdata/repos/templatesample --cases testdata/eval/template-cases.jsonl --json
```

- [x] **Step 7: Commit**

```text
git add internal/extract/php internal/extract/template testdata/repos/phpsample testdata/repos/templatesample testdata/eval/php-cases.jsonl testdata/eval/template-cases.jsonl
git commit -m "feat: strengthen PHP and Smarty extraction"
```

---

### Task 7: Strengthen JavaScript, TypeScript, and Node.js Coverage

**Files:**
- Modify: `internal/extract/jsts/*`
- Modify: `testdata/repos/jstssample`
- Modify: `testdata/eval/jsts-cases.jsonl`

- [x] **Step 1: Add extension tests**

Ensure:

```text
.mts .cts .d.ts .d.mts .d.cts
```

are classified as TypeScript. `.json` remains config/data, not JavaScript.

- [x] **Step 2: Add failing parser tests**

Cover:

```text
type-only import/export
export assignment
namespace import
decorators
accessor keyword
static block
private field/method
satisfies
as const
overload signatures
ambient declaration
JSX fragments
React component arrow
CommonJS destructuring require
module.exports object
dynamic import static literal
top-level await
regex vs division
nested template interpolation
```

- [x] **Step 3: Improve module relations**

Static relative ESM/CommonJS specifierをnormalizeし、拡張子候補を決定的に生成する。`node_modules`、package exports、runtime条件はこのTaskでは確定しない。

Candidate order for TS importer:

```text
explicit path
.ts .tsx .mts .cts
.js .jsx .mjs .cjs
/index.ts /index.tsx /index.js /index.jsx
```

JS importerではJS候補を先にする。

- [x] **Step 4: Improve tests**

Jest、Vitest、Mocha、Playwright、Denoのstatic test callbackを認識する。`test.each`、`describe.each`を扱う。

- [x] **Step 5: Add Node fixture metadata files**

`package.json`、`tsconfig.json`、ESM/CommonJS混在、re-export、workspace-like directoryをfixtureへ追加する。metadata解析自体はTask 16で行うが、source relationのbaselineを作る。

- [x] **Step 6: Verify**

```text
go test ./internal/extract/jsts -v
go test ./...
focalspan index --root testdata/repos/jstssample --quiet
focalspan eval --root testdata/repos/jstssample --cases testdata/eval/jsts-cases.jsonl --json
```

- [x] **Step 7: Commit**

```text
git add internal/extract/jsts internal/language testdata/repos/jstssample testdata/eval/jsts-cases.jsonl
git commit -m "feat: strengthen Node JavaScript and TypeScript extraction"
```

---

### Task 8: Add First-Class Rust Extraction

**Files:**
- Create: `internal/extract/rust/*`
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Create: `testdata/repos/rustsample/*`
- Create: `testdata/eval/rust-cases.jsonl`

**Interfaces:**
- `Name() == "rust-structural"`
- Supports `language == "rust"` and `.rs`

- [x] **Step 1: Write lexer tests**

Cover:

```text
line comment
nested block comment
normal/byte/C strings
raw strings r"", r#"..."#, br##""##
lifetime vs character literal
attribute #[...]
inner attribute #![...]
macro token tree
generic angle brackets
async/unsafe/const/extern
malformed raw string
cancellation
```

- [x] **Step 2: Write declaration tests**

Required symbols:

```text
mod
use
struct
enum
union
trait
impl
free function
impl method
trait method
associated const
associated type
type alias
const
static
macro_rules
extern block/function
test
```

Qualified examples:

```text
crate::auth::TokenService
crate::auth::TokenService::validate_token
crate::auth::TokenValidator::validate
```

- [x] **Step 3: Implement hierarchy/chunks**

Owner `crate_module`; class-like outline for struct/enum/trait/impl; method/function body as independent chunk; module-level unclaimed code as bounded fragment.

- [x] **Step 4: Implement relations**

```text
mod/use            imports
trait impl         references
base/type paths    references
calls              calls
#[test]            test
test body call     tests
contains hierarchy contains
```

`self::`、`super::`、`crate::` pathは字句的にnormalizeする。同一fileで一意なfunction/methodだけresolveする。macro expansionやtrait dispatchは確定しない。

- [x] **Step 5: Add Rust fixture**

Minimum:

```text
Cargo.toml
src/lib.rs
src/auth/mod.rs
src/auth/token_service.rs
src/http/middleware.rs
tests/token_service_test.rs
unrelated/report.rs
```

Include traits、impl、async fn、generic、macro、nested comment、raw string、`#[tokio::test]`。

- [x] **Step 6: Add evaluation cases**

```text
where is an expired Rust token rejected?
what calls TokenService validate_token?
what tests cover expired Rust tokens?
which module imports token_service?
which trait does TokenService implement?
```

- [x] **Step 7: Register extractor and remove Rust from generic dispatch**

- [x] **Step 8: Verify**

```text
go test ./internal/extract/rust -v
go test ./...
focalspan index --root testdata/repos/rustsample --quiet
focalspan eval --root testdata/repos/rustsample --cases testdata/eval/rust-cases.jsonl --json
```

- [x] **Step 9: Commit**

```text
git add internal/extract/rust internal/app internal/extract/generic testdata/repos/rustsample testdata/eval/rust-cases.jsonl
git commit -m "feat: add first-class Rust extraction"
```

---

### Task 9: Add First-Class Python Extraction

**Files:**
- Create: `internal/extract/python/*`
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Create: `testdata/repos/pythonsample/*`
- Create: `testdata/eval/python-cases.jsonl`

**Interfaces:**
- `Name() == "python-structural"`
- Supports `.py`, `.pyw`, `.pyi`

- [x] **Step 1: Write lexer tests**

Cover:

```text
indent/dedent
tabs policy without executing Python
single/double/triple strings
raw/byte/f-string prefixes
f-string interpolation braces
line continuation
parenthesized multiline statement
comments
decorators
type comments
malformed triple string
cancellation
```

- [x] **Step 2: Write declaration tests**

Symbols:

```text
module
class
function
async function
method
async method
nested function
property
class variable outline
type alias
protocol/interface-like class
test
```

`@property`、`@classmethod`、`@staticmethod`をsignature/kindへ反映する。lambdaを独立symbolにしない。

- [x] **Step 3: Implement imports and calls**

```text
import x
import x as y
from x import y as z
relative import
function call
self.method
cls.method
Class.method
constructor call
```

同一scope・同一classで一意なtargetだけresolve。monkey patch、dynamic import、decorator semanticsは未解決。

- [x] **Step 4: Implement test recognition**

```text
pytest test_*
unittest.TestCase methods
pytest.mark.parametrize
async tests
fixtures as fixture kind
```

Test body callから`tests` relationを作る。

- [x] **Step 5: Add fixture/evaluation**

Minimum:

```text
pyproject.toml
src/auth/token_service.py
src/http/middleware.py
tests/test_token_service.py
types/token_service.pyi
unrelated/report.py
```

Queriesはdefinition、callers、tests、imports、protocol implementation。

- [x] **Step 6: Register extractor and remove Python from generic indentation dispatch**

- [x] **Step 7: Verify**

```text
go test ./internal/extract/python -v
go test ./...
focalspan index --root testdata/repos/pythonsample --quiet
focalspan eval --root testdata/repos/pythonsample --cases testdata/eval/python-cases.jsonl --json
```

- [x] **Step 8: Commit**

```text
git add internal/extract/python internal/app internal/extract/generic testdata/repos/pythonsample testdata/eval/python-cases.jsonl
git commit -m "feat: add first-class Python extraction"
```

---

### Task 10: Add First-Class Ruby Extraction

**Files:**
- Create: `internal/extract/ruby/*`
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Create: `testdata/repos/rubysample/*`
- Create: `testdata/eval/ruby-cases.jsonl`

- [x] **Step 1: Write lexer tests**

Cover Ruby strings、interpolation、symbols、regex、percent literals、heredoc、comments、`=begin/=end`、`do/end` nesting、modifier `if/unless`。

- [x] **Step 2: Write declaration tests**

```text
module
class
singleton class
def
self.method
define_method with static symbol
attr_reader/writer/accessor
constant
alias
test
```

Blocks are not all symbols. Named RSpec examples can be test symbols.

- [x] **Step 3: Implement relations**

```text
require/require_relative imports
include/extend/prepend references
inheritance references
same-class call calls
RSpec/Minitest tests
contains
```

Dynamic metaprogramming is unresolved.

- [x] **Step 4: Add fixture/eval**

Include Gemfile、gemspec、service、middleware、RSpec、Minitest、unrelated file。

- [x] **Step 5: Register and remove Ruby from generic indentation dispatch**

- [x] **Step 6: Verify**

```text
go test ./internal/extract/ruby -v
go test ./...
focalspan index --root testdata/repos/rubysample --quiet
focalspan eval --root testdata/repos/rubysample --cases testdata/eval/ruby-cases.jsonl --json
```

- [x] **Step 7: Commit**

```text
git add internal/extract/ruby internal/app internal/extract/generic testdata/repos/rubysample testdata/eval/ruby-cases.jsonl
git commit -m "feat: add first-class Ruby extraction"
```

---

### Task 11: Add First-Class Lua Extraction

**Files:**
- Create: `internal/extract/lua/*`
- Modify: `internal/app/service.go`
- Create: `testdata/repos/luasample/*`
- Create: `testdata/eval/lua-cases.jsonl`

- [x] **Step 1: Lexer tests**

Cover:

```text
-- comment
--[[ long comment ]]
--[=[ nested delimiter ]=]
short strings
long bracket strings
escape
function/end nesting
table constructors
malformed long string
```

- [x] **Step 2: Declarations**

```text
local function name
function name
function table.name
function table:method
name = function
local name = function
module/table owner
test
```

- [x] **Step 3: Relations**

```text
require("module") imports
same-table method calls
local function calls
setmetatable/type-like references as low confidence
busted describe/it tests
contains
```

Dynamic table indexing stays unresolved.

- [x] **Step 4: Fixture/eval**

Include service、middleware、busted tests、rockspec、unrelated report。

- [x] **Step 5: Verify and commit**

```text
go test ./internal/extract/lua -v
go test ./...
focalspan index --root testdata/repos/luasample --quiet
focalspan eval --root testdata/repos/luasample --cases testdata/eval/lua-cases.jsonl --json
git add internal/extract/lua internal/app testdata/repos/luasample testdata/eval/lua-cases.jsonl
git commit -m "feat: add first-class Lua extraction"
```

---

### Task 12: Add First-Class AMX Mod X / Pawn Extraction

**Files:**
- Create: `internal/extract/pawn/*`
- Modify: `internal/app/service.go`
- Modify: `internal/language/*`
- Create: `testdata/repos/pawnsample/*`
- Create: `testdata/eval/pawn-cases.jsonl`

- [x] **Step 1: Lexer tests**

Cover:

```text
// and /* */
preprocessor directives
line continuation
strings/chars
tagged types
array dimensions
enum
new/const/static
public/stock/native/forward
malformed directive
```

- [x] **Step 2: Declaration tests**

Kinds:

```text
function
public
stock
native
forward
enum
constant
global
callback
```

Recognize AMXX lifecycle callbacks:

```text
plugin_init
plugin_precache
plugin_cfg
client_connect
client_disconnect
client_putinserver
```

These are functions with callback metadata, not hard-coded ranking boosts.

- [x] **Step 3: Relations**

```text
#include imports
function calls calls
native/forward references
register_clcmd/register_concmd/register_event/register_logevent handler string -> references
set_task handler string -> references
menu handler string -> references
contains
```

String handlerはstatic literalかつ同一fileで一意なfunctionの場合だけresolveする。

- [x] **Step 4: `.inc` conflict tests**

PHP marker優先、Pawn score、explicit override、plain `.inc` fallbackを検証する。

- [x] **Step 5: Fixture/eval**

Minimum:

```text
addons/amxmodx/scripting/auth_plugin.sma
addons/amxmodx/scripting/include/auth.inc
tests or test-like debug harness
unrelated/report_plugin.sma
```

Queries:

```text
where is plugin_init defined?
what handles the say /login command?
which include declares validate_token?
what calls validate_token?
where is the client authorization callback?
```

- [x] **Step 6: Verify and commit**

```text
go test ./internal/extract/pawn ./internal/language -v
go test ./...
focalspan index --root testdata/repos/pawnsample --quiet
focalspan eval --root testdata/repos/pawnsample --cases testdata/eval/pawn-cases.jsonl --json
git add internal/extract/pawn internal/language internal/app testdata/repos/pawnsample testdata/eval/pawn-cases.jsonl
git commit -m "feat: add first-class AMX Mod X Pawn extraction"
```

---

### Task 13: Add First-Class VB6 and VB.NET Extraction

**Files:**
- Create: `internal/extract/vb/*`
- Modify: `internal/app/service.go`
- Create: `testdata/repos/vb6sample/*`
- Create: `testdata/repos/vbnetsample/*`
- Create: `testdata/eval/vb6-cases.jsonl`
- Create: `testdata/eval/vbnet-cases.jsonl`

**Interfaces:**
- Two extractor names:
  - `vb6-structural`
  - `vbnet-structural`

- [x] **Step 1: Shared lexer tests**

Cover:

```text
apostrophe comment
REM comment
string with doubled quote
line continuation _
colon-separated statements
#If/#Else/#End If
case-insensitive keywords
Attribute VB_Name
malformed End block recovery
```

- [x] **Step 2: VB6 declarations**

```text
Form/Class/Module/UserControl owner
Sub
Function
Property Get/Let/Set
Event
Declare Function/Sub
Type
Enum
Const
public/private/friend
Implements
WithEvents
event handler naming
```

`.frm` designer preambleを一つのbounded `form-layout` chunkとして保持し、binary `.frx`はscannerでskipする。

- [x] **Step 3: VB.NET declarations**

```text
Namespace
Class/Module/Structure/Interface/Enum/Delegate
Partial
Sub/Function
Constructor New
Property
Event
Operator
Imports
Inherits
Implements
Handles
AddHandler/RemoveHandler
Async/Iterator
generic Of T
```

- [x] **Step 4: Relations**

VB6:

```text
Project component imports
Implements references
function calls
event handler references
contains
```

VB.NET:

```text
Imports
Inherits/Implements
Handles event
AddHandler handler
calls
tests
contains
```

- [x] **Step 5: Fixtures**

VB6:

```text
Project.vbp
MainForm.frm
AuthService.cls
AuthModule.bas
AuthControl.ctl
unrelated/ReportForm.frm
```

VB.NET:

```text
Project.vbproj
Views/MainWindow.xaml
Views/MainWindow.xaml.vb
Forms/MainForm.vb
Forms/MainForm.Designer.vb
Tests/AuthTests.vb
unrelated/ReportService.vb
```

- [x] **Step 6: Evaluation**

VB6 queries: command/event handler、Property、Implements、project component。  
VB.NET queries: Handles、WPF code-behind、interface、tests、partial designer。

- [x] **Step 7: Verify and commit**

```text
go test ./internal/extract/vb -v
go test ./...
focalspan index --root testdata/repos/vb6sample --quiet
focalspan eval --root testdata/repos/vb6sample --cases testdata/eval/vb6-cases.jsonl --json
focalspan index --root testdata/repos/vbnetsample --quiet
focalspan eval --root testdata/repos/vbnetsample --cases testdata/eval/vbnet-cases.jsonl --json
git add internal/extract/vb internal/app testdata/repos/vb6sample testdata/repos/vbnetsample testdata/eval/vb6-cases.jsonl testdata/eval/vbnet-cases.jsonl
git commit -m "feat: add VB6 and VB.NET structural extraction"
```

---

### Task 14: Add First-Class Nim Extraction

**Files:**
- Create: `internal/extract/nim/*`
- Modify: `internal/app/service.go`
- Create: `testdata/repos/nimsample/*`
- Create: `testdata/eval/nim-cases.jsonl`

- [ ] **Step 1: Lexer/indent tests**

Cover indentation、`#[ ]#` nested comments、triple strings、raw strings、pragmas、backtick identifiers、continuation inside delimiters。

- [ ] **Step 2: Declarations**

```text
module
proc
func
method
iterator
converter
template
macro
type
object
enum
distinct
concept
const
let
var
test
```

- [ ] **Step 3: Relations**

```text
import/include/from imports
type inheritance/reference
same-module calls
method references
unittest suite/test tests
contains
```

Compile-time macro expansionは行わない。

- [ ] **Step 4: Fixture/eval/verify**

```text
go test ./internal/extract/nim -v
go test ./...
focalspan index --root testdata/repos/nimsample --quiet
focalspan eval --root testdata/repos/nimsample --cases testdata/eval/nim-cases.jsonl --json
git add internal/extract/nim internal/app testdata/repos/nimsample testdata/eval/nim-cases.jsonl
git commit -m "feat: add first-class Nim extraction"
```

---

### Task 15: Add First-Class Zig Extraction

**Files:**
- Create: `internal/extract/zig/*`
- Modify: `internal/app/service.go`
- Create: `testdata/repos/zigsample/*`
- Create: `testdata/eval/zig-cases.jsonl`

- [ ] **Step 1: Lexer tests**

Cover line comments、normal strings、multiline string lines beginning `\\`、character literal、builtin `@name`、comptime blocks、error unions、optional types、malformed braces。

- [ ] **Step 2: Declarations**

```text
module
pub fn/fn
const/var
struct
enum
union
opaque
test
comptime declaration
usingnamespace
extern/export function
```

`const Name = struct { ... }`をtype symbolとして認識する。`const f = fn`的な値と区別する。

- [ ] **Step 3: Relations**

```text
@import static literal imports
same-module calls
type references
test body tests
contains
```

Compile-time evaluationは行わない。

- [ ] **Step 4: Fixture/eval/verify**

```text
go test ./internal/extract/zig -v
go test ./...
focalspan index --root testdata/repos/zigsample --quiet
focalspan eval --root testdata/repos/zigsample --cases testdata/eval/zig-cases.jsonl --json
git add internal/extract/zig internal/app testdata/repos/zigsample testdata/eval/zig-cases.jsonl
git commit -m "feat: add first-class Zig extraction"
```

---

### Task 16: Add Read-Only Project Metadata and Conservative Repository Linking

**Files:**
- Create: `internal/projectmeta/*`
- Create: `internal/linker/*`
- Modify: `internal/indexer/indexer.go`
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: fixtures from Tasks 3-15

**Interfaces:**

```go
type Fact struct {
    SourcePath string
    Kind       string
    Name       string
    Target     string
    Confidence float64
}

type Provider interface {
    Supports(path string) bool
    Parse(ctx context.Context, root string, file model.SourceFile) ([]Fact, []model.Diagnostic, error)
}

type Linker struct {
    Store *store.Store
}

func (l *Linker) Link(ctx context.Context, facts []Fact) error
```

実際の型は既存設計へ合わせてよいが、metadata parsingとsymbol linkingを分離する。

- [ ] **Step 1: Add project metadata parser tests**

Read-only parsing only:

```text
go.mod / go.work
Cargo.toml
package.json
tsconfig.json / jsconfig.json
.csproj / .vbproj
composer.json
pyproject.toml / setup.cfg
Gemfile / *.gemspec（static literalだけ）
*.rockspec（static literalだけ）
Project.vbp
.nimble
build.zig.zon
```

XML/TOML/JSONは既存dependencyまたは標準ライブラリでparseする。外部commandを起動しない。

- [ ] **Step 2: Implement minimum metadata facts**

Go:

```text
module path
replace local path
workspace use path
```

Rust:

```text
package name
lib/bin path
workspace members
path dependencies
```

Node:

```text
name
type
main/module/types
exports static entries
workspaces
tsconfig baseUrl/paths static patterns
```

.NET:

```text
RootNamespace
AssemblyName
ProjectReference
Compile Include
Page/ApplicationDefinition
EmbeddedResource/DependentUpon
```

PHP:

```text
PSR-4
PSR-0
classmap
files
```

Python:

```text
project/package name
package-dir
src root
tool-specific static extra paths when safely parseable
```

Ruby:

```text
gem name
require_paths static literals
local path gem entries
```

Lua:

```text
rockspec package name
source directory/static modules
```

VB6:

```text
Form/Class/Module/UserControl entries from .vbp
```

Pawn:

```text
configured include directories only if represented in repository config; do not inspect machine-global AMXX install
```

Nim:

```text
nimble package name
srcDir
static local path dependencies
```

Zig:

```text
build.zig.zon package name
static path dependencies
module root paths that can be read without evaluating build.zig
```

- [ ] **Step 3: Add linker store queries**

Add deterministic indexed lookups for:

```text
file path exact
file path suffix constrained by importer directory
qualified symbol exact
symbol name + owner/module constraint
same partial type
declaration/definition signature
```

Do not choose the first of multiple ambiguous matches.

- [ ] **Step 4: Run linker after file updates**

After `ApplyIndex` succeeds, or inside the same safe index transaction if current architecture permits, resolve only exact/scoped facts. If linking fails, do not mark index run successful without reporting the error.

No schema change is preferred. Existing `relations` rows may be replaced/rebuilt deterministically. If schema change becomes unavoidable, document and test migration before implementation.

- [ ] **Step 5: Resolution precedence**

```text
exact static path
exact qualified symbol
manifest-constrained module + exported name
same owner/scope unique name
simple unique repository-wide name
ambiguous -> unresolved
```

Simple repository-wide uniqueness may resolve only when no scope/module information contradicts it.

- [ ] **Step 6: Add cross-file tests**

Minimum:

```text
Go import -> package file
Rust mod/use -> module
C++ header declaration -> source definition
C# partial class and XAML x:Class -> code-behind
PHP PSR-4 class -> file
JS/TS import alias -> exported symbol
Python relative import -> module
Ruby require_relative -> file
Lua require -> module
Pawn include -> .inc
VB6 project component -> source
VB.NET ProjectReference/XAML -> code
Nim import -> module
Zig @import -> file
```

- [ ] **Step 7: Verify all relation directions**

`expand imports`、`callers`、`callees`、`references`、`tests`でforward/reverse candidateが実用的に返る。

- [ ] **Step 8: Commit**

```text
git add internal/projectmeta internal/linker internal/indexer internal/store testdata
git commit -m "feat: link polyglot project metadata conservatively"
```

---

### Task 17: Final Evaluation, Extractor Version, Documentation, and Release Gate

**Files:**
- Modify: `internal/indexer/indexer.go`
- Modify: `internal/indexer/indexer_test.go`
- Modify: `README.md`
- Modify: `docs/design.md`
- Modify: `docs/evaluation.md`
- Modify: `docs/implementation-plan.md`
- Modify: `PLAN.md`

- [ ] **Step 1: Bump extractor version exactly once**

Current baseline is`extractors-v4`。本計画の全Extractor formatが確定した後に一度だけ新しい値へ更新する。例:

```text
extractors-v5-polyglot
```

現在checkoutですでに別値へ進んでいる場合は巻き戻さず、次の一意な値を使う。

- [ ] **Step 2: Test reindex behavior**

```text
old extractor version + unchanged source -> reparse
new version first update -> all required files refreshed
new version second update -> unchanged
old generic chunks for upgraded languages are removed
schema migration is not required
```

- [ ] **Step 3: Run all unit tests**

```text
gofmt -w <changed Go files/directories>
go test ./...
go vet ./...
```

- [ ] **Step 4: Run race tests where supported**

```text
CGO_ENABLED=1 go test -race ./...
```

Windowsでnative C compilerがない場合は未検証と記録し、成功扱いしない。

- [ ] **Step 5: Run every evaluation**

At minimum:

```text
authsample
phpsample
templatesample
cppsample
csharpsample
jstssample
dotnetsample
rustsample
pythonsample
rubysample
luasample
pawnsample
vb6sample
vbnetsample
nimsample
zigsample
ja-auth
ja-jsts
```

各rootは評価直前にindexする。JSON結果を保存し、`docs/evaluation.md`へ実測値を記載する。

- [ ] **Step 6: Add language matrix to README**

能力を次の階層で正確に表す:

```text
AST
first-class structural
composite structural
metadata-assisted structural
generic fallback
```

「対応」とだけ書かず、型推論、動的dispatch、macro expansion、runtime resolutionの制限を書く。

- [ ] **Step 7: Update design docs**

Document:

```text
language detection precedence
language_overrides
owner symbols
per-language parser boundaries
project metadata facts
link resolution precedence
ambiguity behavior
error recovery
source duplication policy
security/no-execution policy
```

- [ ] **Step 8: Cross-build**

```text
CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/focalspan
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/focalspan
```

Cross-build artifactsをrepository rootへ残さない。

- [ ] **Step 9: CLI/MCP regression**

Verify:

```text
focalspan init
focalspan index
focalspan update
focalspan status --json
focalspan query
focalspan expand
focalspan impact
focalspan eval
focalspan explain
focalspan doctor
focalspan serve
focalspan mcp install/status/uninstall/print
```

MCP:

```text
code_context
code_expand
code_impact
code_restart
code_status
```

stdoutへlogが混入しない。

- [ ] **Step 10: Run self-review**

Check:

```text
all target languages select dedicated extractor
ambiguous .inc obeys override/content rules
no duplicate parser implementation remains in generic
no invalid spans
no duplicate handles
no resolved relation from simple-name ambiguity
no fixture hard-code
no full-file duplication
no production unfinished marker/stub/panic
all docs match implementation
all existing cases retained
```

- [ ] **Step 11: Commit**

```text
git add internal/indexer README.md docs PLAN.md
git commit -m "docs: complete polyglot coverage v0.3"
```

---

## Milestone Gates

### Gate A — Existing First-Class Profiles

Tasks 0-7 complete:

- Existing six profile evaluations pass.
- Go、C/C++、C#/.NET、PHP/Smarty、JS/TS regressions are documented.
- XAML/WinForms/WPF profile has fixture and evaluation.
- Language detection/override behavior is stable.

### Gate B — High-Priority New Languages

Tasks 8-9 complete:

- Rust and Python are no longer generic.
- Each has first-class fixture, relation tests, and acceptance metrics.

### Gate C — Scripting and Legacy Profiles

Tasks 10-13 complete:

- Ruby、Lua、Pawn、VB6、VB.NET select dedicated extractors.
- `.inc` conflict is deterministic and configurable.
- AMXX handler-string relations and VB event relations are tested.

### Gate D — Systems Long-Tail Profiles

Tasks 14-15 complete:

- Nim and Zig are first-class structural profiles.
- Compile-time/runtime semantics are not falsely claimed.

### Gate E — Repository-Aware Linking

Tasks 16-17 complete:

- Static manifest facts improve cross-file retrieval.
- Ambiguity remains explicit.
- All evaluations, tests, cross-builds, docs, and MCP regression checks pass.

---

## Final Acceptance Table

Codex must fill this table with measured values before claiming completion.

| Profile | Cases | hit@1 | hit@3 | hit@5 | Symbol recall | Path recall | Budget | Forbidden | Deterministic | Median reduction |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Go | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| PHP | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Smarty/template | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| C/C++ | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| C# | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| .NET WinForms/WPF | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| JS/TS/Node | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Rust | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Python | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Ruby | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Lua | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Pawn/AMXX | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| VB6 | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| VB.NET | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Nim | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |
| Zig | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED | UNMEASURED |

`UNMEASURED`のまま完了報告しない。未実行profileは`UNVERIFIED`と明記する。

---

## Out of Scope for This Plan

次は別計画へ分離する。

```text
compiler-grade type checking
Clang/Roslyn/rust-analyzer/TypeScript Compiler API invocation
SCIP import
Tree-sitter adapter
embedding/vector database
learned reranker
query-aware control-flow semantic zoom
model-specific tokenizer
HTTP MCP transport
Web UI
filesystem watcher
remote telemetry
runtime package resolution
macro expansion
dependency injection/container resolution
```

---

## Final Report Format

実装完了時は次の順序で報告する。

1. 開始時commitとworktree状態。
2. 実装したTaskと未実装Task。
3. language detectionとoverride規則。
4. 既存言語の強化内容。
5. 新規Extractorごとの対応構文。
6. WinForms/WPF/XAML対応。
7. project metadataとlinkerの解決規則。
8. ambiguity/error recovery方針。
9. 変更した主要file。
10. 追加したfixture/evaluation。
11. 全profileの実測table。
12. `go test ./...`結果。
13. `go vet ./...`結果。
14. race test結果または未検証理由。
15. CGO-free native/cross-build結果。
16. CLI/MCP regression結果。
17. Git commit一覧またはcommit不能理由。
18. 既知のsemantic limitation。
19. 次に実装すべき別計画。
20. ユーザーの既存変更をreset、restore、stash、cleanしていないこと。

失敗、未実行、未達thresholdが残る場合は「完了」と表現しない。

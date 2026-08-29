# FocalSpan Retrieval Quality v0.2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development`（推奨）または `superpowers:executing-plans` で、この計画をTask単位に実装してください。進捗は各チェックボックスを更新して追跡します。

**Goal:** FocalSpanの検索を「FTSで候補を拾って固定ウェイトで並べる方式」から、質問意図を明示的に計画し、複数retrieverを決定的に統合する方式へ進化させ、日本語を含む問い合わせでもdefinition・callers・callees・tests・imports・references・impactを正しく選択できるようにする。

**Architecture:** `internal/query`で正規化とQuery Planを一度だけ生成し、`internal/search`でexact symbol・qualified symbol・prefix・FTS・path・relationを独立取得する。各ranked listをWeighted Reciprocal Rank Fusionで統合し、`internal/rank`がintent別profileで最終順位を決める。既存のpacker、CLI/MCP出力、SQLite schemaは維持し、Evaluation 2.0と`focalspan explain`で改善効果と判断根拠を検証可能にする。

**Tech Stack:** Go 1.26+、`database/sql`、SQLite FTS5（`modernc.org/sqlite`）、既存MCP Go SDK、標準`flag`、既存fixture/evaluator。新しい外部依存は追加しない。

**Spec:** `docs/design.md` の Retrieval and ranking / Token budget / CLI and MCP、`docs/evaluation.md`、および本計画。

## Codexへの実行指示

このファイルを渡されたCodexは、次の順で作業してください。

1. `AGENTS.md`、`PLAN.md`、`docs/design.md`、`docs/evaluation.md`を読む。
2. `git status --short`、`git diff --stat`、`git log -10 --oneline`を確認する。
3. Task 0から順に、テストを先に追加して実装する。
4. Taskを完了するたびに本ファイルのチェックボックスを更新する。
5. 各Taskの局所テストと`go test ./...`を通してから次へ進む。
6. Gitへ書き込める場合はTask単位でcommitする。権限がない場合は未commit差分を保持し、最終報告へ理由を書く。
7. 設計確認だけで停止せず、Task 9の検証と最終報告まで進める。

重大な仕様矛盾、既存データ破壊の危険、またはユーザーの変更を失う危険がある場合だけ質問してください。

## Global Constraints

- 現在のcheckoutを唯一のsource of truthとして扱う。
- 開始時から存在する未コミット差分を消さない。
- `git reset`、`git restore`、`git checkout --`、`git clean`、`git stash`を実行しない。
- Go 1.26以上を維持する。
- `CGO_ENABLED=0`のWindows amd64、Linux amd64、macOS arm64 buildを維持する。
- Python、Node.js、PHP、.NET、Clang、language server、外部LLMをproduction pathから起動しない。
- ネットワークアクセスを追加しない。
- リポジトリ内コードを実行しない。
- shell文字列を組み立てない。
- SQLite schema version 1を維持する。この計画ではmigrationを追加しない。
- MCP tool名とstructured output schemaを変更しない。
- MCP stdioのstdoutをプロトコル専用のまま維持する。
- CLIの既存commandと既存flagを壊さない。
- 検索結果は決定的でなければならない。
- 最終serialized payloadのtoken budget遵守を100%維持する。
- fixture固有の名前・path・queryをproduction codeへ埋め込まない。
- ranking weightは名前付き定数またはprofile structへ集約する。
- 曖昧なrelationを確定関係として扱わない。
- production pathへ未完成stubや未実装panicを残さない。
- C/C++、C#、JS/TS、PHP、Go、Smarty/templateの既存Extractorを本計画で再実装しない。
- query-aware source slicing、manifest linker、SCIP、embedding、model-specific tokenizerは本計画へ混ぜない。

---

## Current Baseline and Scope Gate

現行実装には、Go、PHP、C/C++、C#、JavaScript/TypeScript、Smarty/templateの専用Extractorが登録されている。既存評価は全profileでhit@5、budget compliance、determinismの基準を満たしている。

一方、検索経路には次の制約がある。

- query token抽出がASCII中心で、日本語の意図語を構造化していない。
- すべての検索がFTSから始まる。
- relation expansionのanchorもFTS結果からしか選べない。
- relation intentは英語の小さなswitchで判定される。
- すべてのqueryが同じranking profileを使う。
- evaluatorは最終hit/recallを測るが、Query Plan、retriever別寄与、ablationを測らない。
- packerのtest intent判定とsearch/rankのintent判定が分散している。

この計画の成功条件は「既存hit@5を維持する」だけではない。FTS単独では拾いにくいrelation intent queryと、日本語＋code identifierのqueryで、full retrievalがFTS-onlyより良い順位を出すことを証明する。

## Non-goals for v0.2

この計画では次を実装しない。

- compiler-grade cross-file type resolution
- `composer.json`、`tsconfig.json`、`.csproj`、`compile_commands.json` resolver
- query-aware control-flow slicing
- MMRまたはutility-per-token packer
- session state / known handles
- new MCP tool
- vector search / embedding
- FTS tokenizerまたはDB schemaの変更
- HTTP transport
- Web UI
- learned reranker
- telemetryまたは外部送信

## Target File Map

### Create

```text
internal/query/model.go
internal/query/normalize.go
internal/query/normalize_test.go
internal/query/fts.go
internal/query/fts_test.go
internal/query/planner.go
internal/query/planner_test.go

internal/search/retrieval.go
internal/search/retrieval_test.go
internal/search/fusion.go
internal/search/fusion_test.go
internal/search/trace.go

internal/rank/profile.go
internal/rank/profile_test.go

testdata/eval/ja-auth-cases.jsonl
testdata/eval/ja-jsts-cases.jsonl
```

### Modify

```text
internal/model/model.go

internal/app/service.go
internal/app/service_test.go

internal/search/search.go
internal/search/search_test.go
internal/search/query.go
internal/search/query_test.go

internal/store/store.go
internal/store/store_test.go

internal/rank/rank.go
internal/rank/rank_test.go

internal/budget/budget.go
internal/budget/budget_test.go

internal/eval/eval.go
internal/eval/eval_test.go

internal/cli/run.go
internal/cli/run_test.go

README.md
docs/design.md
docs/evaluation.md
docs/implementation-plan.md
PLAN.md
```

`internal/search/query.go`は移行期間の互換wrapperとして残す必要がなければ削除してよい。最終状態で同じquery normalization実装を二か所に残してはいけない。

---

## Shared Types and Interfaces

Task 1で次の型を`internal/query`へ追加する。

```go
package query

type Intent string

const (
    IntentDefinition Intent = "definition"
    IntentCallers    Intent = "callers"
    IntentCallees    Intent = "callees"
    IntentTests      Intent = "tests"
    IntentImports    Intent = "imports"
    IntentExports    Intent = "exports"
    IntentReferences Intent = "references"
    IntentImpact     Intent = "impact"
)

type Terms struct {
    Words       []string `json:"words"`
    Identifiers []string `json:"identifiers"`
    Phrases     []string `json:"phrases"`
    Paths       []string `json:"paths"`
    Symbols     []string `json:"symbols"`
    UnicodeRuns []string `json:"unicode_runs,omitempty"`
}

type Plan struct {
    RawQuery      string   `json:"raw_query"`
    Terms         Terms    `json:"terms"`
    Intents       []Intent `json:"intents"`
    PrimaryIntent Intent   `json:"primary_intent"`
    Anchors       []string `json:"anchors"`
    Relations     []string `json:"relations"`
    Profile       string   `json:"profile"`
}

func (p Plan) HasIntent(intent Intent) bool
func (p Plan) IntentStrings() []string
```

Task 4で次の型を`internal/search`へ追加する。

```go
package search

type RetrievalMode string

const (
    RetrievalFull        RetrievalMode = "full"
    RetrievalFTSOnly     RetrievalMode = "fts-only"
    RetrievalNoRelations RetrievalMode = "no-relations"
)

type RetrieverID string

const (
    RetrieverQualified RetrieverID = "qualified-symbol"
    RetrieverSymbol    RetrieverID = "symbol-exact"
    RetrieverPrefix    RetrieverID = "symbol-prefix"
    RetrieverFTS       RetrieverID = "fts"
    RetrieverPath      RetrieverID = "path"
    RetrieverRelation  RetrieverID = "relation"
)

type RankedList struct {
    Retriever RetrieverID
    Items     []model.RankedCandidate
}

type RetrievalContribution struct {
    Retriever   RetrieverID `json:"retriever"`
    Rank        int         `json:"rank"`
    Weight      float64     `json:"weight"`
    Contribution float64    `json:"contribution"`
}

type CandidateTrace struct {
    Handle        string                  `json:"handle"`
    Path          string                  `json:"path"`
    Symbol        string                  `json:"symbol"`
    Contributions []RetrievalContribution `json:"contributions"`
    FusionScore   float64                 `json:"fusion_score"`
    FinalScore    float64                 `json:"final_score"`
    Reasons       []model.ScoreReason     `json:"reasons"`
}

type RetrieverSummary struct {
    Retriever RetrieverID `json:"retriever"`
    Count     int         `json:"count"`
}

type SearchTrace struct {
    Mode       RetrievalMode     `json:"mode"`
    Lists      []RetrieverSummary `json:"lists"`
    Candidates []CandidateTrace  `json:"candidates"`
}

type SearchResult struct {
    Plan       query.Plan
    Candidates []model.RankedCandidate
    Trace      *SearchTrace
}
```

Task 3で`CandidateStore`を次へ拡張する。

```go
type CandidateStore interface {
    SearchFTS(ctx context.Context, ftsQuery string, limit int) ([]model.RankedCandidate, error)
    SearchQualifiedSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error)
    SearchExactSymbols(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error)
    SearchSymbolPrefixes(ctx context.Context, values []string, limit int) ([]model.RankedCandidate, error)
    SearchPaths(ctx context.Context, hints []string, limit int) ([]model.RankedCandidate, error)
    RelatedCandidates(ctx context.Context, handles []string, relation string) ([]model.RankedCandidate, error)
}
```

Task 4で、fusion scoreを最終rankまで失わないため`model.RankedCandidate`へ内部fieldを追加する。

```go
type RankedCandidate struct {
    // existing fields
    RetrievalScore float64
}
```

`RetrievalScore`はContextBundleへ直接serializeしない。FTSのraw BM25値は各ranked list内の順序決定にだけ使い、異なるretrieverのraw scoreを直接加算しない。

Task 6で`SearchRequest`へ追加する。

```go
type SearchRequest struct {
    Query       string
    Paths       []string
    ChangedOnly bool
    Changed     map[string][]LineRange
    Limit       int
    Mode        RetrievalMode
    Trace       bool
}
```

---

### Task 0: Protect the Merge, Verify Extractor Wiring, and Capture Baseline

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `docs/evaluation.md`
- Modify: `PLAN.md`

**Interfaces:**
- Consumes: current Extractor constructors and `extract.Registry.For`
- Produces: `func newExtractorRegistry() *extract.Registry`
- Produces: checked-in baseline table for every existing fixture

- [x] **Step 1: Record the starting worktree without modifying it**

Run:

```text
git status --short
git diff --stat
git log -10 --oneline
```

Copy the starting status into the final report. Do not reset, restore, stash, or clean anything.

- [x] **Step 2: Write a registry wiring test that fails until registry construction is testable**

Add a table-driven test to `internal/app/service_test.go`:

```go
func TestNewExtractorRegistrySelectsDedicatedExtractors(t *testing.T) {
    registry := newExtractorRegistry()
    cases := []struct {
        path     string
        language string
        wantName string
    }{
        {"main.go", "go", "go-ast"},
        {"auth.php", "php", "php-structural"},
        {"auth.cpp", "cpp", "cpp-structural"},
        {"Auth.cs", "csharp", "csharp-structural"},
        {"auth.ts", "typescript", "jsts-structural"},
        {"page.tpl", "smarty", "template-structural"},
    }
    for _, tc := range cases {
        extractor, ok := registry.For(tc.path, tc.language)
        if !ok {
            t.Fatalf("%s/%s: no extractor", tc.path, tc.language)
        }
        if got := extractor.Name(); got != tc.wantName {
            t.Errorf("%s/%s: got %q, want %q", tc.path, tc.language, got, tc.wantName)
        }
    }
}
```

実際の`Name()`が上記と異なる場合は、現在の専用Extractorが返す正式名を期待値に使う。generic名を期待値へ合わせてはいけない。

- [x] **Step 3: Run the focused test and verify the intended failure**

Run:

```text
go test ./internal/app -run TestNewExtractorRegistrySelectsDedicatedExtractors -v
```

Expected: `newExtractorRegistry`が未定義でFAILする。

- [x] **Step 4: Extract registry construction without changing order**

In `internal/app/service.go`:

```go
func newExtractorRegistry() *extract.Registry {
    return extract.NewRegistry(
        goast.NewExtractor(),
        php.NewExtractor(),
        cpp.NewExtractor(),
        csharp.NewExtractor(),
        jsts.NewExtractor(),
        templateextract.NewExtractor(),
        generic.NewExtractor(),
    )
}
```

`NewWithConfig`はこのhelperを使用する。

- [x] **Step 5: Run registry and full tests**

Run:

```text
go test ./internal/app -run TestNewExtractorRegistrySelectsDedicatedExtractors -v
go test ./...
```

Expected: PASS.

- [x] **Step 6: Capture all existing evaluation baselines**

Build once:

```text
go build -o .focalspan-plan-bin ./cmd/focalspan
```

Run all checked-in case sets that exist:

```text
.focalspan-plan-bin index --root testdata/repos/authsample --quiet
.focalspan-plan-bin eval --root testdata/repos/authsample --cases testdata/eval/cases.jsonl --json

.focalspan-plan-bin index --root testdata/repos/phpsample --quiet
.focalspan-plan-bin eval --root testdata/repos/phpsample --cases testdata/eval/php-cases.jsonl --json

.focalspan-plan-bin index --root testdata/repos/templatesample --quiet
.focalspan-plan-bin eval --root testdata/repos/templatesample --cases testdata/eval/template-cases.jsonl --json

.focalspan-plan-bin index --root testdata/repos/cppsample --quiet
.focalspan-plan-bin eval --root testdata/repos/cppsample --cases testdata/eval/cpp-cases.jsonl --json

.focalspan-plan-bin index --root testdata/repos/csharpsample --quiet
.focalspan-plan-bin eval --root testdata/repos/csharpsample --cases testdata/eval/csharp-cases.jsonl --json

.focalspan-plan-bin index --root testdata/repos/jstssample --quiet
.focalspan-plan-bin eval --root testdata/repos/jstssample --cases testdata/eval/jsts-cases.jsonl --json
```

Windows PowerShellでは`.focalspan-plan-bin.exe`へ読み替える。

- [x] **Step 7: Record the measured baseline**

`docs/evaluation.md`へ`Retrieval Quality v0.2 pre-change baseline`表を追加する。実行結果だけを記載し、未実行値を補完しない。

- [ ] **Step 8: Remove the temporary binary and commit**

```text
git add internal/app/service.go internal/app/service_test.go docs/evaluation.md PLAN.md
git commit -m "test: lock extractor wiring and retrieval baseline"
```

一時binaryはcommitしない。

---

### Task 1: Create the Shared Unicode-Aware Query Model

**Files:**
- Create: `internal/query/model.go`
- Create: `internal/query/normalize.go`
- Create: `internal/query/normalize_test.go`
- Create: `internal/query/fts.go`
- Create: `internal/query/fts_test.go`
- Modify or delete after migration: `internal/search/query.go`
- Modify or relocate: `internal/search/query_test.go`

**Interfaces:**
- Consumes: raw query string
- Produces: `query.Terms`
- Produces: `query.BuildFTS(terms Terms) string`
- Produces: deterministic, escaped, bounded FTS expression

- [x] **Step 1: Write normalization tests before implementation**

Tests must cover:

```go
func TestNormalizeMixedJapaneseAndIdentifier(t *testing.T) {
    got := Normalize(`ValidateToken の呼び出し元を探して`)
    assertContains(t, got.Identifiers, "ValidateToken")
    assertContains(t, got.Symbols, "ValidateToken")
    assertAnyContains(t, got.UnicodeRuns, "呼び出し元")
}

func TestNormalizeQualifiedSymbolsAndPaths(t *testing.T) {
    got := Normalize(`App\Auth\TokenService::ValidateToken in src/Auth/TokenService.php`)
    assertContains(t, got.Identifiers, `App\Auth\TokenService::ValidateToken`)
    assertContains(t, got.Paths, "src/Auth/TokenService.php")
}

func TestNormalizeIsDeterministicAndDeduplicated(t *testing.T) {
    first := Normalize(`ValidateToken ValidateToken validate_token`)
    second := Normalize(`ValidateToken ValidateToken validate_token`)
    if !reflect.DeepEqual(first, second) {
        t.Fatalf("non-deterministic: %#v %#v", first, second)
    }
}
```

さらに次をtestする。

- quoted phrase
- snake_case
- camelCase / PascalCase
- C++ `::`
- C#/Java `.`
- Windows path
- Japanese punctuation
- UTF-8 symbol
- malformed quote
- 128 runeを超えるtoken
- 32件を超える検索term
- NUL runeを含むquery
- 空白query

- [x] **Step 2: Run tests and verify failure**

```text
go test ./internal/query -run 'TestNormalize' -v
```

Expected: packageまたは関数未定義でFAIL。

- [x] **Step 3: Implement rune-based token scanning**

`regexp.MustCompile("[A-Za-z...]")`を中心にせず、rune scannerで以下を認識する。

- ASCII/Unicode letters and digits
- `_`, `:`, `.`, `/`, `\`, `-`
- quoted phrase
- CJK lexical run
- code identifierと日本語助詞の境界

`ValidateTokenの呼び出し元`から`ValidateToken`を分離する。日本語文全体を一つのcode symbolとして扱わない。

- [x] **Step 4: Implement bounded FTS construction**

`BuildFTS`は次を満たす。

```go
func BuildFTS(terms Terms) string
```

- raw queryをFTS syntaxとして渡さない
- double quoteを安全にescape
- empty termを除外
- deterministic order
- exact phraseを先に保持
- identifier、word、Unicode runをdeduplicate
- 最大32 term
- 1 term最大128 rune
- 結果が空なら空文字列
- user inputから`NEAR`、`OR`、`*`等をoperatorとして実行させない

- [x] **Step 5: Add FTS safety tests**

```go
func TestBuildFTSEscapesSyntax(t *testing.T) {
    got := BuildFTS(Normalize(`foo" OR * NEAR(bar)`))
    if strings.Contains(got, ` OR * `) {
        t.Fatalf("raw FTS syntax leaked: %q", got)
    }
}
```

一時SQLite FTS5 tableへ生成queryを渡し、次がerrorにならないことも確認する。

- unmatched quote
- parentheses
- `*`
- colon
- Japanese
- emoji
- path
- empty query

- [x] **Step 6: Migrate existing query tests**

既存`search.NormalizeQuery` / `search.BuildFTSQuery`のcall siteを`query.Normalize` / `query.BuildFTS`へ移す。

移行後に同じ実装を`internal/search/query.go`へ残さない。互換wrapperを一時的に置く場合も、このTaskの最後に全call siteを更新して削除する。

- [ ] **Step 7: Run tests and commit**

```text
gofmt -w internal/query internal/search
go test ./internal/query ./internal/search
go test ./...
git add internal/query internal/search
git commit -m "feat: add unicode-aware query normalization"
```

---

### Task 2: Add a Deterministic English/Japanese Query Planner

**Files:**
- Create: `internal/query/planner.go`
- Create: `internal/query/planner_test.go`
- Modify: `internal/query/model.go`

**Interfaces:**
- Consumes: `query.Terms`
- Produces: `func PlanQuery(raw string) Plan`
- Produces: intent, anchors, relations, profile exactly once per query

- [x] **Step 1: Write intent tests**

At minimum:

```go
func TestPlanJapaneseCallerIntent(t *testing.T) {
    plan := PlanQuery(`ValidateToken の呼び出し元はどこですか`)
    assertIntent(t, plan, IntentCallers)
    assertString(t, plan.Anchors, "ValidateToken")
    assertString(t, plan.Relations, "callers")
}

func TestPlanJapaneseTestIntent(t *testing.T) {
    plan := PlanQuery(`ValidateTokenを検証するテスト`)
    assertIntent(t, plan, IntentTests)
    assertString(t, plan.Relations, "tests")
}

func TestPlanEnglishCalleeIntent(t *testing.T) {
    plan := PlanQuery(`what does ValidateToken call?`)
    assertIntent(t, plan, IntentCallees)
    assertString(t, plan.Relations, "callees")
}

func TestPlanDoesNotTreatCallerIDAsCallerIntent(t *testing.T) {
    plan := PlanQuery(`find callerID`)
    if plan.HasIntent(IntentCallers) {
        t.Fatal("callerID is an identifier, not a relation request")
    }
}
```

- [x] **Step 2: Define the exact intent lexicon**

Use package-level immutable data with deterministic matching.

English:

```text
definition: define, defined, definition, implementation, declaration, where is
callers: caller, callers, called by, calls <anchor>, usages, used by
callees: callee, callees, calls from, what does, dependencies called
tests: test, tests, testing, coverage, spec
imports: import, imports, include, includes, require, extends, layout, partial
exports: export, exports, re-export
references: reference, references, implements, interface, inherits, type usage
impact: impact, affected, blast radius, what breaks
```

Japanese:

```text
definition: 定義, 実装, 宣言, どこにある, 場所
callers: 呼び出し元, 使用箇所, 利用箇所, どこから呼ばれる, 参照元
callees: 呼び出し先, 何を呼ぶ, 内部で呼ぶ, 依存先
tests: テスト, 検証コード, カバレッジ, 試験
imports: 読み込み, インポート, インクルード, require, 継承元テンプレート, 部品テンプレート
exports: エクスポート, 再エクスポート, 公開元
references: 参照, 実装している, 継承, 型の使用箇所
impact: 影響, 影響範囲, 波及, 壊れる箇所, 変更範囲
```

長いphraseを短いphraseより先に判定する。substringだけで英語identifierを誤認しない。

- [x] **Step 3: Define planner precedence**

Primary intent precedence:

```text
impact
tests
callers
callees
imports
exports
references
definition
```

複数intentは保持するが、ranking profileはprimary intentで決める。

relation mapping:

```go
var relationForIntent = map[Intent][]string{
    IntentCallers:    {"callers"},
    IntentCallees:    {"callees"},
    IntentTests:      {"tests"},
    IntentImports:    {"imports"},
    IntentExports:    {"exports"},
    IntentReferences: {"references"},
    IntentImpact:     {"callers", "tests", "references"},
}
```

definitionにはrelationを付けない。

- [x] **Step 4: Implement anchor extraction**

Anchor order:

1. fully qualified identifiers
2. exact probable symbols
3. path hints
4. quoted phrase only when it resembles code
5. fallback lexical term

Intent phrase自体をanchorにしない。最大8 anchors。重複なし。入力順を保つ。

- [x] **Step 5: Add ambiguity and mixed-intent tests**

Test:

- `ValidateTokenの実装とテスト`
- `auth.tsをimportしている箇所`
- `変更すると何が壊れるか`
- `TestValidateToken` only
- `importToken` identifier only
- `coverageReport` identifier only
- Japanese-only query with no code identifier
- C++ qualified name
- PHP qualified name
- C# dotted name

- [ ] **Step 6: Run tests and commit**

```text
gofmt -w internal/query
go test ./internal/query -v
go test ./...
git add internal/query
git commit -m "feat: plan retrieval from English and Japanese intent"
```

---

### Task 3: Add Store-Native Exact Symbol and Path Retrieval

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`
- Modify: all `SearchFTS` call sites and test fakes

**Interfaces:**
- Consumes: exact/qualified/prefix/path lookup values
- Produces: bounded `[]model.RankedCandidate`
- Preserves: existing candidate projection and source spans

- [x] **Step 1: Write failing store tests**

Use a temporary Store populated with symbols that intentionally do not all share FTS content.

Tests:

```go
func TestSearchExactSymbolsDoesNotDependOnFTSContent(t *testing.T)
func TestSearchQualifiedSymbolsUsesExactQualifiedName(t *testing.T)
func TestSearchSymbolPrefixesIsCaseInsensitiveAndBounded(t *testing.T)
func TestSearchPathsMatchesNormalizedPathHints(t *testing.T)
func TestSearchMethodsUseStableTieBreaks(t *testing.T)
func TestSearchMethodsClampLimits(t *testing.T)
```

Fixture facts:

- symbol `ValidateToken`
- qualified symbol `App\Auth\TokenService::ValidateToken`
- same simple symbol in another file
- path with Windows-origin slash normalization
- documentation chunk containing the same text but no matching symbol
- 250 generated candidates to verify limit clamp

- [x] **Step 2: Change `SearchFTS` to accept an explicit limit**

Signature:

```go
func (s *Store) SearchFTS(
    ctx context.Context,
    query string,
    limit int,
) ([]model.RankedCandidate, error)
```

Clamp helper:

```go
func retrievalLimit(value, fallback, maximum int) int
```

Use fallback 100 and maximum 500. Never interpolate the limit into SQL; bind it as a parameter.

Update all interfaces, fakes, app baseline calls, and tests.

- [x] **Step 3: Implement exact symbol retrieval**

```go
func (s *Store) SearchExactSymbols(
    ctx context.Context,
    values []string,
    limit int,
) ([]model.RankedCandidate, error)
```

Use normalized lowercase equality against `symbols.name`, join symbols to chunks by `symbol_handle`, and order by:

```text
exact input order
confidence DESC
span size ASC
path ASC
start_line ASC
handle ASC
```

When a symbol has multiple chunks, return source-bearing concrete chunks before outline chunks.

- [x] **Step 4: Implement qualified symbol retrieval**

```go
func (s *Store) SearchQualifiedSymbols(
    ctx context.Context,
    values []string,
    limit int,
) ([]model.RankedCandidate, error)
```

Match `symbols.qualified_name` exactly after case-normalization appropriate to the current storage convention. Do not globally merge case-sensitive C/C++ names.

Implementation rule:

- first attempt exact stored string
- optional lowercase fallback only as candidate retrieval
- ranking must retain exact-case preference
- no arbitrary first-result resolution

- [x] **Step 5: Implement prefix and path retrieval**

```go
func (s *Store) SearchSymbolPrefixes(
    ctx context.Context,
    values []string,
    limit int,
) ([]model.RankedCandidate, error)

func (s *Store) SearchPaths(
    ctx context.Context,
    hints []string,
    limit int,
) ([]model.RankedCandidate, error)
```

Escape SQL LIKE metacharacters. Use parameter binding. Normalize path separators to `/`. Exact path and suffix match rank ahead of substring match.

- [x] **Step 6: Verify cancellation and malformed inputs**

Tests must show:

- canceled context returns context error
- empty values return empty slice
- `%`, `_`, `'`, `\`, NUL-like input do not alter SQL structure
- duplicate input terms do not duplicate rows
- all returned line ranges are valid

- [ ] **Step 7: Run tests and commit**

```text
gofmt -w internal/store internal/app internal/search internal/eval
go test ./internal/store -v
go test ./...
git add internal/store internal/app internal/search internal/eval
git commit -m "feat: add exact symbol and path retrieval"
```

---

### Task 4: Build Independent Retrievers and Weighted RRF Fusion

**Files:**
- Create: `internal/search/retrieval.go`
- Create: `internal/search/retrieval_test.go`
- Create: `internal/search/fusion.go`
- Create: `internal/search/fusion_test.go`
- Create: `internal/search/trace.go`
- Modify: `internal/search/search.go`
- Modify: `internal/search/search_test.go`

**Interfaces:**
- Consumes: `query.Plan`, `SearchRequest`, `CandidateStore`
- Produces: independent ranked lists
- Produces: fused candidates with trace
- Preserves: deterministic cap and path/changed filtering

- [x] **Step 1: Write retriever selection tests**

Test matrix:

```text
definition -> qualified, exact, prefix, FTS, path
callers    -> qualified, exact, prefix, FTS, path, relation(callers)
callees    -> qualified, exact, prefix, FTS, path, relation(callees)
tests      -> qualified, exact, prefix, FTS, path, relation(tests)
imports    -> qualified, exact, prefix, FTS, path, relation(imports)
references -> qualified, exact, prefix, FTS, path, relation(references)
fts-only   -> FTS only
no-relations -> all base retrievers, no relation retriever
```

- [x] **Step 2: Implement concrete retrieval without a plugin framework**

Implement one orchestration type:

```go
type RetrieverSet struct {
    store CandidateStore
}

func (r *RetrieverSet) Retrieve(
    ctx context.Context,
    plan query.Plan,
    req SearchRequest,
) ([]RankedList, error)
```

Do not create one interface and one file per fixed retriever.

Caps:

```go
const (
    qualifiedLimit = 50
    exactLimit     = 50
    prefixLimit    = 50
    ftsLimit       = 100
    pathLimit      = 50
    relationLimit  = 100
    fusedLimit     = 400
)
```

- [x] **Step 3: Make relation anchors independent of FTS**

Anchor candidate order:

1. qualified symbol results
2. exact symbol results
3. exact path results
4. high-confidence prefix results
5. FTS result only as final fallback

Limit anchors to 8 after deterministic deduplication.

This test must pass:

```go
func TestRelationRetrievalWorksWhenFTSMissesAnchor(t *testing.T) {
    // FTS returns no candidate.
    // SearchExactSymbols returns ValidateToken.
    // RelatedCandidates returns Authenticate.
    // caller query must include Authenticate.
}
```

- [x] **Step 4: Implement Weighted Reciprocal Rank Fusion**

Exact formula:

```go
contribution := weight / (rrfK + float64(rank))
```

Where rank is 1-based and:

```go
const rrfK = 60.0

var retrieverWeights = map[RetrieverID]float64{
    RetrieverQualified: 2.00,
    RetrieverSymbol:    1.80,
    RetrieverRelation:  1.60,
    RetrieverPrefix:    1.20,
    RetrieverFTS:       1.00,
    RetrieverPath:      0.90,
}
```

Use named constants/data, not map construction inside a candidate loop.

- [x] **Step 5: Define canonical candidate identity**

Primary key: `Handle`.

Fallback only when Handle is blank:

```text
path + NUL + start_byte + NUL + end_byte + NUL + kind + NUL + symbol
```

Never merge two non-empty different handles merely because content hashes match at fusion time. Content/spatial deduplication remains the reranker's responsibility.

- [x] **Step 6: Preserve contribution traces**

For every fused candidate:

- source retriever
- 1-based rank
- configured weight
- exact RRF contribution
- total fusion score

Fusion後は、各list固有のraw `Score`を最終scoreとして持ち越さない。採用したcandidate copyの`Score`を0へ初期化し、合計RRF値を`RetrievalScore`へ設定する。FTS BM25等のraw scoreはlist内順位へ既に反映されているため、異なる尺度のraw値を重ねて加算しない。

Sort contribution list by `RetrieverID` for deterministic JSON.

- [x] **Step 7: Define fusion tie breaking**

1. fusion score descending
2. number of contributing retrievers descending
3. confidence descending
4. span size ascending
5. path ascending
6. start line ascending
7. handle ascending

- [x] **Step 8: Test duplicate and deterministic behavior**

Tests:

- same handle in FTS and exact list merges once
- different handles with same symbol do not merge
- input list map iteration order cannot alter output
- equal score tie is stable
- canceled context stops relation retrieval
- retriever error includes retriever name and wraps cause
- candidate cap is enforced after fusion
- fts-only never calls relation store

- [ ] **Step 9: Run tests and commit**

```text
gofmt -w internal/search
go test ./internal/search -v
go test ./...
git add internal/search
git commit -m "feat: fuse independent retrieval signals"
```

---

### Task 5: Replace One-Size-Fits-All Ranking with Intent Profiles

**Files:**
- Create: `internal/rank/profile.go`
- Create: `internal/rank/profile_test.go`
- Modify: `internal/rank/rank.go`
- Modify: `internal/rank/rank_test.go`

**Interfaces:**
- Consumes: `query.Plan`
- Produces: `func RankWithPlan([]model.RankedCandidate, query.Plan) []model.RankedCandidate`
- Preserves: `Rank` as a small compatibility wrapper only where existing internal callers still need it

- [x] **Step 1: Write ranking profile tests**

Tests must demonstrate:

```text
definition query -> exact implementation ahead of caller
callers query -> caller relation ahead of definition, definition still retained
callees query -> callee relation ahead of unrelated exact-text hit
tests query -> test relation ahead of production-only hit
imports query -> import relation ahead of docs mentioning import
references query -> type/reference relation ahead of lexical noise
Japanese test query -> test candidate is not penalized
non-test query -> unrelated tests remain penalized
```

- [x] **Step 2: Define immutable profiles**

Example:

```go
type Profile struct {
    Name                 string
    QualifiedExact       float64
    SymbolExact          float64
    Prefix               float64
    LexicalMax           float64
    PathMax              float64
    ChangedFile          float64
    TestMatch            float64
    NonTestPenalty       float64
    DocumentationPenalty float64
    RelationWeights      map[string]float64
}
```

Return a fresh profile or immutable package value from:

```go
func ProfileFor(plan query.Plan) Profile
```

Do not mutate shared maps.

- [x] **Step 3: Seed final ranking with fusion score**

The RRF score must contribute via an explicit reason:

```go
model.ScoreReason{
    Code:   "retrieval-fusion",
    Weight: candidate.RetrievalScore * fusionScale,
    Detail: "candidate is supported by ranked retrieval signals",
}
```

Set `fusionScale` as a named constant. Store BM25等のraw scoreはranked-list内の順位決定に利用し、fusion後に別尺度として再加算しない。この変換は`RetrievalScore`とtraceで明示する。

- [x] **Step 4: Centralize relation weights**

Remove the current relation-weight map from the inner loop. Define relation weights per profile.

Suggested relative behavior:

```text
definition: relation candidates useful but secondary
callers: callers strongest, tests secondary
callees: callees strongest
tests: tests strongest, definition retained
imports: imports/exports strongest
references: references strongest
impact: callers/tests/references all strong
```

Exact numeric values may be adjusted only against checked-in evaluation and must be recorded in `docs/design.md`.

- [x] **Step 5: Use planner intent for test behavior**

Remove duplicate English-only test detection from ranking.

```go
if plan.HasIntent(query.IntentTests) { ... }
```

`TestValidateToken` as an identifier alone must not force a test-intent profile unless the query itself asks for tests; it can still receive symbol exact relevance.

- [x] **Step 6: Preserve reasons and deterministic tie break**

No reason code should be appended twice to the same candidate. Keep existing final tie order:

```text
score DESC
confidence DESC
span size ASC
path ASC
start line ASC
handle ASC
```

- [ ] **Step 7: Run tests and commit**

```text
gofmt -w internal/rank
go test ./internal/rank -v
go test ./...
git add internal/rank
git commit -m "feat: rank results with query intent profiles"
```

---

### Task 6: Integrate Query Plan, Search Trace, and Intent into App/Packer

**Files:**
- Modify: `internal/model/model.go`
- Modify: `internal/search/search.go`
- Modify: `internal/search/search_test.go`
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `internal/budget/budget.go`
- Modify: `internal/budget/budget_test.go`

**Interfaces:**
- Produces: `Searcher.SearchDetailed(ctx, req) (SearchResult, error)`
- Preserves: `Searcher.Search(ctx, req) ([]model.RankedCandidate, error)`
- Propagates: one `query.Plan` from search through ranking to packing
- Adds: `IntentHints []string` to internal `model.PackRequest`

- [x] **Step 1: Add internal intent hints to PackRequest**

```go
type PackRequest struct {
    Query         string
    IndexRevision string
    TokenBudget   int
    Mode          string
    Candidates    []RankedCandidate
    IntentHints   []string
}
```

This field is internal and must not be added to MCP structured output.

- [x] **Step 2: Write packer tests for Japanese intent**

```go
func TestPackerUsesJapaneseTestIntentHint(t *testing.T) {
    req := model.PackRequest{
        Query:       "ValidateTokenを検証するテスト",
        TokenBudget: 1200,
        Mode:        "source",
        IntentHints: []string{"tests"},
        Candidates: []model.RankedCandidate{
            productionCandidate(),
            testCandidate(),
        },
    }
    bundle := NewPacker(NewEstimator()).Pack(req)
    if !containsKind(bundle.Items, "test") {
        t.Fatal("test candidate was dropped despite tests intent")
    }
}
```

Also test empty hints fallback does not break direct packer callers.

- [x] **Step 3: Implement SearchDetailed**

```go
func (s *Searcher) SearchDetailed(
    ctx context.Context,
    req SearchRequest,
) (SearchResult, error)
```

Flow:

```text
validate query
PlanQuery once
retrieve ranked lists
fuse lists
apply path filter
mark changed overlap
rank with plan
limit final candidates
build trace
```

`Search` calls `SearchDetailed` and returns only `.Candidates`.

- [x] **Step 4: Keep trace generation off the hot output path unless requested**

When `req.Trace == false`, return `SearchResult.Plan` and candidates but leave `SearchResult.Trace == nil`. Do not retain intermediate source copies solely for trace. When trace is enabled, it may retain handles, path, symbol, ranks, numeric scores, and reasons, but not source content.

- [x] **Step 5: Integrate Service.Query**

`Service.Query` uses `SearchDetailed` and passes:

```go
IntentHints: result.Plan.IntentStrings()
```

to `PackRequest`.

The public `ContextBundle` schema remains unchanged.

- [x] **Step 6: Fix evaluation baseline candidate consistency**

Add:

```go
func (s *Service) BaselineTokensForRequest(
    ctx context.Context,
    req QueryRequest,
) (int, error)
```

It must use the same retrieval mode, path filters, and query plan as the evaluated query, then count each candidate file once.

Keep:

```go
func (s *Service) BaselineTokens(ctx context.Context, query string) (int, error)
```

as a compatibility wrapper around a default `QueryRequest`.

Do not compare full-retrieval output against an unrelated FTS-only baseline.

- [x] **Step 7: Update Impact and Expand deliberately**

`Impact` may continue using its direct changed-span candidate flow in this version. Use a synthetic impact plan when ranking:

```go
plan := query.Plan{
    RawQuery:      "Git impact",
    Intents:       []query.Intent{query.IntentImpact},
    PrimaryIntent: query.IntentImpact,
    Profile:       "impact",
}
```

`Expand` may keep relation-explicit behavior; pass its relation as an intent hint when possible.

- [x] **Step 8: Test backward compatibility**

- `Service.Query` returns same JSON shape
- MCP tool list unchanged
- `code_context` has no trace field
- query without explicit intent still works
- `ChangedOnly` and path filters still apply before final rank
- cancellation propagates
- blank query remains validation error
- no source appears in trace
- final budget remains compliant

- [ ] **Step 9: Run tests and commit**

```text
gofmt -w internal/model internal/search internal/app internal/budget
go test ./internal/search ./internal/app ./internal/budget -v
go test ./...
git add internal/model internal/search internal/app internal/budget
git commit -m "feat: propagate query plans through retrieval and packing"
```

---

### Task 7: Implement Evaluation 2.0 and Ablation Comparison

**Files:**
- Modify: `internal/eval/eval.go`
- Modify: `internal/eval/eval_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Create: `testdata/eval/ja-auth-cases.jsonl`
- Create: `testdata/eval/ja-jsts-cases.jsonl`

**Interfaces:**
- Consumes: existing Case JSON/JSONL unchanged
- Adds optional case expectations
- Adds: `--ablation full|fts-only|no-relations|all`
- Preserves: default `focalspan eval` output fields

- [x] **Step 1: Extend Case additively**

```go
type Case struct {
    Name              string   `json:"name"`
    Query             string   `json:"query"`
    TokenBudget       int      `json:"token_budget"`
    ExpectedSymbols   []string `json:"expected_symbols"`
    ExpectedPaths     []string `json:"expected_paths"`
    ForbiddenPaths    []string `json:"forbidden_paths"`
    ExpectedIntents   []string `json:"expected_intents,omitempty"`
    ExpectedRelations []string `json:"expected_relations,omitempty"`
    ExpectedKinds     []string `json:"expected_kinds,omitempty"`
}
```

Existing case files must parse identically.

- [x] **Step 2: Extend results additively**

```go
type CaseResult struct {
    // existing fields
    IntentRecall   float64 `json:"intent_recall,omitempty"`
    RelationRecall float64 `json:"relation_recall,omitempty"`
    KindRecall     float64 `json:"kind_recall,omitempty"`
    RetrievalMode  string  `json:"retrieval_mode"`
}
```

`Report` adds aggregate fields and:

```go
ByPrimaryIntent map[string]IntentReport `json:"by_primary_intent,omitempty"`
```

Do not rename or remove existing JSON keys.

- [x] **Step 3: Pass retrieval mode into QueryRequest**

Add an internal field to `app.QueryRequest`:

```go
RetrievalMode search.RetrievalMode
```

CLI `query` and MCP default to `full`; this Task does not expose ablation mode on normal query or MCP.

- [x] **Step 3a: Make evaluator baseline mode-aware**

Update the evaluator interface to use the exact request being measured:

```go
type Queryer interface {
    Query(context.Context, app.QueryRequest) (model.ContextBundle, error)
    BaselineTokensForRequest(context.Context, app.QueryRequest) (int, error)
}
```

`Evaluate` must pass the same `RetrievalMode`, query, path constraints, and token budget context to both the query and its baseline. Intent recall is computed from `query.PlanQuery(item.Query)`; relation and kind recall are computed from the returned `ContextBundle.Items`.

- [x] **Step 4: Add eval CLI flag**

```text
focalspan eval --ablation full
focalspan eval --ablation fts-only
focalspan eval --ablation no-relations
focalspan eval --ablation all
```

Rules:

- default `full`
- unknown value is validation error
- `all` runs three reports in deterministic order
- JSON for `all`:

```json
{
  "reports": {
    "full": {},
    "fts-only": {},
    "no-relations": {}
  }
}
```

- human output prints one section per mode
- each mode runs every case twice for determinism
- baseline token calculation uses the same mode

- [x] **Step 5: Add Japanese auth cases**

`testdata/eval/ja-auth-cases.jsonl`:

```json
{"name":"ja-go-callers","query":"ValidateToken の呼び出し元はどこですか","token_budget":1200,"expected_symbols":["Authenticate"],"expected_paths":["http/middleware.go"],"forbidden_paths":["unrelated/report.go"],"expected_intents":["callers"],"expected_relations":["callers"]}
{"name":"ja-go-tests","query":"ValidateTokenを検証するテストを探して","token_budget":1200,"expected_paths":["auth/service_test.go"],"forbidden_paths":["unrelated/report.go"],"expected_intents":["tests"],"expected_relations":["tests"],"expected_kinds":["test"]}
{"name":"ja-go-definition","query":"期限切れ認証トークンを拒否するValidateTokenの実装","token_budget":1200,"expected_symbols":["ValidateToken"],"expected_paths":["auth/service.go"],"forbidden_paths":["unrelated/report.go"],"expected_intents":["definition"]}
```

現在fixtureの実際のpath/symbolが異なる場合は、fixtureの事実に合わせて修正する。production codeをcaseへ合わせない。

- [x] **Step 6: Add Japanese JS/TS cases**

`testdata/eval/ja-jsts-cases.jsonl`:

```json
{"name":"ja-ts-callers","query":"validateToken の呼び出し元を探して","token_budget":1400,"expected_paths":["src/http/auth-middleware.ts"],"forbidden_paths":["unrelated/report.ts"],"expected_intents":["callers"],"expected_relations":["callers"]}
{"name":"ja-ts-tests","query":"期限切れtokenを検証するテスト","token_budget":1400,"expected_paths":["tests/token-service.test.ts"],"forbidden_paths":["unrelated/report.ts"],"expected_intents":["tests"],"expected_relations":["tests"],"expected_kinds":["test"]}
{"name":"ja-ts-imports","query":"token-serviceを読み込んでいるmodule","token_budget":1400,"expected_paths":["src/http/auth-middleware.ts"],"forbidden_paths":["unrelated/report.ts"],"expected_intents":["imports"],"expected_relations":["imports"]}
```

- [x] **Step 7: Add ablation assertions**

Automated test must show:

```text
full hit@3 >= fts-only hit@3 on relation-intent Japanese cases
full relation recall > fts-only relation recall
no-relations produces zero relation recall where a relation is required
all modes remain deterministic
all modes remain budget compliant
```

Do not require full to beat FTS-only on every lexical definition case.

- [x] **Step 8: Keep acceptance thresholds strict**

For existing case sets under `full`:

```text
hit@5 = 1.0
budget compliance = 1.0
forbidden violations = 0
determinism = 1.0
median reduction <= 0.25
```

For new Japanese sets under `full`:

```text
intent recall = 1.0
hit@5 = 1.0
budget compliance = 1.0
forbidden violations = 0
determinism = 1.0
relation recall = 1.0 for relation-bearing cases
```

- [x] **Step 9: Run tests and commit**

```text
gofmt -w internal/eval internal/cli internal/app
go test ./internal/eval ./internal/cli ./internal/app -v
go test ./...
git add internal/eval internal/cli internal/app testdata/eval
git commit -m "test: compare retrieval modes and Japanese intent"
```

---

### Task 8: Add `focalspan explain` for Local Retrieval Debugging

**Files:**
- Modify: `internal/app/service.go`
- Modify: `internal/app/service_test.go`
- Modify: `internal/cli/run.go`
- Modify: `internal/cli/run_test.go`
- Modify: `internal/render/` only if an existing renderer is the natural fit

**Interfaces:**
- Produces: `Service.Explain`
- Adds CLI only: `focalspan explain`
- Does not add an MCP tool
- Does not return source body by default

- [x] **Step 1: Define explain request/result**

In `internal/app` or a neutral internal package:

```go
type ExplainRequest struct {
    Query         string
    Paths         []string
    ChangedOnly   bool
    NoUpdate      bool
    Limit         int
    RetrievalMode search.RetrievalMode
}

type ExplainResult struct {
    Plan       query.Plan              `json:"plan"`
    Mode       search.RetrievalMode    `json:"mode"`
    Lists      []search.RetrieverSummary `json:"lists"`
    Candidates []search.CandidateTrace `json:"candidates"`
}
```

- [x] **Step 2: Implement Service.Explain**

Behavior:

- same auto-update rule as Query unless `NoUpdate`
- same root/path/changed restrictions as Query
- `Trace: true`
- default limit 20
- maximum limit 100
- no source code content in result
- no arbitrary file reads beyond normal index update
- cancellation propagation
- blank query error

- [x] **Step 3: Add CLI**

Syntax:

```text
focalspan explain --query "ValidateToken の呼び出し元" --json
focalspan explain --query "what calls ValidateToken?" --limit 10
focalspan explain --query "what imports auth.ts?" --ablation fts-only
```

Flags:

```text
--query
--root
--path (repeatable if existing CLI helper supports it)
--changed-only
--no-update
--limit
--ablation full|fts-only|no-relations
--json
```

Do not add `--include-source` in this version.

- [x] **Step 4: Define concise human output**

Example:

```text
query: ValidateToken の呼び出し元
intent: callers
anchors: ValidateToken
relations: callers
mode: full

1. http/middleware.go:44 Authenticate
   final=648.1 fusion=0.071
   sources=relation#1, fts#4
   reasons=relation-callers, lexical, retrieval-fusion
```

No source body and no full `ScoreReason.Detail` in human output.

- [x] **Step 5: Test safety and determinism**

Tests:

- JSON is valid and stable across repeated calls
- no `Content` or source body field
- source text from fixture does not appear
- Japanese plan appears
- FTS-only trace has no relation contribution
- full trace records relation contribution
- stdout/stderr separation
- usage includes `explain`
- existing `serve` stdout behavior unchanged
- existing MCP tool list remains exactly four tools

- [x] **Step 6: Run tests and commit**

```text
gofmt -w internal/app internal/cli internal/render
go test ./internal/app ./internal/cli ./internal/mcpserver -v
go test ./...
git add internal/app internal/cli internal/render
git commit -m "feat: explain retrieval and ranking decisions"
```

---

### Task 9: Documentation, Full Regression, Cross-Build, and Acceptance

**Files:**
- Modify: `README.md`
- Modify: `docs/design.md`
- Modify: `docs/evaluation.md`
- Modify: `docs/implementation-plan.md`
- Modify: `PLAN.md`

**Interfaces:**
- Documents: Query Planner, retrievers, RRF, profiles, ablations, explain
- Records: actual measured results only
- Leaves: next-stage roadmap clearly separated

- [x] **Step 1: Update design documentation**

`docs/design.md`へ次を記載する。

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

Decision logへ記録:

- planner is deterministic and local
- Japanese support is intent/lexical normalization, not translation or semantic embedding
- exact symbol retrieval is no longer gated by FTS
- relation expansion anchors exact structural lookup before FTS fallback
- RRF combines ranks instead of raw BM25/heuristic score scales
- MCP contract remains unchanged
- explain is CLI-only and source-free
- schema remains version 1

- [x] **Step 2: Update README**

Add examples:

```text
focalspan query --query "ValidateToken の呼び出し元はどこですか" --budget 1200
focalspan query --query "ValidateTokenを検証するテスト" --budget 1200
focalspan explain --query "ValidateToken の呼び出し元" --json
focalspan eval --root testdata/repos/authsample --cases testdata/eval/ja-auth-cases.jsonl --ablation all --json
```

State limits accurately:

- no automatic translation
- no compiler-grade cross-file resolution
- Japanese relation intent and mixed identifier queries supported
- Japanese-only conceptual search remains lexical
- no embedding/vector search

- [x] **Step 3: Update implementation plan status**

`docs/implementation-plan.md`へv0.2 completion sectionを追加する。古いMVP historyを削除しない。完了していない次段階を実装済みと書かない。

- [x] **Step 4: Run formatting and static checks**

```text
gofmt -w .
go test ./...
go vet ./...
```

Expected: all PASS.

- [x] **Step 5: Run race test where supported**

```text
go test -race ./...
```

Windows環境でC compilerがなく実行不能なら、PASSと書かない。command、error、未検証範囲を記録する。
今回の環境では`CGO_ENABLED=1`指定でもPATH上に`gcc`がなく未検証。
`C:\\cygwin64\\bin\\gcc.exe`を`CC`へ指定しても、GoがCygwin compilerをnative Windows向けに拒否した。

- [x] **Step 6: Run CGO-free builds**

```text
CGO_ENABLED=0 go build -o .build/focalspan ./cmd/focalspan
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o .build/focalspan-windows-amd64.exe ./cmd/focalspan
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o .build/focalspan-linux-amd64 ./cmd/focalspan
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o .build/focalspan-darwin-arm64 ./cmd/focalspan
```

PowerShellでは環境変数設定をシェルに合わせて実行する。生成物はcommitしない。

- [x] **Step 7: Run every existing evaluation under full mode**

Run all case files present in `testdata/eval`, including:

```text
cases.jsonl
php-cases.jsonl
template-cases.jsonl
cpp-cases.jsonl
csharp-cases.jsonl
jsts-cases.jsonl
ja-auth-cases.jsonl
ja-jsts-cases.jsonl
```

Each root must be indexed immediately before its case set.

- [x] **Step 8: Run Japanese ablation comparisons**

```text
focalspan eval --root testdata/repos/authsample --cases testdata/eval/ja-auth-cases.jsonl --ablation all --json
focalspan eval --root testdata/repos/jstssample --cases testdata/eval/ja-jsts-cases.jsonl --ablation all --json
```

Confirm actual result items, not only aggregate numbers.

- [x] **Step 9: Verify MCP regression**

Run MCP integration tests and a bounded stdio startup smoke. Confirm:

```text
code_context
code_expand
code_impact
code_restart
code_status
```

are the only MCP tools in the current checkout, and stdout contains no
application log. The current checkout retains the pre-existing five-tool
`code_restart` extension.

- [x] **Step 10: Record measured results**

Update `docs/evaluation.md` with:

- pre-change and post-change metrics
- full / fts-only / no-relations Japanese comparison
- existing profile regressions
- test/vet/build results
- race test status
- any case that worsened and its explanation

Do not hide regressions by changing thresholds or removing cases.

- [x] **Step 11: Self-review the diff**

Check:

1. normalization exists in one package only
2. planner is called once per query
3. relation retrieval no longer requires an FTS anchor
4. FTS-only truly calls only FTS
5. Japanese intent is not duplicated in rank/budget/search
6. RRF contribution is deterministic
7. fixed profile weights are not built inside hot loops
8. trace contains no source
9. MCP contract did not change
10. SQLite schema did not change
11. existing extractors were not rewritten
12. existing hit@5/budget/determinism did not regress
13. no fixture-specific production branch exists
14. no generated binary is staged
15. no pre-existing user change was lost

- [ ] **Step 12: Remove build artifacts and commit**

```text
git add README.md docs PLAN.md
git commit -m "docs: document retrieval quality v0.2"
```

Do not add `.build/` or temporary indexes.

---

## Final Acceptance Criteria

The work is complete only when all items below are true.

### Behavior

- A single deterministic Query Plan is generated per query.
- Mixed Japanese/code queries preserve the code identifier.
- Japanese callers/callees/tests/imports/references/impact intent is recognized.
- Exact symbol lookup works even when FTS returns no anchor.
- Base retrievers are independently testable.
- Relation retrieval is disabled in `fts-only` and `no-relations` as specified.
- Weighted RRF combines candidate ranks with an explainable contribution.
- Intent-specific ranking changes ordering without fixture-specific branches.
- Test intent reaches the packer without duplicated English-only logic.
- Existing `ContextBundle` and MCP structured output remain backward compatible.
- `focalspan explain` shows plan, retriever contribution, and final reasons without source content.
- Normal query and MCP calls do not expose trace payloads.

### Quality

- Existing full-mode hit@5 remains 1.0 for every checked-in profile.
- Existing full-mode budget compliance remains 1.0.
- Existing forbidden-path violations remain 0.
- Existing deterministic output remains 1.0.
- Existing median reduction ratio remains at or below 0.25.
- Japanese case intent recall is 1.0.
- Japanese case full-mode hit@5 is 1.0.
- Japanese relation-bearing case relation recall is 1.0.
- Full-mode hit@3 is not worse than FTS-only on the new relation-intent set.
- At least one checked-in relation-intent case proves full retrieval succeeds where FTS-only misses or ranks the expected relation lower.
- All returned items retain valid repository paths and line ranges.
- No large unrelated full file enters a bundle.

### Engineering

- `go test ./...` passes.
- `go vet ./...` passes.
- Race status is reported honestly.
- CGO-free native and three target cross-builds pass.
- No new runtime dependency is introduced.
- SQLite schema remains version 1.
- Existing CLI commands work.
- MCP exposes exactly four existing tools.
- MCP stdout remains protocol-only.
- Current working-tree changes are preserved.
- Documentation matches observed behavior.

---

## Deferred Follow-up Plans

Do not implement these as part of this file.

### v0.3 — Query-Aware Semantic Zoom

- evidence spans
- query-hit and enclosing-control-flow slicing
- source skeleton levels
- marginal utility per token
- context diversity / deterministic MMR
- stateless known-handle exclusion

### v0.4 — Repository Linker

- `composer.json` PSR-4
- `tsconfig.json` / `jsconfig.json`
- `.csproj` / `Directory.Build.props`
- `compile_commands.json`
- declaration/definition pairing
- explicit ambiguous candidate sets
- relation provenance and resolution state

### v0.5 — Optional Semantic Facts

- SCIP importer
- optional Clang/Roslyn/TypeScript fact adapters
- model-specific tokenizer
- no change to core MCP output contract unless separately specified

---

## Final Report Format

Codexの最終報告は次の順序にする。

1. 開始時のGit状態
2. Query modelと日本語normalization
3. Query Plannerのintentとprecedence
4. 追加したstore retrieval methods
5. retriever構成と各cap
6. RRF formulaとweight
7. intent-aware ranking profiles
8. app/packerへのplan伝播
9. Evaluation 2.0とablation
10. `focalspan explain`の使用例
11. 変更した主要file
12. unit/integration test結果
13. 既存fixtureのpre/post metrics
14. 日本語fixtureのfull/fts-only/no-relations比較
15. `go vet`結果
16. race test結果または未検証理由
17. CGO-free build結果
18. MCP regression結果
19. Git commit一覧またはcommit不能理由
20. 既知の制限
21. 次に進めるべきdeferred plan

実行していない検証を成功と表現しない。失敗が残る場合は完了と表現せず、失敗内容、原因、影響範囲、残作業を具体的に報告する。

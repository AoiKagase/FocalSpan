# C++監査20件の機能修正 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: `superpowers:executing-plans` を使用し、以下を順に実行する。共有するパーサーを扱うため実装は直列とし、最後に独立レビューを行う。

**Goal:** 監査CP01〜CP20を正式回帰テストで修正し、C++の宣言・原文範囲・呼出し・モジュール関係と検索結果を一致させる。

**Architecture:** 字句解析、宣言解析、呼出し分類、名前解決、公開検索の順に修正する。既存の静的・保守的解析を維持し、確定できない関係を同名シンボルへ推測接続しない。共有するC++名前探索規則を抽出・リンク・未解決関係の検索に適用する。

**Tech Stack:** Go、既存C++トークンパーサー、SQLite/FTS5、既存Evidence/MCPテスト。新規依存なし。

**Spec:** `docs/audits/2026-10-18-cpp-focused-audit.md` および同名JSON。監査基準HEADは `ca46099c2b190ba03539dc092794d9473e21c5bd`。20件、失敗再現27条件、成功対照7条件。件数を増減させる再分類は行わない。

## Purpose / Big Picture

数字区切り・BOM・template・namespace・関数宣言を理由に後続のシンボルが消える問題、関数の本文範囲が欠ける問題、同名関数への誤接続、正しい関係が検索に届かない問題を修正する。各IDについて修正前FAILと修正後PASSを残し、Query/Expand/Impactまで必要な経路を検証する。

このファイルは**実行承認前の提案計画**である。現時点では完了済みルート`PLAN.md`を置き換えず、製品コードも変更しない。実装開始時に後述の計画移行を行う。

## Global Constraints

1. サイバーセキュリティ項目、脆弱性診断、権限・境界対策の追加、原因未確定のsupervisor EOFを対象外とする。今回の20件はC++機能上の不整合である。
2. MCP公開フィールド、Evidence形式、DBスキーマを維持する。原文のバイト列・行位置、wire予算、共有検索ロック、更新設定、候補範囲と順位の契約を維持する。
3. 新規依存・ネットワーク・外部LLM・解析対象C++のコンパイル／実行・パッケージ復元を追加しない。通常のGo製品／テスト検証と、解析対象コードの実行を区別する。
4. 既存dirtyファイル、稼働DB、設定、バイナリ、未知のロックを保持する。pushと`H:/bin/focalspan.exe`への再配置は行わない。
5. 不明な型、動的receiver、macro展開、完全なoverload/virtual dispatch、template実体化、全C++規格の実装は行わない。解析不能・曖昧な入力は保守的に未解決とし、既存diagnostic経路が適用できる場合に記録する。

## Review Focus

以下の5観点には所有タスクで名前付き回帰テストを置く。

1. **原文境界:** BOM・CRLF・数字区切り・コメント継続でも元バイト位置を保持する。Task 1〜4の`SourceOffsets`。
2. **宣言と式の区別:** custom type、template header、operator、initializer、requires、catchの境界を誤認しない。Task 8〜13の`DeclarationBoundary`。
3. **遮蔽と明示修飾:** parameter/local/block/callback/receiver/`::`で、後段linkerも誤接続を復活させない。Task 16〜19の`NoFalseEdgeAfterIndex`。
4. **索引間の一致:** 同名関数の別namespace追加、module提供元の変更・削除で、差分索引と全索引が同じ意味上の関係集合になる。Task 19〜21の`IncrementalParity`。
5. **公開原文契約:** Query/Expand/Impactのsource、行位置、role、既知handle、不変条件、実測wire予算が修正後も成立する。Task 21の`EvidenceContract`。

## Progress

- [x] 監査20件と再現fixture、既存の計画ポリシーを照合した。
- [x] 修正順序、内部インターフェース、受け入れ条件を計画化した。
- [ ] 実装承認後にTask 0を実施し、ここからルートPLANへ移行する。
- [ ] Task 1〜20のRED→GREENとID別コミットを完了する。
- [ ] Task 21の更新移行・全検証・独立レビュー・統合を完了する。

実行時はUTC日時、実際のコマンド、終了コード、証跡パスを追記する。未実行の検証を完了扱いにしない。

## Surprises & Discoveries

- 27の失敗条件を20問題へ集約している。CP04はinitializer/requires/function-try-blockの3条件を含む。
- CP19は未解決関係のnamespace情報不足と検索精度低下であり、CP10の誤った確定関係とは別問題である。
- rootのclassだけで修飾名が重複するCP15が他の再現を隠すため、元fixtureは名前付きnamespaceで問題を分離している。その分離を残す。
- include関係の内部状態だけでは公開機能の不具合と断定できない。監査で撤回したC18は追加対象にしない。

## Decision Log

- 2026-10-18: 修正対象をCP01〜CP20に限定。サイバーセキュリティを継続除外する。
- 2026-10-18: 共通の字句／scope修正を先に直列実施し、関係解決の修正を後に行う。
- 2026-10-18: 公開Signatureと原文は保持し、宣言・定義照合用の型キーを内部に分離する。
- 2026-10-18: namespace alias/usingは後続宣言を飲み込まないところまでを必須とし、未対応のalias展開・using集合から確定関係を推測しない。
- 2026-10-18: C++ moduleは既存のsymbol種別`module`で論理名を表し、既存`translation_unit`所有者を残す。importの到達先は提供ファイルの所有者にする。DB列やMCPフィールドを増やさない。

## Outcomes & Retrospective

未実装。完了時に各CP ID、正式テスト名、RED/GREEN証跡、コミットSHA、全検証結果、独立レビュー結果、master到達SHA、今回branch削除結果を記録する。未確認項目があれば完了へ進めない。

## Context and Orientation

主な修正箇所は`internal/extract/cpp/{lexer,parser,extractor,token}.go`。現在は字句解析結果から宣言・chunk・relationを生成する。名前解決は`internal/linker/linker.go`、未解決関係を含む関連候補検索は`internal/store/store.go`、公開受け入れは`internal/app`と`internal/mcpserver`にある。

監査fixtureは`testdata/audits/cpp-focused-20261018/{extractor_test.go,app_test.go,run.py,README.md}`。一時overlayで実行される監査テストを、通常`go test`で走る正式テストへ移す。元の期待結果を弱めず、fixture本体は文字列または`testdata/`配下に置く。実行依存としてoverlayを残さない。

正式回帰テストの主ファイルを`internal/extract/cpp/cpp20_regression_test.go`、`internal/app/cpp20_integration_test.go`とする。各IDをトップレベル`TestCPP20_CPxx`、追加条件をsubtestとする。複数packageに同じIDのテストを置いてよい。新規の共有規則は`internal/cppscope/scope.go`と`scope_test.go`、複雑なC++専用処理は後述のprivate helperファイルに分ける。

現状の未コミットMCP変更（`internal/mcpserver/server.go`、`context_v2_test.go`等）は今回の修正に含めない。実行時に正確な差分を再確認し、元checkoutでの追加検証にだけ適用する。

## Interfaces and Dependencies

以下は新設する内部契約。実装開始後に必要な変更が判明した場合、先にDecision Logと全利用箇所を更新する。

1. `cpp/call_target.go`: private `callTarget{Kind callTargetKind; Name, Qualified string; Absolute bool}`。Kindはunqualified/namespace/member。`parseCallTarget(tokens []Token, beforeParen int) (callTarget, bool)`がtemplate引数・operator名を扱う。明示namespaceとmemberの失敗時はunqualifiedへ変換しない。
2. `cpp/bindings.go`: private `bindingIndex`と`buildBindingIndex(tokens []Token, decl *declaration) bindingIndex`、`(bindingIndex).shadows(name string, atByte int) bool`。元バイト位置と有効範囲を使用し、header参照・body呼出し・callback参照が同じ判定を使う。
3. `cpp/signature.go`: `canonicalParameterKey(tokens []Token) (string, bool)`。型・pointer/reference・cv/ref・variadic・nested declaratorを保持し、引数名とdefault値を除く。失敗時は曖昧な宣言を確定ペアにしない。公開Signatureやhandle生成用の既存原文Signatureをこのキーで上書きしない。
4. `internal/cppscope`: `LookupLevels(namespace, target string, absolute bool) []string`、`Resolve(from model.Symbol, target string, symbols []model.Symbol) []model.Symbol`、`Matches(from model.Symbol, target string, anchor model.Symbol, symbols []model.Symbol) bool`。親symbol列からnamespaceと型の所属を得る。最も近い有効な宣言集合で探索を止め、曖昧なら複数候補のままとする。先頭`::`はglobal限定。member形式はleaf一致にしない。無名namespaceは同一ファイルに限定する。C++のみに適用する。
5. `cpp/modules.go`: private `moduleDirective{Kind, Name string; Start, End int}`と`parseModuleDirective(tokens []Token, start int) (moduleDirective, bool)`。named declaration/import/header-unit/global-fragment/private-fragmentを区別する。module symbolはtranslation_unitをParentHandleに持つ。linkerは一致するmodule providerが一意の場合だけ同ファイル所有者へ接続する。

共有ヘルパーはmodelだけに依存させ、cpp→linker→storeの循環依存を作らない。既存`model.Relation`に列を追加しない。receiverは`.`/`->`の識別を未解決targetにも保持し、linker/storeの共通規則が誤ったbare-name一致を拒否する。

## Plan of Work

### Task 0 — 実行開始・計画移行・正式再現の準備

**Files:** ルート`PLAN.md`、本提案、`docs/superpowers/plans/completed/2026-10-18-residual11-before-cpp20.md`、監査MD/JSON、上記監査fixture。

1. [ ] branch、HEAD、dirty/untracked、worktree、既存lockを記録する。元checkoutをreset/clean/stashしない。使用可能なmanaged worktreeを確認し、専用`codex/cpp-20-functional-fixes`を作る。ブランチが既存なら履歴を確認し、上書きしない。
2. [ ] 実装に必要な監査資料とfixtureをバイト一致で作業先へ渡す。ユーザーの他のdirty変更は持ち込まない。監査基準から変化があれば各再現の有効性を確認する。
3. [ ] 完了済みルートPLANをバイト単位でarchiveし、SHA-256一致を確認してから本計画をroot PLANとして有効化する。計画・監査証跡だけを明示stageして独立docsコミットにする。
4. [ ] 元fixtureの27失敗条件・7成功対照をIDへ対応付ける。各タスクで正式テストを先に追加してREDを保存する。先行修正によって後続条件が既に通る場合は基準HEADの隔離検証先でFAILを確認し、理由を記録する。
5. [ ] Progressへdirty基準、作業場所、実行証跡ディレクトリ、計画移行SHAを記録する。

以降の各タスクは表記順に実施する。RED/GREENコマンドは同一とし、REDが意図したassertionで失敗したことを確認する。新設package/APIがある場合はコンパイル可能な最小の型・宣言を先に用意し、package不在・未定義名をRED証拠にしない。実装後は対象package全体も実行する。共有テストファイルを累積更新するため、後続IDの未実装テストを先に追加してpackage全体を常時失敗させない。コミットは指定対象パスのみstageし、PLANのID・証跡・SHA表を更新する。

### Task 1 — CP20: UTF-8 BOM

**Files:** `internal/extract/cpp/lexer.go`、`cpp20_regression_test.go`。**Consumes:** 元source bytes。**Produces:** 先頭BOMを構文から除外した、元offset付きtoken。

1. [ ] `TestCPP20_CP20/SourceOffsets`を追加。BOM+CRLFのfirst/second、非ASCII識別子、文字列内と先頭以外のU+FEFFを検査する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP20$' -count=1`。first欠落を確認する。
3. [ ] byte0のUTF-8 BOMだけを読み飛ばす。入力全体の切り詰めやoffset補正の二重適用をしない。
4. [ ] 同コマンドでGREEN。symbol/chunkの元バイト範囲と行位置を照合する。
5. [ ] `fix(cpp): preserve declarations after UTF-8 BOM`として対象をコミットする。

### Task 2 — CP01: 数字区切り文字

**Files:** `lexer.go`、`cpp20_regression_test.go`。**Produces:** 数字literal内部のapostropheを含むtoken。

1. [ ] `TestCPP20_CP01`に`1'000`後のreal、hex/binary、exponent、隣接する本物のchar literalと`SourceOffsets`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP01$' -count=1`。後続関数欠落を確認する。
3. [ ] 数字literalの走査規則を追加し、有効な数字間のseparatorをchar開始にしない。文字列・文字literalの規則は保持する。
4. [ ] 同コマンドでGREEN。壊れたliteralでも無関係な後続宣言を過剰に飲み込まないことを確認する。
5. [ ] `fix(cpp): lex digit separators without consuming declarations`をコミットする。

### Task 3 — CP12: 継続行コメント

**Files:** `lexer.go`、`cpp20_regression_test.go`。**Produces:** escaped newlineを含む正しいline-comment範囲。

1. [ ] `TestCPP20_CP12`にLF/CRLFの`// ... \\`継続、phantom非抽出、継続終了後real抽出、`SourceOffsets`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP12$' -count=1`。phantomの誤抽出を確認する。
3. [ ] line spliceをコメント終端判定に適用し、元sourceを改変せずline/byteを追跡する。macro評価は追加しない。
4. [ ] 同コマンドでGREEN。raw string・通常string中の`//`対照も通す。
5. [ ] `fix(cpp): honor escaped newlines in line comments`をコミットする。

### Task 4 — CP13: コメント付きinclude

**Files:** `lexer.go`、`parser.go`、`cpp20_regression_test.go`。**Produces:** コメントを空白として扱うdirective解析。

1. [ ] `TestCPP20_CP13`に先頭block comment、includeとoperand間comment、quote/angle、継続行、URL/raw string、`SourceOffsets`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP13$' -count=1`。dep.hpp欠落を確認する。
3. [ ] directive開始位置とoperandをtokenから判断する。文字列をregex置換で壊さない。`include_next`は既存kindを維持し、macro operandは確定パスへ変換しない。
4. [ ] 同コマンドでGREEN。include関係と元line、コメント内の偽include非抽出を確認する。
5. [ ] `fix(cpp): parse include directives around comments`をコミットする。

### Task 5 — CP15: class修飾名の重複

**Files:** `parser.go`、`cpp20_regression_test.go`。**Produces:** namespaceと型所有者を分けた宣言context。

1. [ ] `TestCPP20_CP15`にglobal Item::run、constructor、n::Item::run、Outer::Inner::run、out-of-class定義を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP15$' -count=1`。Item::Item::runを確認する。
3. [ ] `namespaceFor`とparseScopeへのcontext引渡しを修正する。namespaceなしを空として扱い、型名を一度だけ付与する。
4. [ ] 同コマンドでGREEN。既存F42 namespace関数・F44大文字小文字の回帰も通す。
5. [ ] `fix(cpp): keep namespace and type qualification distinct`をコミットする。

### Task 6 — CP05: namespace alias/usingの終端

**Files:** `parser.go`、`cpp20_regression_test.go`。**Produces:** alias/directiveとnamespace bodyの区別。

1. [ ] `TestCPP20_CP05`にaliasとusing namespace後のlater、通常・nested namespace、文字列内namespaceを追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP05$' -count=1`。later欠落を確認する。
3. [ ] namespace構文を解析し、aliasとusingは`;`で終了させる。`;`を越えた次の`{`をbodyにしない。
4. [ ] 同コマンドでGREEN。aliasから架空のcontainerや確定callを生成しないことを確認する。
5. [ ] `fix(cpp): stop namespace directives at their terminator`をコミットする。

### Task 7 — CP06: 無名namespaceの所属

**Files:** `parser.go`、`extractor.go`、`cpp20_regression_test.go`。**Produces:** 同一ファイル・同一親で再利用される無名namespace所有者。

1. [ ] `TestCPP20_CP06`にhiddenのParentHandle、named namespace内、同一scope再open、別ファイル同名hidden、無関係な行挿入を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP06$' -count=1`。所属なしを確認する。
3. [ ] 正規化済み相対pathと親scopeから安定した内部識別名を作る（行番号をIDにしない）。表示名は無名namespaceと分かるものにする。同一scopeの再openを同じ所有者へ統合する。
4. [ ] 同コマンドでGREEN。元ソースchunkを合成名で置換せず、別ファイルのprivate scopeを混同しない。
5. [ ] `fix(cpp): retain anonymous namespace ownership`をコミットする。

### Task 8 — CP14: template型引数と本宣言

**Files:** `parser.go`、必要なら`declaration_header.go`、`cpp20_regression_test.go`。**Produces:** template headerを消費した本宣言開始位置。

1. [ ] `TestCPP20_CP14/DeclarationBoundary`にclass T/typename T、nested `>>`、non-type/template-template引数とBox::runを追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP14$' -count=1`。Box欠落・class T誤抽出を確認する。
3. [ ] balanced template headerを読むhelperを追加し、内部の型引数をscope宣言にしない。実宣言へheaderを関連付ける。
4. [ ] 同コマンドでGREEN。通常class/struct、既存template関数、比較・shift式の対照を通す。
5. [ ] `fix(cpp): separate template parameters from declarations`をコミットする。

### Task 9 — CP02: constructor名を含む範囲

**Files:** `parser.go`、`cpp20_regression_test.go`。**Consumes:** 修正済み型context。**Produces:** constructor名から始まるSignature/chunk。

1. [ ] `TestCPP20_CP02`に先頭member constructor、後続method、名前付きnamespaceとglobal class、`DeclarationBoundary`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP02$' -count=1`。Signature=`() {`の欠落を確認する。
3. [ ] constructor認識をheader棄却より先に行い、namePositionがheader開始前のparenを再採用しない。destructorとdefault/delete対照を保持する。
4. [ ] 同コマンドでGREEN。constructor名・全body・独立したlaterの原文を照合する。
5. [ ] `fix(cpp): include constructor names in declaration spans`をコミットする。

### Task 10 — CP03: ユーザー定義戻り値型

**Files:** `parser.go`、`declaration_header.go`、両正式回帰テスト。**Produces:** built-in whitelistに依存しない関数宣言判定。

1. [ ] `TestCPP20_CP03`にItem make、qualified/template/pointer/reference戻り値、QueryEvidence、`DeclarationBoundary`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp ./internal/app -run '^TestCPP20_CP03$' -count=1`。makeのsymbolとsource欠落を確認する。
3. [ ] declaration contextとbalanced declaratorから関数を認識する。変数初期化・function pointer・lambda・単なるcallを関数定義へ昇格しない。
4. [ ] 同コマンドでGREEN。int known_function対照と既存modern declarationテストを通す。
5. [ ] `fix(cpp): recognize functions with user-defined return types`をコミットする。

### Task 11 — CP07: operator宣言名

**Files:** `parser.go`、`declaration_header.go`、`cpp20_regression_test.go`。**Produces:** 正規のoperator名と実parameter listの位置。

1. [ ] `TestCPP20_CP07/DeclarationBoundary`にoperator[]/()/+/=、conversion bool、new/delete、qualified定義を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP07$' -count=1`。欠落・`+`/`operator`への誤命名を確認する。
3. [ ] operator名のtoken列とparameter listを分ける。`operator()`の最初のparenを引数と誤認しない。callable kindを既存規約にそろえる。
4. [ ] 同コマンドでGREEN。各Name/QualifiedName/Signatureと原文位置、既存operator=成功対照を確認する。
5. [ ] `fix(cpp): preserve operator and conversion function identities`をコミットする。

### Task 12 — CP18: typedef declaratorの名前

**Files:** `parser.go`、`declaration_header.go`、`cpp20_regression_test.go`。**Produces:** 基底型とalias名の区別。

1. [ ] `TestCPP20_CP18`にExisting Alias、qualified型、pointer/ref/array/function pointer、複数declarator、`DeclarationBoundary`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP18$' -count=1`。Alias欠落を確認する。
3. [ ] typedefのdeclarator位置から名前を得る。解析不能な列を最初のidentifierへ置き換えない。using/conceptの既存経路を保持する。
4. [ ] 同コマンドでGREEN。既存CallbackTypedefとfunction pointer対照を通す。
5. [ ] `fix(cpp): extract typedef names from declarators`をコミットする。

### Task 13 — CP04: 実bodyとhandlerの終端

**Files:** `parser.go`、新規`function_body.go`、`cpp20_regression_test.go`。**Consumes:** Task 8〜11のheader位置。**Produces:** 実body開始と全function終端。

1. [ ] `TestCPP20_CP04/DeclarationBoundary`にbrace initializer、requires-expression、function-try-blockの3再現と複合条件を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP04$' -count=1`。consume/x.work/recoverが範囲外になることを確認する。
3. [ ] post-parameter走査をinitializer/requires/noexcept/trailing-return/body/handlerに区別する。括弧対応を使い、最初の`{`で決定しない。80 tokenの探索打ち切りによる同種の欠落も回避する。
4. [ ] 同コマンドでGREEN。全catch、最後のbody文字、内部call、次memberの独立範囲、default/delete/宣言のみを確認する。
5. [ ] `fix(cpp): find complete function bodies and handlers`をコミットする。

### Task 14 — CP11: 宣言・定義の型キー

**Files:** 新規`signature.go`、`parser.go`、`extractor.go`、`cpp20_regression_test.go`。**Produces:** 内部canonical parameter key。

1. [ ] `TestCPP20_CP11`にvalue/renamedの宣言定義、default引数、同名overload、cv/ref、variadic、function-pointer引数を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP11$' -count=1`。必要なdeclares edge欠落を確認する。
3. [ ] `canonicalParameterKey`を実装し、declarationKeyだけに適用する。member関数の後置cv/ref qualifierも別途内部キーへ保持する。型の異なるoverloadや異なるcv/refを統合せず、曖昧な候補へ確定edgeを作らない。
4. [ ] 同コマンドでGREEN。元Signature、source bytes、public handle入力を勝手にcanonical keyで置換していないことを確認する。
5. [ ] `fix(cpp): pair declarations by parameter types`をコミットする。

### Task 15 — CP17: template呼出し

**Files:** 新規`call_target.go`、`extractor.go`、`cpp20_regression_test.go`。**Produces:** callTargetとtemplate引数を除いたcallee識別。

1. [ ] `TestCPP20_CP17`にwork<int>()、nested `>>`、namespace qualification、dependent template、比較・shiftの対照を追加する。
2. [ ] RED: `go test ./internal/extract/cpp -run '^TestCPP20_CP17$' -count=1`。call自体の欠落を確認する。
3. [ ] 既存callNameを`parseCallTarget`経由へ移す。balanced template引数を剥がし、曖昧なoverload/template依存は未解決のままとする。
4. [ ] 同コマンドでGREEN。通常call・operatorの名前、比較式からの偽call非生成を確認する。
5. [ ] `fix(cpp): recognize calls with explicit template arguments`をコミットする。

### Task 16 — CP08: parameter/local/callback遮蔽

**Files:** 新規`bindings.go`、`extractor.go`、両正式回帰テスト、`internal/linker/cpp20_test.go`。**Produces:** 有効範囲付きbindingIndex。

1. [ ] `TestCPP20_CP08/NoFalseEdgeAfterIndex`にfunction-pointer parameter、Callback handler、local/block/catch/for/lambda parameterとcapture、scope外のglobal成功対照を追加する。
2. [ ] RED: `go test ./internal/extract/cpp ./internal/linker ./internal/app -run '^TestCPP20_CP08$' -count=1`。global calls/header references/callback referencesへの誤接続を確認する。
3. [ ] header/bodyのbindingを一度作り、3経路で共有する。遮蔽された動的calleeはglobal leafの未解決関係も残さない。捕捉・可視範囲の解析が曖昧なら確定edgeを作らない。
4. [ ] 同コマンドでGREEN。full index後にも誤edgeが復活せず、scope終了後の非遮蔽callは解決することを確認する。
5. [ ] `fix(cpp): respect local bindings across relation extraction`をコミットする。

### Task 17 — CP09: receiver付きcallの分類

**Files:** `call_target.go`、`extractor.go`、新規`internal/cppscope/scope.go`と`scope_test.go`、`linker.go`、両正式回帰テスト。**Produces:** member形式を保持した未解決target。

1. [ ] `TestCPP20_CP09/NoFalseEdgeAfterIndex`にobject.target、pointer->target、this->target、同名global、別class同名methodを追加する。
2. [ ] RED: `go test ./internal/extract/cpp ./internal/cppscope ./internal/linker ./internal/app -run '^TestCPP20_CP09$' -count=1`。global targetへの誤edgeを確認する。
3. [ ] memberをnamespace修飾と区別する。確実な同一class/this以外はreceiver形式のまま未解決とし、linkerでleaf fallbackを禁止する。型推論やvirtual dispatchは追加しない。
4. [ ] 同コマンドでGREEN。展開でもglobalのcallerに混入せず、既存F44のcase-sensitiveなreceiver対照を通す。
5. [ ] `fix(cpp): avoid global fallback for member calls`をコミットする。

### Task 18 — CP10: 明示namespace修飾call

**Files:** `call_target.go`、`extractor.go`、`internal/cppscope/*`、`internal/linker/cpp20_test.go`、両正式回帰テスト。**Produces:** absolute/relative namespace探索規則。

1. [ ] `TestCPP20_CP10/NoFalseEdgeAfterIndex`にexternal::work対other::work、先頭`::`、nested relative scope、成功対照を追加する。
2. [ ] RED: `go test ./internal/extract/cpp ./internal/cppscope ./internal/linker ./internal/app -run '^TestCPP20_CP10$' -count=1`。誤った確定edgeとExpand callers混入を確認する。
3. [ ] LookupLevels/Resolveを実装し、現在scopeから外側へC++の探索順を適用する。明示修飾の失敗をbare leafへ落とさない。未対応namespace aliasから推測しない。
4. [ ] 同コマンドでGREEN。extractorだけでなく実DB・差分更新・旧形式/Evidence Expandでwrong callerが返らないことを確認する。
5. [ ] `fix(cpp): resolve explicitly qualified calls within their scope`をコミットする。

### Task 19 — CP19: 別ファイル関係のnamespace保持

**Files:** `internal/cppscope/*`、`extractor.go`、`internal/linker/{linker.go,cpp20_test.go}`、`internal/store/{store.go,cpp20_test.go}`、`cpp20_integration_test.go`。**Produces:** linkingと関連候補検索で一致するC++探索規則。

1. [ ] `TestCPP20_CP19`にn::Target/n::Caller、m::Target追加、`IncrementalParity`、`NoFalseEdgeAfterIndex`、未解決の曖昧対照を追加する。
2. [ ] RED: `go test ./internal/cppscope ./internal/linker ./internal/store ./internal/app -run '^TestCPP20_CP19$' -count=1`。精度低下とm::Target callers混入を確認する。
3. [ ] linkerで呼出し元ParentHandle/contextを使った共通Resolveを適用する。storeのC++未解決call候補にも共通Matchesを適用し、汎用leaf照合で拾った候補を無条件に採用しない。
4. [ ] 同コマンドでGREEN。nの正関係維持、mへの非混入、追加・削除・namespace変更の差分/全索引一致、path/変更行条件と候補上限を検査する。
5. [ ] `fix(cpp): preserve namespace lookup across linking and expansion`をコミットする。

store実装ではC++未解決callの候補をscope判定**前にLIMITしない**。既存確定edge・他言語のSQL経路は維持する。対象C++未解決edgeをsource/target symbol情報とともに取得し、共通Matchesで評価してから既存候補と重複排除・順位統合する。要求内でsymbol集合を再利用し、edgeごとのDB照会を避ける。大量同名候補で正候補が上限の後方にある回帰を置く。scope情報不足なら曖昧なままとし、別namespaceの確定callerとして扱わない。

### Task 20 — CP16: module targetとimports展開

**Files:** 新規`modules.go`、`parser.go`、`extractor.go`、`internal/linker/{linker.go,cpp20_test.go}`、両正式回帰テスト。**Produces:** 論理module symbolと提供元translation_unitへのimport関係。

1. [ ] `TestCPP20_CP16`にmodule宣言/import、論理名とfilenameの不一致、partition/header-unit、global/private fragment、重複providerと`IncrementalParity`を追加する。
2. [ ] RED: `go test ./internal/extract/cpp ./internal/linker ./internal/app -run '^TestCPP20_CP16$' -count=1`。targetが行全体になることとExpand importsで提供元が欠落することを確認する。
3. [ ] parseModuleDirectiveで論理名を正規化する。宣言からimportsを生成しない。論理名のmodule symbolを所有者配下に作り、linkerのC++専用経路で一意なproviderのtranslation_unitへ接続する。
4. [ ] 同コマンドでGREEN。`import :part`を現在moduleから正規化し、literal header-unitは既存パス規則を使う。provider変更・削除・重複時に古い確定edgeを残さない。普通のidentifier module/importやstringをdirectiveにしない。
5. [ ] `fix(cpp): link named module imports to provider files`をコミットする。

module symbolには所有者へ戻るParentHandleと元宣言範囲を持たせ、必要な宣言chunkだけを既存source規則で作る。translation_unitのName/QualifiedName=pathを変更しない。複数providerや未知のpartitionは未解決とし、同名namespace/functionへ接続しない。提供元sourceを返す際の重複とwire予算をTask 21で検査する。

### Task 21 — 索引移行・全検証・独立レビュー・統合

**Files:** `internal/indexer/indexer.go`、新規`internal/app/cpp20_refresh_test.go`、`cpp20_integration_test.go`、必要な既存Evidence/wireテスト、`docs/design.md`、ルートPLAN。本タスクにCP IDを追加しない。

1. [ ] extractor version移行テストを先に追加する。RED: `go test ./internal/app -run '^TestCPP20_(Refresh|EvidenceContract)$' -count=1`。v19の古い誤関係を保持した索引から通常更新だけでは修正が反映されない条件を確認する。その後versionを`extractors-v20-cpp-functional`へ一度更新し、同コマンドでGREENにする。通常更新で再抽出→誤edge除去→次回無変更の順を検査する。
2. [ ] `EvidenceContract`でQuery/Expand/Impactの旧形式とEvidence、必要なMCP実wire経路を検証する。custom return、constructor/body/operator、template、moduleのsource bytes/行、role、既知handle、省略・予算・不変条件を確認する。既存wire helperを使い実際のJSON byte数を計測する。no-update、共有read leaseとrevision整合性の既存テストを通す。ユーザーdirtyのserver.go/context_v2_test.goを修正・取り込まない。
3. [ ] 下記の全検証を完了し、ID別20行の対応表を埋める。新規コードを変更したら影響範囲と必要な全体検証を再実施する。製品コード・テスト・設計文書を対象別に明示stageしてコミットする。
4. [ ] 独立reviewerに最終diff、20件表、RED/GREEN、全検証結果とReview Focusを渡す。対象は機能・回帰・source契約で、セキュリティ監査を依頼しない。指摘を修正し、該当テストと必要な全体検証、再レビューを通す。未解決の機能指摘がある間はマージしない。
5. [ ] 完了記録を含む作業HEADで全gateを確認し、masterが作業branchの祖先であることを確認して`--ff-only`でマージする。masterが進んでいれば履歴を上書きせず取り込み・再検証する。master到達を確認後、今回branchだけを削除しmanaged worktreeをarchiveする。元checkoutのdirty内容・ハッシュを照合して保持を確認する。

### ID・テスト・コミット対応表（実行時に埋める）

| ID | Task | 正式テストの先頭名 | RED/GREEN証跡 | 修正コミット |
|---|---:|---|---|---|
| CP01 | 2 | TestCPP20_CP01 | 未実行 | 未実装 |
| CP02 | 9 | TestCPP20_CP02 | 未実行 | 未実装 |
| CP03 | 10 | TestCPP20_CP03 | 未実行 | 未実装 |
| CP04 | 13 | TestCPP20_CP04 | 未実行 | 未実装 |
| CP05 | 6 | TestCPP20_CP05 | 未実行 | 未実装 |
| CP06 | 7 | TestCPP20_CP06 | 未実行 | 未実装 |
| CP07 | 11 | TestCPP20_CP07 | 未実行 | 未実装 |
| CP08 | 16 | TestCPP20_CP08 | 未実行 | 未実装 |
| CP09 | 17 | TestCPP20_CP09 | 未実行 | 未実装 |
| CP10 | 18 | TestCPP20_CP10 | 未実行 | 未実装 |
| CP11 | 14 | TestCPP20_CP11 | 未実行 | 未実装 |
| CP12 | 3 | TestCPP20_CP12 | 未実行 | 未実装 |
| CP13 | 4 | TestCPP20_CP13 | 未実行 | 未実装 |
| CP14 | 8 | TestCPP20_CP14 | 未実行 | 未実装 |
| CP15 | 5 | TestCPP20_CP15 | 未実行 | 未実装 |
| CP16 | 20 | TestCPP20_CP16 | 未実行 | 未実装 |
| CP17 | 15 | TestCPP20_CP17 | 未実行 | 未実装 |
| CP18 | 12 | TestCPP20_CP18 | 未実行 | 未実装 |
| CP19 | 19 | TestCPP20_CP19 | 未実行 | 未実装 |
| CP20 | 1 | TestCPP20_CP20 | 未実行 | 未実装 |

## Validation and Acceptance

### 1. ID別受け入れ

監査の27失敗条件を正式テストでGREENにし、7成功対照も維持する。symbol件数だけでなく名前・所有者・Signature・原文範囲・正関係の存在・誤関係の不在を確認する。各タスクの正例／負例を同じ入力集合に置き、元fixtureのassertionを単なる「未解決でも可」に弱めない。

### 2. 関係・検索の受け入れ

CP03はQueryEvidence、CP09/10/19は実索引後のRelatedCandidateHitsと旧形式/Evidence Expand callers、CP16はExpand importsを必須にする。CP08はheader/body/callbackの全経路を検査する。CP04はbody内のcallも検査する。CP19とmodule変更はfull/incrementalのhandle依存でない意味上の関係集合を比較する。Impactでは変更したC++の現行原文とdependentが得られ、削除ファイルの原文を捏造しない。

### 3. 全体検証（通常テストとraceは直列）

Windowsでshell環境を記録し、リポジトリ規約に従って実行コマンドを`rtk`経由にする。以下の表示はGo側のコマンド本体。

```text
go test ./...
go test -race ./...
go vet ./...
go build -o <task-temp>/focalspan-native.exe ./cmd/focalspan
```

次に`CGO_ENABLED=0`で下表をビルドする。環境変数は検証用子プロセスに限定する。出力はtask-tempに置き、稼働バイナリを上書きしない。

| GOOS | GOARCH | 出力例 |
|---|---|---|
| windows | amd64 | focalspan-windows-amd64.exe |
| linux | amd64 | focalspan-linux-amd64 |
| darwin | arm64 | focalspan-darwin-arm64 |

benchmark/evalの既存cmdも一覧を確認し、native build対象に含める（該当実行ファイルの`go build -o <task-temp>/... ./cmd/...`）。package restoreやネットワークを発生させず、依存が不足する場合は不足gateとして記録し、成功とみなさない。

実symlink等で既存通常テストが権限不足になる場合は、利用可能なWSL-Ubuntuで該当gateと必要な全体検証を実行し、Windowsの実行結果と区別して記録する。不要な権限変更・セキュリティ作業には広げない。利用不能なgateをskipに変えてマージしない。

### 4. dirty overlay・独立レビュー

clean作業branchで全検証を完了した後、元checkoutの既存dirty MCP差分を保持した状態でも影響packageのテストを行う。必要なら別の一時検証先に元差分のバイト一致コピーを適用する。コミットにユーザー差分を混ぜない。clean合格とdirty合格を別の証跡として記録する。

レビューでは特にraw source fidelity、shared helperとSQL候補上限の順序、他言語への非波及、module所有者、未知入力の誤確定を確認する。レビュー担当の指摘と対処、最終判定を保存する。

### 5. 計画・完了記録

`git diff --check`、CP01〜CP20の重複／欠落、正式テストの実在、全検証結果、コミット対象、master ancestry、branch削除、元dirty保存を照合する。実装計画に沿った修正を終えるまでは「20件完了」と記録しない。

## Idempotence and Recovery

1. 再開時はroot PLANのProgressと最後のcommit、実際のdirtyを照合する。既存テスト・branch・archiveを上書きせず、成功済みREDの証跡を再利用する。
2. テストは一時DB/Git repositoryだけを使う。稼働`.focalspan/index.db`を再構築しない。CP version更新による実環境再抽出は将来そのバイナリを利用した時の通常動作であり、今回の検証でlive DBへ適用しない。
3. archiveは元PLANと一致しなければ計画移行を止めて原因を確認する。未知のlockを削除しない。競合は元編集を保持して解消する。
4. 統合時に元checkoutのuntracked監査資料／本提案と追跡ファイルが衝突する場合は、内容比較とバックアップを先に行う。必要な一時renameは可逆にし、差分を失わず復元する。元dirtyへマージ結果を反映できない場合はforceせず統合gateを未完了として報告する。
5. FFできない場合はmasterの新しい変更を取り込み、影響検証とレビューを更新する。履歴の書換えや無関係branch削除をしない。全gate通過前のマージ・push・再配置は行わない。

## 実行前セルフレビュー

- [x] CP01〜CP20が1回ずつタスクと対応表に現れ、27条件と7対照の継承を明記した。
- [x] 5つのReview Focusを所有タスクの名前付きテストへ割り当てた。
- [x] 共有インターフェース、ファイル責務、RED/GREENコマンド、段階コミット、索引移行を指定した。
- [x] 公開形式・原文・予算・未解決関係・full/incremental・dirty保存の受け入れを指定した。
- [x] セキュリティ除外、全検証、独立レビュー、FF統合と今回branchだけの削除を明記した。

Execution 2026-10-18T07:38:53.064383+00:00: Task 1 GREEN, commit 4dc49b9f34797fe366fcf1b70228d3ce449e1e66. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:39:01.281435+00:00: Task 2 GREEN, commit 2c66c11cde11474f06f521bcc4387f69bab41ed5. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:39:14.239122+00:00: Task 3 GREEN, commit 46fc88b30c3685963ad8350694d9cf927d2b2ba6. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:39:33.216426+00:00: Task 4 GREEN, commit 75567eaf258f192a4b29be8e89b9dab29977a04c. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:40:21.197037+00:00: Task 5 GREEN, commit abf22c50d1ed15901d3300c41e88640aaa4f55e2. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:40:29.237467+00:00: Task 6 GREEN, commit d4378a986b51a3c42a5bd0d4400c3bea05f87637. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:41:33.231971+00:00: Task 7 GREEN, commit e58b4e1f6f592b209beded9e244ee805d8162784. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:41:41.074193+00:00: Task 8 GREEN, commit 70abc8a0720059a82d185db6400ec1dec71c2dd0. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:41:48.588526+00:00: Task 9 GREEN, commit 5e362a9cd1dd627aaf7da4400e2dcebd9f29a852. See C:/Temp/cpp20-execution for command output.

Execution 2026-10-18T07:41:56.251850+00:00: Task 10 GREEN, commit 949fd97d470724ac195a3c195d4a8e8bec35cac3. See C:/Temp/cpp20-execution for command output.

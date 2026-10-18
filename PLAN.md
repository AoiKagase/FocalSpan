# 残存11件の機能・整合性修正 Implementation Plan

> **For agentic workers:** `superpowers:executing-plans`で順次実装する。各問題の正式な回帰テストでRED→GREENを確認する。実装の並列化は前提にせず、最終の独立レビューは実装担当とは別のレビュー担当で行う。

**Goal:** 監査R01〜R11の誤接続・依存欠落・範囲誤認・評価誤診断・統計不整合を修正する。

**Architecture:** 既存の抽出器、metadata facts、linker、範囲付き検索、共有読取りロック、Evidence compilerを拡張する。動的な束縛から確定関係を推測せず、ファイル依存と名前付きシンボルを区別する。Impactは現在の索引の構文関係を1段探索する。

**Tech Stack:** Go 1.26以上、既存SQLite・MCP SDK・各言語の静的抽出器。新規依存なし。

**Spec:** `docs/audits/2026-10-18-residual-functional-audit.md`。R01〜R11のID・再現条件を維持する。

**状態:** 実装中。唯一のactive ExecPlanは本書（ルート`PLAN.md`）。前PLANをcompleted/2026-10-18-audit48-before-residual11.mdへbyte-for-byte保存した。

## Global Constraints

- セキュリティ監査・修正・新規の再現検証は対象外。以前除外したF02/F03、依存脆弱性評価、原因未確定のsupervisor EOFを含めない。今回のR01〜R11は機能・索引整合性の問題として扱う。
- MCP公開フィールド、Evidence v1/v2、DBスキーマを維持する。原文bytes・行位置・非重複・実測wire予算・MCP stdoutのプロトコル専用性を守る。
- 新規依存、ネットワーク、外部LLM、解析対象コードの実行、package restoreを追加しない。既存のローカルGo依存とGit fixtureを使い、Go検証では`GOPROXY=off`・`GOSUMDB=off`とする。
- 既存の未コミット変更・設定・索引・他セッションの作業を保持する。専用の作業場所で実装し、対象パス／対象hunkだけをstageする。pushと稼働バイナリ置換は含めない。
- 現在の共有／排他ロック、5秒待機、キャンセル、BusyError、未完了索引の検索拒否を維持する。通常テストとrace検査は直列で実行する。

## Review Focus

- 局所束縛を除外した呼出しが、後段linkerで同名大域関数へ再接続されないこと（R01）。抽出器だけでなく実索引の回帰テストを持つ。
- aliasを持たない内側の設定も境界となり、外側・隣接プロジェクトのaliasを誤って継承しないこと（R04）。設定の追加・削除の増分更新も検証する。
- 同じ対象ファイルにmodule・関数・型が同居しても、ファイル依存の解決と名前付きシンボルの解決を混同しないこと（R05）。
- 非変更ファイルをImpactへ追加する際も現在の本文を検証し、複数アンカー・循環・予算不足で変更箇所やprovenanceを失わないこと（R09）。
- 再抽出・索引済み判定・内容変更件数を分離し、候補上限や更新経路による評価・統計の変化をなくすこと（R10/R11）。

## Purpose / Big Picture

前回の修正後の`master@66d8ea87916cdda57d0d3c0ecd8fd26dfc91e07a`と既存の未コミット変更を含む監査で、11件を再現した。修正後は偽の宣言・関係を減らし、通常の相対import、Impactの依存元・関連テスト、評価の索引済み判定を正しく返す。件数を合わせるための新規項目追加や、除外済み項目の再導入はしない。

## Progress

- [x] R01〜R11の監査根拠・現在の関連コード・既存の契約を照合し、修正方針と受け入れ条件を定義した。
- [x] 実装開始時のHEAD・差分を再確認し、専用ブランチ／作業場所を準備してPLANを切り替える。
- [x] 下記の順序で11件を正式回帰テストへ移し、RED→GREENと問題別コミットを記録する。
- [ ] 全通常テスト→全race検査、vet、ビルド、OS確認、独立レビューを完了する。
- [ ] 検証済み変更をmasterへfast-forward統合し、今回のブランチだけを削除して完了を記録する。

実行開始後はUTC時刻、対象ID、RED/GREENコマンドとログ、commit SHA、レビュー結果をこのProgressに対応付ける。計画時点では修正・検証の成功を記録しない。

## Surprises & Discoveries

R04のalias誤適用は、対象ファイルにmodule以外の宣言があるとR05の解決不能によって隠れる。両方を単一fixtureで確認せず、moduleだけの対照と宣言付きの対照を分ける。

R01は抽出時のToHandleだけを空にしても不十分である。同名のUnresolvedToを残せば後段linkerが確定接続できるため、局所束縛による動的呼出しを同名大域関数の候補として登録しない。

Impactの現行Evidence分類は変更候補を`change`、関係候補を`dependent`とする。R09でもこの既存分類を維持し、test由来はRelationContext／既存whyコードで保持する。新たな公開roleやフィールドを増やさない。

## Decision Log / 確定方針

| 論点 | 決定 |
|---|---|
| R01の動的な局所呼出し | 大域関数への確定・名前だけの候補関係を生成しない。解釈できない束縛は保守的に扱い、解析対象コードを実行しない。 |
| R02のモジュール式 | 引数が単独の静的文字列と確認できる場合だけモジュール候補を生成する。連結・変数・式から部分文字列を抽出しない。無置換templateは既存lexerが静的値として安全に取得できる場合に限る。 |
| R04の設定選択 | 呼出し元を含む最も近い祖先ディレクトリのtsconfig/jsconfigを選ぶ。同じ階層ではtsconfigを優先する。設定にaliasがなくても外側設定へのfallbackはしない。`extends`、project references、include/excludeによる完全なTypeScript project解決は追加しない。 |
| R05の解決単位 | JS/TSのファイル依存では一致した単一ファイルのmodule所有者を選ぶ。名前付きsymbol関係や他言語へ無条件に適用しない。候補ファイルが複数なら未解決のままにする。 |
| R09の契約 | 説明を縮小するだけの対応は採らず、変更シンボルから既存の構文関係を1段探索して依存元・関連テストを追加する。推移閉包・実行時意味解析は追加しない。 |
| R10の索引済み判定 | retrieval用の候補上限から切り離し、パス・シンボル・kindをDB上で直接判定する。レポートschemaと既存指標の定義は維持する。 |
| R11の統計 | added/changed/unchangedはソースの存在とハッシュによる分類を維持する。関係再構築のための再抽出回数を加算しない。 |
| 既存索引の移行 | 全抽出修正を含む共有のextractor version更新を1コミットで行う。既存索引は次の正規更新で再抽出される。DB migrationは追加しない。 |

## Context and Orientation

抽出結果のSymbols/Chunks/Relationsは`internal/extract/*`から`internal/indexer`へ渡り、`internal/store`へ保存され、`internal/linker`が未解決関係を解決する。metadata factsは`internal/projectmeta`が走査済みファイルから作り、manifest revisionが変わると再抽出・再リンクされる。

`internal/app/evidence.go`の`impactCandidates`は旧形式ImpactとEvidence形式Impactの共通取得経路である。`internal/app/read.go`が取得から最終リビジョンまで共有ロックを保持する。R09はこの内側で行い、内部ヘルパーから共有ロックを重複取得しない。

一時再現の入口は`C:/Temp/fs-residual-audit-20261018.py`、証拠は`C:/Temp/fs-residual-audit-20261018/`。正式テストの実行依存にはしない。fixtureとassertionをリポジトリへ移し、外部一時ファイルがなくても再現できる状態にする。

## Plan of Work

実装順序はR01→R02→R03→R05→R04→R06→R07→R08→R10→R11→R09。共有linker処理はR05を先に整え、R04とR09は前提となる関係の精度を確認してから実装する。

各Taskの5段階を順に完了する。テスト名は固定し、既存の`audit50_test.go`を無関係に書き換えず、新しい回帰テストファイルへ追加する。複数IDの共有変更はcommitの責任範囲と依存関係を明記する。

### 準備 — 作業場所とPLANの切替

- [x] HEAD・master・既存差分・添付worktreeを確認し、既存ユーザーファイルのハッシュ／差分を記録する。計画時点のHEADを無条件に現在値と見なさない。
- [x] `codex/residual-11-functional-fixes`をmasterから作り、管理された隔離worktreeで実装する。既存変更をreset・clean・stashしない。
- [x] 現行の完了済み`PLAN.md`を`docs/superpowers/plans/completed/2026-10-18-audit48-before-residual11.md`へbyte-for-byte保存し、SHA256一致を確認する。同名ファイルが存在したら上書きしない。
- [x] 承認された本計画をルート`PLAN.md`へ切り替え、監査報告を含む文書差分だけでPLAN切替をコミットする。候補ファイルは後続の編集元にせず、以後の進捗はルートPLANだけに記録する。
- [x] 再現fixtureを`testdata/regressions/residual11/`、回帰テストを各packageの`residual11_test.go`へ移す。各IDの修正前FAILはそのIDの着手時に記録する。

### Task R01 — JS/TSの束縛と呼出し先

**Files:** `internal/extract/jsts/extractor.go`、`parser.go`、新規`bindings.go`、`internal/extract/jsts/residual11_test.go`、`internal/app/residual11_test.go`。

**Interfaces:** Tokenのbyte位置とdeclarationの親子範囲から内部`bindingIndex`を作る。呼出し位置の名前が局所の動的束縛か、宣言された関数か、未束縛かを判定し、`resolveCall`と関係生成の両方で同じ判定を使う。MCP/model/DBへのフィールド追加はしない。

- [x] `TestR01ShadowedCalls`を追加する。引数・default引数・destructuring・const/let/var・catch引数・内側ブロック・nested closureで同名の大域関数へ接続しないことを検査する。局所のobject引数への`obj.method()`も同名の無関係な関数へ接続しない。束縛範囲外、通常の局所関数宣言、宣言として認識できる局所arrow/function expressionは正しい対象へ接続する。
- [x] `rtk proxy go test ./internal/extract/jsts -run TestR01 -count=1`でREDを記録する。実索引を通す`TestR01ShadowedCallsStayUnlinked`も追加し、後段linkerで誤接続が復活することを防ぐ。
- [x] 束縛のスコープと宣言種別を判定し、動的な局所呼出しの同名関係は登録しない。varの関数スコープ、let/constのブロックスコープ、引数・型注釈・分割代入の区別を扱い、別関数の束縛を漏らさない。引数名等の宣言位置を型参照として同名大域関数へ接続せず、型注釈にある真の参照は保持する。
- [x] `rtk proxy go test ./internal/extract/jsts ./internal/app -run TestR01 -count=1`でGREENを確認する。親子宣言、thisによるメソッド、import aliasの既存回帰も対象packageで検証する。
- [x] 対象hunkだけをstageし、R01のテスト・修正・ログ対応を記録してコミットする。

### Task R02 — 静的なモジュール引数だけを認識する

**Files:** `internal/extract/jsts/parser.go`、`internal/extract/jsts/residual11_test.go`、JS/TS fixture。

**Interfaces:** 内部`staticModuleArgument(tokens []Token, significant []int) (string, bool)`を追加する。入力は呼出し括弧の内側のsignificant token列とし、第一引数が単独の静的文字列の場合だけ値を返す。`import()`と既存`require()`のモジュール抽出で共有する。

- [x] `TestR02StaticModuleArguments`を追加する。変数、連結、関数呼出し、置換付きtemplateから部分文字列を採用しない。単純な文字列、コメント、改行、importの第二引数options、末尾カンマを対照にする。
- [x] `rtk proxy go test ./internal/extract/jsts -run TestR02 -count=1`でREDを記録する。
- [x] `firstString`による式内探索を静的引数判定に置き換える。第二引数optionsの文字列をモジュール名として採用しない。実行時の式評価は行わない。
- [x] 同コマンドでGREENを確認し、import／requireの既存回帰を実行する。
- [x] R02を対象パス限定でコミットする。

### Task R03 — exportの値と再export先を区別する

**Files:** `internal/extract/jsts/parser.go`、`internal/extract/jsts/residual11_test.go`、JS/TS fixture。

**Interfaces:** `parseModules`でモジュール指定として扱うのはexport構文の`from`直後の静的文字列だけとする。既存のexport-symbol関係は宣言名・aliasから生成する。関数body等の内部文字列を外部モジュールに変換しない。

- [x] `TestR03ExportValueIsNotModule`を追加する。constの文字列、文字列union型、default値、関数内の文字列からmodule関係を作らず、`export * from`、`export {x as y} from`、ローカルの`export {x}`は保持する。export関数内の真のdynamic importも検査する。
- [x] `rtk proxy go test ./internal/extract/jsts -run TestR03 -count=1`でREDを記録する。
- [x] 文全体のstringLiterals列挙を構文別の読取りに置き換える。export宣言を読み飛ばして内部のimport/requireを取りこぼさないように、モジュール走査位置も修正する。
- [x] 同コマンドでGREENを確認し、R02および前回F34の改行・セミコロン省略・文字列名importの回帰を再検証する。
- [x] R03を対象パス限定でコミットする。

### Task R05 — ファイルimportのmodule所有者を選ぶ

**Files:** `internal/linker/linker.go`、`internal/linker/residual11_test.go`、`internal/app/residual11_test.go`、TS依存fixture。

**Interfaces:** 内部`preferModuleOwner(from model.Symbol, relation model.Relation, candidates []model.Symbol) []model.Symbol`を追加し、`LinkWithScope`の最終一意性判定前に使う。SourceでJS/TSのモジュール関係とexport/import-symbol関係を区別する。

- [x] `TestR05ModuleOwnerResolution`と`TestR05RelativeImportIntegration`を追加する。参照先にmodule＋複数の関数／型があってもmoduleへ接続し、宣言なしの対照も維持する。相対パス、拡張子省略、index.ts、require、reexport、名前付きimport、複数ファイルの曖昧性を検査する。
- [x] `rtk proxy go test ./internal/linker ./internal/app -run TestR05 -count=1`でREDを記録する。
- [x] JS/TSのファイル依存で候補が単一の実ファイルを指す場合だけ、そのファイルの一意なmodule所有者へ絞る。名前付きsymbol、calls/tests、他言語の解決をこの規則で置換しない。module所有者不在や複数ファイルでは根拠なく1件を選ばない。
- [x] 同コマンドでGREENを確認し、全リンクと増分リンクの結果を比較する。前回F15/F17/F18/F21の回帰も維持する。
- [x] R05をコミットし、R04/R09がこのmodule選択に依存することをPLANへ記録する。

### Task R04 — aliasの所属設定を限定する

**Files:** `internal/projectmeta/discover.go`、新規`internal/projectmeta/scope.go`、`internal/linker/linker.go`、両packageとappの`residual11_test.go`。

**Interfaces:** 走査されたtsconfig/jsconfigごとに内部Factの`Kind="project-config"`を生成する。空の設定にもSourcePathを保持し、不正JSONの場合も境界の存在は保持して既存の解析診断を返す。`projectmeta.ApplicablePathAliases(sourcePath string, facts []Fact) []Fact`を追加し、最も近い設定のpath-aliasだけを返す。Fact構造とDBスキーマは変更しない。

- [x] `TestR04ApplicableAliases`と`TestR04AliasScopeIntegration`を追加する。隣接A/B、祖先設定、より近い設定、aliasなし／不正JSONの内側設定、同階層tsconfig/jsconfig、設定の追加／削除／alias変更を検査する。対象TSファイルはmoduleだけ／宣言付きの両方を用意する。
- [x] `rtk proxy go test ./internal/projectmeta ./internal/linker ./internal/app -run TestR04 -count=1`でREDを記録する。
- [x] alias候補化と最終解決の両方に同じ適用設定の判定を使う。設定のパスとbaseUrlによる既存のターゲット正規化、完全一致・wildcardを維持する。無関係なproject-config factをリンク先と解釈しない。新Factは既存manifest revisionに反映する。
- [x] 同コマンドでGREENを確認する。設定だけを変更した増分索引と新規全索引の関係集合が一致し、通常のGo/Composer等のfactsへ影響しないことを検査する。
- [x] R04を対象パス限定でコミットする。

### Task R06 — Ruby文字列内の宣言・呼出しを除外する

**Files:** `internal/extract/ruby/extractor.go`、必要な場合のみ`lexer.go`、`internal/extract/ruby/residual11_test.go`、Ruby fixture。

**Interfaces:** lexerのTokenのbyte範囲を利用して構造解析用のcode viewを作る。改行・byte長を保ち、通常String等の本文を構造解析から除く。require・test DSL等で必要な値は元Tokenまたは原文から取得し、Evidence用の原文は変更しない。

- [x] `TestR06RubyStringRegions`を追加する。単／二重引用符、複数行、escape、percent literal、heredocの中のdef/class/end/呼出しを除外する。文字列外の宣言、通常require、test DSL、改行・byte位置を対照にする。
- [x] `rtk proxy go test ./internal/extract/ruby -run TestR06 -count=1`でREDを記録する。
- [x] 行ベースparserへ渡すcode viewと値抽出用の原文を分離する。文字列を空白化するだけでrequireやtest名まで消す修正にはしない。補間式を解析できない場合、本文から確定関係を推測しない。
- [x] 同コマンドでGREENを確認する。前回heredoc修正と共通の原文・行位置・byte範囲の回帰を実行する。
- [x] R06をコミットする。

### Task R07 — Ruby endless methodの終端

**Files:** `internal/extract/ruby/extractor.go`、`internal/extract/ruby/residual11_test.go`、Ruby fixture。

**Interfaces:** 宣言headerの引数括弧・token境界から、引数のdefault値や`foo=`というsetter名とendless bodyの`=`を区別する。endless bodyは1つの論理式の末尾までとし、通常defのend待ちstackへ積まない。

- [x] `TestR07RubyEndlessMethods`を追加する。単純な式、引数付き、self.method、次の独立宣言、括弧／配列やbegin/endで継続する複数行式を検査する。通常defのdefault引数、setter名、比較演算子を誤認しないことも確認する。
- [x] `rtk proxy go test ./internal/extract/ruby -run TestR07 -count=1`でREDを記録する。
- [x] endless methodの宣言終端・親子関係・修飾名を論理式に合わせて計算する。後続メソッドのendを使って閉じない。曖昧な未完了式から後続宣言を確定的に子へ分類しない。
- [x] 同コマンドとR06回帰でGREENを確認し、原文の連続slice・行位置を検査する。
- [x] R07をコミットする。

### Task R08 — Python simple statementの分割

**Files:** `internal/extract/python/imports.go`、必要な場合のみ`code_view.go`、`internal/extract/python/residual11_test.go`、Python fixture。

**Interfaces:** `pythonImportRecords`の返却型`map[int][]string`を維持する。同一物理行の複数importを同じ行のrecordsへ追加し、文字列／コメントと括弧内を除くセミコロン境界でsimple statementを分割する。import後の実行文をモジュール名へ混ぜない。

- [x] `TestR08SemicolonImports`を追加する。`import os; import sys`、from/import alias、カンマ形式、後続の代入／呼出し、`if flag: import os; import sys`の単純suite、文字列・コメント中の`;`、既存の括弧付き複数行とbackslash継続を検査する。
- [x] `rtk proxy go test ./internal/extract/python -run TestR08 -count=1`でREDを記録する。
- [x] マスク済みcode viewとtoken境界を使ってsimple statementを分割し、各importを既存のrecordsへ追加する。各statementでownerや行番号を取り直して誤ったsource範囲を生成しない。
- [x] 同コマンドでGREENを確認する。複数import・scope lifetime・遮蔽・原文位置の既存回帰も実行する。
- [x] R08をコミットする。

### Task R10 — 評価のidentity判定を直接取得へ切り替える

**Files:** `internal/benchmark/engine.go`、新規`internal/store/identity.go`、`internal/store/residual11_test.go`、`internal/benchmark/residual11_test.go`。

**Interfaces:** Storeへ`SymbolsForIdentity(ctx context.Context, path, name, kind string) ([]model.Symbol, error)`を追加する。SQLでパス・名前・指定kindを条件化し、retrieval候補上限を使わず決定的順序で返す。pathだけの存在確認には既存`FileHash`のfoundを用い、benchmark型をStoreから参照しない。

- [x] `TestR10AttributionIdentityLookup`を追加する。同名502件の末尾、path-only、索引済みの空ファイル、同一ファイルの複数chunk、kind不一致、大文字小文字、真の未索引を検査する。`AttributeLabels`まで通して未索引／retrieval missingを区別する。
- [x] `rtk proxy go test ./internal/store ./internal/benchmark -run TestR10 -count=1`でREDを記録する。
- [x] `AttributionIdentities`の上限500の候補検索を直接identity取得へ置き換える。複数chunkによる重複はidentityでまとめ、schemaや既存のTargetRank/MRR定義を変更しない。
- [x] 同コマンドでGREENを確認する。benchmarkのattribution・required_symbols・report比較の既存テストを実行する。
- [x] R10をコミットする。

### Task R11 — 内容変更件数と再抽出回数を分離する

**Files:** `internal/indexer/indexer.go`、`internal/indexer/residual11_test.go`、`internal/app/residual11_test.go`。

**Interfaces:** 現行IndexRunのフィールドを維持する。最初の走査とhash判定で決まったFilesAdded/FilesChanged/FilesUnchangedを、依存再抽出のキュー追加時に変更しない。

- [x] `TestR11ContentChangeCounts`を追加する。同名対象追加、既存対象変更、削除、競合追加の連鎖、複数経路からの同一依存元を検査する。追加だけの再現は`added=1, changed=0, unchanged=2`とし、返却と保存済みrunで同じ値を確認する。
- [x] `rtk proxy go test ./internal/indexer ./internal/app -run TestR11 -count=1`でREDを記録する。
- [x] 依存再抽出のためのchanged/unchangedの加減算を除き、重複抑止と関係再構築は維持する。別の公開統計フィールドは追加しない。
- [x] 同コマンドでGREENを確認し、関係集合が全索引と一致すること、FilesSeenの分類合計が正常なfixtureで一致することを確認する。
- [x] R11をコミットする。

### Task R09 — Impactへ構文上の依存元・関連テストを追加する

**Files:** `internal/app/evidence.go`、新規`internal/app/impact_relations.go`、`impact_freshness.go`、`internal/app/residual11_test.go`、新規`internal/mcpserver/residual11_test.go`、必要なEvidence回帰テスト。

**Interfaces:** `impactCandidates`の返却型を維持する。内部`impactRelationCandidates(ctx context.Context, changed []model.RankedCandidate) ([]model.RankedCandidate, error)`を追加し、変更シンボルの既存handleから1段の関係を取得する。source検証は現行のScanner/hash確認を共通化し、変更対象と追加対象を同じ検証方針で扱う。

- [x] `TestR09ImpactRelations`を正式化する。変更関数、非変更Caller、関連Testの3ファイルが十分な予算で返ることを、旧形式・Evidence・MCPで確認する。変更関数を持つTSファイルへのmodule importも検査する。複数変更アンカー、循環、未解決候補、低予算、既知handle、削除ファイル、明示Git参照も検査する。
- [x] `rtk proxy go test ./internal/app ./internal/mcpserver -run TestR09 -count=1`でREDを記録する。既存Storeのcallers/tests取得で関係が存在することを対照にする。
- [x] 変更行と重なるシンボルをアンカーとし、callersおよびimports/exports/referencesの入方向、関連testsの入方向から1段の候補を得る。ファイル依存には同じ変更ファイルのmodule/package所有者も探索アンカーとして使い、所有者のoutline全体を変更本文として追加しない。既存RelationContextの方向・確定／候補・実際のAnchorHandleを保持し、重複を排除してから既存上限と順位を適用する。変更候補はChanged=trueを保ち、関係候補は変更扱いにしない。範囲内の変更候補が関係候補だけで押し出されないようにする。
- [x] 自動更新ONは検索前更新を維持する。OFFでは追加される非変更ファイルもhash・索引対象性を検証し、不一致／未索引は更新要求エラーとする。現存しないファイルの旧本文は返さない。共有ロックを最終revisionまで保持し、MCP原文一回・原文/行位置・既存role・実測wire予算・省略時の既存guidanceを検査してGREENを確認する。
- [x] R09をコミットする。既存のdirty MCPファイルへ無関係な変更を混ぜず、新しいテストから公開説明どおりの結果を確認する。

## Validation and Acceptance

各IDは一時overlayを必要としない正式な回帰テストで修正前FAIL・修正後PASSを確認する。fixtureは`testdata/regressions/residual11/`、Git作業場所・DBは`t.TempDir()`へ置く。対象コードを実行せず、抽出結果・実DB・Evidence／MCP応答を比較する。

抽出修正の終了後、`internal/indexer/indexer.go`のextractor versionを一度更新する共有コミットを作る。旧バージョンmetadataの既存索引について、Status/Healthが更新必要を示し、通常の増分更新で全対象を再抽出して誤った既存関係を除去し、その次の更新では不要な再抽出をしないことを検査する。version更新はR01〜R08の該当する抽出変更の共通前提として台帳へ記録する。

最終検証は以下の順序で行う。`rtk`を全shellコマンドへ付け、開始時に各processの環境を設定する。Go raceに必要なローカルC toolchainを確認し、不足時にインストールせず未確認ゲートとして記録する。

| 順序 | 検証 | 合格条件 |
|---|---|---|
| 1 | `rtk proxy go test ./...` | 全通常テストPASS。R01〜R11と既存F項目の回帰を含む |
| 2 | 通常終了後に`rtk proxy go test -race ./...` | 全race検査PASS。通常と並列実行しない |
| 3 | `rtk proxy go vet ./...` | 指摘なし |
| 4 | ネイティブの`go build` | `./cmd/focalspan`、`./cmd/focalspan-bench`、`./cmd/focalspan-eval`を別出力先へビルド成功 |
| 5 | `CGO_ENABLED=0`のクロスビルド | Windows amd64/arm64・Linux amd64・Darwin arm64で上記3実行ファイルがビルド成功 |
| 6 | 利用可能なWSL-Ubuntuで対象テスト | case-sensitive FS上でR04/R05/R10の対象テストがPASS。環境を使用できない場合は成功扱いせず記録 |
| 7 | 既存ローカルbenchmark smoke | R09の関連候補追加を含め、結果変化をIDと期待値へ対応付ける。品質低下をbaselineの上書きで隠さない |
| 8 | 独立した非セキュリティレビュー | 11件の仕様充足、境界条件、既存契約、差分・検証を確認し、未解決の機能不具合指摘がない |

R09でEvidenceの内容・選択が変わるため、`internal/evidence`のwire-budget・不変条件テスト、MCPのv1/v2・stdout・原文重複のテストを省略しない。新規公開フィールドやランキングdebug値は追加しない。

隔離worktreeのコミット対象ツリーを検証した後、元の作業ツリーに残るユーザー所有MCP変更との組合せも一時overlayで検証する。これらの未コミット変更を今回のコミットへ含めることとは区別する。統合前に必要なレビュー／検証が失敗・未実行なら、masterへマージしない。

## Idempotence and Recovery

R04の設定追加・削除およびversion更新の再実行は、同じ関係集合へ収束することを確認する。失敗・キャンセル時は既存のlinking_pending回復と未完了索引の検索拒否を維持する。テストは稼働索引やユーザー設定をfixtureとして使わない。

masterが進んでいた場合は変更を確認して作業ブランチへ取り込み、変わったコードへ必要な検証・レビューをやり直す。ユーザー差分を消す操作やforce mergeは行わない。修正取り消しは今回の対象コミットだけをrevertする。

完了時は検証済みの作業ブランチを`master`へ`--ff-only`で統合し、HEAD到達とユーザー差分の保持を確認する。その後、今回作成したブランチだけを通常のmerged判定付き削除で削除する。他のブランチ／worktreeを一括削除しない。push・再配置は行わない。

## Interfaces and Dependencies

| 内部インターフェース | 所有Task | 利用者／制約 |
|---|---|---|
| JS/TS bindingIndex | R01 | 呼出し位置のbyte offsetを基準にスコープ判定。局所動的束縛を大域候補として残さない |
| staticModuleArgument | R02 | import()/require()の静的な第一引数だけを返す。R03の内部import走査と両立する |
| preferModuleOwner | R05 | JS/TS module関係の最終一意性判定。R04のalias候補にも適用される |
| project-config Fact / ApplicablePathAliases | R04 | 候補収集・最終解決・manifest revisionで共有。SourcePathは設定位置のまま保持 |
| SymbolsForIdentity | R10 | benchmark engine専用の用途。Storeにbenchmark型やretrieval上限を持ち込まない |
| impactRelationCandidates / source検証共通化 | R09 | 既存共有ロックの内側で実行。旧形式とEvidence形式の共通候補取得へ接続 |

## Outcomes & Retrospective

R01〜R11の実装・正式回帰テスト・問題別コミットを完了した。最終検証と統合の実測結果は本書末尾および検証記録へ記載する。

実装完了時に、R01〜R11のcommit対応表、共有version更新、実測検証ログ、独立レビュー、master統合SHA、ブランチ削除結果、残る未確認ゲートをここへ記録する。将来の監査で残存ゼロを保証するものとはしない。

Execution 2026-10-18T05:17:28.789781+00:00: R02 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:18:58.049592+00:00: R03 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution: R01 complete 3061fd4a27b2; R02 complete 365722a60b11; R03 initial b818f0bf plus export-symbol control follow-up. Full normal suite passed with -count=1 after R01/R02; cached baseline attempts were stopped without product failure assertion.

Execution 2026-10-18T05:20:14.339816+00:00: R03-symbols RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:23:17.520667+00:00: R05 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:26:40.274101+00:00: R04 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:29:40.584047+00:00: R06 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:32:48.041030+00:00: R07 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:34:49.931996+00:00: R08 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:37:12.552113+00:00: R10 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:38:37.584238+00:00: R11 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:56:03.062121+00:00: R09 RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T05:56:55.382604+00:00: extractor-version RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T06:07:04.278033+00:00: R01-review RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T06:07:07.607525+00:00: R07-review RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T06:07:11.835767+00:00: R11-controls RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.


## Execution evidence

Base: `66d8ea87916cdda57d0d3c0ecd8fd26dfc91e07a`。
専用worktreeの実装により元の未コミット11ファイルのbytesを保持した。

| ID | 修正コミット | 正式回帰の主要箇所 |
| --- | --- | --- |
| R01 | 3061fd4, fb101f4 | jsts/residual11_test.go, app/residual11_test.go |
| R02 | 365722a | jsts/residual11_test.go |
| R03 | b818f0b, c625ebd | jsts/residual11_test.go |
| R04 | 4299a21 | projectmeta/residual11_test.go, app/residual11_test.go |
| R05 | cfcad62 | linker/residual11_test.go, app/residual11_test.go |
| R06 | d8991f3 | ruby/residual11_test.go |
| R07 | f48bc2f, b04b916 | ruby/residual11_test.go |
| R08 | bb3d9f6 | python/residual11_test.go |
| R09 | 09dae49 | app/impact_relations_test.go, mcpserver/residual11_test.go |
| R10 | d82235a | store/residual11_test.go, benchmark/residual11_test.go |
| R11 | 9035a64, b76c905 | indexer/residual11_test.go |

共通version更新: `731d52c`、v18→v19を正規更新で一度だけ再抽出する回帰を追加。

### 実行時の判断と補足

1. WindowsではPOSIX補助手順をPython実行アダプタへ置換した。対象パス限定stage、RED/GREENの生ログ、BASE・SHA・作業記録を維持した。
2. Goのキャッシュ付きテスト起動がパッケージ実行前に停止したため、`-count=1`を採用した。30秒の診断タイムアウトはapp全件には短く、最終検証は15分上限とした。初回全件では旧version固定テストとStore並行初期化のdeadline失敗が出た。前者を現行version比較へ修正、後者は対象再実行で成功し、最終全件で再確認する。
3. F15の旧fixtureは設定ファイル外のimporterへalias適用を期待していた。importerを同じ設定配下へ移し、既存wildcard検査を保持した。隣接プロジェクトへの不適用は新規回帰で確認する。
4. R09はGo所有者にchunkがない場合も含めるため内部SQL取得を追加し、legacy packerへ内部impact hintを渡した。MCPの公開入力へmodeは追加していない。focused応答で実際に出るsignatureの原文一回を検査し、本文全体の忠実性はapp source形式で別途検査する。
5. 独立レビューは全差分を一度確認し、P2の3件（loop scope、匿名callback引数、Ruby複数行Regex）を指摘した。全件RED確認後に修正した。R01はレビュー担当へ2ファイル限定で修正を委譲し、親が差分を確認した。R07は親が修正し担当が独立読取り確認した。追加の全差分再レビューは行っていない。

サイバーセキュリティ項目、supervisor EOF、push、稼働バイナリ置換は対象外。v19は今回のビルド成果物に含まれるが稼働プロセスには未配置。

Execution 2026-10-18T06:11:27.193555+00:00: R01-typed-boundaries RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

Execution 2026-10-18T06:14:09.177355+00:00: R01-header-scope RED→GREEN; scoped package checks passed. Evidence: C:/Temp/fs11-evidence/. Commit identity: git log and SDD ledger.

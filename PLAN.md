# 残存24問題の修正計画

> **For agentic workers:** 実装時は `superpowers:executing-plans` を使用し、各問題のRED→GREEN→コミットを順に完了する。委任は別途認められた場合だけ行う。

**Goal:** 最初の未修正5件と追加監査19件を修正し、安全な走査・差分処理、原文に忠実なEvidence、継続的に正しい関係索引を提供する。
**Architecture:** 既存のscanner→extractor→store→linker→search→Evidenceの構成を維持する。Gitの検出・参照・差分を内部APIへ集約し、入力・行座標・言語の境界を明示する。既存DBの派生データは抽出器バージョン更新により再構築する。
**Tech Stack:** Go、既存のSQLite/FTS5、ローカルGit、Windows/Linux/macOS。新規依存は追加しない。
**Spec:** 本文の24件対応表と受け入れ条件。前計画は `docs/superpowers/plans/completed/2026-10-17-remaining-five-index-consistency.md` に原本のまま保存した。

## Purpose / Big Picture

この計画の「最初の5件」は直前の追加監査で除外した未修正5件であり、前計画で修正済みの5件ではない。新たな修正対象は合計24件。ユーザーの今回の依頼は計画作成までで、製品コードの実装は開始していない。

8つの修正単位を順に実行する。各単位内でも異なる根本原因は回帰テストとコミットを分ける。実装・回帰テストの見積もりは14〜20時間、全検証・レビューは2〜4時間。実際の進捗と検証結果を以下へ追記する。

## Global Constraints

- MCP公開フィールド、Evidence v1/v2のwire形式、DBスキーマバージョンを変更しない。通常応答へ順位やtoken-savingsのdebug項目を追加しない。原文はstructured contentに1回だけ出す。
- 検索は既存のShared Writerロック、更新はExclusive Writerロックを維持する。待機は最大5秒、期限・キャンセルを優先する。indexGateはFinalizeUpgrade完了まで保持する。
- ネットワーク、依存取得、外部LLM、解析対象ソースの実行、push、稼働バイナリの置換は行わない。通常テストとraceテストは直列に実行する。
- 既存の未コミット変更・未追跡ファイル・ロック・他のworktreeを保持する。今回の計画変更はPLAN.mdと新規archiveだけを所有し、AGENTS.md、README.md、docs/design.md、internal/mcpserver/{server.go,context_v2_test.go}の既存差分を取り込まない。
- シェルコマンドは `rtk` 経由。各不具合の再現テストを修正前に実行し、意図した理由で失敗したことを確認する。Evidence変更には原文・行位置・不変条件・実測wire予算のテストを必須とする。

## Progress

- 2026-10-17 execution: Task 3完了。O02=b678eb3、O05はRED→GREEN。config/generic/template/indexer/appの全テストPASS（C:/Temp/fs24-task3.log）、全fallbackと埋め込みの追加検査もPASS。抽出設定hashを完了transactionへ保存し、extractors-v7-configured-windowsへ更新。

- 2026-10-17 execution: Task 2完了。A05=9d1c962、A14=83217c3、A09=868b37a（gitlink列挙で区別する追補をA06と共に実施）。A06の外側junction拒否・内側許可はRED→GREEN。repository/store/indexer/appはTask 3の先行RED 2テストだけ除外して全PASS（C:/Temp/fs24-task2-green.log）。Task 3のO02/O05も正式REDを観測済み。

- 2026-10-17 execution: Task 1 GREEN: O01=3700ec1、A01=66cb695。A07も正式RED→GREEN。gitx/repository/app/cli全テストPASS（C:/Temp/fs24-task1.log）。安全なref解決・textconv無効化・Git probeを完了。

- 2026-10-17 execution: e986a29で計画移行をコミットし、codex/remaining-24-fixesで開始。O01は正式回帰REDを確認後、Git probeのエラー伝播と本当の非Gitの区別を実装。repository/app/cli全テストGREEN（C:/Temp/fs24-o01.log）。Task 1のA01/A07は未修正。

- [x] 2026-10-17 10:06 UTC: master / 47ef2a1と既存のdirty pathsを確認。24件の対象を照合し、旧PLAN.mdを同一内容でarchiveへ保存した。
- [x] 2026-10-17 UTC: 24件を8単位へ割り当て、依存順・内部API・受け入れ条件を記述。文書検査PASS（24 IDの一意性、8 Task、必須section、archive、末尾空白なし）、旧計画との同一内容確認とgit diff --checkもPASS。製品コードは変更せず、テスト・ビルドは実行していない。
- [ ] 実装開始の指示後に専用ブランチを作成し、計画移行だけをコミットする。
- [ ] Task 1〜8の各RED/GREEN・コミットを完了する。
- [ ] 全検証・レビュー・masterへのfast-forward統合と作業ブランチ削除を完了する。

## Context and Orientation

基点は `master` の47ef2a1。直前監査の一時overlayは `C:/Temp/fs_additional_overlay.json`、最初の5件は `C:/Temp/focalspan_remaining_audit_second_20261017_overlay.json`。これらは証拠資料であり、正式テストの実行依存にしない。必要なfixtureとassertionをリポジトリ内へ移す。追加19件は20の再現FAIL（異言語リンクを2方式で確認）に対応し、シンボリックリンク別名参照のSKIPは今回の24件へ含めない。

| ID | 未修正問題 | 担当Task |
|---|---|---|
| O01 | Git検出失敗を非Gitと扱い、.gitignoreが無効になる | 1 |
| O02 | 言語override等の設定変更で同一バイトのファイルが再抽出されない | 3 |
| O03 | 履歴refの変更行を現在のworktreeへ直接当て、Impactが空になる | 6 |
| O04 | 検索の終盤でキャンセルすると、空revisionを含む成功Evidenceを返す | 7 |
| O05 | generic_chunk_lines / generic_chunk_overlapが未使用 | 3 |
| A01 | base_ref/head_refのGitオプション注入で指定ファイルを上書きする | 1 |
| A02 | sourceモードがsynthetic outlineをverbatimと表示する | 4 |
| A03 | 呼び出し先の差分更新で、未変更の呼び出し元の入辺が消える | 5 |
| A04 | Goの//lineで物理行位置が論理行位置へ置換される | 4 |
| A05 | .ENV等の大文字名が機密ファイル除外を通過する | 2 |
| A06 | index_directoryのジャンクション経由でroot外へDBを作成する | 2 |
| A07 | Git textconvが起動し、原文と異なる差分になる | 1 |
| A08 | 未追跡ファイルがchanged-onlyとImpactから抜ける | 6 |
| A09 | GitサブモジュールのディレクトリをReadFileして索引全体が失敗する | 2 |
| A10 | 識別子の無条件case-foldでGo等の異なる名前を誤リンクする | 5 |
| A11 | 無関係な異言語の同名シンボルを解決済みとしてリンクする | 5 |
| A12 | callsでファイル名一致が優先され、唯一の対象関数も未解決になる | 5 |
| A13 | 関係SQLのLIKEが識別子の_や%をワイルドカードとして扱う | 5 |
| A14 | 8192バイト以降のNULを検出せず索引へ入れる | 2 |
| A15 | 明示的な--query値が引数正規化で壊れる | 8 |
| A16 | enabled=falseのCodex登録をmanaged_matchと判定する | 8 |
| A17 | キャンセル済みのCodex設定操作が書き込みを続行する | 7 |
| A18 | Rust cfg(not(test))をテスト属性と誤認する | 4 |
| A19 | 通常のGoファイルのTestConnection等をテストへ誤分類する | 4 |

## Review Focus

1. Gitが利用不能・壊れた.git・worktreeの.gitファイル・本当の非Gitを区別し、除外の弱体化と既存索引削除を防ぐ。Task 1の境界fixtureで確認する。
2. 新規index_directoryの途中の親がジャンクション/シンボリックリンクの場合も、外側へ作成しない。Task 2で最終パスと親の両方を検証する。
3. A→B→Cの鎖、循環、削除後再追加、変更されたシグネチャでも入辺を保持/再生成する。Task 5で差分更新とfull rebuildを比較する。
4. 履歴ref後の200行挿入、置換、削除、rename、未追跡・unborn・staged後の追加編集が現在の行へ対応する。Task 6の対照テストで確認する。
5. 最後のDB読み取り・設定rename直前のキャンセルとsyntheticの予算縮退が成功扱いにならない。Task 4/7でチャネルを使い決定的に検証する。

## Plan of Work

### Task 1 — Gitの検出・引数・差分実行を安全にする（O01 / A01 / A07）

**Files:** `internal/repository/root.go`, `scanner.go`; `internal/gitx/git.go`; `internal/app/{evidence.go,service.go}`; `internal/cli/run.go`。正式回帰は新規 `internal/gitx/safety_test.go` と `internal/repository/git_probe_test.go`、既存app Gitテストへ追加する。

**Interfaces:** `repository.ProbeGitRepository(ctx context.Context, root string) (bool, error)` を導入する。安全性に関わる呼び出し元はboolだけのIsGitRepositoryに依存しない。`gitx.Client.resolveCommit(ctx context.Context, ref string) (string, error)` は入力を検証し、オプション終了境界付きrev-parseでcommit objectへ解決する。差分へ渡すのは得られたobject IDだけとする。

- [ ] **RED:** `TestGitProbeFailurePreservesIgnoredSourceAndIndex`、`TestDiffRefCannotWriteOutputFile`、`TestDiffIgnoresTextconv` を正式化する。ダミー機密fixtureが格納されないこと、victimのバイト列が保持されること、textconv設定の有無で原文差分が一致することを検査する。
- [ ] **実装:** 空文字は既存の省略規則に従う。先頭`-`、NUL等の不正refを拒否し、無効refのエラーを保持する。全diff経路へ`--no-textconv --no-ext-diff`を適用する。Git利用不能・probe失敗は原則エラー。本当に非Gitと確認できる場合だけwalkへ進む。Git不在でも祖先に.gitマーカーがない非Gitディレクトリは使用可能とし、.gitのdirectory/file、壊れたマーカー、キャンセルを区別する。ローカライズされたstderr文言だけで判別しない。
- [ ] **GREEN:** `rtk go test ./internal/gitx ./internal/repository ./internal/app ./internal/cli`。通常ref、annotated tag、HEAD~1、head省略、非Git、Gitのない非Git、worktree、既存のquoted path・削除hunkを維持する。
- [ ] O01、A01、A07をそれぞれ対象パスだけstageしてコミットする。

### Task 2 — 走査と索引保存先の境界を修正する（A05 / A06 / A09 / A14）

**Files:** `internal/repository/scanner.go`; 新規 `internal/repository/index_directory.go`; `internal/store/store.go`; 新規 `internal/repository/scanner_boundaries_test.go`, `internal/store/index_directory_test.go`。

**Interfaces:** `repository.PrepareIndexDirectory(ctx context.Context, root, relative string) (string, error)` はroot実体内であることを検査して正規化した保存先を返す。既存のscanner、store、indexlockは同じ保存先を使う。通常ファイルの走査エラーを既定除外へ置き換えない。

- [ ] **RED:** .ENV/Credentials.json/ID_RSA、NULの位置8191/8192/EOF近傍、既存/途中親のroot外junction、root内link、Git mode160000のディレクトリを正式fixtureへ移す。拒否時に外側のDB・lock・ディレクトリを作らないことも検査する。
- [ ] **実装:** 既定の機密名判定は大小文字を区別せず安全側に揃え、明示Includeの既存の許可規則を保持する。NULは読み取り済みの全バイトを検査する。Git列挙結果のgitlink/ディレクトリ/非通常ファイルは内容を読まずskipし、サブモジュールへ再帰しない。通常ファイルのread/stat失敗は引き続き更新全体を中止する。
- [ ] **保存先:** 字句上の`..`検査に加え、既存の最も近い親から実体を解決し、作成前と使用前にroot内であることを再検査する。root外を指すindex linkは拒否、root自体のlinkとroot内linkは許可する。OSのlink差し替え競合をこの検査だけで完全防御したと主張しない。リンクfixtureを実行できないOSはSKIPを明記し、Windows junctionは実行確認する。
- [ ] **GREEN:** `rtk go test ./internal/repository ./internal/store ./internal/indexer ./internal/app`。A05/A06/A09/A14を個別にコミットする。

### Task 3 — 設定と抽出結果を一致させる（O02 / O05）

**Files:** `internal/config/config.go`; `internal/indexer/indexer.go`; `internal/extract/generic/generic.go`; `internal/extract/template/{extractor.go,embedded.go}`; `internal/app/service.go`。新規 `internal/indexer/configuration_refresh_test.go`、既存generic/template/appテストへ追加する。

**Interfaces:** `Config.ExtractionHash() string`、`generic.NewExtractorWithOptions(lines, overlap int) Extractor`、`template.NewExtractorWithChunkOptions(lines, overlap int) Extractor` を追加する。既存引数なしconstructorは80行/10行の既定値を保持する。metaの`extraction_configuration_hash`を追加するがDBスキーマは変更しない。

- [ ] **RED:** 同一バイトのlogic.txtをtext→Goへ変更し、差分IndexだけでValidateTokenが得られることを検査する。10行/設定2・0で5chunk、90行/設定80・0で次chunkが81行目、設定80・10で71行目、上限160行、BOM/CRLFを検査する。
- [ ] **実装:** 抽出設定hashの欠落/相違は再抽出を要求し、成功した更新の同じtransactionで新hashを保存する。Status/Healthもこのmetaの欠落/相違をstaleと判定し、読み取りエラーは返す。hashの対象はLanguageOverrides、GenericChunkLines、GenericChunkOverlap。scanner対象の変更は毎回の列挙で反映する。読み取り失敗をhash不一致へ変換しない。generic内の5つの固定windows呼び出しとtemplateの埋め込みfallbackへ設定を通す。worker数・予算だけの変更で抽出全件をやり直さない。
- [ ] **GREEN:** `rtk go test ./internal/config ./internal/extract/generic ./internal/extract/template ./internal/indexer ./internal/app`。失敗した走査/抽出準備で新設定を完了済みと記録しないことを確認し、O02/O05を個別にコミットする。

### Task 4 — Evidenceの忠実性と抽出器の位置・分類を修正する（A02 / A04 / A18 / A19）

**Files:** `internal/evidence/fidelity.go`; `internal/extract/goast/go.go`; `internal/extract/rust/extractor.go`; `internal/indexer/indexer.go`。既存fidelity/wire/invariant、goast/rustテストと新規 `internal/app/source_coordinates_test.go` を使用する。

- [ ] **RED:** 実際のJS/C#/RESX等のsynthetic chunkをsourceモードへ通し、syntheticだけで返すことを検査する。Goの//line virtual.go:900でも物理3行目を維持する。Rust cfg(not(test))を通常関数、実際の#[test]/#[tokio::test]をテストとする。Go connection.goのTestConnectionは通常関数とする。
- [ ] **実装:** synthetic判定を全モードへ適用し、source/excerptへ昇格させない。予算不足時もsynthetic由来の生成文を原文と表示しない。Goは物理位置を使用する。Goテスト分類は_test.goとASTの引数・返り値・Test/Benchmark/Fuzz/Exampleの命名を確認し、既存の正当なテストを維持する。Rustは属性の構文上のパスを比較し、任意の部分文字列testでは判定しない。cfg式の実行・マクロ展開は追加しない。
- [ ] **GREEN:** `rtk go test ./internal/evidence ./internal/extract/... ./internal/app ./internal/mcpserver`。source/focused/outline、予算256/512/1200/4000、v1/v2、BOM/CRLF、変更行限定で原文と位置・wire予算を検査する。
- [ ] 抽出結果を変える最初の修正でExtractorVersionを更新する。以後も、前コミットで生成した索引を次の修正後に再利用できる場合はバージョンを更新する。旧索引のStatusはstale、次の更新で再抽出する。4件を個別にコミットする。

### Task 5 — 関係の保存・解決・照合を修正する（A03 / A10 / A11 / A12 / A13）

**Files:** `internal/indexer/indexer.go`; 新規 `internal/store/relation_dependents.go`; `internal/store/{store.go,links.go}`; `internal/linker/linker.go`; 新規 `internal/language/relation_policy.go`。識別子を小文字化して解決するgoast/jsts/python/rust等の抽出器も対象とする。新規 `internal/app/relation_incremental_test.go`、既存linker/store/polyglotテストへ追加する。

**Interfaces:** `Store.IncomingRelationPaths(ctx context.Context, targetPaths []string) ([]string, error)` は更新前の解決済み入辺のsource pathを返す。indexerは結果を反復し依存元の閉包を求める。`language.SymbolNamesEqual(language, kind, left, right string) bool` と `language.RelationLanguagesCompatible(from, to, relation string) bool` を共通化する。`resolveCandidates`はrelation kindも受け取る。

- [ ] **RED A03:** Caller→Targetを作り、Targetの本文更新後も同じcallsが残ることを検査する。鎖・循環・シグネチャ変更・削除/再追加・抽出失敗・中断を加え、差分索引をfull rebuildのsymbol/relation集合と比較する。
- [ ] **実装 A03:** 変更/削除対象から入辺を持つ未変更sourceを更新前に収集し、現在走査済みのファイルから再抽出する。そのsourceの置換でもさらに入辺が消えるため、訪問集合で閉包を作り各pathを1回だけ処理する。削除済みsourceは再生成しない。全updatesを既存transactionで適用しlinking_pendingの回復を維持する。FilesChanged等は内容変更の件数を保ち、依存再抽出で二重計上しない。
- [ ] **RED/GREEN A10/A11/A12:** Go validate≠Validate、JS/TS・Python・Rustの大小違い、Go→Python拒否、唯一のTargetをtarget.go/defs.goのどちらに置いても解決する対照例を追加する。VB/PHP/Nimの既存許容比較とC/C++・JS/TS間の正当な関係を維持する。言語の一致、識別子種別、関係kindを候補の最終解決前に検査し、callsはsymbol一致を優先する。imports等だけにファイル照合を使い、既存の明示ToHandleを持つtemplate/XAML等の境界関係を壊さない。
- [ ] **RED/GREEN A13:** foo_barとmodule.fooXbar、%を含む有効なpath、Unicode名の境界を検査する。動的識別子をLIKEパターンへ連結せずliteralなprefix/suffix比較にする。意図的なcamel-case探索パターンだけは各literalをescapeする。正確な言語・名前照合をSQL上限より前に適用し、別言語や誤名が130件あっても対象が落ちないようにする。
- [ ] **検証・commit:** `rtk go test ./internal/language ./internal/linker ./internal/store ./internal/indexer ./internal/search ./internal/extract/... ./internal/app`。新しい依存閉包の最悪時は全ファイルを1回ずつ再抽出する。無条件で全件再構築する実装にしない。各IDを個別にコミットし、必要なExtractorVersionを更新する。

### Task 6 — 現在の変更範囲を正しく作る（O03 / A08）

**Files:** `internal/gitx/{git.go,diff.go}`、新規 `internal/gitx/range_mapping.go`; `internal/app/{evidence.go,impact_freshness.go}`。新規 `internal/gitx/range_mapping_test.go`、既存app Git/Impactテストへ追加する。

**Interfaces:** 旧新両方のhunk区間を保持する内部edit型を追加する。`Client.MapRangesToWorktree(ctx context.Context, head string, changed []ChangedFile) ([]ChangedFile, error)` は指定headの行範囲を現在のpath/行範囲へ変換する。既存の公開ChangedFile/Evidence形は変更しない。

- [ ] **RED A08:** 既存HEADとunbornの双方で未追跡new.goを検出する。ignored・機密・binary・過大ファイルは最終候補に出さない。stagedとunstagedとuntrackedを統合してもpath/範囲を二重計上しない。
- [ ] **実装 A08:** `git ls-files --others --exclude-standard -z`で列挙し、現在のファイルに対する全体範囲を付与する。既存のHEAD→worktree差分へunionし、scannerの適格性検査へ通す。明示的な履歴ref間だけのImpactへ無関係な未追跡ファイルを混ぜない。
- [ ] **RED O03:** HEAD~1→HEADの4行目の変更後に200行のコメントを挿入し、現在の203行目以降の関数を返すことを確認する。置換・削除・rename・head省略・現在削除済みのファイルも検査する。
- [ ] **実装 O03:** base/head両方を指定した場合だけheadの座標を現在へ変換する。baseだけ・headだけの指定は既存の単一ref→worktree比較を保ち、既に現在の座標なので再変換しない。両方省略も現在座標とする。両refはTask 1のcommit IDへ固定し、head→worktreeの原文diffから対応を計算する。変更されていない部分は行数差を加算、置換部分は交差した現在側hunkへ保守的に対応、挿入分は後続位置へ反映、削除部分は既存の生存境界規則へ合わせ、全削除ファイルは除外する。Gitが示すrenameだけを追い、曖昧な対応・読めないblobは明確なalignmentエラーにする。異なる座標のまま成功や空結果へ変換しない。索引と現在ファイルのhash検証・auto/manual設定を維持する。
- [ ] **GREEN/commit:** `rtk go test ./internal/gitx ./internal/app ./internal/search ./internal/mcpserver`。両形式のImpactとchanged-only、auto/manual、wire/invariantsを検査し、O03/A08を個別にコミットする。

### Task 7 — 読み取りと設定書き込みでキャンセルを守る（O04 / A17）

**Files:** `internal/app/{read.go,evidence.go,service.go}`; `internal/integration/codex/project_config.go`。新規 `internal/app/read_cancellation_test.go`, `internal/integration/codex/project_cancellation_test.go`。

**Interfaces:** `Service.revision(ctx context.Context) (string, error)` とし、Metaのエラーを全呼び出し元へ返す。`readIndex`は内部の再入経路も含め、callbackの前後でcontextを確認し、キャンセル時にデータ結果を返さない。設定用atomicWriteはctxを受け取る内部形へ変更する。

- [ ] **RED O04:** 最後のcandidate取得直後とrevision取得前後にチャネルでキャンセルし、Query/Expand/Impactのlegacy/Evidence/attributedが成功しないことを検査する。DB metadata読み取りエラーも成功へ変換されないことを検査する。
- [ ] **実装 O04:** revision取得のエラー握り潰しを除去する。成功結果を返す直前にctxを再確認し、キャンセルが観測されたらzero resultとcontextエラーを返す。結果が返った後のキャンセルまで防げるとは主張しない。
- [ ] **RED/実装 A17:** InstallProject/UninstallProjectを開始前とtemp書き込み後・rename前でキャンセルし、元configのバイト列・modeが保持され、tempだけ片付くことを確認する。原子的renameをcommit pointとし、それより前のキャンセルで置換しない。dry-runもキャンセル済みならエラーにする。テスト用の同期点を外部公開APIへ追加しない。
- [ ] **GREEN/commit:** `rtk go test ./internal/app ./internal/integration/codex ./internal/mcpserver`。同時検索・5秒競合・キャンセル・設定の正常install/uninstallを維持し、2件を別コミットにする。

### Task 8 — CLI引数とCodex登録の状態比較を修正する（A15 / A16）

**Files:** `internal/cli/run.go`; `internal/integration/codex/project_config.go`、既存 `cli_test.go`, `project_config_test.go`。

- [ ] **RED A15:** --query needle、-query needle、--query=needle、自然文、--区切り、前後に--root/--path/--known-handle、ハイフンから始まる値を表形式テストへする。明示queryと位置引数の衝突は曖昧に連結せず、既存の入力検証エラーにする。
- [ ] **実装 A15:** normalizeQueryArgsで--query/-queryを値付きとして保持する。自然文からqueryを作る場合だけ補完し、明示値を二重追加しない。
- [ ] **RED/実装 A16:** 正常登録のenabled、enabled_tools、command、root、auto-update、startup/tool timeoutを1つずつ変更し、managed_matchにならないことを検査する。StatusProjectはInstallProjectの期待specと同じtableMatchesを利用し、未管理登録はunmanaged_match/conflict、管理対象はmanaged_match/managed_driftに分ける。バイナリ名が同じ別パスを一致と見なさない。公開状態値とフィールドは維持する。
- [ ] **GREEN/commit:** `rtk go test ./internal/cli ./internal/integration/codex`。CRLF/TOML/他の登録保持/dry-runを維持し、A15/A16を個別にコミットする。

## Interfaces and Dependencies

Task 1→2/6、Task 3→4/5の索引再抽出、Task 4→5の正しいsymbol位置/種別、Task 5→6の正しい関係、Task 6→7の最終読み取り経路の順で進める。Task 8は独立しているが、共有ファイルの混在を避け最後に実行する。

言語照合の内部policyは一般検索の曖昧な名前検索を変更せず、関係の解決・関係SQLの正確性へ適用する。既存のlookup projectionは広い候補取得用に残し、誤った候補を最終照合で確定しない。VB/PHP/Nim等の既存fixtureを先に固定し、一律のcase-sensitive化で互換性を壊さない。

## Validation and Acceptance

各修正前に正式回帰をREDで観測し、対象修正後にGREENを確認する。24 IDすべてについてテスト名・実行結果・対応commitをProgressへ記録する。既存overlayの一括実行だけで完了としない。Fixtureとend-to-end acceptanceはtestdataとpackage integration testsへ置く。

最終ゲートはオフライン（GOPROXY=off / GOSUMDB=off / GOTOOLCHAIN=local）で順に実行する。

1. `rtk go test ./...`、完了後に `rtk go test -race ./...`。Windows raceはCC=C:/msys64/ucrt64/bin/gcc.exe、PATHへ同ディレクトリを追加しCGO_ENABLED=1を使用する。
2. `rtk go vet ./...`、`rtk go build -o .build/remaining-24/focalspan.exe ./cmd/focalspan`。
3. CGO_ENABLED=0でGOOS/GOARCHをwindows/amd64、linux/amd64、darwin/arm64へ順に設定し、出力を `.build/remaining-24/` の別ファイルへ保存する。
4. `rtk git diff --check`、24件対応表とテスト/commitの照合、実測wire予算・原文・行位置・不変条件を確認する。許可された独立レビューが利用可能なら全branch差分を対象とし、使えない場合はその制約を明記して別パスでレビューする。未レビューを独立レビュー済みとは扱わない。
5. 全ゲートと必要なレビューの通過後、既存の統合方針に従いmasterへfast-forwardマージし、到達確認後に今回の作業ブランチだけ削除する。push/稼働バイナリ置換は行わない。最終結果をPLANへ記録する。

## Surprises & Discoveries

- Task 2: このWindows環境ではfilepath.EvalSymlinksがjunctionを字句パスのまま返した。GetFinalPathNameByHandleで開いた実体を検査する。A09の通常ディレクトリskipは既存のファイル→directory読み取り失敗検査を弱めたため、Git stageのmode160000だけを列挙から除き、通常ファイルの型変化は引き続きエラーにした。

- 再現テストの数は問題数と一致しない。A11は内部関数/実索引の2テストで1件、O05は行数/overlapの2テストで1件。24件のIDで管理する。
- A03は入辺のsourceも置換するとさらに上流の入辺が消えるため、直接呼び出し元だけの再抽出では足りない。訪問集合を持つ依存閉包が必要。
- O03は索引を更新するだけでは直らない。履歴headの座標から現在の座標への変換を別に行う。
- A06の保存先チェックは、悪意ある同時link差し替えに対するOSレベルの完全なrace防止とは分けて扱う。

## Decision Log

- 2026-10-17: 対象はO01〜O05とA01〜A19の24件。追加の不確定候補とsupervisorの原因未確定EOFは含めない。
- 2026-10-17: DB schema/public fieldsは変更せず、既存metaに抽出設定hashを保存する。抽出器/解決ルール変更はExtractorVersionで既存派生索引を更新する。
- 2026-10-17: 履歴Impactは現在ソースを返す既存方針を維持し、行範囲を対応付ける。対応不能時は成功結果を捏造せずエラーにする。
- 2026-10-17: 入辺の原ターゲット文字列は解決時に消えるため、推測で復元せず現在ソースから依存元を再抽出する。
- 2026-10-17: 今回は計画文書だけを変更する。実装ブランチ作成、テスト実行、コード修正、commit/mergeは実装開始時に行う。

## Idempotence and Recovery

旧計画archiveを再生成/上書きしない。計画移行は専用の文書commitとし、以後の修正は問題ごとにcommitする。stage前に既存差分との境界を確認し、共有ファイルはhunk単位で扱う。全体のreset/clean/stashを使わない。

索引の失敗は既存のtransaction、linking_pending、更新leaseで回復させる。設定writeの失敗/キャンセルでは元ファイルを保持し、所有するtempだけ削除する。バージョン違いの索引へ旧バイナリを戻す場合は明示的な再索引を行い、古い派生DBを正常と偽らない。

再開時はこのPLAN、現在のbranch/HEAD、dirty paths、最後のRED/GREEN結果、未終了のテスト/レビューを照合する。検証対象のコードが変わった場合は影響するチェックをやり直す。今回の統合作業と無関係な既存worktree/branch/lockを削除しない。

## Outcomes & Retrospective

計画作成済み。24件の実装と最終検証は未着手。今回実行する検証は文書の対応表・参照パス・archive同一性・diffだけとし、文書変更のためにbuild/test/race/vetは実行しない。

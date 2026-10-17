# 全体再監査の修正計画（50件中、対象48件）

> **For agentic workers:** 実装時は `superpowers:executing-plans` を使い、以下のタスクを順番に実行する。並列実装は前提にしない。完了前に独立レビューを行う。

**Goal:** F01〜F50の再現条件を正式な回帰テストへ移し、境界違反・設定消失・索引不整合・抽出誤り・評価の見逃しを修正する。

**Architecture:** 現在のscanner／extractor／store／linker／compilerの責務を維持する。包含・設定保存・候補選択は入口で検証し、抽出器はトークンと原文座標を共有する。評価の適用条件は内部状態として持ち、公開形式へ推測用フィールドを足さない。

**Tech Stack:** Go 1.26、既存のmodernc SQLite・go-toml・MCP SDK・OSロック。新規依存なし。

**Spec:** `docs/audits/2026-10-17-whole-repository-50.json`。元監査のF01〜F50を変更せず対応付ける。

**Plan ID:** `whole-audit-50-20261017`。状態は**実装中**。本書がactive ExecPlan。前PLANを`docs/superpowers/plans/completed/2026-10-18-additional-19-before-audit50.md`へbyte-for-byte保存した。

## Purpose / Big Picture

2026-10-18のユーザー指示によりF02（秘密ファイルの除外迂回）・F03（DBへのアクセス境界）をサイバーセキュリティ項目として除外する。残り48件を対象とする。既存の当該2コミットは履歴を残して逆変更し、今回の統合結果に含めない。監査の44件は単体・補助関数を含む再現済み、6件（F02/F03/F04/F13/F14/F21）は静的確認であり、実装前に該当経路でREDを確認する。再現結果が監査と異なる場合は原因・契約・代替テストを記録し、根拠なく修正したりIDを別の問題に差し替えたりしない。

今回の到達点は50件の正式テスト、修正、最終検証、独立レビュー、承認済みのローカル統合手順まで。件数を満たすための変更は行わない。監査外の追加候補は別のbacklogに記録し、本計画の完了条件を暗黙に拡大しない。

## Global Constraints

- MCP公開フィールド、Evidence v1/v2、DBスキーマを維持する。原文bytes・行位置・非重複・wire予算は公開契約として守る。
- 新規依存・ネットワーク・外部LLM・解析対象コードの実行・package restoreを追加しない。検証対象はFocalSpan自身のテスト／ビルドと入力fixtureの解析に限定する。
- 既存の未コミット変更を保持する。対象パス／対象hunkのみstageし、`git add .`や一括resetを使わない。push・稼働バイナリ置換を行わない。
- 読み取り整合性と既存の5秒ロック／キャンセル／BusyErrorを維持する。外部設定編集の検出後は上書き再試行しない。
- supervisorの原因未確定EOF、監査で除外した再帰self-edge・outline行範囲・Zig文字列呼出し候補等は対象外。OSの協調しない同時link差替えまで完全に防ぐとは主張しない。

## Progress

- [x] 監査50 ID、現行の関連API、master基準と既存差分を確認した。
- [x] 50件を5段階の実装順と正式テストへ対応付けた。製品コードの変更・検証はまだ行っていない。
- [x] 実装開始時の状態保存、専用branch、active PLAN切替の文書コミット（5e175b6）。
- [ ] A〜Eの対象48件をRED→GREENで修正し、IDごとにコミット／証拠を記録。
- [ ] 通常→raceを直列実行し、vet・各ビルド・独立レビュー・FF統合を完了。

実装時はUTC時刻でProgressを追記し、各IDにREDコマンド／実際の失敗、GREENコマンド／結果、commit SHA、review結果を対応付ける。未実行の検証を成功扱いしない。

### 実装台帳

更新UTC: 2026-10-17T16:33:37.586503+00:00。対象48件中 48 件コミット済み。F02/F03の除外は6051885で逆変更。ログは`.superpowers/sdd/PLAN/`。

| ID | 状態 | RED | GREEN | commit |
|---|---|---|---|---|
| F01 | complete | F01-red.log | F01-green.log | 48f15e5c7a50 |
| F02 | excluded: cybersecurity (user) | — | — | 6051885 |
| F03 | excluded: cybersecurity (user) | — | — | 6051885 |
| F04 | complete | F04-red.log | F04-green.log | 7e299c1395ba |
| F05 | complete | F05-expanded-red.log | F05-green.log | 1eb9a99aaedd |
| F06 | complete | F06-red.log | F06-windows-green.log | dff6a2bca1f9 |
| F07 | complete | F07-red.log | F07-indexer-green2.log | 6f8d437 |
| F08 | complete | B-store-red.log | F08-green.log | 018644bbf6d4 |
| F09 | complete | F09-red.log | F09-green.log | ceffd75bc652 |
| F10 | complete | F10-red.log | F10-green.log | 151c5e6692bc |
| F11 | complete | F11-red.log | F11-app-green.log | 0339a074ca28 |
| F12 | complete | F12-red.log | F12-green.log | 2d3afaec7f75 |
| F13 | complete | F13-red.log | F13-green.log | 2d3afaec7f75 |
| F14 | complete | F14-red.log | F14-green.log | 8e73ac7e8c78 |
| F15 | complete | B-linker-red.log | F15-green.log | 81ca9780e2a0 |
| F16 | complete | B-linker-red.log | F16-green.log | 817cd1e1a84d |
| F17 | complete | B-linker-red.log | F17-green.log | 9405fe8a05ca |
| F18 | complete | B-linker-red.log | F18-green.log | aaccfff6f745 |
| F19 | complete | B-meta-red.log | F19-green.log | 34055780a018 |
| F20 | complete | B-meta-red.log | F20-green.log | 2df8df28b7bb |
| F21 | complete | F21-red.log | F21-green.log | fb15caba58d4 |
| F22 | complete | B-store-red.log | F22-green.log | b3128e755dc5 |
| F23 | complete | B-store-red.log | F23-green.log | 15ec6b79a597 |
| F24 | complete | F24-red.log | F24-green.log | 6d9e5a47b7b0 |
| F25 | complete | F25-red.log | F25-green.log | f424252328d1 |
| F26 | complete | F26-red.log | F26-green.log | eb08c3b3761e |
| F27 | complete | F27-red.log | F27-green.log | 29b2d19f57c9 |
| F28 | complete | F28-red.log | F28-green.log | b7743f5095dd |
| F29 | complete | F29-red.log | F29-green.log | 55407a7cb248 |
| F30 | complete | F30-red.log | F30-green.log | 6d9e5a47b7b0 |
| F31 | complete | F31-red.log | F31-green.log | e1cfc4524714 |
| F32 | complete | F32-red.log | F32-green.log | 42d6df7c95e6 |
| F33 | complete | F33-red.log | F33-green.log | 42d6df7c95e6 |
| F34 | complete | F34-red.log | F34-green.log | e05b79839da4 |
| F35 | complete | F35-red.log | F35-green.log | e05b79839da4 |
| F36 | complete | F36-red.log | F36-green.log | e05b79839da4 |
| F37 | complete | F37-red.log | F37-green.log | e1cfc4524714 |
| F38 | complete | F38-red.log | F38-green.log | 012d2c0505c4 |
| F39 | complete | F39-red.log | F39-green.log | 2623b2684a6e |
| F40 | complete | F40-red.log | F40-green.log | 02b90ff98332 |
| F41 | complete | F41-red.log | F41-green.log | 0d1a5e7a846e |
| F42 | complete | F42-red.log | F42-green.log | eb08c3b3761e |
| F43 | complete | F43-red.log | F43-green.log | 29b2d19f57c9 |
| F44 | complete | F44-red.log | F44-green.log | eb08c3b3761e |
| F45 | complete | F45-red.log | F45-green.log | 29b2d19f57c9 |
| F46 | complete | F46-red.log | F46-green.log | 2623b2684a6e |
| F47 | complete | F47-red.log | F47-green.log | 790cab79b15b |
| F48 | complete | F48-red.log | F48-green.log | 790cab79b15b |
| F49 | complete | F49-red.log | F49-green.log | 8e38b90d27a1 |
| F50 | complete | F50-red.log | F50-green.log | f9a4d736a3ea |

## Surprises & Discoveries

監査には重要な境界がある。F23は空DBでは再現せず、FTSに1件以上入ったDBで構文エラーになる。F09は9言語でキャンセル後のnil errorを確認したが、C++の同じテストはPASSだったためC++を不具合対象に数えない。F15/F16/F18はhelperで再現したので、正式化時はStore→Linker→検索の経路も確認する。

元監査の一時overlayは有用な種だが、`C:/Temp`の存在を実行条件にしない。永続化した監査JSONの入力・期待動作から正式fixtureを作り、テストはテストデータのみを読み込む。

## Decision Log

- 2026-10-18 JST: 独立した言語解析・CLI評価をparallel-agents skillに従って担当分離。共有lexer/parserの複数IDは言語単位の統合コミットとして明示し、各IDのRED/GREENを個別記録する。F19/F20は構造化manifest読取、F15/F16はpackage同定の対照を追加して不十分な初回修正を補完。


- 2026-10-18 JST: ユーザーがサイバーセキュリティ項目の除外を指示。F02/F03を除外し、他48件は通常の正確性・データ保持・操作性の修正として継続する。除外した既実装差分は逆変更する。


| 決定 | 理由・影響 |
|---|---|
| 50件を1 milestone、各IDを1修正タスクとする | 共通前提を共有しながら、完了・テスト・commitの追跡を独立させる。単一原因の6言語EOFはF01の1件。 |
| F05の既存global登録の置換はTOMLの原子的保存へ変更 | remove→addや失敗後の再追加では元の全設定を保持できず、外部編集との競合も生じるため。新規登録・通常削除は既存CLI経路を維持する。 |
| F04は新しいv2 baselineを実測して追加 | v1をschema名だけ変更するとN19の正解順位の意味が一致しない。旧baselineは変更しない。 |
| F49は内部の展開実施状態を追加 | 数値0だけでは「未実施」「対象ラベルなし」「全欠落」を区別できない。公開reportフィールドは追加しない。 |
| 抽出／リンクの変更後は既存索引を再抽出させる | 修正が新規全索引だけに効くことを防ぐ。最終統合でExtractorVersionを更新し、既存索引の差分更新と全再構築を比較する。 |

2026-10-17 UTC Ruling: ユーザーがローカルLinux代替を選択し、WSL-Ubuntuを指定した。実file symlinkの受入れはUbuntuで行い、Windowsはnative回帰と既存junction受入れを行う。Windowsの権限依存file symlinkテストのskipをWindows実受入れPASSとは数えない。UbuntuにはGoを追加せずWindowsでcross-buildしたtest binaryを実行する。この差分を独立レビュー対象にする。

## Context and Orientation

作成時の基準は`master@9400ffb55e11d65de773a48df49db69eb7f2e067`。既存差分は`AGENTS.md`、`README.md`、`docs/design.md`、`internal/mcpserver/context_v2_test.go`、`internal/mcpserver/server.go`、未追跡の`.agent-state/`、`.focalspan.json`、`TASKS.md`。内容・HEADが変わっていたら現状を読み直して適応し、元に戻さない。

| 領域 | 変更責務 | 主な正式テストの配置 |
|---|---|---|
| 境界・保存 | repository、store lifecycle、Codex設定、benchcli出力 | 各packageの`audit50_test.go`、OS別テストファイル |
| 索引・検索 | indexer、store links/candidate取得、linker、projectmeta、search | 各packageの`audit50_test.go`、`testdata/audit50/` |
| 言語解析 | extract配下のlexer/parser/builder、Registry | 各言語の`audit50_test.go`、共通conformanceテスト |
| CLI・評価 | cli、evalcli、benchmark、eval、CI | 各packageの`audit50_test.go`、`testdata/benchmark/`、workflow |

新規の製品helperは既存の`internal/*`の担当packageに置く。大規模なファイル分割・抽出器統合・汎用AST導入は行わない。

## Review Focus

1. **link・秘密ファイル・DB副ファイル:** 無害な別名、DBの葉、WAL／journal／migration一時ファイルを使って境界を迂回できないことをF02/F03で検査する。
2. **設定保存の中断・競合:** add失敗だけでなく、保存直前キャンセル・外部編集・別process待機で旧内容と無関係設定が保持されることをF05/F06/F10で検査する。
3. **有効コードと非コード:** EOF、CRLF、BOM、escaped quote、CDATA、heredoc、PHP境界の両側をF01/F25〜F39/F43/F46で検査する。
4. **既存索引との一致:** 新規シンボル・削除・大文字小文字・大量条件・相対importで、増分更新と全再構築の関係集合が一致することをF07/F08/F15〜F23で検査する。
5. **数値0と未実施の区別:** 検索空結果・展開anchorなし・正解全欠落・展開不要をF23/F47〜F50で検査し、評価器が成功を捏造しないことを確認する。

## Plan of Work

### 0. 実装開始時の準備

1. HEAD／branch／statusを再取得する。既存差分と未追跡のユーザーファイルを読み取り専用でsnapshotし、SHA256と保存先を記録する。実行用fixtureや本物のユーザー設定として流用しない。
2. `codex/whole-audit-50-fixes`を作成する。同名branchがあれば内容を確認し、削除・上書きせず適切に再開する。隔離が必要な場合は既存の適切なworktreeを優先し、ユーザー変更を持ち込まずに扱う。
3. 現ルートPLANを`docs/superpowers/plans/completed/`の一意な名前でbyte-for-byte保存し、SHA256一致を確認する。本提案をルート`PLAN.md`へ移し、以後はルートだけを更新する。文書切替を独立commitにする。
4. `testdata/audit50/`に入力fixtureを置く。`/C:/Temp`等の一時overlay依存や解析対象言語の実行は加えない。元監査と正式テスト名の対応表をProgressに追加する。
5. A→B→C→D→Eの順に実行する。各行は独立した修正タスクであり、以下の共通5手順を適用する。

### 全タスク共通の5手順

- [ ] **RED作成:** 下表の正式テストを先に追加する。修正対象の入力だけでなく、表中のpositive controlも含める。
- [ ] **RED確認:** 対象packageで`go test <package> -run '^TestFxx' -count=1 -v`を実行し、コンパイル成功後、狙ったassertionでFAILすることを記録する。
- [ ] **最小修正:** 指定された方針とInterfacesに沿って変更する。新たな共通前提が必要なら理由・利用ID・独立commitを記録する。
- [ ] **GREEN確認:** 同じコマンドと関連packageのテストを実行してPASSを確認する。抽出結果は原文byte slice／行位置／関係も検査する。
- [ ] **commit:** IDの関連パスだけstageし、`fix(audit50): Fxx <内容>`でcommitする。共有ファイルは他のIDや既存ユーザー変更を混ぜない。

以下の各`TestFxx...`は必須の主テスト名。必要なsubtestや統合テストを追加してよいが、名前だけの空テスト・実装をなぞるだけのassertionにはしない。

### A. クラッシュ・境界・保存・共通入口（8件）

| ID | 変更ファイル | 修正内容 | 正式テスト・完了条件 |
|---|---|---|---|
| F01 | `extract/{cpp,csharp,jsts,lua,pawn,zig}/lexer.go`（全て`internal/`配下） | escape後のカーソルをEOF超過させない。未終端は原文内に収め、既存形式の診断を返す。panicをrecoverして正常化する修正にはしない。 | 各言語`audit50_test.go`の`TestF01UnterminatedEscape`。2 bytes `{34,92}`、単引用符／JS backtickの該当形式、正常なescapeを検査。全6lexerと公開Extractがpanicせず、範囲が入力内。 |
| F05 | `internal/integration/codex/service.go`, 新規`user_config.go`, `project_config.go`の保存helper | 既存global登録のforce更新はremove→addを廃止。UserConfigPathの対象tableだけをTOML文字列・quoted table名・子tableを理解して置換し、既存の設定ロック／atomicWrite／ConfigConflictErrorを再利用。CLI結果と保存元tableが対応しなければ変更前に競合error。無関係なtable・コメント・改行・modeは保持。 | `codex/audit50_test.go`の`TestF05ForceUpdatePreservesOriginalOnFailure`。rename失敗・キャンセル・外部編集・別process lock・CRLF・quoted name・nested envを検査。既存設定を消すremove/addが呼ばれず、失敗時の旧ファイルはbyte不変。成功時は対象登録だけ置換。 |
| F06 | `internal/benchcli/run.go` | 固定名`.tmp`を廃止し、同一directoryの`os.CreateTemp`で所有する一時ファイルだけを作成／cleanup。最終出力のforce=false検査は原子的なno-clobber publishを内部OS別helperで保証する。既存出力や別人のtmpを消さない。 | `benchcli/audit50_test.go`の`TestF06OutputTempOwnership`。既存`.tmp`のsentinel、同時writer、出力が保存直前に作られる場合、write/close/publish失敗を検査。force=falseは既存出力を保持し、force=trueだけ明示置換。 |
| F09 | `internal/extract/{csharp,jsts,python,ruby,rust,nim,zig,xaml,resx}`のExtract wrapper／builder | builder前後でctx.Errを確認し、途中キャンセルをnil errorにしない。必要なbuilderだけerror戻り値を持たせる。Registry経由と各公開Extractorの両方で保証。部分結果を保存可能な成功として返さない。 | `extract/audit50_test.go`の`TestF09BuilderCancellation`。実contextを各チェック境界でcancelする決定的gateを使い、9言語でerrors.Is(context.Canceled)。C++は現行PASSのcontrol。indexerの既存データ／完了metadataも不変。 |
| F10 | `internal/integration/codex/user_cli.go`, `service.go` | globalの正規化された登録にenabled・transport type・cwd・env・env_vars・timeout／tool制限の存在と値を保持する。旧CLIの省略値は既存の既定動作として扱い、明示false／固定cwd／環境注入を一致としない。Status、Installのunchanged、legacy管理判定で同じ比較を使う。 | `codex/audit50_test.go`の`TestF10GlobalExecutionDrift`。旧形式／現形式JSON、欠落既定値、disabled、cwd、env、timeout、tool制限を検査。通常登録だけmatchし、変更された実行条件はdriftとなる。F05でforceした後にmatchへ戻る。 |
| F11 | `internal/extract/extract.go`, 各Extractorの`Supports`, `internal/app/service.go`のregistry構成 | languageが非空ならその言語に対応するExtractorを優先し、拡張子fallbackはlanguageが空の場合だけ。既存言語名を維持し、text／未知言語はgenericへ渡す。Nim等のSupportsも言語指定を尊重させる。 | `extract/audit50_test.go`の`TestF11LanguageOverrideDispatch`。実production registryで`.go=python`、`.cs=text`、未知拡張子=nim/zig/resource、language空の拡張子fallback。単なるForの型判定だけでなく、scanner→Extractで期待宣言が得られる。 |

F02/F03はユーザー指示による除外対象であり、当該修正・受入れ検査は今回の完了条件に含めない。

### B. 索引・関係・metadata・検索（11件）

| ID | 変更ファイル | 修正内容 | 正式テスト・完了条件 |
|---|---|---|---|
| F07 | `internal/indexer/indexer.go`, `internal/store/links.go` | 新規／変更symbolのlookup keyに依存する既存resolved関係のsource pathを取得し、未変更sourceも再抽出して関係を再生成する。to_handleが非nullという理由で依存元を除外しない。既存の変更・削除target依存closureと統合する。 | `indexer/audit50_test.go`の`TestF07AddedTargetMatchesRebuild`。unique Foo→別fileにFoo追加→削除／renameの各段階で、増分と全再構築の関係集合・確定度が一致し、誤確定しない。 |
| F08 | `internal/store/links.go`、F07の依存元取得helper | Paths／LookupKeysをsort/dedupしてJSON配列としてbindし、既存SQLiteの`json_each(?)`で展開する。条件数分のplaceholder生成を廃止。DBスキーマ変更なし。 | `store/audit50_test.go`の`TestF08LargeRelinkScope`。4万key、paths/key混在、duplicate、空scope、キャンセル。小分け参照実装と結果集合・順序が一致し、SQL variable errorなし。 |
| F15 | `internal/linker/linker.go` | wildcardなしaliasは文字列全体一致。`*`があるaliasはprefixとsuffixの両方を照合し、対応する部分だけtargetへ代入する。正規化後の包含を確認する。 | `linker/audit50_test.go`の`TestF15ExactAndWildcardAliases`。`@cfg`と`@cfgExtra`、`@x/*/tail`、baseUrl、複数target・曖昧候補。helperと実Store経由の両方で誤リンクなし。 |
| F16 | `internal/linker/linker.go`, 必要なら`internal/projectmeta/discover.go` | fact kind別にimport名とpackage／export／参照名の対応を検査し、defaultで全path-valued factを返す処理を廃止。不明kindや対応不明なら未解決を維持。既存manifest identityを使い、推測で単一候補へ接続しない。 | `linker/audit50_test.go`の`TestF16UnrelatedManifestDoesNotResolve`。missing-package＋main/module/project-reference、正しい自己package import、複数package。無関係なfactでは未解決、対応が証明できる正例のみ解決。 |
| F17 | `internal/store/links.go`, `internal/store/store.go`のlookup生成 | 全リンクprojectionで安全に候補を絞れないpath系import/exportを落とさずresolverへ渡す。シンボルkeyだけで判定可能な最適化は残す。最適化の有無で意味を変えない。 | `linker/audit50_test.go`の`TestF17ProjectedAndScopedLinkAgree`。manifestなしで`src/main.ts`→`./dep.ts`、relative parent、root file。full／projection無効／scope付きで同一関係集合。 |
| F18 | `internal/linker/linker.go` | importer directoryとrelative importを先に結合し、正規化した後で候補の拡張子なしpathとも照合する。複数拡張子が競合した場合は勝手に選ばず未解決。root外への正規化は拒否。 | `linker/audit50_test.go`の`TestF18ExtensionlessRelativeImport`。`./dep`／`../dep`／拡張子あり、複数候補、対象外path。Storeからの完全リンク・限定リンク・検索関係が一致。 |
| F19 | `internal/projectmeta/discover.go` | Cargoのkeyに一致しなかった時のfirstQuoted fallbackを削除。package tableのnameと、dependencies等のinline/subtableのpathを区別して読む。TOMLは既存ライブラリを利用し、通常文字列を依存先にしない。 | `projectmeta/audit50_test.go`の`TestF19CargoAssignmentScope`。package name/version、dependency version/path、comment、escaped string、workspace table。version由来factなし、実path依存だけ残る。 |
| F20 | `internal/projectmeta/discover.go` | Composerのautoload／autoload-dev、psr-0／psr-4でstringとstring配列を受け入れ、各directoryのfactを正規化・dedup。非string要素は診断し誤factを生成しない。 | `projectmeta/audit50_test.go`の`TestF20ComposerArrayMappings`。複数directoryと文字列形式の同値性、空配列、重複、混合型。manifestだけ変更した増分と全索引も一致。 |
| F21 | `internal/linker/linker.go`, `internal/store/links.go`, file lookup生成 | import pathと索引の正規化相対pathはcase-sensitiveに照合し、無条件ToLower／EqualFoldを除く。Windowsの互換case補正は同じ実在ファイルと確認できる入口に限定し、異なる索引file identityを合流させない。symbol名の既存言語別ポリシーは変更しない。 | `linker/audit50_test.go`の`TestF21ImportPathCaseIdentity`。LinuxでFoo.ts／foo.ts併存、片方だけ、wrong case。Windowsは同一実体の通常表記をcontrolとし、別fileの誤解決なし。lookup再構築後も同じ結果。 |
| F22 | `internal/store/store.go`, `candidate_scope.go`, `internal/rank/rank.go`の既存補正 | 全candidate projection／Scanの列対応へestimated_tokensを含める。FTS、symbol、path、relation、scope付きの各経路を揃え、既存のlarge-span補正を有効にする。 | `store/audit50_test.go`の`TestF22EstimatedTokensRoundTrip`。5001の保存・全経路取得・rank reason確認、小さいchunk control。位置・content・handleの列ずれがない。 |
| F23 | `internal/search/retrieval.go`, `internal/store/store.go`, `candidate_scope.go` | BuildFTS結果が空ならFTS retrieverを呼ばず空listを返す。Storeの直接SearchFTSにも同じguardを置く。symbol/path/relationの他retrieverは続行し、raw queryが非空という理由でSQLiteへ空MATCHを渡さない。 | `search/audit50_test.go`の`TestF23NoSearchableTerms`。非空FTS索引で`!!!`、全空白、通常語、scope、FTS-only。構文errorなし、通常検索は従来順位、直接Store呼出しも安全。 |

### C. literal・comment・領域境界（10件）

| ID | 変更ファイル | 修正内容 | 正式テスト・完了条件 |
|---|---|---|---|
| F25 | `internal/extract/rust/lexer.go` | 有効char literalをlifetimeより先に認識し、escape・Unicode・区切り文字を1tokenにする。不正literalは診断し構造delimiterを捏造しない。 | `rust/audit50_test.go`の`TestF25CharLiteralScope`。`'{'`／`'}'`／`'\n'`／escaped quote／Unicodeと、`'a` lifetime。後続関数が独立し、範囲が正しい。 |
| F27 | `internal/extract/csharp/lexer.go` | 通常string、char、verbatimの終端規則を分ける。verbatimの`""`だけをquote escapeとし、backslashをescapeにしない。 | `csharp/audit50_test.go`の`TestF27LiteralEscapeModes`。末尾backslashのverbatim、doubled quote、escaped apostrophe、通常string、未終端。後続Fが存在し原文sliceが一致。 |
| F28 | `internal/extract/nim/{extractor,parser,builder}.go` | lexerのtriple string／nested comment範囲を宣言・関係解析へ渡し、コード部分のみ検索する。置換する場合もbytesと改行を保持する。 | `nim/audit50_test.go`の`TestF28NonCodeIgnored`。triple string、`#[ ]#`入れ子、通常文字列の偽proc／callを除外し、隣接する本物は抽出。 |
| F29 | `internal/extract/ruby/{lexer,extractor}.go` | heredocとcolumn規則に沿うblock commentをlexerで区切り、raw line parserへ非コードを流さない。複数heredoc／quoted delimiter／indented terminatorも既存対応範囲で扱う。 | `ruby/audit50_test.go`の`TestF29HeredocAndBlockComment`。本文のdef/end/call、`=begin`／`=end`、本物の後続def。偽宣言・関係なし、行座標不変。 |
| F30 | `internal/extract/vb/{common,vbnet,vb6,builder}.go` | 文字列／commentを無効化したcode viewとraw signatureを分離し、宣言・call regexはcode viewだけに適用。二重引用符escapeとline継続を保持する。 | `vb/audit50_test.go`の`TestF30StringsAreNotDeclarations`。VB.NET／VB6で文字列中のClass／Sub／callを除外、実宣言とコメント付き文字列は正常。 |
| F31 | `internal/extract/lua/{builder,parser}.go` | raw lineからのrequire検索をやめ、code上のrequire callとliteral引数のtoken位置からimportを作る。literal値の取得とコード検索を分ける。 | `lua/audit50_test.go`の`TestF31RequireOnlyInCode`。comment、long string、通常文字列中の偽requireなし。通常requireの正例と非literal引数の保守的扱いを検査。 |
| F32 | `internal/extract/resx/{scanner,builder}.go` | encoding/xmlのtoken境界を用い、実際のdata要素だけをresourceとして扱う。コメント／CDATAをmarkupとして再走査しない。InputOffsetと元bytesを使い座標を保持。外部entityを取得しない。 | `resx/audit50_test.go`の`TestF32XMLRegions`。comment／CDATAのFakeを除外しRealのみ抽出。namespace、escaped attribute、CRLF、壊れたXMLで偽resourceを作らない。 |
| F38 | `internal/extract/php/lexer.go` | `//`／`#` commentはnewlineまたは実際の`?>`で終え、PHP mode遷移はmain loopへ戻す。block comment／quoted stringの規則を混同しない。 | `php/audit50_test.go`の`TestF38ClosingTagInLineComment`。終了後のHTML function文字列を除外し、後続の再開PHPブロックの本物だけ抽出。 |
| F39 | `internal/extract/template/scanner.go`, 必要ならPHPの内部lexical helper | embedded PHPの終端はPHPのstring／comment／heredoc状態を使って判定する。F38と同じclose-tag規則を共有し、raw文字列検索で最初の`?>`を選ばない。 | `template/audit50_test.go`の`TestF39PHPRegionBoundary`。PHP文字列中の`?>{block...}`は非Smarty。line comment中の実終了tag、block comment、通常Smarty／PHP再開も検査。 |
| F43 | `internal/extract/{cpp,csharp}/lexer.go` | preprocessor stackに親の有効状態と既知のbranch選択を保持。literal falseのelifを有効にせず、既知true後のelseも除外。未知macroを実行／完全評価しない。 | 各言語`audit50_test.go`の`TestF43ConditionalBranches`。if false→elif false→else、elif true、入れ子、未知条件。無効分岐の宣言なし、選ばれた分岐だけ存在。 |

### D. 宣言・scope・依存抽出（13件）

| ID | 変更ファイル | 修正内容 | 正式テスト・完了条件 |
|---|---|---|---|
| F24 | `internal/extract/vb/vbnet.go` | semantic kindとは別にSub／Functionの終端種別を保持し、constructor NewはEnd Subで閉じる。既存階層回復を維持する。 | `vb/audit50_test.go`の`TestF24ConstructorTerminator`。Newの終端は3行目、Afterの親はclass。通常Sub／Function control、未終端診断。 |
| F26 | `internal/extract/cpp/parser.go` | type keywordから最初の構造delimiterまでを解析し、`;`が先なら前方宣言として終了する。後続関数の`{`を借用しない。 | `cpp/audit50_test.go`の`TestF26ForwardDeclarationBoundary`。struct/class/enum前方宣言、通常定義、後続関数。Aはsemicolonまで、fが存在。 |
| F33 | `internal/extract/resx/{scanner,builder}.go` | F32の実data要素でtype=System.Resources.ResXFileRefを認識し、valueの先頭パス成分をimportへ変換。旧対応形式も壊さず、XML entityはtoken値としてdecodeする。 | `resx/audit50_test.go`の`TestF33StandardFileRef`。標準型属性＋value、空／不正値、通常resource、旧fixture。asset依存と原文座標を検査。 |
| F34 | `internal/extract/jsts/parser.go` | importの構文を必要なmodule literalまで消費して終了し、次のstatement全体をstring探索しない。multiline named importは維持する。 | `jsts/audit50_test.go`の`TestF34SemicolonlessImportBoundary`。import直後のmessage文字列、side-effect import、multiline named import、semicolonあり。対象moduleとaliasのみ。 |
| F35 | `internal/extract/jsts/parser.go` | type aliasのRHSはbalanced delimiterを消費し、object／tuple／union／generic内部のsemicolonで終了しない。外側statementの境界を区別する。 | `jsts/audit50_test.go`の`TestF35ObjectTypeAlias`。User object、nested object、union、後続function。kind=type、完全なsource span。 |
| F36 | `internal/extract/jsts/parser.go` | bare identifier parameter→arrowと、括弧付きparameter→arrowを別分岐で認識する。async／expression body／block bodyの既存形式も維持する。 | `jsts/audit50_test.go`の`TestF36SingleParameterArrow`。`value => value*2`、async、括弧付き、通常変数。doubleがarrow_functionで完全なspan。 |
| F37 | `internal/extract/lua/parser.go` | elseifのthenを新規blockとせず同じif branchへ遷移させる。function／if／do／repeatの閉じ方を区別する。 | `lua/audit50_test.go`の`TestF37ElseifScope`。複数elseif、nested if、後続g。fは自身のendで閉じ、gは兄弟。 |
| F40 | `internal/extract/python/extractor.go` | 既存のscope／token情報からparameter・local assignment・import等のbindingを先に収集する。遮蔽されたbare callはglobalへ結ばず、receiver付きcallはreceiverを捨てない。現在classのself/cls以外の動的属性は保守的に未解決にする。 | `python/audit50_test.go`の`TestF40CallBindingScope`。parameter／local／importによる遮蔽、other.target、self.target、普通のglobal call、nested scope。誤resolved edgeなし、確定可能な正例は維持。 |
| F41 | `internal/extract/goast/go.go` | TypeSpec.Assignがある場合はtype_aliasを優先し、RHSがstruct/interfaceでも後から上書きしない。 | `goast/audit50_test.go`の`TestF41AnonymousTypeAliases`。匿名struct/interfaceへのalias、named alias、通常定義。alias kindとsignature／sourceが正しい。 |
| F42 | `internal/extract/cpp/parser.go` | namespace修飾の有無ではなくclass／struct所有者でmethodを判定する。namespace内自由関数はfunctionとする。 | `cpp/audit50_test.go`の`TestF42NamespaceFreeFunctionKind`。demo::f、class内method、class外定義、nested namespace。自由関数とmethodが正しく区別される。 |
| F44 | `internal/extract/cpp/extractor.go` | receiverParentと関連所有者探索をC++のcase-sensitive identityへ揃える。Aとaを同一視しない。 | `cpp/audit50_test.go`の`TestF44ReceiverCaseIdentity`。A／aとa::f、namespace付きreceiver、未定義receiver。親handleは小文字aのみ。 |
| F45 | `internal/extract/csharp/parser.go` | namespaceとtype nestingを別々にstackへ保持し、qualified nameに外側type列を含める。ParentHandleとnameを一致させる。 | `csharp/audit50_test.go`の`TestF45NestedTypeQualification`。N.Outer.Inner.F、別Outer内の同名Inner、file-scoped namespace。qualified name衝突なし。 |
| F46 | `internal/extract/template/parser.go` | Smarty attributeをkey／equals／valueとして解析し、等号前後の空白を独立して許可する。quoted value内部を分割しない。 | `template/audit50_test.go`の`TestF46AttributeWhitespace`。`file='x'`／`file = 'x'`／`file= 'x'`／`file ='x'`が同じimport。空白入りvalue・escaped quote・positional形式のcontrol。 |

### E. CLI・評価・CI（8件）

| ID | 変更ファイル | 修正内容 | 正式テスト・完了条件 |
|---|---|---|---|
| F12 | `internal/cli/run.go` | substring判定をやめ、setupが管理するroot index directoryの正の除外ruleを既存行の後に確保する。コメント／否定／別directoryの文字列は一致扱いしない。設定path中のGit pattern文字をescapeする。 | `cli/audit50_test.go`の`TestF12EffectiveGitignoreRule`。comment、negation、nested-only、CRLF、空白入り／pattern文字入りpath。再実行で重複せず、実Gitのcheck-ignoreでDB／lockが除外される。 |
| F13 | `internal/cli/run.go` | update／serveともFlagSet.Parse後に残引数を検査し、service生成／更新／process起動の前に拒否する。 | `cli/audit50_test.go`の`TestF13RejectUnexpectedArguments`。`typo --root other`、`--root root typo`、正規flags。errorでcwd／他repoのDBを作らず、serve workerも起動しない。 |
| F14 | `internal/evalcli/run.go`, `cmd/focalspan-eval/main.go`の呼出し | 内部runへstderrを渡し、flag error／usageをstderrへ出す。明示helpはstdoutへ正常出力し、JSON成功時stdoutはJSONだけにする。 | `evalcli/audit50_test.go`の`TestF14DiagnosticStreams`。unknown flagはnonzero＋stdout空、--helpは0、通常--jsonはparse可能でstderr空。 |
| F47 | `internal/benchmark/compare.go` | v2比較でTargetRank／ReciprocalRankの低下も比較する。rank=0はmissで最悪、正rankは小さいほど良い。v1同士は旧比較規則を維持する。 | `benchmark/audit50_test.go`の`TestF47RankRegression`。1→5、1→0、0→1、同じrecall、v1/v1、v1/v2拒否。v2だけ正しくregression／improvement。 |
| F48 | `internal/benchmark/compare.go` | v2で展開path/symbol recall、forbidden violations、relation validityを比較する。初回検索の改善で展開悪化を相殺しない。 | `benchmark/audit50_test.go`の`TestF48ExpansionRegression`。展開1→0／partial、禁止path混入、relation不正、初回改善＋展開悪化、両方展開なし。同一matrixで独立にregression。 |
| F49 | `internal/benchmark/{failure,runner,expand}.go` | 内部ExpansionAssessmentを使い、要求数・実行数・anchor不足・path/symbol正解数を保持する。既知のexpansion_anchor_missingを再生成で捨てず、展開要求があり全欠落でもfailureにする。複数expandは件数加重で集約し、最後の結果で上書きしない。 | `benchmark/audit50_test.go`の`TestF49ExpansionFailureAccounting`。anchor不足、0/partial/全成功、symbol-only期待、展開なし、2展開の一方失敗と順序逆転。failure codeは安定順で重複なし、未実施を成功としない。 |
| F50 | `internal/eval/evidence.go` | focused caseで指定された各期待path/symbolのContainsを、対応するEvidence上ですべて満たした場合だけ1。未充足は0。非focusedとContains指定なしは従来の非対象扱い1を維持する。 | `eval/audit50_test.go`の`TestF50LateHitPreservation`。empty packet、全欠落、一部のみ一致、別file同文、excerpt内late hit、複数期待。欠落は0、正例は1。 |
| F04 | `.github/workflows/ci.yml`, 新規`docs/benchmarks/results-whole-audit-baseline-v2.json`, benchmark説明文書 | F47〜F50後に比較規則を固定し、基準9400ffbのFocalSpan実行ファイルを隔離checkoutでbuildして同じhistory suite／profiles／budget／repeatからv2 baselineを実測。旧results-v0.5.jsonは保持。smoke/fullとも新baselineへ変更し、絞り込みmatrixを一致させる。 | `benchmark/audit50_test.go`の`TestF04CIBaselineCompatibility`とCLI compare受入れ。checked-in baseline schema=v2、provenanceの生成commit=9400ffb、CIとsuite/matrix一致。v1のschema文字列置換で作らない。candidate比較はincompatible=3にならず、真のregressionは従来どおりnonzero。 |

F04の基準checkoutはユーザーの作業treeと分離し、build先も一時directoryとする。既存のGit objectに必要なhistoryがない場合はネットワークで補完せず欠落commitを報告し、baselineを捏造しない。旧reportの数値やschemaは書き換えない。

### 最終統合の共通前提

全抽出／リンク修正後、`internal/indexer/indexer.go`のExtractorVersionを`extractors-v18-audit50-extraction-linking`へ更新する独立commitを作る。F07/F11/F15〜F46の利用前提としてPLANへ記録し、旧versionの索引に対してStatus/Healthが更新要求を示すこと、updateで全ソースとlookup／関係が再生成されることを正式テストで確認する。

新versionでの初回更新中にcancel／errorが発生した場合も、未完了をfreshにしない。回復後にfreshへ戻り、同じ作業treeの新規全索引とsymbols/chunks/relations集合が一致することを受け入れ条件に加える。

## Interfaces and Dependencies

既存の公開形は維持する。以下は`internal/*`内だけの変更であり、MCP／DB／Evidence schemaを追加しない。

| 提供側 | 内部interface・責務 | 利用側 |
|---|---|---|
| Store／F07 | `DependentSourcePaths(ctx context.Context, lookupKeys []string) ([]string, error)`。resolvedを含むrelation_lookupからsource pathを安定順で返す。F08と同じJSON条件方式。 | Indexerの既存依存closure。変更sourceとのunionを強制再抽出。 |
| Codex／F05 | `replaceUserRegistration(ctx context.Context, configPath, name string, spec RegistrationSpec, observed userRegistration) (OperationResult, error)`。ロック取得後にraw内容・登録identityを検査し、対象tableの置換をatomicWriteへ渡す。 | installUserの既存登録force分岐。新規登録／削除のCLI APIは維持。 |
| Codex／F10 | userRegistrationへ実行条件とpresenceを内部保持。`sameUserRegistration`／`managedUserRegistration`は同じ正規化された比較に統一。global既定値とproject明示設定の比較を混同しない。 | Status、Install、legacy migration。既存RegistrationStatusフィールドを利用。 |
| Extract／F09/F11 | `Extractor.Extract(context.Context, model.SourceFile) (model.Extraction, error)`とRegistryのsignatureは維持。内部builderのerror伝播だけ調整。Supportsは非空languageを優先する契約。 | すべての言語解析とIndexer。 |
| Benchmark／F49 | `type ExpansionAssessment struct { Requested, Completed, MissingAnchors, RequiredPaths, MatchedPaths, RequiredSymbols, MatchedSymbols int }`、`FailureCodesWithExpansion(r QualityResult, assessment ExpansionAssessment) []string`を追加。既存FailureCodesは互換wrapperを維持。 | Runnerが展開ごとにassessmentを加算してから判定。公開QualityResultは既存fieldのみ使用。 |

F05/F10のglobal用の期待値は、現在の`codex mcp add NAME -- COMMAND ARGS...`が指定しているcommand／argsと既定の有効stdio実行条件である。project用RegistrationSpecのtimeout／enabled_toolsを、globalに新たに強制しない。global更新後はcommand／args／enabled=trueを保存し、古いcwd／env／env_vars／timeout／tool制限の明示overrideは対象tableから除く。省略された既定値とnull／空の等価性は旧／現JSON fixtureで固定する。dry-runはOperationResult.Blockに置換対象の新tableを示し、実行しないremove→addをArgvに掲示しない。秘密の旧env値を診断へ出さない。

依存順はA→B→C→D→E。特にF05はF10の修正後に成功状態の統合検査を追加する。F32→F33、F38→F39、F11→各言語の公開経路確認、F07/F08→旧索引更新の集合比較、F47/F48/F49/F50→F04を固定する。各IDのcommitは1件を基本とし、必要な共通前提commitは利用IDを明記する。

F06のno-clobber helperはGo標準APIと既存x/sysだけを使う。Unixは同一filesystemのexclusive link publish、Windowsは既存先を置換しないOS renameを使い、force=trueの置換とは分ける。非対応filesystemでは既存ファイルを上書きせずerrorを返す。原子性のないstat→rename fallbackを追加しない。

## Validation and Acceptance

### 各修正と5段階の受け入れ

各IDで共通RED/GREENを実行する。Aでは外部sentinel・旧設定保持、Bでは増分／全索引の集合一致、C/Dではsource bytesと構造、Eでは評価の正負と実際のCLI exit codeを確認する。テストは時刻待ちの偶然ではなくchannel／gate／fake filesystem操作境界で同期させる。

抽出が変わる全言語に対して、共通conformanceで`Content == file.Content[StartByte:EndByte]`、byte境界、LF/CRLF/BOM、行位置、handleの安定性を確認する。synthetic outlineはverbatimとして判定しない。Evidence v1/v2について、対象symbol・caller・importの原文と位置、不変条件、serialization後のmodel-visible wire予算を検査する。新しい公開fieldやranking debug fieldを出さない。

### 最終コマンド

依存はローカルcacheのみ使用し、必要なら`GOPROXY=off`、`GOSUMDB=off`、`GOTOOLCHAIN=local`を設定する。cache不足は復元せず報告する。shell commandはRTK規則に従う。

1. `go test ./...`を完了させてから、`go test -race ./...`を実行する。通常とraceを同時に走らせない。race用C compilerは既知の利用可能な環境を明示する。
2. `go vet ./...`、`go build ./cmd/focalspan`を実行する。benchmark/evalも含むnative buildは`go build -o <temporary-output-directory> ./cmd/...`で確認し、稼働binaryを上書きしない。
3. `CGO_ENABLED=0`でWindows amd64、Windows arm64、Linux amd64、Darwin arm64について、`./cmd/focalspan`、`./cmd/focalspan-bench`、`./cmd/focalspan-eval`を各々一時保存先へbuildする。cross-buildはOS runtime testの代わりにしない。
4. F02/F03/F21はWSL-Ubuntuの実symlink／大小文字受入れとWindows native通常回帰／既存junction受入れで検査し、設定の別process lockテストも実行する（Decision Logのユーザー指定によるOS代替）。スキップや未実行は明示し、必須OS受入れが残る間は統合しない。benchmark/evalは解析対象コードを実行せず、F04のCLI比較まで検証する。
5. 50 IDのテスト／commit対応、Evidence契約、設定喪失、境界、増分整合性を独立レビューする。指摘修正後は影響検証を再実行し、最終sourceで全通常→全raceを再確認する。`git diff --check`と既存ユーザー変更の保持も確認する。

開始済みの確認を同じsourceで理由なく繰り返さない。追加修正・未解決の懸念があれば、その影響に応じて再検証する。すべてのGateは実行したcommit SHAとlogをPLANへ記録する。

## Idempotence and Recovery

作業再開時はルートPLANとHEAD／差分から再開点を決める。RED済みでもコードが変わったら前提を確認する。既存のユーザー変更を含むファイルはhunk単位でstageする。検証用の設定・DB・report・Git fixtureは一時directoryに作り、ユーザーの実設定や既存DBをテストに使わない。

設定保存や出力失敗時に削除してよいのは操作自身が作成した一時ファイルのみ。Codex外部編集を検知したらその編集を保持し、再試行可能な競合errorを返す。DB境界拒否で外部DB／副ファイルを修復・移動・削除しない。索引更新失敗を正常完了として保存しない。

同じ不具合に対する修正が3回失敗した場合は、疑っている前提・観測・未確定点を明示して当該タスクを止める。無関係なデータを削除する回復策を使わない。テストと契約が監査を否定した場合はDecision Logで理由を残し、50件のID追跡自体は消さない。

統合は全Gateと独立レビュー通過後に限る。masterが進んだ場合は再統合・必要な再検証を行い、force resetしない。`master`へfast-forwardマージし、修正HEADへの到達を確認してから、今回の作業branchだけを削除する。一般のマージ済みbranchを一括削除しない。完了記録のみを最後の文書commitとして残し、push／稼働binary置換はしない。

## Outcomes & Retrospective

現時点の対象修正コミットは28/48。各件の状態・実測証拠はProgressの台帳に記載。最終検証・独立レビュー・統合は未完了。実装完了時に50件の主テスト／関連受入れ／commit SHA、旧索引の更新結果、全Gate、review修正、未解決項目の有無、FF統合先と既存変更保持の証拠を記入する。

本書作成時の検証は文書のID完全性・参照先・`git diff --check`のみ。計画作成のためにビルド・通常テスト・race・vet・restore・生成を実行しない。

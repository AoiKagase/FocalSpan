# 追加19件の修正計画

## Purpose / Big Picture
N01〜N19を修正し、範囲外参照・データ消失・古い関係・抽出誤り・評価指標の誤判定を防ぐ。ユーザー承認済み。executing-plans/TDDで①〜⑤を順に実装し、問題単位でコミットする。

## Progress
- 2026-10-17 12:09 UTC: masterを6bd4259からa1f6930へfast-forward統合し、branch到達と既存7ファイルの内容保持を再確認。codex/additional-19-fixesを削除済み。検証済み9d1869cからcmd/internal/go.mod/go.sum/testdataの変更はなく、最後の変更は完了記録のみ。push/稼働binary置換なし。
- 2026-10-17 12:08 UTC: 最終9d1869cで全通常・全race（UCRT64 GCC）を直列に完了、いずれもexit 0。go vet、native、CGO_ENABLED=0のWindows amd64/Linux amd64/Darwin arm64で3 commandsすべてbuild成功（C:/Temp/fs19-build-gates.log）。19 ID/正式テスト照合・archive byte同一性・既存7ファイルSHA256・git diff --checkもPASS。master=6bd4259を確認して承認済みFF統合へ進む。
- 2026-10-17 12:07 UTC: 最終9d1869cで全通常テストPASS（C:/Temp/fs19-all-normal-final.log、exit 0）。その完了後に全raceを開始。既存user変更7ファイルのSHA256は開始時と全一致。独立reviewのP2解消確認済み。
- 2026-10-17 12:05 UTC: 独立reviewの唯一のP2（Python継続文字列/escaped triple delimiter）を正式RED→GREENで修正し9d1869cへcommit。修正差分の独立確認は追加P1/P2なし。最終sourceで全通常テストを再実行中。19 IDと全正式テスト参照、前PLAN archiveのbyte同一性を確認済み。
- 2026-10-17 12:00 UTC: 全19件の実装・個別RED/GREEN・受け入れ補足完了。別process設定lock/5秒/cancel、Install/Uninstall競合保持、BOM/CRLF UnicodeのEvidence v1/v2原文/実測wire確認済み。独立reviewでPython文字列境界のP2が1件あり、正式REDを確認して修正・検証中。全通常テスト351958bは成功したが、最終修正後に再実行する。
- [x] 専用branch codex/additional-19-fixesをmaster@6bd4259から作成し、前PLANをそのままarchive。
- [x] ① N01〜N03: ファイル境界・ID・snapshot所有範囲。
- [x] ② N04・N05: 設定競合・管理マーカー。
- [x] ③ N06〜N10: Git形式固定・manifest再抽出・JSONC。
- [x] ④ N11〜N18: Python/Go抽出。
- [x] ⑤ N19: 正解順位とreport v2。
- [x] 全通常→race、vet、native/cross-build、独立レビュー、FF統合・作業branch削除。

## Context and Orientation
監査証拠: C:/Temp/focalspan-additional-audit-20261017.json、fs50-overlay.json、fs50-all-repro.jsonl。20本の失敗を正式化し同一原因を19 IDで管理。一時overlayを実行依存にしない。

| ID | 対象 | 受け入れ条件 |
|---|---|---|
| N01 | repository/root.go | 実体パスのsource包含判定、外側junction拒否・内側許可 |
| N02 | benchmark/model.go,runner.go; benchcli/run.go | ID共通検証、全入口のworkspace包含 |
| N03 | benchmark/snapshot.go | 既存destination拒否、所有する新規保存先だけ削除 |
| N04 | integration/codex/project_config.go; indexlock | 設定毎5秒/cancel lock、外部変更は競合error |
| N05 | integration/codex/managed_block.go | TOML文字列外の独立コメント行だけmarker |
| N06 | gitx; benchmark/diff.go | 全解析diffの色無効化 |
| N07 | gitx; benchmark/diff.go | 全解析diffでa/・b/prefix固定 |
| N08 | indexer/indexer.go | manifest変更時に全ソース再抽出、旧resolved関係を再生成 |
| N09 | projectmeta/discover.go | compilerOptions.baseUrl/paths、標準優先・旧形式互換 |
| N10 | projectmeta | ts/jsconfigのみJSONC、package/composerは厳格JSON |
| N11 | extract/python | lexer位置でstring/comment内の偽宣言/呼出しを除外 |
| N12 | extract/python | 有効なindent scopeから親決定 |
| N13 | extract/python | クラス内test_のtest分類維持 |
| N14 | extract/python | Unicode識別子をruneで扱う |
| N15 | extract/python | 複数import/aliasの分離 |
| N16 | extract/goast | generic/括弧付き呼出しの展開 |
| N17 | extract/goast | local変数/引数の遮蔽を大域関数へ解決しない |
| N18 | extract/goast | import literalをunquote |
| N19 | benchmark/metrics.go,runner.go,compare.go | symbols優先の正解順位、report v2、混在比較拒否 |

## Plan of Work
1. 各IDで正式テストを先に追加してFAILを観測し、最小修正→関連GREEN→対象パスだけstageしてcommit。
2. N01: root/candidate双方のphysicalPathを使い解決失敗の字句fallback廃止。N02: 既存許可文字を保ちdot/dotdot、separator、volume、Windows予約名、末尾dot/spaceを拒否し全入口と保存先を検証。N03: os.Mkdirで所有を確保し既存destinationは空でも拒否。
3. N04: 設定毎の固定lockを観測前からrenameまで保持し、保存直前の存在/内容変更は再試行可能error。Install/Uninstall両方、tempのみcleanup。N05: TOMLのbasic/literal/multiline string/escapeを追跡してmarker判定。
4. N06/N07: 全解析diffに--no-color --src-prefix=a/ --dst-prefix=b/固定。N08: 抽出前にfacts比較、変更時全ソース再抽出。N09/N10: 安全なJSONC変換、compilerOptions優先、baseUrlをaliasへ反映。extends追加なし。
5. N11〜N15: lexer tokenをcode view/scopeへ渡し、byte/newline保持、string/comment偽code除外、class test/Unicode/multiple importを修正。f-string literalから関係を生成せず未解析式の確定関係を推測しない。N16〜N18: AST unwrap、遮蔽除外、strconv.Unquote。抽出器version更新。
6. N19: required_symbols(path/name/指定kind/role)優先、無ければrequired_paths、両方無ければ順位0。v2出力、v1/v1比較維持、v1/v2拒否。

## Interfaces and Dependencies
MCP公開field・Evidence v1/v2・DB schema維持。設定lock/競合errorは内部API追加。benchmark reportは同じfieldでschema=focalspan.benchmark-report.v2。新規依存・network・解析対象code実行なし。

### Regression ledger
| ID | 正式テスト | commit |
|---|---|---|
| N01 | TestAuditScannerMustNotReadOutsideJunction / TestSourceContainmentAllowsInsideJunction | 53529c5 |
| N02 | TestWorkspaceIDRejectsUnsafeComponents / TestSuiteRejectsTraversalIDs / TestSnapshotDestinationRejectsOutsideParentJunction | 32276c1,526b561 |
| N03 | TestSnapshotPreservesExistingDestination / TestSnapshotCancellationRemovesOwnedDestination | ce86554,b9dc1ca |
| N04 | TestAuditConcurrentConfigEdit / TestProjectWritesSerialize / TestConfigConflictPreservesExternalEditOnBothOperations / TestConfigurationLockAcrossProcesses / TestInstallCancellationBeforeDirectoryCreation | 1b5250b,351958b,962b7ed |
| N05 | TestAuditMarkersInsideTOMLString / TestMarkersOnlyOutsideTOMLStrings | 7a2ac02,351958b |
| N06 | TestDiffDisablesConfiguredColors | 1f98bbe |
| N07 | TestDiffPinsPathPrefixes | b0a1303 |
| N08 | TestManifestOnlyUpdateRebuildsResolvedRelations | 18b0197 |
| N09 | TestTSConfigReadsCompilerOptionsAliases / TestTSConfigKeepsLegacyAliases | bf345ee |
| N10 | TestTSConfigAcceptsJSONCWithoutChangingStrings / TestJSONCPreservesStringContents / TestPackageJSONRemainsStrict | 334362b |
| N11 | TestAuditPythonStringIsNotCode / TestAuditPythonCallInsideCommentOrString / TestPythonFixesPreserveEvidenceSourceAndWireBudget / TestPythonMaskRespectsMultilineStringBoundaries | 22b4794,351958b,9d1869c |
| N12 | TestAuditPythonClosedParentScope | 5efee01 |
| N13 | TestAuditPythonClassTestClassification | aa50ba8 |
| N14 | TestAuditPythonUnicodeName / TestPythonUnicodeCallRelation | d26eb7f |
| N15 | TestAuditPythonMultipleImports / TestPythonImportTargetsPreserveOriginalNames | 5f400e1 |
| N16 | TestAuditGoGenericCall | 44eb821 |
| N17 | TestAuditGoShadowedFunctionCall | 0e2fbe0 |
| N18 | TestAuditGoRawStringImport | 4908cb3 |
| N19 | TestTargetRankUsesRequiredLabels / TestReportComparisonVersions / TestRunnerSequencesCasesProfilesBudgetsAndRepeats | c84784d |

## Validation and Acceptance
各RED/GREENをProgressへ追記。source原文/byte/line、Evidence invariant/実測wire予算、index差分/全再構築の関係集合、設定別goroutine/別process・5秒・cancel・CRLF/mode/temp保持、ID/destinationの既存データ保持を検査。
最後にgo test ./...→go test -race ./...を直列実行（Windows raceはC:/msys64/ucrt64/bin/gcc.exe）、go vet ./...、native build、CGO_ENABLED=0のwindows/amd64・linux/amd64・darwin/arm64でfocalspan/focalspan-bench/focalspan-evalをbuild。独立reviewとgit diff --checkを通す。

## Surprises & Discoveries
④⑤: 全extract/app/indexer/evidence/mcpserver GREEN（C:/Temp/fs19-extraction-green.log）、全benchmark/benchcli GREEN（fs19-n19-green.log）。全19件の実装完了。N11=22b4794、N12=5efee01、N13=aa50ba8、N14=d26eb7f、N15=5f400e1、N16=44eb821、N17=0e2fbe0。N18/N19を個別commit後、追加の受け入れ・全検証・review・統合を行う。
③: N06=1f98bbe、N07=b0a1303、N08=18b0197、N09=bf345ee、N10は正式RED→GREEN。N08は監査の直接LinkerテストをIndexerの差分/全再構築比較へ正式化した（失われた参照名は原文からだけ復元）。N09で既存facts期待値2件をbaseUrl込みへ更新。C:/Temp/fs19-n06〜n10-{red,green}.log参照。
②: N04の外部変更/並行writerとN05のstring内markerは正式RED→GREEN。全indexlock/codex成功（C:/Temp/fs19-settings-green.log）。N04=1b5250b。設定lockの別process・Uninstall競合は351958bで補足完了（fs19-settings-acceptance.log / fs19-settings-acceptance-followup.log）。
①: N01/N02/N03の正式REDを確認後、repository/store/indexer、benchmark/benchcliはGREEN。ログC:/Temp/fs19-n01-{red,green}.log、fs19-n02-{red,green}.log、fs19-n03-{red,green}.log。N01=53529c5、N02=32276c1。既存dirty paths保持。

## Decision Log
- 2026-10-17: N11のreview指摘は、従来lexerの境界誤りがcode maskで抽出結果へ波及するものと正式テストで確認。全stringの継続位置・closing lineの残りtoken・escaped triple delimiterを修正した。Evidence本文の受け入れはself展開を明示し、最終改行を含まない半開byte範囲に対して原文一致を検査する。
- 2026-10-17: ユーザー選択: 設定競合は中止・再試行案内、正解判定symbol優先。
- 2026-10-17: 承認済み手順に従い既存checkoutの専用branchを使用。dirty paths保持、worktree追加なし。Windowsではshell用skill helperの代わりに本PLANと専用ledgerで追跡。
- 2026-10-17: 同時link差替え/非協調editorへの完全なOS-level CASは保証範囲外。push・稼働binary置換・supervisor EOF修正なし。

## Idempotence and Recovery
前PLAN archive変更禁止。再開時にbranch/HEAD・dirty paths・commit・最終テスト結果を照合し、完了済み修正を再実装しない。reset/clean/stashや無関係branch/worktree/lock削除なし。最終検証後masterへFFし今回のbranchだけ削除。

## Outcomes & Retrospective
全19件の実装と個別RED/GREENを完了。独立reviewのP2を修正し、追補reviewで未解決のP1/P2なし。最終sourceの通常/race/vet/native・3環境cross-buildはすべて成功。masterへa1f6930までfast-forward統合し、作業branchを削除した。既存user変更7ファイルの内容は開始時と一致。MCP/Evidence/DB形式は維持し、benchmark reportだけ評価定義を示すv2へ更新した。build成功は対象OS上の実行確認ではなく、push・稼働binary置換も行っていない。

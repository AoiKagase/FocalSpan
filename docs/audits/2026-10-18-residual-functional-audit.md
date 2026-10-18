# 残存する機能・整合性問題の監査（2026-10-18）

11件を確認した。すべて一時overlayテストで期待値との差を再現した。セキュリティ項目は対象外とし、前回除外されたF02/F03も再監査・再現していない。

基準は `master@66d8ea87916cdda57d0d3c0ecd8fd26dfc91e07a` の作業ツリー。既存の未コミット変更を含む。製品コード・設定・稼働バイナリは変更していない。本書は監査記録であり、完了済みのルートPLAN.mdを新しい修正計画へ切り替えるものではない。

## 結果一覧

P2は通常優先の機能不具合、P3は結果本文に直接影響しない統計不整合を表す。IDは今回の監査内で固定し、前回のF01〜F50とは区別する。

| ID | 優先度 | 問題 | 主な影響 | 再現範囲 |
|---|---|---|---|---|
| R01 | P2 | JS/TSの引数による遮蔽を無視して同名の大域関数へ確定接続する | 呼出し元・関連テストの誤提示 | 抽出器 |
| R02 | P2 | 動的importの式中にある部分文字列を固定のモジュール名と扱う | 存在しない依存候補を生成 | 抽出器 |
| R03 | P2 | exportする値の文字列を再export先と扱う | 値リテラルから無関係な依存候補を生成 | 抽出器 |
| R04 | P2 | 別プロジェクトのtsconfigのaliasを適用する | monorepo内で誤ったimport先へ接続 | linker単体・実索引 |
| R05 | P2 | 一意な相対importでも参照先の宣言数によって解決できなくなる | 通常のファイル依存を取りこぼす | 実索引・対照あり |
| R06 | P2 | Rubyの通常の複数行文字列内に偽の関数を作る | 実在しない宣言が検索対象になる | 抽出器・対照あり |
| R07 | P2 | Rubyのendless methodが後続メソッドを取り込む | 範囲・親子関係・修飾名が壊れる | 抽出器・対照あり |
| R08 | P2 | Pythonのセミコロン区切りimportを正しく分割しない | import先を誤記し、後続importを欠落させる | 抽出器・対照あり |
| R09 | P2 | Impactが説明にある依存元・関連テストを返さない | 影響確認で変更ファイル以外を見落とす | Git fixture・app API |
| R10 | P2 | benchmarkの索引済み判定が500候補で打ち切られる | 索引済みの正解を未索引と誤診断 | Store実データ・評価engine |
| R11 | P3 | 競合関係の再抽出をソース変更件数へ加算する | 増分更新のchanged/unchanged統計が不正確 | 実索引 |

## 発生条件とコード根拠

### R01 — JS/TSの遮蔽された呼出しを誤接続する

`function target() {}` と `function caller(target: () => void) { target(); }` を同じファイルに置く。後者の `target()` は引数への呼出しだが、大域関数への `calls` 関係が `Confidence=0.95`、`Source=jsts-call` で生成される。

`internal/extract/jsts/extractor.go:247-261` の `resolveCall` は修飾名・親クラス・名前の一意性を確認するだけで、引数やローカル束縛を確認していない。根拠テストは `TestResidualJSShadow`。引数のない通常の大域関数呼出しは `TestResidualJSControls` で成功した。前回F40のPython向け遮蔽修正とは別実装である。

### R02 — 動的importの部分文字列をモジュール名と扱う

`const modulePromise = import(name + './dep');` から、`dep`、`dep.ts`、`dep/index.ts` 等の固定import候補が生成される。式の値は `name` に依存するため、`./dep` だけをimport先と確定できない。

`internal/extract/jsts/parser.go:93-98` が引数式の構造を確認せず `firstString` を取り出し、`internal/extract/jsts/extractor.go:175-177` がその文字列を依存候補へ展開する。根拠テストは `TestResidualJSDynamic`。固定文字列による通常のimportは対照テストで保持されることを確認した。

### R03 — exportする値を外部モジュールと扱う

`export const label = './dep';` は文字列値のexportだが、`dep`、`dep.ts` 等への `exports` 関係が `Source=jsts:export` で生成される。

`internal/extract/jsts/parser.go:133-151` はexport文内のすべての文字列を `stringLiterals` で列挙する。`from` の有無はSource文字列の変更にしか使っておらず、値とモジュール指定を区別していない。根拠テストは `TestResidualJSExportValueString`。前回F34のimport終端修正とは異なるexport側の問題である。

### R04 — aliasの適用元プロジェクトを確認しない

`packages/a/tsconfig.json` だけに `@dep -> dep.ts` を定義し、`packages/a/dep.ts` と `packages/b/main.ts` を置く。B側の `import '@dep'` が、A側の設定を使って `packages/a/dep.ts` へ解決される。Bへの設定継承は存在しない。

`internal/linker/linker.go:139-145` は呼出し元の所属設定を確認せず全factsを候補化する。`factPathCandidates`（同347行以降）でも呼出し元のパスを受け取らない。設定ファイル位置をターゲットパスへ反映する処理だけでは、適用範囲を限定できない。

根拠は `TestResidualAliasScope` と `TestResidualAliasScopeIntegration`。後者は実際のScanner・metadata discovery・Indexer・Storeを通して誤った確定関係を確認した。参照先を関数付きファイルにするとR05の曖昧性がこの問題を隠すため、aliasの統合再現には宣言のない有効なTSファイルを使用した。

### R05 — モジュールとその内部宣言を競合候補と扱う

`main.ts` の `import './dep.ts';` と、`dep.ts` の `export function work() {}` を索引化する。ファイルの指定は一意だが、import関係は解決されない。同じ `dep.ts` をコメントだけのファイルに変更すると解決される。

`internal/linker/linker.go:282-299` が、パスに一致するモジュール所有者と内部関数をまとめて返す。`LinkWithScope` の同76-81行は候補数が1以外なら解決を見送る。ファイルimportでは一意なモジュール所有者を選ぶ処理がない。

根拠テストは `TestResidualImportModuleAmbiguity`。phase 0（関数あり）が失敗し、phase 1（宣言なし）の対照は成功した。前回F17のprojectionによる候補欠落、F18の拡張子省略によるパス不一致とは原因が異なる。今回は拡張子を明示している。

### R06 — Rubyの通常文字列内に宣言を生成する

通常のダブルクォート文字列を複数行にし、その本文へ `def phantom` と `end` を置く。文字列の外には `def real` を置く。`phantom` がConfidence 0.95のfunctionとして生成される。

`internal/extract/ruby/extractor.go:76-86` はHeredocとBlockCommentだけを解析用本文から除く。通常のString tokenを除かないまま、行ベースの `parseRuby` へ渡している。根拠テストは `TestResidualRubyMultilineString`。前回F29で修正されたheredocとは別の通常文字列である。

### R07 — Rubyのendless methodの終端を認識しない

`def first = 1` の次に通常の `def second ... end` を置く。`first` が後続の4行目まで広がり、`second` は `first::second` として子に分類される。本来 `first` は1行で完結し、`second` は独立したメソッドである。

`internal/extract/ruby/extractor.go:186-204` は通常のdefとして宣言を作り、同240-259行のdepth/stack処理でendを待つ。根拠テストは `TestResidualRubyEndless`。両方を通常形式で書いた `TestResidualRubyControls` は成功した。

### R08 — Pythonの同一行の複数importを壊す

`import os; import sys` から `os;` というimport先だけが生成され、`os` と `sys` は生成されない。改行で分けた同じ2つのimportは正しく抽出される。

`internal/extract/python/imports.go:21-45` は括弧と改行継続を処理した後、カンマだけで分割し、最初のfieldをimport先に採用する。セミコロンによるsimple statementの境界を扱わない。根拠テストは `TestResidualPythonSemicolonImports` と正常対照 `TestResidualPythonControls`。

### R09 — Impactの返却範囲が公開説明より狭い

Git fixtureに `Target`、`Caller -> Target`、`TestTarget -> Target` を別ファイルで置き、Targetの本文だけを変更する。4000 token予算で `ImpactEvidence` を呼ぶと、返却パスは `target.go` だけになる。呼出し元・テストの関係がStoreに存在し、個別の関係取得で取得できることは事前に確認した。

`internal/mcpserver/server.go:66` はcode_impactを「changed spans, dependents, and related tests」と説明している。一方、`internal/app/evidence.go:232-300` の `impactCandidates` は変更行と重なる同じファイルの候補だけを収集し、関係探索を行わない。

根拠テストは `TestResidualImpactRelations`。これは説明と実装の不一致として採用した。syntax-onlyという制約自体を欠陥とはしていない。修正時には既存の構文関係まで返す契約か、説明を変更する契約かを明確にする必要がある。今回はMCP越しの再現は行っていない。

### R10 — 評価の索引済み判定に候補上限が混入する

異なる502ファイルへ同名の `Target` をStoreに保存し、末尾の `0501.go:Target` の索引済みidentityを要求する。返却は空になる。先頭の `0000.go:Target` は同じ経路で取得できる。

`internal/benchmark/engine.go:92-112` の `AttributionIdentities` はパス条件を後から確認する前に、全体の `SearchExactSymbols(..., 500)` を適用する。正解が上限外なら `AttributeLabels` の `label_not_indexed` 判定へ進み、抽出・索引の問題と誤診断する。これは製品検索側の範囲付き候補取得を修正しても残る別の呼出し元である。

根拠テストは `TestResidualAttributionLimit`。実Storeを使うが、502ファイルの抽出器・完全なbenchmark CLI経路は実行していない。

### R11 — 再抽出とソース変更を同じ統計へ加算する

2ファイル（Target定義とCaller）を全索引後、別ファイルに同名Targetを追加する。既存2ファイルのbytesは変更しない。増分更新は `added=1, changed=1, unchanged=1` を返すが、内容変更の件数なら `added=1, changed=0, unchanged=2` である。

`internal/indexer/indexer.go:213-218` は、新しく競合する関係の解決をやり直すための再抽出を `FilesChanged++` と `FilesUnchanged--` に加算する。既存の入辺による再抽出経路（同146-177行）は同じ加算を行わず、統計の意味が経路によって異なる。

根拠テストは `TestResidualReextractCounts`。完了済み `2026-10-17-remaining-24-fixes.md` の方針「FilesChanged等は内容変更の件数を保ち、依存再抽出で二重計上しない」とも不一致である。今回の検証では検索本文の誤りは示しておらず、統計のみのP3とした。

## 確認範囲と未確認範囲

| 範囲 | 今回の確認 | 限界・未確認 |
|---|---|---|
| 前回の監査・修正台帳 | F01〜F50と完了PLANを照合し、修正内容との重複を判定 | 前回48件の全回帰テストは再実行していない |
| app / indexer / Store / linker | 読取り・Impact・更新・関係候補の関連コードを確認。実索引によるR04/R05/R09/R11を再現 | 全SQL、全更新順序、別プロセス競合の再監査ではない |
| JS/TS・Python・Ruby | 抽出の該当分岐を精読し、正常対照と不具合を実行 | 全言語構文の網羅ではない。他言語のコードは部分確認のみ |
| benchmark / Evidence / Git / config / repository / MCP | 関連する判定、候補上限、公開説明、整合性経路を静的確認。R10のStore/engine再現 | Evidence wire全組合せ、MCP通信、実Codex再登録、実CIは未実行 |
| OS・運用 | Windowsで上記の一時Goテストを実行 | 今回はLinux/WSL/macOS、race、全通常テスト、vet、ビルド・再配置を実行していない |

セキュリティ評価、依存関係の脆弱性評価、原因未確定のsupervisor EOFは対象外。未確認範囲に問題がないとは主張しない。今回の件数は確認できた問題の数であり、目標件数による打切りや残存ゼロの宣言ではない。

## 再現証拠と再実行

再現スクリプト: `C:/Temp/fs-residual-audit-20261018.py`

overlay・一時テスト・最終ログ: `C:/Temp/fs-residual-audit-20261018/`

最終ログ: `C:/Temp/fs-residual-audit-20261018/reproduction.log`

実行コマンド:

```powershell
rtk proxy python C:/Temp/fs-residual-audit-20261018.py
```

スクリプトは `GOPROXY=off`、`GOSUMDB=off`、`CGO_ENABLED=0` で、6パッケージの `go test -overlay=... -run ^TestResidual -count=1 -v` を実行する。製品ソースへテストを追加せず、fixtureは `t.TempDir()` 配下に作成する。Windowsのサンドボックス内で子プロセスの実行が制限されたため、最終の再現は通常権限で実行した。

最終結果は、11問題に対応する12テストが期待値との差でFAIL、3つの独立した正常対照がPASS。R04は単体と統合の2テストを持つため、失敗テストの数を問題件数に数えていない。R05/R09/R10には各テスト内部にも対照確認がある。これらのFAILは不具合の再現結果であり、既存の通常テストが新たに失敗したという意味ではない。

既存の未コミットMCPファイルのSHA256は前回配置時と一致した:

```text
internal/mcpserver/server.go
e9df4a91a137a2467e93455bcc20735bd4d4560d4035de4d7a361346615c9c9f
internal/mcpserver/context_v2_test.go
601807bdc9f188ae5d33fd5b186cc85b9782c8232b5ce3c9e31491e6873d9a62
```

新たな成果物は本監査文書と一時再現証拠のみ。修正・コミット・マージ・push・再配置は実施していない。

# 残存11件の実装・検証記録

R01〜R11を実装し、正式な回帰テストのRED→GREENを確認した。
サイバーセキュリティ項目・supervisor EOFは除外した。

コード最終コミット: `959fb8479544062d149dfa0d7ed1796a5a805d85`。
開始時master: `66d8ea87916cdda57d0d3c0ecd8fd26dfc91e07a`。
問題別コミット・実行時の判断はルートPLANのExecution evidenceを参照。

## 検証結果

| 検証 | 結果 | 生ログ（C:/Temp/fs11-evidence/） |
| --- | --- | --- |
| 全通常テスト | PASS・47 packages | final-normal-complete.log |
| 全race | PASS・通常終了後に実行 | final-race.log |
| 元の未コミットMCP変更を重ねた全通常／全race | PASS・直列実行 | overlay-normal.log, overlay-race.log |
| go vet ./... | PASS | final-vet.log |
| 3 CLIネイティブビルド | PASS | build-*-native-amd64.exe.log |
| CGO無効クロスビルド | Windows amd64/arm64、Linux amd64、Darwin arm64の3 CLIすべてPASS | build-*.log, build-sha256.json |
| WSL-Ubuntu実受入れ | ext4 /tmp上でapp R04/R05/R09、projectmeta R04、linker R05/case、store・benchmark R10がPASS | linux-run-*.log |
| benchmark v2 smoke | validate/run/既存baseline比較PASS | benchmark-*.log, benchmark-candidate.json |
| 独立レビュー | 具体的指摘を全件修正・未解決ブロッカーなし | 2026-10-18-residual11-independent-review.md |

通常・raceは`-p 2 -count=1 -timeout 15m ./...`、通常はCGO無効、raceは既存UCRT GCCでCGO有効。
全パッケージの集合一致、FAIL/DATA RACEなしを照合した。GOPROXY/GOSUMDBはoff、新規依存・取得はない。
UbuntuにはGoが未導入のため、Windowsの既存GoでLinuxテストバイナリをビルドし、Ubuntuのext4へ展開したソースfixtureを使って実行した。Linuxの対象テスト実行とクロスビルド成功を区別し、Darwin/Windows arm64の実機実行は主張しない。

R09の低予算・既知handle・複数変更・循環・削除・自動更新ON/OFF・現在本文検証、原文bytes/行位置/packet不変条件/実測wire予算/MCP原文一回を含む正式テストは全通常・raceに含まれる。
R11は複数依存経路と連続した競合追加も確認し、返却・保存済みrun・全再構築の関係集合を照合した。
抽出version v19への更新は、既存v18索引の正規再抽出と次回の非再抽出を確認した。DB schemaの変更はない。

## 失敗の扱いとレビュー

初回全件では旧v18を固定したテストとStore並行初期化deadlineが失敗した。旧version固定を現行定数比較へ修正し、Storeの製品コードは変えずに対象・全件で再確認した。deadline失敗の原因は確定していない。Goキャッシュ付き起動の停止・短い診断timeout・引用誤りによる未実行は成功結果へ数えていない。
独立レビューの3指摘と修正中に追加確認したtyped callbackの2境界は、再現FAILを保存してから修正した。最終全件はそれらすべてを含むコードに対して実行した。
R01の初回2指摘はレビュー担当へ限定修正を委譲し親が差分確認したため、その部分のGREENは修正本人の検証を含む。R07と親の追加修正はレビュー担当が読取り確認した。全差分レビューは一度で、無関係な追加監査は実施していない。

原文・公開MCP/Evidence形式・公開DB schemaを維持した。ユーザーの未コミットファイル11件は開始時ハッシュで保持確認する。push・稼働バイナリ置換は実施しない。

## 統合

全ゲート通過。masterへfast-forward統合・統合後通常テスト・今回ブランチ削除を実行し、実測結果を追記する。

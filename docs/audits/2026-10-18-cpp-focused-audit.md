# C++重点監査 — 2026-10-18

**再現確認済み20件（P1: 8件、P2: 12件）。** 対象は`master@ca46099c2b190ba03539dc092794d9473e21c5bd`と既存の未コミット変更を含む作業ツリー。サイバーセキュリティ項目、supervisor EOFの調査は除外した。修正・コミット・稼働バイナリ置換・実索引更新は行っていない。

P1は通常のC++入力で索引・原文範囲・確定関係を大きく誤るため優先するもの、P2は特定構文や境界の正確性・精度の問題とした。件数上限を設けた監査ではない。初期疑いを正常対照で取り下げたものは問題数へ含めていない。

## 確認済み・未確認の範囲

| 範囲 | 今回の確認 | 限界 |
| --- | --- | --- |
| C++ lexer / token | 数字、引用・raw文字列、継続コメント、includeコメント、既存条件分岐、UTF-8/BOM/CRLF | 全ての前処理式・全てのliteral形式の組合せを網羅したものではない |
| C++ parser / extractor | 関数・constructor・operator・typedef・class template・namespace・本文範囲・呼出し/参照・宣言定義対応 | C++全規格の完全な構文適合性、型検査、overload解決は未検証 |
| 索引 / linker | 実DBの別ファイル・include・module・誤った確定関係、無関係なnamespace追加後の再リンク | 大規模C++専用リポジトリでの性能・全ての増分更新パターンは未確認 |
| Query / Expand / Impact / Evidence | custom-return関数の検索欠落、誤callerのEvidence、module展開、include展開・ヘッダ変更Impact | C++ケースを実MCPプロトコル越しには再実行していない。全予算・全クライアントの組合せは未確認 |
| 実行と環境 | Goの既存C++/linkerテスト、隔離DB/Git fixture、Go overlayの再現 | Clang/GCC/MSVCでC++断片をビルド・実行していない。ネットワーク・新規依存・設定変更なし |

`docs/design.md:246–251`の仕様どおり、macro展開・overload解決・仮想/動的dispatchの意味解析をしないこと自体は問題に数えていない。CP08–CP10は、それらを未解決にせず無関係な確定関係へしてしまう点を指摘する。CP19は明示的に未解決候補としての精度劣化であり、誤った確定とは扱わない。

## 実測検証

既存の`go test -p 2 -count=1 -timeout 10m ./internal/extract/cpp ./internal/linker`は両packageでPASS。新規overlayは34の独立サブテストで27の不具合再現FAILと7の正常対照PASS。同じ問題の複数構文、抽出器とpublic APIの重複検証を20件へ集約しており、27件の独立問題とは数えていない。

正常対照には、入れ子型の所有者、operator=、通常のint関数、u8 raw文字列、日本語識別子/CRLF、include Expand、ヘッダ変更Impactがある。CP19内の追加前の正しいnamespaceリンクと増分/全再構築の関係集合一致も成功した。内部include関係が未解決であることだけを根拠に挙げかけた問題は、公開Expand/Impactが成功したため取り下げた。

原文chunkの`Content == source[StartByte:EndByte]`は再現入力で検査した。ただし、bytesが正しくても範囲自体を誤って短くするCP02/CP04は見逃せるため、関数名と末尾の本文が範囲内にあることも別途assertしている。

生ログ・入力SHA256・実際のコマンド・HEADは`C:/Temp/focalspan-cpp-audit-20261018/`に保存した。`baseline.log`、`extraction-repro.log`、`integration-repro.log`と対応JSONを参照。隔離fixturesは毎回`t.TempDir()`を使い、稼働DBを使用しない。

## 問題一覧

### 字句・前処理・文字コード

| ID / 優先度 | 再現入力と実測結果 | 根拠 |
| --- | --- | --- |
| CP01 / P1 | **数字区切り文字で後続の宣言が消える**<br>入力: `constexpr int count=1'000; void real() {}`<br>実測: アポストロフィを文字リテラル開始として扱い、realがSymbols/Chunksに存在しない<br>期待: realを関数として抽出する | [lexer.go:130](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:130) / [lexer.go:103](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:103)<br>test: `C01_digit_separator`<br>層: extractor only |
| CP12 / P2 | **行継続された//コメントから偽の関数を抽出する**<br>入力: `// note \ followed by newline, then void phantom() {}`<br>実測: 改行だけでLineCommentを終了し、コメント内のphantomを実関数として生成する<br>期待: 継続行をコメントとして扱い、phantomを生成しない | [lexer.go:67](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:67)<br>test: `C13_spliced_line_comment`<br>層: extractor only |
| CP13 / P2 | **コメントを伴う有効な#includeを落とす**<br>入力: `/* note */ #include "dep.hpp" / #include /* note */ "dep.hpp"`<br>実測: 前者はlexerのlineStart判定、後者はinclude正規表現で脱落し、両方importsが生成されない<br>期待: コメントを空白相当としてimportsを生成する | [lexer.go:29](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:29) / [lexer.go:58](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:58) / [parser.go:428](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:428)<br>test: `C14_commented_include_prefix`<br>層: extractor only |
| CP20 / P2 | **UTF-8 BOM付きファイルの先頭関数を落とす**<br>入力: `UTF-8 BOM + void first() {} CRLF void second() {}`<br>実測: 先頭firstが未抽出、後続secondは存在する。BOMなしの日本語識別子とCRLFは正常対照として成功<br>期待: BOMを識別子へ取り込まずfirstとsecondを抽出する | [lexer.go:12](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:12) / [lexer.go:118](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:118) / [lexer.go:203](H:/sourcecode/FocalSpan/internal/extract/cpp/lexer.go:203)<br>test: `C22_utf8_bom`<br>層: extractor only |

### 宣言と型の認識

| ID / 優先度 | 再現入力と実測結果 | 根拠 |
| --- | --- | --- |
| CP02 / P2 | **コンストラクタ名が原文範囲から脱落する**<br>入力: `namespace n { struct Item { Item() {} }; }`<br>実測: constructor自体は存在するがSignatureは() {、chunkは() {}になる。名前付きnamespaceで検査しCP15との混同を除いた<br>期待: 署名とchunkをItem()から開始する | [parser.go:378](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:378) / [parser.go:242](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:242)<br>test: `C02_first_constructor`<br>層: extractor only |
| CP03 / P1 | **ユーザー定義型を返す普通の関数を抽出しない**<br>入力: `struct Item {}; Item make() { return {}; }`<br>実測: makeが未抽出。QueryEvidence(make)も該当原文を返さない。int known_function()の同じAPI対照は成功<br>期待: makeのsymbol/source chunkを生成し検索で返す | [parser.go:384](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:384)<br>test: `C03_custom_return_type`<br>層: extractor + QueryEvidence |
| CP07 / P2 | **複数の演算子宣言が欠落・誤命名される**<br>入力: `operator[] / operator+ / operator bool / operator()`<br>実測: operator[]と変換演算子が欠落、operator+はName=+のmethod、operator()はName=operatorになる。operator=の正常対照は成功<br>期待: それぞれを演算子の名前・kindで抽出する | [parser.go:375](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:375) / [parser.go:393](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:393)<br>test: `C08_operators`<br>層: extractor only |
| CP14 / P1 | **templateのclass型引数を本物のclass宣言と誤認する**<br>入力: `namespace n { template<class T> struct Box { void run() {} }; }`<br>実測: 型引数Tをclassとして抽出してBoxのbodyを取り込み、Boxとrunの本来の所有者が失われる。templateの実体化を要求しているのではなく字句上の宣言判定の問題<br>期待: n::Boxとn::Box::runを抽出する | [parser.go:116](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:116) / [parser.go:173](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:173)<br>test: `C15_template_type_parameter`<br>層: extractor only |
| CP18 / P2 | **既存型からのtypedefでalias名を誤る**<br>入力: `struct Existing {}; typedef Existing Alias;`<br>実測: aliasをExistingという名前で作り、Aliasのsymbolがない。最初のIdentifierを別名として採用している<br>期待: aliasの名前はAlias | [parser.go:200](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:200) / [parser.go:216](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:216)<br>test: `C20_typedef_custom_type`<br>層: extractor only |

### 原文範囲とnamespace

| ID / 優先度 | 再現入力と実測結果 | 根拠 |
| --- | --- | --- |
| CP04 / P1 | **関数定義の開始・終端を取り違え本文が欠ける**<br>入力: `explicit Item(int x): value{x} { consume(); } / requires requires(T y) { y.work(); } { return x.work(); } / void work() try {} catch(int) { recover(); }`<br>実測: constructorはvalue{x}で終了、requires版は制約式で終了、function-try-blockはcatch/recoverを含まない。同じ境界判定問題として3条件を1件に集約<br>期待: 初期化リストやrequires式を実bodyと区別し、function-try-blockのhandlerまで含める | [parser.go:279](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:279) / [parser.go:285](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:285)<br>test: `C04_braced_initializer_span / C04_requires_expression_body / C04_function_try_block`<br>層: extractor only |
| CP05 / P1 | **namespace alias/using directiveが後続関数を飲み込む**<br>入力: `namespace original {} namespace alias=original; void later() {} / using namespace original; void later() {}`<br>実測: parseNamespaceが;を越えて次の{をnamespace bodyに採用し、laterのsymbol/chunkが消える<br>期待: 文末;で宣言を完了しlaterを独立抽出する | [parser.go:151](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:151) / [parser.go:160](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:160)<br>test: `C05_namespace_alias / C05_using_namespace`<br>層: extractor only |
| CP06 / P2 | **無名namespaceの所属が失われる**<br>入力: `namespace { void hidden() {} } void visible() {}`<br>実測: hiddenはQualifiedName=hidden、ParentHandle空で抽出され、namespaceの区別がない。別ファイルでの誤解決まではこの条件では主張しない<br>期待: 無名namespaceをコンテナとして区別してhiddenに紐付ける | [parser.go:157](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:157) / [parser.go:96](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:96)<br>test: `C06_anonymous_namespace`<br>層: extractor only |
| CP15 / P2 | **グローバルclassの修飾名にclass名が重複する**<br>入力: `struct Item { void run() {} };`<br>実測: 実際はItem::Item::run。constructorはItem::Item::Itemとなる。namespace n内のn::Item::runは正常対照として成功<br>期待: QualifiedName=Item::run | [parser.go:121](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:121) / [parser.go:555](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:555) / [parser.go:421](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:421)<br>test: `C16_global_class_qualification`<br>層: extractor only |

### 呼出しと束縛

| ID / 優先度 | 再現入力と実測結果 | 根拠 |
| --- | --- | --- |
| CP08 / P1 | **局所callback引数を同名の大域関数へ接続する**<br>入力: `void target() {} void caller(void (*target)()) { target(); } / install(Callback handler) { register_callback(handler); }`<br>実測: callsは大域targetへconfidence=.95。callback引数でも大域handlerへのreferences=.9が生成される。header参照とcallback参照も束縛確認対象<br>期待: 動的な引数から大域関数の確定関係を作らない | [extractor.go:363](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:363) / [extractor.go:473](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:473) / [extractor.go:298](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:298) / [extractor.go:413](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:413)<br>test: `C09_parameter_shadow / C09_callback_argument_shadow`<br>層: extractor only |
| CP09 / P1 | **object.method()を無関係な自由関数へ接続する**<br>入力: `void target() {} namespace n { struct Object { void target(); }; } void caller(n::Object& object) { object.target(); }`<br>実測: callerから自由関数targetへconfidence=.95のcallsが生成される。Objectは名前付きnamespaceに置きCP15との混同を除いた<br>期待: receiverを解決できない場合は未解決に保ち、大域targetを選ばない | [extractor.go:473](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:473) / [extractor.go:644](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:644)<br>test: `C10_receiver_method`<br>層: extractor only |
| CP10 / P1 | **明示的な別namespaceの呼出しを同名関数へ確定接続する**<br>入力: `#include "external.hpp"; namespace other { void work() {} } void caller() { external::work(); }`<br>実測: 同一ファイルのother::workへconfidence=.95で確定接続。external.hppに実際のexternal::work宣言を置いても索引に残り、other::workのcallers Evidenceにcallerが出る<br>期待: external::workへ限定し、other::workへfallbackしない | [extractor.go:473](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:473) / [extractor.go:490](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:490)<br>test: `C11_external_qualified_call / C11_wrong_qualified_call_indexed`<br>層: extractor + real index + RelatedCandidateHits + ExpandEvidence |
| CP19 / P2 | **無関係なnamespaceの追加で呼出しが未解決へ退化する**<br>入力: `a.hpp: n::Target, b.cpp: n::Caller->{Target()}, later add unrelated.hpp: m::Target`<br>実測: 追加前のn::Caller->n::Target確定を対照確認。追加後はUnresolvedTo=Targetとなり、m::Targetのcallers EvidenceにもCallerが混ざる。confidence=.25/Resolved=falseであり、CP10の誤った確定接続とは区別する。増分と全再構築の関係集合自体は一致<br>期待: 同じnamespaceとincludeに基づくn::Targetの区別を保持する | [extractor.go:392](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:392) / [extractor.go:473](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:473) / [linker.go:308](H:/sourcecode/FocalSpan/internal/linker/linker.go:308) / [store.go:1110](H:/sourcecode/FocalSpan/internal/store/store.go:1110)<br>test: `C21_cross_file_namespace_after_addition`<br>層: real index + incremental/full parity control + RelatedCandidateHits + ExpandEvidence |

### 宣言対応・module・template呼出し

| ID / 優先度 | 再現入力と実測結果 | 根拠 |
| --- | --- | --- |
| CP11 / P2 | **仮引数名の違いだけで宣言・定義の対応が消える**<br>入力: `void target(int value); void target(int renamed) {}`<br>実測: SignatureKeyに引数名を含めてグループ化し、同一ファイル・単一候補でもdeclaresが生成されない<br>期待: 同じ関数の宣言と定義をdeclaresで対応付ける | [parser.go:336](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:336) / [extractor.go:321](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:321) / [extractor.go:231](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:231)<br>test: `C12_parameter_names_in_pair`<br>層: extractor only |
| CP16 / P2 | **C++ moduleの依存先が文全体になり展開できない**<br>入力: `export module utilities; / import utilities;`<br>実測: Target=import utilities ;などの文全体を保存し、module-declarationもimports関係にする。utilities.cppmを置いてもimports ExpandEvidenceにproviderが出ない<br>期待: module宣言を依存と区別し、importのTargetをutilitiesへ正規化する | [parser.go:449](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:449) / [parser.go:467](H:/sourcecode/FocalSpan/internal/extract/cpp/parser.go:467) / [extractor.go:220](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:220)<br>test: `C17_module_target / C17_module_expansion`<br>層: extractor + real index + ExpandEvidence |
| CP17 / P2 | **テンプレート呼出しが関係候補にも現れない**<br>入力: `template<class T> void work() {} void caller() { work<int>(); }`<br>実測: (の直前が>なのでcallNameが空を返し、callsが丸ごと欠落する。型引数やoverloadの意味解析による確定解決は要求していない<br>期待: 少なくともworkへの未解決callsを残す | [extractor.go:381](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:381) / [extractor.go:644](H:/sourcecode/FocalSpan/internal/extract/cpp/extractor.go:644)<br>test: `C19_template_call`<br>層: extractor only |

## 再現手順と保存物

再現fixtureは`testdata/audits/cpp-focused-20261018/`へ保存した。意図的にFAILする監査assertionを通常のGoテストへ混ぜないため、`testdata/`から一時overlayで読み込む。C++は文字列として抽出するだけで実行しない。

```powershell
rtk proxy python testdata/audits/cpp-focused-20261018/run.py
```

baselineは0、extraction/integrationは未修正の問題が残る間nonzeroになる。runnerが表示する各logで個別のFAILを確認する。このnonzeroは既存テストの新規regressionではなく、現行実装の問題をassertした再現結果である。全通常・race・cross buildを今回通過したとは主張しない。

機械可読版は同名`.json`。C07とC18は正常対照用の内部テストIDであり、CPの問題IDとは別。原本のGo fixtureと再現入力のSHA256はJSONへ記録した。既存の完了済み`PLAN.md`・ユーザーの既存差分・master HEADは変更していない。

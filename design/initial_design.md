# EJQuick 設計書

> Status: Draft  
> 対象: 英辞郎 / 和英辞郎 TXT データを利用したローカル辞書検索ツール  
> 正式プログラム名: EJQuick

---

## 1. 概要

本プロジェクトは、BOOTH で販売されている英辞郎（英和）および和英辞郎（和英）の TXT データを、ローカル環境で高速に検索するための TUI / CLI 辞書ツールを実装することを目的とする。

主な特徴は以下とする。

- Linux / Windows / macOS の3 OSに対応する
- Pure Go で実装し、単一バイナリ配布を基本とする
- 起動速度と検索レスポンスを重視する
- 英辞郎・和英辞郎の TXT データを SQLite に変換して利用する
- 前方一致検索と部分一致検索を組み合わせる
- TUI では fzf のようなインクリメンタル検索体験を目指す
- 辞書データ変換処理と検索アプリ本体は別バイナリに分離する
- アプリケーション本体は OSS として GitHub で公開する
- 辞書データそのものはリポジトリへ含めない

将来的に GUI フロントエンドを追加する可能性はあるが、初期フェーズでは TUI / CLI に集中する。

---

## 2. 目的

### 2.1 主目的

BOOTH で販売されている以下の TXT データを元に、ローカルで高速に利用できる辞書検索ツールを作成する。

- 英辞郎: 英和辞書
- 和英辞郎: 和英辞書

検索対象データは購入者自身が用意し、本プロジェクトでは辞書データそのものを配布しない。

### 2.2 重視する点

優先順位は以下とする。

1. 起動が速い
2. キー入力に対する検索結果表示が速い
3. Linux / Windows / macOS で同じ操作感を提供する
4. 依存関係を少なくする
5. 辞書データが数百万件規模でも快適に動作する
6. TUI / CLI としてシェル環境との親和性を高くする
7. 将来 GUI を追加しやすい内部構造とする

### 2.3 初期フェーズでは行わないこと

以下は初期リリースの対象外とする。

- GUI アプリケーション
- Web アプリケーション
- サーバー型辞書 API
- 辞書データのネットワーク共有
- 辞書データ自体の配布
- クラウド同期
- ユーザー辞書編集機能
- 複数ユーザーでの辞書共有

---

## 3. 対応環境

### 3.1 対応 OS

- Linux
- Windows
- macOS

### 3.2 実装言語

Go を使用する。

原則として Pure Go で実装し、OS ごとのネイティブ依存を極力避ける。

SQLite driver には CGo を必要としない `modernc.org/sqlite` を採用し、検索アプリと DB Builder の両方で同じ driver を使用する。

正式実装へ進む前に、FTS5 trigram、external content table、実行中 query の context cancellation、read-only open が動作することを smoke test で確認する。また、`CGO_ENABLED=0` で配布対象の OS / architecture を build できることを継続的に検証する。

### 3.3 TUI ライブラリ

基本構成は以下とする。

- Bubble Tea v2
- Lip Gloss（必要な場合）

Bubble Tea の既存コンポーネントを無理に利用するのではなく、高速性と fzf 的 UI を優先し、検索結果一覧は必要に応じて独自描画する。

---

## 4. 全体アーキテクチャ

システムを大きく以下の2つに分離する。

1. 辞書データ変換ツール
2. 辞書検索ツール

概念構成:

```text
英辞郎 TXT --------------------+
                               |
                               v
                         +-------------+
                         | DB Builder  |
                         | / Converter |
                         +-------------+
                               |
                               v
                         eiji.sqlite3
                               |
                               |
                               +-------------------+
                                                   |
                                                   v
                                                TUI/CLI
                                                   ^
                                                   |
                               +-------------------+
                               |
                         waei.sqlite3
                               ^
                               |
                         +-------------+
                         | DB Builder  |
                         | / Converter |
                         +-------------+
                               ^
                               |
和英辞郎 TXT --------------------+
```

英辞郎と和英辞郎は同一 SQLite ファイルへ入れず、別々の database file とする。

例:

```text
~/.local/share/ejquick/
├── eiji.sqlite3
└── waei.sqlite3
```

実際の DB パスは設定ファイルで変更可能とする。

---

## 5. バイナリ構成

正式な製品表記は `EJQuick` とする。README、help、release titleなど利用者向けの文章ではこの表記を使用する。

command、binary filename、config・data directoryなどの技術識別子には小文字の `ejquick` を使用する。検索アプリは `ejquick`、DB変換ツールは `ejquick-build` とし、大文字小文字や空白を含む別名は設けない。

GitHub repository名は `ejquick`、URLは `https://github.com/simosako/ejquick` とする。Go module pathも `github.com/simosako/ejquick` に統一する。

### 5.1 検索アプリ

```text
ejquick
```

役割:

- TUI 起動
- CLI 検索
- SQLite DB の読み込み
- インクリメンタル検索
- 検索結果表示
- 辞書エントリ詳細表示

### 5.2 DB 変換ツール

```text
ejquick-build
```

役割:

- TXT ファイル読み込み
- CP932（Windows-31J）から Unicode / UTF-8 への変換
- 英辞郎 / 和英辞郎フォーマットのパース
- 正規化用データ生成
- SQLite DB 作成
- B-tree INDEX 作成
- FTS5 virtual table 作成
- FTS index の生成
- ANALYZE / PRAGMA optimize と、指定時のみ VACUUM を実行
- DB メタデータ保存
- 変換時の統計情報表示
- エラー行の検出・報告

検索プログラム本体には TXT パーサーや DB 作成ロジックを極力持たせない。

---

## 6. 辞書データ

### 6.1 入力データ

対象:

- 英辞郎 TXT
- 和英辞郎 TXT

BOOTH で購入した TXT ファイルをユーザー自身が指定して変換する。

辞書データそのものは本プロジェクトの GitHub リポジトリには含めない。

### 6.2 文字コード

元 TXT の文字コードは CP932（Windows-31J）として固定する。汎用的な文字コード自動判定や、入力文字コードを変更する option は初期リリースでは提供しない。

CP932 として不正な byte sequence を検出した場合は、Unicode replacement character へ置換して処理を継続せず、入力行番号と byte offset を報告して変換を停止する。

DB Builder 内で以下を行う。

```text
CP932 (Windows-31J)
   ↓
Unicode
   ↓
UTF-8
   ↓
parse
   ↓
SQLite
```

Go 内部では UTF-8 を使用する。

### 6.3 行形式と異常行

英辞郎・和英辞郎ともに、1物理行を1エントリとして扱う。CRLF を除去した後、ASCII文字列 ` : ` がちょうど1個存在し、その前後がどちらも空でない行を正常とする。

```text
headword : body
```

区切りの欠落、複数の区切り、空の見出し語、空の本文は malformed line とする。malformed line はDBへ登録せず、入力行番号と理由を警告として報告して変換を継続する。今回確認した和英辞郎の ` : ` が2個存在する行も、この規則によりskipする。

decode error は行構造を安全に解釈できない入力破損として変換を停止する。SQLite error、disk full、FTS整合性エラーも停止対象とする。見出し語の重複は正常データであり、skipせずすべて保持する。

Builder は読込行数、登録件数、skip件数を完了時に表示し、DB metadata にも保存する。skipが発生しても、1件以上の正常なエントリを登録でき、その他の完成条件を満たしていればbuildは成功とする。

### 6.4 英辞郎と和英辞郎の分離

英和・和英は利用目的、検索キー、将来的な追加属性が異なる可能性があるため、DB file を分離する。

```text
eiji.sqlite3
waei.sqlite3
```

利点:

- DB ごとのスキーマを独立して進化させられる
- 英和のみ利用するユーザーが和英 DB を持つ必要がない
- 起動時に不要な DB を開かなくてよい
- DB ファイルの更新・再構築を個別に行える
- バックアップや差し替えが容易
- 将来、辞書ごとに異なる検索戦略を採用しやすい

---

## 7. SQLite 設計

### 7.1 基本方針

各辞書 DB には以下を持つ。

- 元データ相当の通常テーブル
- headword 検索用 INDEX
- FTS5 virtual table
- DB バージョンなどを持つ metadata

例:

```text
eiji.sqlite3
├── entries
├── entries_fts
└── metadata

waei.sqlite3
├── entries
├── entries_fts
└── metadata
```

### 7.2 英辞郎テーブル案

初期 schema:

```sql
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    headword      TEXT NOT NULL,
    headword_norm TEXT NOT NULL,
    body          TEXT NOT NULL
);
```

想定:

- `id`
  - 1始まりの元TXT物理行番号
- `headword`
  - 表示用見出し語
- `headword_norm`
  - 検索用に正規化した見出し語
- `body`
  - 語義・説明

元行を重複保存する `raw` 列は設けない。正常行の `id` には連番を振り直さず、Builder が元TXTの物理行番号を明示的にINSERTする。malformed lineをskipした場合は、その行番号がIDの欠番として残る。

これにより追加列なしでDBエントリと元TXTの行を対応付けられる。同一の元ファイルから再構築した場合はIDも安定する。SQLiteの `INTEGER PRIMARY KEY`、B-tree index、FTS5 rowidはいずれも欠番を許容するため、検索処理への影響はない。

### 7.3 和英辞郎テーブル案

基本形は同様だが、和英独自の追加属性が必要になった場合は別スキーマとして拡張できる。

```sql
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    headword      TEXT NOT NULL,
    headword_norm TEXT NOT NULL,
    body          TEXT NOT NULL
);
```

`id` と元TXT物理行番号の対応、および `raw` を保持しない方針は英辞郎と同じとする。

### 7.4 B-tree INDEX

前方一致検索用に `headword_norm` へ index を作成する。

```sql
CREATE INDEX idx_entries_headword_norm
ON entries(headword_norm);
```

1〜2文字入力時の検索では、この index を中心に利用する。

### 7.5 FTS5

3文字以上の部分一致検索用に FTS5 を利用する。

`entries` を正とする external content table 方式を採用する。

```sql
CREATE VIRTUAL TABLE entries_fts
USING fts5(
    headword_norm,
    content='entries',
    content_rowid='id',
    tokenize='trigram case_sensitive 1 remove_diacritics 0'
);
```

FTS5 trigram tokenizer を `case_sensitive 1 remove_diacritics 0` で使用する。大文字小文字を利用者に区別させる意図ではなく、Builderと検索queryへ適用済みの辞書種別ごとの正規化結果を、FTS5側でさらに変換しないための設定である。

大文字小文字の吸収はUnicode Case Folding、アクセントを区別するかどうかはNFC/NFKCを含むアプリケーション側の正規化仕様で一元管理する。これにより、B-treeによる完全一致・Prefix検索とFTS5によるSubstring検索で文字の同一性を揃え、SQLite内部のcase foldingへ依存しない。

初期リリースでは部分一致検索の対象を見出し語に限定し、検索用に正規化した `headword_norm` のみを FTS index へ登録する。`body` の全文検索は初期リリースの対象外とし、将来追加する場合は見出し語検索と分離した検索モードおよび index として設計する。

external content table の作成だけでは、既存の `entries` は FTS index へ登録されない。Builder は `entries` の投入後に以下を実行し、FTS index を明示的に構築する。

```sql
INSERT INTO entries_fts(entries_fts) VALUES('rebuild');
```

完成後のDBは検索アプリから read-only で使用し、`entries` を更新しないため、初期リリースでは FTS 同期用の INSERT / UPDATE / DELETE trigger を作成しない。Builder は完成前に FTS5 integrity-check を実行し、`entries` と FTS index の整合性を確認する。

`entries_fts` のrowidには `content_rowid='id'` により元TXT物理行番号と同じ値を使用する。IDに欠番があってもFTS検索と `entries` へのJOINには影響しない。

理由:

- 3文字以上の substring 検索と相性がよい
- `%keyword%` の全面 LIKE scan を回避できる
- 数百万件の辞書でも高速検索を期待できる

ただし、FTS5 trigram の Unicode / 日本語における実際の index サイズ、検索品質、性能はプロトタイプで検証する。

### 7.6 1〜2文字と3文字以上を分ける理由

FTS5 trigram は原理上3文字未満の検索には適さない。

そのため検索戦略を以下のように分ける。

```text
query length = 0
    ↓
DB検索を行わず、静的な操作案内を表示

query length = 1-2
    ↓
B-tree index による前方一致

query length >= 3
    ↓
FTS5 trigram 部分一致
（B-tree index の前方一致候補を優先）
```

1〜2文字の部分一致検索や、数百万件に対する全面 LIKE scanは行わない。

### 7.7 Unicode 文字数

検索文字数の判定は byte length ではなく Unicode code point / rune を基準とする。

特に和英辞郎では日本語入力が中心となるため、

```text
"英" = UTF-8では3 bytes
```

であっても、検索上は1文字として扱う。

Go では例えば以下の考え方を使用する。

```go
utf8.RuneCountInString(query)
```

---

## 8. 検索戦略

### 8.1 検索モード

初期リリースで利用者へ提供する検索モードはAutoのみとする。検索モードの設定項目や切替操作は設けない。

Autoモードは正規化後のquery文字数に応じて、以下の内部検索方式を選択する。

- Prefix
  - B-tree indexによる前方一致
- Substring
  - Prefix候補を優先し、FTS5による部分一致候補で残枠を補完

PrefixとSubstringは内部検索方式の名称であり、利用者が直接選択するモードではない。

将来的な候補:

- Exact
- Fuzzy
- Full text
- Regex

初期リリースでは対象外とする。

### 8.2 自動検索戦略

Autoモードでは入力文字数に応じて内部検索方式を自動的に切り替える。

```text
1文字:
    prefix / B-tree

2文字:
    prefix / B-tree

3文字以上:
    substring / FTS5
```

3文字以上で Substring に切り替わった場合も、完全一致および前方一致する見出し語をその他の部分一致より優先する。これにより、2文字から3文字へ入力が進んだ際の候補一覧の変化を抑える。

内部実装では、まず B-tree index で前方一致候補を検索し、検索結果の上限に空きがある場合のみ、FTS5 で前方一致以外の部分一致候補を補う。前方一致候補だけで上限に達した場合、その他の部分一致候補は取得しない。

### 8.3 検索件数

インクリメンタル検索では全件を取得せず、`max_results` を検索結果の上限とする。既定値は50、許容範囲は1〜500とする。

```text
default: 50
minimum: 1
maximum: 500
```

Prefix検索の `:limit` には `max_results` を使用する。Substring検索ではPrefix検索後の残枠だけを最終結果へ追加し、候補poolを含めても既定の内部上限を超えて保持しない。

設定読み込み時に範囲を検証し、範囲外は設定エラーとする。Search Service側でも呼び出し元にかかわらず500件をhard limitとして検証し、過大なqueryを防ぐ。

SQLでは検証済みの値をparameterとしてbindする。

```sql
LIMIT :limit
```

TUIはterminalに表示可能な行だけを描画するが、terminalの高さやresizeによって検索結果集合と `max_results` は変更しない。初期リリースではpaginationや追加読み込みを実装しない。

ユーザーが見られない数千〜数百万件をアプリへロードしない。

### 8.4 Prefix 検索

実装候補:

```sql
SELECT id, headword, body
FROM entries
WHERE headword_norm >= :lower
  AND headword_norm < :upper
ORDER BY headword_norm
LIMIT :limit;
```

あるいは SQLite の index を確実に利用できる形で `LIKE 'xxx%'` を使用する。

実際の query plan は `EXPLAIN QUERY PLAN` で確認する。

### 8.5 Substring 検索

3文字以上では FTS5 trigram を利用する。

Auto モードでは、最初に Prefix 検索を行い、残りの表示枠を Substring 検索で補う。Substring 検索からは、すでに取得した前方一致候補を除外する。

概念:

```sql
SELECT e.id, e.headword, e.headword_norm, e.body
FROM entries_fts f
JOIN entries e ON e.id = f.rowid
WHERE entries_fts MATCH :fts_query
  AND instr(e.headword_norm, :query) > 1
ORDER BY rank, e.id
LIMIT :candidate_limit;
```

`:query` は辞書種別に応じて正規化済みの検索文字列とする。`:fts_query` は、`:query` 内の `"` を `""` へ置換した後、全体を `"` で囲んだFTS5 quoted phraseとする。

```go
ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
```

SQL文字列へ値を連結せず、`:query` と `:fts_query` はどちらもSQL parameterとしてbindする。利用者が入力した `AND`、`OR`、`NOT`、引用符、括弧などはFTS5構文として解釈せず、常に検索対象のliteral文字列として扱う。初期リリースではFTS5 query syntaxを利用者へ公開しない。

`instr(e.headword_norm, :query) > 1` により、正規化済み見出し語にliteralなqueryが実在することを確認すると同時に、先にPrefix検索で取得した完全一致・前方一致候補を除外する。

FTS5の `rank` は最終的な表示順ではなく、アプリケーション側で再rankingする候補poolを絞るためだけに使用する。Prefix検索後の残枠を `remaining` とし、候補poolの初期上限を以下とする。

```text
candidate_limit = min(max(remaining * 10, 100), 500)
```

`remaining` が0の場合はFTS5 queryを実行しない。候補poolの倍率と上限は、実データで検索latencyと検索品質を測定した結果に基づいて調整できる内部定数とし、初期リリースでは設定項目にしない。

### 8.6 並び順

Prefix 検索:

1. 完全一致
2. 短い headword
3. 辞書順
4. `id`

Substring 検索:

1. 完全一致
2. headword の先頭に一致
3. その他の部分一致

完全一致と前方一致はPrefix検索結果から先に確定する。FTS5で抽出したその他の部分一致候補は、候補pool内で以下の順にアプリケーション側で安定sortし、残枠分だけ採用する。

1. `headword_norm` 内の一致開始位置が早い
2. `headword_norm` の文字数が短い
3. `headword_norm` の辞書順
4. `id`

一致開始位置と文字数はbyte数ではなくUnicode code point / rune単位で比較する。FTS5 rankを直接の表示順にしないことで、同じDBとqueryに対して決定的な結果順を維持する。ただし候補poolは全一致集合の一部なので、その他の部分一致全件に対する厳密な最適順は保証しない。インクリメンタル検索の応答時間を優先する。

---

## 9. 正規化

検索用の `headword_norm` を生成する。

表示用 `headword` は元データの表記を保持し、正規化は行わない。Builder で見出し語から `headword_norm` を生成する処理と、検索アプリで query を正規化する処理には、辞書種別ごとに同一の正規化関数を使用する。検索文字数は正規化後の query に対して数える。

### 9.1 英和

英和辞書では以下の正規化を行う。

1. 前後空白を除去する
2. Unicode NFC で正規化する
3. Unicode Case Folding を適用する

例:

```text
English
ENGLISH
english
```

を検索上は同一視する。

アクセント記号、句読点、全角英数字などに対する互換正規化や独自置換は、初期リリースでは行わない。

### 9.2 和英

和英辞書では以下の正規化を行う。

1. 前後空白を除去する
2. Unicode NFKC で正規化する
3. 見出し語に含まれる英字へ Unicode Case Folding を適用する

これにより、全角・半角の英数字、半角・全角カタカナなど、Unicode の互換等価として定義されている表記差を検索上は吸収する。

例:

```text
１
1
```

を検索上は同一視する。

漢数字、ひらがなとカタカナ、長音や送り仮名など、Unicode NFKC の対象外となる日本語固有の表記差は初期リリースでは同一視しない。

```text
一 != 1
```

### 9.3 将来拡張

将来、検索品質と実データでの効果を検証した上で、以下の追加を検討する。

- 英和における NFKC、アクセント記号、引用符、dash 等の表記揺れ吸収
- 和英におけるひらがな・カタカナの統一
- 和英における漢数字と算用数字の検索 alias
- 長音や送り仮名など、日本語固有の表記揺れへの対応

漢数字は一般語の一部にも現れるため、`一` を無条件に `1` へ置換する方式は採用しない。対応する場合は、元の `headword_norm` を維持したまま、追加の検索キーを保持する alias table または query expansion を検討する。

正規化ルールにはバージョンを付け、DB の metadata に `normalization_version` として保存する。正規化ルールを変更した場合はバージョンを更新し、DB を再構築する。検索アプリは対応していない正規化バージョンのDBを使用しない。

初期実装では過度な正規化を避け、異なる語を意図せず同一視しないことを優先する。

---

## 10. TUI 設計

### 10.1 基本コンセプト

fzf のように、起動すると画面最下部に入力欄があり、その上に候補が並ぶインクリメンタル検索 UI を目指す。

概念例:

```text
English             | English
English breakfast   |
English Channel     | Meaning and description of
English horn        | the selected entry...
English language    |
English muffin      |
English-speaking    |
Englishman          |

----------------------------------------------
> eng
```

ユーザーが文字を入力するごとに検索結果を即座に更新する。

起動時およびqueryが空の場合はDB検索を行わない。左の検索結果一覧を空にし、右の詳細ペインに静的な操作案内を表示する。

```text
Type to search
Tab: switch dictionary (when both are available)
Ctrl-C: quit
```

操作案内は検索結果ではなく、選択や詳細表示の対象にしない。実際のkey bindingと食い違わないよう、表示文字列はkey bindingの定義と同じ情報から生成する。利用可能な辞書が1つだけの場合はTabの案内を表示しない。

入力をすべて削除してqueryが空になった場合はrequest IDを更新し、実行中検索をcancelし、検索結果、選択位置、一覧scroll位置、詳細scroll位置、検索errorをclearする。`Selected` は選択なしを表す値へ戻す。その後に到着した旧requestのmessageはrequest IDにより破棄する。

### 10.2 TUI 画面構成

検索結果領域を左右2ペインへ分割する。

```text
検索結果一覧          | 選択中の見出し語
                      |
> selected entry      | 意味・説明
  another entry       | ...
  another entry       |
                      |
------------------------------------------------
status
> query
```

左ペインには検索結果の見出し語一覧を表示し、選択行を強調する。右ペインには選択中エントリの見出し語と本文を表示する。利用者がUp / DownまたはCtrl-P / Ctrl-Nで選択を移動すると、右ペインを即座に更新する。

current requestの新しい検索結果を反映する際は、置換前に選択中エントリの `id` を取得する。新しい結果内に同じ `id` が存在すればそのエントリを選択し続け、存在しなければ先頭を選択する。結果が0件の場合は選択なしとする。配列indexではなく一意な `id` で追跡し、同じ見出し語を持つ重複エントリも区別する。

選択を維持した結果、その行が左ペインの表示範囲外にある場合は、一覧scroll位置を必要最小限だけ移動して選択行を表示範囲内へ入れる。選択行がすでに表示範囲内なら一覧scroll位置を変更しない。新しい結果の反映時は、選択IDを維持できた場合も右ペインの詳細scroll位置を先頭へ戻す。

検索結果には本文も含まれているため、選択移動時にDB queryや `tea.Cmd` は発行しない。`Enter`による詳細画面への遷移は設けず、検索入力へfocusを維持したまま一覧を移動できるようにする。

queryが空の場合は左ペインを空にし、右ペインへ操作案内を表示する。current requestの検索errorがある場合は左右の結果を空にし、右ペインへ短いエラー内容を表示する。queryが空でなく検索errorもなく検索結果が0件の場合は、左ペインを空にし、右ペインへ該当結果がないことを表示する。

ペイン領域の幅を `pane_width`、中央の区切りを1 columnとして、左右の幅を以下のように計算する。

```text
left_width  = clamp(floor((pane_width - 1) / 3), 24, 50)
right_width = pane_width - 1 - left_width
```

左ペインは利用可能幅のおよそ1/3とし、最小24 columns、最大50 columnsに制限する。右ペインには中央区切りを除いた残りすべてを割り当て、長い本文の表示を優先する。statusとquery入力欄はペイン分割の下でterminal全幅を使用する。

通常時のstatusは現在の辞書名だけを `EIJI` または `WAEI` と表示する。Auto、内部検索方式、取得件数、選択位置、検索latency、DB path、request IDは表示しない。検索latencyなどの診断情報はdebug logで確認する。

DB fallbackや利用できない辞書へのTab操作など、利用者へ伝えるべき短い警告がある場合だけ通常の辞書名を警告文へ置き換える。警告は次の通常操作でclearし、現在の辞書名表示へ戻す。current requestの検索errorはstatusではなく右ペインへ表示し、右ペインのscroll indicatorはstatusとは別に扱う。

左ペインに収まらない見出し語は末尾をellipsisで省略し、完全な見出し語は右ペイン上部に表示する。幅の計算と切り詰めはbyte数やrune数ではなくterminal上のdisplay widthを基準とし、全角文字や結合文字を途中で分断しない。ANSI stylingによる制御sequenceは表示幅へ含めない。

terminal全幅が80 columns未満の場合は2ペインを別配置へ変更せず、terminalが狭いことと必要な最小幅を示す警告画面を表示する。query、検索結果、選択位置はmodel内に保持し、80 columns以上へ戻った際に同じ状態で2ペイン表示を復元する。resizeだけではDB queryを再実行しない。

右ペインの本文はペイン幅に合わせて折り返し、表示高を超える場合はPageUp / PageDownで1ページずつscrollする。1回の移動量は表示行数から1行を引いた値とし、前後のページに1行の重なりを残す。先頭と末尾を超えないようscroll位置をclampする。

選択項目を変更した場合、新しい検索結果を反映した場合、またはqueryを空にした場合は、詳細scroll位置を先頭へ戻す。resize時は新しい右ペイン幅で本文を再度折り返し、現在のscroll位置を有効範囲へclampする。

右ペインの下端には、本文が表示範囲を超える場合だけ現在位置と総行数が分かるscroll indicatorを表示する。PageUp / PageDownではqueryを変更せず、DB queryや `tea.Cmd` を発行せず、検索入力のfocusも維持する。

### 10.3 初期キー操作

初期リリースでは以下のkeymapを固定で提供し、key bindingの設定機能は設けない。

英和・和英の両DBが利用可能な場合は、Tabで辞書をtoggleする。切替時はrequest IDをincrementして実行中検索をcancelし、query、検索結果、選択位置、一覧scroll位置、詳細scroll位置、検索errorをすべてclearする。切替直後はDB検索を行わず、右ペインに操作案内を表示する。

利用可能なDBが1つだけの場合はTabを無効化する。Tabが押されても辞書や検索状態を変更せず、他方の辞書が利用できないことをstatusへ短く表示する。起動後にDB fileが追加されても自動検出せず、次回起動時に再検査する。

検索入力のfocusは維持し、利用者は切替先の辞書で新しいqueryを入力する。statusには現在の辞書を常時表示する。

| Key | Action |
|---|---|
| 文字入力 | queryのcursor位置へ挿入 |
| Left / Right | query cursorを前後へ移動 |
| Home / Ctrl-A | query cursorを先頭へ移動 |
| End / Ctrl-E | query cursorを末尾へ移動 |
| Backspace | cursor直前の文字を削除 |
| Delete | cursor直後の文字を削除 |
| Ctrl-W | cursor直前の単語を削除 |
| Ctrl-U | query全体をclear |
| Up / Ctrl-P | 前候補 |
| Down / Ctrl-N | 次候補 |
| PageUp | 右ペインを1ページ上へscroll |
| PageDown | 右ペインを1ページ下へscroll |
| Ctrl-C | 終了 |
| Tab | 英和 / 和英辞書を切替し、検索状態をclear |

文字入力、Backspace、Delete、Ctrl-W、Ctrl-Uはquery内容が実際に変化した場合だけ、request ID更新、旧検索cancel、新検索を行う。Left / Right、Home / End、Ctrl-A / Ctrl-Eはcursor移動だけを行い、検索を開始しない。削除対象がない場合もno-opとし、検索を開始しない。

文字挿入、cursor移動、Backspace、DeleteはUnicode grapheme cluster単位で扱い、結合文字やemoji sequenceを途中で分断しない。Ctrl-Wはcursor直前のUnicode whitespaceを削除した後、その前に連続する非whitespace grapheme clusterを削除する。日本語のように空白を含まない文字列では、cursor直前の連続部分全体が対象となる。Ctrl-Uはcursor位置にかかわらずquery全体をclearし、E2の空query状態へ戻す。

EnterとEscには初期リリースではactionを割り当てない。Ctrl-Cで実行中検索をcancelし、DB connectionとlogをcloseして終了する。

fzf や shell TUI との操作感を大きく外さないようにする。

### 10.4 Bubble Tea モデル

概念:

```go
type Model struct {
    Query         string
    QueryCursor   int
    Results       []Entry
    Selected      int
    ListOffset    int
    DetailOffset  int
    Width         int
    Height        int
    Dictionary    DictionaryType
    Available     map[DictionaryType]bool
    RequestID     uint64
    CancelSearch  context.CancelFunc
    SearchError   string
    StatusWarning string
}
```

### 10.5 描画

大量の検索結果を `list` widget へ渡す必要はない。

DB から表示可能件数 + α のみ取得し、文字列として効率的に描画する。

terminalの既定foreground/backgroundを使用し、文字色と背景色は指定しない。背景がdarkかlightかの判定、ANSI 16色・256色・True Colorのpalette、theme設定は初期リリースへ導入しない。

stylingは以下の属性だけに限定する。

- 選択行: reverse
- 右ペインの見出し語: bold
- 区切り線、操作案内、scroll indicator: dim
- status警告、右ペインの検索error: bold

選択行には `>` も表示し、警告とerrorには内容を表す明示的な文言を使用する。reverse、bold、dimが無効なterminalやplain textであっても、色や属性だけに依存せず状態を判別できるようにする。

ANSI stylingを適用した後の文字列長ではなく、装飾を除いたdisplay widthでlayoutを計算する。過度に複雑なstylingは起動速度・描画速度・保守性の観点から避ける。

### 10.6 Mouse

初期リリースではmouse操作に対応せず、Bubble Teaのmouse trackingを有効化しない。click、wheel、hoverのevent handlerやmouse設定は実装しない。

terminal本来のmouseによる文字選択とcopyを妨げず、検索、一覧選択、詳細scroll、辞書切替、終了のすべてをkeyboardだけで操作可能とする。将来mouse対応を追加する場合は、まずright paneのwheel scrollから別途設計する。

---

## 11. インクリメンタル検索

### 11.1 基本動作

キー入力ごとに query を更新し、検索する。

```text
e
 ↓
search("e")

en
 ↓
search("en")

eng
 ↓
search("eng")

engl
 ↓
search("engl")
```

query内容が変化した入力はdebounceせず、直後に検索を行う。選択移動や画面resizeなど、queryを変更しないeventでは検索を開始しない。

### 11.2 非同期検索

DB query を Bubble Tea の Update 内で長時間ブロックさせない。

検索は `tea.Cmd` を利用し、Repositoryまで `context.Context` を渡す。新しい検索を開始するたびに、実行中の検索contextをcancelする。

概念:

```go
func searchCmd(ctx context.Context, requestID uint64, query string) tea.Cmd {
    return func() tea.Msg {
        result, err := repository.Search(ctx, query)

        return SearchResultMsg{
            RequestID: requestID,
            Result:    result,
            Err:       err,
        }
    }
}
```

Repositoryは `database/sql` の `QueryContext` を使用する。Auto検索内のPrefix queryと、必要な場合に続けて実行するFTS5 queryには同じcontextを渡し、各段階でcancelを反映する。

空でないqueryが別の空でないqueryへ変化した場合は、既存の検索errorをclearする。前回検索が成功していれば、新しい検索結果が返るまで現在の検索結果、選択位置、一覧scroll位置、詳細scroll位置をそのまま表示する。dim表示、spinner、`Searching...`などの追加表示は行わず、loading状態もmodelへ追加しない。query入力だけを即時更新する。

検索待ちの間も左の選択移動と右のscrollを通常どおり受け付ける。current requestの結果が返った時点でN2の選択維持規則に従って新結果へ置き換える。辞書切替時は別辞書の結果を表示し続けず、結果と選択・scroll位置を即座にclearする。

### 11.3 stale result の破棄

高速入力時には検索結果の戻り順が query 順と一致しない可能性がある。

例:

```text
search("e")
search("en")
search("eng")
```

`search("e")` が最後に返る可能性がある。

cancelは不要になったSQLの処理量を減らすために使用するが、結果反映の正しさはcancelの成否に依存させない。SQLがすでに完了し、結果messageがqueueへ入った後ではcancelできないため、単調増加するrequest IDを必ず併用する。

```go
if msg.RequestID != m.RequestID {
    // Ignore stale results, including stale errors.
    return m, nil
}
```

新しい検索を開始する処理は以下の順序とする。

1. modelのrequest IDをincrementし、新しいIDをcurrentとする
2. 前回の `CancelSearch` があれば呼び出す
3. 新しいcontextと `CancelFunc` を作成してmodelへ保持する
4. request IDとcontextを渡して `tea.Cmd` を開始する

query文字列そのものによる一致判定は使用しない。例えば `a` → `ab` → `a` と入力が変化した場合、最初と最後の `a` を異なるrequestとして識別する必要がある。

旧requestの結果とエラーは、完了時点やcancel結果にかかわらず破棄する。`context.Canceled` は通常の制御フローであり、statusやlogへ利用者向けエラーとして表示しない。辞書切替、queryの空文字化、アプリケーション終了時にもcurrent requestを無効化して実行中検索をcancelする。

current requestの結果messageを処理する場合も、最初に `context.Canceled` を判定する。それ以外の検索errorの場合は、現在のqueryを保持したまま検索結果、選択位置、一覧scroll位置、詳細scroll位置をclearし、右ペインへ利用者向けの短いエラー内容を表示する。詳細なerrorは辞書種別とrequest IDを付けてlog fileへ一度だけ記録する。

検索errorでTUIを終了せず、次のquery変更または辞書切替で通常の検索を再試行する。次の検索開始時に画面上の検索errorをclearし、成功結果を受け取った場合も検索errorをclearしてから結果を反映する。

### 11.4 debounce

初期リリースではdebounceを入れず、待機timerや設定項目も設けない。queryが変化した同じ `Update` 処理内でrequest IDを更新し、旧contextをcancelして、新しい検索 `tea.Cmd` を返す。fzfのような即応感を優先する。

実データbenchmarkでcancel頻度やDB負荷が問題になった場合のみ、将来の設計変更としてSubstring検索への短いdebounceを再検討する。初期実装へ未使用のdebounce機構は入れない。

---

## 12. CLI 設計

同じ `ejquick` binaryをTUIとCLI検索に使用し、positional queryの有無で動作を決める。subcommandや `--tui` / `--query` のような動作選択optionは設けない。

### 12.1 起動形式

positional argumentがない場合はTUIを起動する。

```bash
ejquick
```

positional argumentが1つある場合は、その値をqueryとしてCLI検索を1回実行し、結果をstdoutへ出力して終了する。

```bash
ejquick english
ejquick "take care"
ejquick --dictionary waei "英語"
```

queryは正確に1つのpositional argumentとして受け取る。空白を含むqueryはshellでquoteする。複数のpositional argumentは暗黙に空白で結合せず、usage errorとする。空文字、または辞書種別ごとの正規化後に空となるqueryもusage errorとする。

`-` で始まる文字列を検索する場合は、`--` でoption解析を終了する。

```bash
ejquick -- "-prefix"
```

### 12.2 Option

初期リリースでは以下を提供する。

```text
-d, --dictionary <eiji|waei>
-c, --config <path>
    --limit <1..500>
    --format <plain|jsonl>
-h, --help
-v, --version
```

`--dictionary` を省略した場合はconfigの `default_dictionary` を使用する。`--limit` はconfigの `max_results` をそのprocessだけ上書きし、TUIとCLI検索の両方へ適用する。範囲外の値、未知のoption、option valueの欠落はusage errorとする。

`--format` はCLI検索でのみ使用でき、省略時は `plain` とする。positional queryがないTUI起動で `--format` を明示した場合は、無視せずusage errorとする。

CLI検索もTUIと同じAuto検索のみを使用する。初期リリースでは `--prefix`、`--substring` などの検索方式を選択するoptionを設けない。TUIへ初期queryを渡す機能も設けない。

### 12.3 Stdout format

CLI検索は `plain` と `jsonl` の2形式を提供する。どちらもUTF-8でstdoutへ出力し、ANSI styling、terminal幅による折り返し・切り詰め、見出し行、進捗、警告を含めない。診断とerrorはstderrまたはlog fileへ出力する。

#### Plain

既定形式。各エントリを見出し語、本文、空行の順で出力する。表示用 `headword` と `body` を改変しない。

```text
English
meaning and description...

English breakfast
meaning and description...
```

最後のエントリもnewlineで終了する。結果が0件の場合はstdoutへ何も出力しない。

#### JSON Lines

`--format jsonl` では1エントリを1つのJSON objectとして1行に出力する。

```json
{"id":123,"dictionary":"eiji","headword":"English","body":"meaning and description..."}
```

fieldは `id`、`dictionary`、`headword`、`body` に固定し、`headword_norm`、FTS rank、内部の一致位置は出力しない。文字列は標準JSON規則でescapeし、各objectをnewlineで終了する。JSON配列にはせず、検索結果を順次処理できるようにする。

両形式ともSearch Serviceから返された順序と重複エントリを維持する。

### 12.4 Pipeline

標準出力へ出せることで以下と組み合わせられる。

```bash
ejquick english | less
```

初期リリースではstdinをCLI queryの入力源として使用しない。positional queryがあるCLI検索では、stdinがterminalでもpipeでも読み取らず、入力待ちを発生させない。`--stdin` optionと複数行のbatch検索も提供しない。

positional queryがない場合はstdinの内容やTTY状態からCLI検索へ暗黙に切り替えず、TUI起動を選択する。TUIに必要なinteractive terminal input/outputを利用できない場合は、pipe内容をqueryとして解釈せず、TUIを開始する前に明確なエラーをstderrへ出力する。

将来的には clipboard や shell command との連携も可能。

TUI と CLI の検索ロジックは共通 package を利用する。

### 12.5 Exit status

`ejquick` のCLI検索はgrep系検索commandと同じ3値を使用する。

```text
0  検索に成功し、1件以上の結果を出力した
1  検索に成功したが、結果が0件だった
2  usage、config、DB、検索、stdout書込のerror
```

結果が0件の場合はstdoutを空のままexit code 1で終了する。error内容はstderrへ出力し、stdoutへ混在させない。出力途中のwrite errorは、すでに一部を出力していた場合もexit code 2とする。

`--help` と `--version` はconfigやDBを読み込まず、表示に成功すればexit code 0とする。TUIのCtrl-Cによる通常終了は0、TUI開始前のusage、config、DB、terminal errorは2とする。OS signalによる強制終了codeはshellとplatformの慣例に従う。

---

## 13. 設定ファイル

### 13.1 形式

TOML

### 13.2 配置場所

Linuxでの基本path:

```text
~/.config/ejquick/config.toml
```

Windows では XDG 相当の扱いをどうするかを実装時に決定する。

候補:

- Go の `os.UserConfigDir()`
- XDG_CONFIG_HOME を尊重
- Windows は `%AppData%`

OS ごとの慣習に従う。ただし、TUI/CLIプログラム起動時に -c オプションで、独自の位置、ファイル名で保存されているコンフィグファイルを指定して読み込ませることを可能にする。

### 13.3 設定例

```toml
[eiji]
database = "~/.local/share/ejquick/eiji.sqlite3"

[waei]
database = "~/.local/share/ejquick/waei.sqlite3"

[search]
default_dictionary = "eiji"
max_results = 50
```

`search.max_results` を省略した場合は50を使用する。1〜500の範囲外は起動時の設定エラーとし、暗黙に丸めない。

`search.default_dictionary` は `eiji` または `waei` とし、それ以外は設定エラーとする。指定した既定辞書が利用できない場合のfallbackは19.1の規則に従う。

候補項目:

- DB path
- default dictionary
- result limit

初期リリースでは設定項目を増やしすぎない。

---

## 14. DB Builder 設計

### 14.1 基本 CLI

例:

```bash
ejquick-build \
  --type eiji \
  --input EIJIRO144-10.TXT \
  --output eiji.sqlite3
```

和英:

```bash
ejquick-build \
  --type waei \
  --input WAEIJI-144-10.TXT \
  --output waei.sqlite3
```

既存DBを明示的に置換する場合のみ `--force` を指定する。

```bash
ejquick-build --type eiji --input EIJIRO144-10.TXT --output eiji.sqlite3 --force
```

### 14.2 変換フロー

```text
TXT
 ↓
出力先と同じdirectoryに一時DBを作成
 ↓
Builder用PRAGMAを設定
 ↓
strict CP932 decode
 ↓
line reader
 ↓
parser
 ↓
normalize
 ↓
SQLite transaction
 ↓
entries insert
 ↓
B-tree index
 ↓
FTS5 external content table 作成
 ↓
FTS5 rebuild
 ↓
FTS5 integrity-check
 ↓
`--compact` 指定時のみ VACUUM
 ↓
ANALYZE
 ↓
PRAGMA optimize
 ↓
一時DBをcloseしてread-onlyで最終検査
 ↓
完成DBとして原子的にrename / replace
```

### 14.3 Builder PRAGMA

構築中の一時DBには以下の初期設定を使用する。`page_size` はschemaを作成する前に設定する。

```sql
PRAGMA page_size = 4096;
PRAGMA journal_mode = OFF;
PRAGMA synchronous = OFF;
PRAGMA locking_mode = EXCLUSIVE;
PRAGMA temp_store = FILE;
PRAGMA cache_size = -131072;
```

負数の `cache_size` はKiB単位の上限であり、初期値を128 MiBとする。`temp_store=FILE` として、B-tree indexやFTS indexの構築時に使用メモリが無制限に増加することを避ける。

一時DBは他processから利用せず、構築・検査中にエラーまたはprocess停止が発生した場合は破棄する。このためrollback journalと途中状態の耐久性を省略し、bulk buildの速度を優先する。エラー後に同じ一時DBをrollbackして再利用しない。

完成DBはread-onlyで使用し、WAL modeは使用しない。完成時に `-wal` や `-shm` などのsidecar fileを必要としない単一DB fileとする。

`page_size=4096` と `cache_size=-131072` は初期値とし、実データbenchmarkではDB size、build時間、検索latency、最大RSSを測定する。変更する場合は測定結果を根拠とする。

### 14.4 INSERT 性能

数百万行を扱うため、1行ごとの autocommit は行わない。

以下を利用する。

- transaction
- prepared statement
- batch insert
- index 作成タイミングの最適化

候補:

1. entries table を作成
2. transaction 内で一括投入
3. B-tree INDEX 作成
4. FTS external content table 作成
5. FTS index rebuild
6. FTS5 integrity-check
7. `--compact` 指定時のみ VACUUM
8. ANALYZE
9. PRAGMA optimize

プログラム作成後に実際にデータを投入し、速度を測定。遅いようなら最適化を検討する。

### 14.5 Compaction

通常buildでは `VACUUM` を実行しない。新しい空DBへのbulk buildでは断片化が比較的小さく、`VACUUM` はbuild時間とpeak disk使用量を大きくするため、明示的な `--compact` 指定時だけ実行する。

配布用などDB sizeを優先するbuildでは以下のように指定する。

```bash
ejquick-build \
  --type eiji \
  --input EIJIRO144-10.TXT \
  --output eiji.sqlite3 \
  --compact
```

`VACUUM` はentries投入transactionとindex構築を完了した後、一時DBに対してtransaction外で実行する。失敗した場合はbuild全体を失敗とし、一時DBを破棄して既存の完成DBには影響させない。

`--compact` の有無にかかわらず、optional `VACUUM` の後に `ANALYZE` と `PRAGMA optimize` を実行し、その後に一時DBをcloseしてread-onlyの最終検査を行う。

### 14.6 Builder metadata

DB に生成情報を保存する。

例:

```sql
CREATE TABLE metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

候補:

```text
schema_version
dictionary_type
source_version
source_filename
build_time
builder_version
source_line_count
entry_count
skipped_entry_count
encoding
normalization_version
fts_version
compacted
```

`compacted` は `--compact` による `VACUUM` を完了した場合に `true`、通常buildでは `false` を保存する。

検索アプリは `schema_version` を見て互換性を確認する。

### 14.7 完成DBの公開

Builder は `--output` へ直接書き込まず、出力先と同じdirectoryに衝突しない名前の一時DBを作成する。同じfilesystem内でのrenameを利用できるよう、systemの一時directoryは使用しない。

optional `VACUUM`、`ANALYZE`、`PRAGMA optimize`を含むすべての構築処理を完了した後、DBをcloseし、一時DBをread-onlyで開き直してschema、metadata、`compacted`、件数、検索smoke testを検査する。検査に成功した一時DBだけを完成DBとして公開する。

公開前に一時DB fileを明示的に同期する。renameまたはreplace後は、OSが対応する場合に出力先directoryも同期し、電源断後に完成DBのdirectory entryが失われる可能性を抑える。

`--output` がすでに存在し、`--force` が指定されていない場合は、既存DBを変更せずエラー終了する。`--force` が指定された場合はOSごとの原子的な置換機能を使用し、既存DBを削除してからrenameする実装にはしない。置換処理に失敗した場合は既存DBを維持し、エラー終了する。

Windowsでは検索アプリなどが既存DBを開いていると置換に失敗する可能性がある。その場合は既存DBを維持したまま、使用中のアプリケーションを閉じて再実行するよう表示する。

構築または検査に失敗した場合は完成DBを変更せず、一時DBの削除を試みる。process強制終了などで一時DBが残っても完成DBとしては認識しない。

### 14.8 Progress output

Builderのprogress、警告、error、完了summaryはstderrへ出力し、stdoutは使用しない。stderrがterminalかredirect先かによって形式を変えず、すべてnewlineで終了する固定行として出力する。carriage returnによる行上書き、ANSI styling、spinner、progress bar、進捗率、ETAは使用しない。

各phaseの開始時と完了時に `start` / `done` を表示する。TXTの読込み・parse・INSERT中は、長時間無表示になることを避けるため、10万物理行ごとに累計の物理行数、登録件数、skip件数を追加表示する。入力完了時は10万行の境界かどうかにかかわらず最終値を表示する。FTS rebuildやVACUUMのように途中件数を取得できないphaseは開始と完了だけを表示する。

例:

```text
Reading: start
Reading: lines=100000 entries=99999 skipped=1
Reading: done lines=2389908 entries=2389907 skipped=1
B-tree index: start
B-tree index: done
FTS build: start
FTS build: done
Validation: start
Validation: done
Source lines: 2389908
Entries: 2389907
Skipped: 1
Database: 1.2 GiB
Compacted: no
Elapsed: 2m34s
```

完了summaryには物理行数、登録件数、skip件数、完成DB size、経過時間、compact有無を含める。`--compact` を指定した場合だけVACUUM phaseの開始・完了も表示する。初期リリースではprogressの頻度・形式を変更するoptionや `--quiet` optionを設けない。

### 14.9 Rebuild policy

初期リリースではincremental rebuildに対応せず、入力TXT全体から毎回新しい一時DBを構築する。既存の完成DBをbuildの入力、更新先、またはcacheとして使用せず、entries、B-tree index、FTS table、FTS index、metadataをすべて一から生成する。

`--force` は検査済み一時DBの公開時に既存DBを置換できるようにするoptionであり、build処理を差分更新へ変更しない。入力が前回と同一でもbuildを省略せず、source hashによるskip、append-only更新、行単位の差分適用は行わない。`--incremental` などのoptionや差分管理用metadataも設けない。

正常行の `entries.id` には入力TXTの物理行番号を使うため、途中行の追加・削除があれば後続entryも新しい物理行番号で再生成する。schema、正規化、FTS設定のversion変更時も同じfull rebuild経路を使用する。

---

## 15. Go package 構成案

```text
cmd/
├── ejquick/
│   └── main.go
└── ejquick-build/
    └── main.go

internal/
├── config/
├── dictionary/
├── search/
├── sqlite/
├── tui/
├── parser/
│   ├── eiji/
│   └── waei/
├── normalize/
└── builder/

pkg/
└── （原則必要になった場合のみ）
```

原則として public package を増やさず、初期段階では `internal` を中心にする。

検索ロジックを TUI に埋め込まない。

概念:

```text
TUI ----+
        |
CLI ----+--> Search Service --> Repository --> SQLite
```

これにより将来 GUI を追加する場合も、

```text
GUI ----+
TUI ----+--> Search Service --> Repository --> SQLite
CLI ----+
```

のように同じ検索エンジンを利用できる。

---

## 16. 将来 GUI を追加する場合

GUI は初期対象外。

ただし内部ロジックは GUI から利用可能な構造にする。

分離対象:

```text
presentation
    TUI
    CLI
    GUI (future)

application
    SearchService

domain
    Entry
    SearchQuery

infrastructure
    SQLiteRepository
```

HTTP server を前提にした architecture にはしない。

ローカル辞書用途なので、必要以上に抽象化しない。

---

## 17. 性能設計

### 17.1 起動

目標:

- SQLite DB を開くだけで即 TUI 表示
- DB 全体をメモリへ読み込まない
- 起動時に FTS rebuild をしない
- 起動時に大量の設定や plugin を読まない

### 17.2 検索

目標:

キー入力から画面更新まで、体感上遅延を感じないこと。

初期性能目標候補:

- prefix: 数 ms 程度
- substring: 数 ms〜数十 ms
- TUI redraw: 数 ms

正確な数値目標は実データで benchmark した後に決定する。

### 17.3 DB connection

ローカル read-only 検索が主体。

DB connection pool を大きくする必要はない。

新しい検索の開始時に前回のcontextをcancelし、request IDが一致する最新結果だけを反映する。DB driverによるcancel完了が遅れた場合でも、stale resultをTUIへ反映しない。

### 17.4 Read-only

検索アプリから辞書 DB を変更する必要はないため、可能なら read-only で open する。

これにより、

- 誤更新防止
- locking の単純化
- 安全性向上

が期待できる。

---

## 18. DB サイズとメモリ

数百万エントリを扱うが、全件をメモリ上にロードしない。

SQLite page cache と OS page cache に任せる。

FTS5 index により DB ファイルは TXT より大きくなる可能性が高い。

特に trigram index はサイズ増加が予想されるため、以下をベンチマークする。

- 元 TXT サイズ
- entries table サイズ
- B-tree index サイズ
- FTS5 index サイズ
- VACUUM 後サイズ
- prefix query latency
- substring query latency

---

## 19. エラー処理

### 19.1 Search app

起動時に英和・和英DBを独立して検査する。

- config file
- 各DB path
- read-only DB open
- schema version
- dictionary type
- normalization version
- required table
- FTS5 tableの存在とread-only smoke query

設定された既定辞書が利用可能なら、その辞書を選択して起動する。既定辞書が利用できず他方が正常な場合は、正常な辞書へfallbackしてTUIを起動し、利用できなかった辞書と理由をstatusおよびlog fileへ記録する。

利用可能なDBが1つだけの場合も検索機能を提供し、辞書切替を無効化する。両DBが利用できない場合のみTUI表示前にエラー終了し、各DBの失敗理由とBuilderによる作成方法をstderrおよびlog fileへ表示する。

起動時に確定した利用可能DBの集合はsession中に変更しない。DB fileを追加・置換した場合は、検索アプリを再起動して検査し直す。

問題がある場合は、短く明確なエラーを表示する。

例:

```text
eiji database not found:
  ~/.local/share/ejquick/eiji.sqlite3

Create it with:
  ejquick-build --type eiji ...
```

起動後の検索errorでは、request IDがcurrentであることと `context.Canceled` でないことを確認する。current requestのerrorだけを右ペインへ短く表示し、詳細をlog fileへ記録する。前回の検索結果はclearするがqueryは維持し、TUIは継続する。

### 19.2 Builder

変換時には以下を区別する。

- decode error: 変換を停止する
- malformed line / parse error: 該当行をskipし、行番号と理由を警告する
- SQLite error / disk full: 変換を停止する
- duplicate headword: 正常データとして保持する
- その他の unexpected data: 安全に行単位で分離できる場合のみskipし、それ以外は停止する

skipした行の内容全体は通常の進捗表示へ出力せず、行番号と理由のみを表示する。完了時にskip件数を明示し、DB metadata の `source_line_count`、`entry_count`、`skipped_entry_count` が一致することを検査する。

---

## 20. ロギング

起動失敗のような起動前のエラーはstderrに出力する。また、log fileにも出力する。
そのほかログは、log fileに追記する。(TUI の stdout をログで汚さない)
ただし、利用者に知らせるべきエラーや警告はTUI上にも表示する。current requestの検索errorは右ペインへ表示し、その他の短い警告はstatus領域へ表示する。
TUI のデバッグログは明示的なオプションで有効化する。

Builderのprogress、警告、error、完了summaryは14.8の固定行形式でstderrへ出力する。

---

## 21. テスト

### 21.1 Parser

英辞郎・和英辞郎それぞれについて fixture を作成する。

ただし実データを OSS repository に含めない。

重要 : テストデータはライセンス上問題のない人工データを使用する。

fixture には CP932 固有文字、CRLF、不正な byte sequence を含む人工データを用意し、正常な decode と異常位置の報告を検証する。

区切りの欠落、複数の区切り、空の見出し語、空の本文を含む人工データについて、該当行だけがskipされ、行番号・理由・集計値が正しく報告されることを検証する。重複見出し語はすべて保持されることも検証する。

登録された `entries.id` が元TXTの物理行番号と一致し、skipした行がIDの欠番として残ることを検証する。

### 21.2 Normalization

以下を unit test する。

- ASCII case
- Unicode
- 日本語
- whitespace
- punctuation

### 21.3 Search

SQLite の小規模 fixture DB をテスト時に生成する。

検証:

- exact
- prefix
- substring
- Unicode
- B-treeとFTS5でのcase folding結果の一致
- アクセント付き文字とアクセントなし文字の区別
- `AND`、`OR`、`NOT`、引用符、括弧を含むliteral query
- FTS5 query syntax errorとSQL injectionが発生しないこと
- Substring候補から完全一致・前方一致候補が除外されること
- FTS候補poolが内部上限を超えないこと
- Substring候補が一致位置、文字数、辞書順、`id`の順に安定して並ぶこと
- 日本語の一致位置と文字数をrune単位で比較すること
- `max_results` の既定値、および1・500の境界値
- 0、負数、501以上のresult limitを拒否すること
- terminal resizeで検索結果集合が変化しないこと
- 新しい検索開始時に前回のcontextがcancelされること
- cancel前に完了済みの旧requestが後から到着しても反映されないこと
- `a` → `ab` → `a` のように同じqueryへ戻ってもrequest IDで旧結果を区別すること
- stale requestの結果、エラー、`context.Canceled`をTUIへ表示しないこと
- current requestの検索errorで結果、選択、左右scroll位置をclearすること
- current requestの検索errorを右ペインへ表示し、queryとTUIを維持すること
- 検索errorの詳細を一度だけlog fileへ記録すること
- 次の検索開始時と成功時に以前の検索errorをclearすること
- `modernc.org/sqlite` の `QueryContext` で実行中queryを中断できること
- query変更と同じ `Update` で待機timerなしに検索commandを返すこと
- queryを変更しないeventでは検索commandを返さないこと
- 1文字
- 2文字
- 3文字
- 起動時とquery全削除時にDB検索を行わず、静的な操作案内を表示すること
- empty queryで結果、選択、scroll位置、検索errorがclearされること
- 操作案内を検索結果として選択できないこと

### 21.4 Performance

実辞書データは repository 外で benchmark する。

例:

```bash
go test -bench .
```

Builderのbenchmarkでは、少なくとも以下を記録する。

- DB size
- build時間
- Prefix / Substring検索latency
- 最大RSS
- 一時disk使用量

`--compact` の有無でDB size、build時間、最大RSS、peak disk使用量を比較し、配布用buildでの利用判断に使う。

### 21.5 Builder

以下を各対応OSで検証する。

- 既存DBがあり `--force` がない場合は変更せず失敗する
- 構築・検査失敗時に既存DBが維持される
- `--force` 成功時に検査済みDBへ原子的に置換される
- 置換失敗時に既存DBが維持される
- 残存した一時DBを完成DBとして使用しない
- 通常buildではVACUUMを実行せず、metadataの `compacted` が `false` になる
- `--compact` ではVACUUMを実行し、完了DBの `compacted` が `true` になる
- `--compact` 中の失敗で一時DBを破棄し、既存DBを維持する
- 両方のbuildでANALYZE、PRAGMA optimize、read-only最終検査を実行する
- progress、警告、error、完了summaryをstderrだけへ出力し、stdoutを空に保つ
- TXT処理中は10万物理行ごと、および入力完了時に累計件数を出力する
- 各phaseをnewlineで終了する固定行のstart / doneとして出力する
- terminalとredirect先で同じ形式を使用し、carriage returnやANSI sequenceを出力しない
- 途中件数を取得できないphaseでは割合やETAを表示しない
- 完了summaryに物理行数、登録件数、skip件数、DB size、経過時間、compact有無を含める
- `--compact` を指定した場合だけVACUUM phaseを表示する
- 通常buildと `--force` の両方が既存DBを再利用せず、新しい一時DBへ全件構築する
- 入力途中の行追加・削除後に再構築すると、各 `entries.id` が新しい物理行番号と一致する
- 新しい入力から削除されたentryが置換後のDBに残らない
- 入力が前回と同一でもbuildを省略しない
- `--incremental`、append-only更新、差分管理用metadataを提供しない

### 21.6 TUI

- 左ペインに検索結果、右ペインに選択中エントリの詳細を表示する
- 選択移動でDB queryを発行せず、右ペインだけを更新する
- queryが空の場合は右ペインへ操作案内を表示する
- 検索結果が0件の場合は右ペインへ該当結果がないことを表示する
- current requestの検索errorでは前回結果をclearし、右ペインへ短いエラーを表示する
- `Enter`なしで検索、選択、詳細確認を継続できる
- 左幅が利用可能幅の1/3かつ24〜50 columnsとなり、右へ残りが割り当てられる
- 全角文字、結合文字、ANSI stylingを含む見出し語をdisplay widthで安全に省略する
- 79 columns以下では警告し、80 columns以上へ戻ると状態を保ったまま2ペインを復元する
- resize時にDB queryを再実行しない
- PageUp / PageDownで右本文を1行重複させてページscrollする
- 選択変更、新しい検索結果、empty queryで詳細scroll位置を先頭へ戻す
- resize後の再折り返しでscroll位置を有効範囲へclampする
- scroll操作でquery、検索focus、DB queryが変化しない
- 新結果に同じ選択IDがあれば維持し、なければ先頭を選択する
- 重複見出し語を `id` で区別して選択を維持する
- 選択行が左ペインの表示範囲へ入るよう一覧scroll位置を調整する
- 非空queryの検索待ち中は前回結果、選択、scroll位置を通常表示で維持する
- 検索待ち中も選択移動と詳細scrollを受け付ける
- 辞書切替時は別辞書の結果と選択・scroll位置を即座にclearする
- Tabによる辞書切替でqueryと検索errorもclearし、DB検索を開始しない
- 辞書切替後も検索入力のfocusを維持し、statusの辞書表示を更新する
- 既定DBのみ、他方のみ、両方、どちらも利用不可の起動パターンを検証する
- 既定DBが利用不可なら正常な他方へfallbackして警告する
- 利用可能DBが1つならTab案内を隠し、Tabで検索状態を変更しない
- 両DBが利用不可なら両方の理由を示してTUI起動前に失敗する
- session中に追加されたDBは再起動まで利用可能集合へ加えない
- 通常のstatusには現在の辞書名だけを表示する
- 短い警告中だけstatusを警告文へ置き換え、次の通常操作で辞書名へ戻す
- 内部検索方式、件数、選択位置、latencyをstatusへ表示しない
- K2の固定keymapが対応するactionを実行する
- grapheme clusterを分断せずに挿入、cursor移動、前後削除を行う
- Ctrl-WがUnicode whitespaceと直前の非whitespace範囲を削除する
- Ctrl-Uでqueryをclearし、空query状態へ戻る
- queryを変えないcursor移動とno-op削除で検索を開始しない
- EnterとEscにactionが割り当てられていない
- terminal既定のforeground/backgroundを変更しない
- 選択、見出し、補助表示、警告・errorへ定義した属性だけを適用する
- ANSI属性を除外したdisplay widthでlayoutする
- stylingなしでも選択記号と文言で状態を判別できる
- mouse trackingを有効化せず、mouse event handlerを持たない
- keyboardだけですべての初期機能を操作できる

### 21.7 CLI invocation

- positional argumentなしでTUIを選択する
- positional argumentが1つならCLI検索を選択する
- 空文字、正規化後の空文字、複数のpositional argumentをusage errorにする
- `--` の後に `-` で始まるqueryを受け付ける
- `--dictionary` と `--limit` がconfigをprocess内だけで上書きする
- 未知のoption、option value欠落、範囲外のlimitをusage errorにする
- `--prefix` と `--substring` を提供しない
- CLI検索の既定formatがplainである
- plainが見出し語、本文、空行をANSI stylingや折り返しなしで出力する
- jsonlが固定fieldを正しくescapeし、1エントリ1行で出力する
- 両formatが結果順と重複を維持し、0件ではstdoutへ何も出力しない
- TUI起動で明示された `--format` をusage errorにする
- positional queryがある場合はstdinを読み取らない
- pipeされたstdinだけではCLI検索へ切り替わらない
- interactive terminalがないTUI起動を、stdinをqueryとして解釈せず拒否する
- `--stdin` と複数行batch検索を提供しない
- CLI検索が一致ありで0、一致なしで1、errorで2を返す
- 0件時はstdoutが空で、error時は診断がstderrだけに出る
- `--help` と `--version` がconfigとDBなしで0を返す
- TUIのCtrl-Cは0、TUI開始前errorは2を返す

---

## 22. OSS / ライセンス

アプリケーションは GitHub 上で OSS として公開予定。

EJQuick自身のsource codeとprojectで作成したdocumentationにはMIT Licenseを採用し、repository rootの `LICENSE` に全文を配置する。

```text
MIT License
Copyright (c) 2026 EJQuick contributors
```

第三者libraryはEJQuickのMIT Licenseへ変更せず、それぞれのlicenseに従う。依存versionを `go.mod` で固定した後、推移依存を含むlicense auditを実施して `THIRD_PARTY_NOTICES` を生成する。release前にも固定済み依存graphを再検査する。

`modernc.org/sqlite` のBSD-3-Clause copyright、license本文、免責条項をsource・binaryそれぞれの再配布条件に従って保持する。upstream SQLiteとFTS5はpublic domainとして扱う。将来 `modernc.org/sqlite/vec` を使用する場合は、同packageのMIT noticeも追加する。

重要:

- MIT Licenseの対象はEJQuickのproject成果物であり、利用者が用意する辞書データには適用されない
- 英辞郎 / 和英辞郎 TXT を repository に含めない
- 辞書TXTから生成した SQLite DB をrepositoryやreleaseへ含めない
- テスト fixture に辞書本文をコピーしない
- README ではユーザー自身が正規にデータを入手することを前提とする
- プロジェクトの OSS ライセンスと辞書データの利用条件を混同しない

---

## 23. 配布

Pure Go の利点を活かし、GitHub Releases で各 OS 向け binary を配布することを想定する。

候補:

```text
linux-amd64
linux-arm64
windows-amd64
windows-arm64
darwin-amd64
darwin-arm64
```

macOS Apple Silicon を正式対象とする。

将来的には以下も検討可能。

- Homebrew
- Scoop
- Winget
- AUR
- Nix
- `go install`

初期フェーズでは GitHub Releases を優先する。

各release archiveにはbinaryだけでなく、EJQuickの `LICENSE` と固定済み依存graphから生成した `THIRD_PARTY_NOTICES` を同梱する。

---

## 24. セキュリティ / プライバシー

本アプリはローカル完結を基本とする。

初期リリースでは外部通信を必要としない。

辞書 query、検索履歴、辞書データを外部へ送信しない。

Telemetry も原則導入しない。

---

## 25. 初期実装フェーズ案

### Phase 1: DB Builder prototype

- TXT reader
- CP932（Windows-31J）→ UTF-8
- parser
- entries table
- B-tree index
- FTS5 trigram
- `modernc.org/sqlite` の機能 smoke test
- `CGO_ENABLED=0` での cross build
- benchmark

目的:

まず検索方式が数百万件データで十分高速かを確認する。

### Phase 2: CLI search

```bash
ejquick english
```

を実装する。

TUI より先に検索 service と repository を固める。

### Phase 3: Minimal TUI

左右2ペイン型UI:

```text
results | selected entry detail
results |
results |

> query
```

検索、選択、詳細preview、PageUp / PageDownによる詳細scroll、Tabによる辞書切替をこのphaseで実装する。

### Phase 4: TUI refinement

- selection
- resize
- keyboard UX

### Phase 5: Distribution

- Linux
- Windows
- macOS

向け release binary を作成する。

### Phase 6: Future

必要に応じて GUI を検討する。

---

## 26. 設計上の重要原則

### 26.1 Fast path を単純にする

もっとも頻繁な処理:

```text
keypress
  ↓
normalize
  ↓
SQLite query
  ↓
LIMIT N
  ↓
render
```

この path に不要な abstraction、network、serialization を入れない。

### 26.2 DB は事前生成

検索時に以下をしない。

- TXT parsing
- encoding conversion
- FTS build
- index build

すべて Builder で事前に完了させる。

### 26.3 UI と検索エンジンを分離

Bubble Tea に SQLite query を直接埋め込まない。

```text
TUI
 ↓
SearchService
 ↓
Repository
 ↓
SQLite
```

とする。

### 26.4 辞書データと OSS を分離

repository はプログラムだけを公開する。

```text
GitHub
  source code
  schema
  importer
  tests
  documentation

Local only
  EIJIRO*.TXT
  WAEIJI*.TXT
  eiji.sqlite3
  waei.sqlite3
```

### 26.5 数百万件を前提にする

「データ量が少ない場合だけ高速」な構造を避ける。

常に以下を前提にする。

- index
- FTS
- LIMIT
- lazy access
- prebuilt DB

---

## 27. 現時点の推奨技術構成

```text
Language
  Go

TUI
  Bubble Tea v2
  Lip Gloss (optional)

Storage
  SQLite
  modernc.org/sqlite
  CGo disabled

Prefix search
  B-tree index

Substring search
  SQLite FTS5
  trigram tokenizer (case_sensitive=1, remove_diacritics=0)
  headword_norm only

Normalization
  eiji: NFC + Unicode Case Folding
  waei: NFKC + Unicode Case Folding

Configuration
  TOML

Source encoding
  CP932 (Windows-31J)

Internal encoding
  UTF-8

Databases
  eiji.sqlite3
  waei.sqlite3

Binaries
  search/TUI/CLI binary
  DB builder binary

Platforms
  Linux
  Windows
  macOS

Distribution
  GitHub Releases

GUI
  Future / undecided
```

---

## 28. 今後の検証項目

設計上、最初に実測すべきなのは SQLite / FTS5 部分である。

英辞郎・和英辞郎の実データを用いて以下を測定する。

1. TXT → SQLite 変換時間
2. SQLite DB サイズ
3. FTS5 trigram index サイズ
4. `e` の prefix 検索時間
5. `en` の prefix 検索時間
6. `eng` の prefix 検索時間
7. `eng` の substring 検索時間
8. 日本語1文字 prefix 検索時間
9. 日本語2文字 prefix 検索時間
10. 日本語3文字 substring 検索時間
11. cold start 時の検索 latency
12. warm cache 時の検索 latency
13. Windows / macOS / Linux の差
14. DB open から最初の TUI 描画までの時間

この結果を元に、FTS5 trigram を正式採用するか、別方式を検討する。

---

## 29. まとめ

本プロジェクトは、英辞郎 / 和英辞郎 TXT を事前に SQLite 化し、B-tree index と FTS5 を使って高速検索するローカル辞書ツールとする。

ユーザー体験の中心は fzf 的なインクリメンタル検索であり、

```text
起動
 ↓
すぐ入力
 ↓
入力ごとに結果更新
 ↓
候補選択
```

という短い操作経路を重視する。

英和と和英の database file を分離し、TXT → DB 変換処理を専用 Builder binary とすることで、検索アプリ本体を小さく・高速・単純に保つ。

初期実装では TUI / CLI に集中し、GUI は検索 engine が安定した後の将来拡張とする。

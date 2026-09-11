# 英辞郎・和英辞郎 TUI/CLI 辞書ツール 設計書

> Status: Draft  
> 対象: 英辞郎 / 和英辞郎 TXT データを利用したローカル辞書検索ツール  
> プログラム名: `ejquick`

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

プログラム名はメインプログラム（辞書検索）が ejquick 、そのためのデータを作るプログラム（DB変換）が ejquick-build

### 5.1 検索アプリ

```text
ejquick
```

役割:

- TUI 起動
- CLI 検索
- SQLite DB の読み込み
- インクリメンタル検索
- 検索モード切り替え
- 検索結果表示
- 辞書エントリ詳細表示

### 5.2 DB 変換ツール

```text
ejquick-build
```

役割:

- TXT ファイル読み込み
- Shift_JIS から Unicode / UTF-8 への変換
- 英辞郎 / 和英辞郎フォーマットのパース
- 正規化用データ生成
- SQLite DB 作成
- B-tree INDEX 作成
- FTS5 virtual table 作成
- FTS index の生成
- 必要に応じて ANALYZE / VACUUM 等を実行
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

元 TXT は Shift_JIS 系の文字コードを前提とする。

DB Builder 内で以下を行う。

```text
Shift_JIS
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

### 6.3 英辞郎と和英辞郎の分離

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

初期案:

```sql
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    headword      TEXT NOT NULL,
    headword_norm TEXT NOT NULL,
    body          TEXT NOT NULL,
    raw           TEXT
);
```

想定:

- `headword`
  - 表示用見出し語
- `headword_norm`
  - 検索用に正規化した見出し語
- `body`
  - 語義・説明
- `raw`
  - 元行を保持する必要がある場合のみ利用

`raw` を常に保存するかは容量と保守性を見て決定する。

### 7.3 和英辞郎テーブル案

基本形は同様だが、和英独自の追加属性が必要になった場合は別スキーマとして拡張できる。

```sql
CREATE TABLE entries (
    id            INTEGER PRIMARY KEY,
    headword      TEXT NOT NULL,
    headword_norm TEXT NOT NULL,
    body          TEXT NOT NULL,
    raw           TEXT
);
```

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
    tokenize='trigram'
);
```

FTS5 trigram tokenizer の採用を第一候補とする。

初期リリースでは部分一致検索の対象を見出し語に限定し、検索用に正規化した `headword_norm` のみを FTS index へ登録する。`body` の全文検索は初期リリースの対象外とし、将来追加する場合は見出し語検索と分離した検索モードおよび index として設計する。

external content table の作成だけでは、既存の `entries` は FTS index へ登録されない。Builder は `entries` の投入後に以下を実行し、FTS index を明示的に構築する。

```sql
INSERT INTO entries_fts(entries_fts) VALUES('rebuild');
```

完成後のDBは検索アプリから read-only で使用し、`entries` を更新しないため、初期リリースでは FTS 同期用の INSERT / UPDATE / DELETE trigger を作成しない。Builder は完成前に FTS5 integrity-check を実行し、`entries` と FTS index の整合性を確認する。

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
検索しない / 初期表示

query length = 1-2
    ↓
B-tree index による前方一致

query length >= 3
    ↓
FTS5 trigram 部分一致
（B-tree index の前方一致候補を優先）
```

ユーザーが「前方一致モード」を明示的に選択している場合は、3文字以上でも B-tree index を利用できるようにする。

ユーザーが Substring モードを明示的に選択していても、正規化後の query が1〜2文字の場合は Prefix 検索へ一時的にフォールバックする。数百万件に対する全面 LIKE scan は行わない。

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

最低限、以下の2モードを想定する。

- Prefix
  - 前方一致
- Substring
  - 部分一致
  - 正規化後の query が1〜2文字の場合は Prefix へ一時的にフォールバック

将来的な候補:

- Exact
- Fuzzy
- Full text
- Regex

初期リリースでは対象外とする。

### 8.2 自動検索戦略

デフォルトでは入力文字数に応じて検索手段を自動切り替えする。

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

明示的な Substring モードで1〜2文字の query を Prefix 検索へフォールバックしている間は、TUI のステータス領域に実効モードを表示する。

ただし、設定またはキー操作によって「常に prefix」を選択可能とする。

### 8.3 検索件数

インクリメンタル検索では全件を取得しない。

例:

```sql
LIMIT 50
```

画面サイズや設定値に応じて 50〜100 件程度を上限とする。

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
SELECT e.id, e.headword, e.body
FROM entries_fts f
JOIN entries e ON e.id = f.rowid
WHERE entries_fts MATCH :query
LIMIT :limit;
```

具体的な MATCH query のエスケープ、特殊文字処理、ランキングは実装時に確定する。

### 8.6 並び順

初期案:

Prefix 検索:

1. 完全一致
2. 短い headword
3. 辞書順

Substring 検索:

1. 完全一致
2. headword の先頭に一致
3. その他の部分一致

前方一致以外の候補内での並び順は以下を候補とする。

1. headword 内で早い位置に一致
2. 短い headword
3. 必要に応じて FTS rank

単純な FTS rank が辞書検索に適するとは限らないため、ランキング方式は実測して決定する。

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
English
English breakfast
English Channel
English horn
English language
English muffin
English-speaking
Englishman

────────────────────────────────────
> eng
```

ユーザーが文字を入力するごとに検索結果を即座に更新する。

### 10.2 TUI 画面構成

最終レイアウトは未決定。

今後比較検討する。

候補:

#### A. fzf 型

```text
検索結果
検索結果
検索結果
検索結果

-------------------------------
> query
```

最もシンプルで高速。

#### B. 候補 + preview 型

```text
候補一覧
候補一覧
候補一覧
-------------------------------
選択中エントリの意味
複数行の説明
-------------------------------
> query
```

Enter を押さなくても意味を確認できる。

#### C. 検索 / 詳細画面切替型

検索画面:

```text
候補一覧
...
> query
```

Enter:

```text
見出し語

意味
説明
...
```

Esc で検索画面へ戻る。

初期実装では A を最小構成とし、その後 B または C を評価する方針が有力。

### 10.3 キー操作案

未確定だが以下を候補とする。

| Key | Action |
|---|---|
| 文字入力 | query 更新 / 即検索 |
| Backspace | query 削除 / 即検索 |
| Up / Ctrl-P | 前候補 |
| Down / Ctrl-N | 次候補 |
| Enter | 選択 / 詳細表示 |
| Esc | 戻る |
| Ctrl-C | 終了 |
| Tab | 英和 / 和英切替候補 |
| Ctrl-F | Prefix / Substring 切替候補 |

fzf や shell TUI との操作感を大きく外さないようにする。

### 10.4 Bubble Tea モデル

概念:

```go
type Model struct {
    Query       string
    Results     []Entry
    Selected    int
    Width       int
    Height      int
    SearchMode  SearchMode
    Dictionary  DictionaryType
}
```

### 10.5 描画

大量の検索結果を `list` widget へ渡す必要はない。

DB から表示可能件数 + α のみ取得し、文字列として効率的に描画する。

Lip Gloss は以下に限定して使う。

- 選択行
- 区切り線
- status
- query prompt
- highlight

過度に複雑な styling は起動速度・描画速度・保守性の観点から避ける。

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

Web UI 的な大きな debounce は原則入れない。

SQLite + index + LIMIT で十分高速であれば、入力直後に検索を行う。

### 11.2 非同期検索

DB query を Bubble Tea の Update 内で長時間ブロックさせない。

検索は `tea.Cmd` を利用する構成を想定する。

概念:

```go
func searchCmd(query string) tea.Cmd {
    return func() tea.Msg {
        result, err := repository.Search(query)

        return SearchResultMsg{
            Query:  query,
            Result: result,
            Err:    err,
        }
    }
}
```

### 11.3 stale result の破棄

高速入力時には検索結果の戻り順が query 順と一致しない可能性がある。

例:

```text
search("e")
search("en")
search("eng")
```

`search("e")` が最後に返る可能性がある。

そのため検索結果 message に query または sequence number を保持する。

```go
if msg.Query != m.Query {
    // 古い検索結果
    return m, nil
}
```

より厳密には monotonic な request ID / generation ID を用いてもよい。

### 11.4 debounce

初期実装では debounce を入れない。

性能問題が出た場合のみ、

```text
20〜50 ms
```

程度の短い debounce を検討する。

fzf のような即応感を優先する。

---

## 12. CLI 設計

TUI だけでなく CLI としても利用可能にする。

例:

```bash
ejquick english
```

英和検索:

```bash
ejquick --eiji english
```

和英検索:

```bash
ejquick --waei 英語
```

prefix:

```bash
ejquick --prefix eng
```

substring:

```bash
ejquick --substring language
```

標準出力へ出せることで以下と組み合わせられる。

```bash
ejquick english | less
```

将来的には clipboard や shell command との連携も可能。

TUI と CLI の検索ロジックは共通 package を利用する。

---

## 13. 設定ファイル

### 13.1 形式

TOML

### 13.2 配置場所

基本:

```text
~/.config/<program-name>/config.toml
```

仮称:

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
default_mode = "auto"
max_results = 50

[tui]
preview = false
```

候補項目:

- DB path
- default dictionary
- default search mode
- result limit
- preview mode
- key bindings
- theme

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

### 14.2 変換フロー

```text
TXT
 ↓
encoding detection / Shift_JIS decode
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
ANALYZE
 ↓
必要に応じて compaction
 ↓
完成 DB
```

### 14.3 INSERT 性能

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
7. ANALYZE
8. 必要に応じて VACUUM

プログラム作成後に実際にデータを投入し、速度を測定。遅いようなら最適化を検討する。

### 14.4 Compaction

DB 作成後に必要に応じて以下を検討する。

```sql
ANALYZE;
VACUUM;
PRAGMA optimize;
```

VACUUM は時間と一時ディスクを多く使用するため、常に実行するか optional にするかは検討する。

例:

```bash
ejquick-build --compact
```

### 14.5 Builder metadata

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
entry_count
encoding
normalization_version
fts_version
```

検索アプリは `schema_version` を見て互換性を確認する。

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

同時に大量 query を発行しないよう stale query 制御を行う。

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

起動時に以下を確認する。

- config file
- DB path
- DB open
- schema version
- required table
- FTS5 availability

問題がある場合は、短く明確なエラーを表示する。

例:

```text
eiji database not found:
  ~/.local/share/ejquick/eiji.sqlite3

Create it with:
  ejquick-build --type eiji ...
```

### 19.2 Builder

変換時には以下を区別する。

- decode error
- malformed line
- parse error
- SQLite error
- disk full
- duplicate / unexpected data

大量データ処理なので、異常行1件ですべて停止するか、skip + report にするかを指定可能にすることを検討する。

---

## 20. ロギング

起動失敗のような起動前のエラーはstderrに出力する。また、log fileにも出力する。
そのほかログは、log fileに追記する。(TUI の stdout をログで汚さない)
ただし、ユーザーに知らせたほうが良いエラーや警告に関してはTUI上に表示する（TUI上にステータスを表示する領域を作り、そこにエラー・警告を出す）
TUI のデバッグログは明示的なオプションで有効化する。

Builder では、途中経過を画面に表示する。

例:

```text
Reading:   2,100,000 entries
Inserted:  2,100,000
FTS build: done
Database:  1.2 GiB
Elapsed:   ...
```

---

## 21. テスト

### 21.1 Parser

英辞郎・和英辞郎それぞれについて fixture を作成する。

ただし実データを OSS repository に含めない。

重要 : テストデータはライセンス上問題のない人工データを使用する。

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
- 1文字
- 2文字
- 3文字
- empty query

### 21.4 Performance

実辞書データは repository 外で benchmark する。

例:

```bash
go test -bench .
```

---

## 22. OSS / ライセンス

アプリケーションは GitHub 上で OSS として公開予定。

プロジェクト自身の OSS license は未決定。

候補例:

- MIT
- BSD-2-Clause / BSD-3-Clause
- Apache-2.0

ライセンスは採用依存ライブラリとの整合性を確認して決定する。

重要:

- 英辞郎 / 和英辞郎 TXT を repository に含めない
- 変換後 SQLite DB を repository に含めない
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

---

## 24. セキュリティ / プライバシー

本アプリはローカル完結を基本とする。

初期リリースでは外部通信を必要としない。

辞書 query、検索履歴、辞書データを外部へ送信しない。

Telemetry も原則導入しない。

---

## 25. 未決定事項

以下は今後決定する。

### プロジェクト

- 正式プログラム名
- OSS license
- repository 名

### SQLite

- FTS5 tokenizer 設定
- raw 列を保持するか
- page size / journal mode 等の PRAGMA

### 検索

- substring のランキング
- prefix と substring の UI 上の切替方法
- 検索結果 limit

### TUI

- 最終画面構成
- preview pane の有無
- detail view の有無
- key binding
- 色・テーマ
- status line
- 英和 / 和英切替 UI
- mouse support の有無

### CLI

- command / option 名
- stdout format
- JSON output の有無
- pipe 入力対応

### Builder

- source TXT format の厳密な parser
- malformed line の扱い
- VACUUM を default にするか
- progress UI
- incremental rebuild の有無

---

## 26. 初期実装フェーズ案

### Phase 1: DB Builder prototype

- TXT reader
- Shift_JIS → UTF-8
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

fzf 型 UI:

```text
results
results
results

> query
```

を実装する。

### Phase 4: TUI refinement

- selection
- detail
- preview
- mode switch
- dictionary switch
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

## 27. 設計上の重要原則

### 27.1 Fast path を単純にする

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

### 27.2 DB は事前生成

検索時に以下をしない。

- TXT parsing
- encoding conversion
- FTS build
- index build

すべて Builder で事前に完了させる。

### 27.3 UI と検索エンジンを分離

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

### 27.4 辞書データと OSS を分離

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

### 27.5 数百万件を前提にする

「データ量が少ない場合だけ高速」な構造を避ける。

常に以下を前提にする。

- index
- FTS
- LIMIT
- lazy access
- prebuilt DB

---

## 28. 現時点の推奨技術構成

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
  trigram tokenizer
  headword_norm only

Normalization
  eiji: NFC + Unicode Case Folding
  waei: NFKC + Unicode Case Folding

Configuration
  TOML

Source encoding
  Shift_JIS

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

## 29. 今後の検証項目

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

## 30. まとめ

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

# EJQuick ソースコードレビュー

レビュー日: 2026-09-12

基準文書: `design/initial_design.md`

対象: Git 管理されている Go ソース、テスト、`README.md`、`Makefile`、依存関係・ライセンス文書

## 総評

全体のアーキテクチャは設計書によく沿っている。Builder と検索アプリは別バイナリで、TUI/CLI は同じ Search Service と read-only Repository を共有し、正規化も Builder と検索側で共用されている。B-tree prefix 検索、FTS5 trigram substring 検索、候補数制限、非同期検索の request ID と context cancellation、事前生成 DB という主要方針も実装されている。

一方、現状のままリリースする前に修正すべき問題がある。最重要なのは、Builder で入力と出力に同じパスを指定すると購入済み TXT を DB で上書きできる点である。また、TUI の grapheme cluster 入力と Ctrl-W には、カーソル不整合、panic、文字列破壊につながるバグがある。設定・CLI にも設計書で明記された入力検証との差があり、command package のテストがまったくないため、これらが検出されていない。

重大度別の件数は次のとおり。

| 重大度 | 件数 | 意味 |
|---|---:|---|
| Critical | 1 | データ損失につながるため最優先で修正 |
| High | 3 | panic、入力破壊、明示された契約違反 |
| Medium | 9 | 利用者向け不具合または重要な設計差分 |
| Low | 6 | 保守性、冗長性、リリース品質の問題 |

## Critical

### C-1. `--input` と `--output` が同じ場合、入力 TXT を上書きする

該当箇所: `internal/builder/builder.go:64-75`, `internal/builder/builder.go:119-123`, `internal/builder/builder.go:400-414`

`Run` は入力ファイルを開いた後、`--force` があれば既存の出力を許可し、完成した一時 DB を `os.Rename` で出力パスへ置換する。入力パスと出力パスが同一ファイルかどうかは検証していない。

このため、Linux で人工データに対して次の形を実行すると exit code 0 で成功し、元の TXT が SQLite DB に置き換わることを確認した。

```text
ejquick-build --type eiji --input sample.TXT --output sample.TXT --force
```

辞書 TXT は購入データであり、上書きは回復不能なデータ損失になり得る。パス文字列の単純比較だけでは、相対パス、symlink、hard link、Windows の大文字小文字差を見逃す。

推奨変更: 入力と既存出力を `os.Stat` し、`os.SameFile` で同一ファイルなら `--force` の有無にかかわらず拒否する。加えて、clean 済み絶対パスも比較し、出力がまだ存在しない場合や OS ごとのパス表記差も扱う。人工 fixture で同一パス、相対表記違い、symlink/hard link をテストする。

## High

### H-1. grapheme cluster 入力後にカーソルが不正になり、後続操作が panic し得る

該当箇所: `internal/tui/update.go:143-148`, `internal/tui/update.go:204-225`

文字入力後の `QueryCursor` は、挿入内容にかかわらず常に 1 だけ増える。

```go
m.Query = insertRunes(m.Query, m.QueryCursor, msg.Text)
m.QueryCursor++
```

`Key.Text` が複数 grapheme cluster を含む場合、カーソルは挿入文字列の途中に残る。また、結合文字を別イベントとして入力すると、既存の cluster と結合して全体の cluster 数が増えないにもかかわらずカーソルだけが増える。たとえば `e` の後に combining acute accent を入力すると、表示文字列は1 clusterでもカーソルは2になる。その後の Backspace は `gs[i:]` の範囲外 slice に到達し、panic し得る。

単純に `graphemeCount(msg.Text)` を加算するだけでも、挿入前後の cluster が結合するケースは直らない。

推奨変更: 挿入 helper が「新しい query」と「挿入 byte 位置を全体で再分割した後の cursor cluster index」を一緒に返すようにする。結合文字、ZWJ emoji、regional indicator、複数文字の IME commit をテストする。

### H-2. Ctrl-W の whitespace-only 分岐が削除対象を返し、cursor 後方を消す

該当箇所: `internal/tui/update.go:237-255`

cursor より前がすべて whitespace の場合、次の分岐は削除後の文字列ではなく `gs[j:i]`、つまり削除対象そのものを返している。

```go
if j == 0 {
    return joinGraphemes(gs[j:i]), j, true
}
```

query が空白だけなら文字列は変わらず cursor だけ0になる。cursor 後方に文字がある場合は、その後方文字列を失う。設計書 10.3 の「cursor 直前の Unicode whitespace を削除」と逆の動作である。

推奨変更: この分岐では `joinGraphemes(gs[i:])` を返す。全空白、先頭空白の直後に cursor がある場合、cursor 後方に suffix がある場合を追加テストする。

### H-3. `Options.Progress == nil` は「出力無効」ではなく panic になる

該当箇所: `internal/builder/builder.go:24-39`, `internal/builder/builder.go:189-191`, `internal/builder/builder.go:299-300`, `internal/builder/builder.go:394`, `internal/builder/builder.go:431-444`

`Options.Progress` のコメントは `nil disables output` と明記しているが、`insertEntries`、`validate`、`phaseStart`、`phaseDone` は nil の `io.Writer` をそのまま `fmt.Fprint*` に渡す。この呼び出しは panic する。`writeSummary` だけは nil guard を持つため、実装も一貫していない。

command は常に `os.Stderr` を渡すものの、package 内 API の明示された契約と zero value の安全性が壊れている。テスト helper が nil を `os.Stderr` に置換するため、この問題が隠れている。

推奨変更: `Run` の入口で nil を `io.Discard` に置換し、以降の分岐を不要にする。`Progress: nil` で build が成功し、出力がないことをテストする。

## Medium

### M-1. CLI の空 query と TUI 用 `--format` の検証が設計と異なる

該当箇所: `cmd/ejquick/main.go:70-79`, `cmd/ejquick/main.go:95-171`, `cmd/ejquick/main.go:200-232`

設計書 12.1、12.2、21.7 との差は次のとおり。

| 入力 | 設計 | 現在の動作 |
|---|---|---|
| `ejquick --format jsonl` | usage error | `--format` を無視して TUI 起動を試みる |
| `ejquick ''` | usage error | positional query なしと区別できず TUI 起動を試みる |
| `ejquick '   '` | 正規化後空なので usage error | DB を開いた後、DB が正常なら no-result の exit code 1 |

前二つは実行して、TUI の TTY open error まで進むことを確認した。根本原因は query の有無を空文字列で表現しており、「引数なし」と「空の positional argument」を区別していないことである。

推奨変更: `parseArgs` から query 本体とは別に `queryPresent` を返す。TUI 分岐前に `--format` を拒否し、辞書種別確定後かつ DB open 前に query を正規化して空なら usage error にする。

### M-2. `max_results = 0` を明示してもエラーにならず50へ置換される

該当箇所: `internal/config/config.go:23-29`, `internal/config/config.go:85-95`, `internal/config/config.go:104-111`

設計書 8.3 と 13.3 は許容範囲を1から500とし、範囲外を暗黙に丸めないよう要求している。しかし `validate` は0を許可し、`applyDefaults` が0を「未指定」と解釈して50に置換する。実際に `max_results = 0` の config が受理されることを確認した。

推奨変更: TOML decode 用の raw struct では `*int` などを使って「未指定」と「明示的な0」を区別する。未指定だけ50にし、指定値は1から500で検証する。

### M-3. default config path の解決・検査エラーを黙って捨てる

該当箇所: `cmd/ejquick/main.go:183-195`

`loadConfig` は `config.DefaultPath()` のエラーと、`os.Stat` の `not exist` 以外のエラーをすべて無視して defaults を返す。設計書 13.2、21.8 は `os.UserConfigDir()` のエラーを起動エラーにするよう明記している。

`HOME` と XDG 関連変数を空にした検証では、`determine user config directory` ではなく相対的な `eiji.sqlite3` の open error まで進んだ。

推奨変更: `DefaultPath` のエラーは返す。`os.Stat` は `os.IsNotExist` の場合だけ defaults とし、permission、`ENOTDIR`、I/O error は起動エラーにする。

### M-4. 未知の TOML key が黙って無視される

該当箇所: `internal/config/config.go:48-64`

`Load` のコメントは unknown key を startup error としているが、`toml.Unmarshal` は unknown field を拒否する設定になっていない。未知 section と `[search]` 内の未知 key がどちらも受理されることを実行確認した。設定名の typo が defaults への silent fallback になる。

推奨変更: `toml.NewDecoder(...).DisallowUnknownFields()` 相当の strict decode を使い、top-level、section 内、似た名前の typo をテストする。

### M-5. 起動時検査に FTS read-only smoke query と B-tree index 検査がない

該当箇所: `internal/search/repository.go:37-48`, `internal/search/repository.go:57-95`

`verifySchema` は3 table の名前と4 metadata 値だけを検査する。設計書 19.1 が要求する FTS5 table の read-only smoke query を実行せず、prefix 用 `idx_entries_headword_norm` の存在も確認しない。metadata と同名 table だけを持つ不完全 DB が起動検査を通り、最初の検索時に失敗する可能性がある。

Builder 側の validation も、辞書順で最初の headword が3 runes未満なら FTS smoke query を省略する (`internal/builder/builder.go:365-392`)。

推奨変更: Repository open 時に prefix query planまたは index 存在、parameter bind した安全な FTS smoke queryを検査する。Builder validation は「3 runes以上の行を探して検査」にし、対象行がなければ FTS table 自体へ無害な query を実行する。

### M-6. detail pane の PageUp/PageDown に1行の重なりがない

該当箇所: `internal/tui/view.go:92-100`, `internal/tui/view.go:189-194`, `internal/tui/update.go:132-140`

本文が overflow すると scroll indicator 用に1行を予約するため、実際に見える本文行数は `detailHeight()-1` になる。一方、page step は `detailHeight()-1` である。したがって step は表示本文行数と同じで、設計書 10.2 が要求する前後ページの1行 overlap が発生しない。

推奨変更: indicator を差し引いた実際の visible body rows を共通 helper で計算し、step をその値から1引く。短い pane、先頭、末尾、resize 後をテストする。

### M-7. TUI の headword 表示が設計書の保存・表示方針と異なる

該当箇所: `internal/tui/view.go:153-160`, `internal/tui/view.go:183-186`, `internal/tui/view.go:283-290`

設計書 9 は表示用 `headword` の先頭 `■` を保持すると明記しているが、TUI は左右両 pane で `stripMarker` を呼んで除去する。CLI は DB の headword をそのまま出すため、TUI と CLI の表示も一致しない。

また設計書 10.2 は完全な headword を右 pane 上部に表示するとしているが、右 pane でも `truncateToWidth` により省略する。

推奨変更: 設計を正とするなら marker 除去をやめ、右 pane の headword は複数行 wrap などで完全表示する。UI 上 marker を隠す意図なら、実装ではなく設計書を先に変更し、CLI の仕様も同時に決める。

### M-8. 長い query と長い headword の描画が仕様どおりにならない

該当箇所: `internal/tui/view.go:241-257`, `internal/tui/view.go:292-318`

`renderQueryRow` は terminal 幅を考慮せず、長い query が折り返して pane 全体の高さを崩す。cursor を見える範囲へ保つ horizontal viewport もない。

`truncateToWidth` は、文字列が上限幅ちょうどまで埋まった後に続きがある場合、ellipsis 用の幅を確保できず、ellipsis なしで返す。これは設計書 10.2 の「末尾を ellipsis で省略」に反する。

推奨変更: query 行には cursor を含む display-width 基準の viewport を設ける。truncate は切り詰めが必要と分かった時点で末尾 cluster を戻して ellipsis の1 columnを確保する。全角、結合文字、幅ちょうど、幅超過を table test にする。

### M-9. debug log の query と検索 error の記録回数が設計と異なる

該当箇所: `cmd/ejquick/main.go:221-228`, `internal/tui/model.go:105-127`, `internal/tui/update.go:28-41`

設計書 20.2 は正規化済み query を debug log へ出すよう要求しているが、CLI/TUI とも raw query を記録する。TUI で通常 error が発生し `--debug` が有効な場合、`searchCmd` が DEBUG の `interrupted` 行を出し、その後 `Update` が ERROR 行を出すため、「詳細を一度だけ記録」という 11.3、21.3 の要件にも反する。

推奨変更: Search request に raw と normalized を明示的に持たせるか、Service が normalized query を返す。DEBUG は成功時だけ、current request の非-cancel error は ERROR だけにする。CLI にも設計上必要な request ID を付けるか、設計書側で CLI は対象外と明確化する。

## Low

### L-1. log file の実行中 write error が通知されない

該当箇所: `internal/logging/logging.go:121-129`

設計書 20.1 は log open または書き込み失敗時に stderr へ1行だけ警告する方針だが、`fmt.Fprintf` の戻り値を捨てている。open 後の disk full、権限変更、I/O error は無通知で、以後も失敗する write を繰り返す。

推奨変更: 最初の write error を `sync.Once` で stderr に警告し、その後は writer を `io.Discard` に切り替える。`Close` と write の同期も同じ mutex で扱う。

### L-2. bracketed paste が処理されない

該当箇所: `internal/tui/update.go:16-51`

Bubble Tea v2 は paste を `tea.PasteMsg` として送るが、`Update` は WindowSize、search result、KeyPress しか処理しない。そのため terminal の bracketed paste は query に入らない。fzf 的な検索 UI としては基本操作上の不足である。

推奨変更: `tea.PasteMsg.Content` を通常入力と同じ安全な grapheme insertion 経路へ渡し、改行を許可するか除去するかを明示する。

### L-3. 生成済み CP932 table を再生成・監査できない

該当箇所: `internal/parser/cp932table.go:1-9`

約80 KiBの table は `Code generated` とあるが、generator、`go:generate`、参照した Microsoft mapping table の URL・版・checksum が repository にない。現状の decoder test は一部 code point のみで、table 全体の正しさや依存更新時の再現性を確認できない。

推奨変更: 小さな generator と入力 mapping の取得元・checksumを管理する。ライセンス上 mapping file を同梱できない場合は取得・検証手順を文書化し、生成結果の代表範囲と未割当範囲をテストする。

### L-4. `go.mod` が tidy 状態でなく、未使用依存が残る

該当箇所: `go.mod:5-21`, `THIRD_PARTY_NOTICES:15-20`

`go mod tidy -diff` は差分ありで終了した。直接 import している Bubble Tea と `uniseg` が indirect 扱いで、未使用の Lip Gloss が残っている。完成 binary の `go version -m` には Lip Gloss は含まれない一方、`THIRD_PARTY_NOTICES` は release binary に埋め込まれる module として記載している。

推奨変更: `go mod tidy` 後の差分を確認して依存分類を正し、実際の release binary 2本の `go version -m` を基準に notices を再生成する。

### L-5. 小さい dead code、古いコメント、不要処理が散在する

該当箇所:

| 箇所 | 内容 |
|---|---|
| `internal/tui/update.go:153-154` | 存在しない `lastSearchedQuery` field を説明する古いコメント |
| `internal/tui/view.go:218-225` | 未使用引数 `w` と `_ = w` |
| `internal/tui/view.go:10,415` | documentation を理由にした未使用 `search` import と blank identifier |
| `internal/sqlite/smoke_test.go:33-80,262` | 呼ばれない `createFixtureDB` と import 維持用 `var _ = sql.DB{}` |
| `internal/config/config.go:96-100,209-225` | 結果を何も検証しない分岐と、そのためだけの `expandHome` |
| `tools/mkzip/main.go:37,67` | 計算後に `_ = root` するだけの変数 |
| `cmd/ejquick-build/main.go:48-55` | 取得直後に `_ = stats` する値 |
| `cmd/ejquick/main.go:51-54` | 現在の `parseArgs` から到達しない `opts == nil` 分岐 |

推奨変更: それぞれ削除するか、実際に必要な検証・動作へ接続する。特に stale comment は動作理解を誤らせるため早めに直す。

### L-6. 小規模ながら不要な allocation と非 idiomatic な処理がある

該当箇所: `internal/tui/update.go:258-265`, `internal/search/service.go:196-221`, `internal/builder/builder.go:245-252`

`joinGraphemes` は loop 内の `+=`、`runeIndex` は `sub` を `[]rune` へ2回変換、`sourceVersion` は build ごとに同じ regexp を compile している。query と候補 pool が小さいため現状の実害は限定的だが、簡単に除去できる無駄である。

推奨変更: `strings.Builder`、1回だけの rune 変換、package-level compiled regexp を使う。最適化より先に H-1 の正しい cursor semantics を確立する。

## テストレビュー

### T-1. command package のテストがなく、設計書 21.5・21.7 の主要契約が未検証

`cmd/ejquick` と `cmd/ejquick-build` は statement coverage 0.0% である。引数解析、TUI/CLI 分岐、plain/jsonl、stdout/stderr、exit code、`--help`/`--version` の DB 非依存、`--format` 制約、空 query が未テストである。今回の M-1 は、この欠落により残っている。

推奨変更: `main` から `run(args, stdin, stdout, stderr) int` を分離し、subprocess に頼らず大半を test 可能にする。実際の exit code と TTY 判定だけを少数の subprocess test で補う。

### T-2. 検索順テストの中心部分が何も assert していない

該当箇所: `internal/search/search_test.go:86-109`

`TestSearchExactComesFirstThenShorter` は exact の先頭だけを確認し、「短い headword 順」を確認する loop では変数を `_ =` に渡すだけで assertion がない。substring も先頭と1候補の存在だけで、位置、長さ、辞書順、ID の tie-break 全体は検証していない。

推奨変更: 期待 ID/headword の完全な sequence を比較する。重複 headword、日本語 rune 数、同位置・同長・同 norm の tie を fixture に含める。

### T-3. Builder test helper が `Scan` error を捨て、行 iteration error も確認しない

該当箇所: `internal/builder/builder_test.go:151-163`, `internal/builder/builder_test.go:319-339`

`checkMeta` の次のコードは `rows.Scan` の戻り値ではなく、外側の古い `err` を判定する。

```go
if rows.Scan(&k, &v); err != nil {
```

同 helper は loop 後の `rows.Err()` も見ない。ID 読み取り側も `rows.Scan(&id)` を無視している。production code では同種の error を適切に処理しているため、test code だけ品質が低い。

推奨変更: short declaration で Scan error を受け、すべての `Rows` loop 後に `rows.Err()` を確認する。

### T-4. Builder の置換・progress・compact test が設計要件を十分に検証しない

該当箇所: `internal/builder/builder_test.go:206-265`

`TestBuildForceReplacesOutput` は2回目の入力へ `removed` という entry を追加し、それが存在することを確認している。設計書 21.5 の「新しい入力から削除された entry が置換後 DB に残らない」のテストにはなっていない。

また progress は `t.Log` へ流すだけで固定行、10万行間隔、stderr-only、failure phase を assert せず、compact test は metadata の文字列だけを確認する。OS 別の原子的置換・置換失敗 test もない。

推奨変更: 旧入力だけにある entry を2回目から削り、不在を確認する。`bytes.Buffer` で progress 全体を比較し、publish 操作を注入可能にして失敗時の既存 DB 保持を検証する。対象 OS の CI job で実行する。

### T-5. TUI の重要な境界条件が未テストで、一部期待値も曖昧

該当箇所: `internal/tui/tui_test.go:181-219`

Ctrl-W test は正解の `"take "` と、現在の仕様では不正な `"take"` の両方を許可する。grapheme test は単一 rune の emoji だけで、結合文字、ZWJ、複数 cluster inputを扱わない。PageUp/PageDown の overlap、dual dictionary Tab、resize 後 scroll clamp、全角 display width、ellipsis、Enter/Esc no-op も未検証である。

推奨変更: 設計書 21.6 を table-driven checklist に落とし、今回の H-1、H-2、M-6、M-8 を再現する test を先に追加する。

### T-6. cancellation smoke test が別種の error でも pass する

該当箇所: `internal/sqlite/smoke_test.go:186-226`

実行中 query が `context.Canceled` 以外の error を返しても `t.Logf` するだけで test は成功する。設計書 3.2 と 21.3 が driver の context cancellation を正式実装前の必須 smoke test としているため、ここは強い assertion が必要である。

推奨変更: `errors.Is(err, context.Canceled)` でなければ test を失敗させる。cancel 後も同じ connection の既存 table を query し、単なる新規 in-memory connection の `Ping` で代用しない。

### T-7. benchmark と継続的 cross-build 検証が repository にない

Go source 内に `Benchmark...` はなく、`go test -bench . -benchmem ./...` は benchmark を1件も実行しなかった。設計書 17、21.4、25、28 の性能判断を再現できない。GitHub Actions 等もなく、6 target の `CGO_ENABLED=0` build は `make release` を手動実行したときだけ検査される。

推奨変更: ライセンス上安全な人工 DB の microbenchmark と、repository 外実データ benchmark の記録 template/script を用意する。CI で test、raceまたは定期 race、vet、tidy check、6 target cross-build を実行する。

## ドキュメント・配布

### D-1. README の option 一覧に `--debug` がない

該当箇所: `README.md:67-76`, `README.md:106-119`

logging section では `--debug` を説明しているが、option 一覧から欠けている。`cmd/ejquick/main.go:35` の help には存在する。

推奨変更: option 一覧へ追加し、help text と README を同じ定義から検証する golden test を検討する。

### D-2. release 用 zip helper の error 処理と入力正規化が弱い

該当箇所: `tools/mkzip/main.go:29-68`

出力 file の最終 `Close` error を無視し、途中失敗時に壊れた zip を残す。directory argument の末尾 separator の有無で archive root の計算が変わり得る一方、計算した `root` は未使用である。各 file を `os.ReadFile` で全量読み込みする必要もない。

推奨変更: clean 済み directory を基準に archive name を作り、`io.Copy` で stream する。zip writer と file の Close error を両方返し、失敗時は output を削除する。archive entry 名と同梱物を unit test する。

## 設計に沿っている点

次の点は適切に実装されており、維持すべきである。

| 項目 | 評価 |
|---|---|
| Binary 分離 | `ejquick` と `ejquick-build` が分離されている |
| Package 境界 | TUI/CLI -> Search Service -> Repository -> SQLite の依存方向になっている |
| DB schema | entries、external-content FTS5、metadata、B-tree index が設計と一致 |
| 検索方式 | 1-2 runes は prefix、3 runes以上は prefix 優先 + substring 補完 |
| 上限制御 | Service でも1から500を検証し、SQL LIMIT は bind parameter |
| 安全な FTS query | quote escape と bind parameterを使い、SQL文字列へ入力を連結していない |
| 正規化 | Eiji NFC + fold、Waei NFKC + fold を Builder/検索で共用 |
| 非同期 TUI | context cancellation と単調増加 request ID を併用し stale result を破棄 |
| DB 公開 | 同一 directory の一時 DB、read-only validation、file sync、rename、directory sync |
| Read-only 検索 | 検索 Repository は `mode=ro` で openし、write拒否 smoke test がある |
| 辞書データ分離 | `source/`、`*.sqlite3`、`tmp/` は ignoreされ、Git 管理対象に実データなし |
| Pure Go | `modernc.org/sqlite` を使用し、全配布 target が `CGO_ENABLED=0` で build可能 |
| License | root MIT License と release archiveへの notices 同梱処理がある |

## 実施した検証

辞書の購入データ内容は参照・引用せず、人工データと既存 unit testだけを使用した。

| 検証 | 結果 |
|---|---|
| `go test ./...` | 成功 |
| `go test -race ./...` | 成功 |
| `go test -shuffle=on -count=10 ./internal/...` | 成功 |
| `go vet ./...` | 指摘なし |
| `gofmt -d cmd internal tools` | 差分なし |
| `go test -coverprofile=tmp/review-coverage.out ./...` | internal は概ね47%から97%、command/tools は0% |
| `CGO_ENABLED=0` current-platform build | 両 binary 成功 |
| Linux/Windows/macOS x amd64/arm64 cross-build | 全12 binary 成功 |
| `go test -run '^$' -bench . -benchmem ./...` | 成功したが benchmark は0件 |
| `go mod tidy -diff` | 差分あり、exit code 1 |
| config `max_results = 0` | エラーにならず defaults として受理 |
| 未知 TOML section/key | エラーにならず受理 |
| `--format jsonl` かつ queryなし | usage errorにならず TUI 起動へ進む |
| 空文字 positional query | usage errorにならず TUI 起動へ進む |
| `HOME`/XDG config環境なし | config path errorを捨て、DB openまで進む |
| Builder の input/output同一 + `--force` | exit 0、人工 TXT が SQLite DB に置換された |

`staticcheck` と `govulncheck` は環境にインストールされていなかったため未実施である。実 Windows/macOS 上の runtime test、実辞書を使う性能測定、terminal を使う end-to-end TUI test も今回の検証範囲外である。

## 推奨修正順

1. C-1 の同一ファイル拒否を実装し、購入 TXT の破壊を防ぐ。
2. H-1、H-2 の TUI 編集処理を修正し、grapheme/cursor invariantsを test で固定する。
3. H-3、M-1からM-4の入力・設定検証を直し、command/config testを追加する。
4. M-5、M-6、M-8、M-9を直し、起動時検査と TUI の設計契約を満たす。
5. test の偽陽性要因と未検証項目を解消し、CI と benchmark の基盤を追加する。
6. dead code、依存関係、README、release helperを整理する。

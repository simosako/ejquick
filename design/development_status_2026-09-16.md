# EJQuick 開発状況整理

> **調査時点:** 2026-09-16  
> **対象ブランチ:** `main`  
> **対象コミット:** `733bcd1` (`v0.3.1`)  
> **文書の性質:** 現時点のリポジトリ、Git履歴、設計書、CI、GitHub Releasesを突き合わせたスナップショット。恒久的な仕様書ではなく、その後の変更に応じて更新または置き換える。

## 1. 結論

M1とM2は完成している。実際には、その先のBuilder GUI統合とLinux GUIパッケージ作成スクリプトまで実装済みである。

ただし、[`gui_design.md`](gui_design.md) が定義する「初期Linux GUIリリース完了」にはまだ到達していない。主な未完了点はGUI機能そのものではなく、以下の配布・検証工程である。

1. 完成GUIパッケージをCIで作成・検証する
2. GitHub ReleasesへGUIパッケージを公開する
3. 固定したglibc / Qt環境で互換性を確認する
4. KWin / Fcitx5およびMutter / IBusの実環境確認を記録する
5. 配布物の依存関係・ライセンス検査を完成させる

現在地は、**機能実装はかなり完成しているが、正式配布物としての検証と公開が未完成**という段階である。

## 2. コア機能の開発状況

[`initial_design.md`](initial_design.md) のPhase 1〜5は実質的に完了している。

| フェーズ | 状況 |
|---|---|
| Phase 1: DB Builder prototype | 完了 |
| Phase 2: CLI search | 完了 |
| Phase 3: Minimal TUI | 完了 |
| Phase 4: TUI refinement | 完了 |
| Phase 5: Distribution | 完了 |
| Phase 6: GUI | 主要機能は実装済み。配布・検証工程が未完了 |

実装済みの主要項目は次のとおり。

- CP932の厳密なdecode
- 辞書TXTのparseとSQLite変換
- B-treeによる前方一致検索
- FTS5 trigramによる部分一致検索
- 辞書種別ごとのUnicode正規化
- CLIのplain / JSON Lines出力
- 非同期TUI検索、context cancellation、stale result排除
- Linux / Windows / macOS、amd64 / arm64向けPure Go成果物
- Builderによる検査済みDBの原子的な公開・置換
- Builderのprocess間output lock
- 人工データによるtest、benchmark、cross-platform CI

`review/code_review.md`に記録された過去のコードレビュー指摘は、同文書の記載および後続コミット上では対応済みである。

## 3. GUIの開発状況

### 3.1 M1: GUI骨格

**完成。**

起点となるコミットは`7d6ce2d`（`Add GUI skeleton: ejquick-gui entry, gui build tag, Qt window (M1)`）。

主な内容:

- `ejquick-gui`バイナリ
- Qt Widgets + MIQT
- `gui` build tagによるQt依存の分離
- 基本window
- GUI用command-line parser
- 既存Pure Go frontendへのQt依存混入回避

### 3.2 M2: 検索GUI

**完成。**

検索GUIの統合は`3e091af`（`Implement the search GUI (M2)`）までに行われ、関連する主要実装は`5518f1e`（`Add Qt desktop search window`）に含まれる。

主な内容:

- 2-pane検索画面
- 英和・和英の切り替え
- 非同期incremental search
- request IDとcontext cancellation
- 結果一覧と選択entryの詳細表示
- input-centricなkeyboard操作
- IME preeditへの配慮
- clipboardへのentry全体copy
- `QSettings`によるwindow / splitter状態保存
- menu、About、Open Log
- config / DB errorのGUI表示
- Qt main threadへの安全なdispatch
- 非同期shutdown処理

### 3.3 M2以降に相当する実装

M3以降の正式なmilestone名は設計書や履歴で固定されていない。しかし、[`gui_design.md`](gui_design.md) で示された内部実装順序のうち、次も実装済みである。

#### Builder protocol / process supervisor

- Version付きJSON Lines machine protocol
- GUIとBuilderのproduct version照合
- stdinによる`start` / `cancel`
- graceful cancel
- 10秒後のhard kill
- Builder stderrの上限付き保持
- hard kill後の一時DB安全確認・cleanup
- output DBのwriter lock
- protocol / process errorのcategory分類

主なファイル:

- `internal/buildprotocol/protocol.go`
- `internal/gui/buildprocess/process.go`
- `cmd/ejquick-build/protocol.go`
- `internal/builder/output_lock_*.go`

#### Builder GUI

- 辞書種別選択
- 購入済みTXTのfile picker
- 出力DB path表示
- 既存DB置換の明示確認
- phase・件数の進捗表示
- cancel
- 成功・失敗表示
- 完成DBの再open
- applicationを再起動せず検索画面へ復帰

主なファイル:

- `internal/gui/builder_dialog_gui.go`

#### Linux desktop integration / package staging

- desktop entry template
- per-user登録・解除script
- Weston headless Wayland smoke test
- Qt / QPA / input context pluginを含む`tar.zst`作成script

主なファイル:

- `packaging/linux/package-gui-linux.sh`
- `packaging/linux/test-wayland.sh`
- `packaging/linux/install-desktop.sh`
- `packaging/linux/uninstall-desktop.sh`

## 4. 調査時点の検証状況

この調査では、VPSのresource制約を考慮し、Qt / MIQTの再buildを伴わない範囲を確認した。

| 検証 | 結果 |
|---|---|
| `go test ./...` | 成功 |
| `go vet ./...` | 成功 |
| `go mod tidy -diff` | 差分なし |
| `make test-desktop` | 成功 |
| Git working tree | clean |
| HEADに対するGitHub Actions CI | 全job成功 |

### GUI testに関する注意

調査に使用したLinux VPSはmemoryが6 GiBであり、Qt / MIQT周辺のcompileではmemory不足になる可能性が高い。そのため、VPS上の`make test-gui`は完了結果を得ておらず、ローカル検証成功とは扱わない。

一方、同じ対象コミット`733bcd1`に対するGitHub Actionsの`GUI (Ubuntu 24.04 / Qt 6.11.2)` jobは成功している。今後もQt / MIQTのbuild、GUI test、package作成は、原則として十分なmemoryを持つCIまたは専用build環境で行い、このVPSで不用意に実行しない。

## 5. 初期Linux GUIリリースの未完了事項

### 5.1 GUI成果物がGitHub Releasesへ公開されていない

調査時点のv0.3.1 release assetsは次のPure Go成果物だけである。

- Linux amd64 / arm64
- Windows amd64 / arm64
- macOS amd64 / arm64
- `checksums.txt`

`ejquick-gui_<version>_linux_amd64.tar.zst`は公開されていない。

`.github/workflows/release.yml`はGoReleaserを実行するが、`make package-gui-linux`を実行していない。したがってGUIは、READMEに記載されたlocal build / package手順は存在するものの、正式なrelease artifactにはなっていない。

### 5.2 CIが完成パッケージをtestしていない

現在のGUI CIが確認しているもの:

- Qt 6.11.2によるGUI build
- GUI packageのtest / vet
- Weston上でのbuild treeの`tmp/ejquick-gui`起動
- 人工DBによる検索
- result model、detail表示、shutdown

一方、D35 / D36が要求する次の確認は不足している。

- `package-gui-linux.sh`による実package作成
- 作成した`tar.zst`を展開しての起動
- build環境のQtへのfallback禁止確認
- package内QPA / input context pluginのload元確認
- package内の同梱`ejquick-build`をGUIからchild processとして起動するE2E test

現在の`test-wayland.sh`はbuild treeのbinaryを対象としており、完成配布物のtestではない。

### 5.3 glibc 2.34 baselineが実証されていない

D7-Lでは初期Linux GUIについて、x86_64、glibc 2.34以降、RHEL 9系相当のbuild環境を採用している。

現在のGUI CIはUbuntu 24.04上で動作している。より新しいglibc環境での成功だけではglibc 2.34互換の根拠にならないため、現状ではこのbaselineを正式保証できない。

### 5.4 実desktop / IME確認記録がない

D36ではrelease前に少なくとも次の実環境smoke testを要求している。

- KDE Plasma / KWin + Fcitx5
- GNOME / Mutter + IBus

確認対象:

- 日本語preedit
- candidate選択・確定・cancel
- Escape、矢印、PageUp / PageDown等との競合
- clipboard
- file dialog
- scale変更
- close / reopen

これらのrelease checklistや実施記録は、調査時点のrepositoryには存在しない。

### 5.5 Package dependency / license検査が未完成

`package-gui-linux.sh`にはruntime収集、RUNPATH、禁止辞書artifact、notice生成等の検査がある。ただしD29の完成条件に対しては次が不足している。

- 全`DT_NEEDED` dependencyの分類manifest
- 未分類SONAMEを失敗させる検査
- RHEL 9系、Ubuntu 22.04 / 24.04、Debian 12でのclean test
- Qt / Fcitx5 source versionとchecksumのrelease記録
- 必須license / copyright文書が見つからない場合の明確なfailure条件

### 5.6 GUI性能baselineの正式記録がない

D8 / D34では、release gateにはしないが次を継続計測するとしている。

- startup time
- idle RSS / PSS
- 検索後RSS / PSS
- search latency
- 圧縮時 / 展開時package size

初回GUI releaseの比較基準となる正式な記録は、調査時点ではrepositoryに存在しない。

## 6. 設計書の鮮度

設計判断自体は詳細であり、`gui_design.md`のD1〜D50はすべて決定済みである。大きな未決定事項は残っていない。

調査開始時には、文書のstatusや将来形の記述が実装状況に追いついていなかった。`gui_design.md`は本調査後に更新済みだが、`initial_design.md`には次の古い記述が残っている。

### `initial_design.md`

- StatusがDraftのまま
- GUIが`Future / undecided`のまま
- FTS5検証など、すでに完了した項目が今後の作業として残る

### `gui_design.md`

- 調査開始時はStatusがDraftのままで、最終節も「次に最小windowを作る」という実装前の記述だった
- 本調査後の2026-09-16更新で、M1 / M2、Builder GUI、package stagingの到達状況とrelease前の未完了事項を反映した

今後は文書内で次を区別すると管理しやすい。

1. 決定済み設計
2. 実装済み
3. Release前未完了
4. 将来候補

## 7. 初期Linux GUIリリースまで不要な作業

次は初期Linux GUIリリースの完了条件に含めない。

- Windows GUI
- macOS GUI
- Linux arm64 GUI
- X11 / XWaylandの正式対応
- AppImage / Flatpak / DEB / RPM
- system tray
- global shortcut
- launcher型UI
- 検索履歴
- 自動update
- 独自theme
- 独自fontのbundle
- font選択設定
- config editor
- application内蔵log viewer
- log rotation
- debounce
- spinner / loading animation
- 全一致件数取得
- 複数GUI process間のstate同期
- DB自動reload / file watcher
- 独自application icon
- 日本語GUI翻訳
- 3つ以上の辞書対応

### Open issueの位置付け

| Issue | 判断 |
|---|---|
| #1 3つ以上の辞書切り替え | 初期GUI release後。現在の2辞書固定設計を変更するため別途設計が必要 |
| #7 辞書表示のlocale対応 | 初期GUI release後。D41の英語GUI方針との調整が必要 |
| #8 README改善・日本語翻訳 | 購入先と制約の明確化はrelease前に有用。日本語翻訳はblockerではない |

### XCBとinput context pluginについて

XCBおよびCompose / IBus / Fcitx5 pluginは一見するとscopeが広いが、現在の設計では単純に不要とは言えない。

- D26でQtによるplatform / input methodの自動選択を採用している
- その結果としてD27 / D28でWayland + XCB、Compose + IBus + Fcitx5をbundleする

Packageをnative Wayland専用へ縮小する場合は、XCBだけを削除するのではなくD26〜D28を一体として再決定する必要がある。現時点では変更しない方が安全である。

## 8. 次に行うべきこと

### 優先度1: GUI release完了条件のchecklist化

D34〜D36を短い運用checklistへ落とし込む。この項目は本調査後、[`gui_release_checklist.md`](gui_release_checklist.md)の追加により完了した。

最低限含める項目:

- Package作成成功
- Package内Qtだけで起動
- Package内Builder起動
- Builder成功、cancel、置換、output busy
- Native Wayland起動
- Fcitx5 / IBus確認
- Desktop登録・解除
- Dependency / license検査
- 性能baseline記録
- GitHub Releasesへの公開

### 優先度2: GUI packageをCI / release成果物にする

- 固定Qt / Fcitx5環境で`make package-gui-linux`（CI run `35065872913`で初回成功）
- 作成した`tar.zst`を展開・静的検査（CI run `35065872913`で初回成功）
- 通常CIでarchive、SHA-256、build metadataをartifactとして保持（CI artifact作成・再download検証済み）
- 展開物に対するWayland smoke test
- Tag release時だけGitHub Releasesへ追加
- GUI packageをchecksum対象へ含める

### 優先度3: 完成packageのE2E test

- Package外Qtへのfallback禁止
- Wayland QPA pluginのload元確認
- Compose / IBus / Fcitx5 pluginのstartup
- 同梱Builderとのversion一致
- GUIからのBuilder成功
- Cancelとhard-kill cleanup
- Output busy
- 既存DB置換
- Build後のDB再open
- Desktop登録・解除

### 優先度4: 正式build baselineと配布検査

- glibc 2.34相当環境でのbuild
- `DT_NEEDED` manifest作成
- 未分類dependencyの検出
- 必須license fileの検査
- 対象clean environmentでの起動確認

### 優先度5: 実環境testと性能記録

- KWin + Fcitx5
- Mutter + IBus
- 100% / 200% scaling
- Startup time、RSS / PSS、search latency、package size
- 辞書内容やprivate pathを含めない実施記録

## 9. 現在地の要約

現時点の状況は次のように表現するのが正確である。

> コア、TUI、CLI、Builderは完成し、複数platform向けにrelease済み。Linux GUIは検索画面、Builder UI、desktop統合、package作成機能まで実装済み。現在は初回GUI配布に向けたpackage CI、互換性、実環境、dependency / license検証の段階にある。

次に新しいGUI機能を追加するのではなく、既存GUIを正式な配布物として完成させることを優先する。

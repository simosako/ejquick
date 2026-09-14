# EJQuick GUI 初期設計書

> Status: Draft  
> 最終更新: 2026-09-14  
> 対象: EJQuick のデスクトップ GUI フロントエンド

---

## 1. この文書の位置付け

本書は、既存の TUI / CLI に加えて開発する EJQuick GUI の設計判断を記録する。
辞書データ形式、SQLite schema、正規化、検索戦略などの共通仕様は
[`initial_design.md`](initial_design.md) を正とし、本書では GUI 固有の要件と、今後決定する事項を扱う。

GUI は既存 TUI の置き換えではなく、同じ辞書データを利用する追加のフロントエンドとする。
TUI / CLI と `ejquick-build` は引き続き提供する。

設計項目は次の状態で管理する。

- **決定済み**: 実装の前提とする
- **暫定**: prototype や計測結果を見て確定する
- **未決定**: 選択肢を比較してから決定する

---

## 2. 決定済み事項

### 2.1 GUI framework

- GUI library には **Qt 6** を使用する。
- View technology には **Qt Widgets** を使用する。
- 開発言語は既存 code と同じ **Go** を維持し、Qt 6 binding には **MIQT** を使用する。
- Qt Quick / QML は使用しない。
- 初期 baseline は **Qt 6.11.2、MIQT v0.14.0、Go 1.27.1** に固定する。
- Dependency の更新は自動追従させず、互換性を検証して明示的に行う。

Qt Widgets を選ぶ理由は、今回の検索中心の desktop UI に Qt Quick / QML の scene graph、animation、独自 visual design は過剰であり、起動速度、memory、配布 size を優先するためである。
Go + MIQT を選ぶ理由は、既存の検索、正規化、設定、logging package を同一 process から直接再利用し、開発言語を Go に統一するためである。

### 2.2 対応 OS

- Linux
- macOS
- Windows

ただし、採用する Qt 6 version が公式対応していない OS / CPU architecture は製品の対応対象に含めない。
「動作する可能性があること」と「公式対応構成であること」は区別し、EJQuick の配布物は原則として後者だけを対象とする。

初期 Linux release は **`x86_64`、glibc 2.34 以降、native Wayland** を対象とする。
Release binary は RHEL 9 系相当の glibc 2.34 環境で build し、それより新しい対応環境で実行できる構成を目指す。

### 2.3 品質上の優先事項

以下を優先する。

1. 起動が速い
2. 定常時の消費 memory が少ない
3. 入力から検索結果更新までが速い
4. 配布 package が必要以上に大きくない
5. OS の標準的な keyboard、IME、clipboard、accessibility と自然に連携する

「軽量」は感覚評価だけにせず、起動時間、memory、package size、検索 latency を継続的に計測する。
測定環境と結果の記録方法は prototype 後に確定する。D8どおり固定の合格値は設けない。

### 2.4 配布形態

- GUI は single binary を要件としない。
- Qt runtime library、Qt platform plugin、その他実行に必要な file の同梱を許容する。
- 不要な Qt module と plugin は同梱しない。
- 辞書 TXT および生成済み辞書 DB は配布物へ含めない。
- 初期 Linux release は self-contained な directory tree を `tar.zst` で圧縮して配布する。
- Directory tree には GUI、`ejquick-build`、必要な Qt runtime / plugin、`qt.conf`、desktop metadata、license notice を含める。

### 2.5 実装順序

最初の build target は **Linux / Wayland** とする。
Linux 版が検索 GUI として実用になるまでは、Windows / macOS 固有の実装と packaging を優先しない。
ただし、後から移植できない構造を避け、共通 code と OS 固有 code の境界は最初から明確にする。

### 2.6 Main window

- 通常の **2-pane window** を採用する。
- 上部に辞書選択用の `QComboBox` と検索欄、中央左に検索結果一覧、中央右に選択中 entry の見出し語と本文を配置する。
- `QComboBox` には `English–Japanese` / `Japanese–English` を表示し、利用できる辞書が1つだけなら現在値を表示したまま disabled にする。
- 検索欄の右端に、現在のmodel件数を示す`N shown` labelを配置する。
- 辞書を切り替えたらquery、結果、選択、list / detail scrollをclearし、検索欄へfocusを戻す。
- 左右 pane は横方向の splitter で分割し、利用者が幅を調整できるようにする。
- 左paneは`QListView` + `QStringListModel`、右paneはselectableな`QLabel` + read-onlyの`QPlainTextEdit`で構成する。
- 起動時から検索欄へfocusし、候補選択と詳細scrollをkeyboardで行っても検索欄のfocusを維持する。
- IME変換中はcustom key handlingを停止し、入力メソッドと`QLineEdit`の標準処理を優先する。
- 初回は960x640 logical pixels、最小720x480、左右比率1:2とし、geometry、maximized状態、splitter状態を`QSettings`へ保存する。
- Waylandではwindow位置を復元せずcompositorへ任せる。
- Launcher 型 popup や複数の表示 mode は初期版へ導入しない。

### 2.7 Application lifecycle

- Linux / Windows では、最後の main window を閉じたら application を終了する。
- macOS では、最後の window を閉じても application process と menu bar を残す。
- macOS で Dock icon、application activation、または menu action から main window を再表示できるようにする。
- System tray 常駐を共通機能として実装しない。
- 明示的な Quit 操作では OS にかかわらず検索を cancel し、DB と log を閉じて process を終了する。

### 2.8 性能計測の扱い

- 起動時間、RSS / PSS、検索 latency、package size は継続的に計測する。
- 測定値に固定の合格上限や regression 上限を設けず、release gate にはしない。
- 測定結果は、軽量性の改善、顕著な回帰の調査、実装方式の比較に利用する。
- 性能値だけを理由に release を機械的に止めず、影響と修正 cost を個別に判断する。

### 2.9 辞書 DB の作成導線

- 利用可能な辞書 DB がない場合、GUI から同じ release に含まれる `ejquick-build` を child process として起動できるようにする。
- GUI では辞書種別と購入済み TXT file を選択し、build の進捗と結果を表示する。
- GUI process に build logic を重複実装せず、`ejquick-build` と `internal/builder` を辞書 DB 作成処理の正とする。
- GUI と `ejquick-build` の product version が一致することを起動前に検査する。
- GUI 向けに version 付きの machine-readable progress 出力を Builder へ追加する。
- Cancel または GUI 終了時に child process を停止し、Builder が一時 DB を確実に cleanup できる protocol を設ける。
- Build 成功後は新しい DB を検査して read-only で開き、application を再起動せず検索画面へ移行する。

---

## 3. Qt 6 の対応 platform 調査

### 3.1 調査の前提

2026-09-13 時点の Qt 6.11 公式文書を調査した。
Qt 6.11.2 を初期 baseline として採用し、同じ minor version の公式対応状況を初期 target 検討の基準とする。
Qt の対応構成は minor / patch release 中にも変更される可能性があるため、使用 version の固定時と各 release 前に再確認する。

Qt 公式文書では、一覧にない構成でも動く可能性はあるが、Qt Project の公式対応外とされている。
EJQuick は、個別に対応を決定しない限り、そのような構成を配布対象外とする。

### 3.2 Desktop CPU architecture

| OS | Qt 6.11 の公式記載 | EJQuick での扱い |
|---|---|---|
| Linux | `x86_64`、一部の Debian / Ubuntu で `arm64` | `x86_64` は対象候補。`arm64` は Linux desktop の参照環境に制約があるため別途決定 |
| macOS | `x86_64`、`x86_64h`、`arm64` | Intel Mac の `x86_64` と Apple Silicon の `arm64` を対象候補とする |
| Windows | `x86_64`、Windows on ARM の `ARM64` | `x86_64` と `ARM64` を対象候補とする。`ARM64EC` は Qt 非対応 |

Linux desktop の `arm64` は Qt 6.11 で公式表に含まれるが、Qt は Raspberry Pi 5 / Ubuntu 24.04 を参照環境としており、より広い ARM desktop hardware への通常 support は今後の予定としている。
したがって Linux `arm64` は「Qt が一切非対応」ではないものの、対応 distribution、hardware、build 環境を定めずに一般対応とは表記しない。

次の desktop architecture は、Qt 6.11 の公式対応表にないため、現時点では対象外とする。

- 32-bit x86
- 32-bit ARM
- Linux の RISC-V、PowerPC、s390x など、公式表にない architecture
- Windows の ARM64EC
- 上表以外の desktop CPU architecture

### 3.3 暫定 target matrix

| 優先度 | OS / window system | CPU | 状態 |
|---|---|---|---|
| 1 | Linux / Wayland | `x86_64` | 初期 target。glibc 2.34 以降 |
| 2 | Windows | `x86_64` | Linux 版の後に対応 |
| 2 | macOS | `arm64` | Linux 版の後に対応 |
| 3 | macOS | `x86_64` | 配布方式と test 環境を確認して対応 |
| 3 | Windows on ARM | `ARM64` | Qt は対応するが現行 MIQT の support 表にないため、当面は対象外 |
| 3 | Linux / Wayland | `arm64` | 対応 distribution と hardware を限定して判断 |

Qt の対応範囲と、EJQuick project が継続的に build / test できる範囲は同一ではない。
採用した MIQT が対応しない構成は、Qt 自体が対応していても、EJQuick GUI の build target にはできない。
正式対応を表明する architecture には、CI build に加えて原則として実機または同等環境での起動、IME、描画、packaging の検証を要求する。

### 3.4 Linux / Wayland

- Qt application は Wayland compositor 上で `wayland` QPA plugin を選択して client として動作する。
- 初期配布物には、採用した構成で必要となる Wayland platform plugin を含める。
- compositor、graphics driver、EGL / OpenGL など、OS 側に要求する runtime 条件を release ごとに明記する。
- 初期配布物にはXCB QPA pluginもfallbackとして含めるが、X11 / XWaylandは当面best effortとし、正式対応には含めない。
- 初期の test 対象 compositor は少なくとも KDE Plasma / KWin と GNOME / Mutter を候補とし、実際の開発環境を確認して確定する。

#### Qt 6 + MIQT 日本語入力の確認結果

事前検証用の local sample `~/project/test-qt6-miqt/` により、次の構成で native Wayland 上の日本語入力と表示が動作することを確認済みである。

| 項目 | 確認済み構成 |
|---|---|
| OS / window system | Linux / Wayland |
| Qt | 6.11.2 / Qt Widgets |
| MIQT | v0.14.0 |
| Go | 1.27.1 |
| Input method | Fcitx5 5.1.22 + 日本語入力 engine |

確認済み項目:

- `QLineEdit` の変換中文字列、確定文字列、cursor 移動、削除
- `QTextEdit` の日本語入力と改行
- `QInputMethodEvent` の受信と基底 widget への event 引き渡し
- Go callback で取得した UTF-8 文字列の Qt widget への再表示
- Wayland QPA plugin と Fcitx5 Qt input context plugin の利用

この sample は EJQuick の source dependency にはせず、確認内容を GUI の smoke test へ移植する。
`QT_QPA_PLATFORM`、`QT_IM_MODULE` など入力環境へ影響する値は `QApplication` 生成前に確定している必要がある。
ただし、利用者の desktop session が適切に設定する値を EJQuick が常に上書きする設計にはせず、起動時の platform / input method 選択方針を別途決定する。

### 3.5 Linux binary compatibility

Qt 公式文書では、Qt 6.8 以降の配布 binary は glibc 2.28 以降、Qt 6.10 以降は glibc 2.34 以降を要求するとされている。
一方で、Qt を source build した場合の下限は build 環境に依存する。
また Linux `arm64` の公式 binary には、より新しい glibc を使う build 環境に関する注意がある。

したがって Linux の最低 distribution / glibc version は、Qt version と release build image を選んだ後に決定する。
開発 machine で起動することだけを互換性の根拠にせず、最古の対応環境で生成物を検証する。

---

## 4. 共通 backend との互換性要件

実装言語や process 構成にかかわらず、GUI は以下を満たす。

- `ejquick-build` が生成した既存の `eiwa.sqlite3` / `waei.sqlite3` を read-only で利用する。
- `schema_version`、`normalization_version`、`fts_version`、`dictionary_type` を検査する。
- TUI / CLI と同じ Auto 検索、正規化、上限件数、並び順を提供する。
- query が空なら DB query を発行しない。
- 検索は GUI thread を block しない。
- 新しい query が入力されたら不要な検索を cancel し、request ID により古い結果を破棄する。
- 辞書 DB は GUI から更新しない。
- GUI 固有の都合で DB schema を fork しない。

検索処理を C++ などへ移植する案を採用した場合は、Go 実装と同じ入力 DB / query に対して同じ結果を返す conformance test を用意する。
特に Unicode NFC / NFKC、full case folding、Unicode code point 単位の比較は、library ごとの差異が DB 互換性へ影響するため、代表例だけでなく自動生成 fixture でも検証する。

---

## 5. 論理 architecture

GUI と検索 backend は同一 Go process で動作し、MIQT が CGO 経由で Qt 6 の C++ API と共有 library を利用する。
責務は次のように分離する。

```text
+-------------------------------+
| Qt Widgets                    |
| window / input / list / detail|
+---------------+---------------+
                |
                | MIQT callback / signal
                v
+-------------------------------+
| Go GUI controller             |
| state / request ID / cancel   |
+---------------+---------------+
                |
                | Go interface
                v
+-------------------------------+
| Existing EJQuick search core  |
| normalize / search / ranking  |
+---------------+---------------+
                |
                v
       read-only SQLite DB
```

GUI widget から SQL を直接発行せず、Go の検索 interface を介する。
これにより、GUI test では fake search adapter を使い、検索 core の test では画面を起動しない構成にする。

Qt の GUI object は Qt main thread だけで生成・更新する。
検索は cancellable な Go goroutine で実行し、完了結果は MIQT の main-thread dispatch 機構を介して GUI controller へ戻す。
worker goroutine から widget の `SetText`、model 更新、破棄などを直接実行しない。

入力から表示までの基本 flow は次のとおりとする。

```text
QLineEdit textEdited
    ↓
GUI controller が request ID を更新し、旧 context を cancel
    ↓
goroutine で既存 search.Service.Search(ctx, query)
    ↓
MIQT main-thread dispatch
    ↓
request ID を検査し、current result だけを model / widget へ反映
```

---

## 6. 軽量性の設計原則

- 初期画面に不要な module、resource、font、image を読み込まない。
- Qt WebEngine、Multimedia、network service など、辞書検索に不要な module を導入しない。
- 検索結果を全件保持せず、既存仕様の最大 500 件を hard limit とする。
- 辞書本文を複製する model や cache を必要以上に持たない。
- background 常駐、system tray、global shortcut は、明示的に採用しない限り初期要件へ含めない。
- custom animation、透過、常時再描画を避ける。
- package には実際に必要な Qt library / plugin だけを含める。
- 起動 benchmark は GUI が操作可能になるまでを測り、DB open / schema 検査を含むかを記録する。
- memory は idle 時と検索後の両方で RSS / PSS を記録し、共有 library の影響を区別する。

---

## 7. Deployment と license の基本方針

### 7.1 Deployment

- Qt は dynamic link を基本候補とし、application と必要な Qt runtime / plugin を同じ package で配布する。
- Qt plugin path は `qt.conf` などで package 内へ固定し、開発 machine の Qt installation に依存させない。
- GUI binary は MIQT の要求に従い、`CGO_ENABLED=1`、C/C++ compiler、`pkg-config`、Qt 6 development files を使って `go build` する。
- Qt / MIQT / C/C++ toolchain の version を release build ごとに固定し、互換性のない compiler と Qt library を混在させない。
- Linux には専用の公式 `linuxdeployqt` tool がない。Go binary を起点に共有 library と plugin を収集・検査する再現可能な staging script を用意する。
- Windows では、MIQT と同じ Qt installation に含まれる `windeployqt` の利用を候補とする。
- macOS では、MIQT と同じ Qt installation に含まれる `macdeployqt` の利用を候補とする。
- 配布 package は、Qt を導入していない clean environment で検証する。

### 7.2 License

EJQuick 本体の MIT license とは別に、同梱する Qt module と third-party component の license 条件を満たす必要がある。
Qt Widgets と Qt Quick は open-source 利用時に LGPLv3 / GPLv2 の選択肢があるが、module ごとに条件が異なる可能性がある。

採用 module と link / 配布方式を確定した後、少なくとも以下を release check に含める。

- 採用した Qt module が LGPL で利用可能か
- dynamic link と library 差し替えに関する要件
- Qt と third-party component の copyright / license notice
- Qt 6.8 以降で提供される SBOM の確認
- Qt source code の入手方法に関する案内

本節は技術設計上の確認項目であり、法的助言ではない。

---

## 8. 最初の milestone

Linux / Wayland版の最初の公開releaseには、D34で決定した検索GUI、Builder UI、self-contained packageを含める。

- application window の起動と終了
- 英和 / 和英 DB の検出と切り替え
- 1 行の検索入力
- 非同期 incremental search
- 見出し語一覧と選択中 entry の本文表示
- keyboard、mouse、clipboard、英語 / 日本語 IME の基本操作
- DB 未作成、非互換 DB、検索失敗を説明する error 表示
- window size / splitter position など最小限の GUI 状態保存
- GUIからの辞書DB作成、進捗、cancel、failure recovery
- clean Linux environment 向けの再現可能な package 作成

System tray、global shortcut、自動update、高度なtheme、検索履歴、X11正式対応は、このmilestoneに含めない。

---

## 9. 次に決定する詳細設計と選択肢

以下は優先順に検討する。D1 と D2 は決定済みであり、採用理由と比較した選択肢を decision log として残す。

### D1. View technology

**状態: 決定済み（2026-09-13）**  
**決定: A. Qt Widgets**

#### A. Qt Widgets（採用）

- classic desktop UI として必要な部品が揃っている。
- model/view、IME、keyboard focus、accessibility を利用しやすい。
- QML engine と Qt Quick scene graph を必要とせず、軽量性の目標に合いやすい。
- 高度な animation や独自 visual design には Qt Quick より手間がかかる。

#### B. Qt Quick / QML

- 宣言的 UI、animation、responsive layout を実装しやすい。
- QML engine、Qt Quick、Quick Controls、graphics backend が必要になる。
- 起動時間、memory、配布 size は実測が必要で、今回の優先事項には不利になる可能性がある。

#### C. Widgets と Quick の併用

- 必要な箇所だけ QML を使える。
- bridge、focus、描画、deployment が複雑になる。
- 初期版には過剰であり、特別な UI 要件が出るまでは推奨しない。

Qt Quick / QML は今回の用途には過剰で、軽量性の目標にも合いにくいため不採用とした。

### D2. 実装言語と既存 Go 検索層の利用方法

**状態: 決定済み（2026-09-13）**  
**決定: B. Go + MIQT**

#### A. C++ / Qt + C++ で検索処理を実装

- Qt の主要かつ公式な開発経路であり、全 target の toolchain と deployment 情報が揃っている。
- 1 process で構成でき、Go runtime や IPC を不要にできる。
- SQLite C API、検索 SQL、ranking、設定読込、Unicode 正規化を C++ 側にも実装する必要がある。
- Go 版との挙動差を防ぐ conformance test と、DB metadata version の厳密な検査が必須になる。

#### B. Go + community 製 Qt 6 binding（採用）

- 同一 process から既存の Go package を直接呼び出せる。
- 現時点では MIQT が Qt 6.4+ の QtCore / QtGui / QtWidgets などを提供している。
- Go binding は Qt Company の公式 support 対象ではなく、Qt update 追従、API coverage、cross build、長期保守を project 側で検証する必要がある。
- MIQT の現行 support 表には Windows `ARM64` がないため、この案では将来 target が Qt 自体の対応範囲より狭くなる可能性がある。
- CGo と C++ toolchain は必要で、従来の Pure Go / cross compile 手順は GUI には適用できない。
- native Wayland、Fcitx5、日本語表示・入力は local sample で確認済みである。

#### C. C++ / Qt + Go `c-shared` library

- GUI は公式な C++ API を使い、検索・設定処理は既存 Go code を再利用できる。
- 1 process で動作し、resident backend process は不要になる。
- C ABI、memory ownership、threading、callback、cancel、error 表現を新たに設計する必要がある。
- Qt と Go の両 toolchain を組み合わせた Windows / macOS / Linux build を検証する必要がある。
- Go runtime と既存 SQLite driver も process に含むため、C++ 完結案より memory が増える可能性がある。

#### D. C++ / Qt + resident Go backend process

- 既存検索処理をほぼそのまま再利用でき、language 間 ABI を JSON 等の protocol に限定できる。
- backend crash の分離と単体 test がしやすい。
- process 起動、IPC、protocol versioning、終了処理が必要になる。
- 2 process 分の memory と起動 cost があり、軽量性では不利になりやすい。
- query ごとに CLI process を起動する方式は採用せず、採用するなら起動中は resident とする。

#### E. Python / PySide6

- Qt Company 公式の Python binding であり、prototype は速い。
- Python runtime と binding の同梱により、起動時間、memory、package size の目標に不利になりやすい。
- 今回の優先事項には合いにくいため、比較対象には含めるが推奨しない。

C++、Go shared library、別 process、Python の各案は採用しない。
MIQT 自体は community project であるため、MIQT version の固定、更新時の smoke test、必要な API が将来も利用できるかの監視を継続する。

### D3. Main window の形

**状態: 決定済み（2026-09-13）**  
**決定: A. 通常の 2-pane window**

#### A. 通常の 2-pane window（採用）

- 常設の通常 window とし、上部に辞書 selector と検索欄、中央左に結果一覧、中央右に本文を配置する。
- 中央は横方向の `QSplitter` とし、利用者が左右幅を変更できる。
- Status / errorは右paneのcontextual message（D15）、件数はheader右端（D17）へ表示し、常設status barは設けない。
- 既存 TUI の情報構造を保ち、long entry を読みやすい。
- `QLineEdit`、`QListView`、plain-text の詳細 widget という標準部品で実装できる。
- 起動後から検索欄へ focus を維持し、上下 key で候補選択、PageUp / PageDown で詳細を scroll できる。
- 1 window 内で情報が完結し、window activation や popup 固有処理が不要なので、最も単純で堅牢である。

概念図:

```text
+-------------------------------------------------------+
| [English–Japanese ▼] [ query........ ] [50 shown]     |
+-------------------+-----------------------------------+
| > headword        | Selected headword                 |
|   headword        |                                   |
|   headword        | Meaning and description...        |
|   ...             |                                   |
+-------------------+-----------------------------------+
```

#### B. Launcher 型 compact window

- Spotlight / command palette のように、検索欄と候補一覧を小さな縦長 window に表示する。
- 選択項目の本文は、同じ window の展開領域、別 popup、または Enter 後の表示へ切り替える。
- 検索だけを素早く始められ、screen space を取らない。
- long entry の閲覧性が低く、本文を表示する interaction を追加する必要がある。
- この形を活かすには global shortcut、focus loss 時の挙動、window positioning、常駐の検討が必要になり、D4 と強く結び付く。

概念図:

```text
+-------------------------------------------+
| [English–Japanese ▼] query............... |
+-------------------------------------------+
| > headword                                |
|   headword                                |
|   headword                                |
+-------------------------------------------+
| 選択中の本文を展開、または別表示          |
+-------------------------------------------+
```

#### C. 1-column master-detail window

- 上部に検索欄、その下に結果一覧、さらに下に選択中の本文を縦に並べる。
- narrow window でも成立し、実装は通常 window のまま保てる。
- 横幅を本文へ広く使える一方、結果一覧と本文が縦の高さを奪い合う。
- desktop の横長画面では空間効率が悪く、長い候補一覧と本文を同時に見にくい。

概念図:

```text
+-------------------------------------------+
| [English–Japanese ▼] query............... |
+-------------------------------------------+
| > headword                                |
|   headword                                |
|   headword                                |
+-------------------------------------------+
| Selected headword                         |
| Meaning and description...                |
+-------------------------------------------+
```

#### D. 通常 window と launcher の 2 mode

- A と B を切り替えて利用できる。
- 利用場面は広いが、window state、focus、検索状態の同期、shortcut、test 面積が増える。
- 初期版には過剰であり、launcher 需要を確認してから将来機能として検討する方が安全である。

検索結果と長い本文を同時に確認でき、標準的な Qt Widgets だけで単純に実装できるため A を採用した。
Launcher 型 UI は、将来 global shortcut や常駐機能への需要が確認された場合に改めて検討する。

### D4. Window を閉じた後の lifecycle

**状態: 決定済み（2026-09-13）**  
**決定: B. OS 標準の lifecycle に合わせる**

#### A. Main window を閉じたら全 OS で終了する

- Linux、Windows、macOS のすべてで main window の close を application 終了として扱う。
- Qt の `quitOnLastWindowClosed` を有効にし、検索 context、DB、log を閉じて終了する。
- background process と memory を残さず、挙動と test が最も単純である。
- 再利用時には毎回起動 cost が発生する。
- macOS では「window を閉じても application 自体は終了しない」という一般的な操作感と異なる。

#### B. OS 標準の lifecycle に合わせる（採用）

- Linux / Windows では最後の window を閉じたら終了する。
- macOS では window を閉じても process と menu bar を残し、Dock icon または File menu から再表示できるようにする。
- macOS 利用者の期待に合いやすい一方、OS ごとに挙動が異なり、macOS 固有 code と test が必要になる。
- macOS では window がない間も memory と open 中の DB connection が残る。DB を閉じて再表示時に開き直す案もあるが、状態管理が増える。

#### C. Close で隠して system tray / menu bar に常駐する

- 全 OS で close を非表示として扱い、tray icon または menu bar item から再表示する。
- 再表示は速く、将来 global shortcut と組み合わせやすい。
- 使用していない間も memory を消費する。
- Linux desktop、特に GNOME では tray icon の利用可否や表示方法が環境により異なる。
- 誤って終了できない状態を避けるため、明示的な Quit action と設定が必要になる。

#### D. User 設定で選べる

- 既定動作を A または B とし、close 時に終了するか常駐するかを設定できる。
- 両方の需要を満たせるが、設定 UI、状態遷移、OS ごとの test が初期段階から増える。
- 初期版で需要が確認できていない選択肢まで実装することになる。

Desktop ごとの一般的な操作感に合わせるため B を採用した。
Linux / Windows では close と Quit が同じ結果になり、macOS では Close Window と Quit EJQuick を区別する。

macOS で window がない間に main window、検索結果 model、DB connection を保持するか、必要な resource を解放して再表示時に復元するかは、D8 の idle memory 計測後に確定する。
初期実装では両方式へ変更できるよう、window lifecycle と検索 service lifecycle を直接結合しない。

### D5. 辞書 DB 未作成時の導線

**状態: 決定済み（2026-09-13）**  
**決定: B. GUI から `ejquick-build` process を起動**

#### A. Command の案内だけ表示

- 利用可能な DB がなければ、空の main window に理由、期待する DB path、実行すべき `ejquick-build` command を表示する。
- Command を clipboard へ copy する button と、TXT / DB の場所を選び直す設定導線だけを提供する。
- GUI 自体は source TXT を読み込まず、builder process も起動しない。
- 最小実装で検索 GUI と Builder の責務を完全に分離できる。
- Terminal 操作が必要となり、GUI application としては導入体験が不完全になる。

#### B. GUI から `ejquick-build` process を起動（採用）

- File picker で source TXT と辞書種別を選び、同じ release に含まれる `ejquick-build` を child process として起動する。
- GUI は build logic を持たず、既存 CLI と `internal/builder` をそのまま正とする。
- Builder の SQLite cache や一時 memory は child process 終了時に OS へ返るため、検索 GUI の定常 memory へ影響を残しにくい。
- Builder の失敗や crash を GUI process から分離できる。
- 現行の stderr は固定行の進捗を出すが、GUI 向けには version 付き JSON Lines などの machine-readable progress mode を追加する方が堅牢である。
- Cancel と application 終了時の扱いには、signal / process termination と一時 DB cleanup の cross-platform 設計が必要になる。
- `ejquick-build` の存在確認、実行 file path、GUI と Builder の version 一致も検査する。

想定 flow:

```text
No usable DB
    ↓
"Build Dictionary Database..."
    ↓
辞書種別と購入済みTXTを選択
    ↓
ejquick-buildをchild processとして開始
    ↓
進捗dialogへphase / lines / skippedを表示
    ↓
成功後にDBをopenして検索画面へ移行
```

#### C. `internal/builder` を GUI process に統合

- Go の `internal/builder.Run` を worker goroutine から直接呼び出す。
- Child process と protocol が不要で、成功時の `Stats` を型付きで受け取れる。
- 現行 API は `context.Context` と型付き progress event を持たないため、`RunContext` と callback interface へ拡張する必要がある。
- Build 中に GUI を終了した場合も一時 DB を確実に削除できる cancel point が必要になる。
- Builder の cache と一時 allocation が GUI process の peak RSS を増やし、build 完了後も Go runtime が一部 memory を保持する可能性がある。
- Builder panic や native library の問題が GUI 全体へ影響する。

#### D. 初回起動では案内だけ出し、独立した GUI Builder を後から追加

- 検索 GUI の初期 release は A とし、後続 release で `ejquick-build-gui` などの専用 application を提供する。
- Search application を小さく保ち、Builder 固有 UI を独立して設計できる。
- 配布 component と保守対象が増え、利用者は複数 application の役割を理解する必要がある。

検索 GUI の定常 memory へ Builder の大きな SQLite cache を残さず、既存の build logic を再利用し、失敗を GUI process から分離できるため B を採用した。
Machine-readable progress の event schema、cancel protocol、一時DB cleanup、既存 DB を置換する確認 UI は、D21 / D22 / D23で決定したとおりとする。

### D6. 最初の Linux package

**状態: 決定済み（2026-09-13）**  
**決定: A. Self-contained directory + `tar.zst`**

どの案でも、GUI binary、`ejquick-build`、必要な Qt shared library / plugin、`qt.conf`、license noticeを同じversionの成果物として配布する。
辞書TXTと生成済みDBは含めない。

#### A. Self-contained directory + `tar.zst`（採用）

- Relocatableなdirectory treeを作り、`tar.zst`で圧縮して配布する。
- GUIは同じtree内の`ejquick-build`を絶対pathへ解決して起動する。
- ELFのRPATHを`$ORIGIN`基準に設定し、`qt.conf`でplugin pathをpackage内に固定する。
- Wayland / XCB QPA pluginを同梱し、必要なsystem Wayland / X11 / graphics libraryは要件として明記する。
- Staging後に`ldd`、plugin debug log、clean environmentで不足dependencyを検査する。
- 内容とdependencyが明確で、AppImageやsandbox固有の問題を避けられる。
- 利用者が展開場所を選び、desktop fileやmenu登録を手動で行う必要がある。

想定構成:

```text
EJQuick/
├── bin/
│   ├── ejquick-gui
│   ├── ejquick-build
│   └── qt.conf
├── lib/
│   └── libQt6*.so.6
├── plugins/
│   ├── platforms/
│   └── platforminputcontexts/
├── share/
│   └── applications/
└── licenses/
```

`share/applications/`にはD30の登録scriptが利用するdesktop entry templateを置き、install済みentry自体はpackageへ含めない。

#### B. AppImage

- GUIと`ejquick-build`を1つのAppDirへ配置し、AppImage 1 fileとして配布する。
- Download後に実行権限を付けるだけで起動でき、desktop integration toolとも連携できる。
- Qt runtimeを圧縮状態で保持でき、利用者から見えるfile数が少ない。
- Qt公式deploymentの外側にcommunity packaging toolが必要になる。
- AppImage runtime / FUSE、native Wayland、system graphics library、input method pluginの組み合わせを検証する必要がある。
- Child BuilderのpathはmountされたAppDir内から解決し、GUI終了後もbuild完了までmountが維持されることを保証する必要がある。

#### C. Flatpak

- Wayland permissionとdesktop integrationをmanifestで管理できる。
- Qtを含むruntimeを共有でき、application package自体は小さくできる。
- Sandbox内のdefault data pathと既存CLIが使うhost側pathが異なる可能性がある。
- 購入済みTXTのfile picker portal、生成DBの保存先、既存DBへのaccess permissionを設計する必要がある。
- TUI / CLIと同じDBを自然に共有するには追加のfilesystem permissionまたはexport導線が必要になる。

#### D. DEB / RPM

- Distributionのpackage manager、desktop menu、uninstallへ自然に統合できる。
- Private Qt runtimeを`/opt/ejquick`等へ同梱するか、system Qtへ依存するかをdistributionごとに決める必要がある。
- DEB / RPMの両方を提供するとbuild、dependency metadata、署名、repository運用の範囲が広がる。
- `tar.zst`より利用者体験は良いが、最初のWayland prototypeとreleaseには作業量が多い。

#### E. `tar.zst`とAppImageを同時に提供

- Aをdebug可能な基本成果物、Bを一般利用者向け成果物として両方配布する。
- AppImage固有問題が起きても`tar.zst`をfallbackにできる。
- Release artifact、test、checksum、support対象が初期段階から増える。

Single binaryを要件とせず、最初のreleaseではdependencyの透明性、debugのしやすさ、packaging固有問題の少なさを優先するためAを採用した。
AppImage、Flatpak、native packageは、基本directory packageの動作が安定した後に追加を検討する。

### D7. Qt minor version policy

**状態: 決定済み（2026-09-13）**  
**決定: A. 確認済みの組み合わせを初期 baseline として固定**

Qt と MIQT の組み合わせは build 再現性、platform support、CGO toolchain、配布 library の ABI に影響する。
どの案でも、各 release artifact は実際に使用した Qt / MIQT / compiler の正確な version を記録する。

#### A. 確認済みの組み合わせを初期 baseline として固定（採用）

- 最初の baseline を Qt 6.11.2、MIQT v0.14.0、Go 1.27.1 とする。
- `go.mod` / `go.sum` で MIQT を固定し、release build image でも Qt と compiler を固定する。
- Qt patch / minor または MIQT を更新する場合は、Wayland、IME、main-thread dispatch、build、package の smoke test を実行し、baseline を明示的に更新する。
- 事前 sample で動作確認した組み合わせから開始でき、再現性が最も高い。
- Security fix や platform fix を取り込むには、定期的な update 判断が必要になる。

#### B. Qt 6.11 minor 内の最新 patchを追跡

- Qt は 6.11.x の最新 patch、MIQT は互換性を確認した最新 release を使用する。
- Fix を早く取り込める一方、同じ EJQuick source でも build 時期によって dependency が変わる。
- Release build では最終的な exact version を記録するが、開発環境間の差異が生じやすい。

#### C. 開発時点の最新 stable Qt 6 minor を追跡

- 現時点なら Qt 6.11 系から開始し、新しい stable minor が出るたびに更新する。
- 新しい platform fix と通常 support を利用しやすい。
- Minor update ごとに MIQT compatibility、起動性能、表示、IME、packaging の回帰確認が必要になる。
- 最も更新頻度が高く、release の再現性を保つための build image 管理が重要になる。

#### D. Qt 6.8 LTS 系へ固定

- API と環境を長期間固定しやすい。
- Qt 6.8 LTS の追加 patch への即時 access は commercial customer に限定されるため、open-source 利用時の security / bug fix 方針を別途決める必要がある。
- 確認済み sample の Qt 6.11.2 から version を下げた再検証が必要になる。

#### E. Distribution 提供の Qt 6 を使用

- System package と統合しやすい。
- Distribution ごとに version が異なり、Qt runtime 同梱方針と再現可能な build に合いにくい。
- 開発時には利用できるが、公式 release artifact の build 方針としては採用しにくい。

事前 sample で native Wayland と日本語 IME を確認できており、問題発生時の変数を減らせるため A を採用した。
更新時は version だけを変更せず、Qt / MIQT / Go / compiler の組み合わせを一つの baseline として更新履歴へ記録する。

### D7-L. 初期 Linux build / runtime baseline

**状態: 決定済み（2026-09-13）**  
**決定: A. `x86_64` / glibc 2.34 以降**

Qt 6.11.2 の Linux binary は glibc 2.34 以降を基本要件とする。
初期 target は native Wayland とし、X11 / XWayland を動作保証へ含めない。

#### A. `x86_64` / glibc 2.34 以降（採用）

- 最初の release architecture を `x86_64` とする。
- Runtime baseline を glibc 2.34 以降とし、release binary も glibc 2.34 環境で build する。
- RHEL 9 系相当の build image を候補とし、Qt 6.11.2、compiler、Wayland development library を image 内で固定する。
- Ubuntu 22.04 / 24.04、Debian 12、RHEL 9 系など、Qt の対応表と baseline を満たす環境から代表的なものを clean test に使う。
- 広い互換性を期待できる一方、古い build imageへQt 6.11.2とGo 1.27.1を再現可能に導入する作業が必要になる。

#### B. `x86_64` / Ubuntu 22.04 以降

- 最初の正式対応を Ubuntu 22.04以降のnative Waylandに限定する。
- Ubuntu 22.04のglibc 2.35とGCC 11をbaselineにbuild / testする。
- Qt 6.11の公式対応構成に含まれ、Aより検証範囲を狭くできる。
- Debian、Fedora、RHEL、Arch Linux等では動く可能性があっても、初期releaseではbest effortとする。

#### C. `x86_64` / Ubuntu 24.04 以降

- 最初の正式対応を Ubuntu 24.04以降のnative Waylandに限定する。
- 新しいcompiler、Wayland stack、glibc 2.39を前提にでき、build環境を用意しやすい。
- Ubuntu 22.04やglibc 2.34〜2.38のdistributionを対象外にするため、利用可能範囲が狭い。

#### D. `x86_64` / rolling distributionを開発基準にする

- 確認済みsampleと同様に、Arch Linux等のcurrent packageを使って開発する。
- Local開発は最も簡単だが、更新によりQtやsystem libraryが変化し、最低runtime環境と再現可能なrelease artifactを定義しにくい。
- Prototype限定なら利用できるが、正式release baselineには推奨しない。

#### E. 初期から`x86_64`と`arm64`を提供

- Ubuntu 24.04を共通の中心環境として、architecture別にbuild / packageする。
- MIQTはLinux `arm64`をsupport表に含め、Qt 6.11も一部Linux distributionで`arm64`を公式対応に含める。
- QtのLinux ARM desktop参照hardwareが限定的であり、cross buildだけでなくARM64実機でWayland、IME、描画を検証する必要がある。
- 初期段階のCI、dependency staging、実機testの範囲が大きくなる。

Qt 6.11.2が要求するglibc baselineを維持しながら、特定の一つのdistributionだけに配布対象を狭めないためAを採用した。
正式な動作確認distributionとcompositorの組み合わせは、release環境を構築した後に記録する。

### D8. 軽量性の合格基準

**状態: 決定済み（2026-09-13）**  
**決定: D. 性能値を release gate にしない**

どの案でも、reference machine の CPU、memory、storage、distribution、compositor、Qt / MIQT version を記録し、同じ条件で次を測定する。

- process 開始から、検索欄が表示され入力を受け付けるまでの startup time
- window 表示中に操作せず安定した時点の idle RSS / PSS
- macOS で全 window を閉じた状態の RSS / PSS
- 実データ相当 DB で、確定した入力から current request の結果表示までの p50 / p95 latency
- 100 回以上検索した後の RSS / PSS と、idle 時からの増加量
- Qt runtime / plugin を含む package の圧縮時 / 展開時 size

OS page cache の有無を区別し、warm start と cold start を混在させない。
辞書データを含む測定結果は repository や CI artifact へ公開せず、公開 benchmark には人工 DB を使う。

#### A. 実装前に暫定の絶対上限を決める（積極的な軽量化）

- 例として Linux reference machine で、warm startup p95 250 ms、cold startup p95 750 ms、表示中 idle PSS 60 MiB、検索 latency p95 50 ms、圧縮 package 40 MiB を初期上限とする。
- 実装開始時から明確な制約として扱える。
- Qt / driver / hardware の実測前なので、数値が過度に厳しい、または緩い可能性がある。
- 上限変更には測定結果を添えた設計変更を必要とする。

#### B. 採用構成の小規模 prototype を同一環境で計測してから決める

- Qt Widgets + Go + MIQT の最小 window と、既存検索 service を接続した 2-pane window の2段階を測定する。
- 各指標を原則 30 回以上測定し、外れ値を隠さず p50 / p95 として記録する。
- 2段階目の測定後、初期 release の絶対上限と、以後の許容 regression 率を本書へ追記する。
- 現実的な基準を設定できる一方、基準確定までは prototype を feature 実装とみなさない運用が必要になる。
- 数値を決めないまま本実装へ進むことは、この案に含めない。

#### C. 前 release 比の regression だけを管理する

- 初版の測定値を baseline とし、例えば p95 startup、idle PSS、package size の 10% を超える悪化を release blocker とする。
- Hardware 差の影響を比較的受けにくく、継続運用は容易である。
- 初版自体が重い場合を検出できず、利用者に対して「軽量」の具体的な目安を示せない。

#### D. 性能値を release gate にしない（採用）

- 測定結果は記録するが、合否には使用せず、体感上問題が出た場合だけ最適化する。
- 開発速度と実用上の判断を優先できる。
- 軽量性という優先要件を固定値で保証するものではないため、計測自体を省略したり、明らかな回帰を理由なく放置したりしない。

### D9. 辞書切り替え control

**状態: 決定済み（2026-09-13）**  
**決定: A. `QComboBox`**

英和と和英を自動判定せず、現在の辞書を常に画面上で確認できるようにする。
利用できない辞書の表示とbuild導線はD39 / D45、切り替え時の検索stateはD10の決定に従う。

#### A. `QComboBox`（採用）

- Main window上部の検索欄左側へ、`English–Japanese` / `Japanese–English`のdrop-downを配置する。
- OS theme、keyboard操作、screen readerへ標準的に対応できる。
- 将来辞書種別が増えてもlayoutを変更せず対応できる。
- 現在値は明確だが、他方の辞書へ切り替えるにはdrop-downを開いて選ぶ2段階の操作になる。
- 利用可能な辞書が1つだけならcontrolをdisabledにし、現在の辞書名は表示し続ける。

```text
+-------------------------------------------------------+
| [English–Japanese ▼] [ query....................... ] |
+-------------------------------------------------------+
```

#### B. 2個のexclusive button

- `English–Japanese`と`Japanese–English`の`QToolButton`または`QRadioButton`を`QButtonGroup`で排他的にする。
- 1 clickで切り替えられ、両方の選択肢と現在値を常に確認できる。
- Combo boxより横幅を使うが、辞書が2種類だけなら操作が最も直接的である。
- Segmented control風の独自stylesheetは使わず、native styleで表示しないとplatformごとの差異と保守が増える。

```text
+-------------------------------------------------------+
| [English–Japanese●] [Japanese–English○] [ query... ] |
+-------------------------------------------------------+
```

#### C. `QTabBar`

- 検索領域の上に`English–Japanese` / `Japanese–English` tabを表示する。
- 標準widgetで1 click切り替えでき、keyboardでtab間を移動できる。
- Tabごとに別のqueryや選択状態を保持するUIに見えやすく、同じ検索画面のbackendだけを切り替える設計とは意味が合わない可能性がある。
- Window上端の縦方向を1行多く使用する。

```text
+-------------------------------------------------------+
| [ English–Japanese ] [ Japanese–English ]            |
| [ query............................................. ] |
+-------------------------------------------------------+
```

#### D. Menu / keyboard shortcutだけで切り替える

- Main windowには現在の辞書名だけをlabel表示し、切り替えはmenu actionとshortcutで行う。
- Headerを最もcompactにできる。
- Mouse利用者が切り替え方法を発見しにくく、現在値labelとmenuの状態同期も必要になる。
- 初回利用時の分かりやすさより画面上の省スペースを優先する案である。

標準的な操作性とaccessibilityを維持し、headerの横幅を抑えられるためAを採用した。
初期版では辞書が2種類だけだが、将来追加されてもcontrolの構造を変更せずに対応できる。

### D10. 辞書切り替え時の検索 state

**状態: 決定済み（2026-09-13）**  
**決定: A. Queryと検索stateをすべてclear**

どの案でも、辞書を切り替える前にrequest IDを更新して実行中検索をcancelし、切り替え前の遅延結果を新しい辞書の画面へ反映しない。
検索errorと一時的なstatus messageも切り替え時にclearする。

#### A. Queryと検索stateをすべてclear（採用）

- `QLineEdit`を空にし、結果一覧、選択、list scroll、詳細本文、detail scrollをclearする。
- 切り替え直後はDB検索を行わず、検索欄へfocusを戻す。
- 英和と和英では通常queryの言語が異なるため、意図しない検索を避けられる。
- 既存TUIの辞書切り替え仕様と一致し、controllerのstateが最も単純になる。
- 同じ文字列を両辞書で試したい場合は再入力またはpasteが必要になる。

#### B. Queryを保持し、切り替え先ですぐ再検索

- `QLineEdit`の文字列を維持し、選択とscrollをclearして新しい辞書で検索を開始する。
- 同じqueryを両辞書で比較しやすい。
- 英語queryを和英へ、日本語queryを英和へ持ち越して、ほぼ常に0件となる可能性がある。
- Switch操作のたびにDB queryが発生する。

#### C. 辞書ごとに独立した検索stateを保持

- 英和・和英それぞれにquery、結果、選択、list / detail scrollを保持し、切り替えると以前の状態を復元する。
- 複数の検索を往復して参照しやすい。
- `QComboBox`からは独立stateの存在が見えにくく、以前のqueryが突然戻るように感じる可能性がある。
- State、memory、stale result管理、test caseが増える。

#### D. Queryを保持するが自動検索しない

- `QLineEdit`は維持するが、結果と詳細をclearし、次にqueryが編集されるかEnterが押されるまで検索しない。
- 不要なDB queryを避けながら文字列を持ち越せる。
- Incremental searchなのに表示中queryと結果が対応しない一時状態が生まれ、利用者にとって分かりにくい。

英和と和英では通常queryの言語が異なり、既存TUIとも同じ予測可能な動作になるためAを採用した。
Clearによる`QLineEdit`のsignalから意図せず空query検索を開始しないよう、辞書切り替え処理中はsignalをblockするか、controller側で一つのstate transitionとして処理する。

### D11. 検索結果listと詳細paneのwidget構成

**状態: 決定済み（2026-09-13）**  
**決定: A. `QListView` + `QStringListModel` + plain-text詳細**

全案で、完全な`[]search.Entry`はGo controllerがcurrent resultとして保持する。
選択変更時はrow indexからGoのentryを参照し、追加のDB queryを行わない。
本文はHTMLとして解釈せず、plain textとして表示して選択・copyできるようにする。

#### A. `QListView` + `QStringListModel` + plain-text詳細（採用）

- 左paneは`QListView`、modelは`QStringListModel`とし、Qt modelには表示用headwordだけを渡す。
- Entry IDとbodyはGoの`[]search.Entry`だけに保持し、Qt側へ本文を重複保存しない。
- 新しい検索結果ではheadwordのstring listを一括置換し、選択中IDが残っていれば該当row、なければ先頭rowを選択する。
- 右paneは見出し語用のselectableな`QLabel`と、read-onlyの`QPlainTextEdit`で構成する。
- Qt標準model/viewを使いながら、Go virtual callbackを多用せず単純に実装できる。
- HeadwordはGoとQtの両方に一時的に保持されるが、最大500件なのでmemoryへの影響は限定的である。

```text
+-------------------+-----------------------------------+
| QListView         | QLabel: selected headword         |
| > headword        |                                   |
|   headword        | QPlainTextEdit (read-only)        |
|   headword        | meaning and description...        |
+-------------------+-----------------------------------+
```

#### B. `QListWidget` + plain-text詳細

- 左paneはitem-based convenience widgetの`QListWidget`を使う。
- Resultごとに`QListWidgetItem`を生成し、headwordを設定する。ID / bodyはGo側を正とする。
- Model classを扱わずに実装でき、初期codeは最も短い。
- 検索ごとにitem objectを破棄・再生成するため、連続するincremental searchではAよりallocationとsignal処理が増える。
- Default 50件、最大500件なら実用上問題ない可能性が高いが、更新costはprototypeで確認する。
- 右paneはAと同じ`QLabel` + `QPlainTextEdit`とする。

#### C. `QListView` + custom `QAbstractListModel`

- Custom Go modelが`[]search.Entry`を直接参照し、DisplayRoleでheadwordを返す。
- Qt用のheadword listを別に作らず、role追加でID等も提供できる。
- QtからGoへのvirtual method callbackが描画中に頻繁に発生し、CGO boundaryのcostとthreadingを検証する必要がある。
- Model reset、index lifetime、signal、Go object lifetimeをMIQT経由で正しく管理する必要があり、今回の最大500件には複雑すぎる可能性がある。
- 右paneはAと同じ`QLabel` + `QPlainTextEdit`とする。

#### D. `QTableView`でbody previewも表示

- 左paneにheadwordとbody先頭の2 columnを表示し、右paneには全文を表示する。
- 候補を選ぶ前に意味の一部を比較できる。
- 同じ本文情報が左右に重複し、row height、column width、省略表示の設計が増える。
- Headwordを素早く走査する辞書UIとしては情報量が多く、compactさを損なう。

Qt標準model/viewを単純に利用しながら、Qt側へ全entryの本文を複製しないためAを採用した。

実装上の対応関係は次のとおりとする。

```text
Go controller:
    entries[0] = {ID, Headword, Body}
    entries[1] = {ID, Headword, Body}

QStringListModel:
    row 0 = entries[0].Headword
    row 1 = entries[1].Headword

QListView selected row = 1
    ↓
entries[1]を参照
    ↓
QLabel.SetText(entries[1].Headword)
QPlainTextEdit.SetPlainText(entries[1].Body)
```

`Entry.ID`は元TXTの物理行番号で欠番を許容するため、slice indexとして使用しない。
選択変更時は`QModelIndex.Row()`を検証して`entries[row]`を参照する。
新しい検索結果で選択中IDを維持するときは最大500件の`entries`を線形探索し、同じIDのrowを探す。

Qt側には全resultのheadwordに加えて、現在表示中entryのheadwordとbodyがwidget内部の`QString` / text documentとしてcopyされる。
全resultのbodyをQt modelへ保持することはしない。

初期設定:

- `QListView`: single selection、row単位、編集不可、1行表示、末尾elide
- `QStringListModel`: 検索結果の表示順どおりにheadwordを一括設定
- `QLabel`: word wrap有効、mouse / keyboardで文字列選択可能
- `QPlainTextEdit`: read-only、plain text、widget幅でwrap、undo / redo不要
- Custom delegateとrowごとのchild widgetは初期版では使用しない

### D12. Keyboard focus model

**状態: 決定済み（2026-09-13）**  
**決定: A. 検索欄focusを維持するinput-centric方式**

日本語IMEの変換中は、矢印、Enter、Space等をEJQuickのshortcutとして奪わないことを全案の必須条件とする。
`QLineEdit`の標準editing shortcutとinput method eventを優先し、custom key handlingは変換中文字列がない場合だけ適用する。

#### A. 検索欄focusを維持するinput-centric方式（採用）

- 起動時から`QLineEdit`へfocusし、検索中も候補を選択してもfocusを移さない。
- IME変換中でなければ、Up / Downで`QListView`のcurrent row、PageUp / PageDownで詳細paneのscroll位置を変更する。
- 選択変更は`QItemSelectionModel`を介し、focusのないlistにも選択行を表示する。
- 続けて文字を入力するだけでqueryを編集でき、既存TUIやfzfに近い高速な操作になる。
- `QLineEdit`のkey eventを条件付きで処理するevent filterが必要で、preedit状態の追跡を誤るとIMEを壊す可能性がある。

#### B. Qt標準のfocus traversalだけを使う

- 検索欄、結果list、詳細pane、辞書comboの間をTab / Shift+Tabで移動する。
- Result listにfocusがある時だけUp / Down、詳細paneにfocusがある時だけPageUp / PageDownを処理する。
- Custom key event処理が少なく、IME、accessibility、platform標準動作との衝突が最も少ない。
- 検索を修正するたびに検索欄へfocusを戻す操作が必要で、incremental searchの操作速度が落ちる。

#### C. Downで結果listへ移るhybrid方式

- 通常は検索欄へfocusし、IME変換中でないDownまたは明示shortcutで結果listへfocusを移す。
- Listでは標準のUp / Downを使い、検索へ戻るには`Ctrl+L`等のshortcutまたはmouseを使う。
- 検索欄のcustom handlingをfocus移動の1操作に限定できる。
- Focus移動後に文字入力してもqueryへ入らず、現在focusを意識する必要がある。
- Detail scrollのためのfocusまたはshortcutは別途必要になる。

#### D. 結果更新時にlistへ自動focus

- 最初の検索結果が返った時点で`QListView`へfocusを移し、標準list操作で選択する。
- 候補選択は直接的だが、続けてqueryを入力する前に毎回検索欄へfocusを戻す必要がある。
- 高速入力中に非同期結果が返るとfocusが突然移動し、その後の文字がqueryへ入らない危険があるため推奨しない。

Incremental search中にfocusを移さず入力と候補選択を往復でき、既存TUIに近い操作感になるためAを採用した。

実装上の制約:

- `QLineEdit`が受け取る`QInputMethodEvent`からpreedit stringの有無を追跡し、基底classの処理を必ず呼び出す。
- Preedit中、またはinput methodが処理すべきeventは、候補移動やdetail scrollへ転用しない。
- Preedit中に`QLineEdit.SetText`を呼ばず、変換中文字列とcursorを壊さない。
- Custom navigationは検索欄のevent filterに限定し、application全体のplain key shortcutとして登録しない。
- `QItemSelectionModel`でcurrent rowを変更しても`QListView.SetFocus`は呼ばない。
- MouseまたはTab focus traversalでlistや詳細paneへfocusした場合は、各widgetの標準選択・copy・scroll操作を妨げない。
- Wayland / Fcitx5のIME smoke testに、preedit中のUp / Down、PageUp / PageDown、Enter、Escapeを含める。

### D13. Exact keyboard shortcut

**状態: 決定済み（2026-09-13）**  
**決定: A. Input-centric固定keymap**

全案で`QLineEdit`の標準text editing、selection、copy / cut / paste、undo / redoを変更しない。
GUIでは`Ctrl+C` / `Cmd+C`をcopy、Tab / Shift+Tabをfocus traversalとして維持するため、TUIの終了・辞書切り替えkeyをそのまま使用しない。
QuitはQtのplatform標準`QKeySequence::Quit`を使う。

#### A. Input-centric固定keymap（採用）

| Key | Action |
|---|---|
| Up / `Ctrl+P` | 前の検索結果を選択 |
| Down / `Ctrl+N` | 次の検索結果を選択 |
| PageUp / PageDown | 詳細本文を1 page scroll |
| `Ctrl+L` / `Cmd+L` | 検索欄へfocusし、query全体を選択 |
| Escape | Queryと検索stateをclear |
| `Ctrl+Tab` | 利用可能な辞書が2つある場合に切り替え |
| `Ctrl+Shift+C` / `Cmd+Shift+C` | 選択entry全体をcopy |
| `Ctrl+Q` / `Cmd+Q` | Applicationを終了 |

- Up / Down、`Ctrl+P` / `Ctrl+N`、PageUp / PageDown、`Ctrl+L` / `Cmd+L`、Escape、`Ctrl+Tab`、entry全体copy shortcutはIME preedit中にEJQuick側で処理しない。
- 候補移動は先頭・末尾で停止し、wrapしない。
- `Ctrl+Tab`による辞書切り替えもD10に従ってqueryと検索stateをclearする。
- Mouseを使わず、検索欄focusを維持したまま主要操作を実行できる。
- `Ctrl+P` / `Ctrl+N`など、GUI applicationとしては一般的でないaliasが増える。

#### B. Platform標準を優先した最小keymap

| Key | Action |
|---|---|
| Up / Down | 前後の検索結果を選択 |
| PageUp / PageDown | 詳細本文を1 page scroll |
| `Ctrl+L` / `Cmd+L` | 検索欄へfocusし、query全体を選択 |
| `Ctrl+Q` / `Cmd+Q` | Applicationを終了 |

- 辞書は`QComboBox`、query clearはclear buttonまたは標準editingで操作する。
- Shortcut数と衝突可能性が最も少なく、helpを読まなくても標準widgetから操作できる。
- Keyboardだけで辞書切り替えやquery clearを素早く行う操作が不足する。

#### C. TUI互換を最大化

- Aに加えて`Ctrl+U`でquery clear、Tabで辞書切り替えを行う。
- TUI利用者は同じkeyで操作しやすい。
- Tabによるfocus traversalを失い、accessibilityとGUI標準操作を損なう。
- `Ctrl+C`はcopyと衝突するため、TUIと同じ終了keyにはできず、完全な互換にはならない。

#### D. Shortcutをuser設定可能にする

- Actionごとのkey sequenceを設定fileまたは設定dialogで変更できるようにする。
- 利用者の好みに合わせられる。
- Conflict検出、default復元、platform別modifier表記、設定migrationが必要になり、初期版には複雑である。

検索欄focusを維持したまま主要操作を完結でき、既存TUI利用者にも馴染みのあるaliasを提供できるためAを採用した。
Shortcutは初期版では固定し、menuや操作案内に表示するkey sequenceと実際の`QAction` / event filter定義を同じsourceから生成する。

### D14. 非同期検索中の表示

**状態: 決定済み（2026-09-13）**  
**決定: A. 直前の成功結果をそのまま表示**

全案で、queryが変化したら直ちにrequest IDを更新し、旧contextをcancelして新しい検索を開始する。
Debounceは初期実装へ導入しない。
空queryは例外として結果、選択、詳細、検索errorを即座にclearし、DB queryを発行しない。

Stale resultと`context.Canceled`は表示せず破棄する。
Current requestがerrorなら結果と選択をclearし、右paneに短いerrorを表示する。
Current requestが0件なら結果と選択をclearし、右paneに該当結果がないことを表示する。

#### A. 直前の成功結果をそのまま表示（採用）

- 空でないqueryから別の空でないqueryへ変化した場合、新結果が返るまで現在のlist、選択、詳細をそのまま表示する。
- Loading indicator、spinner、dim表示を追加しない。
- 通常は数msで結果が返るため、画面のclear / redrawによるちらつきを避けられる。
- Timerと追加animationが不要で、既存TUIと同じ単純なstateになる。
- 検索が通常より遅い場合、検索欄のqueryと表示中結果が一時的に一致しないことが画面から分からない。

#### B. 直前の結果を即座にclear

- Query変更時にlist、選択、詳細をclearし、右paneへ静的な`Searching...`表示を出す。
- 表示中の内容が新queryの結果ではないことが明確になる。
- 高速入力のたびに2-paneが空になり、結果到着後に再表示されるため、ちらつきが発生しやすい。
- 選択中entryを読みながらqueryを修正する使い方ができない。

#### C. 直前の結果を保持し、遅い場合だけstatus表示

- 直前の結果を維持し、例えば検索開始から100msを超えた場合だけstatus areaへ静的な`Searching...`を表示する。
- 通常の高速検索ではAと同じ見た目で、遅延時だけ状態を説明できる。
- Requestごとのtimer cancel、statusの競合、境界時刻のtestが必要になる。
- Spinner animationは使わない。

#### D. 直前の結果をdim表示して操作不可にする

- 新結果が返るまでlistと詳細をdimにし、候補移動を無効化する。
- Staleな内容であることを視覚的に示せる。
- Palette / themeごとのdim表現、accessibility上の状態通知、短時間のstyle更新が増える。
- 検索待ちの間に直前の本文をscrollできなくなる。

通常の検索latencyではloading表示を認識する前に完了し、listのclearと再表示の方が視覚的なnoiseになるためAを採用した。
検索中も直前の候補移動と詳細scrollを許可し、current requestの結果が届いた時点でD11のID維持規則に従ってmodelを置換する。

### D15. Empty / no-result / search-error表示

**状態: 決定済み（2026-09-13）**  
**決定: A. 右の詳細paneへcontextual messageを表示**

本項は、少なくとも1つの辞書DBを利用でき、main windowが通常の検索画面を表示している場合を対象とする。
利用可能なDBがない初回導線はD5のBuilder画面として別に扱う。

#### A. 右の詳細paneへcontextual messageを表示（採用）

- Queryが空なら左listを空にし、右paneへ短い操作案内を表示する。
- Current requestが0件なら左listを空にし、右paneへ`No matching headwords`と表示する。
- Current requestがerrorなら左listを空にし、右paneへ短いerror概要と、queryを変更すると再試行する旨を表示する。
- 詳細なerrorは既存logへ1回だけ記録し、必要なら`Open Log` actionを表示する。
- 右paneは`QStackedWidget`で通常のentry詳細pageとmessage pageを切り替え、messageを`QPlainTextEdit`内の辞書本文として見せない。
- Modal dialogを出さないため、入力を続けて回復できる。
- 右paneの通常内容とmessageを同じ場所で切り替えられ、既存TUIとも一貫する。
- Errorが右側にあるため、window幅が広い場合は検索欄から視線が離れる。

状態例:

```text
Empty query:
    Enter a search term
    Up/Down: Select result  Ctrl+Tab: Switch dictionary

No result:
    No matching headwords

Search error:
    Search failed
    Edit the query to try again  [Open Log]
```

#### B. 検索欄直下のinline bannerへ表示

- Queryが空の時は右paneへ操作案内を表示し、0件とerrorは検索欄直下の全幅bannerへ表示する。
- Queryとの関連が視覚的に近く、errorを見落としにくい。
- Bannerの出現・消失で2-paneの高さが変化し、入力ごとにlayoutが動く。
- Errorとno-resultのために専用widgetとstateを追加する必要がある。

#### C. 常設status barへ表示

- `QStatusBar`を常にwindow下部へ置き、辞書名、結果件数、0件、errorを表示する。
- Layoutが動かず、Qt desktop applicationとして一般的である。
- Empty状態の操作案内や複数行errorを表示するには狭い。
- 通常時にも1行を消費し、辞書名や件数など不要な情報を常時表示しやすくなる。

#### D. Errorをmodal dialogで表示

- Emptyと0件は右pane、検索errorだけは`QMessageBox`で表示する。
- Errorを確実に認識できる。
- Incremental searchでは入力ごとにerrorが繰り返される可能性があり、dialogが入力を妨げる。
- 一時的なDB errorからquery変更で回復する設計と相性が悪いため推奨しない。

入力を止めずに回復でき、通常は空いている右paneを利用できるためAを採用した。

右paneの`QStackedWidget`は次の2 pageで構成する。

1. Entry detail page
   - Selectableなheadword `QLabel`
   - Read-onlyのbody `QPlainTextEdit`
2. Contextual message page
   - Word wrap可能なmessage `QLabel`
   - Search error時だけ表示する`Open Log` action

Empty / no-result / errorごとにpage objectを増やさず、message pageのtext、accessible description、action visibilityをstateに応じて更新する。
Empty時の操作案内はD13のkeymap定義と同じ情報から生成し、実際のshortcutと食い違わないようにする。
Raw error、DB path、request ID、stack traceはmessageへ表示せずlogへ記録する。

### D16. Initial window sizeと状態保存

**状態: 決定済み（2026-09-13）**  
**決定: A. `QSettings`でgeometry / maximized / splitterを保存**

初回起動時は次を共通default候補とする。Sizeはdevice pixelではなくQtのlogical pixelで扱う。

```text
initial size:  960 x 640
minimum size:  720 x 480
left:right:    1:2
left minimum:  240
right minimum: 320
```

Waylandの標準protocolではapplicationがtop-level windowの絶対位置を設定・取得できず、Qtの位置指定が無視されたり`(0, 0)`が返ったりする。
したがって、Waylandで前回と同じscreen座標へ戻すことは要件にしない。

#### A. `QSettings`でgeometry / maximized / splitterを保存（採用）

- `QMainWindow.saveGeometry()`と`restoreGeometry()`、`QSplitter.saveState()`と`restoreState()`を利用する。
- Windows / macOSではwindow位置を含め、platformが復元できる範囲で前回状態へ戻す。
- Waylandではsize、maximized状態、splitter比率を復元し、window位置はcompositorへ任せる。
- GUI状態はQtの`QSettings`へ保存し、辞書pathと検索設定を持つ既存TOMLへ混在させない。
- Invalidまたは未保存の値は無視して共通defaultを使い、minimum sizeとpane minimumを必ず再適用する。
- Desktop applicationとして自然で、Qt標準APIだけで実装できる。
- TOMLとは別にplatform固有のstate storageが作られる。

#### B. Sizeとsplitterだけを保存

- Windowのwidth / heightとsplitter比率だけを数値で保存し、位置とmaximized状態は保存しない。
- Multi-monitor構成変更やWaylandの制約によるgeometry問題を避けやすい。
- Maximizedで終了しても次回は通常windowで開き、platformによっては期待と異なる。
- 保存先はAと同じ`QSettings`とする。

#### C. Window状態を保存しない

- 毎回960x640、splitter 1:2で起動する。
- State fileとrestore errorがなく、最も単純である。
- 利用者が毎回windowとpaneを調整する必要があり、desktop GUIとして不便である。

#### D. 既存の`config.toml`へ`[gui]`を追加

- Window size、maximized、splitter比率を人が読める数値として保存する。
- 設定fileを1つに統一できる。
- GUI操作のたびに、利用者が手書きする設定fileをapplicationが安全に書き換える必要がある。
- Commentとformatの保持、atomic write、TUI / CLIとのschema共有が必要になり、transientなGUI状態には重い。

Qt標準のdesktop state保存を利用でき、利用者が管理する辞書設定TOMLをGUIが書き換えずに済むためAを採用した。

初期key案:

```text
gui/stateVersion
mainWindow/geometry
mainWindow/splitter
```

`saveGeometry()`の情報からplatformが対応するsize、位置、maximized状態を復元する。
Waylandでは位置情報を信頼せず、compositorの配置を受け入れる。
`stateVersion`が未対応、byte arrayが空またはrestoreに失敗、復元sizeがminimum未満の場合はdefaultへ戻す。
Splitter復元後もleft / right minimum widthを適用し、片方のpaneが見えなくなる状態を許可しない。

### D17. Result countと通常statusの表示

**状態: 決定済み（2026-09-13）**  
**決定: A. Header右端に表示件数だけを表示**

検索は全一致件数を数えず`max_results`で打ち切るため、GUIが把握できるのは現在表示している件数だけである。
`50`を総一致件数の意味で表示せず、`50 shown`のように表現する。
Current dictionaryは`QComboBox`で分かるため、別の場所へ重複表示しない。

#### A. Header右端に表示件数だけを表示（採用）

- 検索欄の右に小さな`QLabel`を置き、成功結果がある時だけ`N shown`と表示する。
- `N == max_results`なら`Showing first N`として、さらに一致する可能性があることを示す。
- Queryが空、検索中の旧結果がない、error時は空欄にする。0件はD15の右pane messageだけで示す。
- Windowの縦方向を消費せず、表示中listとの対応も分かる。
- Narrow windowでは検索欄の横幅を少し減らす。

```text
| [English–Japanese ▼] [ query........ ] [50 shown] |
```

#### B. 常設`QStatusBar`へ表示

- Window下部の左に一時message、右に`N shown`を配置する。
- 検索欄の横幅を最大限使え、desktop applicationとして一般的である。
- 通常時にもwindow下部を1行消費する。
- D15でerrorを右paneへ表示するため、status barの役割が件数表示だけになりやすい。

#### C. Result listの下へ表示

- 左pane内の`QListView`直下へ`N shown` labelを置く。
- どのlistの件数か最も明確で、右paneの幅へ影響しない。
- Listの表示可能な高さが常に1行減る。
- Left paneだけ下端が分割され、2-paneの見た目が揃わない可能性がある。

#### D. 件数と通常statusを表示しない

- Headerは`QComboBox`と検索欄だけにし、status barも設けない。
- 最もcompactで、表示件数を総一致件数と誤解されない。
- Result上限に達して候補が打ち切られたことを利用者が判断できない。
- 診断情報はlogだけで確認する。

Windowの縦方向を消費せず、表示中のresult上限を利用者へ伝えられるためAを採用した。

実装規則:

- Labelは`QStringListModel`に現在入っているrow数と同時に更新する。
- D14により旧resultを表示している検索中は、旧modelに対応する件数もそのまま表示する。
- 1〜`max_results - 1`件では`N shown`、`max_results`件では`Showing first N`とする。
- 0件、empty query、search errorではtextを空にする。
- 最大文言`Showing first 500`に合わせたminimum widthを確保し、件数更新で検索欄の幅が動かないようにする。
- Total countを取得する追加queryは発行しない。
- Screen reader向けaccessible nameは`Search results shown: N`とし、空欄時は読み上げ対象から外す。

### D18. 選択entryのclipboard copy

**状態: 決定済み（2026-09-13）**  
**決定: B. 標準copyに加えて「entry全体をcopy」actionを提供**

全案で、headword `QLabel`とbody `QPlainTextEdit`はplain textとしてmouse / keyboard selectionを利用できるようにする。
Bodyでは`QPlainTextEdit`の標準context menuを利用し、headwordへcontext actionが必要な案では明示的に追加する。
`QPlainTextEdit`へfocusした時の`Ctrl+C` / `Cmd+C`は選択範囲だけをcopyする。
IME preedit中のkeyをcopy actionへ転用しない。

#### A. Qt標準のtext selection / copyだけを提供（最小構成）

- MouseまたはTabで詳細paneへfocusし、必要な範囲を選択して`Ctrl+C` / `Cmd+C`でcopyする。Bodyでは標準context menuも使用できる。
- 専用button、shortcut、clipboard整形を追加しない。
- OS標準の予測可能な動作で、画面とkeymapを増やさない。
- 検索欄focusを維持したままentry全体をcopyすることはできない。
- Headwordとbodyを一度にcopyするには、それぞれ別widgetなので2回の操作が必要になる。

#### B. 標準copyに加えて「entry全体をcopy」actionを提供（採用）

- Detail paneのcontext menuとEdit menuへ`Copy Entire Entry` actionを追加する。
- `Ctrl+Shift+C` / `Cmd+Shift+C`を固定shortcutとし、検索欄にfocusがあっても選択中entryをcopyする。
- Clipboard textは`headword + "\n" + body`とし、末尾newlineは追加しない。
- 選択entryがなければactionをdisabledにする。
- Headwordまたはbodyの一部だけをcopyする場合は標準selectionを使う。
- Input-centric操作のまま辞書entryを利用できるが、独自shortcutが1つ増える。

#### C. Headword / body / entry全体の3 actionを提供

- Context menuとEdit menuへ`Copy Headword`、`Copy Body`、`Copy Entire Entry`を追加する。
- Copy対象を選べ、mouse selectionなしでも各部分を取得できる。
- Menu項目とshortcut設計が増え、単純な詳細paneに対して機能が多い。
- Visible buttonは置かず、headerのcompactさは維持する。

#### D. Detail paneへvisibleなcopy buttonを置く

- Headword横にcopy iconまたは`Copy` buttonを表示し、entry全体をcopyする。
- Mouse利用者が機能を発見しやすい。
- Headword表示幅を減らし、icon resource、tooltip、hover / focus、accessible nameの管理が増える。
- 押した後のfeedbackを表示する場所も必要になる。

検索欄focusを維持したままentry全体を利用でき、visible buttonを増やさずに済むためBを採用した。

実装規則:

- `Ctrl+Shift+C` / `Cmd+Shift+C`、Edit menu、detail paneのcontext menuは同じ`QAction`を共有する。
- Copy元はQt widgetの表示textではなく、Go controllerが保持するcurrent selected `search.Entry`とする。
- Clipboardへは`text/plain`として`Headword + "\n" + Body`を書き込み、HTML、dictionary名、entry ID、末尾newlineを追加しない。
- 選択entryがない場合はactionをdisabledにし、shortcutを押してもclipboardを変更しない。
- Copy後にmodal、toast、status messageを表示しない。
- Qtの`QClipboard`をmain threadから操作する。

### D19. Menu bar構成

**状態: 決定済み（2026-09-13）**  
**決定: A. 標準`QMenuBar`を使用**

Menu actionとshortcut actionを別々に実装せず、同じ`QAction` instanceをmenu、context menu、shortcutから利用する。
Actionのenabled / checked状態もGo controllerのstateから一元的に更新する。

#### A. 標準`QMenuBar`を使用（採用）

- `QMainWindow`へFile / Edit / Dictionary / Helpの4 menuを置く。
- Linux / Windowsではwindow上部、macOSではnative application menu barとして表示する。
- QtのMenuRoleを設定し、macOSではQuitとAboutをplatform標準位置へ移動できるようにする。
- Keyboard shortcutを知らない利用者にもBuilder、copy、辞書切り替え、log、Aboutを発見しやすい。
- Linux / Windowsではmenu barが縦方向を1行使用する。

初期menu案:

```text
File
  Build Dictionary Database...
  ----------------
  Quit

Edit
  Copy Entire Entry
  ----------------
  Focus Search
  Clear Search

Dictionary
  English–Japanese
  Japanese–English

Help
  Keyboard Shortcuts
  Open Log
  ----------------
  About EJQuick
```

- Dictionary menuの2 actionはcheckable / exclusiveとし、`QComboBox`と同じ辞書選択controllerへ接続する。
- 利用できない辞書のactionはdisabledにする。
- 初期版ではtoolbarとiconを追加しない。

#### B. File / Helpだけの最小`QMenuBar`

- Builder、Quit、log、Aboutだけをmenuへ置く。
- Copyと検索操作はshortcut / context menu、辞書切り替えは`QComboBox`だけにする。
- Menu bar自体の高さはAと同じだが、項目数と重複導線を減らせる。
- 固定shortcutの一覧やcopy actionを発見しにくい。

#### C. Header右端のoverflow menu

- `QMenuBar`を置かず、表示件数の横へ`QToolButton`を置いて全actionを1つのmenuにまとめる。
- Linux / Windowsで縦方向を節約できる。
- Desktop標準のmenu分類、mnemonic、macOS native menu barを活用できない。
- Result countとmenu buttonが検索欄の横幅をさらに減らす。

#### D. OSごとにmenu表現を変える

- macOSはnative `QMenuBar`、Linux / Windowsはoverflow menuとする。
- 各platformの見た目を優先できる。
- Action配置、shortcut help、screenshot、UI testがOSごとに分岐する。
- 初期のcross-platform frontendとしては保守範囲が広い。

Desktop標準の操作性、macOS native menu、actionの発見しやすさを優先してAを採用した。

実装規則:

- `QMenuBar`は`QMainWindow`に所有させ、toolbarは作成しない。
- Quit actionには`QAction::QuitRole`、About actionには`QAction::AboutRole`を設定する。
- `Copy Entire Entry`、`Focus Search`、`Clear Search`はD13 / D18と同じ`QAction`を使う。
- Dictionary action groupと`QComboBox`は、signalを相互に直接連結せず、Go controllerのdictionary変更処理を共通入口にする。
- Controllerがaction / comboのchecked / selected状態を更新する間はsignalをblockし、切り替え処理を二重実行しない。
- Menu text、shortcut help、empty時の操作案内は共通action定義から生成する。
- Iconは初期版で使用せず、platform標準のtext menuとする。

### D20. Builder UIのwindow構成

**状態: 決定済み（2026-09-13）**  
**決定: A. Setupから完了まで1つのmodal `QDialog`**

全案で`ejquick-build`はchild processとして非同期起動し、Qt main threadをblockしない。
同時に起動できるBuilderは1 processだけとし、実行中は新しいbuild actionをdisabledにする。
Source TXTの内容や辞書entryをGUI logへ出力しない。

Builder UIは少なくとも次の2 stateを持つ。

1. Setup
   - 辞書種別
   - 購入済みTXT file path
   - 出力DB path
   - 既存DBを置換する場合の明示確認
2. Progress / result
   - 現在phase
   - 読み込み行数、登録件数、skip件数
   - Cancel
   - 成功時の件数、DB size、所要時間
   - 失敗時の短いerrorとlog / detail導線

#### A. Setupから完了まで1つのmodal `QDialog`（採用）

- File menuまたはDB未作成messageのbuttonからdialogを開く。
- SetupでBuildを押すと同じdialog内をprogress表示へ切り替え、完了までmain window操作を無効にする。
- Dialog自体のevent loopとchild process I/Oは動き続け、windowは応答可能なままにする。
- Build中にdialogを閉じようとした場合はcancel確認を表示する。
- 状態遷移、同時操作、testが最も単純で、初回導線に適する。
- 既存DBで検索できる場合も、build中はmain windowで検索・閲覧できない。

#### B. Setupはmodal、progress開始後はmodeless

- 入力確認まではmodal dialogとし、Build開始後はmodeless progress windowとしてmain window操作を再び許可する。
- 既存の辞書で検索しながら、別辞書の作成や再buildを待てる。
- 検索とbuildが同時にdisk / CPUを使い、双方の応答へ影響する可能性がある。
- Progress windowを閉じた場合にbuildをcancelするかbackground継続するか、application終了時にどう待つかを設計する必要がある。

#### C. Main windowをBuilder pageへ切り替える

- 通常の2-pane central widgetとBuilder pageを`QStackedWidget`で切り替える。
- 利用可能なDBがない初回起動では、独立dialogより自然なsetup画面を作れる。
- 既存DBがある状態でbuildすると検索画面が隠れ、戻る・cancel・完了後遷移のnavigationが必要になる。
- Main window controllerへ検索とbuildの2種類の大きなstate machineが入る。

#### D. 独立したBuilder window

- Setup / progress専用のtop-level windowをmain windowと並行して開く。
- Main windowから独立して移動・最小化でき、長時間buildを監視しやすい。
- Parent、application lifecycle、複数window、macOSのClose / Quit、focus復帰のtestが増える。
- 初期版の補助機能としてはwindow管理が重い。

初回導線を単純にし、build中のmain window操作と複数buildを確実に防げるためAを採用した。

実装規則:

- Dialogはmain windowに対するwindow-modalとし、application全体を対象とする不要なglobal modalにはしない。
- BlockingなGo処理やchild process待機をQt main threadで実行しない。
- Nested event loopへ依存する同期的な結果取得を避け、dialogのsignal、process I/O goroutine、MIQT main-thread dispatchでstateを更新する。
- SetupからProgressへ移行した後は入力値を変更できない。
- Build実行中のClose、Escape、window managerのclose requestは、すぐ閉じずcancel確認を表示する。
- Cancel開始後はbuttonをdisabledにして`Canceling...`を表示し、process終了を待つ。
- Success時はsummaryと`Close`を表示し、閉じた後に作成したDBをopenしてmain windowのdictionary stateを再構築する。
- Failure時は入力値を保持したままSetupへ戻れるようにし、修正後に再実行できるようにする。

### D21. Builder progress / cancel protocol

**状態: 決定済み（2026-09-13）**  
**決定: A. stdout JSON Lines + stdin control channel**

通常の`ejquick-build` CLIでは既存どおりhuman-readable progressをstderrへ出し、stdoutを空に保つ。
GUIから起動する時だけ明示optionでmachine modeを有効にし、通常CLIとの互換性を壊さない。

全案で次を満たす。

- Protocolに独立したinteger versionを持たせ、product versionだけで互換性を判断しない。
- Source TXTの本文、headword、bodyをeventへ含めない。
- GUIはstdout / stderrを逐次drainし、全出力をmemoryへ保持しない。
- Progressは総行数を事前scanせず、phase名と処理済みlines / entries / skippedを表示する。Percentageは表示しない。
- GUIはcompletion eventとexit code 0の両方を確認した場合だけ成功とする。
- Malformedまたは非互換なprotocolを検出したら成功扱いせず、childをcancelして短いerrorを表示する。

#### A. stdout JSON Lines + stdin control channel（採用）

- GUIは例えば`--progress-format jsonl --control-stdin`を指定してBuilderを起動する。
- BuilderはUTF-8 JSON objectを1 event 1 lineでstdoutへ出し、human-readable diagnosticsだけをstderrへ出す。
- GUIからBuilderへのcancelはstdinへversion付きcommandを1行書く。
- `--control-stdin`時のstdin EOFはparent消失とみなし、Builderもcancelする。
- OS signalへ依存せずLinux / Windows / macOSで同じcancel経路を使える。
- Builderに`context.Context`、typed progress event、stdin command readerを追加する必要がある。

Event例:

```jsonl
{"protocol":1,"event":"started","product_version":"v0.1.0","dictionary":"eiwa"}
{"protocol":1,"event":"phase","phase":"reading","state":"started"}
{"protocol":1,"event":"progress","phase":"reading","lines":100000,"entries":99998,"skipped":2}
{"protocol":1,"event":"phase","phase":"fts","state":"started"}
{"protocol":1,"event":"completed","source_lines":2577796,"entries":2577700,"skipped":96,"db_size":123456789,"elapsed_ms":123456}
```

Cancel command例:

```jsonl
{"protocol":1,"command":"cancel"}
```

#### B. stdout JSON Lines + OS signalでcancel

- Progress eventはAと同じJSON Linesを使い、cancelはSIGINT / SIGTERM等をchild processへ送る。
- Builder CLIをterminalから中断するsignal handlingと処理を共有できる。
- Windows GUI processからconsole control signalを安全に送る方法、process group、子孫processの扱いがUnixと異なる。
- Signalを送れない環境ではhard killへfallbackする必要がある。

#### C. 既存stderrをparse + hard kill

- 現在の`Reading: lines=...`等をGUIがparseし、cancel時は`os.Process.Kill`相当で即時終了する。
- Builder側の変更を最小限にできる。
- Human-readable text変更でGUIが壊れ、protocol versionもない。
- Hard killではdefer cleanupが実行されず、一時DBが残る可能性がある。
- Cross-platformに動いても安全なcancelと正しい完了判定を保証しにくい。

#### D. Local socket / named pipeによる双方向protocol

- GUIが一時endpointを作り、Builderが接続してJSON等でprogressとcontrolを双方向送信する。
- stdout / stderrをdiagnostics専用に保ち、command追加へ拡張しやすい。
- Unix domain socketとWindows named pipeの差、endpoint認証、cleanup、接続timeoutが必要になる。
- Parent-child間の単純な1対1通信には過剰である。

OS signalやhuman-readable textに依存せず、3 OSで同じ進捗・cancel経路を使えるためAを採用した。

Protocolの初期仕様:

- GUIは`ejquick-build --machine-protocol 1 ...`でmachine modeを有効にする。
- Machine modeではstdoutをJSON Lines event専用、stdinをJSON Lines command専用、stderrをhuman-readable diagnostics専用とする。
- 最初のstdout eventは、入力fileを開く前に`ready`としてprotocol version、product version、dictionary typeを通知する。
- GUIは`ready`を検証し、一致する場合だけstdinへ`start` commandを送る。Version不一致ならbuildを開始せずcancelする。
- `start`は1回だけ受理し、未受理の間はDBとsource TXTへ触れない。
- Terminal eventは`completed`、`failed`、`cancelled`のいずれか1つとする。
- Graceful cancelのexit codeは130、成功は0、usage errorは1、build failure / protocol errorは2とする。
- GUIは`completed` eventとexit code 0の両方が揃った場合だけ成功とする。
- Event / commandは1行64 KiBを上限とし、超過または不正JSONをprotocol errorにする。
- Unknown fieldは同じprotocol version内では無視できるが、unknown event / commandはerrorとする。

Builder APIは次の方向へ拡張する。

```go
func RunContext(
    ctx context.Context,
    opts Options,
    report func(Event) error,
) (Stats, error)
```

既存`Run`は`context.Background()`とhuman-readable reporterを使うwrapperとして残し、通常CLIとの互換性を維持する。
TXT読み込みloopでは定期的に`ctx.Err()`を確認し、SQLite処理には`ExecContext` / `PrepareContext`等を使用する。
Cancel時も通常のerror経路でtransaction rollback、DB close、一時DB削除を完了してから`cancelled`を送る。

GUI側はcancel command送信後もstdout / stderrをdrainしながらchildの終了を待つ。
一定時間応答しない場合のhard-kill timeoutと一時DBの回収方法は、D23の決定（10秒、今回の一時DBのみ厳密検証して削除）に従う。

### D22. Builder setup fields / overwrite policy

**状態: 決定済み（2026-09-13）**  
**決定: A. 必須項目だけのsingle-page form**

どの案でも、source TXTとoutput DBが同じpath、sourceが通常fileでない、sourceをreadできない、output parentを作成できない等の明白なerrorはBuild開始前に検出する。
TXT内容のdecode / format検証は二重実装せずBuilderへ任せる。

#### A. 必須項目だけのsingle-page form（採用）

- 辞書種別`QComboBox`、source TXT path、read-onlyのoutput DB pathを1 pageへ表示する。
- Source pathはeditableな`QLineEdit`と`Browse...` buttonで指定する。
- Outputは現在のEJQuick configが選択辞書に対して返すpathへ固定し、GUIから任意pathへ変更しない。
- Outputが存在する時だけ`Replace the existing dictionary database` checkboxを表示し、checkedになるまでBuild buttonをdisabledにする。
- Build開始時の追加confirmation dialogは出さず、checkboxの文言、output path、既存file sizeで置換対象を明示する。
- `--compact`は初期GUIへ公開せずfalseとする。必要な利用者はCLIを使用する。
- 項目が少なく、GUIがconfig.tomlを書き換えず、誤った出力先を作りにくい。

```text
Dictionary: [ English–Japanese ▼ ]
Source TXT: [ /path/EIJIRO...TXT       ] [Browse...]
Output:     /home/user/.../eiwa.sqlite3

[ ] Replace the existing dictionary database  (only when present)

                              [Cancel] [Build]
```

#### B. CLI optionをすべて公開するsingle-page form

- Aに加えてeditableなoutput path、`compact` checkboxを表示する。
- GUIだけで`--output`、`--force`、`--compact`相当を指定できる。
- Custom outputを選んでも既存config.tomlは自動更新しないため、次回起動時にそのDBを見失う可能性がある。
- Advanced optionとvalidationが増え、初回導線が複雑になる。

#### C. 必須項目 + 折りたたみAdvanced section

- 通常はAと同じ表示にし、Advancedを開いた時だけoutput pathとcompactを変更できる。
- 初回利用者の画面を単純に保ちながらCLI機能も提供できる。
- Custom outputを検索configへどう反映するかを別途設計する必要がある。
- Collapsible widget、状態保存、追加testが必要になる。

#### D. 複数pageのwizard

- 辞書種別 / source、output / overwrite、最終確認をBack / Nextで順に入力する。
- 各段階で説明を多く表示でき、誤操作を防ぎやすい。
- 必須入力が少ない今回のBuilderにはpage数が過剰で、開始までの操作が増える。

初回導線を短くし、GUIがconfig.tomlを書き換えず、buildしたDBを次回も確実に見つけられるためAを採用した。

実装規則:

- DB未作成messageから開いた場合は対象の辞書種別、Dictionary menuから開いた場合はcurrent dictionaryを初期選択する。
- Source pathのfile dialogはTXT filterを最初に表示するが、extensionだけで拒否せずall filesも選択可能にする。
- Source pathを手入力またはpasteできるようにし、前後の空白を勝手にtrimして実在するpathを変えない。
- Output labelは`config.Config.Database(dictionaryType)`の絶対pathを表示し、編集widgetにしない。
- Outputが存在しない場合はreplace checkboxを表示しない。
- Outputが存在する場合はfile sizeとpathを表示し、replace checkboxがcheckedになるまでBuildをdisabledにする。
- Checkboxをcheckedにして開始する操作を明示確認とし、追加の`QMessageBox`は出さない。
- GUIは`--compact`を指定しない。
- Build開始時に対象DBの既存search serviceをcloseし、Windowsでも完成DBを置換できる状態にする。
- Build成功時はsummaryを表示し、dialog close後に新DBを検査してopenする。失敗またはcancel時は旧DBが残っていればdialog close後に再openする。
- 再openにも失敗した場合はその辞書をunavailableにし、main windowのcontextual messageとlogへ理由を記録する。

### D23. Builder hard-kill / temporary DB cleanup

**状態: 決定済み（2026-09-13）**  
**決定: B. Grace period後にhard killし、今回の一時DBだけGUIが削除**

通常のcancelではD21のstdin commandから`context.Context`をcancelし、Builder自身がtransaction rollback、DB close、一時DB削除を行う。
本項は、native callの停止不能、driver不具合、process hang等によりgraceful cancelが完了しない場合を扱う。

#### A. Hard killせず、終了するまで待つ

- Cancel後は時間制限を設けず、Builderの`cancelled` eventとprocess終了を待つ。
- Builder自身のcleanupを必ず通せる可能性が最も高い。
- Processがhangするとdialogとapplication終了も完了せず、利用者が外部からkillするしかない。
- GUI crashやOS shutdownに対するstale file問題は解決しない。

#### B. Grace period後にhard killし、今回の一時DBだけGUIが削除（採用）

- Cancel command送信後10秒待ち、childが終了しなければhard killする。
- Builderは一時DB作成直後に、その絶対pathをmachine eventでGUIへ通知する。
- Hard kill後、GUIは通知された今回の一時pathだけを検証して削除する。
- Output parentと同じdirectory、期待するbasename prefix、regular file、symlinkでないことを確認し、final output pathは絶対に削除しない。
- Pathを受信していない場合や検証に失敗した場合は自動削除せず、残存可能性とpathをlogへ記録する。
- Hangから回復でき、一時DBも通常は回収できるが、安全なpath検証とplatform別kill testが必要になる。

追加event例:

```jsonl
{"protocol":1,"event":"temporary_created","path":"/data/ejquick/.ejquick-build-abc.sqlite3"}
```

#### C. Grace period後にhard killし、一時DBは削除しない

- 10秒後にhard killしてdialogとapplicationを回復させる。
- GUIはfileを削除せず、残った可能性のある一時pathをerror詳細とlogへ表示する。
- 誤削除の危険はない。
- 数GB規模の一時DBが残り、利用者が手動で削除するまでdiskを消費する可能性がある。

#### D. 起動時にoutput directoryをscanして古い一時DBを削除

- GUIまたはBuilder起動時に`.ejquick-build-*.sqlite3`を探し、一定時間より古いfileを削除する。
- GUI crashやOS強制終了で残ったfileも回収できる。
- 別のBuilder processが使用中のfile、利用者が偶然同名にしたfile、clock差を誤判定する危険がある。
- Ownership markerとprocess lockなしの自動scan / deleteは安全でないため推奨しない。

Hangから有限時間で回復しつつ、自動削除の対象を今回起動したchildが作成したfileだけに限定できるためBを採用した。

実装規則:

- 10秒はmonotonic clockで測り、cancel要求時にstdinへのcommand writeを試みた直後から、write結果にかかわらず数える。
- Terminal eventを受けてもchildが終了するまではwaitを続け、process終了を確認した時だけhard-kill timerを停止する。
- Timeout時はplatformのprocess kill APIで対象childだけを停止し、process終了をwaitしてからfile cleanupを開始する。
- Cleanup対象は、同じchildから受信した最後の`temporary_created.path` 1件だけとする。
- `filepath.Clean` / absolute pathで比較し、output parentと同じdirectory、basenameが`.ejquick-build-` prefixと`.sqlite3` suffix、final outputと異なることを確認する。
- `Lstat`でsymlinkを拒否し、regular file以外を削除しない。
- 条件を1つでも満たさない、path eventがない、削除に失敗した場合は自動削除せずlogへ記録する。
- 過去のprocessが残した一時DBをdirectory scanで自動削除しない。
- GUIの通常終了中も同じcancel、10秒wait、hard kill、cleanupの順序を使う。
- GUIが異常終了してもstdin EOFを受け取ったBuilderがgraceful cancelすることをintegration testで確認する。

### D24. Loggingと`Open Log` action

**状態: 決定済み（2026-09-13）**  
**決定: A. 既存append-only logを外部applicationで開く**

GUIは既存`internal/logging` packageとTUI / CLIと同じlog fileを利用する。
Logging failureはbest effortとしてapplicationの起動・検索を妨げない。
辞書TXTの内容と辞書entry本文はlogへ記録しない。

#### A. 既存append-only logを外部applicationで開く（採用）

- Errorは既存どおり1つの`ejquick.log`へ追記し、rotationやsession別fileを追加しない。
- Help menuとD15のsearch-error messageから同じ`Open Log` actionを呼ぶ。
- `QUrl::fromLocalFile`と`QDesktopServices::openUrl`を使い、OSで関連付けられたtext viewerへ渡す。
- Loggerは実際にopenできたpathと利用可否をGUIへ公開し、logging unavailable時はactionをdisabledにする。
- `openUrl`がfalseならuser操作に対する短いerror dialogでpathを表示し、手動で開けるようにする。
- Debug loggingは起動option `--debug`でだけ有効にし、実行中のmenu切り替えは設けない。
- 現行実装をほぼ再利用できるが、長期間debugを有効にするとlogが増え続ける。

#### B. Size-based rotationを追加して外部applicationで開く

- 例えば起動時に5 MiBを超えていれば`ejquick.log.1`へrotateし、backupを1世代だけ保持する。
- Disk使用量に上限を設けられる。
- TUI / GUIの同時起動、Windowsのopen file、rename failure、複数process間lockを設計する必要がある。
- 既存logging package全体の動作変更になり、GUI初期実装の範囲を超えやすい。

#### C. GUI専用log fileへ分離

- `ejquick-gui.log`を追加し、GUI errorだけを記録する。
- GUI問題を探しやすく、TUIとの同時writeを避けられる。
- Search coreの同じerrorがfrontendごとに別fileへ分かれ、利用者が確認する場所が増える。
- Product全体で1つのlogを使う既存方針と一致しない。

#### D. Application内蔵log viewer

- Read-only dialogで末尾の一定行を表示し、copyやreloadを提供する。
- 外部text viewerがなくても内容を確認できる。
- File tail、文字code、不完全な最終行、更新監視、大きなfile、検索等の追加実装が必要になる。
- 辞書検索frontendの初期機能としては過剰である。

既存frontendと診断情報の保存先を統一し、log viewerやrotationの追加複雑性を避けるためAを採用した。

実装規則:

- `logging.Logger`へ、実際にopenできたlog pathと利用可否をread-onlyで取得するAPIを追加する。
- Loggerが`io.Discard`へfallbackした場合、`Open Log` actionをdisabledにする。
- Action実行時はQt main threadで`QDesktopServices::openUrl(QUrl::fromLocalFile(path))`を呼ぶ。
- `openUrl`のtrueはOSへrequestを渡せたことだけを意味し、外部applicationでの表示完了とは扱わない。
- `openUrl`がfalseなら、userが明示的に実行したactionの失敗として短い`QMessageBox`を1回表示し、log pathを選択・copy可能なtextで示す。
- GUI起動時に既存と同じ`--debug`が指定された場合だけ、正規化済みquery、result count、elapsed time等のDEBUG行を出す。
- Menuからdebugを切り替える機能、log rotation、GUI専用log、内蔵viewerは初期版へ追加しない。
- Error logにも辞書entry本文とsource TXT本文を含めない。

### D25. GUI executable名とcommand-line interface

**状態: 決定済み（2026-09-13）**  
**決定: A. `ejquick-gui`を新設し、GUI用の最小optionだけを持つ**

既存の`ejquick`は引数なしでTUI、positional queryありでCLI検索を行うsingle binaryであり、そのcontractを維持する。
`ejquick-build`も既存名を維持する。

#### A. `ejquick-gui`を新設し、GUI用の最小optionだけを持つ（採用）

- Source entry pointを`cmd/ejquick-gui`、binary filenameを`ejquick-gui`とする。
- Window title、desktop menu、About、desktop entry等の利用者向けproduct表記は`EJQuick`とする。
- `ejquick`と`ejquick-build`の既存CLI contract、Pure Go build、single-binary配布へ影響を与えない。
- GUI packageだけがMIQT、CGO、Qt runtimeへ依存する。

初期CLI案:

```text
Usage: ejquick-gui [options]

Options:
  -c, --config <path>  config file path
      --debug          enable debug logging
  -h, --help           show help
  -v, --version        show version
```

- Positional argument、初期query、`--dictionary`、`--limit`は初期GUIで受け付けない。
- Config default、`--config`、`--debug`の意味は既存`ejquick`と同じにする。
- Help / versionはconfig、DB、`QApplication`を初期化せずstdoutへ出力して0で終了する。
- Usage errorはstderrへ出力して2、GUIの通常終了は0、起動・runtime errorは2とする。
- Desktop entryは引数なしで`ejquick-gui`を起動する。

#### B. `ejquick`をGUIへ変更し、TUIを`ejquick-tui`へrename

- Productの代表commandをGUIにできる。
- 既存user、script、README、release artifactの`ejquick`動作を破壊する。
- TUI / CLIのbinary名とinstall手順を変更するmigrationが必要になる。

#### C. 1つの`ejquick` binaryへGUIも統合

- `--gui` / `--tui`または起動環境でfrontendを選択する。
- 利用者が覚えるbinary名を1つにできる。
- TUI / CLI binaryまでCGOとQt runtimeへ依存し、既存single-binary / Pure Go方針を維持できない。
- Headless CLIでもQt libraryを含む配布物が必要になり、release sizeとbuild matrixが大きくなる。

#### D. `EJQuick`または`ejquick-desktop`を使用

- `EJQuick`はdesktop applicationらしいが、既存の技術識別子をlowercaseに統一する命名規則から外れる。
- `ejquick-desktop`は役割が明確だが、`ejquick-gui`より長く、source directory名との対応も弱い。
- 機能上の利点は小さい。

既存TUI / CLIの軽量な単一binaryとcommand contractを維持し、GUI dependencyを完全に分離できるためAを採用した。

実装規則:

- `cmd/ejquick-gui/main.go`は薄いentry pointとし、UI / controller実装は`internal/gui`以下へ置く。
- GUIもTUI / CLI / Builderと同じ`internal/buildinfo.Version`を使用し、製品versionを分けない。
- Argument parseは`QApplication`生成前に行う。
- `--help` / `--version`はQt platform plugin、config、log、DBを初期化しない。
- GUIの`--config`省略時は既存`config.DefaultPath()`と、fileがなければ`config.Defaults()`を使う規則を共有する。
- Positional argumentと未定義optionはusage errorとし、暗黙に初期queryへ変換しない。
- Qt内部optionを無制限にpass-throughせず、必要なplatform選択はD26でEJQuickのcontractとして定義する。
- Release archiveには既存`ejquick`、`ejquick-build`とは別に`ejquick-gui`を含める。

### D26. Linux Wayland / input method選択

**状態: 決定済み（2026-09-13）**  
**決定: C. Qtの自動選択へ完全に任せる**

Qt platform pluginとinput method pluginの選択は`QApplication`生成時に行われるため、必要な環境変数は生成前に確定する。
Package内pluginは`qt.conf`から探索し、異なるsystem Qtを指す`QT_PLUGIN_PATH`をEJQuick自身では設定しない。

#### A. Waylandを要求し、input methodは利用者環境を尊重

- Linuxでは`QT_QPA_PLATFORM`が未設定なら、`QApplication`生成前に`wayland`を設定する。
- 明示的な`QT_QPA_PLATFORM`が`wayland`以外なら、初期releaseの非対応構成として短いstartup errorを出す。
- `WAYLAND_DISPLAY`等の環境値だけで接続可否を断定せず、debug時の診断情報として記録する。
- `QT_IM_MODULE`はEJQuickから設定・上書きせず、desktop sessionまたは利用者のFcitx5 / IBus設定を尊重する。
- Fcitx5利用者が`QT_IM_MODULE=fcitx`を設定した確認済み構成はそのまま動作する。
- `QApplication`生成後にplatform nameがWaylandであることを検査し、debug logへ記録する。
- Native Wayland targetを保証しつつ、特定input method frameworkだけへ固定しない。

#### B. Wayland + Fcitx5へ固定

- 未設定かどうかにかかわらず`QT_QPA_PLATFORM=wayland`、`QT_IM_MODULE=fcitx`を設定する。
- 確認済みsampleと同じ環境を強制できる。
- GNOME標準のIBus、Wayland text-input protocol、その他input methodを利用する環境を壊す可能性がある。
- Applicationがdesktop全体のinput method設定を上書きするため、一般配布には適さない。

#### C. Qtの自動選択へ完全に任せる（採用）

- `QT_QPA_PLATFORM`と`QT_IM_MODULE`をどちらも設定・検査しない。
- Distributionとdesktop environmentの標準挙動へ最も自然に従う。
- Qt設定によってはWayland session上でも`xcb` / XWaylandを選ぶ可能性があり、native Wayland初期targetを保証できない。
- Bundleしていないplatform pluginが選択されるとstartupに失敗する。

#### D. EJQuick独自optionで選択可能にする

- `--platform wayland|xcb`、`--input-method fcitx|ibus|...`等を追加する。
- Trouble shootingと将来のX11対応には便利である。
- 初期CLIが増え、未検証の組み合わせを利用者向け機能として公開することになる。
- Input method名はQt plugin実装に依存し、安定したEJQuick contractにしにくい。

Desktop environmentと利用者が選んだQt / input method設定をapplicationが上書きしないためCを採用した。

実装規則:

- EJQuickは`QT_QPA_PLATFORM`、`QT_IM_MODULE`、`QT_PLUGIN_PATH`を設定、変更、削除しない。
- Qtの`-platform`等をEJQuick固有optionとして追加せず、D25のargument parserから無制限にpass-throughもしない。
- `QApplication`生成後、実際のplatform nameと、値がある場合は関連環境変数をdebug logへ記録する。
- Production起動ではplatformを固定しないが、初期releaseの正式なGUI testは`QT_QPA_PLATFORM=wayland`を明示してnative Wayland上で行う。
- Qtが`xcb` / XWaylandを自動選択し起動できた場合も、X11を正式対応と表記するまではbest effortとする。
- 選択されたplatform pluginをloadできない場合はQtのstartup failureとなる。Packageするplugin範囲はD27で決定する。
- Fcitx5、IBus、Wayland text-input protocol等のどれを利用するかはQtとdesktop環境へ任せ、EJQuick側でinput method名を判定して分岐しない。

### D27. Linux QPA platform pluginのbundle範囲

**状態: 決定済み（2026-09-13）**  
**決定: A. WaylandとXCBの2種類をbundle**

Qtの自動選択を使うため、package内に存在するpluginとそのruntime dependencyが起動可能なplatform範囲を決める。
Package内のQt 6.11.2と異なるsystem QPA pluginを混在させない。

#### A. WaylandとXCBの2種類をbundle（採用）

- `qwayland`系と`qxcb`のplatform pluginをpackageへ含める。
- Wayland sessionではQtの自動選択に従いnative WaylandまたはXWayland、X11 sessionではXCBで起動できる可能性がある。
- 初期の正式test / support対象はWaylandだけとし、XCBはfallback / best effortとして扱う。
- XCB pluginが要求するX11 / xcb libraryもdependency stagingとclean testの対象になる。
- Package sizeとruntime dependencyはWaylandだけの場合より増えるが、D26-Cの自動選択と最も整合する。

#### B. Wayland pluginだけをbundle

- 初期targetに必要な`qwayland`系だけを含め、XCBは含めない。
- Packageとdependencyを最小化できる。
- Qtのdefault選択がXCBになった環境では、Wayland sessionでも起動に失敗する可能性がある。
- 確実にWaylandを選ぶにはD26-Aのような明示指定が必要になり、決定済みD26-Cとの組み合わせが弱い。

#### C. Waylandをbundleし、XCBはsystem pluginへfallback

- Waylandは固定versionを同梱し、XCBが必要ならdistributionのQt pluginを探索する。
- XCB関連fileの同梱を減らせる。
- PackageのQt 6.11.2とsystem Qtのversion / build configurationが一致せず、plugin load失敗やABI問題の原因になる。
- `qt.conf`による自己完結配置と矛盾するため推奨しない。

#### D. Buildされた全QPA pluginをbundle

- Wayland、XCBに加えてoffscreen、minimal、EGLFS等も含める。
- CIや特殊環境でplatformを選びやすい。
- Desktop辞書に不要なembedded / headless backend、library、plugin metadataが増える。
- 未検証platformを配布物へ含め、support範囲を分かりにくくする。

D26でQtの自動選択を採用したため、一般的なLinux desktopで選択される2つのbackendを自己完結package内へ用意できるAを採用した。

実装規則:

- Releaseで固定した同一Qt 6.11.2 buildから、Wayland client用plugin一式と`libqxcb.so`をstageする。
- `minimal`、`offscreen`、`eglfs`、`linuxfb`等、desktop applicationに不要なQPA pluginは含めない。
- `qt.conf`の`Plugins`をpackage内の`plugins/`へ向け、system QtのQPA pluginをpackage構成として利用しない。
- QPA plugin本体だけでなく、そのQt libraryと非Qt shared-library dependencyをrelease build成果物から機械的に列挙する。
- 何をbundleし、何をsystem requirementとするかはbasenameだけで決めず、各releaseのELF dependency検査で確定する。
- Stagingはagentのsandbox内`/usr/lib`を根拠にせず、固定したrelease build image内のinstall prefixとdependency manifestを正とする。
- Clean testではsystem Qt pluginを探索できない環境でWayland起動を確認し、`QT_DEBUG_PLUGINS=1`の出力からpackage内pluginだけがloadされたことを検査する。
- XCBは少なくともstartup smoke testを行うが、当面はfallback / best effortであり、正式なX11 supportとは表記しない。
- Plugin load failure時に別versionのsystem Qt pluginへ自動fallbackする処理は追加しない。

### D28. Linux platform input context pluginのbundle範囲

**状態: 決定済み（2026-09-13）**  
**決定: A. Compose、IBus、Fcitx5をすべてbundle**

D26によりEJQuickは`QT_IM_MODULE`を上書きしないため、利用者環境が選択したinput methodに対応するpluginを自己完結package内へ用意する必要がある。
Qt 6.11.2のQt BaseはComposeとIBusのplatform input context pluginを持つが、Fcitx5 pluginは別projectの`fcitx5-qt`から提供される。
異なるsystem Qt向けにbuildされたinput context pluginをpackageのQtへ混在させない。

#### A. Compose、IBus、Fcitx5をすべてbundle（採用）

- Compose / IBus pluginはQPA pluginと同じQt 6.11.2 buildからstageする。
- Fcitx5 pluginはversionまたはcommitを固定した`fcitx5-qt`を同じQt 6.11.2に対してbuildし、必要なFcitx5 Qt DBus addon libraryとともにstageする。
- 利用者の`QT_IM_MODULE`とdesktop sessionに従い、確認済みのFcitx5構成と一般的なIBus構成の両方を利用できる。
- Fcitx5 / IBus daemon、日本語engine、設定toolは同梱せず、host desktop側のserviceを利用する。
- `Qt6DBus`、Fcitx5 addon、XKB関連dependencyと、Fcitx5側のlicense noticeがpackageへ増える。
- D26-Cの「利用者環境を尊重」と最も整合する。

#### B. Qt標準のComposeとIBusだけをbundle

- Qt Baseと同じbuildから得られる2 pluginだけを含め、外部のFcitx5 Qt componentを追加しない。
- Build provenance、license、更新管理をQt中心に保てる。
- `QT_IM_MODULE=fcitx`の環境ではpackage内に対応pluginがなく、確認済みのFcitx5日本語入力構成を正式に提供できない。
- System側Fcitx5 pluginを偶然loadできる構成はversion混在になるため、対応根拠にしない。

#### C. ComposeとFcitx5だけをbundle

- 確認済みFcitx5構成を正式対象にし、IBus pluginは含めない。
- Aよりcomponentを1つ減らせるが、外部`fcitx5-qt`の固定・build・noticeは必要なままである。
- GNOME等のIBus環境では日本語入力できない可能性があり、D26-Cでdesktop設定を尊重する範囲が狭くなる。

#### D. Composeだけをbundle

- Framework固有pluginとQt DBus dependencyを避け、packageを最小化する。
- Latin文字のCompose / dead key入力は扱えるが、Fcitx5 / IBusによる日本語IMEを配布物だけでは保証できない。
- 初期要件の日本語入力と、事前sampleで確認した構成を満たさないため推奨しない。

全案で、input method pluginの選択失敗を検知してEJQuick独自pluginへ切り替える処理は実装しない。
`QT_PLUGIN_PATH`等を利用者が明示してpackage外pluginを追加した構成はbest effortとし、release test対象へ含めない。

確認済みFcitx5構成を維持しつつ、IBusを使用するdesktopでも利用者の設定を上書きせず日本語入力を提供できるためAを採用した。

実装規則:

- `libcomposeplatforminputcontextplugin.so`と`libibusplatforminputcontextplugin.so`は、QPA pluginと同じ固定Qt 6.11.2 buildの成果物だけをstageする。
- `fcitx5-qt`はreleaseまたはcommitとsource checksumを固定し、release Qt 6.11.2に対してbuildした`libfcitx5platforminputcontextplugin.so`だけを使用する。
- Fcitx5 pluginがdynamic linkするFcitx5 Qt DBus addon等はdependency manifestへ記録し、package内へ含めるものは同じbuildからstageする。
- Fcitx5 / IBusのdaemon、input engine、辞書、設定applicationは同梱しない。Host sessionのuser serviceへ接続する。
- Pluginとaddonが利用するQt private APIを考慮し、Qt patch versionを変更する場合もFcitx5 pluginを再buildしてIME smoke testをやり直す。
- `qt.conf`からpackage内の`platforminputcontexts/`を探索し、system Qt向けpluginを暗黙にcopyまたはloadしない。
- License directoryへQt側pluginと`fcitx5-qt`、同梱addonのcopyright、license、対応source入手方法を収録する。
- Native Waylandで、Fcitx5とIBusそれぞれについて日本語preedit、候補確定、Escape、矢印、PageUp / PageDown、Enterを検証する。
- Compose pluginは代表的なCompose sequence / dead keyを検証する。
- 対応daemonが停止または未導入でもstartupがhangせず、通常の直接入力で検索できることを確認する。
- XCB上のIMEはstartupと基本入力をsmoke testするが、X11を正式対応にするまではbest effortとする。

### D29. Linux shared libraryのbundle境界

**状態: 決定済み（2026-09-13）**  
**決定: A. Host integrationを除外する明示allowlist方式**

Self-contained directoryは「system Qtを不要にする」ことを意味し、Linux kernel、glibc、display server、graphics driverまで私有copyに置き換えることは意味しない。
対象libraryは名前だけで決めず、固定release build imageで`ejquick-gui`、全QPA / input context plugin、同梱addonのELF dependency closureを取得して分類する。

#### A. Host integrationを除外する明示allowlist方式（採用）

- 使用するQt shared library、Qt plugin、Fcitx5 Qt addon等、applicationと同じversion管理が必要なcomponentを必ずbundleする。
- それ以外は原則system libraryとし、対応distribution間で必要SONAMEを保証できないportableなleaf dependencyだけをrelease manifestのallowlistへ追加する。
- 例えばQt buildがdynamic ICU等を要求する場合は、無条件にsystemへ委ねず、Qt build optionで依存をなくすか、検証後に一式をbundleする。
- glibc / ELF loader / NSS、compiler runtime、`libwayland-client`、X11 / XCB、DBus、XKB、fontconfig / FreeType、OpenGL / EGL / graphics driver stackはallowlistへ入れずsystem libraryを利用する。
- Host ABIと密接なlibraryの私有copyを避けながら、distribution間でSONAMEが変わるleaf dependencyだけを補える。
- Allowlist追加にはlicense、security update、RPATH、symbol version、全targetでのclean testが必要になる。

#### B. Qtとapplication所有componentだけをbundle

- Qt shared library、Qt plugin、Fcitx5 Qt addonだけを含め、その他のdependencyはすべてsystemへ委ねる。
- Packageと更新対象を最小化でき、host serviceやgraphics stackとの整合性が高い。
- RHEL 9系相当でbuildしても、別distributionで同じSONAMEのleaf libraryが提供されるとは限らない。
- Qt build optionを厳しく制限しない限り、展開先ごとの不足libraryが増える可能性がある。

#### C. glibcとgraphics stack以外を原則bundle

- ELF dependency closureからglibc / loader、OpenGL / EGL / driverだけを除外し、その他を広く同梱する。
- Target systemのinstalled packageへの依存を最も減らせる。
- X11 libraryは実行時にextension libraryをloadする場合があり、Qt公式deployment文書も通常はdynamic system X11 libraryの利用を勧めている。
- DBus、font、Wayland / XCB等のhost integrationでABI不整合を起こす範囲と、package size、更新責任が大きい。

#### D. Qtを含め全shared libraryをsystem提供にする

- Distribution package managerがQtと全dependencyを管理する。
- Release archiveを小さくでき、security updateをOSへ委ねられる。
- DistributionごとにQt versionが異なり、Qt 6.11.2固定、MIQT互換性、自己完結`tar.zst`というD6 / D7の決定と矛盾する。
- 採用する場合はnative DEB / RPM等へ配布方式を変更する必要がある。

Host sessionやgraphics driverと同じABIを使いながら、distribution間で保証できないleaf dependencyだけを補えるためAを採用した。

実装規則:

- Release buildは、Qt library / pluginとFcitx5 Qt plugin / addonを必須bundle集合として明示する。Directory全体のglob copyは行わない。
- 非Qt libraryを追加するallowlistはfile名、SONAME、由来、version、license、追加理由をrelease manifestへ記録する。
- glibc family、ELF interpreter、NSS / resolver、`libstdc++.so.6`、`libgcc_s.so.1`はbundleしない。RHEL 9系相当のcompiler runtimeを最低要件とする。
- `libwayland-*`、X11 / XCB、DBus、XKB、fontconfig / FreeType、OpenGL / EGL / GLX、DRM / GBM、graphics driverとvendor libraryはbundleしない。
- Host integration libraryが別libraryを`dlopen`する場合も、そのmoduleをpackageから注入しない。
- 追加のportable leaf dependencyは、全targetでsystem提供を期待できず、host ABIと独立し、private copyでclean testを通る場合だけallowlistへ入れる。
- Qt build optionで不要なdynamic dependencyを削減できる場合は、libraryを追加する前にsize、license、機能への影響を比較する。
- Executableと同梱shared library / pluginには、それぞれの配置から`lib/`を解決できる相対`DT_RUNPATH`を設定する。
- Globalな`LD_LIBRARY_PATH`を設定するlauncherは使わず、Builderや`QDesktopServices`から起動される外部applicationへprivate library pathを継承させない。
- Release checkは`readelf`等で全`DT_NEEDED` closureと解決元を記録し、意図しないbuild-host path、未分類SONAME、欠落libraryをerrorにする。
- RHEL 9系相当、Ubuntu 22.04 / 24.04、Debian 12のclean environmentで起動、Wayland、IME、font、Builder、外部log viewerを検証する。

### D30. Linux packageの起動方法とdesktop integration

**状態: 決定済み（2026-09-13）**  
**決定: A. Binaryを直接起動し、任意のper-user登録scriptを提供**

D6の`tar.zst`は任意directoryへ展開可能とする。
Freedesktop desktop entryの`Exec`はpackage rootからの相対pathを安全に表現できないため、menu登録時には実際の絶対pathを確定する必要がある。

#### A. Binaryを直接起動し、任意のper-user登録scriptを提供（採用）

- 基本起動方法は展開先の`bin/ejquick-gui`を直接実行する。Shell launcherと環境変数設定を必須にしない。
- Archiveに任意実行のdesktop登録 / 解除scriptを含めるが、展開時や初回起動時に自動実行しない。
- 登録scriptは現在のpackage rootを絶対pathにし、`$XDG_DATA_HOME/applications`へ生成したdesktop entryを配置する。
- System-wide pathへ書き込まず、root権限を要求しない。
- Package directoryを移動した場合は再登録が必要になる。
- Relocatable package、direct debugging、desktop menuの利便性を両立できる。

#### B. Top-level shell launcherを正式入口にする

- Archive rootの`EJQuick`または`ejquick-gui` scriptが自身のpathを解決し、内部binaryを起動する。
- 利用者が`bin/`を意識せず起動でき、将来layoutを変えてもscript内で吸収できる。
- Shell、symlink解決、特殊文字を含むpath、exit / signal forwardingのtestが増える。
- D29で相対RUNPATHを使うため、runtime環境を整えるwrapperとしての必要性はない。

#### C. Per-user install scriptで固定locationへcopy

- Install scriptがpackage全体を例えば`~/.local/opt/ejquick`へcopyし、desktop entryとcommand symlinkも作成する。
- Desktop entryのpathが安定し、展開元を移動・削除できる。
- Upgradeのatomicity、既存versionの置換、uninstall、custom prefix、disk容量を管理するinstaller実装が必要になる。
- 「任意directoryへ展開して利用」という`tar.zst`の単純さが弱くなる。

#### D. Direct binaryだけを提供し、desktop登録は利用者に任せる

- `bin/ejquick-gui`とmetadata templateだけを配布し、install / unregister scriptを用意しない。
- Package側のscriptとfilesystem変更を最小化できる。
- 利用者がabsolute `Exec`、icon path、desktop entry配置を手作業で設定する必要がある。

Desktop entryを提供する案では、basename / application IDを`io.github.simosako.ejquick`、表示名を`EJQuick`、`Terminal=false`とする。
GUIはQtへ同じdesktop file nameを設定し、Wayland compositor上のapplication IDとdesktop entryを一致させる。
初期版はfile / URL argument、MIME association、DBus activation、自動起動を登録しない。

Runtime環境を変更しない直接起動を正としながら、希望する利用者だけがroot権限なしでdesktop menuへ登録できるためAを採用した。

実装規則:

- `bin/ejquick-gui`はcurrent working directoryに依存せず、相対`DT_RUNPATH`、`qt.conf`、実行file自身の絶対pathからruntimeと`ejquick-build`を解決する。
- Archive rootへ`install-desktop.sh`と`uninstall-desktop.sh`を置く。通常起動や展開だけでは実行しない。
- Scriptはpackageをcopy / moveせず、shell profile、`PATH`、`LD_LIBRARY_PATH`、Qt関連環境変数を変更しない。
- `$XDG_DATA_HOME`が空なら`$HOME/.local/share`を使い、desktop entryを`applications/io.github.simosako.ejquick.desktop`へinstallする。
- Desktop entryの`Icon`にはFreedesktop標準のgeneric application icon名`accessories-dictionary`を指定し、application固有iconをinstallしない。
- Generated desktop entryの`Exec`と`TryExec`には、登録時の`bin/ejquick-gui`絶対pathをDesktop Entry Specificationに従ってescapeして記録する。
- Space、quote、backslash、non-ASCIIを含む展開pathをtestし、安全に表現できない改行等を含むpathでは登録を拒否する。
- Desktop entryはtemporary fileからatomicに置換し、解除時はこのapplication IDに属するper-user desktop entryだけを削除する。
- `desktop-file-validate`とdesktop database更新toolが存在すれば利用するが、任意toolの不在だけで登録済みfileを失敗扱いにしない。
- GUIは`QGuiApplication::setDesktopFileName("io.github.simosako.ejquick")`相当を設定し、organization / application metadataとD16の`QSettings` keyも固定する。
- Package移動後に古いdesktop entryから起動できない場合は、移動先で登録scriptを再実行する手順をREADMEへ記載する。
- System-wide install、command symlink、MIME association、auto-start、DBus activationは初期scriptの責務に含めない。

### D31. GUI processのmultiple-instance policy

**状態: 決定済み（2026-09-13）**  
**決定: A. 独立した複数processを許可**

各GUI processが持つmain windowは1つとし、本項では同じuserが`ejquick-gui`を複数回起動した場合のprocess間動作を決める。
辞書DBはread-only検索するため複数readerを許容できるが、Builderによる同じoutput DBへの同時writeは別途process間で保護する必要がある。

#### A. 独立した複数processを許可（採用）

- 二重起動を検出せず、起動ごとに独立したmain window、query、選択、DB connectionを持つ。
- 異なるqueryやconfigを並べて参照でき、local IPC、primary election、stale endpoint、activation protocolが不要になる。
- QtNetwork等のsingle-instance専用dependencyを追加せず、起動pathを単純に保てる。
- 複数起動した時だけQt runtimeとDB connectionのmemoryがprocess数分増える。
- 一方でDBを再buildしても他processは自動reloadされず、各processを再起動するまで旧DBを参照する場合がある。

#### B. Userごとに常に1 process

- 2回目の起動はlocal IPCで既存processへactivate requestを送り、成功後すぐ終了する。
- 重複memoryを防ぎ、desktop applicationとして1つのwindowへ戻りやすい。
- 既存processと異なる`--config`を指定した場合に無視するかerrorにする必要がある。
- Waylandではfocus stealing防止により、既存windowの`raise` / activate requestが必ず成功するとは限らない。

#### C. Canonical config pathごとに1 process

- Default configと、異なる`--config`ごとに別instanceを許可し、同じcanonical pathの2回目だけ既存windowへ転送する。
- Custom configの利用と重複memory抑制を両立できる。
- Config pathのcanonicalization、未作成file、symlink、case sensitivity、hashしたserver名、user限定IPCを設計する必要がある。
- `QLocalServer`等の追加moduleと、crash後のstale socket / named pipe回収testが必要になる。

#### D. 1 process内で複数main windowを管理

- 2回目の起動requestを既存processへ送り、新しいmain windowを同じprocess内に作る。
- Qt runtimeとsearch infrastructureを一部共有しながら複数queryを並べられる。
- D3 / D4の1 main window前提を変更し、windowごとのcontroller、DB lifetime、macOS menu、Quit、Builder modal parentを再設計する必要がある。

全案で、`QSettings`にはqueryやentry本文を保存しない。
複数processを許可する案ではgeometry / splitterの最終保存値は最後に正常終了したprocessの値となり、logは1行を1 writeとしてappendする。

通常は1 processだけを利用しつつ、必要な場合は複数queryやconfigを並べられ、single-instance専用dependencyとWayland activation問題を避けられるためAを採用した。

実装規則:

- Startup lock、PID file、local socket、DBus service等による二重起動検出を追加しない。
- 各processはconfig、dictionary service、query / result state、Builder dialog、cancel contextを共有せず独立して所有する。
- Desktop application IDは同じままとし、compositorやtask switcherが複数windowを同じapplicationとしてgroup化することは許容する。
- `QSettings`へ保存するgeometry / splitterは最後に正常終了してsyncしたprocessの値を次回defaultとして使う。Process間でlive同期しない。
- Loggingは1 log recordを組み立てて1回のappend writeで出し、同じlogを利用するprocess間で行を意図的に分割しない。
- あるprocessがDBを置換しても、他processのsearch serviceを通知、close、reloadしない。他processは保持中の旧DBを終了まで読み続ける場合がある。
- DBを開き直したい利用者には対象processの再起動を案内し、自動file watcherは初期版へ追加しない。
- 複数Builderによる同一outputの競合はD32のwriter lockで防ぎ、single-instance化を排他制御の代用にしない。

### D32. Builder output DBのprocess間lock

**状態: 決定済み（2026-09-13）**  
**決定: A. OS advisory lockを非blockingで取得**

LockはGUI childだけでなく、terminalから起動した`ejquick-build`と将来のfrontendを含む全writerに適用する。
Search processはread-onlyのためwriter lockを取得せず、既存DBを開いているreaderを強制終了しない。

#### A. OS advisory lockを非blockingで取得（採用）

- Output DBごとに同じdirectoryの永続lock fileを開き、Unixでは`flock`相当、Windowsでは`LockFileEx`相当のexclusive lockをprocess終了まで保持する。
- Lockを直ちに取得できなければbuildを開始せず、`Another dictionary database build is already running.`というtyped errorを返す。
- OSがprocess crash時にlockを解放するため、PIDや時刻からstale ownerを推測してlock fileを削除する必要がない。
- Lock file自体は解放後も残す。実行中にunlinkして別inodeへ分裂するraceを避けられる。
- CLIが待ち続けず、GUIもmodal dialog内で明確に再試行を案内できる。

#### B. OS advisory lockが空くまで待つ

- Aと同じlockをblockingまたはpollingで取得し、先行build終了後に開始する。
- 利用者が手動で再実行しなくても順番に処理できる。
- 数分以上待った後に同じDBを再度buildするため、CPU / diskを無駄にしやすい。
- CLIが無期限に停止して見え、GUIにはwaiting state、cancel、owner表示が必要になる。

#### C. `O_EXCL` lock fileとPID metadata

- Atomic createに成功したprocessをownerとし、PID、開始時刻、output pathをlock fileへ記録する。
- Go標準file API中心で実装でき、lock内容を利用者へ表示しやすい。
- Crash後にfileが残るため、process存在確認、PID再利用、host reboot、古いfileの回収規則が必要になる。
- Stale判定を誤るとactive buildのlockを奪う可能性がある。

#### D. Lockを設けずtemporary DBとatomic renameだけに任せる

- 各Builderは別temporary fileへbuildでき、途中成果物は衝突しない。
- 最後にpublishしたprocessが勝つため、先に成功したDBが直後に別buildで置換される。
- `--force`確認後にoutput状態が変わるTOCTOU raceがあり、利用者の意図したsource / dictionary versionを保証できない。
- Disk、CPU、cleanup eventも重複するため推奨しない。

全案で、lock取得前にoutput parent directoryを作成し、lock取得後に既存output、`--force`、inputとの同一性を再検査する。
別processのreaderが原因でWindowsのpublishに失敗した場合は旧DBを保持して通常のbuild failureとし、readerをkillしたりlockを奪ったりしない。

Crash後のstale owner判定を不要にし、GUI / CLIを待機させず競合を明示できるためAを採用した。

実装規則:

- Lock取得処理は`cmd/ejquick-build`やGUIではなく`internal/builder.RunContext`の共通build pathへ置き、直接APIを利用する将来のcallerも迂回できないようにする。
- Linux / macOSは専用build-tag fileからnonblocking exclusive `flock`相当、Windowsは`LockFileEx`相当を呼ぶ。
- Lock機能のためにCGOを追加せず、既存Pure Goの`ejquick-build`配布方針を維持する。
- Lock handleはtemporary DB作成前からpublish、final stat、cleanup完了後まで保持し、成功 / failure / cancel / panic回収可能な通常経路で必ずcloseする。
- `--force`は既存outputの置換許可だけを表し、busy lockを無視するoptionにはしない。
- Busyはsentinel / typed errorとして通常build failureと区別できるようにし、machine protocolではstable code `output_busy`を送る。
- GUIは`output_busy`時にsource path等をlogへ出さず、別のDB作成完了後に再度Buildを押すようSetup pageで案内する。
- 通常CLIは待機やretryをせず、短いerrorをstderrへ出して既存のerror exit codeで終了する。
- Advisory lock APIがunsupported、権限不足、I/O errorの場合は排他なしで続行せずbuild failureにする。
- Integration testでは2 processの同一output競合、異なるoutputの並行build、ownerの正常終了 / hard kill後の再取得を検証する。

### D33. Builder lock fileのpathと安全性

**状態: 決定済み（2026-09-13）**  
**決定: A. Output DBと同じdirectoryのhidden sibling**

Lock fileは辞書内容を含まず、排他対象を識別するためだけに使う。
Lock中のfileをunlinkするとUnixでは別inodeを開いたprocessが同時にlockできるため、active / inactiveにかかわらず自動削除しない設計を優先する。

#### A. Output DBと同じdirectoryのhidden sibling（採用）

- Output `/data/eiwa.sqlite3`に対し、`/data/.eiwa.sqlite3.ejquick-build.lock`のような決定的pathを使う。
- Output directoryを共有する全user / config / frontendが同じlock inodeへ到達し、removable mediaとDBを移動する場合もlock namespaceが対応する。
- Lock fileは初回build後も小さなregular fileとして残る。
- Output directoryにwrite権限があっても、既存lock fileのowner / modeが不適切ならbuildできない場合がある。

#### B. Application state directoryにcanonical output pathのhashを置く

- `$XDG_STATE_HOME/ejquick/locks/<hash>`等を使い、DB directoryへlock fileを残さない。
- User単位のstate directoryなので権限を管理しやすい。
- 同じDBを別user、container、異なるstate directoryからbuildするとlockを共有できない。
- Symlink、mount、case foldingを含むcanonical path hashの互換性を固定する必要がある。

#### C. Final output DB file自体をlock

- 追加lock fileを作らず、既存DB handleへexclusive lockを取る。
- 初回buildではoutputが存在せず、placeholderを作ると`--force`判定とatomic publishを変えてしまう。
- Publishのrenameでinodeが入れ替わり、lock対象とfinal outputが一致しなくなるため利用できない。

#### D. OS runtime / temporary directoryにhash lockを置く

- Linuxでは`$XDG_RUNTIME_DIR`等のsession用directoryを利用し、logout / reboot時にfileが回収される。
- Output directoryを汚さず、per-user permissionを利用できる。
- Runtime directoryが異なるsession、別user、service、Windows / macOSとのpath規則が分岐する。
- DBとlock namespaceが離れ、同じoutputに対する全writerが同じfileへ到達する保証が弱い。

全案でoutput pathはabsolute / cleanにし、存在するparent directoryのsymlinkを解決してからlock identityを決める。
Lock pathの最終要素がsymlink / reparse pointまたはregular file以外なら、安全のためbuildを開始しない。

DBと同じfilesystem namespaceで全writerが同じlockへ到達し、process crash後もowner metadataのstale判定をせず再利用できるためAを採用した。

実装規則:

- Output parentを作成した後にabsolute / clean化し、parentのsymlink / junctionを解決したpathをbuild中のcanonical outputとして固定する。
- Lock basenameは`.` + output basename + `.ejquick-build.lock`とし、default DBでは`.eiwa.sqlite3.ejquick-build.lock` / `.waei.sqlite3.ejquick-build.lock`となる。
- 新規lock fileはUnixではmode `0600`、Windowsではcurrent userの通常ACLで作り、内容はbyte-range lockに必要な固定marker 1 byteだけとする。
- PID、source TXT path、output path、dictionary type、時刻等のmetadataは書き込まない。
- Unixではparent directory handleに対する`openat`相当とno-follow flagを使い、open後のfileがregular fileであることを`fstat`相当で再確認する。
- Windowsではreparse pointを追跡しないopen方法とfile attribute検査を用い、directoryやdeviceをlock対象にしない。
- 既存lock fileのpermission / ownerをGUIが自動修正せず、安全にopenできなければpathを示してbuild failureにする。
- Lock fileはsuccess、failure、cancel、hard kill後もunlinkしない。`--force`、GUI cleanup、起動時scanの削除対象にも含めない。
- Final outputがsymlink / reparse pointまたはregular file以外の場合も、意図しないtargetやspecial fileを置換しないようbuildを拒否する。
- Lock filenameがfilesystemのname / path上限を超える場合は別namespaceへfallbackせず明示errorにする。
- Advisory lockを正しく提供しないnetwork filesystem等は正式対応外とし、lock取得またはintegration testに失敗するfilesystemではbuildしない。
- Testはsymlink lock、non-regular lock、不正permission、長いbasename、parent symlink、lock fileの永続再利用、manual unlinkを行わない通常cleanupを含める。

### D34. 初期Linux releaseの機能scope

**状態: 決定済み（2026-09-13）**  
**決定: A. 検索GUI、Builder UI、self-contained packageを1つの初期releaseに含める**

初期releaseはLinux `x86_64` / glibc 2.34以降 / native Waylandを正式対象とし、XCB / XWaylandはD27どおりbest effortとする。
辞書TXTと生成済みDBは、どの案でもrepository、test fixture、package、releaseへ含めない。

#### A. 検索GUI、Builder UI、self-contained packageを1つの初期releaseに含める（採用）

- D1〜D33で決定したmain window、IME / keyboard、logging / settings、Builder dialog / protocol / cancel / lock、Linux packageをすべて初期release scopeにする。
- TUI / CLI / Builderの既存機能と互換性を維持し、GUIだけで購入済みTXTからDB作成と検索まで完了できる。
- System tray、global shortcut、検索履歴、自動update、独自theme、X11正式対応は含めない。
- 利用者が初回から完結して使える一方、release前にBuilder failure testとdeployment / license検査まで必要になる。

#### B. 内部milestoneを分け、最初の公開releaseまでにAへ揃える

- Prototype、検索GUI、Builder integration、packagingを順番に完成させ、途中成果物はdeveloper向けsnapshotだけにする。
- 公開する初期releaseの最終scopeはAと同じで、実装とreviewを小さく分割できる。
- 「初期releaseに何を含めるか」という利用者向け結論はAと同じであり、違いはproject管理方法だけになる。
- Snapshotのsupport有無とartifact識別を明確にする必要がある。

#### C. 検索GUIだけを先にreleaseし、DB作成はCLIに限定

- Main windowとpackageを先に安定させ、GUI Builderは次releaseへ延期する。
- Child protocol、cancel、hard kill、lock UIの実装を初回releaseから外せる。
- DB未作成時にterminalで`ejquick-build`を使う必要があり、desktop GUIとして初回導線が完結しない。
- D5 / D20〜D23 / D32〜D33は後続release用設計として残る。

#### D. Convenience機能も初期releaseへ追加

- Aに加えてsystem tray、global shortcut、検索履歴、自動update、theme設定等から複数を含める。
- Desktop applicationとしての機能を初回から増やせる。
- 起動速度、memory、privacy、platform差、設定migration、release範囲が増え、軽量な初期版という目的から外れる。

Terminalを使わず初回DB作成から検索まで完結することを、desktop GUIとしての最初の実用単位とするためAを採用した。

実装規則:

- 開発中は最小Qt window、検索GUI、Builder protocol、Builder dialog、package stagingの順に内部milestoneを分けてよいが、途中snapshotを安定releaseとして扱わない。
- 初期Linux GUI artifactはD6のdirectory構成を満たし、`ejquick-gui`と同じproduct versionの`ejquick-build`を含める。
- 既存のPure Go `ejquick` / `ejquick-build` artifactとCLI contractを維持し、GUI追加を理由に既存platformのreleaseを停止しない。
- GUIから英和 / 和英それぞれについて、DBなしの起動、作成成功、cancel、protocol error、output busy、旧DB置換、再open failureを検証する。
- Native Wayland、Fcitx5 / IBus、日本語表示、keyboard、clipboard、accessibility、external log viewer、desktop登録をrelease smoke testへ含める。
- XCB / XWaylandはpackageへ含めてstartup smoke testを行うが、初期release noteの正式対応環境には記載しない。
- 起動時間、RSS / PSS、検索latency、archive / 展開sizeを同じreference環境で記録するが、D8どおり固定値のrelease gateにはしない。
- 辞書TXT、生成DB、検索語、entry本文をrelease artifact、screenshot、test report、CI artifact、log sampleへ含めない。
- Windows / macOSのGUI binaryとinstallerは初期Linux releaseの完了条件に含めない。

### D35. GUI sourceと自動testの分離

**状態: 決定済み（2026-09-13）**  
**決定: A. Build tagでQt依存を分離し、3層testにする**

既存の`make test`、`make test-race`、`go vet ./...`、`CGO_ENABLED=0` cross-buildは、Qt SDKを導入していない環境でもTUI / CLI向けに維持する。
GUI testで使う辞書sourceとDBは購入データから作らず、人工的な小規模dataをtestごとに生成する。

#### A. Build tagでQt依存を分離し、3層testにする（採用）

- Qt / MIQT依存sourceを`gui` build tagの対象にし、通常のPure Go testから除外する。
- Widget非依存のcontroller / state transitionはPure Go packageへ分け、既存のdefault testでfake search / builder adapterを使って検証する。
- `make test-gui`では`-tags gui`と固定Qt SDKを使い、単一`QApplication`上でwidget、signal、focus、model、clipboard、dialog stateを検証する。
- 別のpackage smoke testで完成packageをnative Wayland compositor上に起動し、plugin、IME、desktop metadata、child Builderをend-to-endで確認する。
- Default CIを軽量に保ちつつ、logic、Qt integration、deploymentのfailure位置を分離できる。

#### B. Qt依存を同じmoduleで常時build / testする

- Build tagを使わず、`go test ./...`ですべてのGUI packageもcompileしてtestする。
- Source構成と実行commandが単純で、tag漏れを防ぎやすい。
- Linux / Windows / macOSの既存test jobすべてにQt SDK、C++ compiler、CGO環境が必要になる。
- `CGO_ENABLED=0`の品質確認とcross-buildからGUI packageを別途除外する必要がある。

#### C. GUIをnested Go moduleへ分離

- GUI専用`go.mod`でMIQTとQt-dependent testを管理し、root moduleをPure Goのまま保つ。
- Dependency graphとCI commandを明確に分離できる。
- D25の`cmd/ejquick-gui`配置、rootの`internal` package import、version ldflags、local replace、release source treeが複雑になる。
- Module間の同時変更とdependency更新を2つの`go.mod` / `go.sum`で管理する必要がある。

#### D. Pure Go controller testとmanual GUI smoke testだけにする

- Qt-dependent automated testを作らず、release前に実機で操作確認する。
- GUI test infrastructureとheadless compositorを用意せずに済む。
- Signal接続、focus、IME preedit、dialog lifecycle、main-thread dispatch、plugin stagingの回帰を自動検出できない。
- Cross-platform frontendの継続保守には不足する。

Aの3層は次の責務に分ける。

1. Pure Go unit test: controller state、request ID、cancel、selection、Builder protocol parser
2. Qt widget integration test: widget property、action、signal、focus、model更新、dialog transition
3. Packaged Wayland test: QPA / input context load、実process I/O、runtime dependency、IME smoke test

Pixel単位のscreenshot goldenを主要assertionにせず、visible text、model row、selection、enabled state、focus、accessible property、protocol eventを検査する。

既存frontendのPure Go build / testを維持しながら、Qt固有問題と完成packageの問題も別の層で自動検出できるためAを採用した。

実装規則:

- `cmd/ejquick-gui`とMIQTをimportするsource fileには`//go:build gui`を付け、release GUIは`go build -tags gui`相当で作成する。
- Build tagでUI behaviorを切り替えず、Qt依存packageをdefault package graphから除外する目的だけに使う。
- Controller、request state、selection、Builder protocol / process stateはMIQTをimportしないsub-packageへ置き、`go test ./...`と`go test -race ./...`で検証する。
- Existing `make test`、`make test-race`、`make vet`、cross-build targetはQt / C++ compiler / CGOを要求しないまま維持する。
- `make build-gui`、`make test-gui`、`make vet-gui`、`make package-gui-linux`を別targetとして追加し、固定Qt prefix、`CGO_ENABLED=1`、`-tags gui`を明示する。
- Qt testは1 processにつき1つの`QApplication`だけをmain OS threadで生成し、widget testをparallel実行しない。
- Worker完了待ちは固定sleepではなくsignal / channelとdeadlineを使い、timeout時にはpending stateとlogを出す。
- Test終了時はpending search / Builderをcancelし、Qt objectをmain threadで破棄してから`QApplication`を終了する。
- Widget testはfake search / builder adapterを基本とし、real SQLiteを使うtestでは人工的な小規模sourceからDBを実行時生成する。
- Local verificationのbinary、DB、config、runtime directory、reportはproject-local `tmp/`以下へ置き、repositoryへ追加しない。
- Qt widget test用のQt Test libraryや補助pluginを利用しても、production packageのdependency集合には自動追加しない。
- Packaged testは実際にstageしたQt library / pluginだけを使い、build machineのQt pathへfallbackした場合はfailureにする。
- Actual Fcitx5 / IBus daemonとのcandidate操作はreal desktop smoke testでも確認し、synthetic `QInputMethodEvent`だけをIME互換性の根拠にしない。

### D36. Wayland GUI test用compositorとCI環境

**状態: 決定済み（2026-09-13）**  
**決定: A. Weston headless自動test + KWin / Mutter実環境smoke test**

Waylandでは通常のapplicationが任意のglobal input injectionを行えないため、headless CIだけで実desktopのIME candidate windowとfocus policyを完全に再現できるとは扱わない。
Automated testとreal desktop smoke testの責務を明示的に分ける。

#### A. Weston headless自動test + KWin / Mutter実環境smoke test（採用）

- Dedicated Linux GUI jobでversion固定したWestonをheadless backendとして起動し、Qt widget testとpackage startup testをnative Waylandで実行する。
- CIではQPA plugin load、window生成、focus / shortcutのsynthetic event、model更新、clipboard、dialog、Builder child、終了処理を自動検証する。
- Release前にKDE Plasma / KWin + Fcitx5とGNOME / Mutter + IBusの実desktopで、日本語preedit、candidate操作、clipboard、file dialogを手動smoke testする。
- Pull requestごとの再現性と、実compositor / IMEでしか確認できない項目を両立する。
- Real desktop smoke testは自動CIではないため、checklistと実施記録が必要になる。

#### B. Weston headlessだけを正式test環境にする

- Automated jobだけでrelease判定を完結し、実機checklistを要求しない。
- CIの再現性と保守性が最も高い。
- KWin / Mutter固有のactivation、decoration、clipboard、Fcitx5 / IBus candidate UIを見逃す可能性がある。
- 事前sampleで確認した実IME動作を継続的なrelease確認へ引き継げない。

#### C. Nested KWinとMutterをCI matrixで自動起動

- 2つの正式対象desktop compositorとIME serviceをCI内で起動し、より実環境に近いtestを自動化する。
- Desktop固有の問題をpull requestごとに検出できる可能性がある。
- Session DBus、portal、font、IME daemon、virtual input、GPU / software renderingの構築が重く、hosted CIではflakyになりやすい。
- Container / runner imageとdesktop stackのsecurity update管理も増える。

#### D. `offscreen` / `minimal` QPAだけで自動test

- Compositorを起動せずQt widget logicをtestでき、最も高速である。
- Wayland QPA plugin、socket、protocol、scale、clipboard、input contextのdeploymentを検証できない。
- D27ではproduction packageにoffscreen / minimal pluginを含めないため、release artifactそのもののtestにもならない。

どの案でも、購入辞書をCI secretやartifactとして使用せず、test windowのtitle / text / screenshotには人工dataだけを使う。

Pull requestごとのnative Wayland回帰を再現可能に検出しつつ、headless compositorでは代替できない実IME操作をrelease前に確認できるためAを採用した。

実装規則:

- GUI CI jobはversionとimage digestを固定した専用Linux containerで実行し、WestonとQt / MIQT build環境をdefault Pure Go jobへ追加しない。
- `XDG_RUNTIME_DIR`はproject-local `tmp/`以下へ作成してmode `0700`とし、Weston socket、log、test config、人工DBも同じtest workspace以下へ置く。
- Westonのheadless backendをsoftware renderingで起動し、socketのready確認をdeadline付きで行ってからtestを開始する。
- Automated Wayland jobでは`QT_QPA_PLATFORM=wayland`を明示し、`DISPLAY`をunsetしてXCB / XWaylandへの意図しないfallbackを許可しない。
- `QT_DEBUG_PLUGINS=1`の診断を保存し、Wayland QPAと対象input contextがstage済みpackage pathからloadされたことをassertする。
- Compose、Fcitx5、IBusの各pluginを選択したstartup testを行い、daemonがないcaseでもstartup / shutdownがhangしないことを確認する。
- Synthetic key / input method eventはwidget behaviorの自動testに使用できるが、実Fcitx5 / IBusとの互換性判定には使わない。
- Packaged smoke testはsystem Qt plugin pathを見えなくしたclean environmentで実行し、起動、人工DB検索、Builder child起動、graceful終了をdeadline付きで確認する。
- CI failure時はWeston / Qt plugin / EJQuick logをartifactにできるが、人工dataだけを使い、window screenshotとclipboard内容は既定で保存しない。
- Release checklistでは少なくともKDE Plasma / KWin + Fcitx5とGNOME / Mutter + IBusを記録し、compositor、IME、distribution、Qt / MIQT versionを残す。
- 実環境ではpreedit表示、candidate選択、確定 / cancel、D13 keymapとの競合、clipboard、file dialog、scale変更、close / reopenを確認する。

### D37. Go workerからQt main threadへのdispatch

**状態: 決定済み（2026-09-13）**  
**決定: A. MIQT `mainthread.Start`による非同期dispatch**

Qt widget、model、clipboard、dialog、`QAction`はQt main threadだけで操作する。
Search workerとBuilder stdout / stderr readerはQt objectを保持せず、結果をimmutableなGo valueとしてUI controllerへ渡す。

#### A. MIQT `mainthread.Start`による非同期dispatch（採用）

- Workerはtyped UI eventを作り、`mainthread.Start(func() { controller.apply(event) })`相当でQt event queueへpostする。
- MIQT v0.14.0の既存helperが`QMetaObject::invokeMethod`のqueued connectionを使用するため、独自C++ bridgeを追加しない。
- WorkerはUI処理完了を待たず、cancelやshutdown中の相互待ちを避けられる。
- Request ID、Builder generation、closing flagをmain threadで検査し、stale eventを破棄する必要がある。
- Event loop終了前にworker停止とqueued callbackのdrainを完了しないと、未実行closureとQt object lifetimeが問題になる。

#### B. MIQT `mainthread.Wait`による同期dispatch

- Workerはmain threadでcallbackが完了するまでblockし、UI反映後に次の処理へ進む。
- Naturalなbackpressureがあり、testで反映完了を待ちやすい。
- Main threadがworker終了を待つ、event loopが停止する、modal shutdownへ入る等の状況でdeadlockしやすい。
- 高頻度progressや検索完了がmain threadの応答時間にworker lifetimeを結合する。

#### C. Buffered Go channelを`QTimer`でdrain

- Workerはbounded channelへeventを送り、main thread上のtimer callbackが一定間隔でまとめて処理する。
- Queue size、drop / coalesce、backpressureをGo側で明示できる。
- Idle時もtimer wakeupが発生するか、開始 / 停止管理が必要になり、結果反映にpoll interval分のlatencyが加わる。
- Qtが既に持つqueued invocationと役割が重複する。

#### D. Custom `QObject` signal / slot bridge

- Go eventをQObject signalとしてemitし、queued connectionでcontroller slotへ届ける。
- Qt object lifetimeとevent queueへ自然に統合できる。
- Typed payloadのC++ wrapper、MIQT binding追加、ownership、signal signatureを保守する必要がある。
- 現行MIQT helperで足りる1方向通知には過剰である。

全案で、異なるworkerからの到着順に依存せずrequest / generation IDで正当性を判定する。
Callback内のpanicをCGo境界へ出さず、boundaryでrecoverしてfatal logと安全なshutdownへ移行する。

MIQTが提供するQt queued invocationをそのまま利用でき、workerとmain threadの同期waitを避けられるためAを採用した。

実装規則:

- Qt非依存層には`Post(UIEvent) bool`相当の小さなdispatcher interfaceを定義し、Qt adapterだけが`mainthread.Start`をimportする。
- `UIEvent`はsearch result / error、Builder ready / progress / terminal、shutdown等のclosedなtyped valueとし、workerから任意closureやQt pointerを渡さない。
- Event payloadはpost後にworkerが変更しない。Slice / byte列を共有する場合はownershipをeventへ移すか必要なcopyを作る。
- Qt adapterは`mainthread.Start` callback内でmain-thread controllerの単一`apply`入口を呼び、widgetごとにworker callbackを作らない。
- `apply`はrequest ID、dictionary generation、Builder process generation、application closing stateを検査してからQt objectを更新する。
- 異なるgoroutineからpostされたevent間のFIFOを仮定せず、IDと明示state transitionだけで正当性を決める。
- Dispatcherはaccepting / closing stateとpending callback数を管理し、shutdown開始後の通常eventを拒否できるようにする。
- Rejected eventはworker側でblockせず破棄し、resultやbufferがGo GCで回収できる状態にする。
- Callback開始時とtest buildでは`mainthread.IsCurrent()`相当を検査し、main thread以外で`apply`された場合はtest failure / fatal errorにする。
- Callback boundaryでpanicをrecoverし、可能ならfatal内容をlogへ記録してD38のshutdownを開始する。Panicをexported CGo callbackの外へ伝播させない。
- Production workerから`mainthread.Wait` / `Wait2` / `Wait3`を呼ばない。同期barrierが必要なtest helperもevent loop停止後には使用しない。
- `QApplication`生成前または破棄後に`mainthread.Start`を呼ばないことをlifecycle stateとtestで保証する。
- Search resultは最大500件、Builder progressは低頻度eventに制限し、stderr byte chunkごとにQt callbackをpostしない。

### D38. GUI shutdown時のworker / queued event drain

**状態: 決定済み（2026-09-13）**  
**決定: A. Event loopを維持する2段階の非同期shutdown**

Linux / Windowsのlast-window closeと全OSのQuitはapplication shutdownを開始する。
macOSの通常のwindow closeはprocess shutdownではなく、明示Quitだけが本項のapplication shutdownを開始する。

#### A. Event loopを維持する2段階の非同期shutdown（採用）

1. Main threadでclosing stateへ移行し、新規検索 / Builder / dispatchを停止してactive contextをcancelする。
2. Qt event loopを動かしたままworker終了、Builder cancel / hard kill、queued callback drainを待ち、最後にmain threadでDB / widgetを破棄してquitする。

- Qt main threadをblockしないため、Builderの`Canceling...`表示、process I/O drain、queued callbackが進行できる。
- D23の10秒hard-kill timeoutをそのまま利用し、hung childがapplication終了を無期限に妨げない。
- Pending callback数とworker lifetimeを追跡するshutdown coordinatorが必要になる。
- Last window close時もQtの自動quitだけに任せず、cleanup完了後に明示quitする必要がある。

#### B. Main threadで全workerを同期waitしてからquit

- Close / Quit handler内でcontextをcancelし、`WaitGroup.Wait`、child wait、DB closeを順に実行する。
- Control flowが直線的で、quit後にworkerが残らないことを理解しやすい。
- Workerがmain threadへqueued eventを送る、Builderがcancel表示を更新する、Qt callback内でcleanupする場合にdeadlockする。
- 10秒間windowが応答しない可能性があり、D20 / D37と整合しない。

#### C. Qt event loopを先に終了し、Go側cleanupだけを後で行う

- `QApplication::exec`をすぐ返し、その後main goroutineでworker cancel / wait、DB / log closeを行う。
- Windowは即座に消え、Qt main threadで待たずにGo処理を続けられる。
- Queue済み`mainthread.Start` callbackを実行できず、CGo handle、event payload、Qt object参照が未処理のままになる。
- Builder cancel状況とconfirmation UIも更新できない。

#### D. Workerを待たずprocess終了に任せる

- Closing stateだけ設定して即quitし、OSにgoroutine、DB handle、child processを回収させる。
- 最も速く単純に見える。
- Builder child / temporary DB、log flush、settings、queued callback、Windows file handleを安全に終了できず、既存のcleanup要件に反する。

どの案でもshutdown requestはidempotentとし、window close、Quit action、fatal callback、Builder errorが重なってもcleanupを1回だけ開始する。

D23のchild cleanupとD37のqueued dispatchを進行させながら、Qt main threadのfreeze / deadlockを避けられるためAを採用した。

実装規則:

- Application lifecycleは`running`、`closing`、`finalizing`、`done`の一方向stateとし、最初のshutdown reasonと最終exit codeを保持する。
- Linux / WindowsでもQtのlast-window自動quitだけに依存せず、close eventからshutdown coordinatorを開始し、finalize完了後に明示的にQt event loopを終了する。
- Shutdownを開始した最初のclose eventは一旦ignoreし、cleanup中にmain windowと必要なBuilder status widgetが破棄されないようにする。
- Closing開始時にwindow geometry / splitterを保存して`QSettings.sync()`を行い、新規query、dictionary切り替え、Builder起動、menu actionをdisabledにする。
- Current search contextをcancelし、dispatcherをclosingへ移して通常eventの新規postを拒否する。既にqueue済みのcallbackはclosing stateを見てQt objectへ反映せず完了する。
- Builder実行中はD20のconfirmationを経た後、stdin cancel、stdout / stderr drain、10秒wait、hard kill、temporary cleanupをD23どおり非同期実行する。
- Worker groupとdispatcher pending countはmain thread以外のcoordinator goroutineで待ち、Qt main threadから`WaitGroup.Wait`を呼ばない。
- Workerが終了してpending callbackが0になった時だけ、shutdown専用の最終dispatchを1件postする。通常dispatcherを再openしない。
- Final dispatchはmain threadでBuilder dialog、search service / repository、main window等を順にclose / deleteし、Qt quitを要求する。
- Loggerはcallback panicやfinalize errorを記録できるようQt event loop終了後まで保持し、最後にflush / closeする。
- `QApplication::exec`が戻った後は新しいQt dispatchを禁止し、worker / pending callbackが0であることをassertしてmainをreturnする。
- Shutdown中に再度Close / Quit / fatal eventを受けても新しいcancel goroutineやtimerを作らず、既存coordinatorの完了だけを待つ。
- macOSの通常Closeはcurrent searchをcancelしてwindowを非表示にするだけでapplication dispatcherをclosingにせず、明示Quit時だけ上記flowを使う。
- Search workerはcontext cancelへ応答することをintegration testで確認し、shutdownが無期限になる不具合をtest timeoutで検出する。

### D39. Startup時のconfig / DB failure表示

**状態: 決定済み（2026-09-13）**  
**決定: A. Config errorはstartup dialog、DB errorはmain windowで回復**

Default config fileが存在しない場合は既存contractどおり`config.Defaults()`を使い、errorにしない。
存在するdefault configまたは明示`--config`のread / parse / validation failureは、勝手にdefaultsへ置き換えない。

#### A. Config errorはstartup dialog、DB errorはmain windowで回復（採用）

- Config errorでは検索main windowを作らず、pathと短い理由を示すmodal startup dialogを表示する。
- Dialogは`Retry`、`Open Configuration File`または親directoryを開くaction、`Quit`を提供し、外部editorで修正後に同じprocessでreloadできるようにする。
- Configをloadできたら通常startupへ進み、defaultsで続行するbuttonやGUIによるconfig書き換えは設けない。
- DBは英和 / 和英を独立して検査し、少なくとも1つvalidなら利用可能な辞書でmain windowを開く。
- Missing / unreadable / corrupt / incompatible / wrong dictionary DBはその辞書だけをunavailableにし、Builderまたはlogへの回復導線をmain windowへ表示する。
- Configという全体前提のerrorと、Builderで回復可能な辞書単位errorを分けられる。

#### B. ConfigまたはDBに1件でもerrorがあればdialog後に終了

- Startup validationがall-or-nothingになり、通常main windowは常に英和 / 和英の両方を利用できる。
- Stateとtestが単純で、不完全な構成を許容しない。
- DB未作成状態からGUI Builderを開けず、D5 / D34の初回導線を満たさない。
- 片方の辞書だけを購入・利用するuserもGUIを起動できない。

#### C. Config errorはdefaultsへfallbackし、DB errorはunavailable扱い

- 起動を継続できる可能性が最も高く、startup dialogを減らせる。
- Typoしたcustom DB pathや`max_results`を黙って無視し、利用者が意図しないdefault pathへDBを作る危険がある。
- Existing CLIのstrict config contractと異なる。

#### D. Config errorもmain window内のrepair pageで扱う

- Application windowを常に表示し、config pathの選択、reload、外部editor起動を1 pageへまとめる。
- Fatal-looking modalを避け、file修正を繰り返しやすい。
- Valid configなしではDB path、default dictionary、Builder outputを決められず、限定controllerと追加navigationが必要になる。
- 初期版にconfig editorを持たない設計には過剰である。

どの案でもraw errorはlogへ1回記録し、UIには辞書entry、source TXT内容、stack traceを表示しない。
DB unavailable reasonはstring parseで判定せず、repository open層からtyped categoryとしてcontrollerへ渡す。

Strictなconfig contractを維持しながら、辞書DBだけの問題はGUI Builderで回復できるためAを採用した。

実装規則:

- Argument parseと`--help` / `--version`処理後にloggerを開き、`QApplication`を1回生成してからconfig loadを試みる。Config error時は同じapplication上でstartup dialogを表示し、成功時はそのままDB startupへ進む。
- Default configが存在しない場合だけ`config.Defaults()`を使う。Permission、I/O、parse、unknown key、validation errorはmissing扱いにしない。
- Startup dialogは通常main windowを生成せず、D46のcategory message、selectableなconfig path、該当actionだけを表示する。Raw error detailは表示しない。
- `Retry`は同じpathを再度read / parse / validateし、成功時にdialogを閉じて通常のDB startupへ1回だけ遷移する。
- Config fileが存在する場合の`Open Configuration File`はD24と同じ`QDesktopServices`方式を使い、存在しない明示pathでは既存の最も近いparent directoryを開く。
- External openに失敗してもdialogを閉じず、pathをcopy可能な状態にする。
- `Quit`またはwindow closeではD38の軽量なshutdown pathを通り、process exit code 2とする。Config修正後に正常起動した場合はerror exitに固定しない。
- Defaultsで続行、invalid keyの無視、configの自動書き換え、GUI内config editorは提供しない。
- Config成功後は英和 / 和英を別々にopen / validateし、`valid`、`missing`、`unreadable`、`corrupt`、`incompatible`、`wrong_dictionary`等のtyped statusを作る。
- 少なくとも1辞書がvalidならdefault dictionaryを優先し、defaultがunavailableなら最初のvalid辞書を選択する。Unavailableなcombo / menu itemはdisabledにする。
- 1辞書だけunavailableの場合は検索を妨げるmodal dialogを出さず、item tooltip、File menuのBuilder、logからreasonと回復方法を確認できるようにする。
- 両辞書がunavailableの場合は検索欄とresult listをdisabledにし、右message pageへ状態概要と`Build Dictionary Database...` actionを表示する。この場合`QComboBox`とDictionary menuの両actionもdisabledにする。
- Missingでは新規作成、incompatible / corruptでは既存DBの置換を明示するD22 dialogを開き、unreadableではpermission / config path確認を優先して案内する。
- Raw SQLite errorとDB pathはlogへ記録し、main messageには短いcategoryと辞書名だけを表示する。
- Repository open errorをtyped categoryへ拡張しても既存CLIのhuman-readable errorとexit codeを変更しない。

### D40. Startup時のDB open / validation実行thread

**状態: 決定済み（2026-09-13）**  
**決定: C. 両DBを同期openしてからwindow表示**

`search.OpenRepository`はread-only openに加えてschema、metadata、index、FTS smoke queryを実行する。
本項では、そのopen / validationを起動経路のどこで実行するかと、結果の計測方針を決める。

#### A. Window表示後、英和 / 和英をparallel workerでopen

- Config load後すぐmain windowを表示し、各辞書を独立したcancellable workerでopen / validateする。
- 検査中は静的な`Checking dictionary databases...` messageを表示し、利用可能なcurrent dictionaryが決まるまで検索欄をdisabledにする。
- Default dictionaryがvalidになれば他方の完了前でも検索を開始でき、defaultが失敗した場合は他方の結果を待ってfallbackする。
- 2 DBのmetadata / FTS smoke queryが同時に走るため、同じslow disk上では短時間のI/O競合が起きる可能性がある。
- Windowが先に応答可能になり、片方の遅延やfailureが他方をblockしない。

#### B. Window表示後、defaultを先にopenして他方を後続処理

- Default dictionaryのopenをworkerで開始し、完了後にもう一方をopenする。
- Startup I/Oを1 DBずつに制限し、defaultがvalidなら明確な順序で利用可能になる。
- Default DBがslow / corruptな場合、利用可能なもう一方の辞書検出まで待たされる。
- 全status確定までのwall timeはparallelより長くなる。

#### C. 両DBを同期openしてからwindow表示（採用）

- Controllerは完成したservice mapだけを受け取り、loading stateを持たずに済む。
- Windowが表示された時点ですぐ検索でき、UI stateが最も単純である。
- 現行validationはDB全体をscanせず、connection、schema / metadata、index、FTS indexの少数pageだけを確認するため、local filesystemでは通常短時間で完了すると見込む。
- DB open / verification中はapplication windowが現れず、slow filesystemでは起動していないように見える。
- UI event loop開始前なので表示済みwindowをfreezeさせないが、その時間はstartup latencyへ直接加算される。

初期実装ではCを採用し、実辞書DBでwarm / cold両方のopen時間を計測する。
通常環境で利用者が認識できる遅延またはwindow systemの応答なし判定が確認された場合だけ、Aへ変更する。

#### D. Startupではfile存在だけを確認し、初回検索時にvalidate

- Window表示とstartup処理を最小化でき、使わない辞書をopenしない。
- Query入力後にschema errorやwrong dictionaryが判明し、入力内容と無関係なerrorとして見える。
- Comboのavailable stateとBuilder overwrite導線をstartup時に確定できない。
- Invalid DBをvalidに見せる時間が生じるため推奨しない。

A / Bでworkerを使う場合もQt objectはmain threadだけで更新し、repository / serviceのownershipは成功eventで明示的にcontrollerへ移す。
その場合、`Post`がfalseを返した時はworkerがserviceをcloseし、queue後にgeneration mismatchと判定された時は`controller.apply`がserviceをcloseする。

現行validationはDB全体をscanせず、非同期化によるloading state、ownership移譲、cancel / shutdown分岐の方が初期実装には大きいためCを採用した。

実装規則:

- Config load成功後、通常main windowを生成する前に英和 / 和英を順番に`search.OpenRepository`し、`search.Service`とD39のtyped status mapを完成させる。
- Default dictionaryを先に検査し、続けてもう一方を検査する。Window表示は両方のstatus確定後とする。
- Startup DB open専用goroutine、loading event、progress indicator、cancel contextは初期実装へ追加しない。
- DB open中にQt widgetを作成せず、window表示後のmain-thread event loopを同期I/Oでblockしない。
- 片方のopen failureで他方の検査を省略せず、validなserviceは保持してD39のfallback規則へ渡す。
- `search.OpenRepository`がerrorを返した場合に内部でconnectionをcloseする既存contractを維持し、partial repositoryをcontrollerへ渡さない。
- 両DBがunavailableでもfatal終了せず、status確定後にBuilder導線を持つmain windowを表示する。
- Warm filesystemとcold filesystemで英和、和英、両方のopen / validation時間を個別にdebug logとbenchmark reportへ記録する。
- 辞書DB sizeだけを非同期化理由にせず、実測で利用者が認識できるstartup遅延または応答性問題が確認された場合だけD40-Aを再検討する。
- Custom configがnetwork filesystem等を指す構成は初期の性能保証対象にせず、その例外だけのために通常local startupを複雑化しない。

### D41. GUI表示言語と翻訳

**状態: 決定済み（2026-09-13）**  
**決定: C. 初期releaseは英語UIだけを提供**

辞書entry本文とheadwordは原文をそのまま表示し、本項の翻訳対象にしない。
対象はmenu、dialog、button、message、accessible name、shortcut help、About等のapplication UI textである。

#### A. 初期releaseは日本語UIだけを提供

- 設計書の現在の文言をそのまま利用し、application独自のtranslation catalogとlanguage設定を追加しない。
- 主対象である英和 / 和英辞書の日本語利用者へ最短で提供できる。
- Translation load、locale判定、2言語分のlayout / screenshot / accessibility testが不要になる。
- 日本語を読めない利用者には操作しにくく、英語OS上でnative dialogとapplication textの言語が混在する場合がある。

#### B. 日本語と英語をsystem localeで自動選択

- Source messageを英語としてtranslation API経由で定義し、日本語`.qm` catalogを同梱する。
- Localeが日本語なら日本語、それ以外は英語とし、runtimeのlanguage selectorは設けない。
- Cross-platform applicationとして利用範囲が広がり、将来の翻訳追加にも対応しやすい。
- 全text変更で2言語を同期し、長さの異なるmenu / dialog / accessibility表示をtestする必要がある。
- Qt標準dialogの翻訳catalogをどこまでbundleするかも決める必要がある。

#### C. 初期releaseは英語UIだけを提供（採用）

- Source code、README、CLI helpとGUI textを英語へ統一できる。
- Application固有translation catalogが不要である。
- 日本語IMEと英和 / 和英辞書を中心とする初期利用者にとって導入説明が分かりにくい。
- 本設計書で確定した日本語message / menu案を置き換える必要がある。

#### D. 日本語 / 英語のruntime selectorを提供

- System localeにかかわらず利用者がHelpまたはSettingsから表示言語を選べる。
- Mixed-language desktopや学習用途で明示的に切り替えられる。
- 設定保存、translatorの動的install / remove、既存widgetの再翻訳、再起動要求のいずれかを設計する必要がある。
- 初期版の設定項目とtest matrixが増える。

全案でfile path、DB metadata、Builder protocol field、logのmachine識別子は翻訳しない。
UI textを直接連結して文法を組み立てず、件数等は言語ごとにformat可能なmessage単位で定義する。

README / CLI help / source codeとGUIのproduct languageを揃え、初期releaseへtranslation infrastructureと二重のUI testを追加しないためCを採用した。

実装規則:

- Applicationが所有するmenu、action、dialog、button、message、tooltip、accessible name、shortcut help、About textは英語をcanonical textとする。
- 辞書名の表示は`English–Japanese` / `Japanese–English`とする。Config、protocol、log、internal identifierでは既存の`eiwa` / `waei`を維持する。
- Application独自の`.ts` / `.qm` catalog、`QTranslator`、locale自動選択、runtime language selectorは初期releaseへ追加しない。
- System localeを英語へ強制せず、日本語query、IME、Unicode入力、locale依存のplatform behaviorへ影響を与えない。
- Nativeまたはportal file dialog、system menu等、OS / desktopが所有するUIはhost localeで表示されてもよい。EJQuickがその翻訳を上書きしない。
- Qt translation catalogをapplication UIのためにはbundleしない。D29のallowlistに別のruntime componentとして必要になった場合は用途を明記する。
- UI文字列はaction定義とmessage formatterへ集約し、同じaction名をmenu、context menu、shortcut helpへ重複記述しない。
- 件数は`N shown`、上限到達時は`Showing first N`とし、語順を想定したfragment連結ではなくmessage単位でformatする。
- Menu / buttonにはQt標準の`&` mnemonicを設定し、widget testとaccessibility smoke testでcanonical English text、mnemonic、elision、十分なcontrol幅を確認する。
- 将来翻訳を追加する場合は、source languageを英語のまま保ち、D41を改訂してcatalog更新とlocale別testを導入する。

### D42. Application iconとAbout metadata

**状態: 決定済み（2026-09-13）**  
**決定: C. System themeのgeneric dictionary icon + compact About dialog**

D19の「iconなし」はtoolbar / menu action iconを指し、desktop entry、window、task switcher、Aboutで使うapplication iconは対象外とする。
D30のapplication ID `io.github.simosako.ejquick`と表示名`EJQuick`は全案で維持する。

#### A. 専用のoriginal icon + compactなcustom About dialog

- Project所有の単一SVGをiconの正とし、固定build imageで標準sizeのPNGを再現可能に生成する。
- Iconは小さいsizeでも判別できるbook / search motifとし、文字、国旗、辞書商品固有の意匠、third-party素材を使わない。
- Compactなwindow-modal `QDialog`にicon、`EJQuick`、product version、1行の説明、copyright、MIT license、project websiteを表示する。
- `Open Project Website`、`View Third-Party Notices`、`Close`を提供し、外部URL / fileはD24と同じ`QDesktopServices`経由で開く。
- Qt標準About boxより少し実装が増えるが、license noticeへの導線と表示内容をplatform間で一定にできる。

#### B. 専用のoriginal icon + `QMessageBox::about`

- Application iconはAと同じ方法で用意し、About本文をQt標準message boxへ表示する。
- 実装量が最小で、platform標準のbutton配置とmodal behaviorを利用できる。
- Websiteとthird-party noticesを明確な個別actionとして配置しにくく、長いlegal textやlinkでmessage boxが読みにくくなる。

#### C. System themeのgeneric dictionary icon + compact About dialog（採用）

- `QIcon::fromTheme`相当でhostのgeneric dictionary / search iconを使い、custom artworkを持たない。
- Source assetとraster生成工程を省略できる。
- Themeによって外観やicon availabilityが変わり、desktop entry install時に安定したapplication iconを提供できない。
- 他applicationと識別しにくく、themeに該当iconがなければdesktop menuでもplatform既定iconになる。

#### D. Rich About / diagnostics dialog

- Aのmetadataに加え、Qt / MIQT / Go version、platform plugin、config / log path、DB status等をtab形式で表示しcopyできる。
- Support時に環境情報を収集しやすい。
- 通常のAboutとして過剰で、path等のprivateなhost情報をscreenshotやissueへ誤って含める危険が増える。
- 初期releaseではdiagnostics exportのprivacy / redaction設計が別途必要になる。

全案でAboutへ辞書TXT名、DB path、query、headword、entry本文、machine固有identifierを表示しない。
Aboutを開くためにnetwork accessを行わず、update check、telemetry、remote asset loadは実装しない。

Application固有artworkとraster生成 / install工程を初期releaseへ追加せず、desktop themeとの視覚的一貫性を優先するためCを採用した。
固有iconがなく他applicationとの識別性が低い点は既知のtrade-offとし、必要になれば将来Aへ変更する。

実装規則:

- LinuxではFreedesktop Icon Naming Specificationのstandard application icon `accessories-dictionary`を第一候補とする。
- `QApplication`生成後、最初のwidget生成前に`QIcon::fromTheme("accessories-dictionary")`相当で取得し、non-nullの場合だけapplication / window iconへ設定する。
- Current icon theme、fallback theme、theme search pathをEJQuickから変更せず、Qtとdesktop environmentの選択を尊重する。
- `accessories-dictionary`が得られない場合は別のaction iconやpackage内assetへfallbackせず、null iconのままplatform既定表示を使う。Icon欠落はstartup errorにしない。
- About dialogでは同じresolved iconを使い、nullの場合はicon領域自体を表示しない。
- Source tree、release archive、desktop登録scriptへSVG / PNG / ICO / ICNS等のapplication固有iconを追加しない。
- Linux desktop entryの`Icon`は`accessories-dictionary`とし、D30の登録scriptは`$XDG_DATA_HOME/icons`へfileを作らない。
- Icon themeにより外観が異なることをvisual regression failureにせず、theme icon有 / 無の両方でlayoutと起動をtestする。
- Initial Linux release以外でgeneric theme iconが不十分になった場合は、そのplatform対応を表明する前にnative package icon要件と本決定を再評価する。

### D43. About dialogのproduct / legal text

**状態: 決定済み（2026-09-13）**  
**決定: E. Product名 / version / copyright / website URLだけを表示**

D42によりAboutはcompactなwindow-modal `QDialog`とし、generic theme iconが取得できる場合だけ表示する。
本項では本文とlegal informationへの導線を決める。

#### A. Product概要 + conciseなlegal表示 + local notice導線

- `EJQuick`、`Version <product-version>`、1行の英語説明、copyright、`Licensed under the MIT License.`を表示する。
- `Dictionary data is not included and remains subject to its supplier's terms.`と短く明記する。
- `Open Project Website`、`View License`、`View Third-Party Notices`、`Close`を提供する。
- License本文とthird-party noticesはdialogへ埋め込まず、release内のlocal fileをD24と同じ`QDesktopServices`で開く。
- 必要なidentityとlegal導線を保ちながらdialogを小さく維持できる。

#### B. Product名とversionだけを表示

- `EJQuick`、version、1行の説明、`Close`だけにする。
- 最もcompactで実装も少ない。
- MIT license、third-party notices、辞書dataの扱いをGUIから確認できない。

#### C. Aにruntime component versionも追加

- Qt、MIQT、Go、SQLite driver versionを表示し、support時の基本情報として使える。
- Host pathやqueryを含めずにdiagnostic valueを増やせる。
- Dependency versionの取得 / 注入、表示更新、release検査が増え、通常利用者には情報量が多い。

#### D. License / notices全文をdialog内のtabへ表示

- Networkや外部viewerなしでlegal textを読める。
- Textが長く、compact Aboutではなくなる。
- Package上のlicense fileと埋め込みcopyを同期する必要があり、binary sizeとtest対象も増える。

#### E. Product名 / version / copyright / website URLだけを表示（採用）

- 表示内容を`EJQuick`、`Version <product-version>`、copyright、project website URLの4項目だけにする。
- License説明、辞書data説明、third-party notices、runtime component version、diagnosticsは表示しない。
- WebsiteはURL文字列自体を表示するため、linkを開けない環境でも選択してcopyできる。
- 必要最小限のproduct identityだけを示すcompactなdialogになる。

全案でproduct versionは`internal/buildinfo.Version`だけを正とし、GUI独自versionを追加しない。
`dev` buildもそのまま`Version dev`と表示し、releaseらしいversionへ偽装しない。

利用者が指定した最小構成をそのまま満たし、Aboutへlegal / diagnostics情報を増やさないためEを採用した。

実装規則:

- Dialog titleは`About EJQuick`とする。
- D42のtheme iconが取得できた場合だけdecorative iconを表示し、text metadataは次の4項目に限定する。

  ```text
  EJQuick
  Version <internal/buildinfo.Version>
  Copyright © 2026 EJQuick contributors
  https://github.com/simosako/ejquick
  ```

- Website URLはselectableなlinkとして表示し、activation時だけ`QDesktopServices`で開く。Dialogを開いただけではnetwork accessしない。
- URLを開けない場合もdialogを閉じず、URLをmouse / keyboardで選択してcopyできる状態を維持する。
- Dialog controlはplatform標準の`Close` buttonだけとし、website、license、notices用の追加buttonを置かない。
- One-line description、license名 / 本文、dictionary data notice、Qt / MIQT / Go version、path、DB status、update情報を表示しない。
- `LICENSE`と`THIRD_PARTY_NOTICES`はD6 / D29どおりreleaseへ同梱するが、Aboutからの導線は設けない。
- Release testは表示versionが`ejquick-gui --version`および同梱`ejquick-build`と一致することを確認する。

### D44. Startup / runtime errorのUI detail level

**状態: 決定済み（2026-09-13）**  
**決定: A. Typed categoryごとの短いmessage + contextual action**

D39でconfig errorと辞書単位のDB errorを分離し、repository open層からtyped categoryを返すことは決定済みである。
本項では利用者へ見せるdetail levelと、raw errorの扱いを決める。

#### A. Typed categoryごとの短いmessage + contextual action（採用）

- `missing`、`unreadable`、`corrupt`、`incompatible`、`wrong_dictionary`、`unknown`等を中央のUI mapperで英語messageへ変換する。
- Messageは「何が利用できないか」と「次に何をするか」の2文以内にし、Builder、Retry、Open Log等の該当actionだけを表示する。
- Wrapped raw error、SQLite code、path等はlogへ記録し、通常UIへ表示しない。
- 安全で一貫した表示と、categoryに応じた回復導線を両立できる。

#### B. Operation単位のgeneric messageだけを表示

- Config、DB open、search、Builderごとに1種類の失敗messageだけを表示する。
- UI mapperと文言testが最小になる。
- Missingとcorrupt、permission errorを区別できず、適切な回復方法を案内しにくい。

#### C. Wrapped raw errorをそのまま表示

- 実装が単純で、開発者には原因を確認しやすい。
- SQLite内部文言、絶対path、OS error、実装detailが露出し、長さも予測できない。
- Screenshot / reportへprivateなhost情報が混入しやすい。

#### D. 短いmessage + 展開可能なtechnical details

- 通常はAと同じsummaryを表示し、必要な利用者だけraw errorをdialog内で展開できる。
- Log viewerを開かず調査できる。
- Redaction、copy、折りたたみstate、長文layoutの実装が増え、private pathを共有する危険は残る。

全案でcontext cancellationとshutdownに伴うexpected errorは利用者向けfailureとして表示せず、debug logだけに記録する。
同じ原因をmain pane、dialog、status areaへ重複表示しない。

利用者へ内部実装やprivate pathを露出せず、原因に合った回復導線を提供できるためAを採用した。

実装規則:

- Config load、repository open、search、Builder process / protocol、external URL / file openの各境界で、closed setのtyped categoryへ分類する。
- Category値はGoのnamed typeと定数で定義し、UIは`errors.As` / `errors.Is`またはmachine protocolのcodeから判定する。`error.Error()`の文字列比較は行わない。
- Typed errorは元errorを`Unwrap`できるようにし、既存CLIが表示するhuman-readable errorとexit codeを変更しない。
- UI mapperはoperation、category、必要な辞書種別だけを受け取り、raw `error`やpathをmessage formatterへ渡さない。
- UI messageはcanonical English textとし、title / summary、補足文、operationごとのtableで定義されたcontextual actionから構成する。
- Raw errorはcontroller境界でoperationとcategoryを添えて1回だけlogへ記録する。Lower layerとUI callbackの双方で同じerrorを重複記録しない。
- Error / failure logにはquery、headword、entry本文、source TXT内容を含めない。`--debug`時の正常系DEBUG行だけが、D24と既存contractどおり正規化済みqueryを含めてよい。
- Config / DB pathはD46のstartup dialogとD22のBuilder output表示を除きUIへ表示せず、診断用にlogへ記録したpathを他のUIへ転記しない。
- Unknown categoryには必ずgeneric fallback messageを用意し、未定義値、wrapped third-party error、将来追加されたcodeでblank paneやpanicを起こさない。
- `context.Canceled`、closing generationのstale result、利用者が要求したBuilder cancelはexpected outcomeとして扱い、error message boxを表示しない。
- Search failureはD15のdetail message page、辞書unavailableはD39のmain message page、config failureはstartup dialog、Builder failureはD20のBuilder dialogだけに表示する。
- `Open Log`等の共通actionはD19 / D24と同じ`QAction`またはcontroller commandを使い、画面ごとに処理を複製しない。
- Table-driven testですべてのcategoryについてmessage、action、raw error非露出を検証し、絶対pathやSQLite error textを含むsentinelで漏洩を検出する。

### D45. DB unavailable categoryごとのmessage / action

**状態: 決定済み（2026-09-13）**  
**決定: A. Categoryごとに原因とprimary recoveryを明示**

辞書名の`<dictionary>`にはD41の`English–Japanese`または`Japanese–English`を代入する。
Pathとraw SQLite errorはD44どおりmain message pageへ表示しない。

#### A. Categoryごとに原因とprimary recoveryを明示（採用）

| Category | Message | Actions |
|---|---|---|
| `missing` | `The <dictionary> database was not found.` | `Build Dictionary Database...` |
| `unreadable` | `The <dictionary> database could not be opened. Check its configured path and file permissions.` | `Open Log` |
| `corrupt` | `The <dictionary> database is damaged or is not a valid EJQuick database.` | `Build Dictionary Database...`, `Open Log` |
| `incompatible` | `The <dictionary> database uses an incompatible format.` | `Build Dictionary Database...`, `Open Log` |
| `wrong_dictionary` | `The configured <dictionary> database contains the other dictionary type.` | `Build Dictionary Database...`, `Open Log` |
| `unknown` | `The <dictionary> database is unavailable.` | `Open Log` |

- Category差を利用者が理解でき、Builderを安全な回復手段にできる状態だけに表示できる。
- 文言とaction matrixのtest項目は増える。

#### B. Recoverabilityで3 groupにまとめる

- `missing`、`rebuild_required`、`cannot_open`の3 messageに集約する。
- Corrupt / incompatible / wrong dictionaryはすべて`This database must be rebuilt.`として扱う。
- UI文言とtestが少なく、内部category追加の影響も小さい。
- Config取り違えとformat更新の区別が利用者から見えず、不要なrebuildを選ぶ可能性がある。

#### C. 全categoryで同じgeneric messageを使う

- `The <dictionary> database is unavailable.`と`Open Log`だけを表示する。
- 最も単純で、分類精度が不十分でも誤案内しない。
- DB未作成時にもBuilder導線が直接表示されず、D5 / D39の初回利用体験が悪い。

#### D. Categoryごとにmodal error dialogを表示

- Startup直後に原因とactionを必ず利用者へ提示できる。
- 片方の辞書がvalidでも検索開始前に操作を中断し、両方失敗時はdialogが連続する可能性がある。
- D15 / D39で採用したmain paneのcontextual messageと一貫しない。

全案で`Open Log`はloggerがfileを提供できる場合だけenabledにし、disabled actionの代わりにlog pathをUIへ表示しない。
Builder成功後のDB再openが失敗した場合は新しいtyped categoryでstatusとmessageを更新し、成功表示を残さない。

DB未作成時のBuilder導線を直接提示しながら、permission error等で不用意な置換を勧めないためAを採用した。

実装規則:

- Repository open層は`missing`、`unreadable`、`corrupt`、`incompatible`、`wrong_dictionary`、`unknown`のclosed setを返す。
- `missing`はfile / path不在、`unreadable`はpermissionその他のaccess failure、`corrupt`はSQLiteのcorrupt / not-a-database判定に限定する。
- `incompatible`はSQLite fileとして開けるが、必要なschema object、metadata、schema / normalization version、FTS smoke validationを満たさない場合とする。
- `wrong_dictionary`はmetadataのdictionary typeが期待する英和 / 和英と異なる場合とし、`incompatible`より優先して分類する。
- Driver固有error codeはrepository / SQLite adapter内だけで判定し、GUI controllerへはcategoryとwrapped causeだけを渡す。
- Tableのmessageとactionをcanonical mappingとし、widget側で句読点や補足文を追加しない。
- `Build Dictionary Database...`は該当辞書をpreselectしたD22 dialogを開く。既存outputがあるcategoryではreplacement checkboxを必須にする。
- `unreadable`と`unknown`では、原因を確認せず既存fileを上書きする危険があるためBuilder actionを表示しない。
- 1辞書だけunavailableで他方がvalidならstartupを中断せず、valid辞書を選択する。Unavailable itemのtooltip / accessible descriptionには同じcanonical messageを使う。
- 両辞書がunavailableならdefault dictionaryを先にして両方のmessageをright message pageへ表示し、同名actionは1個にまとめる。Builder dialogの初期値はdefault dictionaryとする。
- Builder成功後は該当辞書だけを再openしてstatusを置換する。他方のservice、query、result stateはD10の切り替え規則が要求する場合を除き変更しない。
- `Open Log`が利用不能でactionを表示できない場合もmessage本文は変更せず、空のbutton rowはlayoutから除く。

### D46. Config startup errorのcategory / message / action

**状態: 決定済み（2026-09-13）**  
**決定: A. Category別にreasonと利用可能な回復actionを表示**

D39によりconfig errorはmain windowを生成せずstartup dialogに表示し、同じprocessでRetryできる。
Config pathはこのdialogに限ってselectable textとして表示する。

#### A. Category別にreasonと利用可能な回復actionを表示（採用）

| Category | Message | Actions |
|---|---|---|
| `path_unavailable` | `The configuration file location could not be determined.` | `Retry`, `Open Log`, `Quit` |
| `missing` | `The configuration file was not found.` | `Retry`, `Open Parent Folder`, `Quit` |
| `unreadable` | `The configuration file could not be read.` | `Retry`, `Open Parent Folder`, `Open Log`, `Quit` |
| `invalid` | `The configuration file is invalid.` | `Retry`, `Open Configuration File`, `Open Log`, `Quit` |
| `unknown` | `The configuration could not be loaded.` | `Retry`, pathに応じたopen action、`Open Log`, `Quit` |

- TOML syntax、unknown key、invalid valueは、いずれもfile編集が必要な`invalid`へまとめる。
- 原因と次の操作が明確で、存在しないfileに対してopen-file actionを出さずに済む。
- Action matrixとfile状態変化を含むtestが必要になる。

#### B. Access errorとinvalid configの2 groupにまとめる

- Read / path関連は`The configuration file could not be read.`、parse / validation関連は`The configuration file is invalid.`とする。
- Categoryとtestを減らしつつ、folderを確認するかfileを編集するかは区別できる。
- Explicit pathのmissing、permission、default path決定失敗の違いはlogを開くまで分からない。

#### C. 全failureで同じgeneric messageを使う

- `The configuration could not be loaded.`とpath、`Retry`、open action、`Quit`だけを表示する。
- UIは単純で、underlying parserやOS error分類に依存しない。
- 利用者がfile作成、permission修正、内容編集のどれを行うべきか判断しにくい。

#### D. Raw parse / validation errorをdialogへ表示

- TOML line / column、unknown key、範囲外の値をその場で確認でき、修正しやすい。
- Parser由来の長文、absolute path、内部型名等が表示される可能性があり、D44-Aのraw error非表示方針と一致しない。
- Safeなfield-level diagnosticを別途設計しない限り採用しない。

全案でdefault config pathが存在しない場合だけD39どおりerrorにせずdefaultsを使用する。
明示`--config` pathのmissingはstartup errorとし、defaultsへfallbackしない。

Fileの編集、作成、permission修正、environment確認のどれが必要かを利用者が判断でき、存在しないfileへのopen-file actionも避けられるためAを採用した。

実装規則:

- Config load境界は`path_unavailable`、`missing`、`unreadable`、`invalid`、`unknown`のclosed setへ分類する。
- `path_unavailable`は`config.DefaultPath()`の失敗とし、`--config`明示時には発生しない。
- `missing`は`os.IsNotExist`相当の場合だけとし、明示`--config` pathに対してのみ発生する。Default pathのmissingはD39どおりerrorにしない。
- `unreadable`はmissing以外のread / access failure、`invalid`はTOML parse、unknown key、validation failureとする。
- Startup dialogはcategory message、selectableなconfig path、tableのactionだけを表示する。Raw error、TOML line / column、内部型名はlogだけに記録する。
- `Retry`はread / parse / validateの全過程を同じpathで再実行し、成功時にdialogを閉じてD40のDB startupへ1回だけ遷移する。
- Actionの有効性はdialog表示のたびに再評価する。`missing`の`Open Parent Folder`は存在する最も近いancestor directoryを開く。
- `Open Log`はloggerがfileを提供できる場合だけenabledにし、提供できない場合はbutton自体を表示しない。
- `Quit`またはwindow closeはD39どおりexit code 2の軽量shutdownとする。
- Table-driven testで各categoryのmessage、action set、file状態変化時のaction再評価、raw error非露出を検証する。

### D47. Search runtime errorのmessage / retry behavior

**状態: 決定済み（2026-09-13）**  
**決定: A. 単一generic message + query変更時のみ自動再試行**

D15でsearch errorはmodal dialogを使わず右paneのmessage pageに表示することは決定済みである。
本項ではmessage文言、retryの発火条件、error stateの遷移を決める。

#### A. 単一generic message + query変更時のみ自動再試行（採用）

- 文言はD15の`Search failed` / `Edit the query to try again`に`Open Log` actionを付けるだけとする。
- Retry button、timer再試行、error category別文言は設けず、query変更による新requestだけが再試行になる。
- Incremental searchの単純なstate modelを維持でき、入力のたびにerror文言が切り替わるnoiseを最小化できる。
- 一時的なDB障害から同じqueryで即座に再試行する手段はない。

#### B. Category別message + 明示Retry action

- `normalization`、`database`、`unknown`等で文言を変え、`Retry` actionで同じqueryを再実行する。
- 原因の見通しと手動再試行を提供できる。
- Errorのたびに文言が変わり、retry連打やstale retry requestのstate管理が必要になる。

#### C. Generic message + 一定間隔の自動retry

- Error後に例えば1秒ごと同じqueryを自動再試行する。
- 一時障害から利用者操作なしで回復できる。
- DB障害中もqueryを継続発行し、timer管理とerror表示のちらつきが増える。

#### D. Error時は検索を停止しrestartを要求

- 安全側に倒して以後のqueryを受け付けない。
- 破損状態で誤った結果を見せない。
- 一過性errorでもGUI再起動が必要になり、D15の「入力を止めずに回復」方針と矛盾する。

全案で`context.Canceled`とstale requestのerrorは表示せず、D44どおりexpected outcomeとして扱う。

D15のmessage例と一致し、timerやretry stateを追加せず入力継続だけで回復できるためAを採用した。

実装規則:

- Error stateはrequest IDに紐付け、current requestのerrorだけを表示する。Stale generationのerrorは破棄する。
- 文言は`Search failed`と`Edit the query to try again`に固定し、error categoryや辞書種別で変えない。
- Error時はresult list、選択、件数labelをclearし、message pageへ`Open Log` actionを表示する。検索欄はenabledのままとする。
- Retry buttonとtimer再試行は設けない。Query文字列が実際に変化した場合だけ新requestが開始され、error stateを置き換える。
- Queryを空にした場合と辞書切り替え時はerror stateをclearし、それぞれempty案内と切り替え後の初期状態へ戻す。
- Search errorは辞書名とrequest IDを添えてwrapped errorのまま1回だけlogへ記録する。Query文字列、path、raw SQLはerror logへ含めない。`--debug`時の正常系DEBUG行だけがD24どおり正規化済みqueryを含む。
- Error後に新requestが成功した時点でmessage pageからentry detail pageへ戻す。成功まで旧errorを表示し続けない。
- `Open Log`が利用不能ならbuttonを表示せず、message文言は変更しない。
- Consecutiveな同種errorでもrequestごとに1回logへ記録する。UI側でのerror集約や件数badgeは設けない。

### D48. Builder runtime errorのcategory / message

**状態: 決定済み（2026-09-13）**  
**決定: A. Category別の短いmessage + Setup復帰 / Open Log**

D20でfailure時はdialog内に短いerrorを表示し、入力値を保持したままSetupへ戻れることは決定済みである。
本項ではBuilder process / protocol errorのcategoryと表示文言を決める。

#### A. Category別の短いmessage + Setup復帰 / Open Log（採用）

| Category | 例 | Message |
|---|---|---|
| `binary_missing` | `ejquick-build`不在 / 実行不能 | `The dictionary builder could not be started.` |
| `version_mismatch` | product / protocol不一致 | `The dictionary builder version does not match this application.` |
| `protocol_error` | 不正JSON / unknown event | `The dictionary builder sent an invalid response.` |
| `output_busy` | D32 writer lock取得失敗 | `Another dictionary database build is already running.` |
| `source_invalid` | TXT不在 / 非regular file / outputと同一 | `The selected source file cannot be used.` |
| `build_failed` | その他のchild側error event | `The dictionary database could not be built.` |
| `process_died` | completion eventなしに異常終了 | `The dictionary builder stopped unexpectedly.` |
| `unknown` | 上記以外 | `The dictionary database could not be built.` |

- すべてfailure stateのdialog内表示とし、`Back to Setup`と`Open Log`を提供する。
- Childのstderrやerror messageをそのまま表示せず、D21のerror code / GUI側分類から文言を生成する。
- `output_busy`のような対処可能な原因を利用者へ説明できる。

#### B. Generic message + Open Logだけを表示

- `The dictionary database could not be built.`に固定し、category表示を持たない。
- UI文言とtestが最小になる。
- Lock競合やversion mismatchを区別できず、利用者が同じ操作を繰り返して失敗する可能性がある。

#### C. Child stderrの末尾をdialogへ表示

- Builderが出した診断をその場で確認できる。
- stderr文言の変更で表示が不安定になり、pathや内部detailが露出する。D44-Aの方針と一致しない。

#### D. Modal `QMessageBox`でerrorを別途表示

- Failureを確実に認識できる。
- Dialog on dialogになり、D20の1 dialog設計と重複する。Setup復帰動線も複雑になる。

全案で`canceled`はerrorとして扱わず、D20 / D23どおりcancel確認と`Canceling...`の既存flowだけを使う。

`output_busy`やversion mismatchのような対処可能な原因を区別でき、D20のSetup復帰flowと整合するためAを採用した。

実装規則:

- Builder errorは`binary_missing`、`version_mismatch`、`protocol_error`、`output_busy`、`source_invalid`、`build_failed`、`process_died`、`unknown`のclosed setとする。
- `binary_missing`、`version_mismatch`、`protocol_error`、`process_died`はGUIのprocess / protocol監督層で分類する。
- `source_invalid`、`output_busy`、`build_failed`はD21のchild error eventのcodeから分類し、eventのmessage文字列はUIへ表示しない。
- FailureはD20のdialog内failure stateに表示し、actionは`Back to Setup`と`Open Log`に限定する。Modal error boxを追加しない。
- `Back to Setup`は辞書種別、source path、置換checkboxの入力値を保持したままSetup stateへ戻し、修正後に同じdialogで再実行できるようにする。
- `output_busy`では自動retry / lock待機をせず、利用者が他buildの終了後に手動で再実行する。
- `version_mismatch`ではbuildを開始せず、messageはtableの固定文言だけとする。Package再展開の案内を追加しない。
- `protocol_error`検出時はD21どおりchildをcancelしてからfailure stateへ遷移する。
- `process_died`はexit codeとdrain済みstderrの要約をlogへ記録する。Source TXT内容、辞書entryは含めない。
- Raw stderr、child error message、stack traceはdialogへ表示せず、categoryと辞書種別を添えて1回だけlogへ記録する。
- Failure後にdialogを閉じた場合、D45の規則に従って辞書statusを再評価し、旧DBが利用可能ならそのまま検索を継続できるようにする。
- Table-driven testで全categoryのmessage、action、stderr / raw error非露出を検証する。

### D49. UI font / text rendering方針

**状態: 決定済み（2026-09-13）**  
**決定: A. Qt / desktop既定fontへ完全に任せる**

UI textは英語だが、辞書のheadword / bodyは日本語を含むためCJK glyphの表示が必須である。
D29のallowlist方式ではfontはsystem側の構成要素としている。

#### A. Qt / desktop既定fontへ完全に任せる（採用）

- `QApplication`のdefault fontをそのまま使い、family、size、weightをcodeやstylesheetで上書きしない。
- CJK glyphはsystemのfont fallback（fontconfig等）に任せ、fontをbundleしない。
- ThemeやDPI環境との一貫性が保て、binary / package sizeを増やさない。
- CJK fontを持たない最小環境ではtofu表示になる可能性があり、その検出や案内は行わない。

#### B. Noto Sans CJK等をbundleして表示を固定

- 全環境で同一のglyph / metricsを保証できる。
- Package sizeが数十MB増え、fontのlicense notice、更新、rendering差分testが必要になる。
- D29の「fontはsystem版を利用」方針と矛盾する。

#### C. UI内にfont選択設定を追加

- 利用者がfamily / sizeを変更できる。
- 設定UI、保存、restore validation、環境差testが増える。初期版の設定項目を増やしたくない方針と合わない。

#### D. Bodyをmonospace fontへ固定

- `QPlainTextEdit`の本文を等幅にし、TUIの表示へ近づける。
- 辞書本文は表組みではなくproseが中心で、等幅化の実益が小さい。
- CJK対応monospace fontのfamily指定は環境差が大きく、style一貫性を崩す。

全案でhinting、anti-aliasing、fractional scaling等のrendering parameterはQt / platformの既定を変更しない。
参照環境（KWin + Fcitx5 / Mutter + IBus）でのCJK表示確認はD36のsmoke testに含める。

D29のfontはsystem側という方針と一致し、package sizeと保守対象を増やさないためAを採用した。

実装規則:

- Source code、stylesheet、`QFont`指定のいずれでもfont family、size、weight、styleを上書きしない。
- `QPlainTextEdit`の辞書本文を含め、すべてのwidgetで`QApplication`のdefault fontを使う。
- Font fileをsource tree / release packageへ追加せず、CJK glyphはsystemのfont fallbackに依存する。
- Fontconfig設定、`QT_FONT_DPI`等のfont関連環境変数をEJQuickから変更しない。
- Desktopの利用者が設定したsystem font / sizeをそのまま尊重し、GUI側に独自のfont size設定を持たない。
- 対応環境として「desktopがCJK表示可能なfontを提供すること」をREADMEの動作要件へ明記する。
- D36のsmoke testで英和 / 和英のheadwordとbodyのCJK表示、preedit、長文wrapを確認する。
- Tofu / missing glyphが参照環境で発生した場合はbundle追加ではなく、環境要件とdocumentの問題として扱う。

### D50. High-DPI / scaling policy

**状態: 決定済み（2026-09-13）**  
**決定: A. Qt 6の既定scalingへ完全に任せる**

Qt 6ではhigh-DPI scalingは常時有効であり、D16のwindow sizeもdevice pixelではなくlogical pixel基準としている。
残る論点はscale factorの丸めと追加の調整を行うかである。

#### A. Qt 6の既定scalingへ完全に任せる（採用）

- Scale factor rounding policyを変更せず、`QT_SCALE_FACTOR` / `QT_ENABLE_HIGHDPI_SCALING`等の環境変数も設定しない。
- Size / minimum sizeは全てlogical pixelで定義し、device pixelを直接扱うcodeを書かない。
- Desktopの表示倍率設定を尊重でき、追加のtest matrixを増やさない。
- Fractional scaling（125% / 150%等）でのわずかなぼやけはQt / compositorの処理に委ねる。

#### B. Integer scalingへ丸めるpolicyを設定

- Scale factor rounding policyを`Round`等へ変更し、125%を100%または200%へ寄せる。
- 描画の鮮明さを優先できる場合がある。
- 利用者が選んだ倍率を上書きし、desktop設定との不整合を起こす。

#### C. `QT_SCALE_FACTOR`等で倍率を固定

- 全環境で同一の見た目を目指す。
- Multi-monitorや利用者設定を壊し、D26の環境自動選択方針と矛盾する。

#### D. 自前でdevice pixel ratioを処理

- `devicePixelRatio`を各描画codeで考慮する。
- Widgetsベースでは不要で、custom paintingを持たない本GUIでは複雑さだけが増える。

全案でtestはlogical pixel基準とし、100%と200%の両方でwindow size、splitter、件数labelのlayoutを確認する。

D26の環境自動選択方針とD16のlogical pixel基準に一致し、利用者の表示倍率設定を上書きしないためAを採用した。

実装規則:

- Scale factor rounding policyを変更せず、`QT_SCALE_FACTOR`、`QT_ENABLE_HIGHDPI_SCALING`、`QT_SCALE_FACTOR_ROUNDING_POLICY`等の環境変数をEJQuickから設定しない。
- Initial / minimum size、pane最小幅、件数labelのminimum width等のsize定義はすべてlogical pixelとし、device pixelを直接扱うcodeを書かない。
- Custom paintingを持たず、`devicePixelRatio`を参照するwidget codeを追加しない。
- Theme iconはD42どおりtheme解決に任せ、DPI別の自前assetを持たない。
- Widget testは100%相当で実行し、200% scalingでの起動・layout崩れはD36の実環境smoke testで確認する。
- Fractional scaling（125% / 150%）の見た目はQt / compositorの処理に委ね、独自の補正を入れない。

### D51. 起動時のXDG desktop portal host登録

**状態: 決定済み（2026-09-14）**  
**決定: A. `QT_NO_XDG_DESKTOP_PORTAL=1`でQtのportal servicesを無効化する**

Qt 6.11.2のUnix services plugin（`QDesktopUnixServices`）は、`QApplication`生成時に`QGuiApplication::desktopFileName`を`org.freedesktop.host.portal.Registry.Register`でportalへ登録し、portal呼び出しの帰属先として利用できるようにする。
一方、xdg-desktop-portal 1.20以降の`Register`は、同一D-Bus接続で1回だけ呼べること、および他のportal method呼び出しより前に行う必要があることを仕様として要求し、呼び出し元の接続はportal method呼び出し時に接続単位でcacheされる。
この組み合わせでは、Qtの登録処理が先行するportal呼び出しやdesktop entry未installと衝突して失敗し、Qtはその失敗を`qt.qpa.services`warningとしてstderrへ出力する。

開発環境（Arch Linux / Hyprland、Qt 6.11.2、xdg-desktop-portal 1.22.1、MIQT v0.14.0）での確認結果は次のとおりである。

- `ejquick-gui`のconfig無し起動で、`qt.qpa.services: Failed to register with host portal ... Could not register app ID: Connection already associated with an application ID`が出力された。
- 同種の警告は、app IDに対応するdesktop entryが見つからない場合は`Could not register app ID: App info not found for '<app-id>'`として現れる。
- Window表示、event loop、通常終了への影響はない。登録成功時の出力はdebug levelのため通常表示されない。CopyQ #3631など複数のQt applicationで同様の警告が報告され、動作への影響は報告されていない。

EJQuick GUIにとってportal servicesは不要である。D24の`QDesktopServices`による外部openのportal経路は、Qtの`checkNeedPortalSupport()`（`/.flatpak-info`の存在または`SNAP`環境変数の設定）を満たす場合だけ選択され、非sandbox環境では使われない。GUI自身も直接portal methodを呼ばない。
したがって登録の成否はEJQuickの動作に影響せず、警告は機能的意味を持たない起動時noiseである。

#### A. `QT_NO_XDG_DESKTOP_PORTAL=1`でportal servicesを無効化する（採用）

- `QApplication`生成前に`QT_NO_XDG_DESKTOP_PORTAL`が空の場合だけ`1`を設定し、Qtのportal services（host登録、screenshot version確認、portal restart監視）をまとめて無効化する。
- Qtが公式に備えるopt-out変数であり、Qt実装の内部挙動への個別workaroundを避けられる。
- 利用者が明示的に値（`0`を含む）を設定した場合はそれを尊重し、EJQuickは上書きしない。
- `QGuiApplication::setDesktopFileName("io.github.simosako.ejquick")`相当とorganization / application metadataは引き続き設定する。Wayland application ID（D30）と`QSettings`配置（D16）はportal servicesとは独立に必要である。
- 警告が確実に消え、起動ごとの不要なDBus呼び出しとservice watcherもなくなる。
- 将来portal連携（portal file dialog、global shortcut等）を採用する場合は本決定を見直す必要がある。

#### B. 警告を無視して何も設定しない

- Code変更が不要で、Qt / portal側の将来的な修正に任せられる。
- 非sandbox環境ではportal services自体が不要なため、起動ごとの無意味なDBus呼び出しとstderr warningが残る。
- 利用者からbug reportとして繰り返し報告される可能性が高い。

#### C. `QLoggingCategory`のfilter rulesで`qt.qpa.services`warningだけを抑制

- portal servicesは有効なままのため、将来portal連携を追加する場合の変更が小さい。
- 登録失敗そのものは解決せず、同categoryの他の診断warningも一緒に消える。
- Filter rulesはlog category名に依存し、category名の変更で静かに効かなくなる。

#### D. D30のdesktop entryを先にinstallして登録を成功させる

- Portalへ正しいapplication IDを登録でき、portal利用時の帰属が正しくなる。
- `App info not found`型の失敗は減らせるが、接続単位のcacheによる`Connection already associated ...`は環境によって残る。
- D30の登録scriptは将来milestoneの成果物であり、警告の解消をその実装に依存させるのは順序として早い。

機能変化なしに警告を根絶でき、Qt公式のopt-out経路を使うためAを採用した。
Portal連携を採用する時点で、D30の実装状況と合わせて本決定を再評価する。

実装規則:

- `internal/gui`の`Run`は`QApplication`生成前に`skipPortalServices()`を呼ぶ。`skipPortalServices`は`QT_NO_XDG_DESKTOP_PORTAL`が空の場合だけ`1`を設定し、明示的な値は上書きしない。
- Organization name、application name、application version、desktop file nameのstatic setterは、Qt文書どおり`QApplication`生成前に呼ぶ。
- 本変数はplatform / input methodの選択には関与しない。D26どおり、EJQuickは`QT_QPA_PLATFORM`、`QT_IM_MODULE`、`QT_PLUGIN_PATH`には一切触れない。
- `QT_NO_XDG_DESKTOP_PORTAL`の効果はQt実装の`QDesktopUnixServices`に依存するため、Qt baseline更新時（D7）には起動時warningの有無をsmoke testで再確認する。
- 未設定時に`1`が設定されることと、利用者の明示的な値を上書きしないことをunit testで検証する。
- Portal連携機能の採用時は、D30のdesktop entry installと合わせて本決定を見直し、本変数を設定しない構成への復帰を検討する。

---

## 10. 推奨する決定順序

決定済み:

1. D1: Qt Widgets
2. D2: Go + MIQT
3. D3: 通常の 2-pane window
4. D4: OS 標準の application lifecycle
5. D8: 性能値は計測するが release gate にしない
6. D5: GUI から `ejquick-build` child process を起動
7. D7: Qt 6.11.2 / MIQT v0.14.0 / Go 1.27.1を初期baselineに固定
8. D7-L: Linux `x86_64` / glibc 2.34以降
9. D6: Self-contained directory + `tar.zst`
10. D9: 辞書切り替えに`QComboBox`を使用
11. D10: 辞書切り替え時にqueryと検索stateをclear
12. D11: `QListView` + `QStringListModel` + plain-text詳細
13. D12: 検索欄focusを維持するinput-centric方式
14. D13: Input-centric固定keymap
15. D14: 検索中は直前の成功結果をそのまま表示
16. D15: 右の詳細paneへcontextual messageを表示
17. D16: `QSettings`でwindow / splitter状態を保存
18. D17: Header右端に表示件数を表示
19. D18: 標準copy + 選択entry全体のcopy action
20. D19: 標準`QMenuBar`
21. D20: Setupから完了までwindow-modal Builder dialog
22. D21: stdout JSON Lines + stdin control channel
23. D22: 必須項目だけのsingle-page Builder setup
24. D23: 10秒後hard kill + 今回の一時DBだけcleanup
25. D24: 既存append-only logを外部applicationで開く
26. D25: `ejquick-gui`を独立binaryとして追加
27. D26: QPA / input method選択をQtと利用者環境へ任せる
28. D27: WaylandとXCBのQPA pluginをbundle
29. D28: Compose、IBus、Fcitx5 input context pluginをbundle
30. D29: Host integrationを除外する明示allowlist方式
31. D30: Direct binary + 任意のper-user desktop登録script
32. D31: 独立した複数GUI processを許可
33. D32: Nonblocking OS advisory writer lock
34. D33: Output DBと同じdirectoryの永続hidden lock file
35. D34: 検索GUI + Builder UI + self-contained packageを初期release scopeとする
36. D35: `gui` build tag + 3層の自動test
37. D36: Weston headless CI + KWin / Mutter実環境smoke test
38. D37: MIQT `mainthread.Start`によるtyped async dispatch
39. D38: Event loopを維持する2段階async shutdown
40. D39: Config errorはstartup dialog、DB errorはmain windowで回復
41. D40: 両DBを同期open / validation後にwindow表示
42. D41: 初期releaseは英語UIだけを提供
43. D42: System themeのgeneric dictionary icon + compact About dialog
44. D43: Aboutはproduct名 / version / copyright / website URLだけを表示
45. D44: Typed categoryごとの短いerror message + contextual action
46. D45: DB unavailable categoryごとに原因とprimary recoveryを表示
47. D46: Config startup errorのcategory別message / action
48. D47: Search runtime errorは単一generic message + query変更時のみ再試行
49. D48: Builder runtime errorのcategory別message + Setup復帰 / Open Log
50. D49: Qt / desktop既定fontへ完全に任せる
51. D50: Qt 6の既定scalingへ完全に任せる
52. D51: `QT_NO_XDG_DESKTOP_PORTAL=1`でQtのportal servicesを無効化

整合確認:

2026-09-13にD1〜D50の全文整合確認を実施し、以下を修正済みである。

- Startup dialogのerror detail表示、query logging scope（error logでは禁止、`--debug`の正常系DEBUG行のみ正規化済みqueryを許可）、path表示例外等のdecision間矛盾
- D3概念図の日本語labelとstatus row、推奨マーカー残置、各前文の「後で決定」記述等の古い記述
- 両辞書unavailable時のcombo / Dictionary menu disabled、成功後DB openのtiming統一等の小さなgap
- D47の内部分類closed setは簡素化のため削除し、wrapped errorをそのまま記録する方針へ変更
- 2026-09-14にD51を追加。Qt 6.11の起動時portal host登録warningへの対処であり、D24 / D26 / D30との整合はD51内に明記した。

次の決定順序:

1. 実装milestoneの確定と実装開始

次に Qt Widgets + Go + MIQT で最小 window を作り、起動時間、memory、Wayland、IME、deployment を検証する。
検索画面を作り込む前に、既存検索 service の非同期呼び出しと Qt main thread への安全な結果反映まで確認する。

---

## 11. 参照資料

すべて 2026-09-13 閲覧。ただし、末尾のD51関連資料は2026-09-14閲覧。

- [Qt 6.11 Supported Platforms](https://doc.qt.io/qt-6/supported-platforms.html)
- [Qt for Linux](https://doc.qt.io/qt-6/linux.html)
- [Wayland and Qt](https://doc.qt.io/qt-6/wayland-and-qt.html)
- [Qt for Linux - Deployment](https://doc.qt.io/qt-6/linux-deployment.html)
- [Qt Widgets](https://doc.qt.io/qt-6/qtwidgets-index.html)
- [Qt QIcon](https://doc.qt.io/qt-6/qicon.html)
- [Qt Quick](https://doc.qt.io/qt-6/qtquick-index.html)
- [Qt Releases](https://doc.qt.io/qt-6/qt-releases.html)
- [Qt Licensing](https://doc.qt.io/qt-6/licensing.html)
- [Qt Base platform input contexts](https://code.qt.io/cgit/qt/qtbase.git/tree/src/plugins/platforminputcontexts?h=6.11.2)
- [Fcitx5 Qt](https://github.com/fcitx/fcitx5-qt)
- [MIQT](https://github.com/mappu/miqt)
- [MIQT v0.14.0 mainthread helper](https://github.com/mappu/miqt/tree/v0.14.0/qt6/mainthread)
- [Qt Base qdesktopunixservices.cpp](https://code.qt.io/cgit/qt/qtbase.git/tree/src/gui/platform/unix/qdesktopunixservices.cpp?h=6.11.2)
- [XDG Desktop Portal Registry (org.freedesktop.host.portal.Registry)](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.host.portal.Registry.html)
- [flatpak/xdg-desktop-portal#1612 — Host App registry: Reregister is problematic as a toolkit](https://github.com/flatpak/xdg-desktop-portal/issues/1612)
- [Freedesktop Icon Naming Specification](https://specifications.freedesktop.org/icon-naming-spec/latest/)

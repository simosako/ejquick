# EJQuick 初期Linux GUIリリースチェックリスト

> **対象:** 初期Linux GUI release
>
> **正式target:** Linux `x86_64`、glibc 2.34以降、native Wayland
>
> **Baseline:** Qt 6.11.2、MIQT v0.14.0、Go 1.27.1
>
> **配布形式:** Self-contained directoryを格納した`tar.zst`
>
> **作成日:** 2026-09-16

## 1. 目的と使い方

この文書は[`gui_design.md`](gui_design.md)のD34〜D36を中心に、初期Linux GUI releaseの完了条件を実行可能なchecklistへまとめたものである。

- `[ ]`: 未確認または未完了
- `[x]`: 対象release artifactまたは対象tagに対する確認が完了し、証跡がある
- Sourceに機能が存在するだけでは完了扱いにしない
- Release候補を作り直した場合、artifactに依存する確認は再実施する
- 自動化可能な項目はCIを正とし、実desktopでしか確認できない項目だけを手動確認する

実施結果には、release tag / commit、CI run URL、artifact checksum、手動確認環境を記録する。辞書TXT、生成DB、検索語、entry本文、private pathを証跡へ含めない。

## 2. Release候補の識別

- [ ] Release tagまたはrelease候補versionが決まっている
- [ ] 対象commitがreview済みで、working treeがcleanである
- [x] `ejquick-gui`と同梱`ejquick-build`へ同じproduct versionが埋め込まれている
- [x] Qt、MIQT、Go、C/C++ compiler、Fcitx5 Qt pluginのversion / commitが記録されている
- [x] Build imageの名称と不変な識別子（digest等）が記録されている

証跡:

```text
Version / tag: v0.3.1-15-g470c0eb (CI validation candidate; not a published release)
Commit: 470c0eb2fdeb0b472456907bce96e3088601bbfb
CI run: https://github.com/simosako/ejquick/actions/runs/35065872913
Artifact: ejquick-gui-linux-amd64-v0.3.1-15-g470c0eb (GitHub Actions artifact ID 10435065314)
SHA-256: af22cccbad4a1a1a415507ece03c9f52db9da63319e929761247e9c734775718
Build image / digest: rockylinux/rockylinux@sha256:91bbb8eb52ca462611c1f9ce5c4cede4172a31bfe64f336e82f29648694a3cfe
Qt: 6.11.2
MIQT: v0.14.0
Go: 1.27.1
C/C++ compiler: GCC 11.5.0
Fcitx5 Qt: 5.1.15 (272b6b9eba97f0ed97e8a5caaf0bb22c6123f462)
```

## 3. Build環境

- [x] Release buildがLinux `x86_64`で実行されている
- [x] glibc 2.34相当の固定環境（RHEL 9系相当）でbuildされている
- [x] Qt 6.11.2を固定している
- [x] MIQT v0.14.0を`go.mod` / `go.sum`で固定している
- [x] Go 1.27.1を固定している
- [x] Fcitx5 input context pluginが同じQt 6.11.2向けにbuildされている
- [x] Qt / MIQT / compiler cacheが別Qt installationを参照していない
- [x] BuildがGitHub Actionsまたは十分なmemoryを持つ専用環境で実行されている

6 GiBの開発VPSではQt / MIQTのfull build、`make test-gui`、`make package-gui-linux`をrelease確認として実行しない。

自動化状況（2026-09-16）:

- `.github/workflows/ci.yml`の`gui-package` jobにRocky Linux 9.6 imageをdigest固定したbuild環境を追加済み
- Go 1.27.1、Python 3.12、Qt 6.11.2、MIQT v0.14.0、Fcitx5 Qt 5.1.15のcommitを検査・固定済み
- 初回成功run `35065872913`で固定baseline、Qt導入、Fcitx5 Qt build、package buildを確認済み

## 4. Sourceと既存frontendの品質確認

- [x] `go test ./...`が成功する
- [x] `go test -race ./...`が成功する
- [x] `go vet ./...`が成功する
- [x] `make fmt-check`が成功する
- [x] `make tidy-check`が成功する
- [x] `make bench-smoke`が成功する
- [x] Linux / Windows / macOSのamd64 / arm64 Pure Go cross-buildが成功する
- [x] Qt 6.11.2環境で`make test-gui`が成功する
- [x] Qt 6.11.2環境で`make vet-gui`が成功する
- [ ] `make test-desktop`が成功する
- [ ] 既存のTUI / CLI / Builder release成果物とCLI contractを維持している

原則として、上記は同じrelease tagから起動したGitHub Actionsの成功を証跡とする。

## 5. Package作成

自動化状況（2026-09-16）:

- `gui-package` jobが`make package-gui-linux`を実行し、archive、SHA-256、build metadataを14日間のCI artifactとして保存する
- `packaging/linux/verify-gui-package.sh`がarchive名、単一root、必須file / plugin、version一致、relative RUNPATH、`qt.conf`、辞書artifact非混入を検査する
- 初回成功run `35065872913`のartifactを再downloadし、保存されたSHA-256との一致を確認済み
- 完成archiveを使ったWayland E2E testと再現build比較は次の作業である

- [x] 固定release環境で`make package-gui-linux`が成功する
- [x] 出力名が`ejquick-gui_<version>_linux_amd64.tar.zst`である
- [x] Archiveを展開・一覧検査できる
- [x] Archive rootがversion付きの単一directoryである
- [ ] 同じsource / versionから再作成したarchiveの内容が再現可能である
- [ ] Package作成時の一時fileがrelease artifactへ混入していない

### 必須内容

- [x] `bin/ejquick-gui`
- [x] `bin/ejquick-build`
- [x] `bin/qt.conf`
- [x] 必要なQt shared libraries
- [x] Wayland QPA plugin
- [x] XCB QPA plugin（fallback / best effort）
- [x] Compose input context plugin
- [x] IBus input context plugin
- [x] Fcitx5 input context pluginと必要なFcitx5 Qt addon
- [x] Desktop entry template
- [x] `install-desktop.sh`
- [x] `uninstall-desktop.sh`
- [x] `README.md`
- [x] `LICENSE`
- [x] `THIRD_PARTY_NOTICES`
- [ ] Qt / Fcitx5を含むGUI runtime noticeと必要なlicense文書

### 含めてはいけないもの

- [x] 英辞郎・和英辞郎のTXT / ZIPが含まれていない
- [x] 生成済み辞書DBやSQLite sidecarが含まれていない
- [ ] 実辞書由来のfixture、検索語、entry本文が含まれていない
- [ ] Build host固有のabsolute path、cache、debug artifactが含まれていない
- [x] 不要なQPA plugin（offscreen、minimal、eglfs、linuxfb等）が含まれていない
- [x] Font、input method daemon、graphics driver、glibc等のhost componentが含まれていない

## 6. Binary / runtime dependency検査

- [x] `ejquick-gui --version`と同梱`ejquick-build --version`がrelease versionと一致する
- [ ] `ejquick-gui --help` / `--version`がconfig、DB、Qt platform初期化なしで成功する
- [ ] Executableと同梱library / pluginの`DT_NEEDED` closureを記録している
- [ ] 全dependencyを「bundle」または「host requirement」に分類している
- [ ] 未分類SONAMEがない
- [x] Qt / Fcitx5 Qt dependencyがpackage内で解決される
- [ ] glibc、ELF loader、Wayland / X11、DBus、XKB、font、OpenGL / EGL等の除外対象を誤ってbundleしていない
- [x] Executableと必要なplugin / libraryのrelative RUNPATHが正しい
- [x] Build hostのQt pathやabsolute RPATHが残っていない
- [x] `qt.conf`がpackage内`plugins/`だけを標準探索先にしている
- [x] System Qt pluginとのversion混在がない

Dependency manifest:

```text
Path / URL:
Checksum:
```

## 7. 完成packageの自動E2E test

この節はbuild treeの`tmp/ejquick-gui`ではなく、作成したarchiveを新しいdirectoryへ展開した成果物に対して実施する。

### Clean runtime

- [ ] Build用Qt pathを環境から除外している
- [ ] Package外のQt library / pluginへfallbackした場合はtestが失敗する
- [ ] `QT_QPA_PLATFORM=wayland`を明示し、`DISPLAY`をunsetしている
- [ ] Project-localなmode `0700`の`XDG_RUNTIME_DIR`を使用する
- [ ] Weston headlessがsoftware renderingで起動する
- [ ] Package内Wayland QPA pluginのloadをpath付きで確認する
- [ ] Window生成、表示、正常終了がdeadline内に完了する

### Search GUI

- [ ] 人工データだけからtest DBを生成する
- [ ] Package内の同梱`ejquick-build`で人工DBを生成できる
- [ ] GUIが人工DBをread-onlyでopen / validationできる
- [ ] 起動時に検索欄へfocusでき、主要widget / actionのaccessible nameとenabled stateが正しい
- [ ] 人工queryの検索結果がresult modelへ反映される
- [ ] 選択entryのheadword / bodyがdetail paneへ反映される
- [ ] Up / Down、PageUp / PageDown、Escape、Ctrl+Tab、copy shortcutが設計どおり動作する
- [ ] Mouseによる選択、scroll、text selection、context menuが標準動作を妨げられない
- [ ] Query変更時の非同期検索とstale result排除が機能する
- [ ] Empty / no-result / search-error messageが正しい
- [ ] Application shutdownがhangせず、worker / DB / logをcloseする

### Builder integration

- [ ] GUIとBuilderのproduct / protocol version handshakeが成功する
- [ ] Build成功後にDBを再openし、再起動せず検索できる
- [ ] Graceful cancelでBuilderが終了し、一時DBが残らない
- [ ] 応答しないchildを10秒後にhard killできる
- [ ] Hard kill後に今回通知された安全な一時DBだけをcleanupする
- [ ] Protocol errorを成功扱いしない
- [ ] `output_busy`をcategory付きfailureとして扱う
- [ ] 既存DB置換成功後に旧entryが残らない
- [ ] Build failure / cancel時に既存DBを維持する
- [ ] DB再open failureを成功表示のままにしない
- [ ] 英和・和英の両方で主要flowを確認する

### Plugin startup matrix

- [ ] Compose pluginを指定してstartup / shutdownできる
- [ ] IBus pluginを指定し、daemon不在でもstartupがhangしない
- [ ] Fcitx5 pluginを指定し、daemon不在でもstartupがhangしない
- [ ] XCB QPAでstartup smoke testが成功する（正式X11 supportとは扱わない）

## 8. Desktop integration

- [ ] `install-desktop.sh`がroot権限なしでper-user desktop entryを登録する
- [ ] `Exec` / `TryExec`が展開先のabsolute pathを正しく指す
- [ ] Space、quote、backslash、non-ASCIIを含む展開pathを安全に扱う
- [ ] 安全に表現できない改行等を含むpathを拒否する
- [ ] `desktop-file-validate`が利用可能な環境で成功する
- [ ] Packageを移動した場合の再登録手順がREADMEにある
- [ ] `uninstall-desktop.sh`が対象のper-user entryだけを削除する
- [ ] Scriptがpackage本体、shell profile、`PATH`、`LD_LIBRARY_PATH`を変更しない

## 9. License / privacy / security

- [ ] Bundled Qt moduleが採用license条件で再配布可能である
- [ ] Dynamic linkとlibrary差し替え要件を満たす
- [ ] Qt copyright / license / third-party attributionを同梱する
- [ ] Fcitx5 Qt plugin / addonのcopyright / licenseを同梱する
- [ ] 対応するQt / Fcitx5 source入手方法を記載する
- [ ] `THIRD_PARTY_NOTICES`が固定済みGo dependency graphと一致する
- [ ] Package作成時に必須noticeが見つからなければ失敗する
- [ ] Applicationは起動・検索・buildにnetwork accessを必要としない
- [ ] Telemetry、remote asset、automatic update checkがない
- [ ] 通常error logにquery、headword、entry本文、source TXT内容を記録しない
- [ ] Screenshot、clipboard dump、実辞書をCI artifactへ保存しない

## 10. Clean environment互換性

各環境では、system Qtへ依存せずpackage内Qtでnative Wayland起動、人工DB検索、同梱Builder起動、正常終了を確認する。

- [ ] RHEL 9系相当 / glibc 2.34
- [ ] Ubuntu 22.04
- [ ] Ubuntu 24.04
- [ ] Debian 12
- [ ] 最低対応環境で不足SONAMEや新しいglibc symbol requirementがない
- [ ] CJK表示可能なsystem fontがある環境で日本語headword / bodyを表示できる
- [ ] CJK fontをpackageへ暗黙にbundleしていない

## 11. 実desktop手動smoke test

Wayland headless CIでは実IME candidate UI、compositorのfocus policy、実clipboardを完全には検証できないため、release候補artifactで次を実施する。

### KDE Plasma / KWin + Fcitx5

- [ ] Native Waylandでpackageを起動する
- [ ] 日本語preeditを表示できる
- [ ] Candidate選択、確定、cancelが機能する
- [ ] IME変換中の矢印、Enter、Escape、PageUp / PageDownをGUI shortcutが奪わない
- [ ] 変換確定後にincremental searchが動作する
- [ ] Clipboardへのentry全体copyが機能する
- [ ] TXT file pickerが機能する
- [ ] Builder成功・cancelを確認する
- [ ] Window closeと明示Quitがhangしない
- [ ] 100% / 200% scalingで主要controlが欠けない

環境記録:

```text
Distribution:
Compositor / version:
Fcitx5 / engine / version:
Scale:
Tester:
Date:
Result / notes:
```

### GNOME / Mutter + IBus

- [ ] Native Waylandでpackageを起動する
- [ ] 日本語preeditを表示できる
- [ ] Candidate選択、確定、cancelが機能する
- [ ] IME変換中の矢印、Enter、Escape、PageUp / PageDownをGUI shortcutが奪わない
- [ ] 変換確定後にincremental searchが動作する
- [ ] Clipboardへのentry全体copyが機能する
- [ ] TXT file pickerが機能する
- [ ] Builder成功・cancelを確認する
- [ ] Window closeと明示Quitがhangしない
- [ ] 100% / 200% scalingで主要controlが欠けない

環境記録:

```text
Distribution:
Compositor / version:
IBus / engine / version:
Scale:
Tester:
Date:
Result / notes:
```

## 12. 性能baseline

D8に従い固定値のrelease gateにはしないが、同一reference環境で測定して次回比較のbaselineを残す。

- [ ] Reference machine / VM、distribution、compositor、storage、Qt / MIQT versionを記録する
- [ ] Warm startup p50 / p95
- [ ] Cold startup p50 / p95
- [ ] Window表示中のidle RSS / PSS
- [ ] 代表的な検索のlatency p50 / p95
- [ ] 100回以上検索後のRSS / PSS
- [ ] Archive size
- [ ] 展開後package size
- [ ] 顕著な遅延・memory増加・package肥大化がないか確認する
- [ ] 測定に実辞書を使った場合、そのDB・query・entry内容をrepository / CI artifactへ含めない

性能記録:

```text
Report path / URL:
Reference environment:
Summary:
```

## 13. GitHub Release公開

- [ ] Release workflowが通常CIの全job成功後にだけGUI packageを公開する
- [ ] 公開対象archiveが上記checklistを通過したものと同一である
- [ ] GUI packageを`checksums.txt`へ含める
- [ ] 公開前にarchive checksumを記録する
- [ ] Release assetsに辞書TXT / DBやprivateなtest artifactがない
- [ ] Release notesに正式対応範囲をLinux `x86_64` / glibc 2.34以降 / native Waylandと記載する
- [ ] XCB / XWaylandをfallback / best effortとし、正式対応と表記しない
- [ ] Host側に必要なdisplay server、graphics、font、DBus、input method daemon等を記載する
- [ ] GUI、同梱Builder、Pure Go版のproduct versionが同じである
- [ ] 公開後にrelease assetを新規downloadし、checksumとarchive一覧を最終確認する

公開証跡:

```text
Release URL:
GUI asset:
SHA-256:
Published at:
Final verifier:
```

## 14. Release判定

### Blocker

次のいずれかに該当する場合は公開しない。

- 必須build / test jobが失敗または未実施
- 完成packageではなくbuild treeだけをtestしている
- glibc 2.34最低環境で起動できない
- Package外Qtへのfallbackが必要
- Builderの成功、cancel、既存DB保護のいずれかを確認できない
- Native Waylandまたは日本語IMEの実環境確認が失敗
- 必須runtime dependencyまたはlicense noticeが不足
- 辞書TXT、生成DB、実辞書由来dataがartifactへ混入
- Versionまたはchecksumが一致しない

### 非blockerだが記録する項目

- D8の性能値が以前より悪化したが、原因・影響を確認して許容した場合
- XCB / XWayland固有の問題
- HostにCJK fontがない場合のtofu表示
- Package外pluginを利用者が明示追加したunsupported構成

### 最終承認

- [ ] 自動checkがすべて成功した
- [ ] 手動smoke testがすべて成功した
- [ ] Blockerが残っていない
- [ ] 非blockerの既知問題をrelease notesへ記載した
- [ ] Release ownerが公開を承認した

```text
Release decision: GO / NO-GO
Version:
Commit:
Approved by:
Date:
Notes:
```

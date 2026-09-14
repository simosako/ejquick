# EJQuick

A fast local dictionary search tool for the [EIJIRO](https://booth.pm/) /
WAEIJI English-Japanese and Japanese-English dictionaries, with a fzf-style
incremental TUI and a scriptable CLI.

- **Linux / Windows / macOS**, single static binary, pure Go (no CGo)
- Converts the purchased dictionary TXT into SQLite once, then searches it
  read-only
- Prefix search on a B-tree index, substring search on FTS5 trigram
- Incremental search that updates on every keystroke

> EJQuick ships **no dictionary data**. Bring your own legally purchased
> EIJIRO / WAEIJI TXT files.

## Install

Download a binary for your platform from
[GitHub Releases](https://github.com/simosako/ejquick/releases), or build
from source:

```bash
make build                    # current platform, into tmp/
make build VERSION=v0.1.0     # optionally stamp a local build
```

Or with plain `go build`:

```bash
VERSION=$(git describe --tags --always --dirty)
go build -ldflags "-X github.com/simosako/ejquick/internal/buildinfo.Version=$VERSION" ./cmd/ejquick
go build -ldflags "-X github.com/simosako/ejquick/internal/buildinfo.Version=$VERSION" ./cmd/ejquick-build
```

## Quick start

1. Build a dictionary database from a purchased TXT file (CP932 encoded):

```bash
ejquick-build --type eiwa EIJIRO144-10.TXT
ejquick-build --type waei WAEIJI-144-10.TXT
```

By default, databases are written as `eiwa.sqlite3` or `waei.sqlite3` in
the platform data directory under `ejquick/`; missing directories are
created automatically. Use `--output <path>` to choose another location.
Progress goes to stderr; the database is built in a temporary file,
validated read-only, and only then published atomically. `--force`
replaces an existing database; `--compact` additionally runs `VACUUM` for
a smaller file.

2. Search:

```bash
ejquick          # start the TUI
```

Type to search. `Up`/`Down` (or `Ctrl-P`/`Ctrl-N`) move the selection,
`PageUp`/`PageDown` scroll long entries, `Tab` switches between the
English-Japanese and Japanese-English dictionaries, `Ctrl-C` quits.

## CLI search

```bash
ejquick english
ejquick --dictionary waei 経済
ejquick --format jsonl --limit 20 care
```

Options:

```text
-d, --dictionary <eiwa|waei>    dictionary to search
-c, --config <path>             config file path
    --limit <1..500>            result limit for this process
    --format <plain|jsonl>      output format (CLI search only)
    --debug                     enable debug logging to the log file
-h, --help                      show help
-v, --version                   show version
```

Exit codes follow the grep convention: `0` results found, `1` no results,
`2` error.

## Configuration

Optional TOML file (created by you, not by EJQuick):

```text
Linux:   ~/.config/ejquick/config.toml
macOS:   ~/Library/Application Support/ejquick/config.toml
Windows: %AppData%\ejquick\config.toml
```

```toml
[eiwa]
database = "~/.local/share/ejquick/eiwa.sqlite3"

[waei]
database = "~/.local/share/ejquick/waei.sqlite3"

[search]
default_dictionary = "eiwa"
max_results = 50
```

Databases omitted from the config default to the platform data directory
under `ejquick/`. `max_results` must be 1..500.

## Logging

Errors are appended to a log file (best effort; a broken log never stops
the app):

```text
Linux:   ~/.local/state/ejquick/ejquick.log
macOS:   ~/Library/Application Support/ejquick/ejquick.log
Windows: %AppData%\ejquick\ejquick.log
```

Run with `--debug` to also log every search (dictionary, request ID,
query, result count, elapsed time). The builder writes its fixed-line
progress to stderr only and does not use the log file.

## Development

Run the standard checks and artificial-data benchmarks with:

```bash
make test
make test-race
make vet
make tidy-check
make bench
make release-check
```

`make release-check` requires GoReleaser v2.18.0 or later. It runs a local
snapshot and does not publish anything.

### GUI (under development)

The Qt 6 Widgets desktop frontend (`ejquick-gui`, design in
[`design/gui_design.md`](design/gui_design.md)) is built behind the `gui`
build tag so every target above stays pure Go and Qt-free (design D35).
Building or testing it additionally requires CGO, a C++ compiler,
`pkg-config`, and Qt 6 development packages (baseline: Qt 6.11.2,
MIQT v0.14.0):

```bash
make build-gui   # tmp/ejquick-gui
make test-gui    # widget tests run on the offscreen platform
make vet-gui
```


See [`bench/README.md`](bench/README.md) for reproducible real-data
measurements. The helper only reads the purchased TXT path supplied to it;
all generated SQLite databases and reports stay under the ignored `tmp/`
directory and are never added to the repository.

## Releasing

EJQuick uses one Semantic Versioning product version for all executables and
frontends. Database `schema_version`, `normalization_version`, and
`fts_version` values are compatibility versions managed independently from the
product version.

The release Git tag is the source of truth. After the release commit has been
reviewed, run the project-local mise task and enter the desired version when
prompted:

```bash
mise run release
# Enter a version such as v0.1.0 when prompted.
```

The task checks that the working tree is clean and that the tag does not already
exist, then runs the equivalent commands:

```bash
git tag -a v0.1.0 -m "EJQuick v0.1.0"
git push origin v0.1.0
```

Pushing a `v*` tag runs the complete CI workflow. If CI succeeds, GoReleaser
builds Linux, Windows, and macOS archives for amd64 and arm64, stamps the same
tag into both executables, and publishes the archives and `checksums.txt` to
GitHub Releases. Ordinary branch pushes do not run CI; pull requests and
manual workflow dispatches still do.

## License

MIT. See [LICENSE](LICENSE). Third-party notices:
[THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).

EJQuick's MIT license covers only this project's code and documentation.
The dictionary TXT data you convert, and the SQLite databases you build
from it, remain governed by the terms of your dictionary purchase and
must not be redistributed.

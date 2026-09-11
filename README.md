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
make build VERSION=v0.1.0     # stamp a version
make release VERSION=v0.1.0   # cross-build dist/ archives for all targets
```

Or with plain `go build`:

```bash
go build -ldflags "-X main.version=$(git describe --tags --always)" ./cmd/ejquick
go build -ldflags "-X main.version=$(git describe --tags --always)" ./cmd/ejquick-build
```

## Quick start

1. Build a dictionary database from a purchased TXT file (CP932 encoded):

```bash
ejquick-build --type eiji --input EIJIRO144-10.TXT --output ~/.local/share/ejquick/eiji.sqlite3
ejquick-build --type waei --input WAEIJI-144-10.TXT --output ~/.local/share/ejquick/waei.sqlite3
```

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
-d, --dictionary <eiji|waei>    dictionary to search
-c, --config <path>             config file path
    --limit <1..500>            result limit for this process
    --format <plain|jsonl>      output format (CLI search only)
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
[eiji]
database = "~/.local/share/ejquick/eiji.sqlite3"

[waei]
database = "~/.local/share/ejquick/waei.sqlite3"

[search]
default_dictionary = "eiji"
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

## License

MIT. See [LICENSE](LICENSE). Third-party notices:
[THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).

EJQuick's MIT license covers only this project's code and documentation.
The dictionary TXT data you convert, and the SQLite databases you build
from it, remain governed by the terms of your dictionary purchase and
must not be redistributed.

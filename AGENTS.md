# what's this project?

A TUI/CLI dictionary application. It converts TXT-format dictionary data
into SQLite databases and provides fast search over them.

# related files

The design lives under `design/`. Consult it before making changes that
touch the architecture, search strategy, or data formats.

The source dictionary data is stored under `source/` as `*.TXT` files.
This data was purchased from Booth and must never be distributed or
published externally (GitHub, releases, test fixtures, etc.).

- `EIJIRO144-10.TXT` : English-Japanese dictionary data
- `WAEIJI-144-10.TXT` : Japanese-English dictionary data

# scratch directory: tmp/

The agent runs inside a firejail sandbox, so `/tmp` as seen by the agent
is NOT the same directory as `/tmp` seen by the user. Never place build
artifacts, databases, configs, or anything the user should inspect under
`/tmp`.

Instead, use the project-local `tmp/` directory for all throwaway and
verification artifacts:

- Built binaries (`ejquick`, `ejquick-build`)
- Generated SQLite databases (`eiji.sqlite3`, `waei.sqlite3`)
- Test config files (`config.toml`)
- Any other build or run output used for verification

`tmp/` is listed in `.gitignore` and is never committed. When running
`ejquick` for verification, point it at `tmp/` explicitly, for example:

```bash
go build -o tmp/ejquick ./cmd/ejquick
tmp/ejquick -c tmp/config.toml <query>
```

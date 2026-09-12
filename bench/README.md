# Performance Benchmarks

All repository benchmarks use artificial data created by the test itself.
They never read files under `source/` and are safe to run in CI:

```bash
make bench
```

Use a single iteration to verify that benchmark setup still works:

```bash
make bench-smoke
```

## Purchased Data

Real dictionary data must remain outside the repository. The helper below
builds normal and compact databases under the ignored `tmp/` directory,
records build timing and resource output, samples peak build-directory use,
and runs search benchmarks against the normal database:

```bash
bench/run-real.sh --type eiji --input /path/to/EIJIRO.TXT
bench/run-real.sh --type waei --input /path/to/WAEIJI.TXT
```

Queries are separated by `|`. Supply representative local queries when the
defaults do not exercise the intended result sets:

```bash
bench/run-real.sh \
  --type waei \
  --input /path/to/WAEIJI.TXT \
  --queries '日|日本|日本語' \
  --count 10
```

The command prints the `tmp/benchmark-real-*` result directory. It contains
the generated databases, raw builder/time output, search benchmark output,
an automated summary, and a copy of `results-template.md`. Do not move the
TXT or generated SQLite files into a tracked directory.

To benchmark an already-built local database directly:

```bash
EJQUICK_BENCH_EIJI_DB=/path/to/eiji.sqlite3 \
EJQUICK_BENCH_EIJI_QUERIES='e|en|eng' \
go test -run '^$' -bench '^BenchmarkReal' -benchmem -count 10 ./bench
```

Use `EJQUICK_BENCH_WAEI_DB` and `EJQUICK_BENCH_WAEI_QUERIES` for WAEIJI.
The benchmark names include only dictionary type, query sequence number, and
rune count; query text and dictionary content are not printed.

Cold-cache measurements and DB-open-to-first-TUI-render measurements are not
automated because portable cache eviction is unavailable and often requires
administrator privileges. Record those platform-specific measurements in
the template without adding local dictionary artifacts to Git.

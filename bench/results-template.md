# EJQuick Real-Data Benchmark

Do not attach or commit the purchased TXT, generated SQLite databases, or
logs containing local paths. This template contains measurements only.

## Environment

| Field | Value |
|---|---|
| Date (UTC) | |
| Git revision | |
| OS and version | |
| Architecture | |
| Go version | |
| CPU | |
| RAM | |
| Storage type | |
| Dictionary type and source version | |
| Source TXT bytes | |
| Source lines | |

## Builder

| Mode | Build time | DB bytes | Maximum RSS (value/unit) | Peak build-directory KiB |
|---|---:|---:|---:|---:|
| Normal | | | | |
| Compact | | | | |

| Component | Normal bytes | Compact bytes | Measurement method |
|---|---:|---:|---|
| entries table | | | `dbstat` or equivalent |
| B-tree index | | | `dbstat` or equivalent |
| FTS5 index | | | `dbstat` or equivalent |
| Total database | | | file size |

## Search

Record `benchstat` output or at least median latency and allocations. Note
whether the OS page cache was cold or warm.

| Dictionary | Query class | Match path | Cache | ns/op | B/op | allocs/op | Results |
|---|---|---|---|---:|---:|---:|---:|
| Eiji | 1 rune | prefix | warm | | | | |
| Eiji | 2 runes | prefix | warm | | | | |
| Eiji | 3+ runes | prefix | warm | | | | |
| Eiji | 3+ runes | substring | warm | | | | |
| Waei | 1 rune | prefix | warm | | | | |
| Waei | 2 runes | prefix | warm | | | | |
| Waei | 3+ runes | substring | warm | | | | |
| Either | representative | auto | cold | | | | |

## Startup

| Measurement | Cold | Warm | Method |
|---|---:|---:|---|
| Repository open and schema verification | | | |
| DB open to first TUI render | | | |

## Notes

- Query selection and result-set characteristics:
- Background load and power mode:
- Cache preparation or eviction method:
- Unexpected warnings, skipped lines, or other observations:

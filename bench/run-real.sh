#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: bench/run-real.sh --type <eiji|waei> --input <TXT> [--queries <q1|q2|q3>] [--count <n>]

Builds normal and compact databases under tmp/, records build resource
usage, and runs the real-database search benchmarks. No source or database
is copied outside tmp/.
EOF
}

require_value() {
  if [[ $# -lt 2 ]]; then
    echo "$1 requires a value" >&2
    usage >&2
    exit 2
  fi
}

dict_type=""
input=""
queries=""
count=5
while [[ $# -gt 0 ]]; do
  case "$1" in
    --type)
      require_value "$@"
      dict_type="$2"
      shift 2
      ;;
    --input)
      require_value "$@"
      input="$2"
      shift 2
      ;;
    --queries)
      require_value "$@"
      queries="$2"
      shift 2
      ;;
    --count)
      require_value "$@"
      count="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$dict_type" != "eiji" && "$dict_type" != "waei" ]]; then
  echo "--type must be eiji or waei" >&2
  exit 2
fi
if [[ -z "$input" || ! -f "$input" ]]; then
  echo "--input must name an existing TXT file" >&2
  exit 2
fi
if ! [[ "$count" =~ ^[1-9][0-9]*$ ]]; then
  echo "--count must be a positive integer" >&2
  exit 2
fi
if [[ -z "$queries" ]]; then
  if [[ "$dict_type" == "eiji" ]]; then
    queries="e|en|eng"
  else
    queries="日|日本|日本語"
  fi
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
run_dir="$root/tmp/benchmark-real-$(date +%Y%m%d-%H%M%S)-$$"
mkdir -p "$run_dir"
summary="$run_dir/summary.txt"
cp "$root/bench/results-template.md" "$run_dir/results.md"

builder_bin="$run_dir/ejquick-build"
(
  cd "$root"
  CGO_ENABLED=0 go build -o "$builder_bin" ./cmd/ejquick-build
)

if /usr/bin/time --version 2>&1 | grep -q GNU; then
  time_command=(/usr/bin/time -v)
elif [[ "$(uname -s)" == "Darwin" ]]; then
  time_command=(/usr/bin/time -l)
else
  time_command=(/usr/bin/time)
fi

printf 'date_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$summary"
printf 'git_revision=%s\n' "$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)" >>"$summary"
printf 'platform=%s\n' "$(uname -a)" >>"$summary"
printf 'go_version=%s\n' "$(go version)" >>"$summary"
printf 'dictionary_type=%s\n' "$dict_type" >>"$summary"
printf 'input_bytes=%s\n' "$(wc -c <"$input" | tr -d ' ')" >>"$summary"

last_db=""
run_build() {
  local label="$1"
  shift
  local db_dir="$run_dir/$label-db"
  local output="$db_dir/$dict_type.sqlite3"
  local stdout_file="$run_dir/$label.stdout"
  local metrics_file="$run_dir/$label.metrics"
  local peak_kib=0
  local current_kib=0
  local status=0
  mkdir -p "$db_dir"

  "${time_command[@]}" "$builder_bin" \
    --type "$dict_type" --input "$input" --output "$output" "$@" \
    >"$stdout_file" 2>"$metrics_file" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    current_kib="$(du -sk "$db_dir" | awk '{print $1}')"
    if (( current_kib > peak_kib )); then
      peak_kib=$current_kib
    fi
    sleep 0.2
  done
  set +e
  wait "$pid"
  status=$?
  set -e
  current_kib="$(du -sk "$db_dir" | awk '{print $1}')"
  if (( current_kib > peak_kib )); then
    peak_kib=$current_kib
  fi

  if [[ -s "$stdout_file" ]]; then
    echo "$label build unexpectedly wrote to stdout" >&2
    exit 1
  fi
  if (( status != 0 )); then
    echo "$label build failed; see $metrics_file" >&2
    exit "$status"
  fi

  printf '%s_build_elapsed=%s\n' "$label" \
    "$(sed -n 's/^Elapsed: //p' "$metrics_file" | head -n 1)" >>"$summary"
  printf '%s_db_bytes=%s\n' "$label" "$(wc -c <"$output" | tr -d ' ')" >>"$summary"
  printf '%s_peak_directory_kib=%d\n' "$label" "$peak_kib" >>"$summary"
  if grep -q 'Maximum resident set size (kbytes):' "$metrics_file"; then
    printf '%s_max_rss_kib=%s\n' "$label" \
      "$(awk -F: '/Maximum resident set size \(kbytes\):/ {gsub(/ /, "", $2); print $2; exit}' "$metrics_file")" \
      >>"$summary"
  elif grep -q 'maximum resident set size' "$metrics_file"; then
    printf '%s_max_rss_bytes=%s\n' "$label" \
      "$(awk '/maximum resident set size/ {print $1; exit}' "$metrics_file")" >>"$summary"
  fi
  printf '%s_metrics_file=%s\n' "$label" "$(basename "$metrics_file")" >>"$summary"
  last_db="$output"
}

run_build normal
normal_db="$last_db"
run_build compact --compact

if [[ "$dict_type" == "eiji" ]]; then
  db_env="EJQUICK_BENCH_EIJI_DB"
  query_env="EJQUICK_BENCH_EIJI_QUERIES"
else
  db_env="EJQUICK_BENCH_WAEI_DB"
  query_env="EJQUICK_BENCH_WAEI_QUERIES"
fi

(
  cd "$root"
  env "$db_env=$normal_db" "$query_env=$queries" \
    go test -run '^$' -bench '^BenchmarkReal' -benchmem -count "$count" ./bench
) | tee "$run_dir/search-benchmark.txt"

echo "Benchmark artifacts: $run_dir"
echo "Automated summary: $summary"
echo "Copy measurements into: $run_dir/results.md"

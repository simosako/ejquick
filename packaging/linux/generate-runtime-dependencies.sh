#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
rules_file=$script_dir/runtime-dependencies.tsv
package_root=${1:-}
output=${2:-}

fail() {
	printf 'generate-runtime-dependencies.sh: %s\n' "$*" >&2
	exit 1
}

[[ -n $package_root && -d $package_root ]] || fail 'usage: generate-runtime-dependencies.sh PACKAGE_ROOT OUTPUT'
[[ -n $output ]] || fail 'usage: generate-runtime-dependencies.sh PACKAGE_ROOT OUTPUT'
[[ -f $rules_file ]] || fail "dependency rules are missing: $rules_file"
command -v find >/dev/null 2>&1 || fail 'required command is missing: find'
command -v readelf >/dev/null 2>&1 || fail 'required command is missing: readelf'

classify() {
	local soname=$1
	local pattern disposition component
	local matches=0
	while IFS=$'\t' read -r pattern disposition component; do
		[[ -n $pattern && $pattern != \#* ]] || continue
		if [[ $soname == $pattern ]]; then
			printf '%s\t%s\n' "$disposition" "$component"
			((matches += 1))
		fi
	done <"$rules_file"
	(( matches == 1 )) || fail "SONAME must match exactly one dependency rule: $soname (matches: $matches)"
}

temporary=$(mktemp "${output}.tmp.XXXXXX")
cleanup() {
	rm -f -- "$temporary"
}
trap cleanup EXIT

{
	printf '# ELF_PATH\tDT_NEEDED\tDISPOSITION\tCOMPONENT\n'
	while IFS= read -r -d '' file; do
		readelf -h "$file" >/dev/null 2>&1 || continue
		relative=${file#"$package_root/"}
		while IFS= read -r soname; do
			[[ -n $soname ]] || continue
			classification=$(classify "$soname")
			disposition=${classification%%$'\t'*}
			component=${classification#*$'\t'}
			if [[ $disposition == bundle ]]; then
				[[ -f $package_root/lib/$soname ]] || fail "bundled dependency is missing: $relative -> $soname"
			elif [[ $disposition == host ]]; then
				[[ ! -e $package_root/lib/$soname ]] || fail "host dependency was bundled unexpectedly: $soname"
			else
				fail "invalid dependency disposition for $soname: $disposition"
			fi
			printf '%s\t%s\t%s\t%s\n' "$relative" "$soname" "$disposition" "$component"
		done < <(readelf -d "$file" | sed -n 's/.*Shared library: \[\([^]]*\)\].*/\1/p')
	done < <(find "$package_root/bin" "$package_root/lib" "$package_root/plugins" -type f -print0)
} | sort -u >"$temporary"

grep -q $'\tbundle\t' "$temporary" || fail 'dependency manifest contains no bundled dependencies'
grep -q $'\thost\t' "$temporary" || fail 'dependency manifest contains no host dependencies'
mv -- "$temporary" "$output"
trap - EXIT

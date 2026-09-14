#!/usr/bin/env bash

set -euo pipefail

application_id=io.github.simosako.ejquick
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
executable=$script_dir/bin/ejquick-gui
template=$script_dir/share/applications/$application_id.desktop.in

fail() {
	printf 'install-desktop.sh: %s\n' "$*" >&2
	exit 1
}

require_safe_absolute_path() {
	local value=$1
	local label=$2
	[[ $value == /* ]] || fail "$label must be an absolute path"
	[[ ! $value =~ [[:cntrl:]] ]] || fail "$label contains a control character"
}

desktop_exec_quote() {
	local value=$1
	local output='"'
	local character
	local index
	for ((index = 0; index < ${#value}; index++)); do
		character=${value:index:1}
		case $character in
		'\') output+='\\\\' ;;
		'"') output+='\\"' ;;
		'$') output+='\\$' ;;
		'`') output+='\\`' ;;
		'%') output+='%%' ;;
		*) output+=$character ;;
		esac
	done
	output+='"'
	printf '%s' "$output"
}

desktop_string_escape() {
	local value=$1
	local output=
	local character
	local index
	for ((index = 0; index < ${#value}; index++)); do
		character=${value:index:1}
		if [[ $character == '\' ]]; then
			output+='\\'
		else
			output+=$character
		fi
	done
	printf '%s' "$output"
}

require_safe_absolute_path "$executable" 'GUI executable path'
[[ $executable != *'='* ]] || fail 'GUI executable path contains an unsupported equals sign'
[[ $executable != *'%'* ]] || fail 'GUI executable path contains an unsupported percent sign'
[[ -x $executable && -f $executable ]] || fail "GUI executable is missing or is not executable: $executable"
[[ -f $template ]] || fail "desktop entry template is missing: $template"

if [[ -n ${XDG_DATA_HOME:-} ]]; then
	data_home=$XDG_DATA_HOME
else
	[[ -n ${HOME:-} ]] || fail 'HOME is not set and XDG_DATA_HOME is empty'
	data_home=$HOME/.local/share
fi
require_safe_absolute_path "$data_home" 'desktop data directory'

applications_dir=$data_home/applications
target=$applications_dir/$application_id.desktop
mkdir -p -- "$applications_dir"
tmp=$(mktemp "$applications_dir/.$application_id.XXXXXX.desktop")
cleanup() {
	if [[ -n ${tmp:-} && -e $tmp ]]; then
		rm -f -- "$tmp"
	fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

quoted_executable=$(desktop_exec_quote "$executable")
try_executable=$(desktop_string_escape "$executable")
while IFS= read -r line || [[ -n $line ]]; do
	case $line in
	'@TRYEXEC@') printf 'TryExec=%s\n' "$try_executable" ;;
	'@EXEC@') printf 'Exec=%s\n' "$quoted_executable" ;;
	*) printf '%s\n' "$line" ;;
	esac
done <"$template" >"$tmp"
chmod 0644 "$tmp"

if command -v desktop-file-validate >/dev/null 2>&1; then
	desktop-file-validate "$tmp"
fi
mv -f -- "$tmp" "$target"
tmp=

if command -v update-desktop-database >/dev/null 2>&1; then
	if ! update-desktop-database "$applications_dir"; then
		printf 'install-desktop.sh: warning: desktop database update failed\n' >&2
	fi
fi
printf 'Installed %s\n' "$target"

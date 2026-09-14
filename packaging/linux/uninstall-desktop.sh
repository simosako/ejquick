#!/usr/bin/env bash

set -euo pipefail

application_id=io.github.simosako.ejquick

fail() {
	printf 'uninstall-desktop.sh: %s\n' "$*" >&2
	exit 1
}

require_safe_absolute_path() {
	local value=$1
	local label=$2
	[[ $value == /* ]] || fail "$label must be an absolute path"
	[[ ! $value =~ [[:cntrl:]] ]] || fail "$label contains a control character"
}

if [[ -n ${XDG_DATA_HOME:-} ]]; then
	data_home=$XDG_DATA_HOME
else
	[[ -n ${HOME:-} ]] || fail 'HOME is not set and XDG_DATA_HOME is empty'
	data_home=$HOME/.local/share
fi
require_safe_absolute_path "$data_home" 'desktop data directory'

applications_dir=$data_home/applications
target=$applications_dir/$application_id.desktop
rm -f -- "$target"

if [[ -d $applications_dir ]] && command -v update-desktop-database >/dev/null 2>&1; then
	if ! update-desktop-database "$applications_dir"; then
		printf 'uninstall-desktop.sh: warning: desktop database update failed\n' >&2
	fi
fi
printf 'Removed %s\n' "$target"

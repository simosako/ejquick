#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repository=$(cd -- "$script_dir/../.." && pwd -P)

archive=${1:-}
expected_version=${2:-}

fail() {
	printf 'verify-gui-package.sh: %s\n' "$*" >&2
	exit 1
}

[[ -n $archive ]] || fail 'usage: verify-gui-package.sh ARCHIVE EXPECTED_VERSION'
[[ -n $expected_version ]] || fail 'usage: verify-gui-package.sh ARCHIVE EXPECTED_VERSION'
[[ -f $archive ]] || fail "archive does not exist: $archive"
[[ $expected_version != */* && $expected_version != *$'\n'* && $expected_version != *$'\r'* ]] || \
	fail 'EXPECTED_VERSION contains an unsafe path character'

for command in cmp find go grep readelf sed sort tar zstd; do
	command -v "$command" >/dev/null 2>&1 || fail "required command is missing: $command"
done

root_name="ejquick-gui_${expected_version}_linux_amd64"
expected_archive_name="$root_name.tar.zst"
[[ $(basename -- "$archive") == "$expected_archive_name" ]] || \
	fail "unexpected archive name: $(basename -- "$archive")"

mkdir -p -- "$repository/tmp"
work_dir=$(mktemp -d "$repository/tmp/verify-gui-package.XXXXXX")
cleanup() {
	rm -rf -- "$work_dir"
}
trap cleanup EXIT

listing=$work_dir/archive.list
tar -I zstd -tf "$archive" >"$listing"
[[ -s $listing ]] || fail 'archive is empty'

while IFS= read -r path; do
	[[ $path != /* ]] || fail "archive contains an absolute path: $path"
	case "/$path/" in
	*/../*) fail "archive contains a parent-directory path: $path" ;;
	esac
	[[ $path == "$root_name" || $path == "$root_name/"* ]] || \
		fail "archive contains a path outside the expected root: $path"
done <"$listing"

tar -I zstd -xf "$archive" -C "$work_dir"
package_root=$work_dir/$root_name
[[ -d $package_root ]] || fail "archive root is missing: $root_name"

required_files=(
	bin/ejquick-gui
	bin/ejquick-build
	bin/qt.conf
	plugins/platforms/libqxcb.so
	plugins/platforminputcontexts/libcomposeplatforminputcontextplugin.so
	plugins/platforminputcontexts/libibusplatforminputcontextplugin.so
	plugins/platforminputcontexts/libfcitx5platforminputcontextplugin.so
	share/applications/io.github.simosako.ejquick.desktop.in
	install-desktop.sh
	uninstall-desktop.sh
	README.md
	LICENSE
	THIRD_PARTY_NOTICES
	DEPENDENCIES.tsv
	GO-DEPENDENCIES.tsv
	SOURCE-COMPONENTS.txt
	licenses/GUI-RUNTIME-NOTICES.txt
	licenses/Qt/qtbase/LGPL-3.0-only.txt
	licenses/Qt/qtbase/GPL-3.0-only.txt
	licenses/Fcitx5-Qt/LGPL-2.1-or-later.txt
	licenses/ICU/LICENSE
)
for path in "${required_files[@]}"; do
	[[ -f $package_root/$path ]] || fail "required package file is missing: $path"
done

shopt -s nullglob
wayland_plugins=("$package_root/plugins/platforms"/libqwayland*.so)
qt_libraries=("$package_root/lib"/libQt6*.so.*)
icu_libraries=("$package_root/lib"/libicu*.so.*)
shopt -u nullglob
(( ${#wayland_plugins[@]} > 0 )) || fail 'Wayland QPA plugin is missing'
(( ${#qt_libraries[@]} > 0 )) || fail 'Qt runtime libraries are missing'
(( ${#icu_libraries[@]} > 0 )) || fail 'ICU runtime libraries are missing'

expected_gui_version="ejquick-gui $expected_version"
expected_builder_version="ejquick-build $expected_version"
[[ $("$package_root/bin/ejquick-gui" --version) == "$expected_gui_version" ]] || \
	fail 'ejquick-gui version does not match the archive version'
[[ $("$package_root/bin/ejquick-build" --version) == "$expected_builder_version" ]] || \
	fail 'ejquick-build version does not match the archive version'

grep -Fxq 'Plugins=../plugins' "$package_root/bin/qt.conf" || \
	fail 'qt.conf does not restrict the standard plugin path to the package'
readelf -d "$package_root/bin/ejquick-gui" | \
	grep -Eq 'RUNPATH.*\$ORIGIN/../lib|RPATH.*\$ORIGIN/../lib' || \
	fail 'ejquick-gui does not contain the expected relative library path'

while IFS= read -r file; do
	while IFS= read -r runtime_path; do
		IFS=: read -r -a path_entries <<<"$runtime_path"
		for path_entry in "${path_entries[@]}"; do
			case $path_entry in
			''|'$ORIGIN'|'$ORIGIN/'*) ;;
			*) fail "ELF file contains a non-relative runtime path: ${file#"$package_root/"} -> $path_entry" ;;
			esac
		done
	done < <(readelf -d "$file" 2>/dev/null | sed -n 's/.*\(RPATH\|RUNPATH\).*\[\([^]]*\)\].*/\2/p')
done < <(find "$package_root/bin" "$package_root/lib" "$package_root/plugins" -type f -print)

generated_dependencies=$work_dir/DEPENDENCIES.tsv
"$script_dir/generate-runtime-dependencies.sh" "$package_root" "$generated_dependencies"
cmp -s "$package_root/DEPENDENCIES.tsv" "$generated_dependencies" || \
	fail 'DEPENDENCIES.tsv does not match the packaged ELF dependency graph'

generated_go_dependencies=$work_dir/GO-DEPENDENCIES.tsv
{
	printf '# MODULE\tVERSION\n'
	for binary in "$package_root/bin/ejquick-gui" "$package_root/bin/ejquick-build"; do
		go version -m "$binary" | sed -n 's/^\tdep\t\([^\t]*\)\t\([^\t]*\).*/\1\t\2/p'
	done
} | sort -u >"$generated_go_dependencies"
cmp -s "$package_root/GO-DEPENDENCIES.tsv" "$generated_go_dependencies" || \
	fail 'GO-DEPENDENCIES.tsv does not match the packaged binaries'
while IFS=$'\t' read -r module module_version; do
	[[ -n $module && $module != \#* ]] || continue
	grep -Fq "  $module $module_version" "$package_root/THIRD_PARTY_NOTICES" || \
		fail "THIRD_PARTY_NOTICES is missing a linked Go module: $module $module_version"
done <"$generated_go_dependencies"

while IFS= read -r path; do
	case ${path^^} in
	EIJIRO*.TXT|WAEIJI*.TXT|*.ZIP|*.SQLITE|*.SQLITE3|*.SQLITE-*|*.SQLITE3-*|SOURCE/*|*/SOURCE/*)
		fail "forbidden dictionary artifact in package: $path"
		;;
	esac
done < <(find "$package_root" -type f -printf '%P\n')

printf 'Verified %s\n' "$archive"

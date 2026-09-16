#!/usr/bin/env bash

set -euo pipefail
export LC_ALL=C

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repository=$(cd -- "$script_dir/../.." && pwd -P)

version=${VERSION:-$(git -C "$repository" describe --tags --always --dirty 2>/dev/null || printf dev)}
qt_prefix=${QT_PREFIX:-/usr}
qt_pkg_config_path=${QT_PKG_CONFIG_PATH:-$qt_prefix/lib/pkgconfig}
output_dir=${PACKAGE_OUTPUT_DIR:-$repository/tmp/dist}
fcitx5_plugin=${FCITX5_PLUGIN:-}
fcitx5_libdir=${FCITX5_LIBDIR:-}
qt_licenses_dir=${QT_LICENSES_DIR:-}
qt_attributions_file=${QT_ATTRIBUTIONS_FILE:-}
fcitx5_licenses_dir=${FCITX5_LICENSES_DIR:-}
fcitx5_copyright_file=${FCITX5_COPYRIGHT_FILE:-}
icu_license_file=${ICU_LICENSE_FILE:-}

fail() {
	printf 'package-gui-linux.sh: %s\n' "$*" >&2
	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "required command is missing: $1"
}

require_command git
require_command go
require_command pkg-config
require_command readelf
require_command tar
require_command zstd

[[ $(go env GOOS) == linux ]] || fail "GOOS must be linux, got $(go env GOOS)"
[[ $(go env GOARCH) == amd64 ]] || fail "GOARCH must be amd64, got $(go env GOARCH)"
[[ $version != */* && $version != *$'\n'* && $version != *$'\r'* ]] || fail 'VERSION contains an unsafe path character'
[[ -d $qt_prefix ]] || fail "Qt prefix does not exist: $qt_prefix"
[[ -n $qt_licenses_dir && -d $qt_licenses_dir ]] || fail 'QT_LICENSES_DIR must identify the reviewed Qt license directory'
[[ -n $qt_attributions_file && -s $qt_attributions_file ]] || fail 'QT_ATTRIBUTIONS_FILE must identify the Qt attribution notice'
[[ -n $fcitx5_licenses_dir && -d $fcitx5_licenses_dir ]] || fail 'FCITX5_LICENSES_DIR must identify the Fcitx5 Qt license directory'
[[ -n $fcitx5_copyright_file && -s $fcitx5_copyright_file ]] || fail 'FCITX5_COPYRIGHT_FILE must identify the Fcitx5 Qt copyright list'
[[ -n $icu_license_file && -f $icu_license_file ]] || fail 'ICU_LICENSE_FILE must identify the ICU license file'
for variable in QTBASE_SOURCE_REV QTWAYLAND_SOURCE_REV FCITX5_QT_SOURCE_REV ICU_SOURCE_REV ICU_VERSION; do
	[[ -n ${!variable:-} ]] || fail "$variable is required"
done

qtpaths=
qtpath_candidates=(
	"${QT_PATHS:-}"
	"$qt_prefix/bin/qtpaths6"
	"$qt_prefix/bin/qtpaths"
	"$qt_prefix/libexec/qtpaths6"
	"$qt_prefix/libexec/qtpaths"
)
if command -v qtpaths6 >/dev/null 2>&1; then
	qtpath_candidates+=("$(command -v qtpaths6)")
fi
if command -v qtpaths >/dev/null 2>&1; then
	qtpath_candidates+=("$(command -v qtpaths)")
fi
for candidate in "${qtpath_candidates[@]}"; do
	if [[ -n $candidate && -x $candidate ]]; then
		qtpaths=$candidate
		break
	fi
done

qt_query() {
	local key=$1
	if [[ -n $qtpaths ]]; then
		"$qtpaths" -query "$key" 2>/dev/null || true
	fi
}

qt_version=$(qt_query QT_VERSION)
qt_libdir=$(qt_query QT_INSTALL_LIBS)
qt_plugins=$(qt_query QT_INSTALL_PLUGINS)
qt_libdir=${qt_libdir:-$qt_prefix/lib}
qt_plugins=${qt_plugins:-$qt_prefix/plugins}

[[ -d $qt_libdir ]] || fail "Qt library directory does not exist: $qt_libdir"
[[ -d $qt_plugins ]] || fail "Qt plugin directory does not exist: $qt_plugins"

pkg_config_path=$qt_pkg_config_path
if [[ -n ${PKG_CONFIG_PATH:-} ]]; then
	pkg_config_path="$pkg_config_path:$PKG_CONFIG_PATH"
fi
export PKG_CONFIG_PATH=$pkg_config_path

if ! pkg_config_version=$(pkg-config --modversion Qt6Widgets 2>/dev/null); then
	fail "Qt6Widgets.pc was not found through PKG_CONFIG_PATH=$PKG_CONFIG_PATH"
fi
qt_version=${qt_version:-$pkg_config_version}
[[ $qt_version == "$pkg_config_version" ]] || fail "Qt version mismatch: qtpaths=$qt_version pkg-config=$pkg_config_version"

find_plugin() {
	local name=$1
	local candidate
	for candidate in "$qt_plugins/platforminputcontexts/$name" "$qt_plugins/platforms/$name"; do
		if [[ -f $candidate ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done
	return 1
}

wayland_plugins=()
shopt -s nullglob
for candidate in "$qt_plugins/platforms"/libqwayland*.so; do
	[[ -f $candidate ]] && wayland_plugins+=("$candidate")
done
shopt -u nullglob
(( ${#wayland_plugins[@]} > 0 )) || fail "Wayland QPA plugin was not found under $qt_plugins/platforms"

xcb_plugin=$(find_plugin libqxcb.so) || fail "XCB QPA plugin was not found under $qt_plugins/platforms"
compose_plugin=$(find_plugin libcomposeplatforminputcontextplugin.so) || \
	fail "Compose input context plugin was not found under $qt_plugins/platforminputcontexts"
ibus_plugin=$(find_plugin libibusplatforminputcontextplugin.so) || \
	fail "IBus input context plugin was not found under $qt_plugins/platforminputcontexts"

if [[ -z $fcitx5_plugin ]]; then
	fcitx5_plugin=$(find_plugin libfcitx5platforminputcontextplugin.so || true)
fi
[[ -n $fcitx5_plugin && -f $fcitx5_plugin ]] || fail "Fcitx5 input context plugin was not found; set FCITX5_PLUGIN to a plugin built for Qt $qt_version"

if [[ -z $fcitx5_libdir ]]; then
	fcitx5_plugin_dir=$(cd -- "$(dirname -- "$fcitx5_plugin")" && pwd -P)
	fcitx5_libdir=$(cd -- "$fcitx5_plugin_dir/../../.." && pwd -P)
fi
[[ -d $fcitx5_libdir ]] || fail "Fcitx5 library directory does not exist: $fcitx5_libdir"

mkdir -p -- "$repository/tmp"
work_dir=$(mktemp -d "$repository/tmp/package-gui-linux.XXXXXX")
cleanup() {
	rm -rf -- "$work_dir"
}
trap cleanup EXIT

root_name="ejquick-gui_${version}_linux_amd64"
package_root=$work_dir/$root_name
mkdir -p -- "$package_root/bin" "$package_root/lib" \
	"$package_root/plugins/platforms" "$package_root/plugins/platforminputcontexts" \
	"$package_root/share/applications" "$package_root/licenses"

build_version_ldflags="-s -w -X github.com/simosako/ejquick/internal/buildinfo.Version=$version"
gui_ldflags="$build_version_ldflags -extldflags=-Wl,-rpath,\$ORIGIN/../lib,--disable-new-dtags"

printf 'Building ejquick-build (version %s)\n' "$version" >&2
CGO_ENABLED=0 go build -trimpath -ldflags "$build_version_ldflags" \
	-o "$package_root/bin/ejquick-build" ./cmd/ejquick-build

printf 'Building ejquick-gui with Qt %s\n' "$qt_version" >&2
CGO_ENABLED=1 PKG_CONFIG_PATH="$PKG_CONFIG_PATH" \
	go build -trimpath -tags gui -ldflags "$gui_ldflags" \
	-o "$package_root/bin/ejquick-gui" ./cmd/ejquick-gui

cp -- "$script_dir/qt.conf" "$package_root/bin/qt.conf"
cp -- "$script_dir/share/applications/io.github.simosako.ejquick.desktop.in" \
	"$package_root/share/applications/"
cp -- "$script_dir/install-desktop.sh" "$script_dir/uninstall-desktop.sh" "$package_root/"
cp -- "$repository/README.md" "$repository/LICENSE" "$repository/THIRD_PARTY_NOTICES" "$package_root/"
chmod 0755 "$package_root/bin/ejquick-gui" "$package_root/bin/ejquick-build" \
	"$package_root/install-desktop.sh" "$package_root/uninstall-desktop.sh"

{
	printf '# MODULE\tVERSION\n'
	for binary in "$package_root/bin/ejquick-gui" "$package_root/bin/ejquick-build"; do
		go version -m "$binary" | sed -n 's/^\tdep\t\([^\t]*\)\t\([^\t]*\).*/\1\t\2/p'
	done
} | sort -u >"$package_root/GO-DEPENDENCIES.tsv"
while IFS=$'\t' read -r module module_version; do
	[[ -n $module && $module != \#* ]] || continue
	grep -Fq "  $module $module_version" "$package_root/THIRD_PARTY_NOTICES" || \
		fail "THIRD_PARTY_NOTICES is missing a linked Go module: $module $module_version"
done <"$package_root/GO-DEPENDENCIES.tsv"

copy_plugin() {
	local source=$1
	local directory=$2
	local name
	name=$(basename -- "$source")
	cp -L -- "$source" "$package_root/plugins/$directory/$name"
	chmod 0755 "$package_root/plugins/$directory/$name"
}

for plugin in "${wayland_plugins[@]}"; do
	copy_plugin "$plugin" platforms
done
copy_plugin "$xcb_plugin" platforms
copy_plugin "$compose_plugin" platforminputcontexts
copy_plugin "$ibus_plugin" platforminputcontexts
copy_plugin "$fcitx5_plugin" platforminputcontexts

find_runtime_library() {
	local name=$1
	local directory
	local candidate
	local -a directories=("$qt_libdir" "$fcitx5_libdir")
	for directory in "${directories[@]}"; do
		candidate=$directory/$name
		if [[ -e $candidate ]]; then
			printf '%s\n' "$candidate"
			return 0
		fi
	done
	return 1
}

declare -A copied_runtime_libraries=()
runtime_queue=(
	"$package_root/bin/ejquick-gui"
	"$package_root/plugins/platforms/$(basename -- "${wayland_plugins[0]}")"
	"$package_root/plugins/platforms/$(basename -- "$xcb_plugin")"
	"$package_root/plugins/platforminputcontexts/$(basename -- "$compose_plugin")"
	"$package_root/plugins/platforminputcontexts/$(basename -- "$ibus_plugin")"
	"$package_root/plugins/platforminputcontexts/$(basename -- "$fcitx5_plugin")"
)
for plugin in "${wayland_plugins[@]:1}"; do
	runtime_queue+=("$package_root/plugins/platforms/$(basename -- "$plugin")")
done

while (( ${#runtime_queue[@]} > 0 )); do
	runtime_file=${runtime_queue[0]}
	runtime_queue=("${runtime_queue[@]:1}")
	while IFS= read -r needed; do
		case $needed in
		libQt6*.so.*|libFcitx5Qt*.so.*|libicu*.so.*)
			if [[ -n ${copied_runtime_libraries[$needed]:-} ]]; then
				continue
			fi
			source=$(find_runtime_library "$needed") || \
				fail "required runtime library was not found: $needed"
			cp -L -- "$source" "$package_root/lib/$needed"
			chmod 0755 "$package_root/lib/$needed"
			copied_runtime_libraries[$needed]=1
			runtime_queue+=("$package_root/lib/$needed")
			;;
		esac
	done < <(readelf -d "$runtime_file" | sed -n 's/.*Shared library: \[\([^]]*\)\].*/\1/p')
done

mkdir -p -- "$package_root/licenses/Qt" "$package_root/licenses/Fcitx5-Qt" "$package_root/licenses/ICU"
cp -a -- "$qt_licenses_dir/." "$package_root/licenses/Qt/"
cp -- "$qt_attributions_file" "$package_root/licenses/Qt/THIRD-PARTY-NOTICES.txt"
cp -a -- "$fcitx5_licenses_dir/." "$package_root/licenses/Fcitx5-Qt/"
cp -- "$fcitx5_copyright_file" "$package_root/licenses/Fcitx5-Qt/COPYRIGHTS.txt"
cp -- "$icu_license_file" "$package_root/licenses/ICU/LICENSE"
[[ -f $package_root/licenses/Qt/qtbase/LGPL-3.0-only.txt ]] || fail 'Qt LGPL-3.0 license text is missing'
[[ -f $package_root/licenses/Qt/qtbase/GPL-3.0-only.txt ]] || fail 'Qt GPL-3.0 license text is missing'
[[ -f $package_root/licenses/Fcitx5-Qt/LGPL-2.1-or-later.txt ]] || fail 'Fcitx5 Qt LGPL-2.1 license text is missing'
[[ -s $package_root/licenses/Qt/THIRD-PARTY-NOTICES.txt ]] || fail 'Qt third-party attribution notice is empty'
[[ -s $package_root/licenses/Fcitx5-Qt/COPYRIGHTS.txt ]] || fail 'Fcitx5 Qt copyright list is empty'
[[ -s $package_root/licenses/ICU/LICENSE ]] || fail 'ICU license text is empty'

cat >"$package_root/licenses/GUI-RUNTIME-NOTICES.txt" <<EOF
EJQuick Linux GUI runtime notice
================================

EJQuick application code is distributed under the MIT License in ../LICENSE.
Go dependency notices are in ../THIRD_PARTY_NOTICES.

This package bundles Qt ${qt_version} shared libraries and Qt plugins. Qt is
used under the GNU Lesser General Public License version 3. Qt license texts
are provided in licenses/Qt. The libraries are dynamically linked and may be
replaced with ABI-compatible modified builds. Corresponding source revisions
are recorded in SOURCE-COMPONENTS.txt.

The Fcitx5 Qt input context plugin is bundled as well. It is provided by
fcitx5-qt and is licensed under the GNU Lesser General Public License 2.1 or
later. Its license texts are in licenses/Fcitx5-Qt and its source revision is
recorded in SOURCE-COMPONENTS.txt.

This package also bundles ICU ${ICU_VERSION} shared libraries distributed with
the Qt SDK. The ICU license is in licenses/ICU/LICENSE and its source revision
is recorded in SOURCE-COMPONENTS.txt.

This archive does not bundle glibc, the ELF loader, Wayland/X11 libraries,
DBus, XKB, font, OpenGL/EGL, graphics-driver, or input-method daemon files.
Those components remain the responsibility of the host system.
EOF

cat >"$package_root/SOURCE-COMPONENTS.txt" <<EOF
Bundled runtime source references
=================================

Qt Base ${qt_version}
Revision: ${QTBASE_SOURCE_REV}
Source: https://github.com/qt/qtbase/tree/${QTBASE_SOURCE_REV}

Qt Wayland ${qt_version}
Revision: ${QTWAYLAND_SOURCE_REV}
Source: https://github.com/qt/qtwayland/tree/${QTWAYLAND_SOURCE_REV}

Fcitx5 Qt ${FCITX5_QT_VERSION:-unknown}
Revision: ${FCITX5_QT_SOURCE_REV}
Source: https://github.com/fcitx/fcitx5-qt/tree/${FCITX5_QT_SOURCE_REV}

ICU ${ICU_VERSION}
Revision: ${ICU_SOURCE_REV}
Source: https://github.com/unicode-org/icu/tree/${ICU_SOURCE_REV}
EOF

validate_needed_libraries() {
	local file=$1
	local needed
	while IFS= read -r needed; do
		case $needed in
		libQt6*.so.*|libFcitx5Qt*.so.*|libicu*.so.*)
			[[ -f $package_root/lib/$needed ]] || fail "staged file has an unstaged dependency: $file -> $needed"
			;;
		esac
	done < <(readelf -d "$file" | sed -n 's/.*Shared library: \[\([^]]*\)\].*/\1/p')
}

validate_needed_libraries "$package_root/bin/ejquick-gui"
for file in "$package_root/lib"/*.so.* "$package_root/plugins/platforms"/*.so \
	"$package_root/plugins/platforminputcontexts"/*.so; do
	[[ -f $file ]] || continue
	validate_needed_libraries "$file"
done

"$script_dir/generate-runtime-dependencies.sh" "$package_root" "$package_root/DEPENDENCIES.tsv"

if ! readelf -d "$package_root/bin/ejquick-gui" | grep -Eq 'RUNPATH.*\$ORIGIN/../lib|RPATH.*\$ORIGIN/../lib'; then
	fail 'ejquick-gui does not contain the expected relative library path'
fi
grep -Fq 'Plugins=../plugins' "$package_root/bin/qt.conf" || fail 'qt.conf does not point to the package plugins directory'

while IFS= read -r file; do
	case $file in
	*.TXT|*.ZIP|*.sqlite3|*.sqlite3-*|*/source/*) fail "forbidden dictionary artifact in package: $file" ;;
	esac
done < <(find "$package_root" -type f -printf '%P\n')

mkdir -p -- "$output_dir"
archive=$output_dir/$root_name.tar.zst
rm -f -- "$archive"
tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
	-I 'zstd -19 -T0' -cf "$archive" -C "$work_dir" "$root_name"
tar -tf "$archive" >/dev/null

printf 'Created %s\n' "$archive"

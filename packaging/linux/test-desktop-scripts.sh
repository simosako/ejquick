#!/usr/bin/env bash

set -euo pipefail

source_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repository=$(cd -- "$source_dir/../.." && pwd -P)
test_root=$(mktemp -d "$repository/tmp/desktop-script-test.XXXXXX")
cleanup() {
	rm -rf -- "$test_root"
}
trap cleanup EXIT

package_root=$test_root/'EJ Quick "日本" \ $ `'
data_home=$test_root/'data home'
mkdir -p -- "$package_root/bin" "$package_root/share/applications" "$data_home"
cp -- "$source_dir/install-desktop.sh" "$source_dir/uninstall-desktop.sh" "$package_root/"
cp -- "$source_dir/share/applications/io.github.simosako.ejquick.desktop.in" "$package_root/share/applications/"
cat >"$package_root/bin/ejquick-gui" <<'EOF'
#!/usr/bin/env bash
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
printf launched >"$script_dir/../launched"
EOF
chmod +x "$package_root/install-desktop.sh" "$package_root/uninstall-desktop.sh" "$package_root/bin/ejquick-gui"

XDG_DATA_HOME=$data_home "$package_root/install-desktop.sh" >/dev/null
desktop_file=$data_home/applications/io.github.simosako.ejquick.desktop
[[ -f $desktop_file ]]
if command -v desktop-file-validate >/dev/null 2>&1; then
	desktop-file-validate "$desktop_file"
fi
! grep -q '@EXEC@\|@TRYEXEC@' "$desktop_file"
grep -q '^TryExec=/' "$desktop_file"
grep -Fq '\\\\' "$desktop_file"
grep -Fq '\\"' "$desktop_file"
grep -Fq '\\$' "$desktop_file"
grep -Fq '\\`' "$desktop_file"

if command -v cc >/dev/null 2>&1 && command -v pkg-config >/dev/null 2>&1 && pkg-config --exists gio-unix-2.0; then
	cat >"$test_root/launch-desktop.c" <<'EOF'
#include <gio/gdesktopappinfo.h>
#include <stdio.h>
int main(int argc, char **argv) {
    GDesktopAppInfo *info = g_desktop_app_info_new_from_filename(argv[1]);
    GError *error = NULL;
    if (info == NULL) return 1;
    if (!g_app_info_launch(G_APP_INFO(info), NULL, NULL, &error)) {
        fprintf(stderr, "%s\n", error->message);
        return 2;
    }
    g_object_unref(info);
    return 0;
}
EOF
	cc $(pkg-config --cflags gio-unix-2.0) "$test_root/launch-desktop.c" \
		$(pkg-config --libs gio-unix-2.0) -o "$test_root/launch-desktop"
	"$test_root/launch-desktop" "$desktop_file"
	for _ in {1..100}; do
		[[ -f $package_root/launched ]] && break
		sleep 0.02
	done
	[[ $(cat "$package_root/launched") == launched ]]
fi

first_copy=$(cat "$desktop_file")
XDG_DATA_HOME=$data_home "$package_root/install-desktop.sh" >/dev/null
[[ $(cat "$desktop_file") == "$first_copy" ]]
XDG_DATA_HOME=$data_home "$package_root/uninstall-desktop.sh" >/dev/null
[[ ! -e $desktop_file ]]

equals_root=$test_root/'equals=unsupported'
mkdir -p -- "$equals_root/bin" "$equals_root/share/applications"
cp -- "$source_dir/install-desktop.sh" "$equals_root/"
cp -- "$source_dir/share/applications/io.github.simosako.ejquick.desktop.in" "$equals_root/share/applications/"
cp -- "$package_root/bin/ejquick-gui" "$equals_root/bin/"
chmod +x "$equals_root/install-desktop.sh" "$equals_root/bin/ejquick-gui"
if XDG_DATA_HOME=$data_home "$equals_root/install-desktop.sh" >/dev/null 2>&1; then
	printf 'installer accepted an equals sign in the executable path\n' >&2
	exit 1
fi

percent_root=$test_root/'percent%unsupported'
mkdir -p -- "$percent_root/bin" "$percent_root/share/applications"
cp -- "$source_dir/install-desktop.sh" "$percent_root/"
cp -- "$source_dir/share/applications/io.github.simosako.ejquick.desktop.in" "$percent_root/share/applications/"
cp -- "$package_root/bin/ejquick-gui" "$percent_root/bin/"
chmod +x "$percent_root/install-desktop.sh" "$percent_root/bin/ejquick-gui"
if XDG_DATA_HOME=$data_home "$percent_root/install-desktop.sh" >/dev/null 2>&1; then
	printf 'installer accepted a percent sign in the executable path\n' >&2
	exit 1
fi

newline_root=$test_root/$'newline\nunsupported'
mkdir -p -- "$newline_root/bin" "$newline_root/share/applications"
cp -- "$source_dir/install-desktop.sh" "$newline_root/"
cp -- "$source_dir/share/applications/io.github.simosako.ejquick.desktop.in" "$newline_root/share/applications/"
cp -- "$package_root/bin/ejquick-gui" "$newline_root/bin/"
chmod +x "$newline_root/install-desktop.sh" "$newline_root/bin/ejquick-gui"
if XDG_DATA_HOME=$data_home "$newline_root/install-desktop.sh" >/dev/null 2>&1; then
	printf 'installer accepted a control character in the executable path\n' >&2
	exit 1
fi

printf 'desktop registration scripts: ok\n'

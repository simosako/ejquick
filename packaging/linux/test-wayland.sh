#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 3 ]]; then
	printf 'usage: %s <gui-binary> <config> <test-root>\n' "$0" >&2
	exit 2
fi

gui_binary=$1
config_path=$2
test_root=$3
runtime_dir=$test_root/runtime

mkdir -p -- "$runtime_dir" "$test_root/state"
chmod 700 "$runtime_dir"

export XDG_RUNTIME_DIR=$runtime_dir
export XDG_STATE_HOME=$test_root/state
export WAYLAND_DISPLAY=${WAYLAND_DISPLAY:-ejquick-wayland-0}
export QT_QPA_PLATFORM=wayland
export QT_IM_MODULE=${QT_IM_MODULE:-compose}
export QT_DEBUG_PLUGINS=1
export EJQUICK_GUI_SMOKE_TEST=1
unset DISPLAY
rm -f -- "$runtime_dir/$WAYLAND_DISPLAY" "$test_root/weston.log" \
	"$test_root/gui.stdout.log" "$test_root/gui.stderr.log" \
	"$test_root/state/ejquick/ejquick.log"

weston --backend=headless-backend.so --renderer=pixman \
	--socket="$WAYLAND_DISPLAY" --idle-time=0 \
	>"$test_root/weston.log" 2>&1 &
weston_pid=$!

print_logs() {
	for log in "$test_root/weston.log" "$test_root/gui.stdout.log" \
		"$test_root/gui.stderr.log" "$test_root/state/ejquick/ejquick.log"; do
		if [[ -f $log ]]; then
			printf '\n===== %s =====\n' "$log" >&2
			cat -- "$log" >&2
		fi
	done
}

cleanup() {
	if kill -0 "$weston_pid" 2>/dev/null; then
		kill "$weston_pid" 2>/dev/null || true
	fi
	wait "$weston_pid" 2>/dev/null || true
}
trap cleanup EXIT

for _ in {1..100}; do
	if [[ -S $runtime_dir/$WAYLAND_DISPLAY ]]; then
		break
	fi
	if ! kill -0 "$weston_pid" 2>/dev/null; then
		print_logs
		exit 1
	fi
	sleep 0.1
done
if [[ ! -S $runtime_dir/$WAYLAND_DISPLAY ]]; then
	print_logs
	exit 1
fi

if ! "$gui_binary" -c "$config_path" --debug \
	>"$test_root/gui.stdout.log" 2>"$test_root/gui.stderr.log"; then
	print_logs
	exit 1
fi

if ! grep -Eiq 'libqwayland\.so|qwayland' "$test_root/gui.stderr.log"; then
	printf 'Wayland QPA plugin was not reported as loaded\n' >&2
	print_logs
	exit 1
fi

app_log="$test_root/state/ejquick/ejquick.log"
if ! grep -Fq 'platform="wayland"' "$app_log" ||
	! grep -Fq 'gui smoke: passed:' "$app_log"; then
	print_logs
	exit 1
fi
printf 'Wayland GUI smoke test: ok\n'

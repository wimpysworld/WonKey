#!/bin/sh
set -eu

# Fixed first-test target. Reinspect before changing either value.
path=1-1.2
node=/dev/hidraw4
root=${WONKEY_CAPTURE_ROOT:-${XFKEY_CAPTURE_ROOT:-"$HOME/.local/state/xfkey-captures"}}
case ${1:-} in
readback | apply)
  command=$1
  shift
  ;;
*)
  echo 'Usage: ./capture-settings.sh readback | apply --write --expect-identifier HEX --expect-version HEX [--lighting NAME] [--colour RRGGBB] [settings]' >&2
  exit 2
  ;;
esac
test "$(id -u)" -ne 0 || {
  echo 'Run this script as the invoking user, not root.' >&2
  exit 2
}
if test "$command" = apply; then
  allowed=no
  for arg; do test "$arg" != --write || allowed=yes; done
  test "$allowed" = yes || {
    echo 'Apply requires explicit --write.' >&2
    exit 2
  }
fi
case $root in /*) ;; *)
  echo 'WONKEY_CAPTURE_ROOT (or fallback XFKEY_CAPTURE_ROOT) must be absolute.' >&2
  exit 2
  ;;
esac
build=$(mktemp -d "${TMPDIR:-/tmp}/wonkey-build-XXXXXXXX")
trap 'rm -rf -- "$build"' EXIT
trap 'exit 130' HUP INT TERM
source=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
(cd "$source" && go build -buildvcs=false -o "$build/wonkey" ./cmd/wonkey)
uid=$(id -u)
# The privileged shell changes only this node's ACL. The tool always runs as the caller.
sudo sh -eu -s -- "$uid" "$build/wonkey" "$path" "$node" "$root" "$command" "$@" <<'SH'
uid=$1; bin=$2; path=$3; node=$4; root=$5; command=$6
shift 6
test "$uid" -ne 0
test ! -L "$node" && test -c "$node"
stamp=$(stat -c '%d:%i:%t:%T' "$node")
inspection=$(sudo -n -u "#$uid" -- "$bin" inspect --path "$path")
printf '%s\n' "$inspection" | python3 -c 'import json,sys; d=json.load(sys.stdin); path,node=sys.argv[1:]; c=[c for c in d["candidates"] if c["physical_path"]==path]; assert d["selected_physical_path"]==path and len(c)==1 and c[0]["descriptor_match_not_model_confirmation"] and c[0]["vendor_interface_3_node"]["path"]==node' "$path" "$node"
check_node() {
  test ! -L "$node" && test -c "$node" && test "$(stat -c '%d:%i:%t:%T' "$node")" = "$stamp"
}
check_node
d=$(mktemp -d /tmp/wonkey-acl-XXXXXXXX)
getfacl -P -p "$node" > "$d/before.acl"
check_node
cleanup() {
  check_node || {
    echo "RESTORATION BLOCKED: node changed. Saved ACL: $d/before.acl" >&2
    return 1
  }
  setfacl -P --restore="$d/before.acl" || return 1
  getfacl -P -p "$node" > "$d/after.acl" || return 1
  cmp "$d/before.acl" "$d/after.acl" || return 1
  echo "ACL restored. Record: $d" >&2
}
finish() {
  status=$?
  trap - EXIT
  if ! cleanup; then
    echo "ACL RESTORATION FAILED. Record: $d" >&2
    exit 1
  fi
  exit "$status"
}
trap finish EXIT
trap 'exit 130' HUP INT TERM
check_node
setfacl -P -m "u:$uid:rw" "$node"
check_node
sudo -n -u "#$uid" -- "$bin" "$command" "$@" --path "$path" --capture-root "$root"
SH

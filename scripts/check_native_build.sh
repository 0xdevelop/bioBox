#!/bin/bash
# Packaging preflight only: no install, network, tag or release side effects.
set -euo pipefail
[[ $# == 1 ]] || { echo 'Usage: check_native_build.sh amd64|arm64' >&2; exit 2; }
arch="$1"
[[ "$(uname -s)" == Linux ]] || { echo 'native Linux host required' >&2; exit 1; }
case "$arch:$(uname -m)" in
  amd64:x86_64|amd64:amd64|arm64:aarch64|arm64:arm64) ;;
  *) echo 'target does not match native runner architecture' >&2; exit 1 ;;
esac
for tool in go gcc zip unzip bun; do
  command -v "$tool" >/dev/null || { echo "missing native packaging tool: $tool" >&2; exit 1; }
done
[[ "$(go env GOHOSTOS)" == linux && "$(go env GOOS)" == linux &&
   "$(go env GOHOSTARCH)" == "$arch" && "$(go env GOARCH)" == "$arch" ]] || {
  echo 'Go host/target mismatch; GOARCH is not a native runtime build' >&2; exit 1;
}
[[ "$(go env CGO_ENABLED)" == 1 ]] || { echo 'SQLite requires CGO_ENABLED=1' >&2; exit 1; }
case "$arch:$(gcc -dumpmachine)" in
  amd64:x86_64-linux-gnu*|arm64:aarch64-linux-gnu*) ;;
  *) echo 'gcc target does not match native runner' >&2; exit 1 ;;
esac
printf '[native] linux/%s, native gcc, CGo enabled; packaging only, no GPU/dependency install\n' "$arch"

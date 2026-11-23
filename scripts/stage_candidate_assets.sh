#!/usr/bin/env bash
# Existing Build-stage candidate path: configured version stays unchanged.
set -euo pipefail
commit="$1"
version=$(go run gen_release_assets.go version)
shopt -s nullglob dotglob
inputs=(tmp/collected/*)
[[ ${#inputs[@]} == 2 ]] || { echo 'candidate requires exactly two native service bundles' >&2; exit 1; }
# mkdir, not mkdir -p: never merge stale files into the public candidate stage.
mkdir tmp/release-assets
for arch in amd64 arm64; do
    name="protenix_service_linux_${arch}_release_${version}.zip"
    cp -- "tmp/collected/${name}" "tmp/release-assets/${name}"
done
(cd tmp/release-assets && sha256sum protenix_service_*.zip > SHA256SUMS)
go run gen_release_assets.go verify --commit="$commit" --dir=tmp/release-assets

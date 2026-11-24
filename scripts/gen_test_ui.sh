#!/bin/bash
set -e

project_path=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
web_path="${project_path}/test_ui/web"
bun_tmp_path="${project_path}/tmp/test-ui-bun-tmp"
bun_cache_path="${project_path}/tmp/test-ui-bun-cache"

if ! command -v bun >/dev/null 2>&1; then
    echo "bun is required to build test_ui/web"
    exit 1
fi

mkdir -p "${bun_tmp_path}" "${bun_cache_path}"

echo "build test_ui/web"
(
    cd "${web_path}"
    export TMPDIR="${bun_tmp_path}"
    export BUN_INSTALL_CACHE_DIR="${bun_cache_path}"
    export XDG_CACHE_HOME="${bun_cache_path}"
    bun install --frozen-lockfile --ignore-scripts
    bun run build
)

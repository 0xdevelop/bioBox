#!/bin/bash
set -e

project_path=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "${project_path}"

if ! command -v go >/dev/null 2>&1 && [[ -x "${HOME}/.local/go/bin/go" ]]; then
  export PATH="${HOME}/.local/go/bin:${PATH}"
fi

go run scripts/gen_api_docs.go "${1:-}"

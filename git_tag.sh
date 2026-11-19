#!/bin/bash
# Release entry only. build.sh builds; Actions selects native runners and publishes.
# No implicit version bump, commit, latest-tag rewrite, force push, or release deletion.
set -euo pipefail
cd "$(dirname "$0")"
usage() {
  echo 'Usage: ./git_tag.sh --check [vX.Y.Z] | --publish vX.Y.Z'
  echo '--check is local/read-only. --publish creates and pushes one annotated tag; requires explicit release approval.'
}
case "${1:---check}" in
  --help|-h) usage; exit 0 ;;
  --check|--publish) mode="${1:---check}" ;;
  *) usage >&2; exit 2 ;;
esac
[[ $# -le 2 ]] || { usage >&2; exit 2; }
if [[ "$mode" == --publish && -z "${2:-}" ]]; then usage >&2; exit 2; fi
args=()
[[ -z "${2:-}" ]] || args+=("--tag=$2")
version="$(go run gen_release_assets.go version "${args[@]}")"
[[ -z "$(git status --porcelain)" ]] || { echo '[git_tag] commit/check changes first; no tag created' >&2; exit 1; }
[[ "$(git branch --show-current)" == main ]] || { echo '[git_tag] release from main only' >&2; exit 1; }
head="$(git rev-parse HEAD)"
if [[ "$mode" == --check ]]; then
  printf '[git_tag] local preflight OK: %s @ %s; no tag/push/release performed\n' "$version" "$head"
  exit 0
fi
# Network/deployment side effects exist ONLY below the explicit --publish boundary.
if git show-ref --verify --quiet "refs/tags/$version"; then
  echo '[git_tag] local tag exists; inspect it manually, never overwrite' >&2; exit 1
fi
remote_main="$(git ls-remote --exit-code origin refs/heads/main)"
[[ "${remote_main%%[[:space:]]*}" == "$head" ]] || {
  echo '[git_tag] origin/main must already equal HEAD; push approved commits separately' >&2; exit 1;
}
remote_tags="$(git ls-remote origin "refs/tags/$version" "refs/tags/$version^{}")"
[[ -z "$remote_tags" ]] || { echo '[git_tag] remote tag exists; never overwrite' >&2; exit 1; }
git tag -a "$version" -m "Release $version" "$head"
git push origin "refs/tags/$version:refs/tags/$version"
echo '[git_tag] tag pushed; Actions must finish both native builds and complete-asset gate before publishing'

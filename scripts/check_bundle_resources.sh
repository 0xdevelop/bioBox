#!/usr/bin/env bash
# Native, rootless validation of the real service release zip. No upstream Python,
# environment installation, weight download, systemd or GPU.
set -euo pipefail
archive="$1"
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "${repo_root}"
mkdir -p tmp
stage=$(mktemp -d tmp/bundle-resource-smoke.XXXXXX)
trap 'rm -rf "$stage"' EXIT
unzip -q "$archive" -d "$stage"
root="$(cd "$stage/bioBox" && pwd)"
chmod u+x "$root/bioBox"
python3 - "$root" <<'PYCODE'
from pathlib import Path
import sys
root=Path(sys.argv[1]);cfg=root/'conf/config.yaml'
text=cfg.read_text()
assert text.count('download_enabled: false')==2
cfg.write_text(text.replace('/bioBox/runtime/',str(root/'runtime')+'/'))
for kind in ['protenix','pxdesign']:
 p=root/'runtime'/kind/'conf'/(kind+'.env')
 p.write_text(p.read_text().replace('@PREFIX@',str(root/'runtime'/kind)))
PYCODE
for set in protenix.v1 pxdesign.preview; do
    "$root/bioBox" resource status "$set" > "$stage/status.json" </dev/null
    python3 - "$stage/status.json" <<'PYCODE'
import json,sys
assert json.load(open(sys.argv[1]))['state']=='download_disabled'
PYCODE
    if "$root/bioBox" resource prepare "$set" > "$stage/prepare.out" 2> "$stage/prepare.err" </dev/null; then
        echo 'missing offline weights unexpectedly prepared' >&2; exit 1
    fi
    if "$root/bioBox" resource verify "$set" > "$stage/verify.out" 2> "$stage/verify.err" </dev/null; then
        echo 'uninstalled runtime unexpectedly verified' >&2; exit 1
    fi
    python3 - "$stage/verify.err" <<'PYCODE'
import sys
assert 'Python was not started' in open(sys.argv[1]).read()
PYCODE
    echo "[bundle-resource-smoke] $set status=download_disabled; missing resources/uninstalled runtime refused before Python"
done

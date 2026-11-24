#!/bin/bash
# Isolated fixtures for integrated update restore. No host systemd, no /bb_service install.
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
cd "$repo"
out="${TEST_OUT:-${repo}/tmp/install-update-restore-fixture}"
rm -rf "$out"
mkdir -p "$out"

write_runtime_stub() {
  local dest=$1
  cat > "$dest" <<'STUB'
#!/bin/bash
set -euo pipefail
me=$(basename "$0")
app=${me#install_}
app=${app%.sh}
prefix_var=$(echo "$app" | tr 'a-z' 'A-Z')_PREFIX
prefix=${!prefix_var}
cmd=${1:-}
current_of() { if [[ -L "$prefix/current" ]]; then basename "$(readlink "$prefix/current")"; fi; }
case "$cmd" in
  update)
    if [[ -f "$prefix/fail-update" ]]; then
      echo "fixture $app update fail" >&2
      exit 42
    fi
    cur=$(current_of)
    mkdir -p "$prefix/releases/new"
    if [[ -n "$cur" && "$cur" != "new" ]]; then
      ln -sfn "releases/${cur}" "$prefix/previous"
    fi
    ln -sfn releases/new "$prefix/current"
    echo "fixture $app updated" >> "${APP_DIR:-/bb_service}/events.log"
    ;;
  rollback)
    if [[ -f "$prefix/fail-rollback" ]]; then
      echo "fixture $app rollback fail" >&2
      exit 1
    fi
    [[ -L "$prefix/previous" ]] || exit 1
    prev=$(basename "$(readlink "$prefix/previous")")
    cur=$(current_of)
    ln -sfn "releases/${prev}" "$prefix/current"
    if [[ -n "$cur" ]]; then
      ln -sfn "releases/${cur}" "$prefix/previous"
    fi
    echo "fixture $app rolled back" >> "${APP_DIR:-/bb_service}/events.log"
    ;;
  uninstall)
    rm -f "$prefix/current" "$prefix/previous"
    rm -rf "$prefix/releases" "$prefix/bin"
    echo "fixture $app uninstalled" >> "${APP_DIR:-/bb_service}/events.log"
    ;;
  status)
    echo "$app current=$(current_of)"
    ;;
  *)
    echo "unsupported $cmd" >&2
    exit 2
    ;;
esac
STUB
  chmod a+x "$dest"
}

setup_world() {
  local world=$1
  local root="$out/$world"
  rm -rf "$root"
  local app_root="$root/root/bb_service"
  local systemd_dir="$root/root/etc/systemd/system"
  local pkg="$root/pkg/bb_service"
  local bindir="$root/bin"
  mkdir -p "$app_root" "$systemd_dir" "$pkg/conf" "$pkg/runtime/protenix" "$pkg/runtime/pxdesign" "$bindir" "$root/root/tmp"
  printf 'old-binary\n' > "$app_root/bb_service"
  chmod a+x "$app_root/bb_service"
  printf 'old-unit\n' > "$systemd_dir/bb_service.service"
  printf 'new-binary\n' > "$pkg/bb_service"
  chmod a+x "$pkg/bb_service"
  printf 'new-unit\n' > "$pkg/bb_service.service"
  printf 'example: true\n' > "$pkg/conf/config.yaml"
  cp -f example_files/install_bb_service.sh "$pkg/install_bb_service.sh"
  chmod a+x "$pkg/install_bb_service.sh"
  write_runtime_stub "$pkg/runtime/protenix/install_protenix.sh"
  write_runtime_stub "$pkg/runtime/pxdesign/install_pxdesign.sh"
  for app in protenix pxdesign; do
    mkdir -p "$app_root/runtime/$app/releases/old" "$app_root/runtime/$app/conf"
    echo "$app" > "$app_root/runtime/$app/.installed_by"
    ln -sfn releases/old "$app_root/runtime/$app/current"
  done
  cat > "$bindir/systemctl" <<'CTL'
#!/bin/bash
set -euo pipefail
log=/bb_service/systemctl-fixture.log
printf '%s\n' "$*" >> "$log"
if [[ "${1:-}" == restart && -f /bb_service/fail-restart ]]; then
  exit 1
fi
if [[ "${1:-}" == is-active ]]; then
  exit 0
fi
exit 0
CTL
  chmod a+x "$bindir/systemctl"
  printf '#!/bin/bash\nexit 0\n' > "$bindir/uv"
  chmod a+x "$bindir/uv"
  mkdir -p "$root/binds"
  echo "$app_root" > "$root/binds/app_root"
  echo "$systemd_dir" > "$root/binds/systemd_dir"
  echo "$pkg" > "$root/binds/pkg"
  echo "$bindir" > "$root/binds/bindir"
}

run_update() {
  local world=$1
  local app_root systemd_dir pkg bindir
  app_root=$(cat "$out/$world/binds/app_root")
  systemd_dir=$(cat "$out/$world/binds/systemd_dir")
  pkg=$(cat "$out/$world/binds/pkg")
  bindir=$(cat "$out/$world/binds/bindir")
  local binds=(
    --unshare-net --tmpfs /
    --dev /dev --proc /proc
    --tmpfs /tmp
    --ro-bind /usr /usr
    --ro-bind /bin /bin
    --ro-bind /lib /lib
    --bind "$app_root" /bb_service
    --bind "$systemd_dir" /etc/systemd/system
    --bind "$pkg" /opt/pkg
    --bind "$bindir" /fixture-bin
    --chdir /opt/pkg
    --setenv PATH /fixture-bin:/usr/bin:/bin
    --setenv HOME /tmp
    --setenv UV /fixture-bin/uv
    --setenv APP_DIR /bb_service
  )
  if [[ -d /lib64 ]]; then binds+=(--ro-bind /lib64 /lib64); fi
  if [[ -d /sbin ]]; then binds+=(--ro-bind /sbin /sbin); fi
  set +e
  bwrap "${binds[@]}" bash -c 'printf "2\nyes\n" | bash ./install_bb_service.sh' \
    >"$out/$world/installer.log" 2>&1
  local rc=$?
  set -e
  echo "$rc" > "$out/$world/installer.rc"
}

current_of() { basename "$(readlink "$1")"; }

fail() { echo "instance update restore fixture FAILED: $*" >&2; exit 1; }

# Case 1: first runtime updates, second fails; both currents must go back, old service starts.
setup_world complete_restore
touch "$(cat "$out/complete_restore/binds/app_root")/runtime/pxdesign/fail-update"
run_update complete_restore
app=$(cat "$out/complete_restore/binds/app_root")
systemd=$(cat "$out/complete_restore/binds/systemd_dir")
rc=$(cat "$out/complete_restore/installer.rc")
[[ "$rc" -eq 1 ]] || fail "complete_restore exit $rc"
[[ "$(current_of "$app/runtime/protenix/current")" == old ]] || fail "protenix not restored"
[[ "$(current_of "$app/runtime/pxdesign/current")" == old ]] || fail "pxdesign not restored"
grep -qx old-binary "$app/bb_service" || fail "binary changed"
grep -qx old-unit "$systemd/bb_service.service" || fail "unit changed"
grep -qx 'start bb_service.service' "$app/systemctl-fixture.log" || fail "old service not started"
grep -q 'runtimes restored' "$out/complete_restore/installer.log" || fail "missing restore message"
if grep -q 'restarting old service' "$out/complete_restore/installer.log" && ! grep -q 'runtimes restored' "$out/complete_restore/installer.log"; then
  fail "old wording treated mixed state as a successful restart"
fi

# Case 2: first runtime updates, second fails, rollback of the first also fails; do not start mixed.
setup_world mixed_no_start
touch "$(cat "$out/mixed_no_start/binds/app_root")/runtime/pxdesign/fail-update"
touch "$(cat "$out/mixed_no_start/binds/app_root")/runtime/protenix/fail-rollback"
run_update mixed_no_start
app=$(cat "$out/mixed_no_start/binds/app_root")
rc=$(cat "$out/mixed_no_start/installer.rc")
[[ "$rc" -eq 1 ]] || fail "mixed_no_start exit $rc"
[[ "$(current_of "$app/runtime/protenix/current")" == new ]] || fail "expected leftover protenix=new"
[[ "$(current_of "$app/runtime/pxdesign/current")" == old ]] || fail "expected pxdesign=old"
grep -qx old-binary "$app/bb_service" || fail "binary changed in mixed case"
if grep -qx 'start bb_service.service' "$app/systemctl-fixture.log"; then
  fail "mixed state started the service"
fi
grep -q 'not starting bb_service with a mixed runtime or binary' "$out/mixed_no_start/installer.log" || fail "missing mixed-state message"

{
  echo "fixture_only=true"
  echo "host_root_install=false"
  echo "complete_restore_exit=$(cat "$out/complete_restore/installer.rc")"
  echo "complete_restore_protenix=$(current_of "$(cat "$out/complete_restore/binds/app_root")/runtime/protenix/current")"
  echo "complete_restore_pxdesign=$(current_of "$(cat "$out/complete_restore/binds/app_root")/runtime/pxdesign/current")"
  echo "complete_restore_start=1"
  echo "mixed_no_start_exit=$(cat "$out/mixed_no_start/installer.rc")"
  echo "mixed_no_start_protenix=$(current_of "$(cat "$out/mixed_no_start/binds/app_root")/runtime/protenix/current")"
  echo "mixed_no_start_pxdesign=$(current_of "$(cat "$out/mixed_no_start/binds/app_root")/runtime/pxdesign/current")"
  echo "mixed_no_start_service_started=false"
} > "$out/result.txt"

echo "instance update restore fixtures PASS"
cat "$out/result.txt"

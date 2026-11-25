#!/bin/bash
# Isolated bwrap fixtures for install/update legacy preflight and integrated update restore.
# No host systemd, no /bioBox install, no /bb_service modification.
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
  install)
    mkdir -p "$prefix/releases/v1" "$prefix/conf" "$prefix/bin"
    echo "$app" > "$prefix/.installed_by"
    ln -sfn releases/v1 "$prefix/current"
    echo "fixture $app installed" >> "${APP_DIR:-/bioBox}/events.log"
    ;;
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
    echo "fixture $app updated" >> "${APP_DIR:-/bioBox}/events.log"
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
    echo "fixture $app rolled back" >> "${APP_DIR:-/bioBox}/events.log"
    ;;
  uninstall)
    rm -f "$prefix/current" "$prefix/previous"
    rm -rf "$prefix/releases" "$prefix/bin"
    echo "fixture $app uninstalled" >> "${APP_DIR:-/bioBox}/events.log"
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

write_systemctl_stub() {
  local dest=$1
  cat > "$dest" <<'CTL'
#!/bin/bash
set -euo pipefail
printf '%s\n' "$*" >> /fixture-state/systemctl.log
if [[ -d /bioBox ]]; then
  printf '%s\n' "$*" >> /bioBox/systemctl-fixture.log
fi

if [[ "${1:-}" == "show" ]]; then
  unit="${@: -1}"
  if [[ "$unit" == "bb_service.service" ]]; then
    if [[ -f /fixture-state/bb_service.show.rc ]]; then
      rc=$(cat /fixture-state/bb_service.show.rc)
      if [[ -f /fixture-state/bb_service.show.out ]]; then
        cat /fixture-state/bb_service.show.out
      fi
      exit "$rc"
    fi
    if [[ -f /fixture-state/bb_service.show.out ]]; then
      cat /fixture-state/bb_service.show.out
      exit 0
    fi
    printf 'LoadState=not-found\nActiveState=inactive\nUnitFileState=\n'
    exit 0
  fi
  printf 'LoadState=loaded\nActiveState=active\nUnitFileState=enabled\n'
  exit 0
fi

if [[ "${1:-}" == "restart" && -f /bioBox/fail-restart ]]; then
  exit 1
fi
if [[ "${1:-}" == "is-active" ]]; then
  unit="${@: -1}"
  if [[ "$unit" == "bb_service.service" ]]; then
    if [[ -f /fixture-state/bb_service.active ]]; then
      exit 0
    fi
    exit 4
  fi
  exit 0
fi
exit 0
CTL
  chmod a+x "$dest"
}

setup_world() {
  local world=$1
  local mode=${2:-existing} # existing | empty
  local root="$out/$world"
  rm -rf "$root"
  local app_root="$root/root/bioBox"
  local systemd_dir="$root/root/etc/systemd/system"
  local pkg="$root/pkg/bioBox"
  local bindir="$root/bin"
  local statedir="$root/state"
  mkdir -p "$systemd_dir" "$pkg/conf" "$pkg/runtime/protenix" "$pkg/runtime/pxdesign" "$bindir" "$statedir" "$root/root/tmp"
  if [[ "$mode" == "existing" ]]; then
    mkdir -p "$app_root/conf" "$app_root/data"
    printf 'old-binary\n' > "$app_root/bioBox"
    chmod a+x "$app_root/bioBox"
    printf 'old-unit\n' > "$systemd_dir/bioBox.service"
    printf 'storage_cfg:\n  storage_sqlite:\n    path: "./data/bioBox.db"\n' > "$app_root/conf/config.yaml"
    printf 'sqlite-biobox-sentinel\n' > "$app_root/data/bioBox.db"
    for app in protenix pxdesign; do
      mkdir -p "$app_root/runtime/$app/releases/old" "$app_root/runtime/$app/conf"
      echo "$app" > "$app_root/runtime/$app/.installed_by"
      ln -sfn releases/old "$app_root/runtime/$app/current"
    done
  fi
  printf 'new-binary\n' > "$pkg/bioBox"
  chmod a+x "$pkg/bioBox"
  printf 'new-unit\n' > "$pkg/bioBox.service"
  printf 'storage_cfg:\n  storage_sqlite:\n    path: "./data/bioBox.db"\n' > "$pkg/conf/config.yaml"
  cp -f example_files/install_bioBox.sh "$pkg/install_bioBox.sh"
  chmod a+x "$pkg/install_bioBox.sh"
  write_runtime_stub "$pkg/runtime/protenix/install_protenix.sh"
  write_runtime_stub "$pkg/runtime/pxdesign/install_pxdesign.sh"
  write_systemctl_stub "$bindir/systemctl"
  printf '#!/bin/bash\nexit 0\n' > "$bindir/uv"
  chmod a+x "$bindir/uv"
  mkdir -p "$root/binds"
  echo "$app_root" > "$root/binds/app_root"
  echo "$systemd_dir" > "$root/binds/systemd_dir"
  echo "$pkg" > "$root/binds/pkg"
  echo "$bindir" > "$root/binds/bindir"
  echo "$statedir" > "$root/binds/statedir"
}

run_installer() {
  local world=$1
  local input_seq=$2
  local extra_setup=${3:-}
  local app_root systemd_dir pkg bindir statedir
  app_root=$(cat "$out/$world/binds/app_root")
  systemd_dir=$(cat "$out/$world/binds/systemd_dir")
  pkg=$(cat "$out/$world/binds/pkg")
  bindir=$(cat "$out/$world/binds/bindir")
  statedir=$(cat "$out/$world/binds/statedir")
  local binds=(
    --unshare-net --tmpfs /
    --dev /dev --proc /proc
    --tmpfs /tmp
    --ro-bind /usr /usr
    --ro-bind /bin /bin
    --ro-bind /lib /lib
    --bind "$systemd_dir" /etc/systemd/system
    --bind "$(dirname "$pkg")" /opt/pkg
    --bind "$bindir" /fixture-bin
    --bind "$statedir" /fixture-state
    --chdir /opt/pkg/bioBox
    --setenv PATH /fixture-bin:/usr/bin:/bin
    --setenv HOME /tmp
    --setenv UV /fixture-bin/uv
    --setenv APP_DIR /bioBox
  )
  if [[ -d "$app_root" ]]; then
    binds+=(--bind "$app_root" /bioBox)
  fi
  if [[ -d /lib64 ]]; then binds+=(--ro-bind /lib64 /lib64); fi
  if [[ -d /sbin ]]; then binds+=(--ro-bind /sbin /sbin); fi
  if [[ -n "$extra_setup" ]]; then
    # shellcheck disable=SC2206
    binds+=($extra_setup)
  fi
  set +e
  printf '%b' "$input_seq" | bwrap "${binds[@]}" bash -c "$extra_cmd_prefix bash ./install_bioBox.sh" \
    >"$out/$world/installer.log" 2>&1
  local rc=$?
  set -e
  echo "$rc" > "$out/$world/installer.rc"
}

run_update() {
  local world=$1
  extra_cmd_prefix=""
  run_installer "$world" '2\nyes\n' "${2:-}"
}

run_install() {
  local world=$1
  extra_cmd_prefix=""
  run_installer "$world" '1\n' "${2:-}"
}

current_of() { basename "$(readlink "$1")"; }

fail() { echo "instance update restore fixture FAILED: $*" >&2; exit 1; }

assert_no_side_effects() {
  local world=$1
  local statedir="$out/$world/state"
  if [[ -f "$statedir/systemctl.log" ]] && grep -Eq '^(stop|start|enable|disable|restart|daemon-reload)\b' "$statedir/systemctl.log"; then
    fail "$world unexpectedly mutated systemd: $(cat "$statedir/systemctl.log")"
  fi
  if [[ -f "$out/$world/root/bioBox/events.log" ]]; then
    fail "$world unexpectedly touched runtimes: $(cat "$out/$world/root/bioBox/events.log")"
  fi
}

# Case 1: first runtime updates, second fails; both currents must go back, old service starts.
setup_world complete_restore existing
touch "$(cat "$out/complete_restore/binds/app_root")/runtime/pxdesign/fail-update"
run_update complete_restore
app=$(cat "$out/complete_restore/binds/app_root")
systemd=$(cat "$out/complete_restore/binds/systemd_dir")
rc=$(cat "$out/complete_restore/installer.rc")
[[ "$rc" -eq 1 ]] || fail "complete_restore exit $rc"
[[ "$(current_of "$app/runtime/protenix/current")" == old ]] || fail "protenix not restored"
[[ "$(current_of "$app/runtime/pxdesign/current")" == old ]] || fail "pxdesign not restored"
grep -qx old-binary "$app/bioBox" || fail "binary changed"
grep -qx old-unit "$systemd/bioBox.service" || fail "unit changed"
grep -qx 'start bioBox.service' "$app/systemctl-fixture.log" || fail "old service not started"
grep -q 'runtimes restored' "$out/complete_restore/installer.log" || fail "missing restore message"

# Case 2: first runtime updates, second fails, rollback of the first also fails; do not start mixed.
setup_world mixed_no_start existing
touch "$(cat "$out/mixed_no_start/binds/app_root")/runtime/pxdesign/fail-update"
touch "$(cat "$out/mixed_no_start/binds/app_root")/runtime/protenix/fail-rollback"
run_update mixed_no_start
app=$(cat "$out/mixed_no_start/binds/app_root")
rc=$(cat "$out/mixed_no_start/installer.rc")
[[ "$rc" -eq 1 ]] || fail "mixed_no_start exit $rc"
[[ "$(current_of "$app/runtime/protenix/current")" == new ]] || fail "expected leftover protenix=new"
[[ "$(current_of "$app/runtime/pxdesign/current")" == old ]] || fail "expected pxdesign=old"
grep -qx old-binary "$app/bioBox" || fail "binary changed in mixed case"
if grep -qx 'start bioBox.service' "$app/systemctl-fixture.log"; then
  fail "mixed state started the service"
fi
grep -q 'not starting bioBox with a mixed runtime or binary' "$out/mixed_no_start/installer.log" || fail "missing mixed-state message"

# Case 3: clean new install (no /bioBox, no /bb_service, no bb_service.service, no old DB) -> succeeds.
setup_world clean_install_ok empty
app_clean="$out/clean_install_ok/root/bioBox"
mkdir -p "$app_clean"
run_install clean_install_ok
rc=$(cat "$out/clean_install_ok/installer.rc")
[[ "$rc" -eq 0 ]] || fail "clean_install_ok exit $rc: $(cat "$out/clean_install_ok/installer.log")"
grep -qx new-binary "$app_clean/bioBox" || fail "clean_install_ok did not install bioBox"
grep -qx new-unit "$out/clean_install_ok/root/etc/systemd/system/bioBox.service" || fail "clean_install_ok did not install unit"
grep -qx 'start bioBox.service' "$out/clean_install_ok/state/systemctl.log" || fail "clean_install_ok did not start bioBox.service"

# Case 4: clean bioBox self-update -> succeeds without false block.
setup_world biobox_self_update_ok existing
run_update biobox_self_update_ok
app_upd=$(cat "$out/biobox_self_update_ok/binds/app_root")
rc=$(cat "$out/biobox_self_update_ok/installer.rc")
[[ "$rc" -eq 0 ]] || fail "biobox_self_update_ok exit $rc: $(cat "$out/biobox_self_update_ok/installer.log")"
grep -qx new-binary "$app_upd/bioBox" || fail "biobox_self_update_ok did not update bioBox"
grep -qx new-unit "$out/biobox_self_update_ok/root/etc/systemd/system/bioBox.service" || fail "biobox_self_update_ok did not update unit"
grep -qx 'sqlite-biobox-sentinel' "$app_upd/data/bioBox.db" || fail "biobox_self_update_ok modified bioBox.db"

# Case 5: legacy /bb_service directory blocks install with zero side effects.
setup_world legacy_dir_blocks_install empty
legacy_dir="$out/legacy_dir_blocks_install/legacy_bb_service"
mkdir -p "$legacy_dir/data"
printf 'legacy-db\n' > "$legacy_dir/data/bb_service.db"
extra_cmd_prefix=""
run_installer legacy_dir_blocks_install '1\n' "--bind $legacy_dir /bb_service"
rc=$(cat "$out/legacy_dir_blocks_install/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_dir_blocks_install unexpectedly succeeded"
grep -q '/bb_service' "$out/legacy_dir_blocks_install/installer.log" || fail "legacy_dir_blocks_install missing /bb_service in error"
grep -q 'bb_service' "$out/legacy_dir_blocks_install/installer.log" || fail "legacy_dir_blocks_install missing bb_service identity"
grep -qx 'legacy-db' "$legacy_dir/data/bb_service.db" || fail "legacy DB modified"
assert_no_side_effects legacy_dir_blocks_install

# Case 6: legacy /bb_service directory blocks update with zero side effects.
setup_world legacy_dir_blocks_update existing
legacy_dir_u="$out/legacy_dir_blocks_update/legacy_bb_service"
mkdir -p "$legacy_dir_u"
run_update legacy_dir_blocks_update "--bind $legacy_dir_u /bb_service"
rc=$(cat "$out/legacy_dir_blocks_update/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_dir_blocks_update unexpectedly succeeded"
grep -qx old-binary "$out/legacy_dir_blocks_update/root/bioBox/bioBox" || fail "bioBox changed despite legacy dir"
assert_no_side_effects legacy_dir_blocks_update

# Case 7: legacy /bb_service dangling symlink blocks install with zero side effects.
setup_world legacy_dir_symlink_blocks empty
extra_cmd_prefix='ln -s /nonexistent/old_bb_service /bb_service;'
run_installer legacy_dir_symlink_blocks '1\n'
rc=$(cat "$out/legacy_dir_symlink_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_dir_symlink_blocks unexpectedly succeeded"
grep -q '/bb_service' "$out/legacy_dir_symlink_blocks/installer.log" || fail "legacy_dir_symlink_blocks missing /bb_service"
assert_no_side_effects legacy_dir_symlink_blocks

# Case 8: legacy bb_service.service active blocks update with zero side effects.
setup_world legacy_unit_active_blocks existing
printf 'LoadState=loaded\nActiveState=active\nUnitFileState=enabled\n' > "$out/legacy_unit_active_blocks/state/bb_service.show.out"
touch "$out/legacy_unit_active_blocks/state/bb_service.active"
run_update legacy_unit_active_blocks
rc=$(cat "$out/legacy_unit_active_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_unit_active_blocks unexpectedly succeeded"
grep -q 'bb_service.service' "$out/legacy_unit_active_blocks/installer.log" || fail "legacy_unit_active_blocks missing unit name"
assert_no_side_effects legacy_unit_active_blocks

# Case 9: legacy bb_service.service inactive leftover unit file blocks install.
setup_world legacy_unit_inactive_file_blocks empty
printf 'legacy-unit\n' > "$out/legacy_unit_inactive_file_blocks/root/etc/systemd/system/bb_service.service"
printf 'LoadState=loaded\nActiveState=inactive\nUnitFileState=disabled\n' > "$out/legacy_unit_inactive_file_blocks/state/bb_service.show.out"
run_install legacy_unit_inactive_file_blocks
rc=$(cat "$out/legacy_unit_inactive_file_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_unit_inactive_file_blocks unexpectedly succeeded"
grep -q 'bb_service.service' "$out/legacy_unit_inactive_file_blocks/installer.log" || fail "missing bb_service.service in error"
assert_no_side_effects legacy_unit_inactive_file_blocks

# Case 10: legacy bb_service.service dangling symlink (even if systemctl show says not-found) blocks install.
setup_world legacy_unit_symlink_blocks empty
ln -s /bb_service/bb_service.service "$out/legacy_unit_symlink_blocks/root/etc/systemd/system/bb_service.service"
run_install legacy_unit_symlink_blocks
rc=$(cat "$out/legacy_unit_symlink_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_unit_symlink_blocks unexpectedly succeeded"
grep -q 'bb_service.service' "$out/legacy_unit_symlink_blocks/installer.log" || fail "missing bb_service.service symlink error"
assert_no_side_effects legacy_unit_symlink_blocks

# Case 11: legacy bb_service.service masked in systemd (no /etc unit file) blocks update.
setup_world legacy_unit_masked_systemctl_blocks existing
printf 'LoadState=masked\nActiveState=inactive\nUnitFileState=masked\n' > "$out/legacy_unit_masked_systemctl_blocks/state/bb_service.show.out"
run_update legacy_unit_masked_systemctl_blocks
rc=$(cat "$out/legacy_unit_masked_systemctl_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_unit_masked_systemctl_blocks unexpectedly succeeded"
grep -q 'bb_service.service' "$out/legacy_unit_masked_systemctl_blocks/installer.log" || fail "missing masked unit error"
assert_no_side_effects legacy_unit_masked_systemctl_blocks

# Case 12: known legacy DB /bioBox/data/bb_service.db blocks update without touching DB.
setup_world legacy_known_db_biobox_dir_blocks existing
printf 'legacy-sqlite-bytes\n' > "$out/legacy_known_db_biobox_dir_blocks/root/bioBox/data/bb_service.db"
run_update legacy_known_db_biobox_dir_blocks
rc=$(cat "$out/legacy_known_db_biobox_dir_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_known_db_biobox_dir_blocks unexpectedly succeeded"
grep -q 'bb_service.db' "$out/legacy_known_db_biobox_dir_blocks/installer.log" || fail "missing bb_service.db in error"
grep -qx 'legacy-sqlite-bytes' "$out/legacy_known_db_biobox_dir_blocks/root/bioBox/data/bb_service.db" || fail "legacy DB modified"
assert_no_side_effects legacy_known_db_biobox_dir_blocks

# Case 13: existing /bioBox/conf/config.yaml pointing to legacy protenix_service.db/bb_service.db blocks install.
setup_world legacy_known_db_config_ref_blocks existing
printf 'storage_cfg:\n  storage_sqlite:\n    path: "./data/protenix_service.db"\n' > "$out/legacy_known_db_config_ref_blocks/root/bioBox/conf/config.yaml"
run_install legacy_known_db_config_ref_blocks
rc=$(cat "$out/legacy_known_db_config_ref_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "legacy_known_db_config_ref_blocks unexpectedly succeeded"
grep -q 'config.yaml' "$out/legacy_known_db_config_ref_blocks/installer.log" || fail "missing legacy config.yaml ref in error"
assert_no_side_effects legacy_known_db_config_ref_blocks

# Case 14: missing systemctl binary must fail closed (cannot assume bb_service absent).
setup_world missing_systemctl_blocks empty
rm -f "$out/missing_systemctl_blocks/bin/systemctl"
run_install missing_systemctl_blocks
rc=$(cat "$out/missing_systemctl_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "missing_systemctl_blocks unexpectedly succeeded"
grep -qi 'systemctl' "$out/missing_systemctl_blocks/installer.log" || fail "missing systemctl failure explanation"
assert_no_side_effects missing_systemctl_blocks

# Case 15: unreadable/non-zero systemctl show must fail closed (cannot assume absent).
setup_world unknown_systemctl_state_blocks existing
printf '1\n' > "$out/unknown_systemctl_state_blocks/state/bb_service.show.rc"
printf 'System has not been booted with systemd as init system\n' > "$out/unknown_systemctl_state_blocks/state/bb_service.show.out"
run_update unknown_systemctl_state_blocks
rc=$(cat "$out/unknown_systemctl_state_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "unknown_systemctl_state_blocks unexpectedly succeeded"
grep -qi 'systemctl' "$out/unknown_systemctl_state_blocks/installer.log" || fail "missing systemctl show error explanation"
assert_no_side_effects unknown_systemctl_state_blocks

# Case 16: systemctl show exits 0 with empty output must fail closed (cannot assume absent).
setup_world empty_systemctl_output_blocks empty
: > "$out/empty_systemctl_output_blocks/state/bb_service.show.out"
run_install empty_systemctl_output_blocks
rc=$(cat "$out/empty_systemctl_output_blocks/installer.rc")
[[ "$rc" -ne 0 ]] || fail "empty_systemctl_output_blocks unexpectedly succeeded"
grep -qi 'systemctl' "$out/empty_systemctl_output_blocks/installer.log" || fail "missing empty systemctl show explanation"
assert_no_side_effects empty_systemctl_output_blocks

{
  echo "fixture_only=true"
  echo "host_root_install=false"
  echo "cases_passed=16"
  echo "complete_restore_exit=$(cat "$out/complete_restore/installer.rc")"
  echo "mixed_no_start_exit=$(cat "$out/mixed_no_start/installer.rc")"
  echo "clean_install_ok_exit=$(cat "$out/clean_install_ok/installer.rc")"
  echo "biobox_self_update_ok_exit=$(cat "$out/biobox_self_update_ok/installer.rc")"
  echo "legacy_dir_blocks_install_exit=$(cat "$out/legacy_dir_blocks_install/installer.rc")"
  echo "legacy_dir_blocks_update_exit=$(cat "$out/legacy_dir_blocks_update/installer.rc")"
  echo "legacy_dir_symlink_blocks_exit=$(cat "$out/legacy_dir_symlink_blocks/installer.rc")"
  echo "legacy_unit_active_blocks_exit=$(cat "$out/legacy_unit_active_blocks/installer.rc")"
  echo "legacy_unit_inactive_file_blocks_exit=$(cat "$out/legacy_unit_inactive_file_blocks/installer.rc")"
  echo "legacy_unit_symlink_blocks_exit=$(cat "$out/legacy_unit_symlink_blocks/installer.rc")"
  echo "legacy_unit_masked_systemctl_blocks_exit=$(cat "$out/legacy_unit_masked_systemctl_blocks/installer.rc")"
  echo "legacy_known_db_biobox_dir_blocks_exit=$(cat "$out/legacy_known_db_biobox_dir_blocks/installer.rc")"
  echo "legacy_known_db_config_ref_blocks_exit=$(cat "$out/legacy_known_db_config_ref_blocks/installer.rc")"
  echo "missing_systemctl_blocks_exit=$(cat "$out/missing_systemctl_blocks/installer.rc")"
  echo "unknown_systemctl_state_blocks_exit=$(cat "$out/unknown_systemctl_state_blocks/installer.rc")"
  echo "empty_systemctl_output_blocks_exit=$(cat "$out/empty_systemctl_output_blocks/installer.rc")"
} > "$out/result.txt"

echo "instance install/update legacy-preflight & restore fixtures PASS (16 cases)"
cat "$out/result.txt"

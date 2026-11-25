#!/bin/bash
# bioBox 安装脚本（沿用 project_template_go 的 install 体系，需 root 执行）。
# 服务与原生运行时（Protenix / PXDesign 的 Python 环境、权重、run 入口）是同一套服务，
# 统一安装在 ${APP_DIR}（/bioBox）下：
#   ${APP_DIR}/bioBox              服务二进制（旧版本保留为 .bak_<时间戳>）
#   ${APP_DIR}/conf/config.yaml          服务配置（重装时旧配置保留为 .bak_<时间戳>）
#   ${APP_DIR}/data/  ${APP_DIR}/logs/   SQLite 数据与作业目录 / 日志
#   ${APP_DIR}/runtime/protenix|pxdesign 原生运行时（bin/<app>-run、conf/<app>.env、data/ 权重、
#                                        releases/<版本>/venv、current、previous）
#   ${APP_DIR}/runtime/uv/               uv 托管的 Python 解释器与缓存
# 菜单：1 安装/重装，2 升级（换二进制 + 升级运行时），3 整体卸载，4 运行时回退上一版本，5 查看状态。

set -e

APP_NAME=bioBox
configFIleName=config.yaml
APP_DIR="/${APP_NAME}"

SYSTEMD_PATH="/etc/systemd/system"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
PARENT_DIR=$(dirname ${SCRIPT_DIR})

# 原生运行时与服务同根：${APP_DIR}/runtime/<app>
RUNTIME_APPS=(protenix pxdesign)
RUNTIME_DIR="${APP_DIR}/runtime"
# venv 依赖的 uv 托管解释器与缓存也放进同一根目录，整体卸载时一起删除
export UV_PYTHON_INSTALL_DIR="${RUNTIME_DIR}/uv/python"
export UV_CACHE_DIR="${RUNTIME_DIR}/uv/cache"

echo parent dir is:$PARENT_DIR

function clear_install_files() {
    if [ -d "${PARENT_DIR}" ]; then
        find "${PARENT_DIR}" -type f -name "${APP_NAME}*" -exec rm -f {} \;
        find "${PARENT_DIR}" -type d -name "${APP_NAME}*" -empty -delete
        rm -rf ${SCRIPT_DIR}
    fi
}

# 发布包里每个运行时都带自己的 install_<app>.sh（版本目录 + current/previous + rollback）
function runtime_pkg_ok() {
    local app
    for app in "${RUNTIME_APPS[@]}"; do
        if [[ ! -f "${SCRIPT_DIR}/runtime/${app}/install_${app}.sh" ]]; then
            echo "installation file error: missing runtime/${app}/install_${app}.sh"
            return 1
        fi
    done
}

# 运行时构建要用 uv（root 下通常不在 PATH，可用 UV=/path/to/uv 指定）
function runtime_uv_ok() {
    if [[ -n "${UV:-}" ]]; then
        [[ -x "${UV}" ]] || { echo "UV=${UV} is not executable"; return 1; }
    elif ! command -v uv >/dev/null 2>&1 && [[ ! -x "${HOME}/.local/bin/uv" ]]; then
        echo "uv not found; install uv or run with UV=/path/to/uv"
        return 1
    fi
}

# 整合模式：显式把运行时目录传给 install_<app>.sh（<APP>_PREFIX），覆盖调用者环境里的同名变量，
# 不使用独立模式的默认目录 ~/.local/share/<app>
function runtime_cmd() {
    local app=$1
    shift
    local prefix_var
    prefix_var="$(echo "${app}" | tr 'a-z' 'A-Z')_PREFIX"
    env "${prefix_var}=${RUNTIME_DIR}/${app}" bash "${SCRIPT_DIR}/runtime/${app}/install_${app}.sh" "$@"
}

function runtime_installed() {
    [[ -f "${RUNTIME_DIR}/$1/.installed_by" && -L "${RUNTIME_DIR}/$1/current" ]]
}

function runtime_current() {
    local link="${RUNTIME_DIR}/$1/current"
    if [[ -L "${link}" ]]; then
        basename "$(readlink "${link}")"
    fi
}

function snapshot_runtime_currents() {
    local app
    RUNTIME_SNAPSHOT_APPS=()
    RUNTIME_SNAPSHOT_CURRENTS=()
    for app in "${RUNTIME_APPS[@]}"; do
        RUNTIME_SNAPSHOT_APPS+=("${app}")
        RUNTIME_SNAPSHOT_CURRENTS+=("$(runtime_current "${app}")")
    done
}

# Restore runtimes whose current changed in this invocation. Returns 0 only when
# every current is back to the snapshot; a leftover mixed tree is a failed restore.
function restore_updated_runtimes() {
    local i app before now rc=0
    for i in "${!RUNTIME_SNAPSHOT_APPS[@]}"; do
        app="${RUNTIME_SNAPSHOT_APPS[$i]}"
        before="${RUNTIME_SNAPSHOT_CURRENTS[$i]}"
        now="$(runtime_current "${app}")"
        if [[ "${now}" == "${before}" ]]; then
            continue
        fi
        if [[ -n "${before}" && -L "${RUNTIME_DIR}/${app}/previous" ]]; then
            if ! runtime_cmd "${app}" rollback; then
                echo "${app}: rollback after failed update did not succeed"
                rc=1
            fi
        elif [[ -z "${before}" ]] && runtime_installed "${app}"; then
            if ! runtime_cmd "${app}" uninstall; then
                echo "${app}: uninstall after failed install did not succeed"
                rc=1
            fi
        else
            echo "${app}: current changed during a failed service update and cannot be restored"
            rc=1
        fi
        now="$(runtime_current "${app}")"
        if [[ "${now}" != "${before}" ]]; then
            echo "${app}: current is ${now:-none}, wanted ${before:-none}"
            rc=1
        fi
    done
    return ${rc}
}

# Port of the template helper: restore files replaced in this update.
# Only start the previous service when the caller confirmed a complete restore.
function restore_previous_update() {
    local restart_old=$1
    if [[ -n "${BINARY_BAK:-}" && -f "${BINARY_BAK}" ]]; then
        mv -f "${BINARY_BAK}" "${APP_DIR}/${APP_NAME}"
        BINARY_BAK=""
    fi
    if [[ -n "${UNIT_BAK:-}" && -f "${UNIT_BAK}" ]]; then
        mv -f "${UNIT_BAK}" "${SYSTEMD_PATH}/${APP_NAME}.service"
        UNIT_BAK=""
        systemctl daemon-reload
    fi
    if [[ "${restart_old}" == "start" ]]; then
        systemctl start "${APP_NAME}.service"
    fi
}

function restart_previous_if_restored() {
    local restore_rc=$1
    shift
    if [[ ${restore_rc} -eq 0 ]]; then
        echo "$*; restarting previous service"
        systemctl start "${APP_NAME}.service"
    else
        echo "$*; not starting ${APP_NAME} with a mixed runtime or binary"
    fi
}

# 安装或升级全部运行时。$1=reinstall 时先把 conf/<app>.env 备份为 .bak_<时间戳> 再写入新模板（与 config.yaml 同语义）
function install_runtimes() {
    local mode=$1 app env_file
    mkdir -p "${RUNTIME_DIR}"
    for app in "${RUNTIME_APPS[@]}"; do
        env_file="${RUNTIME_DIR}/${app}/conf/${app}.env"
        if [[ "${mode}" == "reinstall" && -f "${env_file}" ]]; then
            mv "${env_file}" "${env_file}.bak_${TIMESTAMP}"
        fi
        if runtime_installed "${app}"; then
            runtime_cmd "${app}" update || return 1
        else
            runtime_cmd "${app}" install || return 1
        fi
    done
}

function check_legacy_path() {
    local path=$1 desc=$2 target=""
    if [[ -L "${path}" ]]; then
        target=$(readlink "${path}" 2>/dev/null || true)
        if [[ -e "${path}" ]]; then
            echo "[legacy-preflight] detected ${desc} symlink (identity: bb_service): ${path} -> ${target}" >&2
        else
            echo "[legacy-preflight] detected dangling ${desc} symlink (identity: bb_service): ${path} -> ${target}" >&2
        fi
        return 0
    elif [[ -e "${path}" ]]; then
        echo "[legacy-preflight] detected ${desc} (identity: bb_service): ${path}" >&2
        return 0
    fi
    return 1
}

# Read-only legacy identity preflight before any install/update side effects.
# Refuses silently creating a fresh bioBox.db or colliding on ports with legacy bb_service.
# Fails closed when required inspection tools or paths are unavailable/undetermined.
function legacy_identity_preflight() {
    local legacy_app="bb_service"
    local legacy_dir="/${legacy_app}"
    local legacy_unit="${legacy_app}.service"
    local blocked=0
    local had_nullglob=0
    local p show_out show_rc load_state="" active_state="" unit_file_state="" line cfg_text=""

    if [[ ! -r / ]]; then
        echo "[legacy-preflight] cannot read /; unable to verify whether legacy ${legacy_dir} exists" >&2
        blocked=1
    fi

    if ! command -v systemctl >/dev/null 2>&1; then
        echo "[legacy-preflight] systemctl not found; cannot verify whether legacy ${legacy_unit} exists or is active" >&2
        blocked=1
    fi

    if [[ ! -d "${SYSTEMD_PATH}" || ! -r "${SYSTEMD_PATH}" ]]; then
        echo "[legacy-preflight] systemd directory missing or unreadable (${SYSTEMD_PATH}); cannot verify legacy ${legacy_unit}" >&2
        blocked=1
    fi

    if check_legacy_path "${legacy_dir}" "legacy directory"; then
        blocked=1
        if [[ -d "${legacy_dir}" && ! -r "${legacy_dir}" ]]; then
            echo "[legacy-preflight] legacy directory is unreadable: ${legacy_dir}" >&2
        fi
    fi

    for p in         "${legacy_dir}/data/bb_service.db"         "${legacy_dir}/data/protenix_service.db"         "${APP_DIR}/data/bb_service.db"         "${APP_DIR}/data/protenix_service.db"; do
        if check_legacy_path "${p}" "legacy database"; then
            blocked=1
        fi
    done

    if [[ -e "${APP_DIR}/data" && ! -r "${APP_DIR}/data" ]]; then
        echo "[legacy-preflight] data directory unreadable (${APP_DIR}/data); cannot verify absence of legacy database" >&2
        blocked=1
    fi

    if [[ -L "${APP_DIR}/conf/${configFIleName}" || -e "${APP_DIR}/conf/${configFIleName}" ]]; then
        if [[ ! -r "${APP_DIR}/conf/${configFIleName}" ]]; then
            echo "[legacy-preflight] existing config unreadable (${APP_DIR}/conf/${configFIleName}); cannot verify legacy DB/path references" >&2
            blocked=1
        else
            cfg_text=$(<"${APP_DIR}/conf/${configFIleName}")
            if [[ "${cfg_text}" == *"bb_service.db"* || "${cfg_text}" == *"protenix_service.db"* || "${cfg_text}" == *"/bb_service"* ]]; then
                echo "[legacy-preflight] existing config references legacy identity bb_service/protenix_service.db: ${APP_DIR}/conf/${configFIleName}" >&2
                blocked=1
            fi
        fi
    fi

    if [[ -d "${SYSTEMD_PATH}" && -r "${SYSTEMD_PATH}" ]]; then
        shopt -q nullglob && had_nullglob=1 || true
        shopt -s nullglob
        for p in             "${SYSTEMD_PATH}/${legacy_unit}"             "${SYSTEMD_PATH}/${legacy_unit}.d"             "${SYSTEMD_PATH}"/*.wants/"${legacy_unit}"             "${SYSTEMD_PATH}"/*.requires/"${legacy_unit}"             "/lib/systemd/system/${legacy_unit}"             "/lib/systemd/system/${legacy_unit}.d"             "/usr/lib/systemd/system/${legacy_unit}"             "/usr/lib/systemd/system/${legacy_unit}.d"; do
            if check_legacy_path "${p}" "legacy systemd unit artifact"; then
                blocked=1
            fi
        done
        if [[ ${had_nullglob} -eq 0 ]]; then
            shopt -u nullglob
        fi
    fi

    if command -v systemctl >/dev/null 2>&1; then
        set +e
        show_out=$(systemctl show -p LoadState,ActiveState,UnitFileState -- "${legacy_unit}" 2>&1)
        show_rc=$?
        set -e
        if [[ ${show_rc} -ne 0 ]]; then
            echo "[legacy-preflight] systemctl show failed for ${legacy_unit} (exit=${show_rc}): ${show_out}" >&2
            blocked=1
        else
            while IFS= read -r line; do
                case "${line}" in
                    LoadState=*) load_state="${line#LoadState=}" ;;
                    ActiveState=*) active_state="${line#ActiveState=}" ;;
                    UnitFileState=*) unit_file_state="${line#UnitFileState=}" ;;
                esac
            done <<< "${show_out}"
            if [[ -z "${load_state}" || -z "${active_state}" ]]; then
                echo "[legacy-preflight] systemctl show returned incomplete state for ${legacy_unit} (LoadState='${load_state}', ActiveState='${active_state}'); refusing to assume absent" >&2
                blocked=1
            elif [[ "${load_state}" != "not-found" || "${active_state}" != "inactive" || ( -n "${unit_file_state}" && "${unit_file_state}" != "not-found" ) ]]; then
                echo "[legacy-preflight] detected legacy systemd unit ${legacy_unit} (LoadState=${load_state}, ActiveState=${active_state}, UnitFileState=${unit_file_state:-none})" >&2
                blocked=1
            fi
        fi
    fi

    if [[ ${blocked} -ne 0 ]]; then
        echo "[legacy-preflight] refusing ${APP_NAME} install/update: legacy bb_service installation/unit/database detected or state undetermined." >&2
        echo "[legacy-preflight] no legacy service, directory, or database was stopped, deleted, or migrated automatically." >&2
        echo "[legacy-preflight] stop/disable bb_service.service and explicitly migrate or archive legacy /bb_service data before installing or updating ${APP_NAME}." >&2
        return 1
    fi
    return 0
}

function service_active() {
    systemctl is-active --quiet "${APP_NAME}.service" 2>/dev/null
}

function install() {

    if [[ -f ${SCRIPT_DIR}/${APP_NAME} ]] && [[ -f ${SCRIPT_DIR}/${APP_NAME}.service ]] && [[ -f ${SCRIPT_DIR}/conf/${configFIleName} ]] && runtime_pkg_ok; then
        legacy_identity_preflight || exit 1
        runtime_uv_ok || exit 1
        # 目录可能因运行时等已存在，子目录每次都确保存在
        mkdir -p ${APP_DIR}
        mkdir -p ${APP_DIR}/logs
        mkdir -p ${APP_DIR}/conf

        TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")

        # install runtimes first: 失败时不动二进制与配置，并恢复本次已改动的运行时
        was_active=0
        if service_active; then
            was_active=1
            systemctl stop "${APP_NAME}.service"
        fi
        snapshot_runtime_currents
        if ! install_runtimes reinstall; then
            echo "runtime installation failed; ${APP_NAME} binary/config unchanged"
            if restore_updated_runtimes && [[ ${was_active} -eq 1 ]]; then
                echo "runtimes restored to the pre-install current; restarting previous service"
                systemctl start "${APP_NAME}.service"
            elif [[ ${was_active} -eq 1 ]]; then
                echo "runtime restore incomplete; not starting ${APP_NAME} with mixed runtimes"
            fi
            exit 1
        fi

        # install binary
        if [[ -f "${APP_DIR}/${APP_NAME}" ]]; then
            mv "${APP_DIR}/${APP_NAME}" "${APP_DIR}/${APP_NAME}.bak_${TIMESTAMP}"
        fi
        cp -f "${SCRIPT_DIR}/${APP_NAME}" "${APP_DIR}/${APP_NAME}"

        # configure file
        if [[ -f "${APP_DIR}/conf/${configFIleName}" ]]; then
            mv "${APP_DIR}/conf/${configFIleName}" "${APP_DIR}/conf/${configFIleName}.bak_${TIMESTAMP}"
        fi
        cp -f ${SCRIPT_DIR}/conf/${configFIleName} ${APP_DIR}/conf/${configFIleName}

        # install systemd file to SYSTEMD_PATH and restart service
        service_file=${SCRIPT_DIR}/${APP_NAME}.service
        if [[ -f "${service_file}" ]]; then
            cp -f "${service_file}" "${SYSTEMD_PATH}"
            service_name=$(basename "${service_file}" .service)
            systemctl enable "${service_name}"
            systemctl daemon-reload
            systemctl start "${service_name}.service"
        fi
    else
        echo "installation file error"
        exit 1
    fi
}

function updateBinary() {
    read -p "sure want to update ${APP_NAME} (binary + runtime) [yes/no]：" flag
    if [ -z $flag ]; then
        echo "input error" && exit 1
    elif [ "$flag" = "yes" -o "$flag" = "ye" -o "$flag" = "y" ]; then
        runtime_pkg_ok || exit 1
        legacy_identity_preflight || exit 1
        runtime_uv_ok || exit 1
        TIMESTAMP=$(date +"%Y-%m-%d_%H-%M-%S")
        BINARY_BAK=""
        UNIT_BAK=""
        systemctl stop "${APP_NAME}.service"

        # update runtimes（保留 conf/<app>.env 与权重）。任一运行时失败则回退本次已切换的 current。
        snapshot_runtime_currents
        if ! install_runtimes update; then
            if restore_updated_runtimes; then
                restart_previous_if_restored 0 "runtime update failed; runtimes restored, ${APP_NAME} binary unchanged"
            else
                restart_previous_if_restored 1 "runtime update failed; runtime restore incomplete, ${APP_NAME} binary unchanged"
            fi
            exit 1
        fi

        # update binary
        if [[ -f "${APP_DIR}/${APP_NAME}" ]]; then
            BINARY_BAK="${APP_DIR}/${APP_NAME}.bak_${TIMESTAMP}"
            mv "${APP_DIR}/${APP_NAME}" "${BINARY_BAK}"
        fi
        cp -f "${SCRIPT_DIR}/${APP_NAME}" "${APP_DIR}/${APP_NAME}"

        # update systemd service
        service_file=${SCRIPT_DIR}/${APP_NAME}.service
        if [[ -f "${service_file}" ]]; then
            if [[ -f "${SYSTEMD_PATH}/${APP_NAME}.service" ]]; then
                UNIT_BAK="${SYSTEMD_PATH}/${APP_NAME}.service.bak_${TIMESTAMP}"
                cp -f "${SYSTEMD_PATH}/${APP_NAME}.service" "${UNIT_BAK}"
            fi
            cp -f "${service_file}" "${SYSTEMD_PATH}"
            systemctl daemon-reload
            if ! systemctl restart "${APP_NAME}.service"; then
                echo "update start failed; restoring previous ${APP_NAME} binary, unit, and runtimes"
                restore_previous_update ""
                if restore_updated_runtimes; then
                    restart_previous_if_restored 0 "previous binary, unit, and runtimes restored"
                else
                    restart_previous_if_restored 1 "binary/unit may be restored but runtime restore incomplete"
                fi
                exit 1
            fi
        fi
    fi
}

function uninstall() {
    read -p "sure want to uninstall ${APP_NAME} (including ${RUNTIME_DIR}: venvs and weights) [yes/no]：" flag
    if [ -z "$flag" ]; then
        echo "input error" && exit 1
    elif [ "$flag" = "yes" ] || [ "$flag" = "ye" ] || [ "$flag" = "y" ]; then
        for service_file in ${SYSTEMD_PATH}/${APP_NAME}*.service; do
            if [[ -f ${service_file} ]]; then
                service_name=$(basename ${service_file} .service)
                systemctl disable --now ${service_name}
                rm -f ${service_file}
            fi
        done

        rm -rf ${APP_DIR}
        echo "uninstall ${APP_NAME} success"
    fi
}

function rollbackRuntime() {
    read -p "sure want to rollback runtime (protenix/pxdesign) to previous release [yes/no]：" flag
    if [ -z "$flag" ]; then
        echo "input error" && exit 1
    elif [ "$flag" = "yes" ] || [ "$flag" = "ye" ] || [ "$flag" = "y" ]; then
        runtime_pkg_ok || exit 1
        was_active=0
        if service_active; then
            was_active=1
            systemctl stop "${APP_NAME}.service"
        fi
        rc=0
        for app in "${RUNTIME_APPS[@]}"; do
            if [[ -L "${RUNTIME_DIR}/${app}/previous" ]]; then
                runtime_cmd "${app}" rollback || rc=1
            else
                echo "${app}: no previous release, skip"
            fi
        done
        [[ ${was_active} -eq 1 ]] && systemctl start "${APP_NAME}.service"
        return ${rc}
    fi
}

function status() {
    echo "install dir: ${APP_DIR}"
    systemctl is-active "${APP_NAME}.service" || true
    runtime_pkg_ok || return 1
    for app in "${RUNTIME_APPS[@]}"; do
        runtime_cmd "${app}" status || true
    done
}


echo "============================ ${APP_NAME} ============================"
echo "  1、install ${APP_NAME}"
echo "  2、update ${APP_NAME}"
echo "  3、uninstall ${APP_NAME}"
echo "  4、rollback ${APP_NAME} runtime"
echo "  5、status ${APP_NAME}"
echo "======================================================================"
read -p "$(echo -e "place choose [1-5]：")" choose
case $choose in
1)
    install && wait && clear_install_files
    ;;
2)
    updateBinary && wait && clear_install_files
    ;;
3)
    uninstall && wait && clear_install_files
    ;;
4)
    rollbackRuntime
    ;;
5)
    status
    ;;
*)
    echo "Input error, please try again!"
    ;;
esac

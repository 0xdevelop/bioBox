#!/bin/bash
# 在独立 prefix 用 uv 建 Python 3.14 venv，安装 PXDesign 及其依赖链。
# 可用场景：
#   1) 仓库内：./build.sh pxdesign 或本脚本（经 scripts/repo_src_defaults.sh 定位 submodule）
#   2) 发布包内：包已含 src/ + deps/ + patches/，只使用包内文件（仍需联网拉 pip 依赖与权重）
#
# 运行时需：LAYERNORM_TYPE=torch、TOOL_WEIGHTS_ROOT、PROTENIX_DATA_ROOT_DIR，
# 以及 pipeline 参数 --num_workers 0 --use_fast_ln False --load_checkpoint_dir <数据目录>/release_data/checkpoint
#
# 权重/数据根来自 runtime conf；唯一模型下载者为整合包官方服务二进制。
# 既有文件使用配置/目录软链接复用；不从开发 prefix 隐式复制。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck disable=SC1091
# Managed weights use the native controller; no shell copy/downloader is sourced.
if [[ -f "${ROOT}/scripts/repo_src_defaults.sh" ]]; then
  # shellcheck disable=SC1091
  source "${ROOT}/scripts/repo_src_defaults.sh"
fi

if [[ -n "${PREFIX_ROOT:-}" ]]; then
  PREFIX="${PXDESIGN_PREFIX:-${PREFIX_ROOT}/pxdesign}"
else
  PREFIX="${PXDESIGN_PREFIX:-${ROOT}/prefix/pxdesign}"
fi

PINNED_PROTENIX_REF="${PINNED_PROTENIX_REF:-d18aa1daadd02a001b32bc7fa2278fc8a2f8f025}"
PXDBENCH_REF="${PXDBENCH_REF:-f6d0d72496c23ca1374df5943754f3454ae8c549}"
COLABDESIGN_REF="${COLABDESIGN_REF:-8fd167defd1b0f98e29f00584ff4f3987e6935dc}"
OVERRIDES="${PXDESIGN_OVERRIDES:-${ROOT}/deps/pxdesign.txt}"
TORCH_INDEX_URL="${TORCH_INDEX_URL:-https://download.pytorch.org/whl/cu130}"
PYPI_INDEX_URL="${PYPI_INDEX_URL:-https://pypi.org/simple}"
export CUDA_HOME="${CUDA_HOME:-/usr/local/cuda-13.0}"
VENV="${PREFIX}/venv"
LOG_DIR="${PREFIX}/logs"
TAG="[install_prefix_pxdesign]"

die() { echo "${TAG} FAIL: $*" >&2; exit 1; }

resolve_src() {
  if [[ -n "${PXDESIGN_SRC:-}" ]]; then
    SRC="${PXDESIGN_SRC}"
    SRC_MODE="${PXDESIGN_SRC_MODE:-copy}"
    return 0
  fi
  if [[ -f "${ROOT}/src/SRC_SHA" && -f "${ROOT}/src/pxdesign/setup.py" ]]; then
    SRC="${ROOT}/src/pxdesign"
    SRC_MODE=copy
    SRC_SHA_FILE="${ROOT}/src/SRC_SHA"
    return 0
  fi
  if [[ -n "${REPO_PXDESIGN_SRC:-}" && -f "${REPO_PXDESIGN_SRC}/setup.py" ]]; then
    SRC="${REPO_PXDESIGN_SRC}"
    SRC_MODE=git
    return 0
  fi
  die "no PXDesign source (set PXDESIGN_SRC, or use a release package with src/pxdesign)"
}

resolve_src_sha() {
  if [[ -n "${SRC_SHA:-}" ]]; then
    return 0
  fi
  if [[ -n "${SRC_SHA_FILE:-}" ]]; then
    SRC_SHA="$(tr -d '[:space:]' < "${SRC_SHA_FILE}")"
    [[ -n "${SRC_SHA}" ]] || die "empty ${SRC_SHA_FILE}"
    return 0
  fi
  if [[ "${SRC_MODE}" == git ]]; then
    SRC_SHA="$(git -C "${SRC}" rev-parse HEAD)"
    return 0
  fi
  die "cannot determine source SHA for ${SRC}"
}

stage_upstream_src() {
  if [[ -f "${STAGE}/setup.py" ]]; then
    return 0
  fi
  rm -rf "${STAGE}.partial"
  mkdir -p "${STAGE}.partial"
  case "${SRC_MODE}" in
    git)
      # 显式 Git 来源只导出指定目录对应的树，绝不导出包含它的整个仓库。
      local relative tree git_root
      git_root="$(git -C "${SRC}" rev-parse --show-toplevel)"
      relative="$(git -C "${SRC}" rev-parse --show-prefix)"
      tree="${SRC_SHA}${relative:+:${relative%/}}"
      git -C "${git_root}" archive --format=tar "${tree}" | tar -x -C "${STAGE}.partial"
      ;;
    copy)
      # 包内快照始终复制；所在目录是不是 Git 仓库不改变行为。
      tar -C "${SRC}" --exclude=./.git -cf - . | tar -x -C "${STAGE}.partial"
      ;;
    *) die "invalid PXDESIGN_SRC_MODE: ${SRC_MODE} (expected copy or git)" ;;
  esac
  [[ -f "${STAGE}.partial/setup.py" ]] || die "staged source is missing setup.py"
  mv "${STAGE}.partial" "${STAGE}"
}

run_step() {
  local step="$1"; shift
  local log="${LOG_DIR}/install_${step}.log"
  echo "${TAG} step=${step}: $*"
  local rc=0
  "$@" >"${log}" 2>&1 || rc=$?
  if [[ ${rc} -ne 0 ]]; then
    echo "${TAG} FAIL step=${step} rc=${rc} log=${log}" >&2
    grep -E "Failed to (build|download|prepare)|No solution found|Because |error:|Error" "${log}" | head -20 >&2 || true
    echo "${TAG} ---- last 40 lines of ${log} ----" >&2
    tail -n 40 "${log}" >&2
    exit 1
  fi
}

UV_BIN="${UV:-}"
if [[ -z "${UV_BIN}" ]]; then
  if command -v uv >/dev/null 2>&1; then UV_BIN="$(command -v uv)"
  elif [[ -x "${HOME}/.local/bin/uv" ]]; then UV_BIN="${HOME}/.local/bin/uv"
  else die "uv not found"; fi
fi
[[ -f "${OVERRIDES}" ]] || die "override file missing: ${OVERRIDES}"
resolve_src
case "${SRC_MODE}" in copy|git) ;; *) die "invalid PXDESIGN_SRC_MODE: ${SRC_MODE} (expected copy or git)" ;; esac
[[ -f "${SRC}/setup.py" ]] || die "PXDesign setup.py missing under ${SRC}"
[[ -x "${CUDA_HOME}/bin/nvcc" ]] || die "nvcc not found under CUDA_HOME=${CUDA_HOME}"

DATA="${PXDESIGN_DATA:-${PREFIX}}"
if [[ -n "${WEIGHTS_REUSE_FROM:-}" ]]; then die "WEIGHTS_REUSE_FROM is retired for managed preparation; configure existing paths or a directory symlink (no copy)"; fi
if [[ "${SKIP_WEIGHTS:-0}" != 1 ]]; then
  NATIVE="${ROOT}/../../protenix_service"
  [[ -x "${NATIVE}" ]] || die "integrated native resource coordinator missing; no weight download attempted"
  "${NATIVE}" resource prepare pxdesign.preview || die "resources not ready/confirmed; no Python CLI started"
fi
ENV_FILE="${PXDESIGN_ENV_FILE:-${PREFIX}/conf/pxdesign.env}"
if [[ -f "${ENV_FILE}" ]]; then
  # Resource roots come from runtime config, not cwd or the build-stage prefix.
  source "${ENV_FILE}"
  DATA="${PXDESIGN_DATA:-${DATA}}"
  if [[ "${DATA}" != /* ]]; then DATA="$(cd "$(dirname "${ENV_FILE}")/.." && pwd)/${DATA}"; fi
fi

mkdir -p "${PREFIX}"/{logs,conf,src} "${DATA}"/{release_data/checkpoint,tool_weights,protenix_data}
resolve_src_sha
echo "${TAG} prefix=${PREFIX} src=${SRC}@${SRC_SHA:0:12} protenix=${PINNED_PROTENIX_REF} pxdbench=${PXDBENCH_REF}"

if [[ -x "${VENV}/bin/python" ]]; then
  v="$("${VENV}/bin/python" -c 'import sys;print("%d.%d"%sys.version_info[:2])')"
  if [[ "${v}" != "3.14" ]]; then
    mv "${VENV}" "${VENV}.py${v/./}.bak.$(date +%Y%m%d%H%M%S)"
    echo "${TAG} moved old python ${v} venv aside"
  fi
fi
[[ -x "${VENV}/bin/python" ]] || run_step venv "${UV_BIN}" venv --python 3.14 --seed "${VENV}"
export VIRTUAL_ENV="${VENV}"

run_step torch "${UV_BIN}" pip install --python "${VENV}/bin/python" \
  --index-url "${PYPI_INDEX_URL}" --extra-index-url "${TORCH_INDEX_URL}" --index-strategy unsafe-best-match \
  "torch==2.14.1+cu130"
run_step build_tools "${UV_BIN}" pip install --python "${VENV}/bin/python" --index-url "${PYPI_INDEX_URL}" \
  setuptools wheel ninja packaging cython "numpy>=2.0,<2.5"

STAGE="${PREFIX}/src/pxdesign-${SRC_SHA:0:12}"
stage_upstream_src

STAGE_PATCH="${ROOT}/patches/pxdesign-argparse-percent.patch"
[[ -f "${STAGE_PATCH}" ]] || die "patch missing: ${STAGE_PATCH}"
if patch -p1 -d "${STAGE}" -R -s -f --dry-run -i "${STAGE_PATCH}" >/dev/null 2>&1; then
  echo "${TAG} pxdesign stage patch already applied"
else
  patch -p1 -d "${STAGE}" -N -f -i "${STAGE_PATCH}" || die "apply ${STAGE_PATCH} failed"
fi

REQS=(
  -r "${STAGE}/requirements.txt"
  "protenix @ git+https://github.com/bytedance/Protenix.git@${PINNED_PROTENIX_REF}"
  "pxdbench @ git+https://github.com/bytedance/PXDesignBench.git@${PXDBENCH_REF}"
  "colabdesign @ git+https://github.com/sokrypton/ColabDesign.git@${COLABDESIGN_REF}"
  "jax[cuda13]"
  einops natsort dm-tree posix_ipc
  -e "${STAGE}"
)
run_step deps env DS_BUILD_OPS=0 "${UV_BIN}" pip install --python "${VENV}/bin/python" \
  --override "${OVERRIDES}" \
  --index-url "${PYPI_INDEX_URL}" --extra-index-url "${TORCH_INDEX_URL}" --index-strategy unsafe-best-match \
  --no-build-isolation-package deepspeed \
  "${REQS[@]}"

PATCH_FILE="${ROOT}/patches/pxdbench-json-float.patch"
[[ -f "${PATCH_FILE}" ]] || die "patch missing: ${PATCH_FILE}"
SITE_PKGS="$("${VENV}/bin/python" -c 'import sysconfig;print(sysconfig.get_paths()["purelib"])')"
if patch -p1 -d "${SITE_PKGS}" -R -s -f --dry-run -i "${PATCH_FILE}" >/dev/null 2>&1; then
  echo "${TAG} pxdbench patch already applied"
else
  run_step patch_pxdbench patch -p1 -d "${SITE_PKGS}" -N -f -i "${PATCH_FILE}"
fi

export PATH="${VENV}/bin:${CUDA_HOME}/bin:${PATH}"
export LAYERNORM_TYPE="${LAYERNORM_TYPE:-torch}"
run_step verify_import "${VENV}/bin/python" -c '
import torch, jax, jax.numpy as jnp, colabdesign, protenix, pxdbench, pxdesign
assert torch.cuda.is_available(), "torch.cuda.is_available() is False"
print("torch", torch.__version__, torch.version.cuda, torch.cuda.get_device_name(0))
print("jax", jax.__version__, jax.devices())
assert any(d.platform == "gpu" for d in jax.devices()), "jax sees no GPU"
print("jax matmul", float((jnp.ones((256,256)) @ jnp.ones((256,256))).sum()))
'
cat "${LOG_DIR}/install_verify_import.log"
[[ -x "${VENV}/bin/pxdesign" ]] || die "pxdesign console_script missing"
run_step verify_cli "${VENV}/bin/pxdesign" pipeline --help
"${UV_BIN}" pip freeze --python "${VENV}/bin/python" > "${PREFIX}/conf/pip-freeze.txt"


if [[ "${SKIP_WEIGHTS:-0}" == 1 ]]; then echo "${TAG} status=environment_only src_sha=${SRC_SHA}"; else echo "${TAG} status=ok resources_verified=1 src_sha=${SRC_SHA}"; fi

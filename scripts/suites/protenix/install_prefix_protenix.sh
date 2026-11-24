#!/bin/bash
# 在独立 prefix 用 uv 建 Python 3.14 venv，装 torch(cu130) + Protenix。
# 可用场景：
#   1) 仓库内：./build.sh protenix 或本脚本（经 scripts/repo_src_defaults.sh 定位 submodule）
#   2) 发布包内：包已含 src/ + deps/，只使用包内文件（仍需联网拉 pip 依赖与公开权重）
#
# 可选环境变量：
#   PREFIX_ROOT / PROTENIX_PREFIX / PROTENIX_SRC / PROTENIX_OVERRIDES / PROTENIX_PYTHON
#   UV / CUDA_HOME / TORCH_INDEX_URL / PYPI_INDEX_URL / RECREATE_VENV / LAYERNORM_TYPE
#   PROTENIX_MODEL（默认 v1；v2 需本地授权权重）/ SKIP_WEIGHTS
#   PROTENIX_ENV_FILE（默认 <prefix>/conf/protenix.env；install_protenix.sh 指向安装目录的共用配置）
set -euo pipefail

_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${_SCRIPT_DIR}/../../repo_src_defaults.sh" ]]; then
  ROOT="$(cd "${_SCRIPT_DIR}/../../.." && pwd)"
  SCRIPTS_DIR="${ROOT}/scripts"
else
  ROOT="$(cd "${_SCRIPT_DIR}/.." && pwd)"
  SCRIPTS_DIR="${ROOT}/scripts"
fi
# shellcheck disable=SC1091
# Managed weights use the native controller; no shell copy/downloader is sourced.
if [[ -f "${SCRIPTS_DIR}/repo_src_defaults.sh" ]]; then
  # shellcheck disable=SC1091
  source "${SCRIPTS_DIR}/repo_src_defaults.sh"
fi

if [[ -n "${PREFIX_ROOT:-}" ]]; then
  PREFIX="${PROTENIX_PREFIX:-${PREFIX_ROOT}/protenix}"
else
  PREFIX="${PROTENIX_PREFIX:-${ROOT}/bb_prefix/protenix}"
fi

PY_REQ="${PROTENIX_PYTHON:-3.14}"
if [[ -n "${PROTENIX_OVERRIDES:-}" ]]; then
  OVERRIDES="${PROTENIX_OVERRIDES}"
elif [[ -f "${ROOT}/bb_suites/protenix/deps.txt" ]]; then
  OVERRIDES="${ROOT}/bb_suites/protenix/deps.txt"
else
  OVERRIDES="${ROOT}/deps/protenix.txt"
fi
TORCH_INDEX_URL="${TORCH_INDEX_URL:-https://download.pytorch.org/whl/cu130}"
PYPI_INDEX_URL="${PYPI_INDEX_URL:-https://pypi.org/simple}"
export CUDA_HOME="${CUDA_HOME:-/usr/local/cuda-13.0}"
VENV="${PREFIX}/venv"
LOG_DIR="${PREFIX}/logs"
TAG="[install_prefix_protenix]"

TORCH_SPEC=(torch==2.14.1+cu130 torchvision==0.29.1+cu130)
EXTRA_REQS=(cuequivariance-ops-torch-cu13)
NO_BUILD_ISOLATION_PKGS=(deepspeed)

die() { echo "${TAG} FAIL: $*" >&2; exit 1; }

resolve_src() {
  if [[ -n "${PROTENIX_SRC:-}" ]]; then
    SRC="${PROTENIX_SRC}"
    SRC_MODE="${PROTENIX_SRC_MODE:-copy}"
    return 0
  fi
  if [[ -f "${ROOT}/src/SRC_SHA" && -f "${ROOT}/src/protenix/setup.py" ]]; then
    SRC="${ROOT}/src/protenix"
    SRC_MODE=copy
    SRC_SHA_FILE="${ROOT}/src/SRC_SHA"
    return 0
  fi
  if [[ -n "${REPO_PROTENIX_SRC:-}" && -f "${REPO_PROTENIX_SRC}/setup.py" ]]; then
    SRC="${REPO_PROTENIX_SRC}"
    SRC_MODE=git
    return 0
  fi
  die "no Protenix source (set PROTENIX_SRC, or use a release package with src/protenix)"
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
    *) die "invalid PROTENIX_SRC_MODE: ${SRC_MODE} (expected copy or git)" ;;
  esac
  [[ -f "${STAGE}.partial/setup.py" ]] || die "staged source is missing setup.py"
  mv "${STAGE}.partial" "${STAGE}"
}

UV_BIN="${UV:-}"
if [[ -z "${UV_BIN}" ]]; then
  if command -v uv >/dev/null 2>&1; then UV_BIN="$(command -v uv)"
  elif [[ -x "${HOME}/.local/bin/uv" ]]; then UV_BIN="${HOME}/.local/bin/uv"
  else die "uv not found (install: curl -LsSf https://astral.sh/uv/install.sh | sh)"; fi
fi

[[ -f "${OVERRIDES}" ]] || die "override file missing: ${OVERRIDES}"
resolve_src
case "${SRC_MODE}" in copy|git) ;; *) die "invalid PROTENIX_SRC_MODE: ${SRC_MODE} (expected copy or git)" ;; esac
[[ -f "${SRC}/setup.py" ]] || die "Protenix setup.py missing under ${SRC}"
[[ -x "${CUDA_HOME}/bin/nvcc" ]] || die "nvcc not found under CUDA_HOME=${CUDA_HOME} (CUDA 13 toolkit required)"
command -v nvidia-smi >/dev/null 2>&1 || die "nvidia-smi not found (NVIDIA driver required)"

mkdir -p "${PREFIX}"/{logs,conf,src}
resolve_src_sha
echo "${TAG} prefix=${PREFIX}"
echo "${TAG} src=${SRC} @ ${SRC_SHA}"
echo "${TAG} uv=${UV_BIN} ($("${UV_BIN}" --version))"
echo "${TAG} cuda=$("${CUDA_HOME}/bin/nvcc" --version | grep -o 'release [0-9.]*')  torch_index=${TORCH_INDEX_URL} pypi_index=${PYPI_INDEX_URL}"

ENV_FILE="${PROTENIX_ENV_FILE:-${PREFIX}/conf/protenix.env}"
if [[ ! -f "${ENV_FILE}" ]]; then
  if [[ -f "${ROOT}/bb_suites/protenix/protenix.env" ]]; then
    ENV_TMPL="${ROOT}/bb_suites/protenix/protenix.env"
  elif [[ -f "${ROOT}/conf/protenix.env" ]]; then
    ENV_TMPL="${ROOT}/conf/protenix.env"
  else
    die "env template missing (expected example_files/protenix.env or conf/protenix.env)"
  fi
  mkdir -p "$(dirname "${ENV_FILE}")"
  sed "s|@PREFIX@|${PREFIX}|g" "${ENV_TMPL}" > "${ENV_FILE}"
  echo "${TAG} wrote ${ENV_FILE}"
fi
if [[ -z "${PROTENIX_MODEL:-}" ]]; then
  PROTENIX_MODEL="$(sed -n 's/^PROTENIX_MODEL=//p' "${ENV_FILE}" | tail -n1 | tr -d '"')"
fi
PROTENIX_MODEL="${PROTENIX_MODEL:-protenix_base_default_v1.0.0}"
DATA_ROOT="$(sed -n 's/^PROTENIX_ROOT_DIR=//p' "${ENV_FILE}" | tail -n1 | tr -d '"')"
DATA_ROOT="${DATA_ROOT:-${PREFIX}/data}"
if [[ "${DATA_ROOT}" != /* ]]; then DATA_ROOT="$(cd "$(dirname "${ENV_FILE}")/.." && pwd)/${DATA_ROOT}"; fi
CKPT="${DATA_ROOT}/checkpoint/${PROTENIX_MODEL}.pt"
echo "${TAG} PROTENIX_MODEL=${PROTENIX_MODEL} PROTENIX_ROOT_DIR=${DATA_ROOT}"

# The product binary is the only weight transport/lock owner. No curl fallback.
if [[ -n "${WEIGHTS_REUSE_FROM:-}" ]]; then die "WEIGHTS_REUSE_FROM is retired for managed preparation; configure existing paths or a directory symlink (no copy)"; fi
if [[ "${SKIP_WEIGHTS:-0}" != 1 ]]; then
  NATIVE="${ROOT}/../../bb_service"
  [[ -x "${NATIVE}" ]] || die "integrated native resource coordinator missing; no weight download attempted"
  "${NATIVE}" resource prepare protenix.v1 || die "resources not ready/confirmed; no Python CLI started"
fi

READY_MARK="${PREFIX}/conf/install.ready"
STAGE="${PREFIX}/src/protenix-${SRC_SHA:0:12}"
if [[ "${SKIP_WEIGHTS:-0}" != "1" && "${RECREATE_VENV:-0}" != "1" && -x "${VENV}/bin/protenix" && -x "${VENV}/bin/python" && -s "${CKPT}" && -f "${STAGE}/setup.py" ]]; then
  PYV="$("${VENV}/bin/python" -c 'import sys;print("%d.%d.%d"%sys.version_info[:3])')"
  if [[ "${PYV}" == 3.14.* ]]; then
    {
      echo "src_sha=${SRC_SHA}"
      echo "model=${PROTENIX_MODEL}"
      echo "python=${PYV}"
    } > "${READY_MARK}"
    echo "${TAG} skip: already ready python=${PYV} src_sha=${SRC_SHA:0:12} model=${PROTENIX_MODEL}"
    echo "${TAG} status=ok src_sha=${SRC_SHA} model=${PROTENIX_MODEL} skipped=1"
    exit 0
  fi
fi

if [[ "${RECREATE_VENV:-0}" == "1" && -d "${VENV}" ]]; then
  rm -rf "${VENV}"
  rm -f "${READY_MARK}"
fi
if [[ ! -x "${VENV}/bin/python" ]]; then
  run_step venv "${UV_BIN}" venv --python "${PY_REQ}" --seed "${VENV}"
fi
PYV="$("${VENV}/bin/python" -c 'import sys;print("%d.%d.%d"%sys.version_info[:3])')"
[[ "${PYV}" == 3.14.* ]] || die "venv python is ${PYV}, hard constraint requires 3.14 (RECREATE_VENV=1 to rebuild)"
echo "${TAG} venv=${VENV} python=${PYV}"
export VIRTUAL_ENV="${VENV}"

run_step torch "${UV_BIN}" pip install --python "${VENV}/bin/python" \
  --index-url "${PYPI_INDEX_URL}" --extra-index-url "${TORCH_INDEX_URL}" --index-strategy unsafe-best-match \
  "${TORCH_SPEC[@]}"

run_step build_tools "${UV_BIN}" pip install --python "${VENV}/bin/python" --index-url "${PYPI_INDEX_URL}" \
  setuptools wheel ninja packaging cython "numpy>=2.4.1,<2.5"

stage_upstream_src

NBI_ARGS=()
for p in "${NO_BUILD_ISOLATION_PKGS[@]}"; do NBI_ARGS+=(--no-build-isolation-package "${p}"); done
run_step protenix env DS_BUILD_OPS=0 "${UV_BIN}" pip install --python "${VENV}/bin/python" \
  --override "${OVERRIDES}" \
  --index-url "${PYPI_INDEX_URL}" --extra-index-url "${TORCH_INDEX_URL}" --index-strategy unsafe-best-match \
  "${NBI_ARGS[@]}" \
  "${STAGE}" "${EXTRA_REQS[@]}"

[[ -x "${VENV}/bin/protenix" ]] || die "protenix console_script missing after install"
export PATH="${VENV}/bin:${CUDA_HOME}/bin:${PATH}"
export LAYERNORM_TYPE="${LAYERNORM_TYPE:-torch}"
echo "${TAG} LAYERNORM_TYPE=${LAYERNORM_TYPE}"
run_step verify_import "${VENV}/bin/python" -c '
import torch, protenix, runner.batch_inference, cuequivariance_torch, cuequivariance_ops_torch, biotite, rdkit, gemmi
assert torch.version.cuda and torch.version.cuda.startswith("13"), torch.version.cuda
assert torch.cuda.is_available(), "torch.cuda.is_available() is False"
x = torch.randn(1024, 1024, device="cuda"); y = (x @ x).sum().item()
print("torch", torch.__version__, "cuda", torch.version.cuda, "device", torch.cuda.get_device_name(0), "cap", torch.cuda.get_device_capability(0))
print("biotite", biotite.__version__, "rdkit", rdkit.__version__, "cuequivariance_ops_torch ok")
'
cat "${LOG_DIR}/install_verify_import.log"
run_step verify_cli "${VENV}/bin/protenix" --help
"${UV_BIN}" pip freeze --python "${VENV}/bin/python" > "${PREFIX}/conf/pip-freeze.txt"
echo "${TAG} freeze -> ${PREFIX}/conf/pip-freeze.txt"


# Preserve the existing completion-marker contract only for fully prepared resources.
if [[ "${SKIP_WEIGHTS:-0}" == 1 ]]; then
  echo "${TAG} status=environment_only src_sha=${SRC_SHA} model=${PROTENIX_MODEL}"
else
  {
    echo "src_sha=${SRC_SHA}"
    echo "model=${PROTENIX_MODEL}"
    echo "python=${PYV}"
  } > "${READY_MARK}"
  echo "${TAG} status=ok src_sha=${SRC_SHA} model=${PROTENIX_MODEL}"
fi

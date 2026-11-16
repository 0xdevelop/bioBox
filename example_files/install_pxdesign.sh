#!/bin/bash
# pxdesign 安装 / 升级 / 回退 / 卸载 / 状态。在发布包解压目录里执行：
#   ./install_pxdesign.sh install|update|rollback|uninstall [--purge]|status
# 安装目录固定为 /protenix_service/runtime/pxdesign（与 protenix_service 同一套服务，需 root）；
# 正式安装由 install_protenix_service.sh 菜单调用本脚本，一般不必单独执行。
set -euo pipefail
APP=pxdesign
CLI=pxdesign
PKG="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
source "${PKG}/scripts/install_common.sh"
main "$@"

# bioBox

把 [Protenix](https://github.com/bytedance/Protenix)（结构预测）和 [PXDesign](https://github.com/bytedance/PXDesign)（蛋白设计）打成可在 NVIDIA GPU 机器上原生安装的命令行工具，并整合为对外提供队列与多协议接口的 Go 服务 `bioBox`（Go module：`github.com/0xdevelop/bioBox`；gRPC wire package 兼容保持 `protenix_service.api.v1`；JWT 默认 `issuer`/`audience` 兼容保持 `protenix_service`；API key 前缀：`ptxk_`）。不用 Docker、conda。

两种用法互不干扰：

- **独立使用**：单产品包，`install_<app>.sh` 装到 `~/.local/share/<app>`（`<APP>_PREFIX` 可改），不需要 sudo。
- **与 Go 服务一起安装**：整合包，root 跑 `install_bioBox.sh`，按 Go 模板 install 体系把服务和两个运行时统一装到 `/bioBox`（运行时目录由脚本显式传入，不读独立模式的默认值）。

## 仓库目录结构

- 根目录为 `bioBox` Go 服务源码（`main.go`、`resource_cmd.go`、`go.mod`、`bb_ability/`、`bb_api/`、`bb_cli_proc/`、`bb_common/`、`bb_config/`、`bb_custom_cmd/`、`bb_docs/`、`bb_frontend/`、`bb_policy/`、`bb_runtime_resource/`、`bb_storage/`、`bb_test_ui/`、`example_files/`）。
- `bb_build/`：构建与预构建配置模板及双架构清单（`pre_build_config_template.yaml`、`gen_pre_build_config_linux_{amd64,arm64}.yaml`）。
- `build_srcs/`：上游源码 git submodule（`protenix`、`pxdesign`），不修改内部源码。
- `bb_suites/`：各计算套件依赖覆盖、包装入口与运行期补丁（`protenix/`、`pxdesign/`）。
- `scripts/`：构建、发版、代码/文档生成、套件前缀安装与测试脚本（`build.sh`、`git_tag.sh`、`gen_*.sh`、`suites/`、`tests/`）。
- `docs/`：按归属分为 `docs/bb_service/`（服务与 API）、`docs/bb_build/`（构建、发版与验收记录）、`docs/bb_upstream/`（上游资源与单包说明）。

## 交付物

公开 Release 仅三个附件：双架构整合包 + 一份 `SHA256SUMS`（只覆盖两包）。独立包仍内部构建、测试和保留安装脚本，不再作为发布附件。详见[交付与资源配置](docs/bb_build/release_delivery.md)。

内部每个 arch 仍构建三个 zip：

- `protenix_linux_{amd64,arm64}_release_v*.zip`、`pxdesign_linux_{amd64,arm64}_release_v*.zip`：独立单产品包
- `bioBox_linux_{amd64,arm64}_release_v*.zip`：整合包（`runtime/<app>/` 与单产品包内容相同）

## 一、独立使用（单产品包）

解压后：

```text
install_<app>.sh        安装/升级/回退/卸载/状态
README.md               使用说明
bin/<app>-run           安装后复制到安装目录的命令入口
conf/<app>.env          运行配置模板
scripts/                安装脚本（install_prefix_*、install_common）
deps/ patches/          依赖版本覆盖与运行期补丁
src/<app>/ src/SRC_SHA  上游源码快照（不依赖本仓库）
examples/               示例输入
BUILD_INFO.txt          版本、提交、源码 sha
```

前置条件：NVIDIA 驱动 + CUDA 13 工具链（`nvidia-smi`、`nvcc`，默认 `CUDA_HOME=/usr/local/cuda-13.0`）；[uv](https://github.com/astral-sh/uv)，能解析 Python 3.14；公网访问（pip 依赖、模型权重）。

```bash
unzip protenix_linux_arm64_release_v*.zip && cd protenix
./install_protenix.sh install
# PXDesign 同理
unzip pxdesign_linux_arm64_release_v*.zip && cd pxdesign
./install_pxdesign.sh install
```

| 命令 | 行为 |
| --- | --- |
| `install` | 首次安装到 `$<APP>_PREFIX`（默认 `~/.local/share/<app>`） |
| `update` | 用当前包升级；新版本构建失败不影响原版本，配置和权重保留 |
| `rollback` | 回到上一版本 |
| `uninstall` / `uninstall --purge` | 卸载程序保留配置和权重 / 连同配置和权重一起删除 |
| `status` | 版本、提交、是否可运行 |

换安装目录：`PROTENIX_PREFIX=/dir`（或 `PXDESIGN_PREFIX=/dir`）。既有资源只改 `runtime/<app>/conf/<app>.env` 根 value 或使用既有目录软链接；不复制，不再支持 `WEIGHTS_REUSE_FROM`。独立单产品包内的包装器会检查相邻协调器 `${PREFIX}/../../bioBox`，缺失时以 **exit code 4** 明确拒绝（独立安装闭环尚未验收）；环境-only 构建不等于安装/推理已验收。见[交付与资源配置](docs/bb_build/release_delivery.md)。

安装后目录：`bin/<app>-run`、`conf/<app>.env`、`data/`（权重）、`releases/<版本>/`（每版一套 venv，`current`/`previous` 符号链接指向）。

```bash
~/.local/share/protenix/bin/protenix-run pred -i input.json -o out --use_msa false -s 101
~/.local/share/pxdesign/bin/pxdesign-run pipeline --preset preview -i task.yaml -o out --N_sample 2
```

成功条件：退出码 0、输出目录有 `.cif`、无 `ERR/`、GPU 有占用。详细步骤（切模型、FAQ）见各包内 `README.md`。

## 二、与 Go 服务一起安装（整合包）

解压后：

```text
bioBox/
  bioBox                           服务二进制
  bioBox.service                   systemd 单元（User=root，WorkingDirectory=/bioBox）
  install_bioBox.sh                安装 / 升级 / 卸载 / 运行时回退 / 状态（菜单）
  conf/config.yaml                 服务配置（cli_cfg 指向 /bioBox/runtime/...）
  BUILD_INFO.txt                   版本、提交
  runtime/protenix/  runtime/pxdesign/   与单产品包内容相同
```

### 前置条件

- root 权限（安装到 `/bioBox`，单元放到 `/etc/systemd/system`）
- NVIDIA 驱动 + CUDA 13 工具链（`nvidia-smi`、`nvcc`，默认 `CUDA_HOME=/usr/local/cuda-13.0`）
- [uv](https://github.com/astral-sh/uv)，能解析 Python 3.14。root 的 `PATH` 里没有 uv 时用 `UV=/path/to/uv` 指定
- 公网访问用于 uv/pip 创建 Python 环境。权重默认使用明确本地内容 pin 且下载关闭；可显式确认选择 14 项上游 URL 下载，PXDesign 仍需既有 AF2 或补齐整包可信 pin，见[交付与资源配置](docs/bb_build/release_delivery.md)

### 安装

```bash
unzip bioBox_linux_arm64_release_v*.zip
cd bioBox
# 以 root 执行（用 sudo 时写成 sudo env UV=/path/to/uv ./install_bioBox.sh）
UV=/path/to/uv ./install_bioBox.sh
# 选 1
```

菜单：

| 选项 | 行为 |
| --- | --- |
| 1 install | 安装或重装。先装/升级两个运行时（失败则不动二进制与配置，并恢复本次已改动的运行时）；再把旧二进制、`conf/config.yaml`、`runtime/<app>/conf/<app>.env` 备份为 `.bak_<时间戳>` 后覆盖；复制单元到 `/etc/systemd/system`，`enable` + `start` |
| 2 update | 升级：停服务，升级两个运行时（保留 `<app>.env` 与权重）。任一运行时失败时回退本次已切换的 `current`；只有完整回退成功才启动原服务。二进制启动失败时同样恢复上一版二进制/unit 和运行时。不把混合 runtime 当成成功回退。只换二进制（旧的留 `.bak_<时间戳>`），不动 `conf/config.yaml` |
| 3 uninstall | 整体卸载：`disable --now` 并删除单元，删除整个 `/bioBox`（含运行时的 Python 环境与权重） |
| 4 rollback runtime | 两个运行时各自切回上一版本（`previous`），前后停启服务；服务二进制回退用 `.bak_*` |
| 5 status | 服务状态与两个运行时的版本、是否可运行 |

选项 1–3 完成后按模板约定删除解压目录和 zip；4、5 不删。既有资源通过 runtime 的 `.env` 根 value 选择，默认启用的清单只证明与已授权本地基准内容相等，不是官方权重认证；缺失不会联网。支持的业务输入必须显式选择 v1/no-MSA 或 PXDesign preview/no-MSA。

### 安装后的目录

| 路径 | 内容 |
| --- | --- |
| `/bioBox/bioBox` | 服务二进制；旧版本 `bioBox.bak_<时间戳>` |
| `/bioBox/conf/config.yaml` | 服务配置；重装时旧配置 `config.yaml.bak_<时间戳>` |
| `/bioBox/data/` | SQLite（`bioBox.db`）、作业目录 `cli_jobs/<task_id>/` |
| `/bioBox/logs/` | 服务日志 |
| `/bioBox/runtime/protenix/` | `bin/protenix-run`、`conf/protenix.env`、`data/`（权重与 CCD）、`releases/<版本>/venv`、`current`、`previous` |
| `/bioBox/runtime/pxdesign/` | `bin/pxdesign-run`、`conf/pxdesign.env`、`data/`（权重、AF2/MPNN）、`releases/<版本>/venv`、`current`、`previous` |
| `/bioBox/runtime/uv/` | uv 托管的 Python 解释器（venv 依赖它）与 uv 缓存 |
| `/etc/systemd/system/bioBox.service` | systemd 单元 |

服务通过 `conf/config.yaml` 的 `cli_cfg` 调用 `/bioBox/runtime/<app>/bin/<app>-run`。`protenix` 的 v2 需要授权权重，`cli_cfg.protenix.version_runtimes.v2.binary` 默认为空（未配置时直接拒绝，不回退 v1）。

## 本地构建与开发调试

```bash
git clone git@github.com:0xdevelop/bioBox.git && cd bioBox
git submodule update --init --recursive
./scripts/gen_pre_build_config.sh all
FORCE_PACK=1 ./scripts/build.sh pack        # 双 arch → build/upload_tmp_dir/ 下的单产品包与整合包
./scripts/build.sh pack linux_arm64         # 只打一个 arch
```

- 需要 Go（`go.mod` 指定版本）、bun（现场构建前端与 test_ui）、zip。服务二进制用 cgo（SQLite），打非本机 arch 时需交叉编译器（默认 `x86_64-linux-gnu-gcc` / `aarch64-linux-gnu-gcc`，可用 `CC_linux_amd64` / `CC_linux_arm64` 指定）。
- 本地开发运行：示例配置在 `example_files/config_example.yaml`（可复制为被 gitignore 的 `example_files/config_local.yaml`），默认端口：JSON-RPC `13001`、MCP `13002`、test_ui `13003`、WebSocket `13004`、gRPC `13005`、frontend `13006`。`./scripts/gen_frontend.sh && ./scripts/gen_test_ui.sh && go run .`（或 `go build -o bb_service .`）。
- 发布包交付前先提交、再 `FORCE_PACK=1`，让 `BUILD_INFO.txt` 的 commit 对齐 HEAD。
- 发版入口是无参 `./scripts/git_tag.sh`（模板三段：`release` → `Build` → `RemoveOldRelease`）。Actions 在该结构内用 `ubuntu-24.04`（amd64）和 `ubuntu-24.04-arm`（arm64）做原生构建与六包内部完整性校验，最终仅发布两整合包 + `SHA256SUMS`；`scripts/build.sh` 只负责构建。见[模板审计与恢复说明](docs/bb_build/release_template_audit.md)及[原生双架构说明](docs/bb_build/native_release.md)。

`./scripts/build.sh` 子命令：`pack` 只打发布包（不跑 GPU/pip）；`protenix` / `pxdesign` 只在仓库内 `bb_prefix/` 装/校验开发用运行时（uv + Python 3.14 + torch cu130，GPU 验证用，不是正式安装）；`all`（默认）先打包再装两个 prefix。幂等：已就绪的步骤跳过；`FORCE_PACK=1` / `FORCE_PROTENIX=1` / `FORCE_PXDESIGN=1` 强制重做。

## 任务查询与只读步进

服务支持运行中查看当前业务阶段、只读回看历史步骤，并把业务状态数组与引擎调度诊断分开。页面启用条件、API 查询、关联排障及取消/恢复边界见[任务操作说明](docs/bb_service/task_operations.md)；release 不加载调试页。

## 当前套件与版本机制边界

- 当前仅包含两套硬编码套件：`protenix`（默认 `v1`、`use_msa=false`；`cli_cfg.protenix.version_runtimes.v2.binary` 默认为空并拒绝未配置启动）与 `pxdesign`（固定 `PINNED_PROTENIX_REF=d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`、`preview`、`use_msa=false`）。
- 暂未建立任意多套件或同名多版本目录轴；扩充新套件需同步更新 `bb_build/pre_build_config_template.yaml`、`bb_suites/<suite>/`、`scripts/suites/<suite>/`、`scripts/build.sh`、`example_files/install_bioBox.sh` 与 `scripts/install_common.sh`。

## 两套 Python 环境不可合并

`protenix` 用最新上游；`pxdesign` 链依赖官方钉死的 Protenix v0.5.0+pxd。两者始终是独立 venv：独立模式各自安装；整合模式在 `/bioBox/runtime/` 下由同一个 install 流程安装、升级、卸载。

更多实现口径见 [系统设计](docs/bb_service/design.md)；机器验收记录见 `docs/bb_build/acceptance_*.md`。

## 验证码日志与可选邮件

验证码实际生成后无条件写服务 INFO，邮件为独立可选能力，失败如实返回。配置、迁移、日志权限与排障边界见[操作说明](docs/bb_service/verification_delivery.md)。

## 当前发版基线与安装实测

- **已发布基线（重构前）**：`v0.0.2` 与 `v0.0.3` / `latest`（commit `37fb324`）按重构前命名（`protenix_service`）已在 GitHub 发布；`main` 当前提交 `2473e4e`（`Update LICENSE`，BSD→MIT）。历史 `v0.0.2` 过程、命令与未覆盖项见[v0.0.2 发版与正式安装实测记录](docs/bb_build/acceptance_release_v0.0.2_20261120.md)，此前矩阵记录见[验收矩阵与证据](docs/bb_build/acceptance_product_matrix_20261119.md)。
- **当前工作树（`bb_` 体系重构）**：源码、构建配置、脚本与文档已整理为 `bioBox` / `bb_*`（Go 包全量 `bb_` 前缀、`build_srcs/` 子模块路径、默认产物 `bioBox`）结构（本地未提交、未发版）。后续按官方 GitHub Assets 就位与验收的计划、标准及职责划分见[官方资产验收计划（v0.0.2）](docs/acceptance_plan_official_assets_v0.0.2.md)，当前交付与接续快照见[交付与资源配置](docs/bb_build/release_delivery.md)。

资源准备、上游下载边界与本地验收见[资源准备契约](docs/bb_upstream/resource_preparation_contract.md)。

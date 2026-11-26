# bioBox

把 [Protenix](https://github.com/bytedance/Protenix)（结构预测）和 [PXDesign](https://github.com/bytedance/PXDesign)（蛋白设计）打成可在 NVIDIA GPU 机器上原生安装的命令行工具，并整合为对外提供队列与多协议接口的 Go 服务 `bioBox`（Go module：`github.com/0xdevelop/bioBox`；gRPC wire package 兼容保持 `protenix_service.api.v1`；JWT 默认 `issuer`/`audience` 兼容保持 `protenix_service`；API key 前缀：`ptxk_`）。不用 Docker、conda。

两种用法互不干扰：

- **独立使用**：单产品包，`install_<app>.sh` 装到 `~/.local/share/<app>`（`<APP>_PREFIX` 可改），不需要 sudo。
- **与 Go 服务一起安装**：整合包，root 跑 `install_bioBox.sh`，按 Go 模板 install 体系把服务和两个运行时统一装到 `/bioBox`（运行时目录由脚本显式传入，不读独立模式的默认值）。

## 仓库目录结构

- 根目录为 `bioBox` Go 服务源码（`main.go`、`resource_cmd.go`、`go.mod`、`bb_ability/`、`bb_api/`、`bb_cli_proc/`、`bb_common/`、`bb_config/`、`bb_custom_cmd/`、`bb_docs/`、`bb_frontend/`、`bb_policy/`、`bb_runtime_resource/`、`bb_storage/`、`bb_test_ui/`、`example_files/`）。`bb_ability` 注册领域能力；`bb_cli_proc` 是这两个计算能力共用的进程执行基础设施，不放进 `bb_ability`。
- `bb_build/`：构建与预构建配置模板及双架构清单（`pre_build_config_template.yaml`、`gen_pre_build_config_linux_{amd64,arm64}.yaml`）。
- `build_srcs/`：上游源码 git submodule（`protenix`、`pxdesign`），不修改内部源码。
- `bb_suites/`：Protenix / PXDesign 的依赖覆盖、包装入口、安装脚本与补丁，不是 Go 领域包。
- `scripts/`：只留 Linux 打包、前缀安装和检查。`gen_frontend.sh`、`gen_proto.sh`、`git_tag.sh` 在仓库根。根目录 `build.sh` 是总入口。
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

`./build.sh` 默认按本机编译主包；`./build.sh pack`、`./build.sh protenix`、`./build.sh pxdesign` 分别进入 Linux 发行打包和两个前缀安装。

```bash
git clone git@github.com:0xdevelop/bioBox.git && cd bioBox
git submodule update --init --recursive
./build.sh --help
./build.sh                          # 本机 release
./build.sh test                     # 本机 test
./scripts/gen_pre_build_config.sh all
FORCE_PACK=1 ./build.sh pack        # 双 arch Linux 包 → build/upload_tmp_dir/
./build.sh pack release linux_arm64
```

- 无参数、`release`、`test`、`debug` 不打 Linux 双架构包，也不安装 CUDA 前缀。SQLite 沿用本机 CGO，脚本不把 `CGO_ENABLED` 设成 0，也不改 `GOOS`。
- `pack` / `all` / `protenix` / `pxdesign` 仍是 Linux 发行与 CUDA 13 前缀安装。Actions 调用 `bash ./build.sh pack release linux_<arch>`。在 macOS 上打 Linux 包会要求对应交叉编译器。
- 不经过 `build.sh` 时用 `go build -o bioBox .`。`bioBox.exe` 只在 Windows 上执行根 `./build.sh` 时生成，不是这条手工命令的产物。仓库不另加 Windows 脚本。
- 需要 Go（`go.mod` 指定版本）、bun（现场构建前端与 test_ui）、打 zip 时还要 zip。打非本机 Linux arch 时交叉编译器默认 `x86_64-linux-gnu-gcc` / `aarch64-linux-gnu-gcc`，可用 `CC_linux_amd64` / `CC_linux_arm64` 指定。
- 配置读可执行文件目录下的 `conf/config.yaml`。文件不存在时生成缺省配置。`example_files/config_example.yaml` 是示例，可复制成被 gitignore 的 `example_files/config_local.yaml` 作人工参考；进程不自动读它。默认端口：JSON-RPC `13001`、MCP `13002`、test_ui `13003`、WebSocket `13004`、gRPC `13005`、frontend `13006`。
- GoLand 共享配置 `.run/bioBox-debug.run.xml` 的 Run kind 是 Package，编译主包 `github.com/0xdevelop/bioBox`，工作目录 `$PROJECT_DIR$`。启动前用 bun 构建 `bb_test_ui/web` 和 `bb_frontend`。
- 发布包交付前先提交、再 `FORCE_PACK=1`，让 `BUILD_INFO.txt` 的 commit 对齐 HEAD。
- 发版入口是无参 `./git_tag.sh`（模板三段：`release` → `Build` → `RemoveOldRelease`）。Actions 在该结构内用 `ubuntu-24.04`（amd64）和 `ubuntu-24.04-arm`（arm64）做原生构建与六包内部完整性校验，最终仅发布两整合包 + `SHA256SUMS`；根 `./build.sh pack` 负责 Linux 打包。见[模板审计与恢复说明](docs/bb_build/release_template_audit.md)及[原生双架构说明](docs/bb_build/native_release.md)。

`pack` 只打发布包（不跑 GPU/pip）。`protenix` / `pxdesign` 只在仓库内 `bb_prefix/` 装/校验开发用运行时（uv + Python 3.14 + torch cu130，GPU 验证用，不是正式安装）。`all` 先打包再装两个 prefix。幂等：已就绪的步骤跳过；`FORCE_PACK=1` / `FORCE_PROTENIX=1` / `FORCE_PXDESIGN=1` 强制重做。无参数的根 `./build.sh` 不调用这两条安装路径。

这台 Mac 上 Protenix 与 pxdesign 的计算不能跑，模型推理没有在本机运行。使用层（API、任务、debug web UI）不依赖本机已经跑通模型；这不是本机已经启动并验证过使用层。依据：`scripts/suites/protenix/install_prefix_protenix.sh` 与 `scripts/suites/pxdesign/install_prefix_pxdesign.sh` 要求 CUDA 13（`nvcc`、torch cu130、CUDA 可用）；`build_srcs/protenix` 与 `build_srcs/pxdesign` 是未检出的空目录；本仓没有这两个程序的本机二进制。Linux 的 `pack` / `protenix` / `pxdesign` 保留。

`tmp/`、`build/`、`bb_prefix/`、前端 `dist/` 和 `node_modules/` 都是可再生成产物，不进 git，也不能当成配置或权重事实。本次不删除 `tmp/`。CI 的 bun 只写 `oven-sh/setup-bun@v2` 的 `bun-version: latest`，仓库没有钉死次版本。本机 bun 1.4.5，`tmp/build-host.log` 里 `bun install v1.4.5` 安装后编过前端。

## 任务查询与只读步进

服务支持运行中查看当前业务阶段、只读回看历史步骤，并把业务状态数组与引擎调度诊断分开。页面启用条件、API 查询、关联排障及取消/恢复边界见[任务操作说明](docs/bb_service/task_operations.md)；release 不加载调试页。

## 当前套件与版本机制边界

- 当前仅包含两套硬编码套件：`protenix`（默认 `v1`、`use_msa=false`；`cli_cfg.protenix.version_runtimes.v2.binary` 默认为空并拒绝未配置启动）与 `pxdesign`（固定 `PINNED_PROTENIX_REF=d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`、`preview`、`use_msa=false`）。
- 暂未建立任意多套件或同名多版本目录轴；扩充新套件需同步更新 `bb_build/pre_build_config_template.yaml`、`bb_suites/<suite>/`、`scripts/suites/<suite>/`、`scripts/linux_release_pack.sh`、`scripts/suites/protenix/install_prefix_protenix.sh`、`scripts/suites/pxdesign/install_prefix_pxdesign.sh`、`example_files/install_bioBox.sh` 与 `scripts/install_common.sh`。

## 两套 Python 环境不可合并

`protenix` 用最新上游；`pxdesign` 链依赖官方钉死的 Protenix v0.5.0+pxd。两者始终是独立 venv：独立模式各自安装；整合模式在 `/bioBox/runtime/` 下由同一个 install 流程安装、升级、卸载。

更多实现口径见 [系统设计](docs/bb_service/design.md)；机器验收记录见 `docs/bb_build/acceptance_*.md`。

## 验证码日志与可选邮件

验证码实际生成后无条件写服务 INFO，邮件为独立可选能力，失败如实返回。配置、迁移、日志权限与排障边界见[操作说明](docs/bb_service/verification_delivery.md)。

## 当前发版基线与安装实测

- **已发布基线（重构前）**：`v0.0.2` 与 `v0.0.3` / `latest`（commit `37fb324`）按重构前命名（`protenix_service`）已在 GitHub 发布；`main` 当前提交 `2473e4e`（`Update LICENSE`，BSD→MIT）。历史 `v0.0.2` 过程、命令与未覆盖项见[v0.0.2 发版与正式安装实测记录](docs/bb_build/acceptance_release_v0.0.2_20261120.md)，此前矩阵记录见[验收矩阵与证据](docs/bb_build/acceptance_product_matrix_20261119.md)。
- **当前工作树（`bb_` 体系重构）**：源码、构建配置、脚本与文档已整理为 `bioBox` / `bb_*`（Go 包全量 `bb_` 前缀、`build_srcs/` 子模块路径、默认产物 `bioBox`）结构（本地未提交、未发版）。后续按官方 GitHub Assets 就位与验收的计划、标准及职责划分见[官方资产验收计划（v0.0.2）](docs/acceptance_plan_official_assets_v0.0.2.md)，当前交付与接续快照见[交付与资源配置](docs/bb_build/release_delivery.md)。

资源准备、上游下载边界与本地验收见[资源准备契约](docs/bb_upstream/resource_preparation_contract.md)。

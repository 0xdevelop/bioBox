# protenix_build

把 [Protenix](https://github.com/bytedance/Protenix)（结构预测）和 [PXDesign](https://github.com/bytedance/PXDesign)（蛋白设计）打成可在 NVIDIA GPU 机器上原生安装的命令行工具，并可与对外提供队列与接口的 Go 服务 `protenix_service` 整合安装。不用 Docker、conda。

两种用法互不干扰：

- **独立使用**：单产品包，`install_<app>.sh` 装到 `~/.local/share/<app>`（`<APP>_PREFIX` 可改），不需要 sudo。
- **与 Go 服务一起安装**：整合包，root 跑 `install_protenix_service.sh`，按 Go 模板 install 体系把服务和两个运行时统一装到 `/protenix_service`（运行时目录由脚本显式传入，不读独立模式的默认值）。

## 交付物

每个 arch 三个 zip：

- `protenix_linux_{amd64,arm64}_release_v*.zip`、`pxdesign_linux_{amd64,arm64}_release_v*.zip`：独立单产品包
- `protenix_service_linux_{amd64,arm64}_release_v*.zip`：整合包（`runtime/<app>/` 与单产品包内容相同）

## 一、独立使用（单产品包）

解压后：

```
install_<app>.sh        安装/升级/回退/卸载/状态
README.md               使用说明
bin/<app>-run           安装后复制到安装目录的命令入口
conf/<app>.env          运行配置模板
scripts/                安装脚本（install_prefix_*、install_common、weights_reuse）
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

换安装目录：`PROTENIX_PREFIX=/dir`（或 `PXDESIGN_PREFIX=/dir`）。复用已有权重跳过下载：`WEIGHTS_REUSE_FROM=/已有/prefix/根`。

安装后目录：`bin/<app>-run`、`conf/<app>.env`、`data/`（权重）、`releases/<版本>/`（每版一套 venv，`current`/`previous` 符号链接指向）。

```bash
~/.local/share/protenix/bin/protenix-run pred -i input.json -o out --use_msa false -s 101
~/.local/share/pxdesign/bin/pxdesign-run pipeline --preset preview -i task.yaml -o out --N_sample 2
```

成功条件：退出码 0、输出目录有 `.cif`、无 `ERR/`、GPU 有占用。详细步骤（切模型、FAQ）见各包内 `README.md`。

## 二、与 Go 服务一起安装（整合包）

解压后：

```
protenix_service/
  protenix_service                 服务二进制
  protenix_service.service         systemd 单元（User=root，WorkingDirectory=/protenix_service）
  install_protenix_service.sh      安装 / 升级 / 卸载 / 运行时回退 / 状态（菜单）
  conf/config.yaml                 服务配置（cli_cfg 指向 /protenix_service/runtime/...）
  BUILD_INFO.txt                   版本、提交
  runtime/protenix/  runtime/pxdesign/   与单产品包内容相同
```

### 前置条件

- root 权限（安装到 `/protenix_service`，单元放到 `/etc/systemd/system`）
- NVIDIA 驱动 + CUDA 13 工具链（`nvidia-smi`、`nvcc`，默认 `CUDA_HOME=/usr/local/cuda-13.0`）
- [uv](https://github.com/astral-sh/uv)，能解析 Python 3.14。root 的 `PATH` 里没有 uv 时用 `UV=/path/to/uv` 指定
- 公网访问：安装时用 uv/pip 建 Python 环境，并从上游官方地址下载公开权重

### 安装

```bash
unzip protenix_service_linux_arm64_release_v*.zip
cd protenix_service
# 以 root 执行（用 sudo 时写成 sudo env UV=/path/to/uv ./install_protenix_service.sh）
UV=/path/to/uv ./install_protenix_service.sh
# 选 1
```

菜单：

| 选项 | 行为 |
| --- | --- |
| 1 install | 安装或重装。先装/升级两个运行时（失败则不动二进制与配置）；再把旧二进制、`conf/config.yaml`、`runtime/<app>/conf/<app>.env` 备份为 `.bak_<时间戳>` 后覆盖；复制单元到 `/etc/systemd/system`，`enable` + `start` |
| 2 update | 升级：停服务，升级两个运行时（保留 `<app>.env` 与权重；新版本构建失败时当前版本不动、旧服务重新启动）；只换二进制（旧的留 `.bak_<时间戳>`），不动 `conf/config.yaml`；更新单元后 `restart` |
| 3 uninstall | 整体卸载：`disable --now` 并删除单元，删除整个 `/protenix_service`（含运行时的 Python 环境与权重） |
| 4 rollback runtime | 两个运行时各自切回上一版本（`previous`），前后停启服务；服务二进制回退用 `.bak_*` |
| 5 status | 服务状态与两个运行时的版本、是否可运行 |

选项 1–3 完成后按模板约定删除解压目录和 zip；4、5 不删。复用已有机器上的权重跳过下载：`WEIGHTS_REUSE_FROM=<已校验的 prefix 根>`（与 `UV` 一样作为环境变量传给 install 脚本）。

### 安装后的目录

| 路径 | 内容 |
| --- | --- |
| `/protenix_service/protenix_service` | 服务二进制；旧版本 `protenix_service.bak_<时间戳>` |
| `/protenix_service/conf/config.yaml` | 服务配置；重装时旧配置 `config.yaml.bak_<时间戳>` |
| `/protenix_service/data/` | SQLite（`protenix_service.db`）、作业目录 `cli_jobs/<task_id>/` |
| `/protenix_service/logs/` | 服务日志 |
| `/protenix_service/runtime/protenix/` | `bin/protenix-run`、`conf/protenix.env`、`data/`（权重与 CCD）、`releases/<版本>/venv`、`current`、`previous` |
| `/protenix_service/runtime/pxdesign/` | `bin/pxdesign-run`、`conf/pxdesign.env`、`data/`（权重、AF2/MPNN）、`releases/<版本>/venv`、`current`、`previous` |
| `/protenix_service/runtime/uv/` | uv 托管的 Python 解释器（venv 依赖它）与 uv 缓存 |
| `/etc/systemd/system/protenix_service.service` | systemd 单元 |

服务通过 `conf/config.yaml` 的 `cli_cfg` 调用 `/protenix_service/runtime/<app>/bin/<app>-run`。`protenix` 的 v2 需要授权权重，`cli_cfg.protenix.version_runtimes.v2.binary` 默认为空（未配置时直接拒绝，不回退 v1）。

## 任务查询与只读步进

服务支持运行中查看当前业务阶段、只读回看历史步骤，并把业务状态数组与引擎调度诊断分开。页面启用条件、API 查询、关联排障及取消/恢复边界见[任务操作说明](docs/task_operations.md)；release 不加载调试页。

## 从源码构建发布包

```bash
git clone <本仓库> && cd protenix_build
git submodule update --init --recursive
./gen_pre_build_config.sh all
FORCE_PACK=1 ./build.sh pack        # 双 arch → build/upload_tmp_dir/ 下的单产品包与整合包
./build.sh pack linux_arm64         # 只打一个 arch
```

- 需要 Go、bun（现场构建前端与 test_ui）、zip。服务二进制用 cgo（SQLite），打非本机 arch 时需交叉编译器（默认 `x86_64-linux-gnu-gcc` / `aarch64-linux-gnu-gcc`，可用 `CC_linux_amd64` / `CC_linux_arm64` 指定）。
- 发布包交付前先提交、再 `FORCE_PACK=1`，让 `BUILD_INFO.txt` 的 commit 对齐 HEAD。
- 发版（打 tag、上传）走 `git_tag.sh` 与 `.github/workflows/release.yml`，`build.sh` 只负责构建。

`./build.sh` 子命令：`pack` 只打发布包（不跑 GPU/pip）；`protenix` / `pxdesign` 只在仓库内 `prefix/` 装/校验开发用运行时（uv + Python 3.14 + torch cu130，GPU 验证用，不是正式安装）；`all`（默认）先打包再装两个 prefix。幂等：已就绪的步骤跳过；`FORCE_PACK=1` / `FORCE_PROTENIX=1` / `FORCE_PXDESIGN=1` 强制重做。

## 两套 Python 环境不可合并

`protenix` 用最新上游；`pxdesign` 链依赖官方钉死的 Protenix v0.5.0+pxd。两者始终是独立 venv：独立模式各自安装；整合模式在 `/protenix_service/runtime/` 下由同一个 install 流程安装、升级、卸载。

更多实现口径见 `docs/design.md`；机器验收记录见 `docs/acceptance_*.md`。

# protenix_build

把 [Protenix](https://github.com/bytedance/Protenix)（结构预测）、[PXDesign](https://github.com/bytedance/PXDesign)（蛋白设计）和对外提供队列与接口的 `protenix_service` 打成**一套服务**，在 NVIDIA GPU 机器上原生安装。交付物是每个 arch 一个发布 zip；解压后用 root 跑 `install_protenix_service.sh`，服务与两个原生运行时统一装到 `/protenix_service`。不用 Docker、conda。

## 交付物

- `protenix_service_linux_{amd64,arm64}_release_v*.zip`

解压后：

```
protenix_service/
  protenix_service                 服务二进制
  protenix_service.service         systemd 单元（User=root，WorkingDirectory=/protenix_service）
  install_protenix_service.sh      安装 / 升级 / 卸载 / 运行时回退 / 状态（菜单）
  conf/config.yaml                 服务配置（cli_cfg 指向 /protenix_service/runtime/...）
  BUILD_INFO.txt                   版本、提交
  runtime/protenix/  runtime/pxdesign/
    install_<app>.sh  scripts/  bin/<app>-run  conf/<app>.env
    deps/  patches/  src/<app>/  src/SRC_SHA  examples/  README.md  BUILD_INFO.txt
```

## 前置条件

- root 权限（安装到 `/protenix_service`，单元放到 `/etc/systemd/system`）
- NVIDIA 驱动 + CUDA 13 工具链（`nvidia-smi`、`nvcc`，默认 `CUDA_HOME=/usr/local/cuda-13.0`）
- [uv](https://github.com/astral-sh/uv)，能解析 Python 3.14。root 的 `PATH` 里没有 uv 时用 `UV=/path/to/uv` 指定
- 公网访问：安装时用 uv/pip 建 Python 环境，并从上游官方地址下载公开权重

## 安装

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

## 安装后的目录

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

## 从源码构建发布包

```bash
git clone <本仓库> && cd protenix_build
git submodule update --init --recursive
./gen_pre_build_config.sh all
FORCE_PACK=1 ./build.sh pack        # 双 arch → build/upload_tmp_dir/protenix_service_linux_<arch>_release_<version>.zip
./build.sh pack linux_arm64         # 只打一个 arch
```

- 需要 Go、bun（现场构建前端与 test_ui）、zip。服务二进制用 cgo（SQLite），打非本机 arch 时需交叉编译器（默认 `x86_64-linux-gnu-gcc` / `aarch64-linux-gnu-gcc`，可用 `CC_linux_amd64` / `CC_linux_arm64` 指定）。
- 发布包交付前先提交、再 `FORCE_PACK=1`，让 `BUILD_INFO.txt` 的 commit 对齐 HEAD。
- 发版（打 tag、上传）走 `git_tag.sh` 与 `.github/workflows/release.yml`，`build.sh` 只负责构建。

`./build.sh protenix` / `pxdesign` / `all` 另外会在仓库内 `prefix/` 下装开发用的运行时（GPU 验证用，不是正式安装）。

## 两套 Python 环境不可合并

`protenix` 用最新上游；`pxdesign` 链依赖官方钉死的 Protenix v0.5.0+pxd。两者在 `/protenix_service/runtime/` 下是独立 venv，同一个 install 流程安装、升级、卸载。

更多实现口径见 `docs/design.md`；机器验收记录见 `docs/acceptance_*.md`。

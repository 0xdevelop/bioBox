# 设计说明

面向维护者的实现口径（使用者请看仓库根 `README.md`）。

## 运行栈

- 宿主：CUDA 13、Python 3.14、Node 24 LTS（编排侧）
- torch：仅从 `https://download.pytorch.org/whl/cu130` 安装 cp314 轮子（当前验证 2.14.1+cu130）
- 不用 Docker / conda / micromamba；不用 Python 3.11 回退

## 源码与约束

- 上游在 `build_srcs/{protenix,pxdesign}` submodule；**不修改** submodule 内文件
- 上游 `requirements.txt` 钉位在 3.14/aarch64/cu130 上不可用时，用仓库 `bb_suites/<suite>/deps.txt` 覆盖，并在文件内逐条写原因
- 运行期兼容补丁（如 argparse `%`、pxdbench json float32）只打在 prefix 内快照或 site-packages，不回写 submodule

## 双 prefix

- `bb_prefix/protenix` 与 `bb_prefix/pxdesign` 两套独立 venv，禁止合并
- pxdesign 链依赖官方钉死的 Protenix@v0.5.0+pxd（与 protenix prefix 的「最新 submodule」不是同一套）

## 交付物

两种用法、两类包：Protenix / PXDesign 单独使用时是各自独立的**命令行工具**（单产品包）；与 Go 服务 `bioBox` 结合时按 Go 模板 install 体系整合成**一套服务**（整合包、一个 install 脚本）。

- `./scripts/build.sh pack` 每个 arch 产出独立单产品包 `protenix_` / `pxdesign_linux_<arch>_<mode>_<version>.zip`（内容即下述运行时安装材料），以及整合包 `bioBox_linux_<arch>_<mode>_<version>.zip`：顶层是服务二进制、`install_bioBox.sh`、systemd 单元、`conf/config.yaml`、`BUILD_INFO.txt`；`runtime/<app>/` 下是运行时安装材料——安装脚本（`install_<app>.sh` + `scripts/install_common.sh` + `install_prefix_*`）、命令入口 `bin/<app>-run`、配置模板 `conf/<app>.env`、**git archive 的上游快照**（`src/<product>/` + `src/SRC_SHA`）、`deps/`、`patches/`、`examples/`、`README.md`、`BUILD_INFO.txt`
- 包内安装只读包内路径；联网仅用于 pip 与权重
- 仓库内 `./scripts/build.sh protenix|pxdesign` 通过 `scripts/repo_src_defaults.sh` 指向 submodule（该文件不打进 zip）

## 安装布局

- 独立模式（单产品包 `install_<app>.sh`）：安装目录默认 `~/.local/share/<app>`（`<APP>_PREFIX` 覆盖），不需要 sudo
- 整合模式：服务与两个运行时是同一套服务、同一个 zip：root 跑 `install_bioBox.sh`（模板 install 体系，`APP_DIR=/bioBox`），它调用包内 `runtime/<app>/install_<app>.sh` 并显式传入 `<APP>_PREFIX`，运行时固定装到 `/bioBox/runtime/<app>`，uv 托管的 Python 与缓存在 `/bioBox/runtime/uv/`
- Python 环境和权重不打进 zip：安装时用 uv/pip 现场建 venv，权重由 `bioBox resource prepare` 按配置清单校验/受控下载（`WEIGHTS_REUSE_FROM` 已退役），zip 只带源码快照、依赖覆盖、补丁和脚本
- `releases/<版本>/` 每版一套 venv；`current` / `previous` 符号链接，`update` 先构建新版本、校验通过再原子切换，失败不动旧版本；`rollback` 切回 `previous`
- `conf/`（配置）、`data/`（权重）在 `update` 时保留；单个运行时 `install_<app>.sh uninstall` 默认保留、`--purge` 才删；`install_bioBox.sh` 菜单 3 整体卸载会删除整个 `/bioBox`（含权重）

## 验收分层

脚本可跑、zip 生成、`<app>-run ... --help`、GPU 真推理（有 CIF、无 ERR/、GPU util>0）是不同层级，不可互相替代。

<!-- acceptance rerun marker -->

## 套件扩展点与当前机制边界

- 当前仅支持两套硬编码套件：`protenix`（默认 `v1`、`use_msa=false`；`cli_cfg.protenix.version_runtimes.v2.binary` 默认为空字符串，未配置时 fail-closed 拒绝）与 `pxdesign`（固定 `PINNED_PROTENIX_REF=d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`、`preview`、`use_msa=false`）。
- 现行结构为「一份材料、多处点名」，扩展新套件或同名多版本必须同步修改：`bb_build/pre_build_config_template.yaml`（`products[]`）、`bb_suites/<suite>/`、`scripts/suites/<suite>/`、`scripts/build.sh`（`build_protenix` / `build_pxdesign`）、`example_files/install_bioBox.sh`（`RUNTIME_APPS`）与 `scripts/install_common.sh`（`case` 分支）。
- 机制缺口：尚未建立动态多套件加载器或 `bb_suites/<suite>/<version>` 同名多版本目录轴，不宣称任意套件或同名多版本已开箱支持。

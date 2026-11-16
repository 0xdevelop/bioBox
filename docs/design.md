# 设计说明

面向维护者的实现口径（使用者请看仓库根 `README.md`）。

## 运行栈

- 宿主：CUDA 13、Python 3.14、Node 24 LTS（编排侧）
- torch：仅从 `https://download.pytorch.org/whl/cu130` 安装 cp314 轮子（当前验证 2.14.1+cu130）
- 不用 Docker / conda / micromamba；不用 Python 3.11 回退

## 源码与约束

- 上游在 `build_srcs/{protenix,pxdesign}` submodule；**不修改** submodule 内文件
- 上游 `requirements.txt` 钉位在 3.14/aarch64/cu130 上不可用时，用仓库外层 `deps/` 覆盖，并在文件内逐条写原因
- 运行期兼容补丁（如 argparse `%`、pxdbench json float32）只打在 prefix 内快照或 site-packages，不回写 submodule

## 双 prefix

- `prefix/protenix` 与 `prefix/pxdesign` 两套独立 venv，禁止合并
- pxdesign 链依赖官方钉死的 Protenix@v0.5.0+pxd（与 protenix prefix 的「最新 submodule」不是同一套）

## 交付物

Protenix、PXDesign 两个命令行运行时与提供队列、并发和对外接口的 `protenix_service` 是**同一套服务**，打成一个发布包、一个 install 脚本。

- `./build.sh pack` 每个 arch 产出一个 `protenix_service_linux_<arch>_<mode>_<version>.zip`：顶层是服务二进制、`install_protenix_service.sh`、systemd 单元、`conf/config.yaml`、`BUILD_INFO.txt`；`runtime/<app>/` 下是运行时安装材料——安装脚本（`install_<app>.sh` + `scripts/install_common.sh` + `install_prefix_*`）、命令入口 `bin/<app>-run`、配置模板 `conf/<app>.env`、**git archive 的上游快照**（`src/<product>/` + `src/SRC_SHA`）、`deps/`、`patches/`、`weights_reuse.sh`、`examples/`、`README.md`、`BUILD_INFO.txt`
- 包内安装只读包内路径；联网仅用于 pip 与权重
- 仓库内 `./build.sh protenix|pxdesign` 通过 `scripts/repo_src_defaults.sh` 指向 submodule（该文件不打进 zip）

## 安装布局（`install_protenix_service.sh` + 运行时 `install_<app>.sh`）

- 服务与两个运行时是同一套服务、同一个 zip：root 跑 `install_protenix_service.sh`（模板 install 体系，`APP_DIR=/protenix_service`），它调用包内 `runtime/<app>/install_<app>.sh`，运行时固定装到 `/protenix_service/runtime/<app>`，uv 托管的 Python 与缓存在 `/protenix_service/runtime/uv/`
- Python 环境和权重不打进 zip：安装时用 uv/pip 现场建 venv，权重从上游官方地址下载（或 `WEIGHTS_REUSE_FROM` 复用），zip 只带源码快照、依赖覆盖、补丁和脚本
- `releases/<版本>/` 每版一套 venv；`current` / `previous` 符号链接，`update` 先构建新版本、校验通过再原子切换，失败不动旧版本；`rollback` 切回 `previous`
- `conf/`（配置）、`data/`（权重）在 `update` 时保留；单个运行时 `install_<app>.sh uninstall` 默认保留、`--purge` 才删；`install_protenix_service.sh` 菜单 3 整体卸载会删除整个 `/protenix_service`（含权重）

## 验收分层

脚本可跑、zip 生成、`<app>-run ... --help`、GPU 真推理（有 CIF、无 ERR/、GPU util>0）是不同层级，不可互相替代。

<!-- acceptance rerun marker -->

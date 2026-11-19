# git_tag.sh → 原生双架构 Actions → build.sh

## 职责与入口

- `git_tag.sh`：发版入口。先做版本/干净主分支检查；只有显式 `--publish vX.Y.Z` 才创建并普通推送一个 annotated tag。不会自动 bump、提交代码、推 main、改写 latest tag、强推、覆盖已有 tag 或删除 release。
- `.github/workflows/release.yml`：接 tag push，调度原生 runner、汇集完整产物，然后才创建草稿并发布。手动 `workflow_dispatch` **只构建、校验和留存 Actions artifact**，即使手动选择 tag ref 也不发布。
- `build.sh`：继续复用既有 `pack release linux_<arch>`，不负责 tag/release。Actions 不执行默认的 `all`，避免把 CI 打包误变成 prefix 安装、pip/权重准备或 GPU 任务。

旧链路并未满足目标：根 git_tag.sh 直接 exit 1；Actions 提前创建占位 release，再在单个 ubuntu-latest job 里交叉构建两个 arch，且 workflow_dispatch 也进入发布。现在顺序改为 Build(matrix) → Aggregate → release；移除无实际作用的清理占位，不删除任何旧版本。

## 原生矩阵与依赖边界

| matrix.arch | runs-on | 本 job 目标 |
| --- | --- | --- |
| amd64 | ubuntu-24.04 | linux_amd64 三包 |
| arm64 | ubuntu-24.04-arm | linux_arm64 三包 |

`check_native_build.sh` 同时核对 Linux/uname、Go host 与 target、CGo 开启、本机 gcc 的 GNU target；不是只改 GOARCH。服务已有构建入口仍强制 CGO_ENABLED=1（SQLite），新门禁还检查 zip 内服务 ELF machine 与 Go build info 的 GOOS/GOARCH/CGO_ENABLED，不能用纯 Go 的假 SQLite 成品过门禁。

runner 的打包依赖是原生 Go（protenix_service/go.mod，当前要求 1.26.10；根生成器的 1.25 约束可兼容，不依靠隐式下载切换）、gcc/build-essential、pkg-config、zip/unzip、bun；前端和 test_ui 的 embed 材料沿用原构建脚本，release 不因此启用调试页。CI 构建 GNU/glibc Linux 二进制，不承诺所有 Linux 发行版或 musl 兼容。

独立包以及整合包的 runtime 是上游源码快照 + wrapper + install/deps/patches + 示例，不含预装 Python venv、Torch/CUDA 动态库或模型权重。原生 Actions 构建成功不等于目标机器依赖安装或 GPU 推理成功。安装仍需 NVIDIA 驱动/CUDA 13、uv/Python 3.14、原生构建依赖与可访问的 pip/权重来源；两产品 venv 保持独立。deps/protenix.txt、deps/pxdesign.txt 的依赖覆盖和实证目前主要来自 aarch64/GB10；本轮不把这些旧证据扩大为 amd64 冷安装、wheel 可用性或科学质量保证，也不为打包安装 MySQL/Redis。

## 六包与清单门禁

每架构固定三个文件（版本与预构建配置一致）：

```text
protenix_linux_<arch>_release_vX.Y.Z.zip
pxdesign_linux_<arch>_release_vX.Y.Z.zip
protenix_service_linux_<arch>_release_vX.Y.Z.zip
```

各 job 只上传自己的三个 zip，加 `manifest-linux_<arch>.json` 与 `SHA256SUMS-linux_<arch>.txt`；artifact 名分别是 `release-packages-linux-amd64`、`release-packages-linux-arm64`，不会互相写同名 artifact。重跑只替换本架构同名 artifact；读取限当前 workflow run，可以复用同一 run 中已成功的另一架构，不混其他 run。

Aggregate 在两个 matrix job 都成功后执行，要求恰好十个架构文件、六个当前提交且 dirty=0 的包：逐包校验版本、完整 commit、架构/模式、必须的安装材料、整合包两个嵌套 runtime BUILD_INFO、服务 ELF/CGo、大小和 SHA-256。缺包、旧包、错架构、篡改、重复清单项或多余文件全部失败。然后生成：

- `release-manifest.json`：当前六包的名称、大小、SHA-256 与共同版本/commit。
- `SHA256SUMS`：六 zip、两个架构 manifest、两个架构 checksum 和总 manifest 的 SHA-256。

最终 `release-packages-complete` 恰好十二个附件。发布 job 重新校验完整 artifact，才用 `gh release create --verify-tag --draft` 创建新草稿并上传全部文件；核对远端十二个附件的名称、数量与大小后才 `--draft=false`。这一步是远端 inventory 校验，不冒充重新下载全部远端内容比对哈希。已有 release 不覆盖、不 clobber；失败时至多留下未公开草稿，需人工确认原因，不自动删除或覆盖重试。故障状态不应形成对外可见的半套成功 release。

## 操作（本轮没有执行发布）

只读预检，不建 tag、不联网：

```bash
./git_tag.sh --check
# 可指定预期版本；必须与模板及两份已生成配置完全一致
./git_tag.sh --check vX.Y.Z
```

版本事实仍为 pre_build_config_template.yaml 的 project.version；修改版本后用既有 gen_pre_build_config.sh all 同步两份生成配置，再检查、提交并经批准推 main。入口拒绝三个文件版本不一致、非法类型/版本、脏工作区和非 main；不读取或修改 config_local.yaml。

只有另获明确发版授权时，才运行：

```bash
./git_tag.sh --publish vX.Y.Z
```

该分支还要求 origin/main 已等于 HEAD，本地/远端同名 tag 都不存在；不替用户推待审代码。tag push 失败时本地 tag 可能已创建，停下人工检查，脚本不会清理或强推。此命令会触发真正发版，不能拿它当构建验证入口。嵌套 protenix_service/git_tag.sh 是旧模板脚本，不是本项目系列发版入口，不调用它。

需要无发布的云构建证据时：先经批准推送本次代码，再在该明确 ref 上手动运行 Release_Build 的 workflow_dispatch。验证两个 runner 的 Build、Aggregate、完整 artifact，并确认 release job skipped。本轮尚无此真实 Actions 运行证据。

## 本地检查与证据层级

```bash
go run gen_release_assets.go self-test
bash scripts/check_native_build.sh arm64  # amd64 原生机器使用 amd64
bash -n git_tag.sh build.sh scripts/check_native_build.sh
```

self-test 是表驱动 fixture/静态检查，包含配置类型/版本、BUILD_INFO、两架构 manifest、篡改/缺失/重复/错误架构、远端上传 inventory、ZIP 材料与路径、ELF 拒绝、原生预检分支，以及 workflow 结构/发布条件/命令顺序与内嵌 shell 语法。测试中的“两架构六文件”是明示的 fixture，完整包校验会拒绝它，不能称为原生双架构构建成功。

真实原生 arm64 打包与 ZIP/ELF/CGo 门禁可在 spark 上独立验证；amd64 原生 Actions、artifact 聚合和 GitHub 草稿/发布事务只有真实远端运行后才有云端实证。GPU/HTTP 成品验收与其余发版缺口仍见 [本轮产物验收](acceptance_product_matrix_20261119.md)，本任务不借机修改无关安装生命周期、安全配置或推理逻辑。

### 2026-11-19 本地验证记录（Asia/Shanghai，UTC+8）

- `go run gen_release_assets.go self-test`：50 项 fixture/静态检查通过，`go vet gen_release_assets.go` 通过。
- spark 原生 arm64 预检通过；新门禁已实际检查既有干净提交 861c80c 的三个 arm64 zip，包含实际服务 ELF/CGo 与嵌套 runtime 元数据，成功生成本架构五文件。
- 把这五个 arm64 文件送入聚合入口，实际拒绝缺少 amd64 的半套输入，未创建输出目录。
- 本地记录在 `tmp/native-release-20261119/`；旧文档提交 ecc7c7d 保留。本记录不是 Actions 的 amd64/arm64 双 runner 实跑证据；未推送新提交或创建 tag/release。

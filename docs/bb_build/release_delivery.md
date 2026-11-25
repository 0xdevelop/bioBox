# 当前交付：三个公开附件与既有资源离线模式

更新：2026-11-24（UTC+8）。当前工作树为 AT-1（`bb_` 体系与根目录收敛重构，本地未提交）；**没有运行 `scripts/git_tag.sh`，没有 commit / push / 打 tag / 发布 / 替换或删除已发布资产**。历史安装/GPU 记录与重构前 `v0.0.2` / `v0.0.3` 资产边界均不得当作重构后新包验收。

## 发布清单

公开附件只保留：

- `bb_service_linux_amd64_release_v<version>.zip`
- `bb_service_linux_arm64_release_v<version>.zip`
- `SHA256SUMS`：仅两行，覆盖上述两包，不再套娃校验其他清单。

**基线区分**：重构前 `v0.0.3`（与 `latest` 移动 tag）已在提交 `37fb324` 发布（旧包名 `protenix_service_linux_{amd64,arm64}_release_v0.0.3.zip` + `SHA256SUMS`）；`main` 当前 HEAD 为 `2473e4e20b41ecf750000b751ec1069f9e7bcc62`（`Update LICENSE`，BSD→MIT，父提交 `37fb324`）。当前工作树配置版本来自 `2473e4e` 基线（`v0.0.3`）；不预设下一具体版本号，后续若经审查并获门禁与发版授权，原有**无参** `scripts/git_tag.sh` 会按模板基于当前配置版本递增，预期新公开附件为 `bb_service_linux_amd64_release_v<version>.zip`、`bb_service_linux_arm64_release_v<version>.zip`、`SHA256SUMS`。GitHub 自动 Source code 不属于自造附件；不会删除历史 `v0.0.2` / `v0.0.3` 资产来假装新发布已经收缩。

`scripts/build.sh pack` 仍生成每架构的 protenix / pxdesign 独立包及整合包；两套独立 venv、安装/升级/回退脚本不删除。`arch` 在本架构检查三包并产生五项 Actions job 间内部中转文件（仅用于跨 runner 汇聚，不是交付附件）；`aggregate` 要求两架构十项内部材料，校验六包、BUILD_INFO、ELF/CGo、来源 commit 后，仅复制两个整合包并生成一份校验文件到全新空目录；`verify` 要求恰好三项并再次检查两包和校验；`uploaded` 也只接受三项远端名字/大小（不是远端内容重新下载证明）。

Release workflow 只上传 `tmp/release-assets/*`，不再把它复制回有内部/旧包的 `build/upload_tmp_dir`。非空输出目录/多余 stale 文件均拒绝。三段 release → Build（内含双原生 runner）→ RemoveOldRelease 不改，`scripts/git_tag.sh` 的无参入口/自动版本不改、无新参数；本轮只增量修复缺失旧tag/release时的幂等清理，真实错误仍失败。v0.0.2在本地和远端清理处均显式保留。

独立包改为内部产物，并未以删除独立安装路径来减少附件。但现有独立包没有随包原生资源协调入口时仍会明确失败，不能回退旧 curl。**独立安装完整闭环仍未验证，也不因保留脚本就宣称已修好**；需要独立交付时必须验证协调入口与配置绑定，而不是直接重新挂附件。

## 默认选择：本地内容pin与显式选择的公开下载源

成品 `conf/config.yaml` 来自 `example_files/config_example.yaml`。现默认显式启用完整的 5 项 Protenix + 11 项 PXDesign 清单，`download_enabled: false`，无持久化确认；14项现在有来自固定上游源码/官方Git仓库的HTTPS来源，默认不触发网络。摘要逐项沿用已记录并再次校验的 **USER_EXISTING_LOCAL_SNAPSHOT_ONLY (2026-11-23)** 内容基准，不是 ByteDance 官方发布的 SHA256/签名，更不是上游真实性或科学质量认证；采用此配置就是明确选择这些既有文件的内容 pin，不适用时须替换成所选来源的完整可信清单，不许关闭校验凑通过。

该选择源自本轮对已授权本机资源复用的执行范围，不是把本地基准改名成“官方清单”。精确16项 ID/size/SHA256、局部公开来源证据见 [原资源审查](../bb_upstream/resource_final_review.md#已授权用户既有本地资源现有可用方式)；运行时代码按稳定 ID 将整族路径编译到 runtime conf 根，避免在 YAML 反复写机器绝对路径。

正式安装根统一为 `/bb_service`，unit 仍 `User=root`；默认 binary 值保留官方安装路径，以兼容安装器在复制服务前调用解压目录协调器的次序。`work_root` 使用二进制同根 `./data/cli_jobs`；自配 binary/资源/cache 的相对值同样以二进制/runtime根解析，绝不以调用者 cwd 解析。没有改全局环境、profile、sudoers；上游所需变量仍只传子进程。

### 操作者实际配置步骤（本切片没有执行正式安装）

1. 对新包首次安装/重装，先编辑**解压目录** `runtime/protenix/conf/protenix.env`、`runtime/pxdesign/conf/pxdesign.env` 的根 value，选择已有资源所在目录。安装器会把它们写到目标 runtime conf；仅编辑旧安装配置后选择 reinstall 会被备份，不应误以为仍会沿用。升级 `update` 则保留目标现有 `.env`。
2. 示例根族（用自己已存在的真实目录替换占位值；不复制权重，不强制使用开发 prefix；不创建或覆盖目录软链）：

```ini
# runtime/protenix/conf/protenix.env
PROTENIX_MODEL=protenix_base_default_v1.0.0
PROTENIX_ROOT_DIR=/absolute/existing/protenix-data
# 该根下必须有 common/ 与 checkpoint/ 中所需的5项文件。

# runtime/pxdesign/conf/pxdesign.env
PXDESIGN_DATA=/absolute/existing/pxdesign-data
PXDESIGN_CHECKPOINT_DIR=
TOOL_WEIGHTS_ROOT=
PROTENIX_DATA_ROOT_DIR=
# 三个空 override 分别派生为 DATA/release_data/checkpoint、DATA/tool_weights、DATA/protenix_data。
# 若资源分散，逐一填写三个根的实际绝对路径；不能只改 DATA 却保留旧绝对 override。
```

保留各 `.env` 中其余 CUDA/cache 配置；绝对目录仅是本机选择，不写进发布版硬编码。已有目录软链接也可直接使用，绝不 `ln -sf` 覆盖已有目录。

3. 已具备安装权限、runtime环境及配置后，从任意 cwd、无TTY执行：

```bash
/bb_service/bb_service resource status protenix.v1
/bb_service/bb_service resource prepare protenix.v1
/bb_service/bb_service resource status pxdesign.preview
/bb_service/bb_service resource prepare pxdesign.preview
/bb_service/bb_service resource verify protenix.v1
/bb_service/bb_service resource verify pxdesign.preview
```

`status/prepare=ready` 表示按所选内容pin通过资源完整性校验；`verify` 还要求已安装受管runtime/source/venv，不等于GPU业务验收。默认缺文件返回 `download_disabled`/失败，摘要不匹配拒绝覆盖；不读stdin、不触发上游Python。已有文件不copy、不联网。Protenix v1的5项已有公开URL与明确本地内容pin；PXDesign的9项有公开URL，另2项AF2必须先复用既有文件。选择这些来源并确认接受其本地内容基准后，显式打开所选集合download_enabled，先status取得确认令牌，经原CLI交互或API确认再prepare；确认不落盘，不可从systemd环境隐式取得。下载/恢复/状态/ready到启动的唯一owner仍是Go，禁止同时启动历史Python自动下载器。

4. 业务调用必须显式选择用户已授权的模式，不在代码中暗改科学选项：Protenix arguments 包含 `version: "v1", use_msa: false`（默认模型 `protenix_base_default_v1.0.0`）；PXDesign 包含 `preset: "preview", use_msa: false`。其他正常输入/输出参数照原协议提供。MSA/template/RNA-MSA/ESM/v2/extended等未协调分支不因本清单而受支持，缺显式false仍拒绝。

### 全新联网机器仍缺什么

2026-11-23 17:38 UTC+8 再次只读HEAD公开TOS v1/PXDesign模型、两种CCD及Google AF2 tar：均200，body读取0字节；TOS给出CRC64/multipart ETag，GCS给出MD5及generation，未提供完整所需SHA256。公开出处：

- https://protenix.tos-cn-beijing.volces.com/checkpoint/protenix_base_default_v1.0.0.pt
- https://pxdesign.tos-cn-beijing.volces.com/release_model/pxdesign_v0.1.0.pt
- https://protenix.tos-cn-beijing.volces.com/common/components.cif
- https://pxdesign.tos-cn-beijing.volces.com/release_data/components.v20240608.cif
- https://storage.googleapis.com/alphafold/alphafold_params_2022-12-06.tar

不能据此编造官方SHA256；现有AF2两成员的本地摘要不能代替整个tar的SHA256。PXDesign全新联网安装仍需补齐AF2来源/pin（可信整个tar摘要，或另经审查的下载/提取契约）；Protenix v1可明确选择所给5项上游URL+本地内容pin下载，但不能说这些摘要由官方签发。本次不降级为exists/无校验/TOFU，不下载大权重。默认复用路线已验证到资源ready；14个URL在2026-11-23 18:43 UTC+8只做HEAD均200且size匹配（body读0字节）。远端内容是否仍匹配所选SHA256最终由Go下载器验证，不拿HEAD/CRC/ETag当SHA256通过，不宣称全新环境/真实推理已通过。

## 本次实测证据与未过门禁

项目 `tmp/release-t169/` 保存日志/脚本（均非发布附件）：

- `release-self-test.log`：57项fixture/static契约PASS；包含精确三附件、两行校验、stale/非空stage拒绝、保留内部六包与模板无参入口约束。
- `asset-fixtures/result.json`：9次真实gen_release_assets命令调用验证native arm64 prepare、aggregate/verify/uploaded成功，以及错架构、非空stage、stale、错校验、缺内部独立包拒绝。**输入为合成ZIP/ELF元数据fixtures，不是Actions双原生包、更不是科学/安装证据**；最终暂存恰好两整合zip+SHA256SUMS，旧配置版本v0.0.2仅用于本地fixture，不会发布。
- `test.log`、`race.log`、`final-targeted-race.log`、`build.log`：实例单测、race与本地包构建；配置测试直接读取成品YAML验证完整5+11项、离线状态、相对根覆盖、未显式no-MSA拒绝。发现旧build命令只编main.go遗漏resource_cmd.go，改成编整个main包；未改release三段。
- `local-reuse-result.json`：2026-11-23 17:42 UTC+8，使用成品YAML清单，仅修改tmp配置中的binary值及runtime conf根value，真实16文件共6,571,265,681 bytes，两集合从不同cwd/无TTY执行status和prepare均ready；源device/inode/size/mtime未变，零权重copy/link/网络下载。新tmp布局没有venv，runtime verify正确拒绝在Python之前。**不能把资源ready/假CLI测试说成真实推理通过。**
- `public-provenance.json`：上述公开HEAD结果，非大文件下载/全套官方manifest。

L1工程：本地单测/race/build/发版契约检查；L2：唯一下载owner、保留两套runtime与模板三段、fresh stage隔离；L3：真实既有资源配置→status/prepare ready（环境/模型未运行）；L4正式安装与GPU科学/业务验收：**未执行/未通过**。

仍缺：基于最终提交的新双架构Actions包、从公开资产下载复核、正式根全新环境重建/安装、root/systemd常驻、真实v1/no-MSA与preview/no-MSA任务产物、卸载闭环；独立runtime协调入口/完整安装兼容性也未验收。未发现仓库外部审查完成证据，只有已标边界的本地源码/测试审查。本轮已修复模板/实例清理的不存在no-op：12项离线stub命令回归覆盖不存在、存在、权限/网络/服务器/删除错误；实例另验证v0.0.2保护分支。没有执行真实清理。工作流已恢复为原模板三段触发（tag `v*` / `workflow_dispatch` → `release` → `Build`（内含 `build_native` 双原生构建、内部中转汇总与包内容/资源门禁 smoke）→ `RemoveOldRelease`），不使用分支候选 artifact 发布路线。

所以“实现与本地检查完成”不等于“允许现在声称正式发版验收完成”。本切片在主端收回证据前不执行 `scripts/git_tag.sh`；受保护config_local仅hash，bb_sources/show_yeah未改；原未提交README验收计划链接和计划文件保留、不混提交。

## AT-1 接续快照（DevBuild → DevFull 独立只读审查，2026-11-24 UTC+8）

- **主责、审查方与授权出处**：主责执行者为 **DevBuild**（于 `spark-901f` `/home/certa/workspace/bioBox` 唯一写入者；原路径 `/home/certa/workspace/protenix_build` 已原地迁至 `/home/certa/workspace/bioBox`），独立只读审查方为 **DevFull**（未参与实现、不访问 spark）。全部改动依据用户 `t189-t191`「整理并跑通 / 属性结构贯穿」既有授权执行，仅做结构收敛与最小兼容性修正，不新增功能或扩权。
- **硬限制**：不执行 `git commit` / `git push` / `git tag` / `scripts/git_tag.sh` / `git add`；不执行 `sudo`、系统级 `/bb_service` 安装、systemd 启停、GPU 推理；不修改 `bb_sources/` 上游源码与既有权重；不删除历史 Release；不读取/修改/提交受保护 `example_files/config_local.yaml`；不触碰可能含用户值的 `example_files/config_local_auth_bridge.yaml`。
- **工作基线与发布基线分开**：
  - 实际工作基线：`main` = `2473e4e20b41ecf750000b751ec1069f9e7bcc62`（`Update LICENSE`，BSD→MIT，已通过 `git merge --ff-only origin/main` 原样保留，未 reset/rebase/覆盖）；`origin` = `git@github.com:0xdevelop/bioBox.git`（由原 `git@github.com:0xdevelop/protenix_build.git` 随用户重命名 GitHub 仓库后同步迁移）。
  - 已发布基线：`v0.0.3` 与移动 tag `latest` 仍指向父提交 `37fb324`（重构前旧命名 `protenix_service` 资产）。历史 `v0.0.2` / `v0.0.3` 验收边界保持原样，不将历史结果算作重构后验证；不预设后续下一版本号（如 `v0.0.4`）。
- **目录与兼容性契约映射**：
  - `protenix_service/` → 仓库根（`main.go`、`resource_cmd.go`、`tools.go`、`go.mod`、`ability/`、`api/`、`cli_proc/`、`common/`、`config/`、`custom_cmd/`、`frontend/`、`policy/`、`runtime_resource/`、`storage/`、`test_ui/`、`example_files/`、`.run/bb_service-debug.run.xml`）。
  - **Go 模块与远端一致**：根 `go.mod` module 与全仓 Go import 同步为 `github.com/0xdevelop/bioBox`（原 `github.com/0xdevelop/protenix_build`，与新 `origin` 一致）；服务管理名/二进制/安装根统一为 `bioBox` / `bb_service` / `/bb_service`。
  - **线协议与鉴权兼容契约**：gRPC proto package 恢复 `protenix_service.api.v1`（`APIService_Call_FullMethodName = "/protenix_service.api.v1.APIService/Call"`，`option go_package` 指向 `github.com/0xdevelop/bioBox/api/api_grpc/protobuf;api_grpc_protobuf`）；`config.DefaultSessionIssuer` / `DefaultSessionAudience` 与 `example_files/config_example.yaml` 的 `auth_cfg.session.{issuer,audience}` 恢复 `"protenix_service"`，与已发布 `v0.0.3` 客户端及既有 token 兼容；MCP 工具名（`test`、`protenix`、`pxdesign`、`task.*`、`resource.*`、`auth.*`、`user.*`）、协议版本 `2026-07-28` 与 `ptxk_` 前缀均保持不变，仅 MCP `Implementation.Name` / `_meta.serverInfo.name` 报告 `config.ProjectName`（`"bb_service"`）。
  - `build_srcs/{protenix,pxdesign}` → `bb_sources/{protenix,pxdesign}`：`.gitmodules` 段名已同步为 `[submodule "bb_sources/protenix"]` 与 `[submodule "bb_sources/pxdesign"]`（spark 上 `.git/config` 与 `.git/modules` 同步对齐）；gitlink 提交保持 `85767b811c40ed46e73a9b39519cf6bfca8701ba` 与 `f788441313c84c3074fe9596ac2433f96b15c763`，子模块工作树干净无改。
  - 构建清单与套件：`bb_build/{pre_build_config_template.yaml,gen_pre_build_config_linux_{amd64,arm64}.yaml}`（`project.name: bioBox`、`prefix_root: bb_prefix`）、`bb_suites/{protenix,pxdesign}/`、`scripts/`（含 `scripts/suites/*`、`scripts/tests/*`）、`docs/bb_{service,build,upstream}/`。
- **套件机制边界与扩展落点（不宣称任意套件/多版本已支持）**：
  - 当前仍为两套硬编码套件：`protenix`（默认 `v1`、`use_msa=false`；`cli_cfg.protenix.version_runtimes.v2.binary` 默认空字符串并在未配置时拒绝启动）与 `pxdesign`（固定 `PINNED_PROTENIX_REF=d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`、`preview`、`use_msa=false`）。
  - 扩展落点（「一份材料、多处点名」）：`bb_build/pre_build_config_template.yaml`（`products[]`）、`bb_suites/<suite>/`、`scripts/suites/<suite>/`、`scripts/build.sh`（`build_protenix`/`build_pxdesign`）、`example_files/install_bb_service.sh`（`RUNTIME_APPS`）、`scripts/install_common.sh`（`case` 分支）。
  - 机制缺口：尚未建立动态多套件加载器或 `bb_suites/<suite>/<version>` 同名多版本目录轴，本轮不擅自扩建通用机制。
- **工具/会话状态与缺失证据（禁止误读）**：
  - **工具/会话状态未知**：本记录不包含 xjb `project_id` / `task_id` / `session_id` 或外部连接器状态；**本文档 Markdown 接续记录不代表 xjb 服务端恢复已验证**。
  - **缺失证据与未审边界**：R1（12:30 前）命令未保留独立 `.log` 文件（不补造）；R2 与 R3 原始日志分别保留于 `tmp/at1-r2-logs/` 与 `tmp/at1-r3-logs/`（均非正式交付附件，R2 原日志未改写）。`bb_suites/*/*.env`、受保护 `example_files/config_local.yaml`、`example_files/config_local_auth_bridge.yaml` 与 `docs/acceptance_plan_official_assets_v0.0.2.md` 正文均不进审查包，保留正文未审边界。
  - **`example_files/config_local_auth_bridge.yaml` 归因记录与条件纠偏结果（2026-11-24 13:29 HKT）**：
    - 既有证据边界不变：R1（`2026-11-24 12:23:13 +0800`）曾对其执行全局 `protenix_service` → `bb_service` 替换，R2/R3 未触碰；spark 上 `7ed7b00c-b73a-4b87-abf1-1351b191405a.txt`（第 1、89–93、256、366–367 行）缺少 R1 前独立工作区 SHA-256、R1 前备份、`7ed7b00c` 命令头与独立落盘替换脚本日志，故迁移前原始用户态归因对独立冷审原属未证明。
    - 用户条件授权与原地纠偏执行（`tmp/at1-auth-correction-logs/01_auth_bridge_conditional_restore.log`，`EXIT_CODE: 0`）：依据用户 `2026-11-24 13:27 UTC+8` 在 DevFull 会话（`f192948d-9c7c-43f0-8d40-cff562ab36c9` pos 286 `t13s3`）的明确条件授权，仅在内存核对原索引 blob `53403c1ccbeb8e0cf4d6fea4d4593a794d8552bc`（`4083` bytes，SHA-256 `d53a8a357f4a3f0d65a9d2669c81893ed3756c2186a2c9a0cb43673259d36623`）与写前工作区（`4041` bytes，SHA-256 `efef987c91d6a20dc4c27de0246bbc359e7a4cb7fe14dd517aeca2b07c1988fe`）全部条件通过后（`exact_byte_replace_match=true`、`diff_line_numbers=[15, 18, 19, 20, 25, 26, 27]` 仅 `storage_cfg.storage_sqlite.path` 与 `storage_cfg.storage_{mysql,postgresql}.{dbname,dbuser,dbpwd}` 7 个键、`extra_diff_keys=[]`、`missing_diff_keys=[]`、`yaml_values_replace_only=true`），加锁（`O_RDWR|O_NOFOLLOW` + `flock`）原地写回原索引内容、未产生额外明文备份；写后 `example_files/config_local_auth_bridge.yaml` 保持新路径与权限 `0664`（inode `10896771`），工作区 SHA-256 恢复为 `d53a8a357f4a3f0d65a9d2669c81893ed3756c2186a2c9a0cb43673259d36623`（`4083` bytes），索引 `git ls-files -s` 仍为 `100644 53403c1ccbeb8e0cf4d6fea4d4593a794d8552bc 0`（`git_diff_vs_index_post=''`，`git status` 由 `RM` 变为纯重命名 `R  protenix_service/example_files/config_local_auth_bridge.yaml -> example_files/config_local_auth_bridge.yaml`）。未执行 `git restore`、`git add`、`commit`、`push` 或部署；正文与凭据值未打印、未导出、未做运行时数据库/bridge 实测验收。
- **本轮二轮（R2）真实实测命令与原始日志（`tmp/at1-r2-logs/`，2026-11-24 UTC+8，原日志未改）**：
  - `tmp/at1-r2-logs/01_gen_frontend_test_ui_api_docs.log`：exit 0（`gen_frontend.sh`、`gen_test_ui.sh`、`gen_api_docs.sh` 输出 25 方法）。
  - `tmp/at1-r2-logs/02_go_test_race.log`：exit 0（`go test -count=1 -race ./...` 全仓 uncached PASS，含 gRPC `conn.Invoke` 线名回归、MCP 工具名/ServerInfo 契约回归及 `TestAuthSessionDefaultsPreservePublishedContract`；**后者仅验证配置默认/示例字符串常量等于 `"protenix_service"`，不等于旧 token 签验互通实测**）。
  - `tmp/at1-r2-logs/03_go_build_and_cross_cwd.log`：exit 0（`go build -o tmp/bb_service_check .`、仓内 `version`、`/tmp` 跨 cwd `version` 及 `/tmp` 跨 cwd `gen_pre_build_config.sh validate`）。**准确限定：跨 cwd 配置 `validate` 通过；从 `/tmp` 运行 Debug 模式 `version` 时 `Git 提 交 Hash :  fatal: 不`、`Git提交时间 UTC:` 为空，debug `version` 的 git 身份未跨目录保持（不因复合命令 `exit 0` 而宣称 `/tmp` 下版本 Git 身份有效）**。
  - `tmp/at1-r2-logs/04_release_and_script_tests.log`：exit 0（`gen_release_assets.go self-test` 58 项、`test_release_cleanup.py` 12 项、`test_release_guards.py`、`test_source_staging.sh`、`check_install_update_restore.sh` `bwrap` 隔离测试、全量 `.sh` `bash -n`）。
  - `tmp/at1-r2-logs/05_native_arm64_pack_and_smoke.log`：exit 0（`check_native_build.sh arm64`、`FORCE_PACK=1 bash scripts/build.sh pack release linux_arm64`、`check_bundle_resources.sh` 及 `BUILD_INFO.txt` 校验；`dirty=1` 脏工作树产物非正式交付）。
  - `tmp/at1-r2-logs/06_gitmodules_and_links.log`：exit 0（`.gitmodules` `bb_sources/{protenix,pxdesign}` URL 解析与 `git submodule status` / gitlink SHA 验证）。
- **三轮最小收敛（R3）增量改动与新增日志（`tmp/at1-r3-logs/`，2026-11-24 UTC+8）**：
  - `docs/bb_upstream/package_protenix.md:23` 与 `docs/bb_upstream/package_pxdesign.md:23`：移除仍可复制执行的 `WEIGHTS_REUSE_FROM=... ./install_<app>.sh install` 失效命令，改为说明在整合包 `runtime/<app>/conf/<app>.env` 改写数据根路径或使用既有目录软链接并由 `bb_service resource prepare` 校验就位（保留退役与 exit 4 边界）。
  - `tmp/at1-r3-logs/01_jwt_behavior_test.log`：`go test -v -count=1 -race ./api/api_auth/api_auth_api_key -run TestPublishedSessionIssuerAudienceJWTBehavior` 与 `./config -run TestAuthSessionDefaultsPreservePublishedContract` PASS；离线验证 `DefaultSessionIssuer`/`DefaultSessionAudience`（`"protenix_service"`）在隔离 SQLite 下签发、解析及 `AuthenticateRequest` 验签放行，并拒绝 `"bb_service"` issuer/audience token。**明确限定：未对已发布 `v0.0.3` 实产密钥或真实历史既有 token 做重放测试**。
  - `tmp/at1-r3-logs/02_r3_docs_and_links_check.log`：R3 文档相对链接、失效 `WEIGHTS_REUSE_FROM=` 可执行命令扫描及受保护文件哈希核对。
- **未提交用户文件保护清单**：
  - `example_files/config_local.yaml`（原 `protenix_service/example_files/config_local.yaml`）：未读、未改、未提交，SHA-256 保持 `684b906075c72cd76806cafd645bb7da9c6ac6b235550b5e77eaf15f0e31bf6c`（3,079 bytes）。
  - `example_files/config_local_auth_bridge.yaml`：已凭用户 `2026-11-24 13:27 UTC+8` 条件授权并验证全部 7 键机械替换条件后，原地恢复原索引 blob 内容（SHA-256 `d53a8a35...`，`4083` bytes）并保留新路径与权限 `0664`，索引未变（`R  protenix_service/... -> example_files/...`）；正文未审未导出，仍未做提交/部署/运行时验收。
  - `docs/acceptance_plan_official_assets_v0.0.2.md`：原样保留未改动。
  - `README.md` 中原有[官方资产验收计划（v0.0.2）](../acceptance_plan_official_assets_v0.0.2.md)链接：保留于源工作区，不混入 AT-1 导出 patch。
  - 仓库内既有 gitignored `prefix/`：未改动、未删除。
- **未覆盖范围、下一步与禁止恢复候选路线**：
  - 未覆盖：Git 提交与推送、GitHub Actions 双原生 runner（`amd64` + `arm64`）与 `linux/amd64` 打包、正式 `dirty=0` 发版、目标机 root 安装 `/bb_service` 与 systemd 常驻、真实 GPU `v1/no-MSA` 与 `preview/no-MSA` 推理、独立包协调与安装闭环、实产 `v0.0.3` 历史密钥/既有 token 离线重放。
  - 下一步：由 DevFull 对 `/workspace/biobox-at1-auth-correction/` 进行只读元数据复核；用户排除受保护 `example_files/config_local.yaml` 等本地文件后，再由用户决定是否提交代码、运行 Actions 与后续发版/装机验收。
  - **禁止恢复候选路线**：严禁恢复任何分支 artifact 候选发布路线、绕过三段工作流（`release` → `Build` → `RemoveOldRelease`）的影子发布入口，或重新把单产品包/额外清单挂回公开 Release 附件。
## AT-1 层次化验收分类与 PXDesign 离线一致性切片（2026-11-24 13:45 HKT，只读增量分类）

- **现场基线与并发核查**：`spark-901f` `/home/certa/workspace/bioBox`（原 `/home/certa/workspace/protenix_build`）唯一写入者；核查无在途构建/测试/服务/uv 进程；`HEAD` = `2473e4e20b41ecf750000b751ec1069f9e7bcc62`（`main`，`Update LICENSE`），R1–R3 与 `auth_bridge` 条件纠偏均保留在未提交工作区，未执行 `stage/commit/push/tag/release/install/GPU`。

### 1. 分层状态矩阵（不将局部缺口统称“整体未验收”）

| 层级 | 范围与关键标识 | 已验证事实与复用/新增证据 | 未验证 / 明确限定边界 |
| --- | --- | --- | --- |
| **1. 程序集自身** | `bb_service` Go 主程序（module `github.com/0xdevelop/bioBox`，原 `github.com/0xdevelop/protenix_build`）、`protenix_service.api.v1` gRPC、JWT 默认 `"protenix_service"`、MCP、前端/test_ui/API 文档 | R2 `01`/`02`/`03`（`gen_frontend`/`gen_test_ui`/`gen_api_docs`、`go test -count=1 -race ./...`、`go build`）与 R3 `01`（隔离 SQLite 下 JWT 签发/解析/拒绝 `"bb_service"`）均 exit 0 | ① **本地 Go `1.27.4` ≠ CI `go.mod` `1.26.10` 已验证**；② `/tmp` 跨 cwd 运行 debug `version` 时 Git 字段为 `fatal: 不`（debug git 身份未跨目录保持）；③ `config_local_auth_bridge.yaml` 已完成条件原地纠偏与元数据收口（`d53a8a35...`），**bridge 业务运行与实产 `v0.0.3` 历史 token 重放未验** |
| **2. Submodule 源码与固定 Ref** | 顶层 `bb_sources/protenix` gitlink `85767b811c40ed46e73a9b39519cf6bfca8701ba`；`bb_sources/pxdesign` gitlink `f788441313c84c3074fe9596ac2433f96b15c763`；PXDesign 独立固定 `PINNED_PROTENIX_REF=d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`、`PXDBENCH_REF=f6d0d72496c23ca1374df94374f3454ae8c549`、`COLABDESIGN_REF=8fd167defd1b0f98e29f00584ff4f3987e6935dc` | R2 `06` 验证 `.gitmodules` 与 gitlink；**本切片新增 `01_pxdesign_cross_layer_offline_check.log`**：离线核实 `bb_sources/protenix` 本地对象库 `git tag --points-at d18aa1...` 为 `v0.5.0+pxd`（与 `bb_build/*.yaml` / `BUILD_INFO.txt` 字面一致）；`tmp/` 隔离验证 `pxdesign-argparse-percent.patch` 对 `pxdesign/pxd_server/server_{extended,preview}_mode.py` 可干净应用与逆向校验；确认 `pxdbench-json-float.patch` 独立作用于安装后的 `site-packages/pxdbench/tools/af2/main_af2_{complex,monomer}.py` | ① **未联网 `git fetch` 核对远端 `bytedance/Protenix.git` 标签与 `d18aa1...` 对应**；② 未实际构建新 venv 以实装验证 `pxdbench` 的 `site-packages` 补丁路径；③ 未在 GPU 上执行上游算法科学行为 |
| **3. 集成构建与打包** | `bb_build/*.yaml`、`scripts/build.sh`、`scripts/check_*.sh`、`scripts/gen_release_assets.go`、`build/upload_tmp_dir/*_v0.0.3.zip`、`.github/workflows/release.yml` | R2 `04`/`05`（58 项 manifest 自测、`bwrap` 安装/升级/回滚、`bash -n`、`linux_arm64` 打包与资源门禁）；**本切片新增 `01` 与 `02`**：隔离对比 `pxdesign_linux_arm64_release_v0.0.3.zip` 与 `bb_service_linux_arm64_release_v0.0.3.zip` 下 `runtime/pxdesign` 零差异；打包 `src/pxdesign` 与 gitlink `f788441...` `git archive` 零差异；`SRC_SHA`、`BUILD_INFO.txt`、`bb_suites/pxdesign/{deps.txt,patches/*}` 一致；`protenix` zip `src_sha` = `85767b81...`；全部 zip `venv_entries=0`、`model_weight_entries=0` | ① 当前 arm64 zip 为 `dirty=1`（`2473e4e...-dirty`），**`dirty=0` 正式包未打**；② **`linux/amd64` 原生构建、Actions 双架构（Go `1.26.10`）及三段式 CI 发布未跑**；③ **发现未决语义矛盾（见下）** |
| **4. 依赖 / 缓存 / 持久资源（四者严格分开）** | 旧开发 `prefix/`（`protenix/venv` `6.3G`、`pxdesign/venv` `1.5G`）；`uv` 缓存 `/home/certa/.cache/uv`（`6.7G`）；载荷归档缓存 `~/.resource-archives`（当前 `ABSENT`）；持久权重/CCD（`prefix/protenix/data` `2.0G`、`prefix/pxdesign/{release_data,tool_weights,protenix_data}` `2.9G+782M+549M`）；`bb_prefix`（`ABSENT`） | **本切片新增 `02_nonprod_resource_metadata_check.log`**（仅非秘密体量/存在性 metadata）：确认 `/` 可用 `3.4 TiB`（`/home/certa` 与 `/tmp` 同盘），`NVIDIA GB10` + `CUDA 13.0` + `uv 0.13.7` 就绪；公开 `example_files/config_example.yaml` 清单共计 16 项 `6,571,265,681` bytes（`6.12 GiB`，Protenix 5 项 `1.98 GiB` 全有 URL，PXDesign 11 项 `4.14 GiB` 中 9 项有 URL、2 项 AF2 无 URL）；发布 zip 明确不含 `venv` 与模型权重；`install_<app>.sh` 独立包无整合包协调器时按原契约 `exit 4`（**非本次新增源码 bug**） | ① **复用旧 `prefix/*/venv` ≠ 新 `venv` 验收**，**缓存复用（`~/.cache/uv`）≠ 源码/依赖锁验收**，**持久权重/CCD ≠ `.resource-archives` 载荷缓存**；② 尚未在隔离 `bb_prefix` 或非生产根执行新 Python 3.14 `venv` 重建；③ 若非生产验证直接经 `.env` 指向现有 `prefix/` 权重目录，**权重下载 = 0 bytes**（仅缺省未命中缓存的 wheel 与 git-pinned 依赖可能需要联网）；若从零重拉公开资源则最多约 `6.12 GiB` 且 AF2 两项仍必须本地复用 |
| **5. 成套交付运行（多子层）** | `5a` 独立包与整合包协调兼容性；`5b` `/bb_service` root 安装与 systemd 常驻；`5c` 真实 GPU 推理（`protenix v1/no-MSA`、`pxdesign preview/no-MSA`）；`5d` `auth_bridge`/真实库/线协议联调 | `bwrap` 隔离安装/升级/回滚脚本结构校验已过（`04`）；`resource verify/prepare` 离线单测已过（`02`） | `5a`–`5d` 实机装机、独立包协调闭环、真实 GPU 推理产物与 bridge 运行态**均未实际执行** |
| **6. xjb 独立协作面** | 外部 xjb `project_id` / `task_id` / `session_id` 与连接器运行态 | 仅知与本仓独立，本记录不承载其服务端状态 | 服务端恢复与连接器状态**未知**（**文档接续记录不等于 xjb 恢复已验证**） |

### 2. 未决语义矛盾（仅记录并给出最小修正建议，本切片未改源码）

- **原发现 `WEIGHTS_REUSE_FROM` / `scripts/weights_reuse.sh` 矛盾（已于 13:58 最终离线切片消解）**：已清理 `scripts/build.sh` 失效宣传/透传与 `weights_reuse.sh` 打包拷贝，并同步 `README.md:37`、`docs/bb_service/design.md:26` 与 `bb_suites/*/deps.txt` 注释；`install_prefix_*.sh` 对非空 `WEIGHTS_REUSE_FROM` 的明确 retired 拒绝保留，`scripts/weights_reuse.sh` 源文件保留未删。

### 3. 安全验证入口分层（不执行有后果动作）

- **可继续的本地离线入口（无需 root / 不碰 `/bb_service` / 不跑 GPU）**：
  - `go test -count=1 -race ./...`（Go 单测与线协议/JWT 回归）
  - `bash scripts/check_native_build.sh arm64` / `python3 -m unittest`（脚本与发布守卫单测）
  - `unzip` / `git archive` / `patch --dry-run` 隔离 `tmp/` 校验（已在 `tmp/at1-layer-check-logs/` 覆盖）
- **需非生产新 env / 隔离测试根 / 网络与 GPU 的入口（无需改正式 `/bb_service`）**：
  - 使用独立测试根（如 `bb_prefix` 或 `/tmp/` 测试目录）+ 解压包 `runtime/<app>/conf/<app>.env` 指向现有 `prefix/` 持久权重（0 权重下载），执行非 root `SKIP_WEIGHTS=1` 或协调 `resource prepare/verify` 验证新 Python 3.14 `venv`、`pxdbench` `site-packages` 补丁与 GPU `torch`/`jax` import 及 `v1/no-MSA`、`preview/no-MSA` 冒烟。
- **需正式前置门禁 / root 或 CI 权限的入口**：
  - 排除未提交本地配置后完成提交使工作树 `dirty=0` → GitHub Actions 双原生（`linux/amd64` + `linux/arm64`，Go `1.26.10`）正式打包/发版；目标机 `sudo` 安装 `/bb_service` 与 `systemd` 常驻验收。
## AT-1 仓库与本地目录迁移收口（`0xdevelop/bioBox`，2026-11-24 UTC+8）

- **授权来源**：用户 `2026-11-24 13:44 UTC+8`（`t206u` / DevFull 会话 `pos391` `t18u`）明确授权将本地工作区改名为 `bioBox` 并同步 `git@github.com:0xdevelop/bioBox.git` 与 Go module / 活跃工程身份。仍保持不执行 `git add` / `commit` / `push` / `tag` / `release` / `sudo` / 系统安装 / GPU 运行，不触发 `workflow_dispatch`。
- **新旧映射（历史证据保留原路径不篡改）**：
  - GitHub 远端：`0xdevelop/protenix_build` → `0xdevelop/bioBox`（`git@github.com:0xdevelop/bioBox.git`，`id=1452536016`，`main` = `2473e4e20b41ecf750000b751ec1069f9e7bcc62`）。
  - 本机工作区：`/home/certa/workspace/protenix_build` → `/home/certa/workspace/bioBox`（原地 `mv`，非重新 clone/reset，保留全部未提交改动、索引、`HEAD`、`bb_sources/*` 子模块与 `prefix/` 权重；历史 `tmp/at1-{r2,r3,auth-correction,layer-check}-logs/` 保留迁移前原始输出不变）。
  - Go module 与生成链：`go.mod`、`83` 个引用内部包的 `.go` 文件（含 `scripts/gen_api_docs.go`）、`api/api_grpc/proto/api.proto`（`option go_package`）、`scripts/gen_proto.sh`（`MODULE`）、经 `protoc` 重新生成的 `api/api_grpc/protobuf/api.pb.go`（`rawDesc` 描述符同步更新为 `github.com/0xdevelop/bioBox/api/api_grpc/protobuf;api_grpc_protobuf`）、`.run/bb_service-debug.run.xml`、`scripts/tests/test_release_guards.py` 与 `README.md` 同步为 `github.com/0xdevelop/bioBox` / `0xdevelop/bioBox`。
  - **保留不变项**：`protenix_service.api.v1` gRPC wire package、JWT 默认 `"protenix_service"`、`ptxk_` 与 MCP 工具名；`bb_sources/{protenix,pxdesign}` 上游 gitlink/remote 与科学参数；历史 `docs/bb_service/show_yeah_data_contract.md`、历史验收文档与原始日志；受保护 `example_files/config_local.yaml`（`684b9060...`）、已纠偏 `example_files/config_local_auth_bridge.yaml`（`d53a8a35...`）与 `docs/acceptance_plan_official_assets_v0.0.2.md`（`de1dafe1...`）。
- **迁移后新目录验证日志（`tmp/at1-repo-rename-logs/`）**：
  - `01_pre_move_and_remote_check.log`（exit 0）：移动前零活跃锁、`/home/certa/workspace/bioBox` 无冲突、远端 `git@github.com:0xdevelop/bioBox.git` `HEAD`=`2473e4e2` 与受保护文件 SHA-256 核对。
  - `02_post_move_and_remote_update.log`（exit 0）：原地移动与 `origin` 更新后 `HEAD`、索引、`254` 行 porcelain、`bb_sources/*` 子模块与受保护文件 SHA-256 完全一致。
  - `03_rg_protenix_build_pre_edit.txt`：改码前全仓非秘密命中清单（`385` 处 / `99` 文件）。
  - `04_gen_proto.log`（exit 0）：`scripts/gen_proto.sh` 重生 `api.pb.go` 与 `api_grpc.pb.go` 校验。
  - `05_post_rename_tests_and_arm64_pack.log`（exit 0）：新目录 `/home/certa/workspace/bioBox` 下 `gen_frontend.sh`、`gen_test_ui.sh`、`gen_api_docs.sh`、`go test -count=1 -race ./...`、`test_release_guards.py`、`test_release_cleanup.py`、`go build` + `/tmp` 跨 cwd、`check_native_build.sh arm64`、`FORCE_PACK=1 bash scripts/build.sh pack release linux_arm64` 及 `check_bundle_resources.sh` 实测全部通过。
## AT-1 最终离线收尾：语义矛盾消除、终包验证与全仓残留分类（2026-11-24 13:58 HKT）

- **`254 -> 255` `__pycache__` 时间差事实**：`2026-11-24 13:50 HKT` 在 `05_post_rename_tests_and_arm64_pack.log` 运行 `python3 scripts/tests/test_release_cleanup.py` 时产生未跟踪 `scripts/tests/__pycache__/test_release_cleanup.cpython-312.pyc`，使 `git status --porcelain` 由 `254` 增至 `255`；该缓存未删除也未暂存，后续 Python 检查均使用 `PYTHONDONTWRITEBYTECODE=1`，且既往导出包保持原样不改。
- **最小语义矛盾收尾（不改依赖 pin、科学参数、上游 submodule 与敏感配置，不删死脚本源文件）**：
  - `scripts/build.sh`：移除顶部/usage 中失效的 `WEIGHTS_REUSE_FROM` 说明与透传（并将默认 prefix 描述由 `<repo>/prefix` 对齐为 `<repo>/bb_prefix`），从 `pack_one_product` 移除 `scripts/weights_reuse.sh` 检查与打包拷贝；**保留** `scripts/weights_reuse.sh` 源文件及 `scripts/suites/{protenix,pxdesign}/install_prefix_*.sh` 对非空 `WEIGHTS_REUSE_FROM` 的明确 retired 拒绝。
  - `bb_suites/{pxdesign,protenix}/deps.txt`：仅将头部注释中的旧 `deps/*.txt` / `prefix/*` 口径同步为 `bb_suites/*/deps.txt` / `bb_prefix/*`，依赖版本与参数零改动。
  - `README.md:37` 与 `docs/bb_service/design.md:26`：移除对 `weights_reuse` / `weights_reuse.sh` 打包项的过期列举。
- **本轮最终 `arm64` `dirty=1` 开发包实测与 PXDesign 跨层校验（`tmp/at1-final-offline-logs/`，不引用 `12:52` 旧包）**：
  - `02_weights_reuse_retired_and_build_help_check.log`（exit 0）：`bash -n`、`scripts/build.sh --help` 不再宣传 `WEIGHTS_REUSE_FROM`、隔离 fixture 验证 `install_prefix_{protenix,pxdesign}.sh` 遇到非空 `WEIGHTS_REUSE_FROM` 均以 exit 1 明确拒绝（`WEIGHTS_REUSE_FROM is retired...`），`scripts/weights_reuse.sh` 仍在仓。
  - `03_final_arm64_pack_and_pxdesign_consistency.log`（exit 0）：`2026-11-24 13:58 HKT` 重打终包并完成资源门禁与隔离对比：
    - `bb_service_linux_arm64_release_v0.0.3.zip`：`96,545,525` bytes，SHA-256 `58c7b9fbe463b813d5f0ddb59c827a9cdcb1620f3b3d8f347fb3431bb0e4bbe5`
    - `protenix_linux_arm64_release_v0.0.3.zip`：`67,983,615` bytes，SHA-256 `df1001c86b91f1ed51ea79b75c2e34f31c6cf7d0ebacf7300c1240c11d3f9f13`
    - `pxdesign_linux_arm64_release_v0.0.3.zip`：`16,626,437` bytes，SHA-256 `25fcb457ac28582d0d9c944a2be9c9a901071ac7542d643fb4f1d2e53aaa1b7a`
    - 三包均 `weights_reuse_entries=0`、`venv_entries=0`、`model_weight_entries=0`、`BUILD_INFO` 为 `project=bioBox` / `commit=2473e4e20b41ecf750000b751ec1069f9e7bcc62-dirty` / `dirty=1` / `build_time=2026-11-24_13:58:17`；`tmp/` 隔离验证 `pxdesign` 单包与整合包 `runtime/pxdesign` 零差异、`src/pxdesign` 与 `git archive f788441313c8` 零差异、`deps`/`patches`/`SRC_SHA` 一致、`pxdesign-argparse-percent.patch` 可逆 dry-run 通过。
- **改名后非秘密全仓残留分类（`04_post_rename_nonsecret_residual_scan.log`, exit 0，排除 `config_local*`/`*.env`/权重/cache/`tmp`/`bb_sources`）**：
  - `protenix_build`：活跃代码/脚本/workflow/`.run`/`README.md` **0 残留**；仅 `9` 份历史文档/规则（`51` 处）与 `docs/bb_build/release_delivery.md` 明确旧→新映射说明（`8` 处）。
  - `protenix_service`：全部为兼容线协议/JWT 契约及验证测试（`10` 文件 `42` 处）、`README.md`/`release_delivery.md` 兼容与映射说明（`2` 文件 `19` 处）、历史文档/changelog（`12` 文件 `277` 处）与历史来源注释/`.gitignore`（`3` 文件 `3` 处）。
## AT-1 隔离非生产新环境验证计划与记录（`protenix` 套件，2026-11-24 UTC+8）

- **选型理由**：在 `protenix` 与 `pxdesign` 两套中优先选择 **`protenix`**（依赖链更轻量：无 `colabdesign`/`jax[cuda13]`/`dm-haiku`/`optax`/`pxdbench` 多仓库 git 依赖，仅 `torch==2.14.1+cu130`、`torchvision==0.29.1+cu130`、`cuequivariance-ops-torch-cu13` 与 `bb_suites/protenix/deps.txt` 覆盖，配合 `bb_sources/protenix@85767b811c40ed46e73a9b39519cf6bfca8701ba`）。
- **唯一隔离目标路径**：`/home/certa/workspace/bioBox/tmp/at1-nonprod-protenix-env/sandbox`（其下 `bb_service`、`conf/config.yaml`、`runtime/protenix/`；完全独立于旧 `prefix/`、正式 `bb_prefix/` 与 `/bb_service`）。
- **拟执行准确命令（建 env 前预先登记）**：
  1. 编译隔离二进制与布置包装器：`go build -o tmp/at1-nonprod-protenix-env/sandbox/bb_service .`，复制 `example_files/config_example.yaml` 至 `sandbox/conf/config.yaml`（仅将 `cli_cfg.protenix.binary` 定向 `./runtime/protenix/bin/protenix-run`，保持 `download_enabled: false` 与全部 5 项 SHA-256 校验），复制 `bb_suites/protenix/protenix-run` 至 `sandbox/runtime/protenix/bin/protenix-run`。
  2. 创建新 Python 3.14 venv 与依赖安装验证：`PROTENIX_PREFIX=/home/certa/workspace/bioBox/tmp/at1-nonprod-protenix-env/sandbox/runtime/protenix SKIP_WEIGHTS=1 bash scripts/suites/protenix/install_prefix_protenix.sh`（自动执行 `uv venv --python 3.14 --seed`、`torch`/`build_tools`/`protenix` 依赖安装、`torch.cuda.is_available()` 与 GB10 矩阵乘 `verify_import`、`protenix --help` 与 `pip-freeze.txt` 导出）。
  3. 权重只读复用与资源绑定/缺失区分核查：仅改写 `sandbox/runtime/protenix/conf/protenix.env` 中 `PROTENIX_ROOT_DIR=/home/certa/workspace/bioBox/prefix/protenix/data`（及隔离 `RUNTIME_CACHE_DIR` 指向 `sandbox/runtime/protenix/data/cache`），随后执行 `sandbox/bb_service resource status protenix.v1`、`sandbox/bb_service resource --runtime sandbox/runtime/protenix/bin/protenix-run verify protenix.v1`、`sandbox/runtime/protenix/bin/protenix-run --help`，以及未安装/缺失对照验证（`pxdesign.preview` 与空资源目录对照）。
- **预计副作用边界**：
  - 写入范围仅限 `/home/certa/workspace/bioBox/tmp/at1-nonprod-protenix-env/`、日志目录 `tmp/at1-nonprod-protenix-logs/` 及 `/home/certa/.cache/uv`（允许复用已有 `uv` 缓存并按项目原锁定依赖联网补齐缺失 wheel）；
  - 不修改旧 `prefix/`、`bb_prefix/`、`/bb_service`、系统 Python、`bb_sources/`、科学参数/依赖 pin、受保护 `config_local*.yaml` 或 `auth_bridge`；不重下模型或大数据库，不跑完整推理/常驻服务/提交/发版。
- **实际执行结果（2026-11-24 14:05–14:10 HKT，全部 `EXIT_CODE: 0`）**：
  - `tmp/at1-nonprod-protenix-logs/01_preflight_readonly_check.log`：确认 `aarch64`、Python `3.14.10`、`CUDA 13.0`（`V13.0.88`）、`NVIDIA GB10`（驱动 `580.178.04`）、`/` 可用 `3.4 TiB`、`bb_sources/protenix` 3 个受检 `.py` 文件 SHA-256 与 `runtime_resource/installed.go` 一致、`prefix/protenix/data` 5 项文件大小匹配。
  - `tmp/at1-nonprod-protenix-logs/02_isolated_protenix_install.log`（耗时约 4 分钟）：在 `tmp/at1-nonprod-protenix-env/sandbox/runtime/protenix/venv` 成功创建全新 Python `3.14.10` venv（`6.3G`，`130` 项冻结依赖）；实测 `torch==2.14.1+cu130`、`torchvision==0.29.1+cu130`、`cuequivariance==0.13.0`、`biotite==1.4.0`、`rdkit==2026.9.2`、`gemmi==0.7.5`、`deepspeed==0.20.0`、`numpy==2.4.6` 导入及 GB10 `(1024, 1024)` CUDA 矩阵乘与 `protenix --help` 均通过；`/home/certa/.cache/uv` 复用既有 `~13.79 GiB` 缓存并增量新增 `156,935,196` bytes（`~149.7 MiB`，含 `numpy`/`wandb`/`pydantic-core` 等）。
  - `tmp/at1-nonprod-protenix-logs/03_resource_binding_and_wrapper_check.log`：
    - **缺失对照**：空 `sandbox/runtime/protenix/data` 下 `resource status` 返回 `state="download_disabled", file=4`（exit 0），`resource verify` 拒绝（exit 3），`protenix-run --help` 返回 `exit 4`；同沙箱内未安装的 `pxdesign.preview` 返回 `state="download_disabled", file=10` 与 `verify` 失败（exit 1）。
    - **只读复用就绪**：将 `PROTENIX_ROOT_DIR` 指向 `/home/certa/workspace/bioBox/prefix/protenix/data`（零复制，原文件 mtime 保持 `2026-11-11 16:21`）后，`bb_service resource status protenix.v1` 与 `bb_service resource --runtime .../protenix-run verify protenix.v1` 均返回 `state="ready"`（通过 3 项 `ValidateInstalled` 源码 SHA-256 与 5 项权重/CCD SHA-256 校验），`protenix-run --help` exit 0。
  - **未覆盖边界保持**：本轮未执行完整 GPU 推理任务（`pred` 产物）、未验证独立非生产 `pxdesign` 新 venv 实装、未触及 `/bb_service` / `bb_prefix` / `systemd` / Git 提交 / Actions / 发版。
## AT-1 隔离非生产真实 `protenix-run pred` 验证（2026-11-24 14:15–14:19 HKT）

- **运行边界与输入/权重身份**：
  - 使用上一步建立的隔离沙箱 `tmp/at1-nonprod-protenix-env/sandbox`（Python `3.14.10` venv + `bb_service` 协调器 + 只读复用 `prefix/protenix/data`），未装 `pxdesign`、未并行任务、未改任何依赖 pin 或科学参数。
  - 既有仓内示例输入：`bb_sources/protenix/examples/input.json` 复制到 `tmp/at1-nonprod-protenix-pred-20261124-1416/input.json`（`988` bytes，SHA-256 `3bfe91c73117f3f234891e61945bda0ff361ae41d0d4068ad96e40c40a9bc67d`，单链 `user_test` `693` aa）。
  - 源码与权重身份：`bb_sources/protenix` = `85767b811c40ed46e73a9b39519cf6bfca8701ba`；`PROTENIX_MODEL=protenix_base_default_v1.0.0`（`368.48M` 参数，checkpoint SHA-256 `2b7d5a8b30494514fc47fd2271a16260528cdba170ba09cc112fdecd8f85ec04`）；`cycle=10`、`step=200`、`sample=5`、`dtype=bf16`、`LAYERNORM_TYPE=torch`。
- **实测日志与结果（`tmp/at1-nonprod-protenix-pred-logs/`，均 `EXIT_CODE: 0`）**：
  - `01_preflight_example_and_gpu_check.log`：确认 `NVIDIA GB10` 空闲（`0%` 利用率，`11W`，`42°C`，仅既有空闲 ComfyUI `170MiB` 驻留）、沙箱 `bb_service resource verify protenix.v1` 返回 `state="ready"`。
  - `02_contract_fail_closed_check.log`：省略 `--use_msa false` 时 `protenix-run pred` 在启动 Python 前以 **`exit 4`** 明确拒绝，未创建输出目录。
  - `03_isolated_protenix_pred.log` & `03_gpu_sampling_during_pred.log`：执行 `protenix-run pred -i .../input.json -o .../output --use_msa false -s 101`，总墙钟 `173s`（`Model forward time: 131.80s`、`Seed 101 completed in 136.34s`），`EXIT_CODE: 0`；5s 采样显示 GB10 GPU 利用率持续 `95–96%`（功耗 `47–53.33W`，温度峰值 `67°C`），结束后回落至 `0%`。
  - `04_output_structure_and_post_check.log`：无 `output/ERR/`；产出 `5` 份 mmCIF（`user_test_sample_{0..4}.cif`，`542–553 KB`，经 `gemmi` 解析均为 `models=1, chains=['A'], residues=693, atoms=5370`）与 `5` 份置信度摘要 JSON（`plddt` `33.27–33.97`、`ptm` `0.244–0.252`、`ranking_score` `0.049–0.050`、`has_clash=false`）；`RUNTIME_CACHE_DIR`（`3.7M`）留在沙箱内，`prefix/protenix/data` 5 项文件 mtime 仍保持 `2026-11-11 16:21`。
- **明确限定（不拿产物存在当科学准确性已证）**：本切片为单输入（`--use_msa false`、无 ESM）非生产隔离链路与产物结构可解析验证；无参考基准对照，**未证明科学准确性**；亦未覆盖 `pxdesign` 隔离 env/GPU、`bb_service` 异步 HTTP/gRPC/MCP 入队调度、`/bb_service` root/systemd 安装与 `dirty=0` CI 发版。
- **证据安全收敛与派生脱敏记录（2026-11-24 14:25 HKT）**：
  - 源侧原始日志 `tmp/at1-nonprod-protenix-pred-logs/01_preflight_example_and_gpu_check.log`（`20,914` bytes，SHA-256 `ba9cfc0f5cbe8e68261b3048f8e564c2d41c564ba43cf4b6d98d017244eccd64`）第 `4–5` 行为 `ps` 完整进程参数行（其中第 `5` 行含仍在运行中的 `cursor-agent` 工作进程 `--api-key` 参数）；`release_delivery.md` 本身、其余 `logs/02–05` 及 `summary_confidence` JSON 经程序布尔扫描均未包含该进程行或凭据值。
  - 已在不删除原件的前提下收敛源侧权限（`tmp/at1-nonprod-protenix-pred-logs/` `0700`、其下日志 `0600`），并由程序在内存将第 `4–5` 行整行替换为 `[REDACTED_PROCESS_ARGUMENTS]` 生成**全新派生脱敏副本** `tmp/at1-nonprod-protenix-pred-redacted-logs/01_preflight_example_and_gpu_check.redacted.log`（`20,057` bytes，SHA-256 `4c8aa2bf200dc96c17a92d2fd43d02577d5f71cbc098acc24174fa14974d656b`，不冒称原件，不记录凭据值或凭据哈希）。
  - 旧未脱敏共享归档已停止分享（Box 旧 `pred` 目录留待复核侧处理，AT-1 不再双写）；新派生脱敏审计与状态见 `tmp/at1-nonprod-protenix-pred-redacted-logs/06_redaction_and_containment_audit.log`。该 `--api-key` 当前有效性与实际撤销状态**未知**（禁止探活/读密/停进程/自动轮换）；关联 `PID 3639002`（`comm=MainThread`、`exe=node`）仍在运行，正规处置需持有者在 Cursor 账户/管理控制台核查并撤销/轮换对应 API key 后重启该 worker 进程。后续进程检查仅允许 `pid`/`comm`/`exe`，禁止完整 `ps args`/`env`。
## AT-1 隔离非生产 `pxdesign` 新 env 安装与只读资源绑定验证（2026-11-24 14:30 HKT）

- **执行前登记（命令、目标与预期副作用）**：
  - **独立目标目录**：`/home/certa/workspace/bioBox/tmp/at1-nonprod-pxdesign-env/sandbox`（含 `bb_service`、`conf/config.yaml` 及 `PXDESIGN_PREFIX=.../sandbox/runtime/pxdesign`），不复用旧 `venv`、不改 `prefix/` 或正式 `bb_prefix/`、不安装额外依赖或修改 pin。
  - **安装命令**：`PXDESIGN_PREFIX=/home/certa/workspace/bioBox/tmp/at1-nonprod-pxdesign-env/sandbox/runtime/pxdesign SKIP_WEIGHTS=1 bash scripts/suites/pxdesign/install_prefix_pxdesign.sh`
  - **预期副作用**：仅在 `sandbox/runtime/pxdesign` 内创建 Python `3.14` 独立 `venv`、导出并补丁 `src/pxdesign-f788441313c8`、按 `bb_suites/pxdesign/deps.txt` 固定 pin 安装 `torch==2.14.1+cu130`、`Protenix@d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`、`PXDesignBench@f6d0d72496c23ca1374df5943754f3454ae8c549`、`ColabDesign@8fd167defd1b0f98e29f00584ff4f3987e6935dc`、`jax[cuda13]` 并应用 `pxdbench-json-float.patch`，执行内置 GPU import/matmul 自检与 `pxdesign pipeline --help`，生成 `conf/pip-freeze.txt`；允许 `/home/certa/.cache/uv` 因依赖 cache miss 正常增量写入；不下载任何模型/AF2/大数据库，不修改旧 `prefix/pxdesign`、系统 Python 或 `/bb_service`。
  - **执行结果（`tmp/at1-nonprod-pxdesign-logs/`，`EXIT_CODE: 0`）**：
    - `01_preflight_readonly_check.log`：确认 `NVIDIA GB10` 空闲（`0%` 利用率，`11.38W`，`44°C`，仅既有空闲 `ComfyUI` `170 MiB`），无冲突在途任务（仅 `pid/comm/exe` 检查），并核实 `prefix/pxdesign/` 下 11 项资源（4 checkpoint + 3 `protenix_data` + 2 AF2 + 2 MPNN）均已有 `2026-11-11` 本地快照。
    - `02_isolated_pxdesign_install.log`（`ELAPSED_SECONDS: 413`，`EXIT_CODE: 0`）：`status=environment_only src_sha=f788441313c84c3074fe9596ac2433f96b15c763`；安装 `151` 个冻结包（含 `torch==2.14.1+cu130`、`triton==3.8.0`、`jax==0.12.1`、`jaxlib==0.12.1`、`jax-cuda13-plugin==0.12.1`、`jax-cuda13-pjrt==0.12.1`、`dm-haiku==0.0.17`、`optax==0.2.9`、`biotite==1.0.1`、`biopython==1.88`、`modelcif==0.7`、`matplotlib==3.10.9`、`numpy==2.4.6`、`protobuf==6.33.6`、`transformers==4.51.3`、`deepspeed==0.20.0`、`rdkit==2026.9.2`、`protenix@d18aa1da...`、`pxdbench@f6d0d724...`、`colabdesign@8fd167de...`、`pxdesign` editable）；完成 stage 与 `pxdbench-json-float.patch` 补丁；内置 `verify_import` 输出 `torch 2.14.1+cu130 13.0 NVIDIA GB10`、`jax 0.12.1 [CudaDevice(id=0)]`、`jax matmul 16777216.0`；`pxdesign pipeline --help` 退出 `0`；独立 `runtime/pxdesign` 目录约 `7.0G`（`/home/certa/.cache/uv` 总字节自 `14,960,795,247` 变为 `15,701,214,533`，注：缓存目录体积不能等价于网络下载字节或精确复用量）。
    - `03_resource_binding_and_wrapper_check.log`（`EXIT_CODE: 0`）：空数据目录负例 `verify pxdesign.preview` 返回 `exit 3`（`state="download_disabled"`）、`pxdesign-run --help` 返回 `exit 4`；通过沙箱 `conf/pxdesign.env` 只读绑定既有 `prefix/pxdesign` 且 `RUNTIME_CACHE_DIR=data/cache` 留在沙箱内后，`resource status`/`verify pxdesign.preview` 均返回 `exit 0`（`state="ready"`，通过全部 11 项资源 SHA-256 与 `ValidateInstalled` 11 项源码/site `.py` SHA-256 校验），`pxdesign-run --help` 退出 `0`，省略 `--use_msa false` 的 `pxdesign-run pipeline` 返回 `exit 4`；`prefix/pxdesign` 11 项文件 mtime 仍保持 `2026-11-11`。
    - **本轮边界**：仅完成隔离 `pxdesign` 新 env 安装、PyTorch/JAX GPU 自检、CLI help 与资源绑定正负例校验；未运行 `pxdesign` 推理/pipeline，未修改 `prefix/`、`bb_prefix/`、依赖 pin 或科学参数。
## AT-1 隔离非生产 `pxdesign-run pipeline --preset preview` 验证（2026-11-24 14:43 HKT）

- **执行前登记（命令、目标与预期副作用）**：
  - **唯一隔离作业与输出目录**：`/home/certa/workspace/bioBox/tmp/at1-nonprod-pxdesign-pipeline-20261124-1443`（完整复制 `bb_sources/pxdesign/examples/`，含 `PDL1_quick_start.yaml` `466` bytes SHA-256 `fecb982ec0a6787a301b625e296b60671ae4f5ce9f747379e83f7bea17d88dc8`、`5o45.cif` `498,156` bytes SHA-256 `25ca22e457201020ff0635ff3b52088493403f087df7d86fbf6de2a8eee01d03` 及预置 `msa/PDL1/0/{pairing,non_pairing}.a3m`）；复用上一步验证通过的沙箱 `tmp/at1-nonprod-pxdesign-env/sandbox`（只读绑定 `prefix/pxdesign` 11 项资源）。
  - **计划执行命令**：`(cd /home/certa/workspace/bioBox/tmp/at1-nonprod-pxdesign-pipeline-20261124-1443 && /home/certa/workspace/bioBox/tmp/at1-nonprod-pxdesign-env/sandbox/runtime/pxdesign/bin/pxdesign-run pipeline --preset preview -i ./examples/PDL1_quick_start.yaml -o ./out --N_sample 2 --use_msa false)`（与仓内既定 `scripts/suites/pxdesign/smoke_pxdesign.sh` 验收命令一致并显式补齐受控 `--use_msa false`；wrapper 自动补 `--load_checkpoint_dir`、`--use_fast_ln False`、`--num_workers 0`；科学参数 `N_step=400`、`dtype=bf16`、`eta=const/2.5/2.5`、`N_max_runs=1`、`binder_length=80` 全部保持原样，遵循 `execute_timeout=0` 不设短超时）。
  - **预期副作用**：仅写 `tmp/at1-nonprod-pxdesign-pipeline-20261124-1443/out` 与沙箱内 `RUNTIME_CACHE_DIR`（`sandbox/runtime/pxdesign/data/cache`）；不改动 `prefix/pxdesign`、不联网下载新模型/数据库、不启 API 服务/root/CI/提交。
  - **执行结果（`tmp/at1-nonprod-pxdesign-pipeline-logs/`，`EXIT_CODE: 0`）**：
    - `01_preflight_example_and_gpu_check.log`（`EXIT_CODE: 0`）：确认 `NVIDIA GB10` 空闲（`0%` 利用率，`11.04W`，`43°C`）、沙箱 `verify pxdesign.preview` 返回 `state="ready"`，并核实 `bb_sources/pxdesign/examples/` 原始输入文件大小与 SHA-256。
    - `02_contract_fail_closed_check.log`（`EXIT_CODE: 0`）：省略 `--use_msa false` 与 `--preset extended --use_msa false` 均在启动 Python 前以 **`exit 4`** 拒绝且不创建输出目录。
    - `03_isolated_pxdesign_pipeline.log` & `03_gpu_sampling_during_pipeline.log`（`WALL_SECONDS: 119`，`EXIT_CODE: 0`）：完整经历 `PXDesign` diffusion（`pxdesign_v0.1.0.pt`，`N_sample=2`、`N_step=400`、`dtype=bf16`，GB10 利用率 `93%`、`47.18–49.39W`）→ `converted_pdbs` + `ProteinMPNN` → `JAX/AF2` 复合物预测（`96%` GPU、`39.21W`）→ `AF2` binder 单体预测（`35%` GPU）→ `Preview` 汇总排名（`mode: "Preview"`）；结束后 GPU 回落至 `0%`。
    - `04_output_structure_and_post_check.log`（`EXIT_CODE: 0`）：无 `out/ERR/`；产出 `4` 份 mmCIF（`predictions/PDL1_quick_start_sample_{0,1}.cif` 与 `orig_designed/rank_{1,2}.cif`），经隔离 venv `biotite` 与 `biopython` 解析均为 `chains=['A0', 'B0']`、`residues={'A0': 116, 'B0': 80}`、`atoms=1250`；`out/config.yaml` 实证 `use_msa=false`、`use_fast_ln=false`、`num_workers=0`、`eval_protenix=false`、`eval_protenix_mini=false`；`summary_output.json` 与 `summary.csv` 完整可解析（`af2_easy_success.count=0`，`pLDDT.avg=0.94`，`i_pTM.avg=0.105`，`af2_complex_pred_design_rmsd.avg=10.725`，`pLDDT_MONOMER.avg=0.952`）；`RUNTIME_CACHE_DIR`（`5.2M`）留在沙箱内，`prefix/pxdesign` 11 项资源 mtime 仍保持 `2026-11-11`。
    - **明确限定**：本切片仅证明在隔离新 `PXDesign` venv + 只读旧权重下，`pxdesign-run pipeline --preset preview --use_msa false` 真实端到端工程流水线、GPU 阶段调度及输出 CIF/CSV/JSON 结构可解析；无科学实验基准对照（且本轮 2 样本 `af2_easy_success=0`），**科学准确性未证明**，亦未覆盖 `bb_service` HTTP/gRPC/MCP 异步入队、`/bb_service` root 部署或 `dirty=0` CI 发版。
## AT-1 隔离非生产 `bb_service` 真实 HTTP 入队→执行→产物读取闭环验证（2026-11-24 14:55 HKT）

- **执行前登记（命令、目标与预期副作用）**：
  - **独立服务沙箱**：`/home/certa/workspace/bioBox/tmp/at1-nonprod-service-queue-sandbox`（独立 `bb_service`、隔离 SQLite `./data/bb_service.db`、隔离作业目录 `./data/cli_jobs`，`runtime/pxdesign` 软链复用已验收隔离 `pxdesign` 沙箱及其只读 `prefix/pxdesign` 绑定，不改 `prefix/`、不读/改 `example_files/config_local*.yaml`、不启 systemd/root）。
  - **网络与门禁边界**：仅绑定 `127.0.0.1` 空闲端口（`jsonrpc=127.0.0.1:40163 mcp=127.0.0.1:45717`）；启用 `auth_cfg`（`email.enabled=false`），临时测试账户通过 `auth.verify_code.send.email` → `auth.register` → `auth.login.email` → `auth.api_key.issue` 内存内获取 `ptxk_` 凭证，不在日志/导出包/终端打印任何凭据或验证码；`task_lifecycle_timeout_enabled=false`、`execute_timeout_enabled=false`。
  - **计划动作**：
    1. 启动隔离 `bb_service` 进程并记录受控 `PID`；
    2. 执行负例校验（无凭证、伪造 `ptxk_invalid`、非对象 `arguments`、未知方法）并核查 SQLite `async_tasks` 记录数为 `0`；
    3. 通过 HTTP JSON-RPC `tools/call` `name="pxdesign"` 真入队一条 `preview`（`input="./examples/PDL1_quick_start.yaml"` 绝对隔离路径、`preset="preview"`、`use_msa=false`、`N_sample=2`），经 Policy 认领→`pxdesign-run` GPU 执行→轮询 `task.get` / `task.get.steps` 至终态；
    4. 通过 `task.artifacts`（及 MCP HTTP 只读路径 `task.get` / `task.artifacts`）与 `task.artifact` 读取 base64 产物并与磁盘文件 SHA-256 核对（不 parse summary）；核对非秘密 DB 表（`async_tasks` / `task_steps`）持久化事实；
    5. 仅向自启 `bb_service` PID 发送 `SIGTERM` 正常退出并记录实际 exit，不触碰任何既有用户进程；脱敏后布尔扫描导出 `/workspace/biobox-at1-nonprod-service-queue/`。
  - **执行结果（`2026-11-24 14:55–14:57 HKT`，`tmp/at1-nonprod-service-queue-logs/` 与 `/workspace/biobox-at1-nonprod-service-queue/`）**：
    - **服务启停与隔离**：隔离编译 `bb_service` 在 `tmp/at1-nonprod-service-queue-sandbox` 启动（`PID=178109`，仅监听 `127.0.0.1:40163` JSON-RPC 与 `127.0.0.1:45717` MCP，禁用 gRPC/WebSocket/Frontend/Email）。全部验证完成后向自启 `PID=178109` 发送 `SIGTERM`，`bb_service` 正常停止（`exit_code=0`，总耗时 `99.94s`）；仅以 `ps -eo pid=,ppid=,comm=,stat=,etime=` 与 `/proc/<pid>/exe` basename 复核，无残留 `bb_service`/`pxdesign`/`protenix` 进程。
    - **鉴权与负例拦截（DB 零新增）**：内存内通过 `auth.verify_code.send.email`（`delivery_id=1747a876-763f-4c00-832b-b2edf71ebb0d`）→ `auth.register` → `auth.login.email` → `auth.api_key.issue` 获取临时 `ptxk_` 凭证。随后执行 5 项入队负例（无凭证 `isError=True`、伪造 `ptxk_invalid` `isError=True`、非对象 `arguments` `isError=True`、缺失 `arguments` `rpc_error=True`、未知方法 `nonexistent.method` `isError=True`），随后查询隔离 SQLite `async_tasks COUNT(*)=0`（确认负例均未创建任务）。
    - **真实入队 → Policy 队列 → `pxdesign-run` GPU 执行 → 终态**：通过 `POST http://127.0.0.1:40163/` (`tools/call name="pxdesign"`, `input=".../examples/PDL1_quick_start.yaml"`, `dump_dir="out"`, `preset="preview"`, `use_msa=false`, `N_sample=2`) 成功入队 1 条任务：`task_id=4b8a98a6-c641-4af2-88f0-0d7895188cdb`。经 `task.get` 实测状态流转：`queued (progress=0)` → `running / cli_execution` → `done / process_status=success (progress=100, step=artifact_collection, business_statuses=['inspected','cif_files_found'])`；终态结果 `exit_code=0`、`cif_count=6`、`has_err=false`、`cancelled=false`（口径纠正：`cif_count=6` 包含 `2` 份 `orig_designed/rank_{1,2}.cif`、`2` 份 `passing-AF2-IG-easy/rank_{1,2}.cif` 过滤副本与 `2` 份 `predictions/*_sample_{0,1}.cif`，**并非 6 个独立设计**）。执行期 GB10 GPU 采样达 `93%`/`96%`（`49.83W` / `58°C`）。
    - **Artifact 原契约读取与 MCP 只读复核**：`task.artifacts` 返回 `21` 项产物（含 `output/.../*.cif` 与 `run.log`，无输入 sidecar 泄漏）；`task.artifacts` 无凭证/伪造凭证均被拒绝（`isError=True`），`task.artifact` 路径穿越 `../conf/config.yaml` 被拒绝（`isError=True`）。仅对 `output/design_outputs/PDL1_quick_start/orig_designed/rank_1.cif`（`140669` bytes，`sha256=e3674454ec3d324b40af71940982b1331c5edfcf20e236fbc19e13a38ff9b3a3`）与 `run.log`（`13209` bytes，`sha256=571035f519508396ad2f029951988f6824d0c10491fb124d4b2d425b2261476a`）两项 artifact 通过 `task.artifact` 原字节 `encoding="base64"` 做磁盘 SHA-256 核对（未解析 `summary_output.json`）。同 `task_id` 经 MCP HTTP (`127.0.0.1:45717`) `initialize` + `tools/call task.get` 返回 `init_http=200`、`call_http=200`、`isError=False`、`run_status=done`、`process_status=success`。
    - **非秘密 SQLite 持久化事实**：`async_tasks COUNT(*)=1`（`method=pxdesign`, `run_status=done`, `process_status=success`, `progress=100.0`, `error_code=0`, `execution_count=1`），`async_task_steps COUNT(*)=6`（`accepted` → `claimed` → `input_preparation` → `cli_execution` → `artifact_collection` → `finished`）。未全量导出数据库或任何 `users`/`auth_*` 秘密。
    - **真实边界与口径纠正说明**：`InspectOutput` 仅检查 `cif_count=6` 与 `has_err=false`（不解析 summary）；`run.log` 显示 preview 阶段跳过 `ptx`/`ptx_basic`/`ptx_mini` 过滤（缺列），且 `use_msa=false`、无真实科学基准评估，不构成科学正确性或设计成功率验收；`02_service_queue_e2e_sanitized.log` 所记为客户端驱动摘要而非原始 HTTP 响应正文；保留首次驱动脚本末尾查库因猜测 `finished_at` 列名报错（实为 `completed_at`）之披露。原导出包 `/workspace/biobox-at1-nonprod-service-queue/` 不重写。受保护文件 SHA-256 与 submodule gitlinks（`85767b81...` / `f7884413...`）均核对未变。

## AT-1 隔离非生产 `bb_service` 接口兼容切片（gRPC & WebSocket，2026-11-24 15:03+ HKT）

- **执行前登记（命令、目标与隔离副作用）**：
  - **复用目标**：复用隔离沙箱 `/home/certa/workspace/bioBox/tmp/at1-nonprod-service-queue-sandbox` 及其 SQLite `./data/bb_service.db`（内含已完成任务 `4b8a98a6-c641-4af2-88f0-0d7895188cdb`），**绝不入队新任务、不触发 GPU**。
  - **配置与端口隔离**：仅调整 `tmp/at1-nonprod-service-queue-sandbox/conf/config.yaml` 关闭 `api_cfg_jsonRPC`/`api_cfg_mcp`，启用 loopback 监听 `api_cfg_websocket`（`127.0.0.1:56451`，`allowed_origins: ["127.0.0.1:*", "localhost:*"]`）与 `api_cfg_grpc`（`127.0.0.1:45983`）。
  - **已知鉴权与归属边界（先查源码契约）**：`api/api_grpc/api_grpc.go` 暴露 `/protenix_service.api.v1.APIService/Call` 并经 `authorization` metadata 传 Bearer；`api/api_websocket/api_websocket.go` 在 `GET /` 升级 WebSocket 并传 `Authorization` 头。`ability_task.TaskGet` / `TaskArtifacts` / `TaskArtifact` 均强制校验 `WHERE task_id = ? AND user_id = ?`，且 `auth.login.email` 仅支持 `login_method="password"`（无邮箱验证码登录或找回密码接口）。因上一切片临时账户 `at1_queue_ephemeral` 的随机密码、`jwt_token` 与 `ptxk_` 仅在内存中生成且未落盘，不伪造 JWT、不改写已有任务/用户行。
  - **拟执行命令与预期副作用**：
    1. 使用仓内既有 Go 模块（不改 `go.mod`/`go.sum`）在 `tmp/at1-nonprod-iface-compat-logs/` 编译临时 gRPC+WebSocket 测试客户端；
    2. 启动隔离 `bb_service` 进程并记录 `PID`，实测 gRPC `/protenix_service.api.v1.APIService/Call` 与 WebSocket JSON-RPC 的负例（缺 `request_id`、非法 `method`、无鉴权、坏 `ptxk_`、非法参数）实际 gRPC code / JSON-RPC error / 统一业务字段；
    3. 经既有测试鉴权流（`auth.verify_code.send.email` → `auth.register` → `auth.login.email` → `auth.api_key.issue`，仅限新增测试用户/会话，不创建任何任务）在内存内获取临时 `ptxk_`（凭据/验证码/session 整项不落盘），实测 gRPC/WebSocket `auth.jwt_token.check`、`task.list` 及跨用户访问 `4b8a98a6-c641-4af2-88f0-0d7895188cdb` 的隔离拒绝结果，并如实记录因无法无绕过登录原属主而无法经入口读取 `4b8a98a6-...` 及 `rank_1.cif`/`run.log` 的实证边界；
    4. 向自启 `bb_service` `PID` 发 `SIGTERM` 正常退出并记录实际 exit，核对前后 SQLite `async_tasks COUNT(*)=1` 与 `execution_count=1` 不增；导出新脱敏包 `/workspace/biobox-at1-nonprod-iface-compat/` 并做程序布尔扫描。
  - **执行结果（`2026-11-24 15:08 HKT`，`tmp/at1-nonprod-iface-compat-logs/` 与 `/workspace/biobox-at1-nonprod-iface-compat/`）**：
    - **服务启停与 DB 零新增对比**：隔离进程 `bb_service`（`PID=185335`，仅启用 loopback `127.0.0.1:45983` gRPC 与 `127.0.0.1:56451` WebSocket）启动后运行 `iface_client` 完成全部 20 项调用（`0.44s`）。随后向自启 `PID=185335` 发送 `SIGTERM` 停止，`wait()` 实测返回 `exit_code=-15`（因 `0.44s` 时服务端尚在完成 `test_ui.LoadTestWeb()` / 未进入 `main.go` 末尾 `common.LoadSigHandle` 注册前即收到 `SIGTERM`）；仅以 `ps -eo pid=,ppid=,comm=,stat=,etime=` 与 `/proc/<pid>/exe` basename 复核 `residual_processes=[]`。SQLite 任务表三项指标前后比对完全一致：`async_tasks COUNT(*)=1`（`4b8a98a6-c641-4af2-88f0-0d7895188cdb`, `done`, `success`, `execution_count=1`）、`async_task_steps COUNT(*)=6`（注意：此处及 `03_lifecycle_and_db_compare.log` 中 `db_unchanged=True` **仅指任务数/步骤数/执行计数三项不变**；注册临时用户 `at1_iface_ephemeral` 已写入 `users`/`auth_verify_codes`/`auth_sessions`/`auth_api_keys` 表，不能称整库未变，原日志保留不重写）。
    - **gRPC `/protenix_service.api.v1.APIService/Call` 实测（`127.0.0.1:45983`）**：
      - 传输/协议负例：`grpc_neg_missing_request_id` 返回 gRPC `InvalidArgument`（`"request_id is required"`）；`grpc_neg_invalid_method`（`method="ping"`）返回 gRPC `InvalidArgument`（`"invalid tools/call request: only tools/call is supported"`）。
      - 门禁/参数负例：`grpc_neg_no_auth_task_get` 返回 gRPC `OK` + `isError=true`、`resultType="complete"`、`error_code=10004`（`"permission denied"`）；`grpc_neg_bad_credential_task_get`（`Bearer <test-placeholder>`）返回 gRPC `OK` + `isError=true`、`error_code=10004`（`"permission denied"`）；`grpc_neg_invalid_task_id`（有效鉴权下 `task_id="not-a-uuid"`）返回 gRPC `OK` + `isError=true`、`error_code=10003`（`"invalid arguments"`）。
      - 临时测试鉴权与只读/归属隔离验证：经 gRPC 完成 `auth.verify_code.send.email` → `auth.register` → `auth.login.email` → `auth.api_key.issue`（全部 `password`/`verify_code`/`jwt_token`/`refresh_token`/`api_key`/`key_prefix`/`api_key_id`/`session_id` 仅在内存传递并整项剔除）；`auth.jwt_token.check` 返回 `gRPC OK`、`isError=false`、`auth_source="api_key"`；`task.list` 返回 `gRPC OK`、`isError=false`、`tasks=[]`；跨用户查询 `4b8a98a6-c641-4af2-88f0-0d7895188cdb` 的 `task.get` 与 `task.artifacts` 均返回 `gRPC OK`、`isError=true`、`error_code=10003`（`"invalid arguments"`，未泄漏任务内容）。
    - **WebSocket `ws://127.0.0.1:56451/` 实测（`GET /` + `Upgrade: websocket` JSON-RPC 2.0）**：
      - 非升级 `GET /` 返回 `HTTP 200`；负例 `ws_neg_invalid_method`（`HTTP 101`）返回 JSON-RPC `error.code=-32602`（`"invalid tools/call request: only tools/call is supported"`）；`ws_neg_no_auth_task_get` 与 `ws_neg_bad_credential_task_get`（`HTTP 101`）均返回 `isError=true`、`error_code=10004`（`"permission denied"`）。
      - 有效 Bearer 鉴权下（`HTTP 101`）：`auth.jwt_token.check` 返回 `isError=false`、`auth_source="api_key"`；`task.list` 返回 `isError=false`、`tasks=[]`；跨用户 `task.get`（`4b8a98a6-c641-4af2-88f0-0d7895188cdb`）返回 `isError=true`、`error_code=10003`（`"invalid arguments"`）。
    - **实证阻塞与边界结论（停写交回）**：gRPC（`/protenix_service.api.v1.APIService/Call`）与既有 WebSocket（`GET /` Upgrade JSON-RPC 2.0）的 wire 契约、鉴权门禁、错误码与跨用户隔离均已实测打通；但**在“绝不入队新任务、不改写 DB/用户行、不伪造 JWT”边界下，无法经 gRPC/WebSocket 读取原任务 `4b8a98a6-c641-4af2-88f0-0d7895188cdb` 的 `task.get` 内容或 `rank_1.cif`/`run.log` base64 artifact 核对 hash**——原因是在上一切片中属主 `at1_queue_ephemeral` 的随机密码、`jwt_token` 与 `ptxk_` 仅存内存未落盘，DB 内仅存单向哈希，`auth.login.email` 不支持验证码登录且无重置密码/跨用户只读接口。若后续需经 gRPC/WebSocket 完成同任务级产物 hash 闭环，需主端批准（a）另建一次性持有内存凭据的非生产任务或（b）在隔离测试夹具层提供受控测试账号。原包 `/workspace/biobox-at1-nonprod-service-queue/` 未修改。

## AT-1 隔离非生产 `bb_service` 统一夹具全协议同任务闭环（2026-11-24 15:14+ HKT）

- **执行前登记（命令、目标与隔离副作用）**：
  - **目的**：仅修复测试夹具的主体生命周期缺口（不改产品源码），在单次隔离运行与同一夹具内存内完成注册、真实 GPU 入队一次、全四协议同任务读取/artifact SHA-256 核对、负例与正常退出。
  - **独立沙箱**：`/home/certa/workspace/bioBox/tmp/at1-nonprod-unified-protocol-sandbox`（独立编译 `bb_service`、全新隔离配置 `conf/config.yaml`、全新 SQLite `./data/bb_service.db`、全新作业目录 `./data/cli_jobs/`；`runtime/pxdesign` 软链复用既有已验收隔离 `pxdesign` 沙箱，不触碰旧 `prefix/`、旧 sandbox、旧 DB、`config_local*`、生产 token 或 `systemd`/root）。
  - **四协议 loopback 端口**：仅绑定 `127.0.0.1` 空闲端口：`jsonrpc=57007`、`mcp=40263`、`websocket=41301`（`allowed_origins: ["127.0.0.1:*", "localhost:*"]`）、`grpc=40495`；禁用 `test_ui_cfg` / `frontend_cfg` / `email`；`policy_cfg.max_workers=1`、`policy_duration="1s"`、`task_lifecycle_timeout_enabled=false`、`execute_timeout_enabled=false`。
  - **拟执行命令与预期副作用**：
    1. 构建新沙箱并运行 `./bb_service resource status pxdesign.preview` 确认 `state="ready"`；
    2. 用仓内既有 Go 模块（不改 `go.mod`/`go.sum`）在 `tmp/at1-nonprod-unified-protocol-logs/` 编译统一客户端 `unified_client`；
    3. 启动隔离 `bb_service`，在 `unified_client` 内存内注册 2 个临时测试主体（`owner` 与 `other`，凭据全程仅保留至所有协议读与 SHA-256 核对结束，不落盘不打印）；
    4. 入队前先跑未鉴权/坏凭据/非法参数负例并核对 `async_tasks COUNT(*)=0`；经 HTTP JSON-RPC 以 `owner` 真入队 **1 条** `pxdesign` `preview`（`PDL1_quick_start.yaml`, `use_msa=false`, `N_sample=2`），轮询至终态并采样 GPU；
    5. 以同一 `owner` 内存凭据分别经 **JSON-RPC / gRPC (`/protenix_service.api.v1.APIService/Call`) / WebSocket (`ws://127.0.0.1:41301/`) / MCP (`http://127.0.0.1:40263/`)** 对同一 `task_id` 调用 `task.get`、`task.list`、`task.artifacts` 与 `task.artifact`（读取真实 `rank_1.cif` 与 `run.log` base64 原字节并核对磁盘 SHA-256，JSON 脱敏记录中 CIF base64 仅保长度/SHA 并注明省略载荷）；
    6. 以第二主体 `other` 校验跨用户隔离（`task.list=[]`，`task.get`/`task.artifacts`/`task.artifact` 被拒）、路径穿越负例被拒；核对 SQLite 白名单 `async_tasks COUNT(*)=1`、`execution_count=1` 与 `async_task_steps`；
    7. 释放内存凭据后仅向自启 `bb_service` `PID` 发 `SIGTERM`，分记 `driver_exit_code` 与 `bb_service_sigterm_exit_code`；脱敏后程序布尔扫描并导出 `/workspace/biobox-at1-nonprod-unified-protocol/`。
  - **执行结果（`2026-11-24 15:19–15:21 HKT`，`tmp/at1-nonprod-unified-protocol-logs/` 与 `/workspace/biobox-at1-nonprod-unified-protocol/`）**：
    - **夹具修正披露**：首跑（`15:17 HKT`，`task_id=4772dd4d-...`）时因测试客户端 `unified_client.go` 未给 `coder/websocket` 拨号连接设置 `conn.SetReadLimit(4 << 20)`，读取 ~187 KiB `rank_1.cif` base64 响应时触发客户端默认 32769B 限制（`driver_exit_code=2`，服务端 `bb_service_sigterm_exit_code=0`）。仅在客户端测试夹具 `unified_client.go` 增加 `conn.SetReadLimit(4 << 20)`（与服务端 `api_websocket.go:74` 一致，零产品源码改动），清空临时沙箱 `data/` 与 `logs/` 后完成一次干净统一运行。
    - **服务启停、退出码与 DB 白名单**：隔离 `bb_service`（`PID=191812`，仅监听 `127.0.0.1:{57007,40263,41301,40495}`）总耗时 `101.95s`；统一夹具在所有四协议读取与 SHA-256 核对完成后释放内存凭据并退出（`driver_exit_code=0`）；随后向 `PID=191812` 发送 `SIGTERM`，正常关闭全部 4 协议服务并退出（`bb_service_sigterm_exit_code=0`）；`ps -eo pid=,ppid=,comm=,stat=,etime=` 与 `/proc/<pid>/exe` basename 复核 `residual_bb_or_runner_processes=[]`。SQLite 白名单核对：`async_tasks COUNT(*)=1`（`task_id=63600fbf-a044-4a68-ba9d-6a2916d6dd8e`, `method="pxdesign"`, `run_status="done"`, `process_status="success"`, `progress=100.0`, `execution_count=1`, `error_code=0`），`async_task_steps COUNT(*)=6`（`accepted` → `claimed` → `input_preparation` → `cli_execution` → `artifact_collection` → `finished`）。
    - **主体注册、入队前负例与单次 GPU 任务**：统一夹具内存内完成 `at1_unified_owner` 与 `at1_unified_other` 注册及 `ptxk_` 签发（全部密码/验证码/`jwt_token`/`refresh_token`/`api_key`/`key_prefix`/`api_key_id`/`session_id` 均仅存内存并整项剔除）。入队前 4 项负例（JSON-RPC 无 auth `10004`、坏 `ptxk_` `10004`、非对象 `arguments` `10003`、gRPC 缺 `request_id` `InvalidArgument`）均被拒绝且 `async_tasks COUNT(*)=0`。随后仅经 JSON-RPC 入队 1 条 `pxdesign preview`（`PDL1_quick_start.yaml`, `use_msa=false`, `N_sample=2`），GB10 GPU 采样达 `93%` / `96%`（`50.22W` / `60°C`），终态 `exit_code=0`、`cif_count=5`、`has_err=false`。
    - **四协议同任务全链路一致验证（`task_id=63600fbf-a044-4a68-ba9d-6a2916d6dd8e`）**：
      - **JSON-RPC (`127.0.0.1:57007`, HTTP `200`)** / **gRPC (`/protenix_service.api.v1.APIService/Call` `127.0.0.1:40495`, `grpc_code="OK"`)** / **WebSocket (`ws://127.0.0.1:41301/`, HTTP `101`)** / **MCP (`127.0.0.1:40263`, `initialize` + `tools/call` HTTP `200`)** 均对同一 `task_id` 返回 `isError=false`、`run_status="done"`、`process_status="success"`、`task.list` 仅见属主 `1` 条任务、`task.artifacts` 返回 `20` 项产物；
      - 四协议分别经 `task.artifact` 读取真实 `output/design_outputs/PDL1_quick_start/orig_designed/rank_1.cif`（`141919` bytes，`sha256=fd7ec1bca04912492c2630ff47f385cd103a216d5fa083f17d92ee32692bcef1`，JSON 记录中省略 raw base64 载荷并注明长度/SHA）与 `run.log`（`13252` bytes，`sha256=2adb1e800d61411c2d4ef97c3607688ca696a09f0d0a9d984eb5552cdffe7bc8`），**全部与磁盘文件 SHA-256 一致（`sha256_matches_disk=true`）**。
    - **入队后负例与跨主体隔离**：第二主体 `at1_unified_other` 在 JSON-RPC `task.list` 仅见 `tasks=[]`（`0` 条），经 gRPC（`task.get`/`task.artifact`）、WebSocket（`task.get`）、MCP（`task.get`）访问属主任务均被拒绝（`isError=true`、`error_code=10003` `"invalid arguments"`）；属主路径穿越 `../conf/config.yaml` 被拒绝（gRPC `OK`, `isError=true`, `error_code=10003`）；无鉴权与坏凭据均返回 `error_code=10004` `"permission denied"`。
    - **真实边界与保护校验**：本切片证明在统一非生产夹具下，JSON-RPC/gRPC/WebSocket/MCP 四协议共享同一 `APIExecuter`、鉴权属主隔离、持久化队列与原字节 artifact SHA-256 读契约；`use_msa=false`、`preview` 阶段跳过 `ptx*` 过滤且无实验基准，不证明科学准确性；受保护三文件 SHA-256、index 与 `bb_sources/*` gitlinks 核对未变；`go.mod`/`go.sum` 与产品源码零改动。
## AT-1 统一夹具偏差勘误与隔离非生产 `release` `frontend` 静态壳验收（2026-11-24 15:36+ HKT）

- **上一切片（`at1-nonprod-unified-protocol`）偏差勘误**：
  - 本切片实际先后运行了 **2 次** `pxdesign` GPU 任务（首跑 `4772dd4d-3ff1-442b-a2e5-45785de54814`，次跑 `63600fbf-a044-4a68-ba9d-6a2916d6dd8e`），违反单次 GPU 与留证约束；撤回“仅历史 1 次 GPU”口径。
  - 首跑的沙箱 `data/bb_service.db*`、`data/cli_jobs/4772dd4d-...` 与服务日志已被次跑前 `rm -rf` 清理或覆盖，原磁盘实体在当前指定范围缺失；首跑原始工具返回审计摘录留存于本 box `/workspace/biobox-at1-unified-first-run-audit.txt`。
  - 经独立复核确认：**最终第二轮（`63600fbf-a044-4a68-ba9d-6a2916d6dd8e`）四协议功能证据可用，无产品 mustfix**；源 `tmp/at1-nonprod-unified-protocol*` 与 `/workspace/biobox-at1-nonprod-unified-protocol/` 保持冻结不动。

- **本切片执行前登记（`frontend-static`：命令、目标与隔离边界）**：
  - **目的**：按原用户布局贯穿/跑通范围，仅做最小非 GPU 当前 `release` 模式 `frontend` 静态壳（`frontend/`）验收；不开发 UI、不以 `test_ui`/Three.js 替代正式入口、不触发任何入队或 GPU。
  - **独立目录（全独立命名，失败即停且绝不清库重跑）**：
    - `tmp/at1-nonprod-frontend-static-sandbox/`（仅从现有 `build/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip` 只读解压 `bb_service` 二进制与 `BUILD_INFO.txt`，不改原 zip；写入隔离 `conf/config.yaml` 与 `./data/bb_service.db`）
    - `tmp/at1-nonprod-frontend-static-logs/`
    - `tmp/at1-nonprod-frontend-static-export/` 与 box `/workspace/biobox-at1-nonprod-frontend-static/`
  - **拟执行命令与预期副作用**：
    1. 在现有本地缓存下运行 `go test -count=1 -race ./frontend/...`（无外部下载）；
    2. 解压 `bb_service` 至 `tmp/at1-nonprod-frontend-static-sandbox/bb_service`，运行 `./bb_service version` 与 `go version -m` 核验真实 `Release` 构建身份；
    3. 仅在 `127.0.0.1` 空闲端口启用 `frontend_cfg.enabled: true`，关闭 `test_ui_cfg` / `api_cfg_*` / `auth_cfg.email` / `cli_cfg.*.binary`（禁止入队/GPU），按 `frontend/frontend.go` 契约对 `GET /`、`GET /index.js`、`GET /index.css`、`HEAD /` 及坏路由/非 GET 方法（`/src/index.ts`、`/package.json`、`/scripts/`、`/missing.js`、`POST /`）执行 HTTP 状态、Content-Type、正文内容及 `frontend/dist` SHA-256 比对；
    4. 按 `web-browser-debug` Skill 核查 `ego-browser` 与宿主 Playwright MCP 能力（若均缺失则如实报告浏览器层未验，不装浏览器/依赖/驱动、不驱动替代浏览器或 box 桌面）；
    5. 仅向自启 `bb_service` PID 发 `SIGTERM` 退出，分记 driver/service 退出码，核查无残留进程（仅 `pid`/`comm`/`exe`）、受保护三文件 SHA-256、git index 与 `bb_sources/*` gitlinks 未变，脱敏布尔扫描后导出。
  - **执行结果（`2026-11-24 15:39 HKT`，`tmp/at1-nonprod-frontend-static-logs/` 与 `/workspace/biobox-at1-nonprod-frontend-static/`）**：
    - **测试与真实 `Release` 构建身份**：`GOPROXY=off go test -v -count=1 -race ./frontend/...` 通过（`1.024s`，涵盖 `TestFrontendDisabledAndOccupiedPort` 与 `TestFrontendHTTPAndShutdown` 9 项子用例）。从既有 `build/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip` 只读解压出的 `bb_service`（SHA-256 `8205a42b5eb9e1ab3a5fa0b7e762ddd606c7bdbd7487b037fa2a30f45b1d6913`）经 `./bb_service version` 与 `go version -m` 核实为 `打  包  模  式 :  Release`、`v0.0.3`、`CGO_ENABLED=1`、`linux/arm64`、`build_time=2026-11-24_13:58:17`、`vcs.revision=2473e4e20b41ecf750000b751ec1069f9e7bcc62`。
    - **隔离 `Release` 实进程 HTTP 静态壳核验**：以隔离 `conf/config.yaml` 仅在 `127.0.0.1:33641` 启用 `frontend_cfg.enabled=true`（关闭 `test_ui_cfg` / `api_cfg_*` / `cli_cfg.*.binary`），服务日志实证 `runMode=[Release]`、`Frontend Run On  [http://127.0.0.1:33641]` 且四套 API 服务及 `test_ui` 均未启动；10 项 HTTP 契约检查全部通过：`GET /`（`200`, `text/html; charset=utf-8`, `382` bytes, 与 `frontend/dist/index.html` SHA-256 `b7e3d350...` 一致，含 `<link rel="stylesheet" href="/index.css" />`、`<script type="module" src="/index.js"></script>`、`id="frontend-example"`）、`GET /index.js`（`200`, `text/javascript; charset=utf-8`, `155` bytes, SHA-256 `4df59dc6...`, 含 `"this is example frontend"`）、`GET /index.css`（`200`, `text/css; charset=utf-8`, `346` bytes, SHA-256 `20a727f7...`）、`HEAD /`（`200`, `body_len=0`），以及坏路由/非 GET 方法（`GET /src/index.ts`、`GET /package.json`、`GET /scripts/`、`GET /missing.js`、`GET /../conf/config.yaml`、`POST /`）均按 `frontend/frontend.go:60-76` 契约返回 `HTTP 200` 中性空响应（`body_len=0`）。`sqlite_async_tasks_count=0`。
    - **浏览器层探测结论（未验）**：依 `web-browser-debug` Skill 核查，源机 `ego-browser` 不在 PATH（`NOT_FOUND`），宿主工具目录亦无已配置的 Playwright MCP（且不驱动替代浏览器或 box 桌面）；因此**真浏览器 DOM 渲染、控制台、网络与刷新交互明确未验**。
    - **进程退出与保护核验**：仅向自启 `bb_service` `PID=202162` 发 `SIGTERM`，分记 `driver_exit_code=0`、`bb_service_sigterm_exit_code=0`、`residual=[]`；受保护三文件 SHA-256、git index 与 `bb_sources/*` gitlinks 均未变；导出包经程序布尔安全扫描通过。
  - **浏览器来源分界补充（独立复核口径）**：DevFull 独立复核确认上述 `frontend-static` HTTP 静态检查通过，并另行使用与 `frontend/dist/{index.html,index.js,index.css}` 同字节的派生浏览器夹具验证 `h1` 渲染、JS/CSS 加载与页面刷新通过；**该派生夹具不代表对源机 `bb_service` 实跑二进制的直接浏览器验收**（源机 `ego-browser` 与 Playwright MCP 均缺失，源机二进制直接浏览器访问仍未验，不混作源二进制或 UI 全部通过）。

## AT-1 非 root 隔离部署回滚回归演练（`installer-rollback`，2026-11-24 15:47+ HKT）

- **静态隔离审计与执行前登记（命令、映射边界与目标）**：
  - **严格限定**：仅使用既有 `scripts/tests/check_install_update_restore.sh`（SHA-256 `12e804fa81c431ec3cfa82c0bc3f6a0e04efabce8ac682bd4a039eaf4702cfc3`）调用 `example_files/install_bb_service.sh` 副本（SHA-256 `834c16954956e30bebe6f8ae340d6a6f68a1c94844343a15c60d0da953940058`），不修改脚本、不新增矩阵、不用 GPU/真实 systemd/root/网络下载。**该测试全部使用桩 `uv`、桩 `runtime` 与桩 `systemctl`，只证明跨运行时失败回滚与混合状态阻断回归，不代表真实完整部署或真包安装闭环**。
  - **`bwrap` 映射与 `clear_install_files` 静态边界**：
    - `--unshare-net --tmpfs / --dev /dev --proc /proc --tmpfs /tmp`；
    - 宿主只读挂载：`--ro-bind /usr /usr --ro-bind /bin /bin --ro-bind /lib /lib`（及存在的 `/lib64`、`/sbin`）；
    - 唯一可写 bind（全部位于 `TEST_OUT=/home/certa/workspace/bioBox/tmp/at1-nonprod-installer-rollback-work` 下隔离目录）：`--bind $app_root /bb_service`、`--bind $systemd_dir /etc/systemd/system`、`--bind $pkg /opt/pkg`、`--bind $bindir /fixture-bin`；
    - `clear_install_files`（`install_bb_service.sh:33-39`）在假根内 `SCRIPT_DIR=/opt/pkg`、`PARENT_DIR=/opt`（`tmpfs /` 上）；且矩阵两组失败测试均在 `updateBinary` 内 `exit 1` 提前退出，即使触发亦绝不会写/删宿主 `/bb_service`、`/etc/systemd/system`、旧 `prefix/` 或原仓库/解压目录。
  - **拟执行命令与独立产物目录**：`TEST_OUT=/home/certa/workspace/bioBox/tmp/at1-nonprod-installer-rollback-work bash scripts/tests/check_install_update_restore.sh`；独立日志 `tmp/at1-nonprod-installer-rollback-logs/`；box 导出 `/workspace/biobox-at1-nonprod-installer-rollback/`。
  - **执行结果（`2026-11-24 15:48 HKT`，`tmp/at1-nonprod-installer-rollback-logs/` 与 `/workspace/biobox-at1-nonprod-installer-rollback/`）**：
    - **命令与退出码**：`TEST_OUT=/home/certa/workspace/bioBox/tmp/at1-nonprod-installer-rollback-work bash scripts/tests/check_install_update_restore.sh` 外层 `exit_code=0`（`instance update restore fixtures PASS`）；两个 `bwrap` 假根用例内层 `installer.rc` 均按预期返回 `1`（`complete_restore_exit=1`、`mixed_no_start_exit=1`）。
    - **模拟失败与恢复前后文件哈希（`03_fixture_hashes_and_logs.json`）**：
      - 预置 `old-binary\n` SHA-256 `609a1a7d8da179e8a0f05e24b037909776a00cfa0f8c0d33a659c653b0219a1c`、`old-unit\n` SHA-256 `09cbe4eb121c71165b6e043b854b0d451c6efd1f6f2e923b24dec192d3b19764`。
      - 用例 1（`complete_restore`：`protenix` 升级成功、`pxdesign` 返回 `42`）：`protenix` 与 `pxdesign` 均回退为 `current -> releases/old`（`previous -> releases/new`），假根 `/bb_service/bb_service` 与 `/etc/systemd/system/bb_service.service` 哈希与恢复前 `old-binary`/`old-unit` 一致，桩 `systemctl` 记录 `stop` 后重新 `start bb_service.service`。
      - 用例 2（`mixed_no_start`：`pxdesign` 返回 `42` 且 `protenix` 回滚失败）：`protenix/current -> releases/new`、`pxdesign/current -> releases/old`，二进制与 unit 保持 `old-binary`/`old-unit` 不变，日志输出 `not starting bb_service with a mixed runtime or binary`，桩 `systemctl` 未执行 `start`。
      - 两例 `pkg/bb_service/install_bb_service.sh` 均未被 `clear_install_files` 删除（因 `updateBinary` 失败提前 `exit 1`，SHA-256 保持 `834c1695...`）。
    - **宿主未写入与保护校验（`01`/`04` 日志）**：运行前后 `/bb_service`、`/protenix_service`、`/etc/systemd/system/bb_service.service`、`/etc/systemd/system/protenix_service.service` 均 `ABSENT`；`prefix/` mtime 保持 `2026-11-11`；原脚本与 `install_bb_service.sh` SHA-256 未变；受保护三文件 SHA-256、git index 与 `bb_sources/*` gitlinks 未变；`residual=[]`；导出包布尔安全扫描全部通过。
## AT-1 Go 1.26.10 最低版本校验、`scripts/build.sh` UTC 构建时间最小修正与 `amd64` 能力盘点（2026-11-24 19:50+ HKT）

- **问题层实证（改前核对，未先定根因）**：
  - **显示层（`custom_cmd/custom_cmd.go:27,30`）**：契约显式标注 `Git提交时间 UTC:  %s`（`app.GitCommitTime`）与 `构 建 时 间 UTC:  %s`（`app.PackageTime`）。
  - **Debug 生成层（`main.go:71`）**：`mPackageTime = time.Now().UTC().Format("2006-01-02_15:04:05")` 已按 UTC 生成（附注只读观察：`main.go:60` Debug `git_commit_time` 使用 `git show -s --format="%ci"` 未转 UTC，本切片不越界改动 `main.go`）。
  - **Release/Pack 生成层（`scripts/build.sh:332` vs `442`）**：第 332 行 `commit_time` 使用 `date -u -d ...` 按 UTC 生成（实测既有包 `2026-11-24_02:51:11`）；而第 442 行 `build_time="$(date +'%Y-%m-%d_%H:%M:%S')"` **漏掉 `-u`**，按宿主 `Asia/Shanghai`（UTC+8）生成本地时间并同时写入 `BUILD_INFO.txt`（`scripts/build.sh:287,376`）与 `-X main.mPackageTime=${build_time}`（`scripts/build.sh:333`）。实测既有 `build/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip`（SHA-256 `58c7b9fb...`）`BUILD_INFO.txt` 与 `./bb_service version` 均记录 `2026-11-24_13:58:17`（实际 HKT `13:58:17`，对应 `05:58:17 UTC`），确认为 **生成层（`scripts/build.sh:442`）缺 `-u`**。

- **执行前登记（最小修正、Go 1.26.10 校验、`tmp/` 隔离打包与现有包检查）**：
  - **唯一代码修正**：仅将 `scripts/build.sh:442` 改为 `build_time="$(date -u +'%Y-%m-%d_%H:%M:%S')"`（单处 `-u` 增量，候选 diff 严格局限本切片）。
  - **Go 1.26.10 实跑工具链**：使用仓内 `go.mod` 最低版本一致的本地已缓存 Go `1.26.10`（`/home/certa/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.10.linux-arm64/bin/go`，配 `GOTOOLCHAIN=local`），不以 `go1.27.4` 冒充、不改系统工具链或缓存。
  - **隔离构建与旧包保护**：复制 `bb_build/gen_pre_build_config_linux_arm64.yaml` 为 `tmp/at1-nonprod-go126-build-work/gen_arm64_isolated.yaml`，仅将 `paths.build_root` 与 `paths.upload_tmp_dir` 指向 `tmp/at1-nonprod-go126-build-work/{build,upload_tmp_dir}`，通过 `./scripts/build.sh pack release tmp/at1-nonprod-go126-build-work/gen_arm64_isolated.yaml` 执行真实打包；**保留 `build/upload_tmp_dir/*` 与 `build/release/*` 全部旧包与旧证据零覆盖**（运行前后核对 SHA-256）。
  - **拟执行命令与检查清单**：
    1. 记录修正前后 `build/upload_tmp_dir/*.zip` 基线 SHA-256 与宿主 UTC/HKT 时间对照；
    2. 在 `Go 1.26.10` 下依次执行：`go version`、`bash scripts/check_native_build.sh arm64`、`go test -count=1 -race ./...`、`go run scripts/gen_release_assets.go self-test`；
    3. 执行既有发布脚本/包回归：`python3 scripts/tests/test_release_cleanup.py`、`python3 scripts/tests/test_release_guards.py`、`bash scripts/tests/test_source_staging.sh`、既有旧包 `bash scripts/check_bundle_resources.sh build/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip`；
    4. 执行隔离打包 `./scripts/build.sh pack release tmp/at1-nonprod-go126-build-work/gen_arm64_isolated.yaml`，对新产物执行 `bash scripts/check_bundle_resources.sh`、`BUILD_INFO.txt` 核验、提取新 `bb_service` 执行 `./bb_service version` 与 `go version -m` 核对真实 UTC 时间及 `go1.26.10` 内部身份；
    5. 只读/单次探测 `spark-901f` `amd64` CGo 原生/隔离能力与精确缺口（不安装交叉编译器、不伪称 cross 通过）；
    6. 只读准备源 `release` 进程真浏览器验收及正式包非 root 隔离回滚后续最小方案（本切片均不启动）；核验保护文件 SHA-256、index、`bb_sources/*` gitlinks 及零残留进程。
- **执行结果（`2026-11-24 19:51–19:53 HKT`，`tmp/at1-nonprod-go126-build-logs/` 与 `/workspace/biobox-at1-nonprod-go126-build/`）**：
    - **唯一候选修正（`scripts/build.sh:442`）**：仅将 `build_time="$(date +'%Y-%m-%d_%H:%M:%S')"` 改为 `build_time="$(date -u +'%Y-%m-%d_%H:%M:%S')"`（1 行 1 处 `-u` 增量，未改科学 pins/参数/协议/用户配置/其他脚本）。
    - **Go `1.26.10` 真实检查、回归与既有包校验（全部 `exit_code=0`）**：
      - `go version` → `go version go1.26.10 linux/arm64`（`GOTOOLCHAIN=local`，实际运行 `/home/certa/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.10.linux-arm64/bin/go`，未以 `1.27.4` 冒充，`exit=0`）。
      - `bash scripts/check_native_build.sh arm64` → `[native] linux/arm64, native gcc, CGo enabled; packaging only, no GPU/dependency install`（`exit=0`）。
      - `go test -count=1 -race ./...`（Go `1.26.10`）→ 仓内全部 `18` 个含测试包全过（`exit=0`）。
      - `go run scripts/gen_release_assets.go self-test`（Go `1.26.10`）→ `[release-assets] 58 fixture/static contract checks PASS`（`exit=0`）。
      - 既有脚本/包回归：`python3 scripts/tests/test_release_cleanup.py`（`12 cleanup regression cases PASS`，`exit=0`）、`python3 scripts/tests/test_release_guards.py`（`v0.0.2 local/remote protected guard PASS`，`exit=0`）、`bash scripts/tests/test_source_staging.sh`（`PASS protenix` / `PASS pxdesign`，`exit=0`）。
      - 既有旧包 `bash scripts/check_bundle_resources.sh build/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip`（`58c7b9fb...`）→ `protenix.v1` 与 `pxdesign.preview` 均 `status=download_disabled; missing resources/uninstalled runtime refused before Python`（`exit=0`）。
    - **隔离 `arm64` `release` 新产物、SHA-256 与真实 UTC/本地时间对照**：
      - 打包命令：`./scripts/build.sh pack release tmp/at1-nonprod-go126-build-work/gen_arm64_isolated.yaml`（Go `1.26.10`），`exit_code=0`。
      - 宿主打包时刻实测：`pre-pack = 2026-11-24_11:52:26 UTC` / `2026-11-24_19:52:26 HKT`；`post-pack = 2026-11-24_11:52:57 UTC` / `2026-11-24_19:52:57 HKT`。
      - 新产物（全部位于 `tmp/at1-nonprod-go126-build-work/upload_tmp_dir/`）：
        - `bb_service_linux_arm64_release_v0.0.3.zip`（`96,363,454` B，SHA-256 `a05725d32a05c16a563b8f7fece8b4a57cd2d473fae142984c784ab61bd722b7`）
        - `protenix_linux_arm64_release_v0.0.3.zip`（`67,983,615` B，SHA-256 `0d16bbe15aad06dfb53f38ff0053def9184c51b38dd4e5dbad1226e80c0f41b2`）
        - `pxdesign_linux_arm64_release_v0.0.3.zip`（`16,626,437` B，SHA-256 `1a87b3189da1daa3d3271620395cee07364bd20c9d66f5f5deb87c07ce704d26`）
        - 提取新二进制 `bb_service`（SHA-256 `b8abc84ce2f03262e1c49f927eabfb98864692a73353d3ccefcff2461484bb96`，`ELF 64-bit LSB executable, ARM aarch64`）。
      - 新产物内部身份核验：`BUILD_INFO.txt` 三包均写 `build_time=2026-11-24_11:52:26`、`commit=2473e4e20b41ecf750000b751ec1069f9e7bcc62-dirty`、`dirty=1`、`goos=linux`、`goarch=arm64`、`run_mode=release`；新 `./bb_service version` 输出 `打  包  模  式 :  Release`、`Git 提 交 Hash :  2473e4e20b`、`Git提交时间 UTC:  2026-11-24_02:51:11`、`构  建  语  言 :  go1.26.10`、`构  建  系  统 :  Linux`、`构 建 时 间 UTC:  2026-11-24_11:52:26`（与宿主 `11:52:26 UTC` 一致，不再误印 UTC+8 `19:52:26`）；`go version -m` 确认 `go1.26.10`、`GOOS=linux`、`GOARCH=arm64`、`CGO_ENABLED=1`、`vcs.revision=2473e4e20b41ecf750000b751ec1069f9e7bcc62`、`vcs.modified=true`。
      - 新整合包校验：`bash scripts/check_bundle_resources.sh tmp/at1-nonprod-go126-build-work/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip` → `exit_code=0`。
      - 旧包零覆盖证明：`build/upload_tmp_dir/*.zip` 全部 11 个旧包运行前后 SHA-256 完全一致（`58c7b9fb...`、`df1001c8...`、`25fcb457...` 等均未变化）。
    - **`amd64` CGo 原生/隔离环境能力盘点与精确缺口（只读/`exit=1` 实证，未安装新平台、未伪称 cross 通过）**：
      - 宿主为 `Linux spark-901f ... aarch64 GNU/Linux`；`bash scripts/check_native_build.sh amd64` 返回 `target does not match native runner architecture`（`exit=1`）。
      - 工具链/sysroot/用户态仿真盘点：`x86_64-linux-gnu-gcc`（MISSING）、`x86_64-linux-gnu-cc`（MISSING）、`/usr/x86_64-linux-gnu` 与 `/usr/include/x86_64-linux-gnu`（MISSING）、`qemu-x86_64` / `qemu-x86_64-static`（MISSING）；仅有 `gcc` / `aarch64-linux-gnu-gcc`（`aarch64-linux-gnu`）与 `clang`（`aarch64-unknown-linux-gnu`，无 amd64 sysroot/libc-dev）。
      - `.github/workflows/release.yml` 仅含 `workflow_dispatch`（bump tag + 公开 GitHub Release），无非发版 CI；因此 `spark-901f` **当前无 `amd64` CGo 原生或隔离构建/运行验证能力**，需在 `linux/amd64` 原生 runner 或具备 `x86_64-linux-gnu-gcc` + `amd64` glibc/sysroot 的受控环境补验。
    - **保护校验与后续只读最小方案（本切片未启动服务或安装）**：
      - 受保护三文件 SHA-256、git index 与 `bb_sources/*` gitlinks 核对未变；`residual=[]`；已停写待独立 review。
      - **后续最小方案 A（源 `release` 进程真浏览器验收，待跑）**：前提是宿主/runner 就位 `ego-browser` 或 Playwright MCP；从本次新包 `tmp/at1-nonprod-go126-build-work/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip` 只读解压 `bb_service`，以 `127.0.0.1` 空闲端口 + 仅启 `frontend_cfg.enabled=true`（关闭 `test_ui_cfg`/`api_cfg_*`/`cli_cfg.*.binary`）启动单进程，由真实无头浏览器访问 `/`，核对 DOM（`#frontend-example`）、`/index.css` & `/index.js` HTTP `200`、`console.error=0`、`pageerror=0`、强刷一致，最后 `SIGTERM` 退出并核对 `residual=[]` 与 `async_tasks=0`。
      - **后续最小方案 B（正式包非 root 隔离部署回滚演练，待跑）**：在不写宿主 `/bb_service` 或 `/etc/systemd/system` 的 `bwrap` 隔离根内，放入本次真实打包的 `bb_service_linux_arm64_release_v0.0.3.zip` 解压目录（含真实 `bb_service` 二进制、真实 `install_bb_service.sh` 与 `runtime/{protenix,pxdesign}` 脚本/源码），通过桩 `uv`/`systemctl` 演练跨运行时升级失败回滚与混合状态拒启，证明真实打包产物内安装脚本与升级回滚链闭环。

- **`2026-11-24 20:05–20:12 HKT`｜AT-1 隔离非发版串行推进（C：最小脱敏源码白名单快照导出至本 box；①：新整合包 `a05725d3...` `bwrap` 非 root `resource status/verify` 与最小隔离启动实证，停写待独立 review）**：
  - **C｜最小脱敏源码白名单快照导出（仅供 DevFull 交原 `x86/amd64` 审，未自行发云或提交）**：
    - 导出位置（box）：`/workspace/biobox-at1-nonprod-src-snapshot.tar.gz` 与解包校验目录 `/workspace/biobox-at1-nonprod-src-snapshot/`（源机暂存：`tmp/at1-nonprod-src-snapshot/` 与 `tmp/at1-nonprod-src-snapshot.tar.gz`）。
    - 清单与校验身份：`base_head=2473e4e20b41ecf750000b751ec1069f9e7bcc62`、`dirty=true`（AT-1 局部工作区）、显式逐文件白名单共 **`205`** 个文件（Go 源码、`go.mod`/`go.sum`、`test_ui/web/{dist,src}`、`frontend/dist`、embed 必需 `docs/{docs.go,bb_service/api_methods.md}`、`example_files/{config_example.yaml,bb_service.service,install_bb_service.sh}`、`bb_build/*.yaml`、`.github/workflows/release.yml`、顶层 `scripts/*` 及 `scripts/tests/{test_release_cleanup.py,test_release_guards.py}`）。
    - 显式排除：`example_files/config_local.yaml`、`example_files/config_local_auth_bridge.yaml`、`docs/acceptance_plan_official_assets_v0.0.2.md`、全部其余 `docs/*`、`.git/`、`.cursor/`、`.run/`、`tmp/`、`build/`、`prefix/`、`bb_prefix/`、`bb_sources/`、`changelog/`、`bb_suites/`、`scripts/suites/`、`node_modules/`、`__pycache__/`、`*.pyc`、`*.db*`。
    - 哈希与校验：`manifest_sha256=fa8b3276fc44a5bb237d54ed7cb99c0ec11297b38e28c588e4f2004100d15c86`（`SNAPSHOT_SHA256SUMS`）、`archive_sha256=3673e2a4dd6ebe527a7a06aab3e9d6f6b07de633f16e7ddaa4e1444151f2f98c`；box 解包后 `sha256sum -c SNAPSHOT_SHA256SUMS --quiet` 全数通过（`205/205`）且排除项全不存在。
    - 逐文件秘密扫描（不打印命中值）：`high_confidence_hits=0`（`private_key_pem=0`、`known_token_prefix=0`、`jwt_like=0`、`dsn_with_creds=0`、`bearer_literal=0`）；`heuristic_hits=5`（仅 `secret_or_password_assign` 启发式占位符字段命中：`config/storage_config_test.go`、`example_files/config_example.yaml`、`storage/storage_config/storage_config.go`、`test_ui/web/dist/index.js`、`test_ui/web/src/index.ts`）。
    - 快照边界说明：因不含 `.git/` 与上游子模块 `bb_sources/*`，本快照**不支持** `scripts/build.sh` 全量打包或 `bash scripts/tests/test_source_staging.sh`；仅供原 `linux/amd64` 环境执行 Go `1.26.10` 原生构建与测试核验。
  - **①｜真实新整合包 `a05725d3...` `bwrap` 非 root `resource status/verify` 与最小隔离启动（明确非完整 runtime 安装、非 GPU/任务、非宿主 systemd）**：
    - 隔离契约与输入：`tmp/at1-nonprod-realpkg-smoke-work/ISOLATION_CONTRACT.md`；仅从 `tmp/at1-nonprod-go126-build-work/upload_tmp_dir/bb_service_linux_arm64_release_v0.0.3.zip`（SHA-256 `a05725d32a05c16a563b8f7fece8b4a57cd2d473fae142984c784ab61bd722b7`）只读解压 `bb_service`（SHA-256 `b8abc84ce2f03262e1c49f927eabfb98864692a73353d3ccefcff2461484bb96`）与新包既有 `runtime/{protenix,pxdesign}/bin/*-run`；未运行 `install_bb_service.sh`/`uv`/runtime 重建/`patch`/GPU import，未复用旧 `config_local*.yaml` 或旧 DB。
    - 阻塞与保留现场记录（如实披露）：首轮 `03_bwrap_realpkg_smoke.log` 在 `bwrap` 内验证 `lo` UP（`inet 127.0.0.1/8`）后，因宿主机 `/usr/bin/awk -> /etc/alternatives/awk` 而 `--tmpfs /` 未挂 `/etc/alternatives`，测试驱动第 8 行报错 `awk: 未找到命令`（`exit=127`，未触及 `./bb_service`）；保留 `03_bwrap_realpkg_smoke.log` 与 `inside_bwrap.sh` 原样不动，仅将外层驱动改为 `python3` 解析 `ip -o link show` 并生成 `04_bwrap_realpkg_smoke_run.log` 在同隔离下重跑。
    - 实证结果（`04_bwrap_realpkg_smoke_run.log`，`bwrap_outer_exit_code=0`）：
      1. 网络命名空间：`--unshare-net` 下仅存 `lo`（`1: lo: <LOOPBACK,UP,LOWER_UP>`、`inet 127.0.0.1/8`、`net_devs=[lo ]`），服务与 HTTP 客户端同处该 `bwrap` netns。
      2. 真实二进制身份：`./bb_service version` 输出 `打  包  模  式 :  Release`、`v0.0.3`、`2473e4e20b`、`go1.26.10`、`构 建 时 间 UTC:  2026-11-24_11:52:26`。
      3. 资源负例（零 Python/GPU）：未绑定 runtime 时 `config_resource_absent.yaml` 下 `resource status` 返回 `state="download_disabled"`（`protenix.v1` `file=4` / `pxdesign.preview` `file=10`，`rc=0`），`resource verify` 返回 `runtime revision not verified; Python was not started`（`rc=1`）；错配 `--runtime` 路径返回 `configured resource/runtime binding mismatch`（`rc=1`）。
      4. 资源正例（精细 `ro-bind` 既定 `prefix/*/data` 与沙盒 `.venv`/`src`，零 Python/GPU）：`resource status` 与 `resource verify` 对 `protenix.v1` 与 `pxdesign.preview` 均返回 `state="ready"`、`file=0`、`rc=0`。
      5. 最小隔离启动与同 netns HTTP 验证：以隔离 `/bb_service/conf/config.yaml` 仅启 `frontend_cfg.enabled=true`（`127.0.0.1:13006`），关闭 `test_ui_cfg` / `api_cfg_*` / `auth_cfg.email` / `cli_cfg.*.binary`；服务日志确认 `runMode=[Release]`、`Frontend Run On  [http://127.0.0.1:13006]` 及四类 API/gRPC/WS/MCP 服务均未启动。同 netns Python 客户端实测：`GET /`（`200`, `382` B, SHA-256 `b7e3d350...`）、`GET /index.js`（`200`, `155` B, SHA-256 `4df59dc6...`）、`GET /index.css`（`200`, `346` B, SHA-256 `20a727f7...`）、`HEAD /`（`200`, `len=0`）、中性路由/方法（`GET /src/index.ts`、`GET /missing.js`、`POST /` 均 `200` `len=0`），禁用端口 `13001–13005` 全部 `refused`。`driver_exit_code=0`，`bb_service_sigterm_exit_code=0`（干净优雅关闭），`sqlite_tables=7` 且 `async_tasks_count=0`。
    - 保护校验与导出：`05_post_guards.log` 核验受保护三文件 SHA-256、git index、`bb_sources/*` gitlinks、`prefix/{protenix,pxdesign}` 与旧环境目录 mtime、以及 `build/upload_tmp_dir/*.zip` 全部不变，宿主 `residual=[]`。脱敏导出包位于 `tmp/at1-nonprod-realpkg-smoke-export/`（已扫清秘密，导出至 box `/workspace/biobox-at1-nonprod-realpkg-smoke/`）。

- **`2026-11-24 20:25–20:42 HKT`｜AT-1 交付收口（v2 验收快照、DevFull `amd64` 报告归属、只读 diff/DB 核验、运行入口与最小发布门槛；停写待审）**：
  - **1. v2 最小脱敏源码白名单快照（本 box 独立生成，v1 原封留存）**：
    - v1 留存：`/workspace/biobox-at1-nonprod-src-snapshot.tar.gz`（SHA-256 `3673e2a4dd6ebe527a7a06aab3e9d6f6b07de633f16e7ddaa4e1444151f2f98c`，`SNAPSHOT_SHA256SUMS` `fa8b3276fc44a5bb237d54ed7cb99c0ec11297b38e28c588e4f2004100d15c86`，`205` 文件原字节未改）。
    - v2 新快照：`/workspace/biobox-at1-nonprod-src-snapshot-v2.tar.gz`（SHA-256 `744ea2d8d38f9de77c058aebbc54b4cf1f2a43017b4000cae320f2d324b7a6bc`）与 `/workspace/biobox-at1-nonprod-src-snapshot-v2/`（`SNAPSHOT_SHA256SUMS` SHA-256 `accf56fa0926bd3a49bd9702f5df211eaf3429e03240d90f1f85bc22f60cf3ef`，`209` 文件；相对 v1 原 `205` 文件 `0` 改 `0` 删，仅依 `cli_proc/wrapper_resource_test.go:15-16,32,88-92,105,113-131` 只读引用精确补 4 个 `git status=AM` 文件：`bb_suites/protenix/protenix-run` `a015cdc4...`、`bb_suites/pxdesign/pxdesign-run` `8cfd5a0a...`、`scripts/suites/protenix/install_prefix_protenix.sh` `c43c90bb...`、`scripts/suites/pxdesign/install_prefix_pxdesign.sh` `7eece96a...`；均为测试只读声明，不授权 installer 执行；归档相对路径/无穿越/无链接/排除项全过）。
  - **2. `amd64` v2 实测归属说明**：
    - 外部报告路径 `/cursor/stores/user/grok-bot/agents/f192948d-9c7c-43f0-8d40-cff562ab36c9/assets/01M86RWT2Y309XX133WJY9RDZ4.md` 从 AT-1 box 不可读（`File not found`），以下 `amd64` v2 数字均**明确归属父端转述的 DevFull 独立报告**（AT-1 未亲核原文件）：`Go 1.26.10` / `GOTOOLCHAIN=local` / native CGo 全仓 `-race` `22` 包 `exit=0`、`gen_release_assets.go self-test` `58` / `cleanup` `12` / `guards` `0`，直接 `go build` ELF `x86-64` SHA-256 `27d45702f3990eefce501d098fc68fc41a96b9cf71c4088b31e6ec71baa66c93`；`v2/旧205+4` 双边核通过，无源码修复；该 `amd64` 云因缺 `bun` 且快照无 `.git`/upstream 未跑 `amd64` 完整 pack gate（仅指明该环境缺口，非断言所有 `amd64` 环境无能力）；该 ELF 中 `vcs.modified=false` 来自外层 `/workspace` 检出污染，**不覆盖** `SNAPSHOT_META.txt` 的 `dirty=true`，亦**不**冒称正式 clean 包。
  - **3. 源仓只读 `git diff` 分类与 DB/Schema/迁移实际核验（相对 `HEAD=2473e4e20b41ecf750000b751ec1069f9e7bcc62`，`dirty=true`）**：
    - `git status --porcelain=v1` 共 `255` 项；staged `247` 项（`237 R100`、`7 D`、`3 M`：`.gitmodules`、`go.mod`、`go.sum`）；unstaged `144` 项（`M`）；untracked `2` 项（`docs/acceptance_plan_official_assets_v0.0.2.md`、`scripts/tests/__pycache__/test_release_cleanup.cpython-312.pyc`）。
    - **DB schema / 迁移实查结论（未猜测）**：逐文件比对 `HEAD:protenix_service/...` 与现行树，`ability/ability_task/ability_task_model/ability_task_model.go`、`ability/ability_user/ability_user_model/ability_user_model.go`、`api/api_auth/api_auth_model/api_auth_model.go` 及 `storage/storage_config/storage_config.go` 为 **0 字节变更**；`storage/{storage,storage_sqlite,storage_mysql,storage_postgresql,storage_redis}.go` 仅含 Go module 路径替换（`protenix_service -> bioBox`），`GormAutoMigrate` 6 张表（`User`、`AuthVerifyCode`、`AuthSession`、`AuthAPIKey`、`AsyncTask`、`AsyncTaskStep`）结构与迁移逻辑**无变更**。
  - **4. 必需配置键（仅列键名，不涉值）、既有运行/检查/回滚入口与发布门槛**：
    - 必需配置顶级段落与核心键名（来自 `example_files/config_example.yaml` 与 `config/config.go`、`api/api_auth/api_auth_config/api_auth_config.go`）：`storage_cfg`（`primary`、`storage_sqlite.{enabled,path}` / `storage_mysql` / `storage_postgresql` / `storage_redis`）、`api_cfg.{api_cfg_jsonRPC,api_cfg_mcp,api_cfg_websocket,api_cfg_grpc}.{enabled,bind_address,port}`、`frontend_cfg.{enabled,bind_address,port}`、`test_ui_cfg.{enabled,bind_address,port}`、`policy_cfg.{max_workers,policy_duration,workers_scaller,task_lifecycle_timeout_enabled,task_lifecycle_timeout_seconds,task_execute_retry_limit_enabled,task_execute_retry_limit_count}`、`auth_cfg.{enabled,auth_type,email.*,verification.*,session.*,bridge.*}`（`auth_type=bridge` 时 `bridge.public_key_path` 等必填）、`cli_cfg.{work_root,cancel_grace_seconds,execute_timeout_enabled,execute_timeout_seconds,protenix.{binary,resources.*,version_runtimes.*},pxdesign.{binary,resources.*}}`。
    - 既有入口：启动 `bb_service`（或 `example_files/bb_service.service` `ExecStart=/bb_service/bb_service`、`KillMode=control-group`、`ExecStop=/bin/kill -TERM $MAINPID`）；身份/资源只读核验 `./bb_service version`、`./bb_service resource status <protenix.v1|pxdesign.preview>`、`./bb_service resource verify <set> --runtime <wrapper>`、JSON-RPC 方法 `test`（及静态前端 `GET /`）；安装/升级/回退/状态入口 `example_files/install_bb_service.sh`（菜单 `1 install`、`2 update`、`3 uninstall`、`4 rollback runtime`、`5 status`）。
    - **已验 vs 未验与正式发布前最小必要门槛（仅真实阻塞/授权门槛）**：
      - 已验：`arm64` Go `1.26.10` 原生全仓 `18` 含测试包 `-race`、`self-test`（58）、`cleanup`/`guards`/`staging`、UTC 修正后隔离 `arm64` `release` 打包（`a05725d3...`）、`bwrap` 无 root 资源正负例与同 netns 静态前端 HTTP 启动退出（非完整 runtime 安装）；DevFull 报告 `amd64` v2 Go `1.26.10` 全仓 `-race`（22 包）与直接 `go build` ELF。
      - 明确未验边界（按原范围保留，不再桩重跑）：`amd64` 完整 pack gate（待具 `bun` + `.git` + `bb_sources/*` 的原生 `linux/amd64` runner）、真实 `root`/`systemd`/完整 `runtime` 首次安装、正式双架构干净 Release 构建、源二进制真浏览器渲染/交互（无 `ego-browser`/Playwright MCP）。
      - 正式发布前最小必要阻塞/授权门槛：① 清理/提交当前脏工作区并形成明确基线 commit（消除 `dirty=1`）；② 在原生双架构 runner（`ubuntu-24.04` 含 `bun` 与 `ubuntu-24.04-arm`）或受控完整环境完成 `amd64` pack gate 与真实整合包 `root`/`systemd`/完整 `runtime` 首次安装验收；③ 获得用户明确发版/tag/CI 授权（保护配置 `config_local*.yaml` 与 `v0.0.2` 资产继续保持隔离不变）。

- **`2026-11-25 11:40 HKT`｜AT-1 阶段二实施前基线核查、CLI 实证与完整分类/旧新映射（`bb_` Go 包全量收口 + `build_srcs` 恢复 + 默认入口 `bioBox`）**：
  - **实施前现场与保护守卫复核**：
    - `bioBox` `HEAD`：`565a09f63c5762e332dd16fb8085cdd4067ab888`；`git status --short` 仅 2 项既有未跟踪（`?? docs/acceptance_plan_official_assets_v0.0.2.md`、`?? scripts/tests/__pycache__/`），工作区无其他脏修改或暂存项；
    - 保护 SHA-256：`.git/index` = `d03af1b6cf3c238c1aa9464938e9fc660cb02c31384c27654915438203642c0d`、`example_files/config_local.yaml` = `684b906075c72cd76806cafd645bb7da9c6ac6b235550b5e77eaf15f0e31bf6c`、`example_files/config_local_auth_bridge.yaml` = `d53a8a357f4a3f0d65a9d2669c81893ed3756c2186a2c9a0cb43673259d36623`、`docs/acceptance_plan_official_assets_v0.0.2.md` = `de1dafe19706af12bb6ef0c0beba93c21539ff7b63b579556e36f67411fbddbd`、`scripts/tests/__pycache__/test_release_cleanup.cpython-312.pyc` = `742ef011b390a43f64c861dc6f3bcbcd48fdfabf7f5f9d22bd382121fec86670`；
    - Submodule pins（内容/object ID 不升级、不改内部文件）：`bb_sources/protenix` = `85767b811c40ed46e73a9b39519cf6bfca8701ba`、`bb_sources/pxdesign` = `f788441313c84c3074fe9596ac2433f96b15c763`；
    - 模板参照基线 `project_template_go` `HEAD`：`2990acfdcac78f1847901629f2c7410dbd36cc5e`（补充 `git ls-files -s` 19 项 blob 与 `stage1-template-ref` 完整 `argv`/`cwd`/`exit=0`/`go list -e ./...` 至 `tmp/at1-prefix-rule-audit/excerpts/06_template_2990acf_blobs_and_stage1_golist.txt` 与 box `/workspace/biobox-at1-prefix-rule-audit/`）。
  - **本机 CLI / `xjb_code` 实证**：
    - `acpx` 缺失（禁止安装）；`agent` / `cursor-agent`（`2026.11.11-b912975`）已认证（`isAuthenticated: True` / `agent status` 成功），但 `agent --help` 无细粒度写权限策略参数（`--mode` 仅只读 `plan|ask`，`-p/--print` 放开全写与 shell），且 `~/.cursor/cli-config.json` `approvalMode="allowlist"` 仅放行 `Shell(ls)`，不支持不带 `--force`/`--trust` 的安全受限非交互写委派；`xjb_code` 不在 MCP 目录且无 `12100`/`12101` 监听，无 `.auto_coding/`，沿原 markdown 回退记录于本唯一 `docs/bb_build/release_delivery.md`，并经受支持 `machineId` 直接在原本地 checkout 实施。
  - **完整分类与旧→新映射（共 52 个自有非 `main` Go 包 + 子模块路径 + 默认产物入口；`main`/工具 `main`/第三方/子模块内容/wire/认证不变）**：
    1. **保持不变的 `main` / 工具 `main` / 隔离文件 / 第三方 / 纯资源与纯文档目录**：
       - `main.go`、`resource_cmd.go`（`package main`）、`tools.go`（`//go:build tools` `package tools`）；
       - `scripts/gen_api_docs.go`、`scripts/gen_pre_build_config.go`、`scripts/gen_release_assets.go`（`//go:build ignore` `package main`）；
       - 纯资源/构建/文档目录：`bb_build/`、`bb_suites/`、`bb_prefix/`、`scripts/`（含 `scripts/suites/`、`scripts/tests/`）、`example_files/`、`changelog/`、`.github/`、`docs/bb_build/`、`docs/bb_upstream/`、`docs/bb_service/*.md`（除随 `bb_docs` embed 的生成件 `api_methods.md`）、受保护未跟踪 `docs/acceptance_plan_official_assets_v0.0.2.md`。
       - 认证与 wire 兼容常量保持不变：`APIKeyPrefix="ptxk_"`（不并入阶段一参照的 `"bb_"`）、`DefaultSessionIssuer="protenix_service"`、`DefaultSessionAudience="protenix_service"`、`api.proto` `package protenix_service.api.v1;`（`APIService_Call_FullMethodName="/protenix_service.api.v1.APIService/Call"`）。
    2. **全部 52 个项目自有非 `main` Go 包/子包（每层路径与 `package` 声明加一次 `bb_`，保留语义与层级）**：
       - `ability` → `bb_ability`（`package bb_ability`）
       - `ability/ability_protenix` → `bb_ability/bb_ability_protenix`（`package bb_ability_protenix`，含 `testdata/`）
       - `ability/ability_pxdesign` → `bb_ability/bb_ability_pxdesign`（`package bb_ability_pxdesign`，含 `testdata/`）
       - `ability/ability_resource` → `bb_ability/bb_ability_resource`（`package bb_ability_resource`）
       - `ability/ability_task` → `bb_ability/bb_ability_task`（`package bb_ability_task`）
       - `ability/ability_task/ability_task_model` → `bb_ability/bb_ability_task/bb_ability_task_model`（`package bb_ability_task_model`）
       - `ability/ability_user` → `bb_ability/bb_ability_user`（`package bb_ability_user`）
       - `ability/ability_user/ability_user_model` → `bb_ability/bb_ability_user/bb_ability_user_model`（`package bb_ability_user_model`）
       - `ability/ability_user/ability_user_profile` → `bb_ability/bb_ability_user/bb_ability_user_profile`（`package bb_ability_user_profile`）
       - `api` → `bb_api`（`package bb_api`）
       - `api/api_auth` → `bb_api/bb_api_auth`（`package bb_api_auth`）
       - `api/api_auth/{api_auth_api_key,api_auth_bridge,api_auth_common,api_auth_config,api_auth_model,api_auth_register,api_auth_session,api_auth_verify_code}` → `bb_api/bb_api_auth/bb_api_auth_{api_key,bridge,common,config,model,register,session,verify_code}`（`package bb_api_auth_*`）
       - `api/{api_common,api_config,api_error_code,api_executer,api_supported_methods}` → `bb_api/bb_{api_common,api_config,api_error_code,api_executer,api_supported_methods}`（`package bb_api_*`）
       - `api/api_grpc` → `bb_api/bb_api_grpc`（`package bb_api_grpc`，内含 `.proto` 源 `proto/api.proto`）
       - `api/api_grpc/api_config_grpc` → `bb_api/bb_api_grpc/bb_api_config_grpc`（`package bb_api_config_grpc`）
       - `api/api_grpc/protobuf` → `bb_api/bb_api_grpc/bb_protobuf`（`package bb_api_grpc_protobuf`，由 `scripts/gen_proto.sh` 按新 `option go_package` 原链重生成）
       - `api/api_jsonRPC` → `bb_api/bb_api_jsonRPC`（`package bb_api_jsonRPC`）
       - `api/api_jsonRPC/{api_config_jsonRPC,api_jsonRPC_handler,api_jsonRPC_protocol,api_jsonRPC_request,api_jsonRPC_response}` → `bb_api/bb_api_jsonRPC/bb_{api_config_jsonRPC,api_jsonRPC_handler,api_jsonRPC_protocol,api_jsonRPC_request,api_jsonRPC_response}`（`package bb_*`）
       - `api/api_mcp` → `bb_api/bb_api_mcp`（`package bb_api_mcp`）
       - `api/api_mcp/api_config_mcp` → `bb_api/bb_api_mcp/bb_api_config_mcp`（`package bb_api_config_mcp`）
       - `api/api_websocket` → `bb_api/bb_api_websocket`（`package bb_api_websocket`）
       - `api/api_websocket/api_config_websocket` → `bb_api/bb_api_websocket/bb_api_config_websocket`（`package bb_api_config_websocket`）
       - `cli_proc` → `bb_cli_proc`（`package bb_cli_proc`，含 `testdata/`）
       - `common` → `bb_common`（`package bb_common`）
       - `common/{app_path,resource}` → `bb_common/bb_{app_path,resource}`（`package bb_app_path`、`package bb_resource`）
       - `config` → `bb_config`（`package bb_config`）
       - `custom_cmd` → `bb_custom_cmd`（`package bb_custom_cmd`）
       - `docs/docs.go`（`package docs`）+ 生成物 `docs/bb_service/api_methods.md` → `bb_docs/docs.go`（`package bb_docs`）+ `bb_docs/api_methods.md`（`//go:embed api_methods.md`，不复制双份、不移动 `docs/{bb_build,bb_service,bb_upstream}` 与 `docs/acceptance_plan_official_assets_v0.0.2.md`）
       - `frontend` → `bb_frontend`（`package bb_frontend`，含 `dist/`、`src/`、`scripts/`、`package.json`、`tsconfig.json`）
       - `frontend/frontend_config` → `bb_frontend/bb_frontend_config`（`package bb_frontend_config`）
       - `policy` → `bb_policy`（`package bb_policy`）
       - `policy/policy_config` → `bb_policy/bb_policy_config`（`package bb_policy_config`）
       - `runtime_resource` → `bb_runtime_resource`（`package bb_runtime_resource`）
       - `storage` → `bb_storage`（`package bb_storage`）
       - `storage/storage_config` → `bb_storage/bb_storage_config`（`package bb_storage_config`）
       - `test_ui` → `bb_test_ui`（`package bb_test_ui`，含 `flow_debug.html`、`web/`、`testdata/`）
    3. **子模块路径恢复（保留 gitlink commit ID 与内部内容不变）**：
       - `bb_sources/protenix`（`85767b811c40ed46e73a9b39519cf6bfca8701ba`）→ `build_srcs/protenix`
       - `bb_sources/pxdesign`（`f788441313c84c3074fe9596ac2433f96b15c763`）→ `build_srcs/pxdesign`
       - 同步 `.gitmodules`、索引 gitlink 路径、`scripts/repo_src_defaults.sh`、`scripts/build.sh`、`bb_build/*.yaml`。
    4. **默认 binary 与包/安装入口归位（`bb_service` → `bioBox`）**：
       - `bb_config/config.go`：`ProjectName="bioBox"`、`ProjectBundleID="com.bioBox.bioBox"`、`StorageCfg: bb_storage_config.Default("./data/bioBox.db")`（`ProjectVersion="v0.0.4"` 不变）；
       - `scripts/build.sh` `SERVICE_APP="bioBox"`；`scripts/gen_release_assets.go`、`scripts/check_bundle_resources.sh`、`scripts/tests/check_install_update_restore.sh`、`.github/workflows/release.yml`、`bb_suites/*/*-run`、`scripts/suites/*/install_prefix_*.sh`、`example_files/config_example.yaml`、`example_files/install_bb_service.sh` → `example_files/install_bioBox.sh`、`example_files/bb_service.service` → `example_files/bioBox.service`、`.run/bb_service-debug.run.xml` → `.run/bioBox-debug.run.xml`。
  - **旧 `bb_service` 安装身份 → 新 `bioBox` 升级/一致性风险记录**：
    - 若目标机器历史上通过旧安装包部署了 `/bb_service/`、`bb_service.service` 与 `./data/bb_service.db`，新整合包 `bioBox_linux_<arch>_release_v<version>.zip` 的安装脚本 `install_bioBox.sh`（`APP_NAME=bioBox`、`APP_DIR=/bioBox`、`bioBox.service`）**不会**自动迁移或静默删除 `/bb_service/` 与 `/etc/systemd/system/bb_service.service`；运维升级时需先停旧 `bb_service.service`、显式迁移配置/数据库（`conf/config.yaml` 中 `cli_cfg.*.binary` 指向 `/bioBox/runtime/...`、`storage_sqlite.path` 保持或迁移 DB）与权重数据，避免新旧两套 systemd unit 同时监听默认端口 `13001–13006` 冲突或旧 wrapper 仍指向 `/bb_service`。

- **`2026-11-25 11:55 HKT`｜AT-1 阶段二实施完成与验收实证（52 个非 `main` `bb_*` 包全量落盘 + `build_srcs` + 默认入口 `bioBox`；未 commit/push/tag/安装）**：
  - **模板参照绑定（`2990acfdcac78f1847901629f2c7410dbd36cc5e`）**：
    - 非阻塞补齐 `tmp/at1-prefix-rule-audit/excerpts/06_template_2990acf_blobs_and_stage1_golist.txt`（19 份模板文件 `git ls-files -s` blob SHA-1 与 `tmp/stage1-template-ref` 完整 `argv`/`cwd`/`numeric_exit_code=0`/`go list -e ./...`）并更新 `SHA256SUMS`（`71282f109e82b06076199040bdee0d1c92d06d22e92203835a6a8600c2dda993`），已同步拷贝至 `/workspace/biobox-at1-prefix-rule-audit/` 并以 `sha256sum -c SHA256SUMS`（`exit 0`）验真。
  - **实施范围与 wire/认证/子模块不变性**：
    - 全部 **52 个**项目自有非 `main` Go 包/子包均已按“每层加一次 `bb_`、保留层级与语义、不重复父名”迁移完毕（`bb_ability/*`、`bb_api/*`、`bb_cli_proc`、`bb_common/{bb_app_path,bb_resource}`、`bb_config`、`bb_custom_cmd`、`bb_docs`、`bb_frontend/*`、`bb_policy/*`、`bb_runtime_resource`、`bb_storage/*`、`bb_test_ui`）；`main`、`tools.go`、`scripts/gen_*.go`（`//go:build ignore`）、纯资源/文档目录（`bb_build/`、`bb_suites/`、`bb_prefix/`、`docs/{bb_build,bb_service,bb_upstream}/`）保留原位；
    - `bb_docs/docs.go`（`package bb_docs`，`//go:embed api_methods.md`）与 `bb_docs/api_methods.md`（由 `scripts/gen_api_docs.sh` 生成，25 methods）单源落位，`bb_test_ui/test_ui.go` 修正 `scripts/gen_test_ui.sh` 调用路径、优先读 `bb_docs/api_methods.md` 离仓回落 `bb_docs.APIDocFS`、且 `GRPCProtoImportPath="bb_api/bb_api_grpc/proto"`；
    - Auth/Wire 兼容零漂移：`APIKeyPrefix="ptxk_"`（未并入阶段一参照 `"bb_"`）、`DefaultSessionIssuer/Audience="protenix_service"`、`package protenix_service.api.v1;` 与 `APIService_Call_FullMethodName="/protenix_service.api.v1.APIService/Call"`；仅 `api.proto` `option go_package` 改为 `.../bb_api/bb_api_grpc/bb_protobuf;bb_api_grpc_protobuf` 并经 `scripts/gen_proto.sh` 原链重生成；
    - 子模块路径恢复为 `build_srcs/{protenix,pxdesign}`：gitlink object ID 保持 `85767b811c40ed46e73a9b39519cf6bfca8701ba` 与 `f788441313c84c3074fe9596ac2433f96b15c763`，上游内容零改动；索引仅更新此 2 条 gitlink 路径（`R100 bb_sources/* -> build_srcs/*`），未批量 stage 普通代码；
    - 默认产物与入口统一归位为 `bioBox`（`bb_config/config.go`、`scripts/build.sh`、`scripts/gen_release_assets.go`、`scripts/check_bundle_resources.sh`、`example_files/{bioBox.service,install_bioBox.sh,config_example.yaml}`、`bb_suites/*/*-run`、`scripts/suites/*/install_prefix_*.sh`、`.github/workflows/release.yml`、`.run/bioBox-debug.run.xml`），未新增任何 binary 覆盖参数。
  - **命令与 numeric exit codes（工作目录 `/home/certa/workspace/bioBox`）**：
    - `bash ./scripts/gen_pre_build_config.sh all` → `0`
    - `bash ./scripts/gen_proto.sh` → `0`
    - `bash ./scripts/gen_api_docs.sh` → `0`（写入 `bb_docs/api_methods.md`，25 methods）
    - `bash ./scripts/gen_frontend.sh` → `0`
    - `bash ./scripts/gen_test_ui.sh` → `0`
    - `go list -f '{{.ImportPath}} -> {{.Name}}' . ./bb_...` → `0`（53 个仓库包：`1` `main` + `52` `bb_*`；全仓未加前缀内部 import 扫描 = `0` 漏项）
    - `go vet . ./bb_...` → `0`
    - `go test -count=1 -race . ./bb_...` → `0`（含新增回归：`APIKeyPrefix=="ptxk_"`、`ProjectName=="bioBox"`/`com.bioBox.bioBox`/`./data/bioBox.db`、gRPC `/protenix_service.api.v1.APIService/Call`、`bb_docs.APIDocFS` 离仓回落与 `/api/config`）
    - `go run scripts/gen_release_assets.go version` → `0`（`v0.0.4`）
    - `go run scripts/gen_release_assets.go self-test` → `0`（58 checks PASS）
    - `python3 scripts/tests/test_release_cleanup.py` → `0`（12 cases PASS）
    - `python3 scripts/tests/test_release_guards.py` → `0`
    - `bash scripts/tests/test_source_staging.sh` → `0`
    - `bash scripts/tests/check_install_update_restore.sh` → `0`（`bwrap` 隔离夹具，`host_root_install=false`）
    - `bash ./scripts/check_native_build.sh arm64` → `0`
    - `bash ./scripts/build.sh pack release linux_arm64` → `0`（产出 `bioBox_linux_arm64_release_v0.0.4.zip`、`protenix_linux_arm64_release_v0.0.4.zip`、`pxdesign_linux_arm64_release_v0.0.4.zip`，既有 11 份 `v0.0.1–v0.0.3` 归档与 `v0.0.4`/`latest` git tag 均未覆盖或删除）
    - `bash scripts/check_bundle_resources.sh build/upload_tmp_dir/bioBox_linux_arm64_release_v0.0.4.zip` → `0`
    - 解包产物 `./bioBox version` → `0`（`名字: bioBox`、`包名: com.bioBox.bioBox`、`版本: v0.0.4`、`打包模式: Release`）
  - **保护基线复核**：
    - `example_files/config_local.yaml` (`684b9060...`)、`example_files/config_local_auth_bridge.yaml` (`d53a8a35...`)、`docs/acceptance_plan_official_assets_v0.0.2.md` (`de1dafe1...`)、`scripts/tests/__pycache__/test_release_cleanup.cpython-312.pyc` (`742ef011...`) SHA-256 全数一致；`HEAD` 仍为 `565a09f63c5762e332dd16fb8085cdd4067ab888`；`tmp/stage1-template-ref` 未改；未 commit/push/tag/发布/真实 root 安装。
  - **已验 vs 未验边界**：
    - 已验：全部 52 `bb_*` 包编译/`go vet`/`-race` 单测与协议回归、5 条生成链、`gen_release_assets.go self-test`、隔离 `bwrap` 更新/回退夹具、真实 `linux/arm64` release 打包与二进制 `version`/`resource status|prepare|verify` 门禁。
    - 未验：`linux/amd64` 原生 CGo/release 打包（本机为 `linux/arm64`）、真实 root `/bioBox` + `systemd` 安装与 GPU 上游推理、以及无头浏览器实打开 `/docs_api` 与 `/flow_debug.html` 交互渲染；此外 `tmp/at1-nonprod-iface-compat-logs/iface_client.go` 系 11-24 遗留临时文件（保护未删），因此用 `. ./bb_...` 限定仓内正式 Go 包执行 `go list`/`go vet`/`go test`。

- **`2026-11-25 12:20 HKT`｜AT-1 阶段二 M1 独立 review 修复与完整落盘留证（`install_bioBox.sh` 旧身份预检 + 16 项 `bwrap` 隔离夹具 + 独立目录重打包；仍为未提交 dirty 候选，未安装/未推送）**：
  - **M1 最小修复（`example_files/install_bioBox.sh`）**：
    - 在 `install()` 与 `updateBinary()` 执行任何副作用（`mkdir`、`systemctl stop/start/enable/restart`、移动/覆写配置或二进制、构建/下载运行时、`clear_install_files`）之前，新增只读 `legacy_identity_preflight()`（配合 `check_legacy_path()`）；
    - 检测覆盖：`/bb_service` 目录或 symlink（含悬空 symlink）、`bb_service.service` unit 文件/drop-in/`.wants`/`.requires`/symlink（涵盖 `/etc/systemd/system`、`/lib/systemd/system`、`/usr/lib/systemd/system`）及 `systemctl show -p LoadState,ActiveState,UnitFileState -- bb_service.service` 状态（运行中、未运行/stopped、disabled、masked 等）、以及已知旧 DB 路径（`/bb_service/data/{bb_service,protenix_service}.db`、`/bioBox/data/{bb_service,protenix_service}.db` 与 `/bioBox/conf/config.yaml` 对 `/bb_service`/`bb_service.db`/`protenix_service.db` 的引用）；
    - 缺工具或无法判明（如 `/` 或 `/etc/systemd/system` 不可读、缺少 `systemctl`、`systemctl show` 非零退出或返回空/不完整 `LoadState`/`ActiveState`）一律 fail-closed 拒绝继续，绝不当作不存在；发现任一旧项或不确定状态时清楚报出路径与 `bb_service` 身份并 `exit 1`，不自动停旧、删旧或迁移数据库；未新增 `--force` 跳过参数或额外公共环境变量，未改变调用契约与菜单。
  - **非阻断瑕疵与子模块核查**：
    - `bb_build/pre_build_config_template.yaml:6` 错误注释 `build_srcs/bb_prefix` 已修正为真实双目录分工（源码 `build_srcs/`、环境 `bb_prefix/`），并运行 `bash ./scripts/gen_pre_build_config.sh all`（`exit 0`）；
    - `.gitmodules` section 名 `[submodule "bb_sources/protenix"]` / `[submodule "bb_sources/pxdesign"]` 实测绑定 `.git/config`、`.git/modules/bb_sources/*` 及 `build_srcs/*/.git`（`gitdir: ../../.git/modules/bb_sources/...`），属于 git 内部稳定逻辑标识而非路径，核实保留未盲改，pins（`85767b811c40ed46e73a9b39519cf6bfca8701ba`、`f788441313c84c3074fe9596ac2433f96b15c763`）与 `git submodule status` 均正常。
  - **隔离夹具与回归验证（全程原始命令/cwd/stdout/numeric exit 落盘于 `tmp/at1-m1-fix-review/logs/`）**：
    - `scripts/tests/check_install_update_restore.sh` 扩充至 16 个 `bwrap` 隔离夹具（TDD 先红后绿：`01_tdd_red_*` `exit=1` 复现漏洞 → `02_tdd_green_*` `exit=0` 16/16 PASS），覆盖干净新装（`0`）、`bioBox` 自升级（`0`）、既有更新失败回退/混合态禁启（`1`/`1`）、`/bb_service` 目录与悬空 symlink、active/inactive/symlink/masked `bb_service.service`、已知旧 DB 与旧配置引用、缺失 `systemctl` 及 `systemctl show` 失败/空输出 fail-closed；
    - 完整回归与真实包校验：`gen_pre_build_config.sh all`（`0`）、`go list . ./bb_...`（`0`，53 包）、`go vet . ./bb_...`（`0`）、`go test -count=1 -race . ./bb_...`（`0`）、`gen_release_assets.go self-test`（`0`，58 checks）、`test_release_cleanup.py`（`0`）、`test_release_guards.py`（`0`）、`test_source_staging.sh`（`0`）、`check_native_build.sh arm64`（`0`）；
    - 保留 `build/upload_tmp_dir/` 原有全部 14 份 zip 原样不动（含前次 3 份 dirty 候选 `4a739c56...`/`1d5444f8...`/`156dab8f...`），将 M1 新打出的 3 份 dirty 候选 zip 独立存放在 `build/m1_dirty_candidates/`：
      - `bioBox_linux_arm64_release_v0.0.4.zip`（`96360997` bytes，SHA-256 `37a4628bbe1047d1ae425065a9cd0db327779244190051306959c5e500a2cd72`）
      - `protenix_linux_arm64_release_v0.0.4.zip`（`67983601` bytes，SHA-256 `74221a2f4bdadbec1607dac9129dbedc07450d3f16e8b6260d6cf1aae0242c9d`）
      - `pxdesign_linux_arm64_release_v0.0.4.zip`（`16626427` bytes，SHA-256 `6b2869edc4b8687494586b6a2b0eeb3884e5aa831ac1c15fc7d82c9644e516bb`）
    - 对新包执行 `check_bundle_resources.sh`（`0`）、解包 `./bioBox version`（`0`）、校验解包 `install_bioBox.sh`/`conf/config.yaml`/`bioBox.service` 与仓内文件 SHA-256 一致，并对解包出的 `install_bioBox.sh` 在 `bwrap` 下实跑旧目录拦截（`exit=1`）与干净安装（`exit=0`）。
  - **执行路径与历史日志诚实声明**：本轮与前轮源码修改均由 Grok Bot 经受支持 `machineId` 的 `Shell` 工具完成（前次已核 `acpx` 缺失且 `agent` CLI allowlist 仅放行 `Shell(ls)`，未向 `agent` CLI 发写入调用，无审批阻断）；前次短同步命令无独立落盘原始日志（不事后补造），本轮全部命令均经 `tmp/at1-m1-fix-review/run_logged.py` 完整落盘；仍未提交、未推送、未发版、未真实 root/systemd/GPU 安装，待独立 review 复核。

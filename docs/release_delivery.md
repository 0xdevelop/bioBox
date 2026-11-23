# 当前交付：三个公开附件与既有资源离线模式

更新：2026-11-23（UTC+8）。本次只实现、检查及正常提交推送；**没有运行 git_tag.sh，没有打 tag / 发布 / 替换或删除 v0.0.2 资产**。历史安装/GPU记录不能当作本次新包验收。

## 发布清单

公开附件只保留：

- `protenix_service_linux_amd64_release_v<version>.zip`
- `protenix_service_linux_arm64_release_v<version>.zip`
- `SHA256SUMS`：仅两行，覆盖上述两包，不再套娃校验其他清单。

当前配置版本仍 v0.0.2；后续得到门禁确认后，原有**无参** `git_tag.sh` 会按模板升到 v0.0.3。预期新附件为 `protenix_service_linux_amd64_release_v0.0.3.zip`、`protenix_service_linux_arm64_release_v0.0.3.zip`、`SHA256SUMS`。GitHub 自动 Source code 不属于自造附件；不会删除旧 v0.0.2 资产来假装新发布已经收缩。

`build.sh pack` 仍生成每架构的 protenix / pxdesign 独立包及整合包；两套独立 venv、安装/升级/回退脚本不删除。`arch` 在本架构检查三包并产生五项内部 CI artifact；`aggregate` 要求两架构十项内部材料，校验六包、BUILD_INFO、ELF/CGo、来源 commit 后，仅复制两个整合包并生成一份校验文件到全新空目录；`verify` 要求恰好三项并再次检查两包和校验；`uploaded` 也只接受三项远端名字/大小（不是远端内容重新下载证明）。

Release workflow 只上传 `tmp/release-assets/*`，不再把它复制回有内部/旧包的 `build/upload_tmp_dir`。非空输出目录/多余 stale 文件均拒绝。三段 release → Build（内含双原生 runner）→ RemoveOldRelease 不改，git_tag.sh 的无参入口/自动版本不改、无新参数；本轮只增量修复缺失旧tag/release时的幂等清理，真实错误仍失败。v0.0.2在本地和远端清理处均显式保留。

独立包改为内部产物，并未以删除独立安装路径来减少附件。但现有独立包没有随包原生资源协调入口时仍会明确失败，不能回退旧 curl。**独立安装完整闭环仍未验证，也不因保留脚本就宣称已修好**；需要独立交付时必须验证协调入口与配置绑定，而不是直接重新挂附件。

## 默认选择：本地内容pin与显式选择的公开下载源

成品 `conf/config.yaml` 来自 `protenix_service/example_files/config_example.yaml`。现默认显式启用完整的 5 项 Protenix + 11 项 PXDesign 清单，`download_enabled: false`，无持久化确认；14项现在有来自固定上游源码/官方Git仓库的HTTPS来源，默认不触发网络。摘要逐项沿用已记录并再次校验的 **USER_EXISTING_LOCAL_SNAPSHOT_ONLY (2026-11-23)** 内容基准，不是 ByteDance 官方发布的 SHA256/签名，更不是上游真实性或科学质量认证；采用此配置就是明确选择这些既有文件的内容 pin，不适用时须替换成所选来源的完整可信清单，不许关闭校验凑通过。

该选择源自本轮对已授权本机资源复用的执行范围，不是把本地基准改名成“官方清单”。精确16项 ID/size/SHA256、局部公开来源证据见 [原资源审查](resource_final_review.md#已授权用户既有本地资源现有可用方式)；运行时代码按稳定 ID 将整族路径编译到 runtime conf 根，避免在 YAML 反复写机器绝对路径。

正式安装根仍 `/protenix_service`，unit 仍 `User=root`；默认 binary 值保留官方安装路径，以兼容安装器在复制服务前调用解压目录协调器的次序。`work_root` 使用二进制同根 `./data/cli_jobs`；自配 binary/资源/cache 的相对值同样以二进制/runtime根解析，绝不以调用者 cwd 解析。没有改全局环境、profile、sudoers；上游所需变量仍只传子进程。

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
/protenix_service/protenix_service resource status protenix.v1
/protenix_service/protenix_service resource prepare protenix.v1
/protenix_service/protenix_service resource status pxdesign.preview
/protenix_service/protenix_service resource prepare pxdesign.preview
/protenix_service/protenix_service resource verify protenix.v1
/protenix_service/protenix_service resource verify pxdesign.preview
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

仍缺：基于最终提交的新双架构Actions包、从公开资产下载复核、正式根全新环境重建/安装、root/systemd常驻、真实v1/no-MSA与preview/no-MSA任务产物、卸载闭环；独立runtime协调入口/完整安装兼容性也未验收。未发现仓库外部审查完成证据，只有已标边界的本地源码/测试审查。本轮已修复模板/实例清理的不存在no-op：12项离线stub命令回归覆盖不存在、存在、权限/网络/服务器/删除错误；实例另验证v0.0.2保护分支。没有执行真实清理。候选构建改复用已有native/Build路径，见[候选构建](candidate_build.md)。

所以“实现与本地检查完成”不等于“允许现在声称正式发版验收完成”。本切片在主端收回证据前不执行git_tag.sh；受保护config_local仅hash，build_srcs/show_yeah未改；原未提交README验收计划链接和计划文件保留、不混提交。

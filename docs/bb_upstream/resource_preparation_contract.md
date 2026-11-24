> 最终审查更正（2026-11-23 14:03 UTC+8）：完整性不等于官方认证，已授权本地基准可用，不要求上游签发SHA256；生产默认/MSA兼容性尚未闭合。见 [最终审查与可用本地模式](resource_final_review.md)。

> 最新状态（2026-11-23 13:37 UTC+8）：已继续完成 M2 的配置角色绑定、CLI/API 确认与下载状态、安装单一传输所有者、默认门禁与 wrapper 资源根修正。以下 M1 内容保留为历史证据；其中“opt-in / 无确认入口 / debug cwd”等描述已由 [资源矩阵与当前边界](upstream_resource_matrix.md) 的当前实现取代。真实模型可信摘要仍缺，默认真实推理故意阻止，未宣称产品验收完成。

# 资源准备 / 推理启动门禁：契约与首个实现里程碑

时间：2026-11-23 12:36 UTC+8。当前为本地未提交实现，不是正式产品验收完成。
保留现有 README 与 acceptance_plan_official_assets_v0.0.2.md 未提交改动；不改 build_srcs、受保护配置、systemd，不安装、不跑 GPU，不下载真实权重。

## 工作流恢复与边界

两目标 checkout 没有项目 `.auto_coding/start_coding.md` / `.auto_coding/project.json` / TRACKER；已读本机 xjb_coding 技能源主引擎、模板与实例 CLAUDE.md / AGENTS.md、既有项目 .cursor/rules/boundaries.mdc。
宿主动态工具未发现 xjb_code namespace；没有读取凭据或用私有 HTTP 绕过连接工具。按 skill 使用本 markdown 保存当前断点，不重新初始化、不新建服务端项目/会话，不宣称已恢复服务端锁。服务端旧 session/task ID 未知，接回前要核对原状态。
本轮用户 t162u 明确授权两仓本地实现与有价值测试，覆盖旧 boundaries 中“不改模板 / 不写测试”的历史限制；仍保留禁止 sudo、build_srcs、正式安装和发版的边界。

## 1. 上游实证（只读）

证据根：已下载校验的 v0.0.2 整合包解压副本 `tmp/official-release-v0.0.2/inspect/service/protenix_service/runtime/`。
Protenix SRC_SHA=85767b811c40ed46e73a9b39519cf6bfca8701ba；PXDesign SRC_SHA=f788441313c84c3074fe9596ac2433f96b15c763。

- Protenix `src/protenix/runner/batch_inference.py:427` 调 `download_inference_cache`，并非“包装器不下载，所以推理入口也不下载”。`runner/inference.py:398-489` 对 CCD、RDKit mol、cluster、obsolete csv、模型 checkpoint 检测 `exists`，缺失就用 `urlretrieve` 下载；use_template 分支另需 release_dates/obsolete_pdbs；esm/ism 模型还会下对应 ESM 文件。上游 `exists` 不是强完整性验证。
- Protenix `protenix/web_service/colab_request_parser.py:102-135` 的 RequestParser 在 `download_data_cache` / `get_model` 也会下 CCD 与模型；默认根读取 `PROTENIX_ROOT_DIR`（44-47），未设时回用户 home。`configs/configs_data.py:298-306` 是 common 文件路径，`configs/configs_inference.py` 是 checkpoint 根。
- `runner/template_search.py:101-108`、`runner/rna_msa_search.py:257-300`、`protenix/data/template/template_utils.py` 有额外数据库/模板下载入口；MSA 远程结果下载又在 `web_service/colab_request_utils.py`。不能把默认无 MSA/v1 测试资源集冒充所有 Protenix 模式的完整防重。
- PXDesign `src/pxdesign/pxdesign/runner/inference.py:213` 调 `utils/infer.py:61-138` 的 `download_inference_cache`：下载 3 项 CCD/cluster，当前模型以及 Protenix base/mini/mini_tmpl v0.5.0 checkpoint。缺失判据为 `os.path.exists`。checkpoint 下载后尝试 torch.load；common 文件没有同样的结构验证。
- 已有 PXDesign venv 中 `site-packages/pxdbench/tools/ptx/ptx_utils.py:41-133` 还有 `download_infercence_cache`，会补下 PTX cache、checkpoint 及 ESM 变体。该安装是已有环境实证，不当作未来新安装依赖永久不变的保证。
- 已有 `pxdbench/globals.py` 的 `_require` 只检查文件存在；AF2 predictor (`tools/af2/af2_predictor.py:27-28,60-61`) 要求 `TOOL_WEIGHTS_ROOT/af2/params_model_1{,_ptm}.npz`。MPNN 路径包括 ca/vanilla/soluble_model_weights。AF2/MPNN 资源由 install 脚本下载/拉仓，不是这些 predictor 的自动修复能力。
- 安装脚本 curl 目前只有 --retry 3，失败删 .part，没有 Range/分片/官方摘要验证。此旧脚本本轮未改。

纠正先前核查的细节：依赖 gtbox v0.1.122 的 gtbox_app.NewApp 内部确实用 os.Executable 初始化 release AppRunAsDir / logs，但 main 随后把 AppConfigFilePath 覆盖成 ./conf/config.yaml，所以配置路径仍依赖 cwd。不得把“自有仓无 os.Executable”扩成“全依赖链完全没有”。

## 2. 所有者与启动契约

模板 common/resource 只懂文件 URL、路径、size、可信摘要、下载传输、确认与独占；模型名称、URL 列表、条件资源（MSA/template/ESM）属于实例，不能塞进 task/policy。
同一 canonical 目的文件仅一个准备所有者；同进程并发与跨进程通过稳定 flock 文件协调，不能 unlink 锁文件。不同路径但同一符号链接目的会归一化；硬链接别名跨资源 ID 并非已覆盖的统一所有权，不能宣称任意路径完全去重。
启动顺序：实例选 runtime/模型与完整资源集合 -> 只读校验配置或目录软链接上的已有文件 -> 缺失返回 awaiting confirmation -> CLI/前端明确确认（尚待接入） -> Go 准备/校验 -> 全部 ready -> 才可启动 Python CLI。
Go 准备进行时不能提前探测 Python CLI（包括触发 import/download 的“help”探针）。完整清单未核实的模式不得冒充全面防重。

## 3. 首个实现（代码已落地，验收结果在末尾更新）

模板先行：common/resource/{prepare.go,config.go,lock_unix.go,lock_other.go,prepare_test.go}；common/app_path/{root.go,root_test.go}。
随后同源移植到实例 protenix_service/common/。实例 CLIBinaryConfig / version runtime 新增可选 resources；cli_proc.Spec 接收配置，Run 在 exec.Command 之前先逐资源 Prepare；两个 ability 将所选资源配置送进同一 Run。缺省 enabled=false 保持老配置兼容，不能据此称旧默认推理已全面防重。

- 下载必须显式 download_enabled=true 且调用方 Confirmed=true；无 stdin/read，不会卡住 systemd。未确认返回 ErrConfirmationRequired，禁用返回 ErrDownloadDisabled。
- Confirmed 不接受从持久配置反序列化，需后续 CLI/前端确认流程传递；本轮没有新增对外协议或页面，daemon 当前返回明确错误，不持久等待。
- Range 仅在 HEAD 得到长度、Accept-Ranges=bytes 且强 ETag 时启用。分片并发有界（1-16）；If-Range / Content-Range / ETag 校验，不把 HTTP 200 拼接到分片；忽略 Range 安全降级单流。
- 断线保留本次已写入的分片；恢复重新 HEAD，比对 URL/digest/size/ETag/chunk manifest，变化就放弃旧片重新下；ETag 不可靠时最终摘要失败不发布。
- 必须提供可信 SHA256；size 可选但有值就校验；下载后验证全文件 SHA256 与长度，sync 后用同文件系统硬链接 no-replace 原子发布（不采用可覆盖的 rename），失败/取消无最终文件，不覆盖已有文件/软链接。复用校验成功不访问网络，也不要求写父目录。
- 官方 URL 文件未查到签名/hash manifest，故本轮不编造真实模型摘要，也不把文件非空或 TLS 当真实性证明。资源 File.SHA256 必须来自后续确立的可信上游摘要或用户明确钉住的已有文件摘要；后者只能证明与所选本地文件一致，不证明科学正确性。
- Options.retry_enabled 显式控制 retries；retry_count 是首次之外次数，0 不重试，绝不默认无限；不是复制 policy 调度器。下载 Progress 是字节计数和 downloading/verifying/ready，不写 task.progress，不冒充推理内部进度。
- app_path 锚到 os.Executable（解析符号链接），不 Chdir，不改全局 env。release main 配置文件路径与 SQLite、实例 work_root / binary / version binary 按可执行目录归一。debug 的 go run 历史行为本里程碑保留，不宣称全部资源路径铁律已完成。
- wrapper 内部相对 env 值目前仍依赖子进程 cwd，安装器的 PXDesign_DATA 与 env 配置读取仍有边界差异，留下一步处理，未偷偷改脚本。

## 4. 尚未覆盖（下个切片，不能标产品完工）

- 确认 CLI/前端 control path、确认状态持久化、明确 download 状态查询（不借 task.progress 冒充）。
- 真实模型资源清单、可信摘要来源/用户钉住摘要的交互，以及不同上游条件分支全覆盖。当前仅可选配置清单门禁，**尚不宣称默认真实 Protenix/PXDesign 的所有自动下载已被接管**。
- wrapper 相对资源 env 的统一解析、全量 debug/生成器资源根兼容、完整的结构化配置默认值文档与生成文档审阅。
- HEAD 不支持或无长度的源目前安全拒绝（不发无界请求）；缺少强 ETag 会单流，不能恢复续传；未知完整性拒绝下载。
- 运行期资源删除/篡改的外部竞争、硬链接别名锁归一、跨平台 Windows 原生锁未实现（明确报不支持，不假装互斥）。
- 原安装脚本仍独立下载，需后续把安装资源准备也接到单一所有者；此时不能并行运行旧安装器与 Go 资源准备。
- 尚未改前端视觉，不安装/启动/卸载、不发版，不跑真实 GPU。

## 5. 验收计划与状态

使用本地 httptest / 假 CLI、项目 tmp 下缓存。必须实测：共享准备（2 请求不重复网络）、跨进程互斥、下载未完成 CLI 标记不存在、未确认无网络无 CLI、断线续传、实体变化丢旧片、Range 忽略降级、分片并发上限、checksum 错误不发布、取消无 CLI、配置/软链接 ready 无网络、不同 cwd 找同二进制资源根。
已完成模板 common/resource 与 app_path 的 -race 测试；实例和受影响包的编译/测试也已通过，详见下节。完整业务与官方 assets 安装闭环不在这次本地测试结论中。

## 6. 续做断点

M1 ✅ 本地实现与测试里程碑完成（非产品整体完成）：模板 primitives 已同源移植，实例 CLI 门禁与受影响配置/ability 测试、两项目编译通过。未创建新 .auto_coding，不改旧服务端状态。下一步先读本文件并核对 git diff，补真实资源映射/确认流程；不要把可选门禁当成默认上游全面接管。

## 7. 实测记录（当前源码）

使用现有 Go 1.26.10 arm64；GOTOOLCHAIN=local、GOPROXY=off、GOSUMDB=off，GOCACHE / TMPDIR 均在相应项目 tmp/resource-tests 下。无新增依赖、未接外网下载真实资源。

- 模板：`go test -race ./common/... ./config` PASS；`go build -o tmp/resource-tests/project_template_go .` PASS。
- 实例（protenix_service 内）：`go test -race ./common/... ./config ./cli_proc ./ability/ability_protenix ./ability/ability_pxdesign` PASS；`go build -o ../tmp/resource-tests/protenix_service .` PASS。
- 核心 resource / app_path / cli_proc 曾用 `-race -count=3` 重复通过；最终又新增强 ETag 中途变化和重试次数测试并通过。
- 假 CLI 验证未确认不下载、下载阻塞期间不启动，校验完成后才启动。额外回归曾证实“ready 回调取消仍创建 PID”失败；现已在门禁后复查 context，回归断言结果无 PID 并通过。没有增加推理执行超时。
- 最终日志：模板 本地测试终端记录，实例 本地测试终端记录（均 exit 0）。
- `git diff --check` PASS；两项目通用新增源码逐字节一致；受保护 config_local.yaml 只做 hash 核验，仍为 `684b906075c72cd76806cafd645bb7da9c6ac6b235550b5e77eaf15f0e31bf6c`。
- 已有文件 SHA256 验证/本地分片合并尚非逐块 context-aware，取消可能等本地 IO 完成；不影响门禁在已观察到取消时拒绝启动，但响应延迟仍待改进。
- 测试只覆盖 httptest / 假 CLI；没有安装、GPU 推理、真实模型下载、systemd 启停、卸载、commit/push/tag，不宣称官方 assets 的业务闭环已通过。


## M2 最终实测补记（覆盖上文M1历史限制）

2026-11-23 13:37 UTC+8。配置→显式确认→跨进程单准备者→完整校验ready→CLI门禁已形成可测试纵向闭环。原鉴权/持久任务承载resource.status/resource.prepare，无新增服务或task进度协议；原生resource CLI在正常App启动前运行，不打开数据库/服务。所有模式均以二进制目录为配置/日志/自有相对资源根；wrapper相对资源/cache以runtime根解析，仅上游子进程env，不写全局环境。

- 全量最终 `go test -race ./...` 与build均通过：模板 本地测试终端记录，实例 本地测试终端记录。实例的真实原生PTY/跨OS进程CLI smoke也通过，结果 `protenix_build/tmp/resource-tests/native-cli-result.json`。
- HEAD不支持/无长度但manifest有可信长度可安全有界单流GET；Range不可靠时不续拼。取消检查已进入本地hash/合并。已存在但无效文件拒绝覆盖；整集合预检先于首个网络请求。
- 新增普通tar来源能力：可信归档摘要与成员摘要分别验证，共享同族归档payload锁/缓存，普通regular成员按指定路径读取而非通用目录解压，验证后原子no-replace发布；隐式归档缓存不跟随软链接。缓存/分片会增加峰值磁盘，不支持gzip，不编造真实AF2摘要。
- 快照是最后观测而非进程存活；集合确认绑定集合名和完整配置，配置/目标改变使旧确认失效。不同canonical目标/硬链接别名不承诺全局内容去重，已有硬链接/外部任意突变不是安全沙箱保证。
- 实例的有限默认v1/no-MSA与preview/no-MSA资源知识及Python自动下载分支、生产清单信任缺口详见protenix_build/docs/upstream_resource_matrix.md；未核查模式显式阻止，不是opt-in绕过。任意自配外部CLI只有显式manifest门禁，不等于官方Python防重证明。
- 原历史v0.0.2资产不可变，未重新打包/安装，不能宣称旧包已获得新行为。前端展示适配后续可接原Bun+TS，不改视觉；确认准备不会暗中重跑先前失败推理任务。
- 未执行sudo/systemd/安装/卸载/大权重下载/GPU/commit/push/tag，未修改受保护配置/build_srcs/show_yeah。保留原README及原官方资产验收计划。

### 现有权重仅改配置value的只读实测（2026-11-23 13:36 UTC+8）

已在项目tmp一次性配置中，将runtime env根value指向既有本地权重，完整读取16个所需文件（6,571,265,681 bytes，约6.12GiB）形成明确标记 `LOCAL_EXISTING_FILE_SNAPSHOT_ONLY` 的内容基准；这不是官方/可信上游SHA256，不写入生产默认。对构建出的真实原生二进制，从另一cwd、无TTY执行两个集合的status和prepare，均返回ready：未copy权重、未创建资源目录软链接、未下载、未复用旧venv或启动Python/GPU。源文件inode/size/mtime保持不变。

只验证权重存在及与指定本地快照相等，不能说完整runtime已ready：临时布局故意没有新建venv，两个集合的runtime verify均正确拒绝，stderr明确Python was not started。后续venv仍按官方安装构建，不以既有venv冒充新资产安装验收。

证据：`tmp/resource-tests/<local-baseline-dir>/result.json`；旁边local-baseline.json包含本地来源/长度/摘要，config.json与runtime/*/conf/*.env展示单配置族value复用，不是产品强制prefix。下载开关false、URL未填，避免该只读实验意外联网。复现日志 本地测试终端记录（exit0）。

### 追加代码审查回归

恢复后审查发现：Range恢复时，旧staging中的manifest/part/complete如果是软链接或硬链接，原来的普通open/append/truncate可能跟随并破坏无关文件。现已对内部staging采用O_NOFOLLOW、先fstat确认regular且单链接再truncate/append，O_NONBLOCK避免FIFO阻塞；真实配置资源路径仍允许目录/文件软链接只读复用，不把两者混淆。补6组软/硬链接回归验证无关文件内容不变且不发布，以及资源/内部文件FIFO非阻塞拒绝。不是对同UID恶意并发任意改目录的完整沙箱承诺。

### 最终恢复断点 2026-11-23 13:48 UTC+8

上述staging安全修正后的最终全量race/build也PASS：模板 本地测试终端记录，实例 本地测试终端记录（含最新真实PTY/跨进程CLI smoke）。最终补丁为各项目 `tmp/resource-tests/resource-preparation-final.patch`，审查清单 `tmp/resource-tests/review_manifest.json`；不含原README改动/原assets验收计划。当前模型生产默认清单尚无已确立的真实性来源，不自动补假摘要；已授权的本地权重快照仅用于只读内容相等性实验。后续仍需新assets正式安装/业务/卸载验收及明确清单信任边界，不重打旧tag。

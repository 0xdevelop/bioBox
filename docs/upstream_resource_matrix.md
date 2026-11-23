> 最终审查更正（2026-11-23 14:03 UTC+8）：完整性不等于官方认证，已授权本地基准可用，不要求上游签发SHA256；生产默认/MSA兼容性尚未闭合。见 [最终审查与可用本地模式](resource_final_review.md)。

# 已安装上游资源矩阵与当前受管边界

更新：2026-11-23 13:37 UTC+8。本文件记录只读证据与尚未正式安装/业务验收的实现；不是科学结果保证。历史 v0.0.2 zip 未修改；本轮变更仅在本地 checkout，尚未重新打包/提交/发版。

## 版本与证据根

- 包内 Protenix SRC_SHA `85767b811c40ed46e73a9b39519cf6bfca8701ba`；本机 prefix/protenix venv 分发版本是 **2.0.0**，不等同于产品 API 的 v1/v2。已安装 runner/inference.py 与包内文件逐字节一致，源码 SHA256 `9cf94eb80124056e827ad839fdc78c432955d9108c3ad08dfe813a420fde25e3`。
- 包内 PXDesign SRC_SHA `f788441313c84c3074fe9596ac2433f96b15c763`；本机版本 0.1.0，editable 源 `prefix/pxdesign/src/pxdesign-f788441313c8`；utils/infer.py 与包内一致，源码 SHA256 `de07913dfdc266bdca6b9641ec1617536567064f401a031e078200caac9a44ff`。
- PXDesign 的 Protenix 分发 0.5.0+pxd，installed commit `d18aa1daadd02a001b32bc7fa2278fc8a2f8f025`；pxdbench 0.1.2 commit `f6d0d72496c23ca1374df5943754f3454ae8c549`；ColabDesign 1.1.3 commit `8fd167defd1b0f98e29f00584ff4f3987e6935dc`。取自已安装 dist-info 的版本/commit 字段，未运行 Python。安装脚本默认 ref 现固定为这些已核查 commit，未更换已安装版本。
- 下表 Protenix/PXDesign 源根是 `tmp/official-release-v0.0.2/inspect/service/protenix_service/runtime/<app>/src/<app>/`；依赖源根是已有 `prefix/pxdesign/venv/lib/python3.14/site-packages/`。这些路径仅证据，不是产品强制开发 prefix。
- runtime_resource/installed.go 的哈希是**已审阅源码相等性/漂移检测**，不是官方模型哈希、签名、完整 Python 沙箱或权重真实性证明。

## 资源矩阵

缩写 P=运行时 conf/protenix.env 的 PROTENIX_ROOT_DIR；D=conf/pxdesign.env 的 PXDESIGN_DATA；C=PXDESIGN_CHECKPOINT_DIR；T=TOOL_WEIGHTS_ROOT；X=PROTENIX_DATA_ROOT_DIR。相对值都相对 runtime 根，不相对 cwd。Protenix 默认 P=`/protenix_service/runtime/protenix/data`；PX 默认 D=`/protenix_service/runtime/pxdesign/data`，C=D/release_data/checkpoint，T=D/tool_weights，X=D/protenix_data。

| 配置/调用条件 | 目的资源 | 上游触发与出处 | 当前 Go 就绪/接管判定 |
|---|---|---|---|
| Protenix pred，任何模型 | P/common/components.cif、components.cif.rdkit_mol.pkl、clusters-by-entity-40.txt、obsolete_release_date.csv | runner/batch_inference.py:427 调 runner/inference.py:398–423；逐项 exists，缺失 urlretrieve。路径 configs/configs_data.py:298–306；URL protenix/web_service/dependency_url.py:29–32 | 有限 v1 no-MSA profile 必须列全4项，可信 SHA256 验证；否则拒绝 Python。上游非空/exists 不是 Go 的 ready |
| 默认 v1 模型 | P/checkpoint/protenix_base_default_v1.0.0.pt | runner/inference.py:445–454 缺失下载；URL 字典:17 | role=model；必须与 env 模型和请求一致；与前4项全部 ready 后启动 |
| RequestParser/网页转换入口 | P/common 与 checkpoint | protenix/web_service/colab_request_parser.py:102–135；根:44–47，不设 env 会回 home | 服务只开放已核查 pred 子集；包装器拒绝额外未受管子命令，不以 help/import 探针冒充 ready |
| use_template=True | P/common/obsolete_to_successor.json、release_date_cache.json，以及 mmcif / 模板检索缓存 | runner/inference.py:425–443；runner/template_search.py:101–108；configs/configs_data.py:281–295；template_utils.py 有 mmCIF 下载 | **未支持/拒绝**，不是仅加两个文件就宣称完整接管 |
| use_msa=True / RNA MSA | MSA 服务返回包、seq_to_pdb_index、RNA sequence→chain/msa 目录等 | runner/batch_inference.py:114–134；runner/rna_msa_search.py:257–300；web_service/colab_request_utils.py 的 HTTP 提交/轮询/结果下载 | **未支持/拒绝**，必须显式 use_msa=false。服务结果获取不是“模型下载进度” |
| esm / ism 模型 | P/checkpoint/esm2_t36_3B_UR50D{,_ism}{,-contact-regression}.pt 对应分支 | runner/inference.py:456–489；URL 字典:23–28 | **未支持/拒绝**，不猜模型变体资源集合 |
| protenix-v2 / 其他官方模型 | 授权/其他模型 checkpoint，可能不同条件资源 | URL 字典和对应 model configs；v2 授权事实不能由本地哈希代替 | 当前官方 wrapper profile **拒绝**；不偷偷回退 v1，也不伪造授权 |
| PXDesign inference / pipeline | C/pxdesign_v0.1.0.pt，加 protenix_base_default_v0.5.0.pt、mini_default、mini_tmpl | pxdesign/runner/inference.py:213 调 utils/infer.py:108–131；即便 preview 也会预取后3个 | role=model 与3个具体 .pt ID，必须全列全哈希，不能仅准备 PXDesign 一个模型 |
| PXDesign common | X/components.v20240608.cif、对应 rdkit pkl、clusters-by-entity-40.txt；若无日期版本存在则优先用无日期版本 | pxdesign/configs/configs_data.py:19–46；utils/infer.py:90–106 | 编译 role 时按同样优先规则选择实际路径，不能一边检查 versioned、一边让 Python 使用未校验 unversioned |
| preview 的 AF2 | T/af2/params_model_1.npz、params_model_1_ptm.npz | pxdbench/tools/af2/af2_predictor.py:27–28、60–61 _require；colabdesign/af/alphafold/model/data.py:31–41 仅本地 np.load | role=af2_model_1 / af2_model_1_ptm，可信摘要验证；缺失不让 Python 启动。原官方来源是 GCS alphafold_params_2022-12-06.tar；现已支持普通 tar 的独立归档摘要 + 成员摘要验证、同族共享归档缓存/锁、仅提取指定 regular member 并原子发布；不能拿 archive hash 当提取后 npz hash |
| preview/MPNN 家族 | T/mpnn/ca_model_weights/v_48_020.pt、vanilla_model_weights/v_48_020.pt；额外变体需另核查 | pxdbench/globals.py:40–43；pxd_configs/eval.py:25–29 的默认 ca；vanilla_mpnn_predictor.py:32 检查选定模型 | 有限默认资源要求 ca/bb 两个角色；不再 git clone 下载一整套同目标，其他模型设置未开放给受管 wrapper |
| ColabDesign 的 binder MPNN | venv 中 colabdesign/mpnn/weights/v_48_020.pkl；soluble 变体在 weights_soluble | pxdbench/tools/protmpnn/main_mpnn.py:54–58、130–133；colabdesign/mpnn/model.py:24–35 从包内目录读取，无独立网络修复 | 属于固定 Git 分发的 venv 包数据，不是 Go 再下载一份 .pt 的替代品；源码漂移受检，但没有冒称已获得这些 bundled 权重的独立官方 SHA256/来源证明。完整环境/业务验收仍待做 |
| preview 的 PTX eval | preview 本应关闭 PTX filters | pxdesign/runner/cli.py:290–304 显式关闭；runner/pipeline.py:120–154 只有 use_ptx_filter 才走 target template | 只支持 preview 且显式 use_msa=false；wrapper 拒绝自定义 eval.* / checkpoint CLI 覆盖，防止绕过清单 |
| extended/custom/PTX/模板 | X/common、C 下 PTX/ESM、MSA/目标模板分支 | pxdbench/tools/ptx/ptx_utils.py:41–133；helpers.py:693–725 中 use_msa=True；pipeline.py:131–150 可能切 mini_tmpl | **未支持/拒绝**。不能因为 preview 有界就声称 extended 的所有下载已接管 |

## 单一配置族与最小入口

资源 URL/摘要由实例配置，不进入 task 引擎。资源配置可以用稳定 ID 而省略 path：修改 runtime `.env` 的一个根 value，即由实例 Compile 将同族所有 ID 绑定到新根。既有目录软链接也归一到真实目标；不创建/覆盖软链接，不必 copy，不需要 WEIGHTS_REUSE_FROM。

Protenix IDs：`ccd_components_file`、`ccd_components_rdkit_mol_file`、`pdb_cluster_file`、`obsolete_release_data_csv`、`model`。PXDesign IDs：前三个 common IDs、`model`、`protenix_base_default_v0.5.0.pt`、`protenix_mini_default_v0.5.0.pt`、`protenix_mini_tmpl_v0.5.0.pt`、`af2_model_1`、`af2_model_1_ptm`、`mpnn_ca`、`mpnn_bb`。

配置形状示意（占位摘要不是可用默认值）：

```yaml
cli_cfg:
  protenix:
    binary: runtime/protenix/bin/protenix-run
    resources:
      enabled: true
      download:
        download_enabled: true
        retry_enabled: true
        retry_count: 2 # 首次之外最多2次；false/0不重试
        parallelism: 4
        chunk_size: 8388608
      files:
        - id: model # 其余必需ID必须补齐，缺一项即阻止Python
          url: https://protenix.tos-cn-beijing.volces.com/checkpoint/protenix_base_default_v1.0.0.pt
          sha256: '<必须来自明确可信来源/操作者明确钉住的内容摘要>'
          size: 0 # HEAD不可用/无长度时，必须给可信正长度，方可安全单流GET
```

- 正常 CLI：`./protenix_service resource status protenix.v1`；`resource prepare protenix.v1` 只在真实 TTY 提示 yes/no，无 TTY 返回待确认而不读 stdin；明确的 `--confirm <status指纹>` 只批准该集合。支持 SIGINT/SIGTERM 取消；不启动服务/数据库/systemd。
- API 继续使用统一 tools/call、既有 Bearer 鉴权：resource.status 同步只读；resource.prepare 携 set + confirmation_token，沿用现有持久异步任务；task.get 管任务生命周期，resource.status 管下载字节快照，互不冒充进度。
- 正确顺序：先 status → 用户确认所指集合 → prepare 返回 task_id → status / task.get → ready → 正常 protenix / pxdesign 请求。缺权重的直接推理不会悄悄下载，也不改 task 状态机冒充挂起；原推理任务可能明确失败，确认准备后需重新提交推理请求，不会暗中自动重跑科学计算。
- 对受管默认路径，nil/disabled/空 manifest 不再放行。准备完成后仍要验证完整清单、允许的调用条件、被审阅的运行时源码，再启动 CLI。配置任意外部 executable 只能获得其显式通用 manifest 门禁，**不属于官方 Python profile 的完整性/防自动下载证明**。

## 安装/安全边界与真实阻塞

- 新源码两套 prefix 安装脚本已移除模型 curl、AF2 tar 流和 MPNN clone/copy 权重分支；只调用整合包同级官方服务二进制的 resource prepare。没有独立 downloader 兜底；SKIP_WEIGHTS=1 只用于环境构建，不表示模型 ready。WEIGHTS_REUSE_FROM 的旧 copy hook 对受管准备明确拒绝，不暗中复制源目录。
- 默认依然是 `/protenix_service/runtime/<app>`；没有改成开发 prefix 或 ~/.local/share。独立 runtime 包若不携服务原生协调入口，会明确失败，不回退旧下载器；这项兼容性变化尚未做真实打包/安装验收。
- 包装器验证自身与配置 binary 绑定，先经原生只读 verify，再加载仅含字面赋值的 env；相对资源/cache 路径相对 runtime 根。清除继承的自有资源根/PYTHONPATH/PYTHONHOME，必要的上游 env 仅传给子进程；不写 profile、全局环境变量或 sudoers。
- 进度快照位于二进制根 data/resource_state，含更新时间；是最后观测，不是下载进程存活保证。相同集合有稳定锁，跨集合/跨进程同一 canonical 文件另有文件锁。中断分片可以恢复；已有最终文件不匹配则拒绝覆盖、保留来源，要求明确处理，不自动删掉重下。
- 已检查的包内 URL 字典/本地权重目录没有可直接采用的官方 SHA256 manifest；没有把 TOS URL、TLS、文件非空、自己计算的本地模型 hash 说成上游真实性证明。**实际默认模型 manifest 故意未填假 hash，当前真实默认调用被阻止，而不是已通过业务验收。**
- 生产清单后续需明确可信上游摘要来源，或采用明确标注来源的操作者内容基准（只能证明内容相等，不能证明上游真实性/科学质量）。已授权的现有权重复用已另做只读本地快照验收，不把该快照冒充生产默认/官方摘要，详见末节。AF2 原始 tar 与提取后的 npz 不能混用摘要；bundled venv 权重的独立来源证明不在已完成声明内。
- 不并行运行未改的历史 v0.0.2 安装器/手工 Python 与新准备器；旧 zip 本身仍保留旧下载行为。本轮没有重打 tag 或替换资产，不能声称历史发行包已获得新门禁。
- 不sudo、不安装/卸载、不启停systemd、不下载真实大权重、不运行GPU；既有权重、venv、config_local.yaml、build_srcs 均未改。root 承载能力没有因此被禁止；原官方 unit User=root 事实不变，不是要求换普通用户。

## 普通 tar 资源配置与磁盘边界（M2）

仅支持普通 tar；不是任意归档解压器，不支持 gzip。归档及提取成员分别校验，缓存为目的成员同族目录 `.resource-archives/<archive-sha256>.tar`，两个 AF2 成员共享一个归档传输所有者。隐式归档缓存目录若是软链接/非目录则拒绝，避免额外写入未声明目录；用户显式配置的资源根目录软链接仍可复用。归档保留在缓存中，需考虑完整 tar、分片/合并 staging、目标 npz 的峰值空间，不能只按两个 npz 大小估算。

```yaml
# 位于 PXDesign resources.files；其他必需ID也须完整声明
- id: af2_model_1
  sha256: '<可信的提取后成员摘要，不是tar摘要>'
  size: 0 # 建议声明实际成员长度；0仍要求完整成员SHA256
  archive:
    url: https://storage.googleapis.com/alphafold/alphafold_params_2022-12-06.tar
    sha256: '<独立可信的整个归档摘要>'
    size: 0 # HEAD不可用时须声明可信正长度
    entry: params_model_1.npz
# af2_model_1_ptm: 同一归档metadata，entry=params_model_1_ptm.npz，独立成员摘要
```

以上均是占位示意，不是已经获得的官方摘要；已有 npz 校验通过即复用，不要求获取归档。

## 最终本地闭环与续做断点

- 全套 `go test -race ./...` 和二进制构建：模板与实例均 PASS，日志 本地测试终端记录、`121562.txt`；实例还通过只读已安装源码漂移核验。2026-11-23 13:34:46 UTC+8 已全部结束，没有把旧测试进程盲目重跑当作恢复。
- 真实构建的原生 CLI：真实 PTY yes/SIGINT(130)、无TTY立即待确认、错误集合token拒绝、HEAD405安全GET、两原生OS进程同一新资源仅1次GET、第三status进程见真实非零字节、ready之前无最终文件、不同cwd仍二进制根定位。记录 `tmp/resource-tests/native-cli-result.json`，不是只用单元测试拼接推断。临时HTTP/子进程已关闭。
- 鉴权集成 `TestAuthenticatedResourceConfirmationToInference`：真实临时SQLite/Bearer契约、未鉴权拒绝、两个既有持久prepare任务只1次GET、下载过程中不启动假CLI、其他集合不下载，全部ready后正常提交推理才产出假CIF。下载字节不写成推理task.progress。
- 故障覆盖包括断线续传、Range忽略/ETag改变不拼坏数据、有界分片、跨进程锁、哈希错误/取消不暴露半成品、不完整现有最终文件保留并拒绝覆盖、HEAD405/501或无长度且manifest有界时安全单流GET、归档双哈希/成员路径/取消/共享传输/隐式缓存软链接拒绝。大文件哈希与合并已逐块检查context；已完整原子发布的有效文件可能在稍后取消后保留，不是半成品泄露。
- 可续做点：模型清单生产信任元数据和新assets打包/真实安装/业务/卸载验收仍未完成；不能把本轮httptest/假CLI或只读文件相等性当科学结果。模板/实例通用源码保持逐字节相同；未增加新依赖。

### 现有权重仅改配置value的只读实测（2026-11-23 13:36 UTC+8）

已在项目tmp一次性配置中，将runtime env根value指向既有本地权重，完整读取16个所需文件（6,571,265,681 bytes，约6.12GiB）形成明确标记 `LOCAL_EXISTING_FILE_SNAPSHOT_ONLY` 的内容基准；这不是官方/可信上游SHA256，不写入生产默认。对构建出的真实原生二进制，从另一cwd、无TTY执行两个集合的status和prepare，均返回ready：未copy权重、未创建资源目录软链接、未下载、未复用旧venv或启动Python/GPU。源文件inode/size/mtime保持不变。

只验证权重存在及与指定本地快照相等，不能说完整runtime已ready：临时布局故意没有新建venv，两个集合的runtime verify均正确拒绝，stderr明确Python was not started。后续venv仍按官方安装构建，不以既有venv冒充新资产安装验收。

证据：`tmp/resource-tests/<local-baseline-dir>/result.json`；旁边local-baseline.json包含本地来源/长度/摘要，config.json与runtime/*/conf/*.env展示单配置族value复用，不是产品强制prefix。下载开关false、URL未填，避免该只读实验意外联网。复现日志 本地测试终端记录（exit0）。

### 追加代码审查回归

恢复后审查发现：Range恢复时，旧staging中的manifest/part/complete如果是软链接或硬链接，原来的普通open/append/truncate可能跟随并破坏无关文件。现已对内部staging采用O_NOFOLLOW、先fstat确认regular且单链接再truncate/append，O_NONBLOCK避免FIFO阻塞；真实配置资源路径仍允许目录/文件软链接只读复用，不把两者混淆。补6组软/硬链接回归验证无关文件内容不变且不发布，以及资源/内部文件FIFO非阻塞拒绝。不是对同UID恶意并发任意改目录的完整沙箱承诺。

### 最终恢复断点 2026-11-23 13:48 UTC+8

上述staging安全修正后的最终全量race/build也PASS：模板 本地测试终端记录，实例 本地测试终端记录（含最新真实PTY/跨进程CLI smoke）。最终补丁为各项目 `tmp/resource-tests/resource-preparation-final.patch`，审查清单 `tmp/resource-tests/review_manifest.json`；不含原README改动/原assets验收计划。当前模型生产默认清单尚无已确立的真实性来源，不自动补假摘要；已授权的本地权重快照仅用于只读内容相等性实验。后续仍需新assets正式安装/业务/卸载验收及明确清单信任边界，不重打旧tag。

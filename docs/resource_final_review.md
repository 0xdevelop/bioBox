# 资源准备最终审查：完整性、来源与迁移边界

更新：2026-11-23 14:03 UTC+8。这是未提交源码/本地测试的审查，不是整合资产安装、GPU业务或科学验收完成。

## 结论与默认兼容性

**不能把“上游没有官方SHA256”当作所有可用路径的先决阻塞。** 当前实现只要求配置明确钉住SHA256，并没有“必须官方签发”判断；已授权的用户既有文件可采用明确标注的本地内容基准，完整校验通过就复用，无网络、copy或Python探针。来源真实性与文件完整性是两项不同保证。

当前新源码的真实默认调用仍不能称完成：旧默认配置没有完整启用的资源清单；此外未显式use_msa=false的调用、v2/template/RNA-MSA/ESM/extended等未协调分支也会拒绝。即使填了摘要也不会使这些模式自动受支持。旧版“文件exists就启动/缺什么由Python下载”被改为“先确认、准备、校验再启动”，这是明确的兼容性变化，不应包装成原默认无变化。

- 保留的结构：原统一tools/call、鉴权、持久任务/policy、推理执行超时默认关闭；没有新增服务/执行器/task状态枚举/推理进度协议。
- 必须迁移：为所用集合配置完整resources；自有路径按二进制/runtime根；不再靠cwd、debug隐式config_local、继承的模型根env或WEIGHTS_REUSE_FROM。CLI无TTY缺资源返回待确认，不读stdin。prepare完成后需重新提交原先失败的推理，不自动重跑。
- 受支持profile是明确的Protenix v1/no-MSA和PXDesign preview/no-MSA，不是默认MSA分支已完成。任意外部自配CLI只有其声明manifest门禁，不属于官方Python完整防下载证明。
- 独立runtime包若没有服务原生协调入口现在会失败，不回退旧curl。此兼容性与新整合包布局尚未真实安装验收。原v0.0.2 zip未改，不声称旧资产获得了新行为。
- 官方根仍为/protenix_service，unit仍User=root；没有新增必须普通用户的检查或全局env写入。

## 已核实的公开来源（不把局部证据说成完整认证）

| 来源 | 实际证据 | 可以/不可以证明 |
|---|---|---|
| Protenix固定85767b811c40ed46e73a9b39519cf6bfca8701ba、PXDesign固定f788441313c84c3074fe9596ac2433f96b15c763 | 官方完整tree均未截断；未见模型对象、LFS配置或checksum/manifest命名文件；URL字典只有URL，未见完整16项摘要表 | 仅说明核查范围内没找到完整清单，不宣称全网不存在 |
| GitHub releases | Protenix最近30个release（含v0.5/v1/v2）assets均空；PXDesign列表空 | 没有可采用的这些release模型asset digest；不是模型内容无来源 |
| 官方TOS HEAD | v1、PXDesign、两类CCD均200，给Content-Length、multipart ETag、x-tos-hash-crc64ecma | 有服务端CRC64完整性元数据！不是SHA256；带-分段数的ETag不能当文件MD5，更不能当SHA256或签名 |
| 官方GCS AF2对象 | generation=1670353726160664，size=5587968000，md5=5cc59a561e80601862948ff36c6b040b，CRC32C=YiBdIg== | 是归档级服务端校验元数据；不能推导两个npz成员摘要。现通用组件不支持以MD5/CRC替代配置SHA256，本轮不暗降保证/加算法 |
| 官方ProteinMPNN | commit8907e6671bfbfc92303b5f79c4b5e6ce47cdef57；tree a0b3b3178eb4d6cc10b4de5594a9e06b2d0243d5；ca Git blob 7fd96c7abf26621fad0f4352e3aad15e452251b4、bb 66f81553eea491dfc1307582c679ef029511937b | 两个现有本地文件计算Git blob SHA1均匹配，提供官方固定Git对象佐证；本地SHA256仍标记为本地派生，不伪称官方发布SHA256/签名，且Git SHA1保证不等同SHA256 |
| ModelScope OneScience/protenix | 匿名公共API176项，7个LFS；weight/model_v0.5.0.pt在Revision97c151ef71797123f31b79919e4ff4227ac1d3ac的Sha256=9ea20b0aba42f2256711da1d0cd081510a4b291e64375bff6b70ced70b87a5f1，与本地PXDesign base-v0.5角色相等 | 存在可核实的平台LFS摘要，而非“ModelScope没有hash”。但没有证实OneScience为ByteDance官方发布身份；只覆盖其中一项，不自动升级为全套官方认证 |

公开出处：
- https://github.com/bytedance/Protenix/tree/85767b811c40ed46e73a9b39519cf6bfca8701ba
- https://github.com/bytedance/PXDesign/tree/f788441313c84c3074fe9596ac2433f96b15c763
- https://storage.googleapis.com/storage/v1/b/alphafold/o/alphafold_params_2022-12-06.tar
- https://github.com/dauparas/ProteinMPNN/tree/8907e6671bfbfc92303b5f79c4b5e6ce47cdef57
- https://modelscope.cn/api/v1/models/OneScience/protenix/repo/files?Revision=master&Recursive=true

HEAD仅取响应头，body读取0字节。详细原始小元数据位于tmp/resource-tests/provenance；文中固定版本/关键值不依赖tmp保留。未采用搜索到的第三方HF manifest充当官方来源。

## 已授权“用户既有本地资源”现有可用方式

无需新增trust开关：显式采用下述本地基准清单，download_enabled=false；这不是免校验，文件缺失/字节不符仍拒绝。SHA256完整性只证明与所选本地文件基准一致；是否作为本次工程验收输入是使用已有本地资源的选择，不构成上游认证或科学质量保证。

资源路径仍只由runtime conf的根value决定，清单用ID而不重复path：
- Protenix：PROTENIX_ROOT_DIR → 已有data目录。
- PXDesign：PXDESIGN_DATA，以及显式的PXDESIGN_CHECKPOINT_DIR、TOOL_WEIGHTS_ROOT、PROTENIX_DATA_ROOT_DIR。**如果后三项已经写了绝对值，只改PXDESIGN_DATA不会覆盖它们**；改各对应value，或将三个override值留空让它们从DATA派生。Go和wrapper的空值派生行为一致。
- 可指向既有目录软链接；不替用户创建/覆盖目录或软链接。不更改正式安装根，不用开发prefix当产品强制路径。

实测采用的源码路径仅是当前机器已存在的资源来源：<local-prefix>/protenix/data，以及prefix/pxdesign下release_data/checkpoint、tool_weights、protenix_data；没有copy或复用其venv。16项共6,571,265,681 bytes；原生二进制从不同cwd、无TTY执行status/prepare，两集合均ready，源inode/size/mtime不变。新布局无venv时runtime verify正确拒绝，Python没有启动：权重ready不等于环境/业务ready。

以下是**需明确合入所选配置的片段，不是可独立运行的完整配置文件，也没有修改受保护配置**。来源标签为USER_EXISTING_LOCAL_SNAPSHOT_ONLY（2026-11-23），不是官方摘要；原鉴权等配置仍须保留。没有URL且下载关闭，缺失不会隐式联网。

```yaml
# USER_EXISTING_LOCAL_SNAPSHOT_ONLY (2026-11-23): local content equality baseline, NOT upstream-official authenticity
cli_cfg:
  protenix:
    resources:
      enabled: true
      download: {download_enabled: false, retry_enabled: false, retry_count: 0}
      files:
        - {id: "ccd_components_file", size: 490777362, sha256: "bb31ae5cf6c8bc669924313077cb4231ee5ffefd3a20118cd14f3ec89f8bb6a5"}
        - {id: "ccd_components_rdkit_mol_file", size: 142498117, sha256: "d1cfb71f5993a3ebea7c47877022d7f597bbfbaf86e28a4770e957da6c50cd35"}
        - {id: "pdb_cluster_file", size: 21699572, sha256: "1ab4af905e75b382eda8dec59917dc3608bee0729e36b9e71baf860bbe86850c"}
        - {id: "obsolete_release_data_csv", size: 134716, sha256: "a4f3f63ac5d7eebd78b07995cc669b9eccd6f5d8813c9492c9df02868893cf33"}
        - {id: "model", size: 1475950125, sha256: "2b7d5a8b30494514fc47fd2271a16260528cdba170ba09cc112fdecd8f85ec04"}
  pxdesign:
    resources:
      enabled: true
      download: {download_enabled: false, retry_enabled: false, retry_count: 0}
      files:
        - {id: "ccd_components_file", size: 427767290, sha256: "7240b17369ccfbbcc86e2d02dc8c9db59f46c32e0420f58889c6c121c60bfef0"}
        - {id: "ccd_components_rdkit_mol_file", size: 125903802, sha256: "2d6caced2d26c62015115a1d0a50f4106755a300e0c2a2d2b5c101c9038dcbcd"}
        - {id: "pdb_cluster_file", size: 21699572, sha256: "1ab4af905e75b382eda8dec59917dc3608bee0729e36b9e71baf860bbe86850c"}
        - {id: "model", size: 556554618, sha256: "b075867bae942dc0c6487173736922b0e2913308c1ba542d227418b6e176478d"}
        - {id: "protenix_base_default_v0.5.0.pt", size: 1474265486, sha256: "9ea20b0aba42f2256711da1d0cd081510a4b291e64375bff6b70ced70b87a5f1"}
        - {id: "protenix_mini_default_v0.5.0.pt", size: 537049294, sha256: "3803340c5d9958c038e799ddd2b53b532db21855f261592ad455a5f003791f81"}
        - {id: "protenix_mini_tmpl_v0.5.0.pt", size: 537487513, sha256: "221ca4da769e36ea0e2fa1fa82c46f6ca3a00bc7a1eff8ec0e3ddc02ed830474"}
        - {id: "af2_model_1", size: 373069562, sha256: "f95e453e6a290ddf317ba1c9698d53fa110cf007ea979b0eae43e6ad38b4e364"}
        - {id: "af2_model_1_ptm", size: 373103340, sha256: "5e564f79af5bcd54ccef6e2a6bb0ff01015d01650ebc41d4575e35f0de9ecc84"}
        - {id: "mpnn_ca", size: 6624011, sha256: "f28f40170e21858c5ff31ef50b6e63414ff76dc331b19f85aa8586a12031744a"}
        - {id: "mpnn_bb", size: 6681301, sha256: "c9cb4a671d79604111231f8dbfc7c590e06f1197453b7a6854ac6661a642f5bd"}
```

二进制路径仍用官方runtime/<app>/bin/<app>-run；本地来源根通过该runtime的conf选择，而不是改安装前缀。根配置在/protenix_service时的落位修改需要相应文件权限，本轮没有执行。支持profile还必须显式use_msa=false；不能为保留旧默认而暗改科学选项。

## Diff收口与测试

此前53/25文件中：实例19测试+2文档+25生产Go+7脚本/配置，模板6测试+1文档+18生产Go；模板/实例通用文件是同源副本，并非两套下载系统。平台flock/TTY/staging文件拆分有构建标签需要；未改协议Adapter、任务引擎/policy、依赖锁、前端视觉或build_srcs。保留现有结构，不为压文件数合并重写。

本轮仅生产修正：此前移除旧下载代码时误删了Protenix首次完整安装末尾install.ready写入，且SKIP_WEIGHTS=1仍报status=ok。已恢复原ready标记格式，环境-only只报environment_only，不生成新的完整ready标记；仅以bash尾段/假变量在项目tmp测试，没执行安装。

审查曾怀疑空CLI/禁用SQLite路径会被新root解析拒绝；实际app_path.Resolve早已保留空值，storage.Apply会清除禁用SQLite，因此没有做无必要生产修改。补实际CurrentApp非空的兼容回归；第一次测试错误假定禁用SQLite对象保留而panic，修正断言后通过，不掩盖该中间失败。

归档仍仅普通tar的指定regular成员，不是通用解包：拒绝选中的软/硬链接成员和不安全entry，不使用header路径落位；归档与成员分别校验；no-replace发布保留竞争写入者文件。补真实tar link/no-replace与取消提取后新Preparer复用已验证归档的恢复测试。现有跨OS进程单下载者、Range恢复、数据变化/校验失败等证据继续有效，不伪称做过真实GB归档下载或电源故障测试。

## 下一步必需条件，而非新增功能

1. 已有本地资源路线不必再等待“官方SHA”：明确采用本地基准片段及配置族来源；它仍只证明本地内容相等。不得偷偷写入受保护配置或默认开启信任。
2. 要使“全新机器、旧默认自动下载”恢复可用，必须确定覆盖全部所需文件的发布清单/信任基准；当前16项本地SHA256也可以被明确选择为操作者网络内容pin（不升级为官方认证），但仍需每项可用来源；尤其现实现额外要求AF2整个tar的SHA256，而本地仅有两个成员。完整tar摘要并非成员完整性校验在技术上的唯一方案，但本轮不改该已实现契约。官方元数据是CRC/MD5、Git对象及部分第三方平台LFS，而不是一套可直接采用的完整SHA256清单。本轮不新增算法/隐式TOFU来凑通过。该路线仍真实阻塞。
3. 若要保持原默认MSA等调用，需要另完成其资源/服务协调；或明确选择已有支持的no-MSA profile作为当前验收输入。不能把这一变化悄悄当等价科学结果。
4. 新assets构建上传下载、按官方根安装全新venv、root/systemd启动、真实业务产物与卸载完整闭环仍待执行；当前不发版/提交/安装/GPU，不把本地测试算整体验收。

## 最终实测与保存状态

2026-11-23 14:08 UTC+8：两仓均以现有Go1.26.10、GOTOOLCHAIN=local/GOPROXY=off/GOSUMDB=off、项目tmp下GOCACHE/TMPDIR，执行 `go test -race -count=1 ./...` 和build全部PASS；实例另执行原生CLI smoke及16项实际既有权重只读复用smoke，均PASS。最新日志模板 本地测试终端记录（14:05:38 UTC+8结束），实例 本地测试终端记录（14:05:45 UTC+8结束）。原生跨OS进程同一新资源1次GET、第三进程字节状态、无TTY/真实PTY、不同cwd、假CLI门禁未回归；16项仍ready且缺新venv时拒绝Python。没有用用户当验证者。

AF2 bucket枚举返回401、猜测的.tar.sha256侧车HEAD返回403；没有尝试鉴权/秘密，也不把这些拒绝说成文件不存在。公开命名对象的GCS元数据可读。TOS再次核查所有响应头名称，没有出现已识别SHA256元数据字段。未下载GB级权重；未运行模型、sudo/systemd、正式安装/卸载或发版。所有本轮测试/查询进程已正常结束。

精确全量未提交patch与函数/文件清单仍为各仓tmp/resource-tests/resource-preparation-final.patch、review_manifest.json，排除原README及原assets计划；tmp可重建，本文保留必要来源/信任基准和下一步边界。受保护config_local.yaml只做hash核验，源码/build_srcs/show_yeah/现有权重与venv保护边界不变。

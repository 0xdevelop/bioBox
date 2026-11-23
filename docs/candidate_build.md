# 不发布的原生构建候选与清理修复（t174）

## 非发布候选：原流程的Build路径，不是第四段发版

仍是现有 `.github/workflows/release.yml`，版本计算与 `build.sh` 调用不改，不建新release/tag、不移动latest、不删除旧资产。正式tag事件才进入release→Build→RemoveOldRelease；**workflow_dispatch或main普通push只做候选构建**。因此无需先发布才能取得用于安装验收的新包。

当前已现场发现GitHub connector仅有工作流/运行/日志只读工具，没有dispatch工具；spark也没有gh CLI。没有读凭据或调用内部API。为不把能力缺口变成停工，同一workflow增加main push的正常官方事件入口：本轮正常commit+push就会自动运行候选；人工也可在GitHub Actions选择Release_Build→Run workflow，不传新参数。后续源码普通push同样持续验证，不需要再次请用户批准同一事项。

- `release`与`RemoveOldRelease`、公开Release上传步骤都严格限定push版本tag；候选不经过它们。Build显式按native成功继续（always+!cancelled+needs结果），避免GitHub把已跳过release祖先错误传递成Build也跳过。
- 每个native job对真实整合zip额外做rootless资源门禁smoke：只解压tmp副本、改副本配置根value，无TTY检查missing→download_disabled、prepare拒绝、缺venv时verify在上游Python前拒绝；不安装环境/不下载权重/不碰systemd或GPU。不能把该smoke当推理通过。
- 继续两原生runner：ubuntu-24.04 amd64、ubuntu-24.04-arm arm64；各自内部构建/检查三包，独立包能力没有被裁掉。
- 候选跨job只传各架构整合zip（内部transport artifacts `release-packages-linux-*`），不再传独立包/多份manifest/checksum；Build用原`gen_release_assets.go verify`校验服务ELF/CGo、commit及两套内嵌runtime材料。
- 最终可下载artifact名 `candidate-<完整源SHA>`，**恰好**两整合zip + SHA256SUMS（两行）。原生中间transport各含一个zip，仅为job间搬运，不是额外交付附件。
- `scripts/stage_candidate_assets.sh`要求输入恰好两个预期整合包、新建空输出目录，生成校验后复用原verify。不能混入stale/独立包。
- 候选沿用源码配置版本；当前仍v0.0.2，zip名含v0.0.2不等于替换旧公开Release，必须按candidate源SHA识别。正式版仍只能由原无参git_tag.sh自动决定版本，本轮不运行它。

本地已有合成ZIP/ELF fixture对stage正例、非空输出、额外输入做回归；这不是双原生CI成功或业务推理证据。运行链接/结果以本轮实际Actions为准，不能把创建/排队或本地fixture当完成。

## 清理职责与错误边界

先在project_template_go实现共享`scripts/release_cleanup.sh`及`test_release_cleanup.py`，实例同源适配。无参git_tag.sh仅把最后的local旧tag删除换为经精确查询的幂等函数；Actions第三段仅替换实际清理语句，未重构三段。

- local：`show-ref`退出1才是不存在；其他错误失败；存在则正常删除且必须成功。
- release：成功分页列出repo Releases后精确匹配目标tag，不存在才noop。查询遇到权限/网络/rate-limit/服务端错误直接失败；存在后的delete失败也失败。不用笼统404或`|| true`吞错。
- remote tag：`ls-remote --exit-code --refs --tags`退出2才是不存在，其他失败保留；delete错误失败。
- 实例的local和workflow两处在调用通用清理前均显式跳过v0.0.2。latest仍是允许前进的浮动引用。本轮没有执行真实清理、移动任何tag或修改旧公开资产。

## 公开下载配置推进

成品config_example.yaml保留16个已授权本地内容pin；现在明确列出14个公开HTTPS来源（Protenix5、PXDesign9）。地址只读提取自已固定上游URL字典；MPNN使用官方dauparas/ProteinMPNN固定commit8907e6671bfbfc92303b5f79c4b5e6ce47cdef57的raw路径，不偷换镜像。不称本地SHA256是官方签发。

2026-11-23 18:43 UTC+8，14个URL HEAD全部200、Content-Length与所选基准size逐项一致，body读取0；详情项目tmp/release-t174/source-head-results.json。未下载真实权重。TOS仍只给CRC64/ETag，AF2 GCS元数据仍未补出可采用整tar SHA256，不降低其校验契约。

实际选择方法：保持默认download_enabled=false即可复用已有文件；若要按所列来源下载，操作者需明确选择本地内容pin与URL并设置所用集合download_enabled=true，然后先resource status取得确认令牌，再交互确认或原API确认后prepare；无TTY/systemd不隐式确认。Protenix v1全部5项可以走这条代码路径；PXDesign先保证两个既有AF2成员已匹配，其余9项可同样下载。完整无AF2的新机器路线仍有真实缺口，不做不存在的全联网承诺。使用模式仍显式v1/no-MSA或preview/no-MSA，不扩展MSA/v2。下载/校验期间仍由Go持锁，不能启动上游自动下载CLI。

## 不可替代的root动作（本轮未执行）

普通权限可继续候选校验、配置/资源复用和源码测试；不能写正式根/系统单元或绕过NoNewPrivs。真正安装时最少需要用户在正常有权限终端以root执行官方install_protenix_service.sh（指定其可执行uv路径，先按交付说明修改资源根value），使其写/protenix_service及/etc/systemd/system/protenix_service.service并完成环境构建与既定启停。不是另换开发prefix就算完成，也不是用户只systemctl start即可替代初装。具体root窗口由主端协调，不让用户代做普通验证。

此后仍需真实v1/no-MSA及PXDesign preview/no-MSA任务、产物下载哈希和官方卸载证据；现有开发venv/合成CLI不能替代新环境验收。本轮继续完成所有不用root的步骤，未发正式版。

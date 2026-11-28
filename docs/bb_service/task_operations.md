# 任务查询、业务步进与排障

本说明适用于 `bioBox` 的任务使用者。安装与升级沿用[根目录安装说明](../../README.md)，不需要为步进另装服务、数据库或运行时。任务记录使用 SQLite 主库；一个队列库由一个服务进程负责，不是多实例 HA 队列。

## 1. 先确认入口与运行模式

- 所有协议通过固定业务方法名调用：`protenix`、`pxdesign`、`task.get`、`task.list`、`task.cancel`、`task.artifacts`、`task.artifact`。JSON-RPC 使用 HTTP `POST /`，请求为 `tools/call → name → arguments`，不要拼出 `/v1/protenix`、`/task/get` 等路径。
- 版本、模型、preset 只放在 arguments。Protenix 未传版本时为 `v1`；未配置 v2 runtime 时拒绝，不回退为 v1，也不能把配置路径当成已有授权权重。
- 受保护方法使用任务属主的凭证；API key 放在 `Authorization: Bearer <api_key>`，一次调用只带一种凭证，不再在 arguments 放另一份。不要把密钥写入输入文件、日志、截图或共享命令记录。
- `test_ui_cfg.enabled` 缺席时，非 release 缺省开、release 缺省关；显式 `false` 不调用 `LoadTestWeb`。显式 `true` 覆盖该缺省。正式服务可以直接调用任务 API，不依赖调试页。

| 二进制运行模式 | 配置文件 | test_ui 条件 |
|---|---|---|
| debug | `example_files/config_local.yaml` | `enabled` 缺席则开；显式 false 不调用 `LoadTestWeb` |
| test | 可执行文件目录下的 `conf/config.yaml` | 同上 |
| release | 可执行文件目录下的 `conf/config.yaml` | `enabled` 缺席则关；显式 true 才监听 |

模式在构建时确定，不是 URL 参数或配置开关。`./build.sh test` 生成本机 test 模式主程序；`./build.sh pack test linux_<arch>` 才是 Linux 打包且运行模式为 test。构建前置条件见[服务说明](../../README.md#本地构建与开发调试)。不要用 release 二进制尝试“打开调试模式”。debug 才会读取 CWD 相对的 `./example_files/config_local.yaml`；test 与 release 读取可执行文件目录下的 `conf/config.yaml`（由启动时的 ExecutableDir 解析，不再用 CWD 相对的 `./conf/config.yaml` 覆盖）。

### 隔离运行调试页

已有 test 模式二进制时，可把临时副本与独立配置放到本仓根目录 `tmp/`，不用正式安装，也不要覆盖已有本机配置：

```bash
# 在仓库根目录执行；源路径替换为已有的 test 模式二进制
mkdir -p tmp/task-debug/conf
cp /path/to/test-mode/bioBox tmp/task-debug/bioBox
cp example_files/config_example.yaml tmp/task-debug/conf/config.yaml
# 先按下文配置，再启动；不要原样启动示例配置
(cd tmp/task-debug && ./bioBox)
```

启动前在这个**独立副本**中确认：

1. `test_ui_cfg.enabled: true`，`bind_address: "127.0.0.1"`，port 是未占用的有效端口（示例 17170）。所有启用的 API 监听也设为 `127.0.0.1` 并选不冲突的端口；frontend 不需要时显式 `enabled: false`。不要为调试把监听扩大到公网。
2. `storage_cfg.primary: sqlite`，SQLite 启用且路径指向临时独立库；其他存储显式 `enabled: false`。**不要让临时进程与正式进程共用队列库**。身份与 API key 属于各自服务的库，不能把正式服务的 key 当成临时库里的身份。
3. `cli_cfg.work_root` 指向本仓 `tmp/task-debug/` 下的独立作业目录。需要执行真实业务才配置已安装的 runtime 路径；binary 为空时业务拒绝启动，不会自动安装或下载。未打算推理时，不要提交任务；页面探测只检查调用链，不运行推理。
4. 保留鉴权；按实际服务的认证配置提供有效属主凭证，不要用 `auth_cfg.enabled: false` 代替身份。运行模式与 test_ui 开关不改变业务鉴权。

打开运行该进程的机器上的 `http://127.0.0.1:<test_ui-port>/flow_debug.html`；17170 只是示例，不保证当前机器已启动。跨机器访问只使用已有授权安全连接；浏览器自身的 localhost 不是远端机器。

如果只想通过临时调试页查看已有正式任务，在页面填写**正式服务已经启用的回环协议地址**，使用正式服务的属主 key；test_ui 代理只允许回环目标。不要为此把临时进程接到正式队列库。结束后停止临时进程，再清理自己的 `tmp/task-debug/`，不要删正式库或正式作业目录。

## 2. 在页面查看与回看

1. 填写目标服务 URL 和 API key，选择服务已启用的 JSON-RPC / MCP / WebSocket / gRPC。页面同源 test_ui 提供代理；gRPC 需经该代理，不能把 HTTP 请求直接发到 gRPC 端口。
2. 查看已有任务：填写已保存的 `task_id` 并跟踪，或刷新历史后点击一条记录。`task.list` 只列该属主最近 50 条摘要；步骤详情来自 `task.get`，不能把列表缺少步骤解释成没有历史。
3. 需要新任务时才填写服务端可读的输入路径、选择业务与参数并提交。输入路径不是浏览器本地路径；提交会真正执行配置的 CLI。受理后保存返回的 task_id，查询它，不要因断线再次提交。
4. “业务步进”显示服务返回的业务阶段、状态字符串枚举数组和时间。当前/最后业务步骤由 `current_business_step` 指明；点击任意历史业务行或用键盘 Enter/空格选择，只读查看原始记录；“回到本轮当前业务步骤”恢复查看服务指明的步骤，不执行回滚、重跑或修改。
5. 展开“引擎关联诊断”看通用调度阶段与任务错误。刷新页面、中途跟踪同一 task_id 后，已持久化步骤仍由服务返回，不靠这个浏览器之前的轮询记录拼接。
6. 终态后可列出产物并读取日志或结构；路径只用 `task.artifacts` 返回的相对路径。结构展示、坐标与科学边界见[三维结构说明](structure_visualization.md)。读取失败不能当成任务需要重提。

字段未返回显示“—”；旧库没有步进记录时为 `steps=[]`、`current_step=null`，不会补造历史。原生推理未接逐扩散内部进度，进度条可能从 0 跳到 100；装饰光点、样本切换与业务阶段都不是 GPU 百分比或物理折叠过程。

## 3. 状态分层与关联定位

**程序级归 Go 引擎，业务级归具体业务。** 两者通过同一任务关联，但不混成一套业务步骤：

- 引擎记录 `step_kind=engine`：`accepted`（受理排队）、`claimed`（认领）、`requeued`（孤儿恢复重排）、`finished`（终态）。用于调度和诊断，不表示领域算法阶段。
- 业务记录 `step_kind=business`：Protenix/PXDesign 实际上报 `input_preparation`、`cli_execution`、`artifact_collection`。状态数组分别可能出现 `input_staged`、`process_exited` / `process_failed` / `process_cancelled`、`cif_files_found` / `no_cif_files` 等已注册字符串枚举；完整值域见[Protenix 业务说明](ability_protenix.md)和[PXDesign 业务说明](ability_pxdesign.md)。输入准备失败不会伪造后续未执行阶段。
- 每步的 `run_status` 是**该阶段开启时**引擎写入的程序态字符串枚举 `queued` / `running` / `done`，不是该步骤的业务成功标志；历史业务行即使已关闭仍可能是 `running`。看 `completed_at` 是否为空判断阶段是否关闭，关闭本身不等于成功。
- `business_statuses` 是业务实际发生并上报的字符串枚举数组，不是全部允许值，也不是单个字符串。`[]` 只表示尚无已上报业务状态；不能当作成功、失败或关闭开关。引擎阶段该数组恒为空。
- 任务最终结果看任务级 `run_status` 与 `process_status`：程序态 `done` 是终态；处理结局为 `success` / `failed` / `cancelled` / `system_error`，空串表示尚无处理结局。失败细节看 `error_code`、`error_message` 和真实日志。CIF 文件存在不等于结构质量合格。

定位时先记 **task_id → execution_count → steps[].attempt → step_seq**：execution_count 是已认领次数（首次认领后为 1），attempt 是该记录所属轮次，step_seq 是任务内跨轮次持续递增的顺序号。按同一 attempt 对照业务步骤、引擎记录、消息和时间，避免把上次执行的状态当成本轮状态。API 时间为 UTC；页面时间显示 Asia/Shanghai（UTC+8）。

`current_step` 是服务最后一条记录（可为引擎终态）；`current_business_step` 是当前轮次最后一条业务记录，可能已经关闭。新轮次还没上报业务步骤时为 null，不会把旧轮次步骤冒充当前。完整字段语义见[步进契约](ability_task.md#41-引擎记录与业务步进只读回看)。

## 4. 不用页面也能查询

向目标服务已配置的 JSON-RPC 根地址发送以下 JSON 请求，HTTP header 使用同一个属主的 Bearer 凭证。示例 UUID 仅为格式示例，须替换为受理返回的 task_id。其他协议只更换通信外壳，业务方法名与 arguments 相同；[方法参数清单](../../bb_docs/api_methods.md)是完整参数入口。

列出已有任务：

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"task.list","arguments":{}}}
```

读取任务与全部步骤（执行中也可查询）：

```json
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"task.get","arguments":{"task_id":"b3f11655-08df-458f-aa8d-aaa02a20bd51"}}}
```

返回使用 MCP `CallToolResult`：先检查 `result.isError`，成功时解析 `result.content` 中文本 JSON 的 `task`，再读取 `task.steps` 等字段。不要把 HTTP 200 当作业务成功，也不要只凭任务当前 `progress` 推断阶段。查询只允许该属主；跨用户、无属主任务或无效 task_id 不能靠关闭鉴权绕过。

任务终态后列出文件：

```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"task.artifacts","arguments":{"task_id":"b3f11655-08df-458f-aa8d-aaa02a20bd51"}}}
```

将下面的 `path` 替换为上一步实际返回的相对路径，不拼接服务本机绝对路径或猜下载 URL：

```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"task.artifact","arguments":{"task_id":"b3f11655-08df-458f-aa8d-aaa02a20bd51","path":"run.log"}}}
```

成功取件返回 base64 内容、size 与 sha256；可解码后复算核对，操作细节见[产物方法说明](ability_task.md#34-taskartifacts)。

## 5. 常见排障顺序

| 看到的情况 | 先核对 | 不应据此做什么 |
|---|---|---|
| 页面打不开 | 二进制是不是 release；非 release 是否启用 test_ui、绑定正确、端口未占用；浏览器与服务是否同机 | 不把 release 配置 enabled=true 当成一定有调试页，不盲目放开公网监听 |
| 查不到任务或权限错误 | 目标服务地址、该库的属主凭证、task_id；list 仅最近 50 条 | 不复制其他用户凭证或关闭鉴权来查；不重提任务 |
| 排队且只有 accepted/requeued | 看 current_step、该轮次、在途任务、Policy 与服务日志，确认调度进程仍正常 | 不用修改 SQLite 状态强制认领；不同时启动第二个队列 daemon |
| 卡在某个业务阶段 | 对照同 attempt 的消息、updated_at、任务错误；CLI 运行阶段不保证持续上报 | 不把没有新百分比等同于失败；不伪造内部进度 |
| done 但失败/取消 | 看 process_status/error_*、CLI 结果摘要，再用产物接口取实际 run.log（若存在） | 不把已关闭阶段或已有 CIF 当成成功 |
| 连接中断或页面刷新 | 重连后 task.get 查询同一个 task_id，历史由服务持久返回 | 不把断线当任务失败，不重复提交 |

## 6. 取消、恢复与介入边界

取消是有副作用的操作，只有确实决定取消时才调用，不是查询步骤的方式：

```json
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"task.cancel","arguments":{"task_id":"b3f11655-08df-458f-aa8d-aaa02a20bd51"}}}
```

- 本实例 queued 任务直接返回 `cancelled:true` 并落终态；running 返回 `cancelling:true`，须继续 task.get，直到 Execute 和进程组退出后才落 `done/cancelled`。终态不能再次取消。**通用模板只支持 queued 取消**，不要把实例扩展反推为模板能力。
- 已受理任务独立于浏览器连接。服务按既有孤儿恢复机制识别不在本进程执行集合的 running 任务；重排会关闭旧阶段、保留旧轮次历史，下一次认领增加 execution_count。业务必须幂等或能自查续跑；这不是 GPU 中断无损续算承诺，也不支持多实例共享认领。
- 已请求取消的孤儿按取消收尾；Execute 结果写回不确定时落 system_error，不能当普通孤儿盲目重跑。没有对外修改步骤、删除历史、手动恢复/重跑的接口。不要为了触发恢复而杀进程或手改数据库。
- 没有新增步骤级超时。关闭整体超时、Execute 超时或次数限制分别用 `policy_cfg.task_lifecycle_timeout_enabled: false`、`cli_cfg.execute_timeout_enabled: false`、`policy_cfg.task_execute_retry_limit_enabled: false`，不是用数值 0 隐含关闭。启用超时时 seconds 为正整数；启用重试限制时 count 是首次之外允许的重试次数，0 就是不重试。详细策略见[任务与恢复说明](ability_task.md#4-数据与并发纪律)。

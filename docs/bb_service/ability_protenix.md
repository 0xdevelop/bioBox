# protenix 功能入口

固定方法名：`protenix`（MCP `tools/call` 的 `name`，不是 URL 路径）。

对应命令行：`protenix-run pred`。arguments 与 CLI 长选项对齐：`input`、`out_dir`、`model_name`、`seeds`、`cycle`、`step`、`sample`、`dtype`、`use_msa`、`use_default_params`。产品版本 `version`（`v1`/`v2`）也放在 arguments 中，不作为 CLI 开关、不进路径；缺省 `v1`。`version` 只用于选择 `cli_cfg.protenix.version_runtimes` 中的独立 runtime：`v1` 未单独配置时回退到 `cli_cfg.protenix.binary`，`v2` 必须配置 `cli_cfg.protenix.version_runtimes.v2.binary`，不会使用 v1 的 binary。对应版本未配置时 Execute 拒绝启动，不猜路径。跑通 v2 还需要授权权重，配置了 v2 runtime 也不等于 v2 已验收。

注册为 `Async: true`。受理走 `AcceptAsyncTask` → `storage.GormDB` 主库（本实例 SQLite）→ Policy → 同一 Execute，不安装 MySQL/Redis。

`Execute` 会按所选版本的 runtime 组装命令：把输入拷贝到 `work_root/<task_id>/input/`，输出写到同任务 `output/`，在新进程组中启动该版本的官方包装命令。`v1` 未配置时返回 `protenix CLI binary is not configured`，`v2` 未配置时返回 `protenix v2 CLI binary is not configured`，都不会猜路径或启动进程。结果摘要包含退出码、是否取消、CIF 数量和是否存在 ERR 目录；原生预测是否成功不能只看退出码。

取消：`task.cancel` 支持属主取消 queued/running。running 先持久化 `cancel_requested`，传递 Execute context 到现有 `cli_proc.Run`；SIGTERM/SIGKILL 作用于任务整组，清理后才落 done/cancelled。短脚本 HTTP 测试覆盖取消其一而另一任务完成，不等于全部原生 GPU 工作流取消已验收。

在 `ability/ability.go` 的 `LoadAbilityAPIMethods` 中显式加载，禁止 init/反射自动注册。


## 业务步进（Go 任务层持久化，非推理内部进度）

本业务在自己的 `LoadAPIMethods` 中注册以下阶段及业务状态字符串枚举，通用 task 引擎不定义这些领域含义：

| 业务阶段 `step` | 允许的 `business_statuses` 值 | 实际写入时机 |
|---|---|---|
| `input_preparation` | `arguments_validated`, `input_staged`, `preparation_failed` | 真正进入参数校验、runtime 选择与输入暂存时开启；相应动作已完成/失败才上报状态 |
| `cli_execution` | `process_exited`, `process_failed`, `process_cancelled`, `launch_failed` | 调用 CLI 包装前开启；Run 返回后按实际 PID、取消标记与错误上报；不伪造内部扩散步数、GPU 百分比或模型阶段 |
| `artifact_collection` | `inspected`, `cif_files_found`, `no_cif_files`, `err_artifacts_found`, `inspection_failed` | Run 返回后真正检查产物目录；CIF 状态仅指文件存在性，不是格式/结构质量/折叠有效性验证 |

每步含 Go 引擎写入的程序态 `run_status=running`，业务自己追加的字符串枚举数组、服务端 UTC 时间及消息。业务状态按首次发生顺序去重，`[]` 只表示尚未上报。失败不会补写不存在的后续业务步骤。CLI 失败后若产物检查实际执行，仍如实记录检查步骤；成败以任务 `process_status/error_*` 及当轮业务状态联合定位，不能把“阶段关闭”当成功。

只读查询与回看走 `task.get.steps/current_business_step`，关联 `task_id + attempt + step_seq`；引擎受理、认领、恢复和终态时间线在页面独立诊断区，不冒充业务阶段。新轮次保留旧历史，不携带旧轮次状态冒充当前。客户端断线只查询同一 task_id。直接调用纯业务 Execute 无持久任务 context 时不伪造任务/步进；正式 Async Worker 路径必须持久化，写入失败不静默吞掉。取消后用独立于已取消 context 的短写库调用记录实际 CLI 返回与产物检查，不新增任何步骤超时。

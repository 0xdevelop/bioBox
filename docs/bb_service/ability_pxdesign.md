# pxdesign 功能入口

固定方法名：`pxdesign`（MCP `tools/call` 的 `name`，不是 URL 路径）。

对应命令行：`pxdesign-run pipeline`。arguments 与 CLI 长选项对齐：`input`、`dump_dir`（CLI `-o/--dump_dir`）、`preset`（`preview`/`extended`/`custom`）、`N_sample`、`N_step`、`N_max_runs`、`dtype`、`eta_max`、`eta_min`、`eta_type`。筛选器/preset 放在 arguments，不进入路径。

注册为 `Async: true`。受理走 `AcceptAsyncTask` → `storage.GormDB` 主库（本实例 SQLite）→ Policy → 同一 Execute，不安装 MySQL/Redis。

`Execute` 会按 `cli_cfg.pxdesign.binary` 组装命令：把输入拷贝到 `work_root/<task_id>/input/`；若 YAML/JSON 中有相对路径（例如官方示例的 `./examples/5o45.cif` 与 MSA 目录），会把这些侧车拷到作业目录并保持原文相对路径，不改原文件。输出写到同任务 `output/`，在新进程组中启动官方包装命令。`task.artifacts` 只列出 `output/` 与顶层 `run.log`，不会把暂存的 `examples/` 或 `input/` 当作产物。`binary` 为空时返回 `pxdesign CLI binary is not configured`，不会猜路径，也不会启动进程。结果摘要包含退出码、是否取消、CIF 数量和是否存在 ERR 目录。

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

# pxdesign 功能入口

固定方法名：`pxdesign`（MCP `tools/call` 的 `name`，不是 URL 路径）。

对应命令行：`pxdesign-run pipeline`。arguments 与 CLI 长选项对齐：`input`、`dump_dir`（CLI `-o/--dump_dir`）、`preset`（`preview`/`extended`/`custom`）、`N_sample`、`N_step`、`N_max_runs`、`dtype`、`eta_max`、`eta_min`、`eta_type`。筛选器/preset 放在 arguments，不进入路径。

注册为 `Async: true`。受理走 `AcceptAsyncTask` → `storage.GormDB` 主库（本实例 SQLite）→ Policy → 同一 Execute，不安装 MySQL/Redis。

`Execute` 会按 `cli_cfg.pxdesign.binary` 组装命令：把输入拷贝到 `work_root/<task_id>/input/`；若 YAML/JSON 中有相对路径（例如官方示例的 `./examples/5o45.cif` 与 MSA 目录），会把这些侧车拷到作业目录并保持原文相对路径，不改原文件。输出写到同任务 `output/`，在新进程组中启动官方包装命令。`task.artifacts` 只列出 `output/` 与顶层 `run.log`，不会把暂存的 `examples/` 或 `input/` 当作产物。`binary` 为空时返回 `pxdesign CLI binary is not configured`，不会猜路径，也不会启动进程。结果摘要包含退出码、是否取消、CIF 数量和是否存在 ERR 目录。

取消：`task.cancel` 支持属主取消 queued/running。running 先持久化 `cancel_requested`，传递 Execute context 到现有 `cli_proc.Run`；SIGTERM/SIGKILL 作用于任务整组，清理后才落 done/cancelled。短脚本 HTTP 测试覆盖取消其一而另一任务完成，不等于全部原生 GPU 工作流取消已验收。

在 `ability/ability.go` 的 `LoadAbilityAPIMethods` 中显式加载，禁止 init/反射自动注册。

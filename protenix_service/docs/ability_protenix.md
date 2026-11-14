# protenix 功能入口

固定方法名：`protenix`（MCP `tools/call` 的 `name`，不是 URL 路径）。

对应命令行：`protenix-run pred`。arguments 与 CLI 长选项对齐：`input`、`out_dir`、`model_name`、`seeds`、`cycle`、`step`、`sample`、`dtype`、`use_msa`、`use_default_params`。产品版本 `version`（`v1`/`v2`）也放在 arguments 中，不作为 CLI 开关、不进路径；v2 需要授权权重。

注册为 `Async: true`。受理走 `AcceptAsyncTask` → `storage.GormDB` 主库（本实例 SQLite）→ Policy → 同一 Execute，不安装 MySQL/Redis。

`Execute` 会按 `cli_cfg.protenix.binary` 组装命令：把输入拷贝到 `work_root/<task_id>/input/`，输出写到同任务 `output/`，在新进程组中启动官方包装命令。`binary` 为空时返回 `protenix CLI binary is not configured`，不会猜路径，也不会启动进程。结果摘要包含退出码、是否取消、CIF 数量和是否存在 ERR 目录；原生预测是否成功不能只看退出码。

取消：`task.cancel` 支持属主取消 queued/running。running 先持久化 `cancel_requested`，传递 Execute context 到现有 `cli_proc.Run`；SIGTERM/SIGKILL 作用于任务整组，清理后才落 done/cancelled。短脚本 HTTP 测试覆盖取消其一而另一任务完成，不等于全部原生 GPU 工作流取消已验收。

在 `ability/ability.go` 的 `LoadAbilityAPIMethods` 中显式加载，禁止 init/反射自动注册。

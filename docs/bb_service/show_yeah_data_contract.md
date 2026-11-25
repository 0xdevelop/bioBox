# show_yeah 真实数据对接契约（2026-11-14 UTC+8）

本服务线程不修改 show_yeah。它负责后端和可复核真实结果；前端线程可沿此契约打样，不要自造第二套任务调度。

## 固定功能调用

所有协议复用 `tools/call`，功能由 `params.name` 选择，版本只在 arguments，不放 URL/path/name。当前实测 JSON-RPC HTTP `POST /`，`Authorization: Bearer <ptxk_api_key>`。端口/监听地址来自服务器配置，不能把开发端口写成协议定义。

```json
{"jsonrpc":"2.0","id":"submit-1","method":"tools/call","params":{"name":"protenix","arguments":{"version":"v1","input":"<服务器上输入文件路径>","out_dir":"out","model_name":"protenix_base_default_v1.0.0","use_msa":false,"seeds":"101","cycle":1,"step":2,"sample":1,"use_default_params":false}}}
```

服务返回 MCP CallToolResult：检查 `isError` 后 JSON.parse `content[0].text`，得到 `{task_id,run_status:"queued"}`。这是已落 SQLite 的受理结果，不是结构完成。

用同一入口调用 `task.get`，arguments 为 `{task_id}`，内容为 `{task:{task_id,method,run_status,process_status,progress,progress_message,cancel_requested,created_at,started_at?,completed_at?,result?}}`；`task.list` 只列当前属主最多 50 项。建议 UI 每 1–2 秒轮询，完成后停止。状态值：

- run_status：queued/running/done，done 为唯一终态。
- process_status：空/success/failed/cancelled/system_error。
- 当前原生推理没有逐扩散步进度接线，progress 主要为 0 → 100；不能用动画伪造后端真实百分比。
- 成功 result：`job_dir,out_dir,pid,pgid,exit_code,cancelled,log_path,cif_count,has_err`。

取消调用 `task.cancel`，queued 返回 `cancelled:true,cancelling:false`；running 返回 `cancelled:false,cancelling:true`，继续轮询直至 done/cancelled，不要提前播“已完成取消”。

## 实测数据与当前边界

已真实跑通 HTTP Bearer → SQLite → Policy → Execute → 本机 Protenix v1 → 1 个 CIF，耗时 39.051 秒。输入为合成 20 残基，cycle=1/step=2/sample=1，仅用于链路/3D 打样，不代表科学预测质量。没有伪造 CIF。

本机证据目录（不作为产品硬编码路径）：

`/home/certa/workspace/protenix_build/tmp/handoff-followup/phase3-service/sqlite-http-official-20261114/`

其中 `result-manifest.json` 枚举真实 CIF 与置信度 JSON 的绝对路径，不含凭证。前端可导入该 CIF 做优先打样，并明确标注“真实引擎烟测结果”。临时证据可删除，不是生产数据存储契约。

已完成任务（含失败、取消、系统错误）可用同一入口调用 `task.artifacts`（arguments `{task_id}`）拿到相对路径清单，再用 `task.artifact`（`{task_id,path}`）取 base64 内容。只含作业目录内允许的输出/日志，不含 `input/`，也不返回服务器绝对路径。`task.get` 在 `done` 且有结果摘要时附 `result`；其中 `job_dir` / `out_dir` / `log_path` 仍是本机路径，不能直接 fetch，也不要当公共下载链接。

自包含调试页已放到 `bb_test_ui/flow_debug.html`，经 test_ui 的 `/flow_debug.html` 打开，只填 URL 与 API key，并按上述方法加载真实产物。提交、跟踪、取件和取消使用页面选择的 JSON-RPC / MCP / WebSocket / gRPC，不是只走 JSON-RPC。页面按 mmCIF 列解析官方 Protenix CIF（即使数据行以 `ATOM` 开头），经典 PDB 仍可用；舞台只绘制这些真实坐标，入场排布不是计算进度，悬停显示产物中的残基与坐标。提交或跟踪另一任务时会先清空舞台与产物表，避免上一任务的原子被当成当前任务结果。非 success 终态也会列出真实产物（例如 `run.log`），不会把失败结果自动画成成功结构。流程轨道借鉴 show_yeah 的光纤、光点与枢纽准星，但只点亮探测、受理、`task.get` 状态、取件和结构上舞台这些真实事件，不把光点当成推理百分比。本页不是 Linux 真浏览器对此次视觉改动的再点击验收，也不能用随机进度或伪造 CIF 冒充计算。show_yeah 仍只作视觉效果参考，不在本仓改造。

**版本边界**：已可用 `cli_cfg.protenix.version_runtimes` 按 `version` 选择独立 runtime；`v2` 不会回退到 v1 的 binary。官方 HTTP 烟测仍只验证 v1/base_default_v1.0.0。v2 仍缺授权权重，不可用参数标签或仅配置路径冒充 v2 成功。PXDesign 已有短脚本 HTTP 真任务证据，但不是官方 GPU 设计。


业务步进的事实源是 Go task 层持久化的 `task.get.steps`（`step_kind=business`）与 `current_business_step`，不是浏览器轮询拼接的阶段表。页面可点击历史阶段只读回看，服务未返回的字段显示“—”；引擎通用调度记录单独位于关联诊断区。包装层只记录实际的输入准备、CLI 执行与产物检查，不代表推理内部进度。详细字段、轮次恢复与失败收尾见 [Task 契约](ability_task.md#41-引擎记录与业务步进只读回看)。

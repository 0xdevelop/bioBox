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

**尚无浏览器取件 API**：result 的路径是服务器本地文件路径，不是 URL，不能直接 fetch。现在不得把这些路径当公共下载链接，也不要暴露任意路径读文件。下一片需基于 task 属主与输出清单提供受控产物读取，再让远端 HTML 自动加载。独立 HTML 仅填 URL/key 的最终验收尚未完成。

**版本边界**：本轮只验证 v1/base_default_v1.0.0。现有 version 字段并未完成 v1/v2 独立 runtime 选择，v2 缺授权权重，不可用参数标签冒充 v2 成功。PXDesign 真任务没有在本轮走 HTTP 重新验证。

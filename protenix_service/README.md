# protenix_service

Protenix / PXDesign 常驻服务骨架。module：`github.com/0xdevelop/protenix_service`。API key 前缀：`ptxk_`。嵌套在 `protenix_build` 仓库中，沿用父仓许可证，不另放 LICENSE。

固定功能入口的 Go 服务，异步任务使用 SQLite 主库，不需要 MySQL/Redis；验收事实见父仓 `docs/acceptance_service.md`。

## 构建

在本目录执行（需 Go 1.26.10 工具链、`protoc`、Bun；`GOTOOLCHAIN=auto`）：

```bash
./gen_proto.sh
./gen_frontend.sh
./gen_test_ui.sh
go build -o protenix_service .
```

`build.sh` 会在编译前调用前端生成脚本；不要跳过 `gen_frontend.sh` / `gen_test_ui.sh`。

## 配置

- 示例（可提交、无真实密钥）：`example_files/config_example.yaml`
- 本机运行（不要提交）：把示例复制为 `example_files/config_local.yaml` 后按需修改
- Debug 启动读取 `./example_files/config_local.yaml`；工作目录必须是本目录
- 存储默认只开 SQLite（`storage_cfg.primary: sqlite`，mysql/postgres/redis 为 `enabled: false`）。Auth/User 走主库 SQLite。
- Async/Task 使用 `storage.GormDB` 主库，SQLite 原子认领与持久状态。仅单 daemon 负责一个队列库；本机孤儿恢复不是 HA 租约。
- 原生 CLI 路径写在 `cli_cfg`（见示例配置），不要在代码或 URL 里写死。`cli_cfg.protenix.binary` / `cli_cfg.pxdesign.binary` 为空时，`Execute` 拒绝启动进程。作业输入会拷贝到 `cli_cfg.work_root/<task_id>/input/`，输出写到同任务的 `output/`，不修改原输入文件。`version` 仍只作方法参数，不作为 CLI 开关。
- `example_files/config_local.yaml` 与 `data/` 已加入忽略规则，不要提交密钥或 SQLite 数据文件。

默认端口（示例配置）：JSON-RPC `13001`、MCP `13002`、test_ui `13003`、WebSocket `13004`、gRPC `13005`、frontend `13006`。test_ui 与 frontend 默认只绑定 `127.0.0.1`。本机验收如需避开占用，可在 `config_local.yaml` 改为 `18001`–`18006`。

## 启动

```bash
./protenix_service
```

或使用用户级 systemd 单元（不要用系统级 root 单元）：

```bash
# 单元文件：example_files/protenix_service.service
# Type=simple，ExecStart / WorkingDirectory 指向本目录的二进制
systemd-analyze --user verify example_files/protenix_service.service
systemctl --user start protenix_service.service
# 停止：
systemctl --user stop protenix_service.service
```

不要留下 enabled 且失败的常驻服务。

## 功能入口

所有协议共用一个业务调用入口 `tools/call`，用 `params.name` 选择方法，不要做成 `/v1/protenix` 这类路径。

| name | 说明 | arguments（与原生 CLI 长选项对齐） |
| --- | --- | --- |
| `test` | Public，检查调用链 | 无业务参数 |
| `protenix` | 对应 `protenix-run pred`，Async | 必填 `input`、`out_dir`；可选 `version`（`v1`/`v2`）、`model_name`、`seeds`、`cycle`、`step`、`sample`、`dtype`、`use_msa`、`use_default_params` |
| `pxdesign` | 对应 `pxdesign-run pipeline`，Async | 必填 `input`、`dump_dir`；可选 `preset`（`preview`/`extended`/`custom`）、`N_sample`、`N_step`、`N_max_runs`、`dtype`、`eta_max`、`eta_min`、`eta_type` |

JSON-RPC 示例（Public `test`）：

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "test",
    "arguments": {}
  }
}
```

向 `http://127.0.0.1:<jsonrpc-port>/` 发 POST。方法清单见 `docs/api_methods.md`（由 `./gen_api_docs.sh` 生成，不要手改）。

## 真实任务流调试页

启用 `test_ui_cfg` 后，用浏览器打开：

```text
http://127.0.0.1:<test_ui-port>/flow_debug.html
```

填写服务 URL 和 `ptxk_` API key，按真实 `protenix` → `task.get` → `task.artifacts` → `task.artifact` 流程调试。提交、跟踪、取件和取消走页面上选择的协议（JSON-RPC / MCP / WebSocket / gRPC），功能名固定，版本和模型仍作为参数。探测在已填 key 时会对四种协议各调一次 `task.list`，不提交新任务。历史任务列表只展示所选协议 `task.list` 返回的属主记录，点击一行可跟踪该任务并取回真实产物；没有任务就空着，不编造历史。提交或跟踪另一任务时，舞台、产物表和任务流程枢纽会先撤下上一任务的结果，只显示当前 `task_id` 的状态与产物。进度只显示后端上报值，结构只从取回的产物解析（官方 Protenix CIF 按 mmCIF 列读取 `ATOM` 行）。舞台可拖拽旋转、滚轮缩放，悬停查看残基与原子坐标；入场只是呈现已解析坐标，不是计算进度。右侧流程轨道借鉴 show_yeah 的光纤与枢纽样式，但只点亮探测、受理、`task.get` 状态、取件和结构上舞台这些真实事件，光点不是推理计算进度。页面若在 test_ui 同源，可通过已有 `/api/proxy` 调用上述协议；gRPC 不能在浏览器里直连。通用 test_ui 控制台仍从运行期方法表列出全部方法，包括产物取件。短脚本隔离夹具上已有 Linux Chrome 点击证据（四种协议探测、按所选协议跟踪已有任务并取件、经 MCP 提交并跟踪到成功），以及 pxdesign 短脚本 HTTP 真任务证据。这不是长期服务上的生产前端，也不是官方 GPU 设计或 v2 runtime。


HTTP/JSON-RPC `protenix`/`pxdesign` 经鉴权 → SQLite 持久受理 → Policy → 同一 Execute → 独立 CLI 进程组。`task.cancel` 对 queued 返回 `cancelled:true`；对 running 返回 `cancelling:true`，待进程组退出后 `task.get` 才显示 done/cancelled。正常停机也取消在途任务。`policy_cfg.max_workers: 1` 适合单 GPU；不要按 CPU 核数盲目启动大量推理。Protenix 成功还需 CIF/ERR 核验。已完成任务用 `task.artifacts` / `task.artifact` 按属主读取产物（相对路径，base64），不要把结果里的本机路径当成下载 URL。数据库重开恢复、systemd KillMode=control-group 下短脚本主进程 SIGKILL/清组/重启恢复均有实测；裸 SIGKILL、GPU 中断恢复和多实例 HA 未验收。

当前版本边界：`version` 字段尚未完成 v1/v2 独立 runtime 的结构化选择，实际调用仍是单一配置 binary + model_name；本轮只验 v1，不能把传入 v2 标签当作 v2 跑通。show_yeah 数据契约与取件缺口见 [对接说明](docs/show_yeah_data_contract.md)。

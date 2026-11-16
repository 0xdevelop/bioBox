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
- 原生 CLI 路径写在 `cli_cfg`（见示例配置），不要在代码或 URL 里写死。`cli_cfg.protenix.binary` / `cli_cfg.pxdesign.binary` 为空时，`Execute` 拒绝启动进程。作业输入会拷贝到 `cli_cfg.work_root/<task_id>/input/`，输出写到同任务的 `output/`，不修改原输入文件。`version` 仍只作方法参数，不作为 CLI 开关。`cli_cfg.execute_timeout_seconds` 为正数时，单次 Execute 到期会取消进程组，任务记 `system_error`（`error_message=task execution timed out`）；`0` 表示不限制。
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

填写服务 URL 和 `ptxk_` API key，选择协议后提交或跟踪任务，按属主读取真实产物。三维舞台由现有 TS/Bun → Go embed 构建链打包 Three.js；独立 HTML 仍保留 Canvas 兼容视图。支持切换结构文件、MODEL、链、主链示意/原子和配色，也可仅在浏览器内打开本地 CIF/PDB，不上传。操作、科学边界、原生轨迹能力和验收范围见 [三维结构展示](docs/structure_visualization.md)。


HTTP/JSON-RPC `protenix`/`pxdesign` 经鉴权 → SQLite 持久受理 → Policy → 同一 Execute → 独立 CLI 进程组。`task.cancel` 对 queued 返回 `cancelled:true`；对 running 返回 `cancelling:true`，待进程组退出后 `task.get` 才显示 done/cancelled。正常停机也取消在途任务。`policy_cfg.max_workers: 1` 适合单 GPU；不要按 CPU 核数盲目启动大量推理。Protenix 成功还需 CIF/ERR 核验。已完成任务用 `task.artifacts` / `task.artifact` 按属主读取产物（相对路径，base64），不要把结果里的本机路径当成下载 URL。数据库重开恢复、systemd KillMode=control-group 下短脚本主进程 SIGKILL/清组/重启恢复均有实测；裸 SIGKILL、GPU 中断恢复和多实例 HA 未验收。

当前版本边界：`version` 按 arguments 选择 `cli_cfg.protenix.version_runtimes` 中的独立 runtime，不进 URL。`v1` 未单独配置时回退到 `cli_cfg.protenix.binary`；`v2` 必须配置 `version_runtimes.v2.binary`，不会使用 v1 的 binary，未配置时拒绝启动。官方烟测仍只验证 v1；v2 仍缺授权权重，不能把传入 v2 或仅配置路径当作 v2 已跑通。show_yeah 数据契约与取件说明见 [对接说明](docs/show_yeah_data_contract.md)。

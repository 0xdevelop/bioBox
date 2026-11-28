<!-- TOC -->

- [1. 前端定位与边界](#1-前端定位与边界)
- [2. 配置与启动](#2-配置与启动)
- [3. 构建与发布](#3-构建与发布)
- [4. 验收边界与实现进度](#4-验收边界与实现进度)

<!-- /TOC -->

# 1. 前端定位与边界

`bb_frontend` 是带登录的业务界面，embed 在程序里。未登录时只有登录页，登录成功后才进入业务页。业务页为 `/` 或 `/business`，可提交并跟踪本账号的 Protenix / PXDesign 任务、查看历史、取消未完成任务、取回产物；管理页为 `/manage`，仅服务端确认的 admin 可见入口，可管理用户角色、全站任务、会话与资源准备。非 admin 入口隐藏，直接访问会提示权限不足。17170 测试台不是本界面的一部分。登录沿用 `auth.login.email`。内嵌页面用浏览器访问 bioBox 的主机名和本实例启用的 MCP 端口发送 `POST /` 的 `tools/call`。独立构建时可在 `bb_frontend/package.json` 的 `mcp` 明确指定服务地址。业务页提交输入文件。Protenix 随前端提供 `example_files/test_data_protenix.json` 示例。PXDesign 可在页面选择输入文件，也可在本机构建前通过 `example_files/test_data_pxdesign.json` 指定默认输入。文件先经 `POST /cli-input` 写到本机路径，再把该路径作为已注册方法的 `input` 交给 `protenix` 或 `pxdesign`。服务在调用方未给旗标时补上受管 runtime 要求的 `--use_msa false`，pxdesign 再补 `--preset preview`。本机 `cli_cfg` 的 host 为空时，由已配置的二进制在本机执行。

# 2. 配置与启动

配置沿用 `bb_config.FileConfig` 的 YAML/JSON/TOML 装配：

```yaml
frontend_cfg:
  enabled: true
  bind_address: "127.0.0.1"
  port: 13006
```

`frontend_cfg.enabled` 缺省开。显式 `false` 时 `main` 不调用 `FrontendServicesStart`。embed 仍在二进制里，关的是监听。显式启用时地址和端口必填，非法配置在加载时拒绝，端口占用直接返回错误。新生成的默认配置和发布配置示例包含此段；既有本地配置由维护者显式加入，不自动回写。

`main.go` 在数据库初始化和迁移成功后启动前端，退出时关闭前端监听。页面只提供构建产物，未知路径和非 GET/HEAD 请求返回中性空响应。

# 3. 构建与发布

`bb_frontend/package.json` 的 `build` 使用 Bun 原生构建 TypeScript/CSS，无第三方前端依赖。`bb_frontend/dist/` 不入仓，由 `//go:embed` 随 Go 二进制发布，部署时不依赖 Bun 或源码目录。

GoLand 共享 Run/Debug 配置（Run kind = Package，编译整个主包）和根 `./build.sh` 均在 Go 编译前构建 `bb_test_ui/web` 和 `bb_frontend`。命令行运行：

```bash
./gen_test_ui.sh && ./gen_frontend.sh && go run .
```

共享配置里，`bb_test_ui/web` 仍是 `NpmBeforeRunTask`（`package-manager: bun`，脚本 `build`）。`bb_frontend` 在 Go 编译前先跑 Shell Script 配置 `gen-frontend`，命令是 `bash ./gen_frontend.sh`：先注入测试数据，再 `bun run build`。GoLand 的构建输出位于启动前任务控制台；服务启动日志中的 `Frontend Run On` 仅在 `frontend_cfg.enabled` 时出现。

2026-11-03 用户要求参考 Cursor 会话 `cc85014d-c5c3-4ed9-9859-7e9ed65e99eb` 回灌通用能力：沿用二进制嵌入、配置控制和页面地址日志；测试台监听字段直接归现有 `bb_config.TestUIConfig`。该会话原话为「在之前的 config 基础上增加字段不就行了」。

# 4. 验收边界与实现进度

- [x] ✅ ACCEPTED — TS/Bun 页面壳子、前端配置域、静态资源嵌入与既有构建入口已完成本机验收。
- [x] ✅ ACCEPTED — 配置关闭、非法配置、端口冲突、HTTP 资源访问和监听关闭回归通过；ego lite 实测页面渲染、根路径刷新、JS/CSS 加载及源码路径空响应通过。
- [x] ✅ ACCEPTED — 本地 SQLite 环境中，原 GoLand 配置完成主进程启动、前端地址日志和正常退出验收；退出码为 0，全部监听端口释放。MySQL/PostgreSQL/Redis 的真实业务验收不在此结论内。
- [x] ✅ ACCEPTED — Mac 回环地址和 Spark 局域网 HTTP 地址均经浏览器实测。页面向各自实例的 MCP 端口发送请求，公共 `test` 返回 `isError=false`，无效账号登录返回业务错误。
- [ ] 🟡 DONE — 两个计算按钮和 `task.get` 轮询已实现，带身份提交任务直至取得产物的 Web 流程待验收。

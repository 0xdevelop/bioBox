# bioBox 业务挂载说明

这是承载关系说明，不是业务前端。
程序级 runtime 承载下面挂上去的计算。
业务前端是 `bb_frontend` 里手写的页面，
和这份说明不是同一个东西。

与 [bioBox 业务挂载说明](biobox_runtime.html) 是同一张可点击的图。
点节点看详情。
存储不单列成步骤，API 执行器不指向存储。
测试台可选多种协议。业务 Web 通过 MCP 调用已注册方法。
前置检查通过才调用 Protenix（v1 / v2）或 pxdesign（现阶段一种）。

## 程序级 runtime

四面同时属于 main。
它们承载 Protenix 和 pxdesign，
不排成这两条计算前面的步骤。

### 选配置

debug 用 `./example_files/config_local.yaml`。
test 与 release 用 `./conf/config.yaml`。
读失败或解析失败时进程退出，不继续听协议。
文件不存在时写出缺省再加载。
生成的缺省只监听 `127.0.0.1`。
对外部署改配置里的地址。

### 监听

`ApiConfig.Validate` 拒绝空地址和非法端口，
代码不补本机地址。
通过后 `StartAPIServices` 按 `Enabled` 启动
JSON-RPC、MCP、WebSocket、gRPC。
每种自己的绑定地址。
业务入口都是 `tools/call`。

`APIExecuter` 只收这一次 `tools/call`，
取出 name 和 arguments，
门禁通过后调用已注册的 Execute。
它不指向存储。
存储属于 runtime 已经打开的数据面。
Execute 要用库就用那一面。

生成缺省从 17168 起，只绑 `127.0.0.1`。
示例文件里 JSON-RPC 是 `0.0.0.0:13001`，
MCP 13002，WebSocket 13004，gRPC 13005。
实际听哪一个，以被选中的那份配置为准。

### 调度

监听启动之后，main 调用 `PolicyServicesStart`。
循环按空闲名额派发单次 Worker。
Worker 调用 `ExecuteNextQueuedTask`，
认领后把受理时的 arguments 交回同一个 Execute。
Worker 不常驻。

生成缺省里 `max_workers` 为 1，
`workers_scaller` 为 1。
名额是 CPU 数乘倍率，再受 `max_workers` 封顶。
长任务占着原名额，直到这次结束。

### 容错

重启留下的 `running`、
且不在本进程执行集合里的任务，
由 `RequeueOrphanedTasks` 捞回，
再进入同一个 Execute。
`execute_timeout_enabled` 为 true 时，
从这次开始执行起算超时，
到期取消进程组，任务记 `system_error`。

`task.cancel` 先写下取消，
再取消 Execute 的 context。
进程组退出后才落 done/cancelled。
停机取消在途 Execute。
一条计算失败不把另一条或整个服务记成失败。

## 业务挂载

两条挂在上面这层 runtime 上。
先挂上，再按各自阈值做同一套前置检查，符合才调用。

### 挂到已有 runtime 上

`bb_ability/ability.go` 的 `LoadAbilityAPIMethods`
显式调用两个适配包的 `Load`，
再加载 `protenix` 与 `pxdesign`。
没有 init 自注册。
适配包只提供这一次检查用的阈值，不另注册 API。

两个方法注册时 `Async` 为 true，受保护。
四个协议都把 `tools/call` 交给 `APIExecuter`。
门禁通过后 `AcceptAsyncTask` 写入 queued 任务并返回 `task_id`。
这一步不启动包装命令。
调度认领后才调用同一个 Execute。

### Protenix

方法名 `protenix`。
必填 `input` 和 `out_dir`。
`version` 只允许 v1 或 v2，缺省 v1，不进入 CLI 参数。
`dtype` 只允许 bf16 或 fp32。
参数不对记 `preparation_failed`，不组命令。

`version_runtimes.v1.binary` 非空时，
v1 或缺省用它，否则回退 `cli_cfg.protenix.binary`。
v2 必须有 `version_runtimes.v2.binary`。
为空就返回 `protenix v2 CLI binary is not configured`，
不用 v1 的路径。
集合名是 `protenix.v1` 或 `protenix.v2`。
v2 自己写了 resources 就用那一组，没写则沿用上级 resources。

选中的命令文件名仍是 `protenix-run`
且 version 不是 v1 时，
返回 `runtime resource branch unsupported`，
不暂存输入、不启动进程。

### pxdesign

方法名 `pxdesign`。
必填 `input` 和 `dump_dir`。
preset 只允许 preview、extended、custom。
dtype 只允许 fp32、bf16。
没有 `version_runtimes`。

`bb_ability_adapter_pxdesign.Resolve` 只取 `cli_cfg.pxdesign`。
binary 为空返回 `pxdesign CLI binary is not configured`，
不猜路径，也不借用 Protenix。
集合名 `pxdesign.preview`。
官方文件名是 `pxdesign-run` 时，
清单必须盖住 runtime conf 里算出的那一组文件。

### 同一套前置检查

两条都调用 `bb_cli_proc.ConfirmCommandReady`。
官方包装再核清单、参数和已安装运行环境。
权重按已配置 sha256 与大小核对，不在这次调用里下载。
差异只是这一条已经 Resolve 出来的 binary、参数和阈值。

符合才继续。
不符合就写明先走 `resource.prepare`，
不暂存输入、不启动这一条，
也不拖垮另一条或服务。
`resource.status` 只查询。
`main` 不接受 `resource` 参数，包装不再调用它。

### 符合才调用

检查通过后才 `StageInput`，
拷到 `work_root/<task_id>/input/`。
拒绝符号链接。
真实输出目录是作业目录下的 `output/`。
Protenix 的 `out_dir`、pxdesign 的 `dump_dir`
只做必填校验，不拿去当 CLI 的输出路径。
pxdesign 再按正文里的相对路径拷伴随文件。

`host` 为空时 `Run` 本机 exec 该条 binary。
非空时本机 ssh 到该 host，在那台机器上执行这条 binary。
两条的 host 可以不同。
检查不过不会走到 exec，也不会 ssh。
bioBox 在自己所在的机器检查运行环境和资源，并在 `work_root` 暂存输入、读取产物。
分机时，预检所用文件与作业目录须在两端以相同路径可见。Mac 的 GoLand 远程调试 Spark 时，以上操作都在 Spark 完成。

Protenix 包装加载 `conf/protenix.env` 和 venv 后执行 Python，
子命令只接受 `pred`。`--use_msa` 必须是 false。
pxdesign 包装加载 `conf/pxdesign.env`，
子命令只接受 `pipeline`，preset 必须是 preview。

`Run` 用新进程组。
标准输出和错误写进作业目录的 `run.log`。
Start 失败记 launch_failed，非 0 退出记 process_failed。
取消时先向整个进程组发 SIGTERM，
等待 `cancel_grace_seconds`，再 SIGKILL。
Python 返回 0 但 `output/ERR/` 非空时包装退出 3。
这些状态写在任务上。

## 调用方

调用方在挂载外面。
它们调用已经挂上的 API。
`enabled` 只决定是否把这个面提供出来。

### test_ui

test_ui 是多协议 API 调试台。

`test_ui_cfg.enabled` 缺席时，非 release 开、release 关。
显式 true 或 false 覆盖这个缺省。
main 只在 `TestUIEnabled` 为真时调用 `LoadTestWeb`。
显式 false 不监听。embed 仍在二进制里。

页面里可以选 JSON-RPC、MCP、WebSocket、gRPC，也可以选 REST。
选中的协议经调试台代理转到对应入口，
调的是同一批已注册方法，
包括 `protenix`、`pxdesign` 和 `task.get`。
调试台自己不跑这两条计算。

地址不合法只记错误，不因此停掉已经启动的 API。
示例文件是 `127.0.0.1:13003`。
生成缺省是 `127.0.0.1:17170`。

### bb_frontend

手写页面在 `bb_frontend`。
show_yeah 不是这个 frontend。

`FrontendServicesStart` 按 `frontend_cfg.enabled` 提供这个页面。
缺省开。显式 false 不监听，embed 仍在二进制里。
内嵌页面按当前页面主机名和实例配置的 MCP 端口发送 `tools/call`。在页面登录后，后续调用通过 Bearer 携带会话凭证。

两个表单各自接受输入文件。页面先将文件传到本实例，再分别调用 `protenix` 或 `pxdesign`。每个表单都有自己的任务状态和输出区。

受理返回 `task_id` 之后，
该条每 2 秒调用 `task.get`，
直到 `run_status` 为 done 或调用出错。
转盘画布只读这两条当前的状态文本，
按 PROTENIX、PXDESIGN 两条车道显示。
转盘不发起 `tools/call`。

示例文件前端地址是 `127.0.0.1:13006`。
生成缺省是 `127.0.0.1:17173`。
端口被占用时 main 返回。

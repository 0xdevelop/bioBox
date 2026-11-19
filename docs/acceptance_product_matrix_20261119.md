# 当前成品链路验收：2026-11-19（Asia/Shanghai，UTC+8）

结论：**尚不满足整系列发版条件**。本轮已完成当前 arm64 release 三包 → 解压二进制 → SQLite/Bearer HTTP → 官方 v1/PXDesign GPU → 产物取件 → 完成任务重启回看的真实切片；没有正式安装、打 tag、创建 release 或启动宿主 systemd 服务。邮件和步进是过程机制，不是最终产物完成的替代品。

## 1. 产品与证据矩阵

| 产品 | 实际交付边界 | 本轮实证 | 仍不能宣称 |
| --- | --- | --- | --- |
| 独立 Protenix | 独立 zip / install_protenix.sh / 独立 venv；本项目验收范围为公开 v1 | 当前 arm64 包自包含、安装材料无仓库绝对路径依赖；包内 wrapper 接既有已验证 venv/权重执行官方输入，真实 GPU 成功 | 当前包在新机器上的冷安装；amd64 运行；v2 |
| 独立 PXDesign | 独立 zip / install_pxdesign.sh / 独立 venv，不能与 Protenix venv 合并 | 当前 arm64 包自包含；官方 PDL1 preview 完整流水线真实 GPU 成功 | extended/长跑、当前包冷安装、amd64 运行、科学质量保证 |
| 整合 Go 服务 | 一个 zip、一个 install_protenix_service.sh；正式目标固定 /protenix_service，运行时在 runtime/protenix 和 runtime/pxdesign | 当前 arm64 解压二进制实际启动；SQLite/Bearer、真实两种 GPU 作业、CIF 取件与完成任务重启回看通过 | root 正式安装、真实 systemd 启停、清洁机器、全部失败回退、amd64 整合包 |

独立包的正式目录/安装命令与整合包 install 菜单均见 [根 README](../README.md)。本轮没有改变正式安装前缀，也没有在 ~/.local/share 重复部署。独立模式过往安装/生命周期证据见 [acceptance_package.md](acceptance_package.md)；旧记录不是本轮清洁安装证据。

## 2. 干净提交与本地包

现场模板 `09058578159c03fdfbf4310cb5149364b4e39e17`、实例 `861c80cdb34dd91dfaa9da1882771d89935eef8f`；开始时两仓工作区干净且与 origin/main 同步。

执行既有入口，不执行 all 或任何 install：

```bash
FORCE_PACK=1 ./build.sh pack release linux_arm64
```

exit 0；三个包的 BUILD_INFO 均为实例提交 `861c80cdb34dd91dfaa9da1882771d89935eef8f`、dirty=0、arm64、release。位置在 `build/upload_tmp_dir/`：

| 文件 | SHA-256 |
| --- | --- |
| protenix_linux_arm64_release_v0.0.1.zip | 8defc1d750636fb50a8282167192f52f692d74d5b87605ce8c8d45760023ba72 |
| pxdesign_linux_arm64_release_v0.0.1.zip | c262a8ca8469d84ae1d35572a3575b5781a6173c4015447460dd9ccf06be115f |
| protenix_service_linux_arm64_release_v0.0.1.zip | 754ea0b8664deec578d860527f6af6b497333cd2bea00a916b1ec98957a9f3da |

包审计通过：整合包 Protenix 315 个非目录文件、PXDesign 72 个非目录文件与独立包逐文件一致；私有 config_local.yaml/夹具凭据未入包；可执行安装材料不依赖源码 checkout；默认 SQLite、独立 email 能力块与正式 /protenix_service CLI 路径保持。解压服务二进制 SHA-256 为 `7d61a5c561fef2d85a221ff6edc5fe730c210b6ec87c81d176926f598e58e5a3`。

## 3. 当前 release 二进制的真实 HTTP/GPU 切片

临时根 `tmp/artifact-acceptance-20261119/`。从上面的整合 zip 解压并直接运行其中的 release 二进制，cwd 为本轮独立临时目录，所有监听仅 127.0.0.1。

- 只复用已有 TestPrepareStepBrowserFixture 准备隔离 SQLite 测试身份；临时凭据 0600，不输出到报告。原 fixture CLI **未用于推理**。
- 推理使用当前包的真实 wrapper，临时探针链接既有、已验证的两个独立 venv/权重。因此不是新的依赖安装、不是冷环境或正式安装证据；不存在修改正式安装前缀的做法。
- 使用包内官方 `examples/input.json` 与 `examples/PDL1_quick_start.yaml`。Protenix 指定公开 v1、无 MSA、seed 101；PXDesign 指定 preview、N_sample=2、N_max_runs=1。未运行 v2。

| 项目 | Protenix v1 | PXDesign preview |
| --- | --- | --- |
| 时间（Asia/Shanghai UTC+8） | 13:41:45–13:44:30 | 13:44:31–13:45:59 |
| task_id | e81af560-5d48-46cc-a037-a948ec699a5a | 947ca42b-7596-46c3-b8bf-ec5bb20c0ed5 |
| 观察墙钟 | 165.314 秒 | 88.354 秒 |
| 终态 / 进程退出码 | done/success / 0 | done/success / 0 |
| 实际 CIF / ERR | 5 / false | 5 / false |
| 执行轮次 / 持久步骤 | 1 / 6 | 1 / 6 |
| 运行中观察到业务步进 | 是 | 是 |
| 执行窗口 GPU 利用率峰值 | 96% | 96% |

GPU 数值是窗口采样，不是人为进度或科学质量评分。原有 ComfyUI 常驻进程未结束；验毕 GPU 计算进程仅剩原有 ComfyUI。

实际调用 task.artifacts/task.artifact 取件，base64 解码后校验 SHA-256，并确认是带原子记录的 CIF：

- Protenix：`output/user_test/seed_101/predictions/user_test_sample_0.cif`，542232 字节，SHA-256 `f15cf30bcbd625e22ff289a7192dbaead5bbde6ea336981c86bee7abcbe2919a`。
- PXDesign：`output/design_outputs/PDL1_quick_start/orig_designed/rank_2.cif`，140669 字节，SHA-256 `aaab3d55e3c062170586e57b5efdcca1cf01332387ea6f2d4ccf67b37da59554`。
- 产物列表只含 output/ 与顶层 run.log，不把输入侧车当成结果；原官方输入内容未改。
- 无凭据 task.list 被拒绝；MCP/WebSocket/gRPC 本轮只核对监听存在，不冒充这些协议都完成了实际推理请求。
- release 没有 test_ui 监听，符合既有边界；内嵌 frontend 首页 HTTP 200。**首页 200 不是 Three.js/结构展示验收**；本轮未驱动浏览器，历史浏览器证据见 [acceptance_service.md](acceptance_service.md)。
- 缺输入任务 `7540ccbe-79d1-4b35-8ea4-05af7a0ce532` 实际落 done/system_error，不假成功。
- 停止再启动同一解压二进制，两个完成任务、轮次、步骤及取件保持，不自动重跑完成推理。此项不替代执行中崩溃恢复或真实 GPU 取消验收。

## 4. 实证发现的下一缺口

### P0：整合升级未实现跨运行时失败一致性

使用**当前未修改的安装脚本**，在 bwrap 的最小 tmpfs 假根里把 /protenix_service、systemd 目录、包目录映射到本轮 tmp；宿主 home 不暴露，系统执行文件/库只读，systemctl 与两个 runtime 安装器均为明确的记录桩。不是宿主 root 安装，也没有真实 systemd/pip/GPU 操作。

令第一个 runtime 升级成功、第二个退出 42，根 install 脚本实际退出 1；结果为：

```text
protenix_current=releases/new
pxdesign_current=releases/old
go_binary_unchanged=true
old_service_restart_requested=true
```

这说明单运行时的“失败不切 current”不能当成整合服务跨组件回退。下一修复应先定义并实现同一服务升级失败的一致性/恢复契约，复用已有 install/rollback 机制，不用一次性脚本把 mixed runtime 当作已回退成功。本轮只复现并保留证据，没有擅自改这个生命周期契约。

### P0：正式配置的每部署唯一凭据准备

包中示例仍含开发用认证 secret 占位值；本轮仅输出检测布尔值，没有输出值。正式安装脚本复制示例配置，并未证明会为每部署提供唯一 secret。本轮 HTTP 使用随机测试配置，不能因此宣称原样示例已具备安全生产开户/鉴权条件。该缺口应在正式网络暴露前完成配置供给/校验契约，不能打印或修改受保护的 config_local.yaml。

### P1：amd64 当前产物链缺失

现场 amd64 两个单产品 zip 仍为 `7d208afe550972486b35985cb1de2a5377b85cf5`；当前 amd64 集成 zip 不存在。spark 上默认交叉 C 编译器及有效显式覆盖均未就绪，而 SQLite 服务必须 CGO；不能以 CGO_ENABLED=0 假构建替代。未安装系统编译器，也未触发会创建真实 release 的 workflow。需在授权的构建环境补齐当前六包矩阵，并分别说明 x86 实机验证边界。

### 其他未覆盖

当前整合包在新机器的完整依赖/公开权重准备、/protenix_service 正式安装与 systemd 真启停仍需另行授权与证据；执行中异常退出恢复、官方 GPU 任务取消、当前调试页与本轮真实 CIF 的浏览器联动仍不能用本轮完成任务回看/首页 200 代替。科学质量不是退出码/CIF/GPU 占用能证明的指标。

## 5. 原始证据与收尾

全部原始证据在 `tmp/artifact-acceptance-20261119/`（ignored、可删）：

- `pack-arm64.log`、`arm64-packages.sha256`、`package-audit.json`。
- `http-probe.log`、`http-acceptance.json`、`evidence-summary.json`。
- `protenix-v1-task.json`、`pxdesign-preview-task.json`、`missing-input-task.json`、产物列表 JSON 与两个 `*-download.cif`。
- `service-run/jobs/<task_id>/run.log` 为真实官方 CLI 输出；`release-process.log` 为服务记录。
- `update-failure-fixture/{result.txt,installer.log,root/events.log,root/systemctl-fixture.log}` 为上述升级失败纯 fixture 证据。初次只读根未能创建挂载目标，未进入安装流程、不计入验收；最终复测采用最小 tmpfs 假根并实际经过两个 runtime 记录桩。

自己的临时服务已停止，全部六个临时端口关闭，临时 API key、临时服务配置与未使用的 fixture CLI 已删除，日志中该 key 泄漏计数 0。未触碰原有用户任务；两份受保护配置固定哈希保持，build_srcs 干净；未安装 MySQL/Redis、未发邮件、未使用 Gmail、未新建 tag/release。

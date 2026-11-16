# 三维结构展示

## 使用

沿用现有构建入口：`./gen_test_ui.sh` 安装锁定依赖并由 Bun 打包，`go build` 将资源与调试页嵌入二进制。开发启动也调用同一个生成脚本，不增加 Vite、React 或第二套构建工具。运行环境不需要 Bun、CDN 或 node_modules。

启用 `test_ui_cfg` 后打开 `/flow_debug.html`：

1. 通过现有四协议选择器、属主 API key 跟踪真实 task；服务成功返回后从 `task.artifacts` 列表自动读取首个 CIF/PDB。结构产物下拉框和产物表可切换文件，不重新提交推理。
2. 同一文件多个 `MODEL` 独立选择，不重叠绘制、不推断时间顺序。可选单链、按链/元素配色、原子/Cα 主链示意。
3. Three.js 舞台左键拖拽旋转、滚轮缩放，右键/方向键平移；“暂停旋转”“重置视角”控制相机。悬停原子显示文件原始 Å 坐标，不是居中后的显示坐标。窗口缩放自动适配。`prefers-reduced-motion` 禁止自动旋转和装饰环运动，仍可手动操作。
4. WebGL2 或模块加载失败时显示错误并回落兼容 Canvas；“重试三维舞台”可重新创建。直接保存打开独立 HTML 也可使用兼容视图。接口请求失败、切换任务/样本时撤下旧结构，迟到响应不得串到新任务/样本。
5. “打开本地 CIF/PDB”仅在浏览器内解析，不上传、不调用 GPU、不需要 API key；这会停止当前任务跟踪并清空旧产物。本地文件来源未验证，不能据此标为服务预测。文件上限 16 MiB；WebGL 舞台上限 100000 原子，超出明确拒绝并保留兼容视图，不偷偷截取结构。

不修改 `show_yeah`；仅借鉴其深色仪表、环形枢纽、青绿色光纤的视觉语言。装饰环、星点和流程光点不是分子、去噪帧或推理进度。本切片无需 Blender 装饰模型，不为了工具而加入资产。

## 科学边界

- 当前呈现的是文件中的结构快照，原子相对位置不做散开、聚合或插值。居中和整体等比显示缩放不改变分子几何；旋转只改变相机。球径是显示符号，不是范德华半径。尚无二级结构指派、ribbon/cartoon、溶剂表面、化学键判断或结构质量评分。
- Cα 连线只在同一 MODEL/链/PDB TER 段、无插入码、相邻残基编号且间距 2.5–4.5 Å 时画出；只是主链示意，不是化学键验证。插入码、编号跳跃、链间和 MODEL 间不猜连接。超距会显示警告并断线，不能把“成功生成文件”当成“生物物理合理的折叠”。
- 读取官方逐行 mmCIF/PDB 坐标格式，保留 residue、chain、MODEL、原始坐标；仅显示 primary alternate location（空/A/1），丢弃显式零 occupancy。不是完整通用 mmCIF 语法解析器（不支持 atom_site 的跨行/分号字符串）。未据 B-factor 自动推断置信度；PXDesign dumper 会写入占位 B-factor。
- 文件名里的 seed/sample/rank 和 MODEL 序号都是标识，不能当作物理时间。切换样本不构成折叠动画。

## 已安装上游的轨迹能力证据

本机核查安装源 Protenix `85767b811c40` 与 PXDesign `f788441313c8`，未修改上游或 `build_srcs`。

| 来源（相对本仓根） | 证据 | 结论 |
| --- | --- | --- |
| `prefix/protenix/src/protenix-85767b811c40/protenix/model/generator.py:189–286` | 去噪循环只更新 `x_l`，循环后返回最终 `x_l`；没有逐步保存/回调出口 | 存在内部算法迭代，不代表已有可取件的轨迹 |
| 同安装源 `runner/dumper.py:175–211` | 输入 `[N_sample,N_atom,3]`，按 sample/rank 写出各个最终 CIF | 原生支持多样本；sample/rank 非帧序号 |
| 同安装源 `runner/batch_inference.py:291–363`、`configs/configs_base.py:182–184` | `n_sample` / `sample_diffusion.N_sample` 控制样本数，`N_step` 控制去噪步数 | 控制步数不等于保存中间帧；未发现公开原生轨迹导出参数 |
| `prefix/pxdesign/src/pxdesign-f788441313c8/pxdesign/model/generator.py:133–230` | 内部逐步更新坐标，循环后仅返回最终 `x_l` | 同样没有现成的中间构象产物 |
| 同安装源 `pxdesign/runner/dumper.py:130–157`、`pxdesign/runner/cli.py:118–155` | `N_sample` / `N_step` 参数，逐 sample 输出 CIF；B-factor 为占位零 | 多样本可展示，不推断为物理折叠或置信度 |

因此本切片不实现伪轨迹/伪折叠回放，不改上游以截取中间张量。未来只有拿到带明确迭代序号的真实产物并确认可安全取件，才接“算法去噪轨迹”，且仍不等同物理折叠动力学。

## 验证范围

- [x] ✅ ACCEPTED — 官方 HTTP smoke CIF 168 原子、20 个 Cα 的读取回归通过。该极短去噪烟测的 19 个相邻 Cα 距离均超界，界面会明确警告且不画假主链；其存在不是折叠质量证据。
- [x] ✅ ACCEPTED — 解析/拓扑测试覆盖零坐标、quoted atom name、altloc、零 occupancy、MODEL/链/TER/插入码/缺口与远距断线、非蛋白 calcium 排除、原始坐标保留；VM DOM 测试覆盖不同样本迟到响应、换任务清空、读取失败、MODEL/链切换及 Three 数据交接。VM DOM 不是真浏览器证据。
- [x] ✅ ACCEPTED — 本机只读解析现有上游示例：7pzb PDB 4698 原子/6 链/451 条 Cα 连线；2lwu CIF 20 个 MODEL、首个 MODEL 211 原子/14 条连线；5o45 CIF 2559 原子/4 链/131 条连线。示例不是新增 Protenix/PXDesign 推理。
- [x] ✅ ACCEPTED — Three.js 显示变换单测、严格 TypeScript 检查、Bun 构建、Go 测试与编译通过；HTTP 测试验证离仓 embed 的 JS 内容、MIME 与无外部运行时依赖。
- [x] ✅ ACCEPTED — 2026-11-16 19:06 Asia/Shanghai，Linux Google Chrome 157 真浏览器 WebGL2 验收已通过：同源调试页加载嵌入的 `/structure_stage.js`，对官方烟测 CIF 绘制 168 个真实原子，显示 19 处相邻 Cα 超距警告，悬停坐标与文件一致，左键旋转 / 滚轮缩放 / MODEL 与链切换 / 窗口缩放均在 Three.js 舞台上完成。证据在 `docs/acceptance_service.md` 与 `tmp/handoff-followup/phase4-threejs-browser-20261116/`。不能引用 2026-11-15 的旧 Canvas 截图。SwiftShader 是该次 headless 会话的实际 WebGL 后端，不是 NVIDIA 推理。
- [ ] 🟡 DONE — `prefers-reduced-motion`、强制 WebGL 上下文丢失后的重试、以及连续导入后的 geometry 泄漏计数，本轮点击尚未单独覆盖。

Three 舞台 DOM 的 `data-draw-calls`、`data-geometries`、`data-frame-ms` 提供运行时观测（非性能承诺）。本次验收记录了烟测结构的 draw_calls/geometries/frame_ms，以及旋转、缩放、切模型/链后的实际画面差异；加载失败回退在修正夹具静态资源之前的第一次点击中出现，修正后第二次点击不再回退。

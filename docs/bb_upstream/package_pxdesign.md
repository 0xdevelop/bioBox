# PXDesign 命令行工具

在一台装了 NVIDIA GPU（如 GB10 / aarch64）的机器上，对给定靶点做蛋白 binder 设计。本包是命令行工具，不是常驻服务。

> 本包可独立使用（下文）。与 Go 服务 `bioBox` 一起安装时，改用整合包 `bioBox_linux_<arch>_release_v*.zip`：其中 `runtime/pxdesign/` 就是本包内容，由 root 跑 `install_bioBox.sh` 显式装到 `/bioBox/runtime/pxdesign`，与本包的独立安装目录互不影响。

## 前置条件

- NVIDIA 驱动与 CUDA 13 工具链（有 `nvidia-smi`、`nvcc`，默认 `CUDA_HOME=/usr/local/cuda-13.0`）
- [uv](https://github.com/astral-sh/uv)，且能解析 Python 3.14
- 能访问公网（装 pip 依赖、下载权重与 AF2/MPNN 等工具权重）
- 不需要 sudo、Docker、conda

## 安装

```bash
unzip pxdesign_linux_arm64_release_v*.zip
cd pxdesign
./install_pxdesign.sh install
```

默认装到 `~/.local/share/pxdesign`。换目录：`PXDESIGN_PREFIX=/your/dir ./install_pxdesign.sh install`。
既有机器上的权重不再通过已退役环境变量复制；在整合包部署中请改 `runtime/pxdesign/conf/pxdesign.env` 的数据根路径（或使用既有目录软链接）并由 `bioBox resource prepare` 校验就位。

首次安装会下载约 4GB 权重（PXDesign、Protenix v0.5.0、AF2 两个参数文件、ProteinMPNN、CCD 缓存）。
命令入口是 `~/.local/share/pxdesign/bin/pxdesign-run`。

> **独立模式当前边界（未完成闭环）**：`WEIGHTS_REUSE_FROM` 已退役；独立安装脚本与包装器 `pxdesign-run` 会查找相邻 Go 协调器 `${PREFIX}/../../bioBox`（独立单产品包未随包提供该协调器），缺失时将明确以 **exit code 4** 拒绝执行。独立安装与独立设计流程当前尚未通过验收，请优先使用整合包 `bioBox_linux_<arch>_release_v*.zip`。

## 跑一次设计

```bash
RUN=~/.local/share/pxdesign/bin/pxdesign-run
mkdir -p ~/pxd_job
cp -a examples ~/pxd_job/examples
cd ~/pxd_job
"$RUN" pipeline --preset preview -i ./examples/PDL1_quick_start.yaml -o ./out --N_sample 2
```

`pxdesign-run` 会自动补上权重目录、`--use_fast_ln False`、`--num_workers 0`。
整合包（或提供相邻 `bioBox` 协调器并完成资源就绪后）的成功判断：退出码 0、`out/` 下有 `.cif`、没有 `out/ERR/`、运行时 GPU 有占用；未配置协调器的独立单产品包执行会以 exit 4 失败。

## 升级 / 回退 / 卸载 / 查看状态

```bash
./install_pxdesign.sh update            # 在新版本解压目录里执行；配置和权重保留，失败不影响当前版本
./install_pxdesign.sh rollback          # 回退上一版本
./install_pxdesign.sh status            # 版本、提交、是否可运行
./install_pxdesign.sh uninstall         # 卸载，保留配置和权重
./install_pxdesign.sh uninstall --purge # 连配置和权重一起删
```

## 常见问题

| 现象 | 处理 |
|------|------|
| 找不到 uv 或 Python 3.14 | 装 uv 后确认 `uv python list` 含 3.14 |
| 找不到 nvcc | 装 CUDA 13 工具链，或把 `CUDA_HOME` 指对 |
| 退出码 0 但只有 `ERR/`、没有 CIF | 按失败任务排查输入 |
| GPU 利用率一直 0 | 查驱动、`nvidia-smi`、`torch.cuda.is_available()` |
| layernorm / AF2 相关报错 | 入口默认已带 `--use_fast_ln False --num_workers 0`；确认没被手动覆盖 |

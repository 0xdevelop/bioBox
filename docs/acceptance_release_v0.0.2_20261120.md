# v0.0.2 发版与正式安装实测记录（2026-11-20）

本文只记录已经实际执行的步骤、命令、时间和结果。未执行的步骤明确标为未执行，不把计划写成已发生。时间一律为 Asia/Shanghai（UTC+8）。目标机为 spark-901f（aarch64 / arm64），工作区 `/home/certa/workspace`。

保护约定：未打印或改写 `protenix_service/example_files/config_local.yaml`。实测结束时该文件 sha256 仍为 `684b906075c72cd76806cafd645bb7da9c6ac6b235550b5e77eaf15f0e31bf6c`。本文不记录配置中的凭据。

## 1. 推送已完成的本地提交

推前核对（约 21:02:59）：两个仓库工作区干净；作者与提交者均为 yerikokay；trailer 为空；分支均为 `main`。

- 模板 `project_template_go`：HEAD `468906aa422d038cd55a4a5f74e6c64440b11c44`，远端基线 `09058578159c03fdfbf4310cb5149364b4e39e17`。
- 实例 `protenix_build`：HEAD `8ddf8a596126c0bccf9647375b30729274c9448c`，待推提交依次为 `e50885bacd550338e19df6c991c0d76210afb672`、`8ddf8a596126c0bccf9647375b30729274c9448c`，远端基线 `a7c0bbcf10f7e3112726af97db96362d4a9e938e`。

实际命令为正常 `git push origin HEAD:refs/heads/main`（无 force、无 rebase），先模板后实例。

推后 `git ls-remote` 与 GitHub API 核对：

| 仓库 | 远端 `refs/heads/main` | 时间（约） |
| --- | --- | --- |
| https://github.com/0xdevelop/project_template_go | `468906aa422d038cd55a4a5f74e6c64440b11c44` | 21:03 左右 |
| https://github.com/0xdevelop/protenix_build | `8ddf8a596126c0bccf9647375b30729274c9448c` | 21:03 左右 |

提交页：

- https://github.com/0xdevelop/project_template_go/commit/468906aa422d038cd55a4a5f74e6c64440b11c44
- https://github.com/0xdevelop/protenix_build/commit/e50885bacd550338e19df6c991c0d76210afb672
- https://github.com/0xdevelop/protenix_build/commit/8ddf8a596126c0bccf9647375b30729274c9448c

推后只读核对：`protenix_build` 配置版本仍为 `v0.0.1`；本地与远端均无 tag、无 GitHub Release。工作区干净。受保护配置哈希未变。

## 2. 无参执行一次 `./git_tag.sh`

21:05:36 在 `/home/certa/workspace/protenix_build`、HEAD `8ddf8a596126c0bccf9647375b30729274c9448c`、版本 `v0.0.1` 下只运行了一次：

```bash
./git_tag.sh
```

未加 `--publish` 或其他参数，之后也没有重跑该脚本。

脚本将版本递增为 `v0.0.2`，生成 changelog 与 API 文档，并在 21:05:38 创建提交：

- 提交：`1e3bb50df7b19b5b7f40cce440739b29de487ea6`
- 说明：`Release:--: v0.0.2_2026-11-20_13:05:38_UTC`
- 作者/提交者：yerikokay；trailer 为空
- 父提交：`8ddf8a596126c0bccf9647375b30729274c9448c`
- 变更文件仅：`changelog/v0.0.2.md`、`gen_pre_build_config_linux_amd64.yaml`、`gen_pre_build_config_linux_arm64.yaml`、`pre_build_config_template.yaml`、`protenix_service/config/config.go`、`protenix_service/docs/api_methods.md`
- 本地轻量 tag：`v0.0.2`、`latest`，均指向上述提交

首次发版时脚本按模板执行 `git push --delete origin latest`，远端当时不存在 `latest`，失败信息为：

```
error: 无法删除 'latest'：远程引用不存在
error: 无法推送一些引用到 'github.com:0xdevelop/protenix_build.git'
```

脚本在创建本地提交和 tag 之后退出码 1，远端 `main` 仍停在 `8ddf8a5`，版本提交尚未推送。随后在查清状态后，未重跑 `git_tag.sh`，只把脚本已产生的版本提交与 tag 用正常 push 补完：

```bash
git push origin HEAD:refs/heads/main
git push origin refs/tags/latest
git push origin refs/tags/v0.0.2
```

21:06:41 `git ls-remote` 结果（三条引用均为轻量 tag / 分支，指向同一提交）：

```
1e3bb50df7b19b5b7f40cce440739b29de487ea6	refs/heads/main
1e3bb50df7b19b5b7f40cce440739b29de487ea6	refs/tags/latest
1e3bb50df7b19b5b7f40cce440739b29de487ea6	refs/tags/v0.0.2
```

## 3. GitHub Actions 三段发版

由推送 `v0.0.2` 触发，未使用 `workflow_dispatch`。

- Workflow：`Release_Build`（`.github/workflows/release.yml`）
- Run：https://github.com/0xdevelop/protenix_build/actions/runs/44741421709
- Run id：`44741421709`，run_number 1，event=`push`，`head_sha=1e3bb50df7b19b5b7f40cce440739b29de487ea6`，`head_branch=v0.0.2`
- 开始：21:06:37；整体结束：21:09:47；整体 conclusion 为 `failure`（仅最后一段失败，见下）

| 阶段 | Job | Runner labels | 结论 | 时间（Asia/Shanghai） |
| --- | --- | --- | --- | --- |
| release | `release`（id 136305739795） | ubuntu-latest | success | 21:06:39–21:06:49 |
| 原生构建 | `Native linux/amd64`（id 136305837514） | ubuntu-24.04 | success | 21:06:51–21:08:32；其中打包 21:07:19–21:08:21 |
| 原生构建 | `Native linux/arm64`（id 136305837489） | ubuntu-24.04-arm | success | 21:06:53–21:08:40；其中打包 21:07:25–21:08:30 |
| Build | `Build`（id 136306753311） | ubuntu-latest | success | 21:08:43–21:09:33；上传资产 21:09:21–21:09:27 |
| RemoveOldRelease | `RemoveOldRelease`（id 136307187095） | ubuntu-latest | failure | 21:09:35–21:09:47；失败步骤 21:09:44 |

`release` 阶段已创建公开 Release（非 draft、非 prerelease）：

- https://github.com/0xdevelop/protenix_build/releases/tag/v0.0.2
- Release id：`452705560`
- 名称 / tag：`v0.0.2`
- published_at：2026-11-20 21:06:47（由 UTC `2026-11-20T13:06:47Z` 换算）

`Build` 阶段在校验六包与清单后上传资产。GitHub Release 最终共 12 个已上传资产（6 个 zip + 清单/SHA256），全部 `state=uploaded`，作者为 `github-actions[bot]`：

| 资产 | size（字节） | SHA-256（Release digest） | URL |
| --- | --- | --- | --- |
| `protenix_linux_amd64_release_v0.0.2.zip` | 67983333 | `5dda83eef5b45a93a960145bc8170981be2cf327e59f7f3d2fcbb7051318778b` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/protenix_linux_amd64_release_v0.0.2.zip |
| `protenix_linux_arm64_release_v0.0.2.zip` | 67983333 | `94cfc54cb436ebda6fd9f1b092b851bd737e4925978600ffd5cefd979630edad` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/protenix_linux_arm64_release_v0.0.2.zip |
| `pxdesign_linux_amd64_release_v0.0.2.zip` | 16626302 | `570ee381fb4547ddaa63f98ba4f39f084e0f2b23f320dc6ac3fb7ad62000508d` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/pxdesign_linux_amd64_release_v0.0.2.zip |
| `pxdesign_linux_arm64_release_v0.0.2.zip` | 16626302 | `08f74b61ea6fb8b83fcc69f15028fe512fb01cb44607fdd1d24b8317bb87a333` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/pxdesign_linux_arm64_release_v0.0.2.zip |
| `protenix_service_linux_amd64_release_v0.0.2.zip` | 97669994 | `d61456458d5b7aadf852570b13adf269addb9ef8c20e613541a1e7e97f0bbc44` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/protenix_service_linux_amd64_release_v0.0.2.zip |
| `protenix_service_linux_arm64_release_v0.0.2.zip` | 96278262 | `0d1dafb03e2e61811ed70cf9266b77c4453a07b50c7afc339812d76735705a7e` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/protenix_service_linux_arm64_release_v0.0.2.zip |
| `manifest-linux_amd64.json` | 716 | `611dfad3b627e73e117254a4d7b2ccc3e78bc8bb5e50376a354a8ab206cac900` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/manifest-linux_amd64.json |
| `manifest-linux_arm64.json` | 716 | `a0c9a436cb2b31dd52d9183e1dc3fb9a550e8aff683487d711869f7a169af734` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/manifest-linux_arm64.json |
| `release-manifest.json` | 1232 | `8e05671993565750075ec3c61e254b9d4eea7e419c954ce96b39ebb0ee2634ef` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/release-manifest.json |
| `SHA256SUMS` | 1110 | `1d96d613c349af6653396d05033d0261a921b0048aead97e8cb5bedbb16d799a` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/SHA256SUMS |
| `SHA256SUMS-linux_amd64.txt` | 418 | `25609d4d71141ba55c31bf7448b02df82e0461cf78f4a9faeb25c17843a345ea` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/SHA256SUMS-linux_amd64.txt |
| `SHA256SUMS-linux_arm64.txt` | 418 | `db2776d2a0082c888051665046f572b0f0bb96f802bc0f5c67ffaf65c240232a` | https://github.com/0xdevelop/protenix_build/releases/download/v0.0.2/SHA256SUMS-linux_arm64.txt |

`RemoveOldRelease` 按模板计算 version-2，日志原文：

```
Old version to remove: v0.0.0
gh release delete v0.0.0 -y
release not found
##[error]Process completed with exit code 1
```

这是首次发版、仓库中不存在 `v0.0.0` Release 时的模板行为，发生在六包与清单已经上传到公开 Release 之后。未因此手搓半套 Release，未用本地包冒充 GitHub 资产，也未无依据再次 bump / 重跑 `git_tag.sh`。该失败使 workflow 整体为红色，但 `v0.0.2` 公开 Release 与 12 个资产仍然完整保留。

官方整合包内 `BUILD_INFO.txt`（从已校验的 arm64 zip 读取，未改文件）记录：`version=v0.0.2`，`goos=linux`，`goarch=arm64`，`install_dir=/protenix_service`，`commit=1e3bb50df7b19b5b7f40cce440739b29de487ea6`，`dirty=0`，`build_time=2026-11-20_13:07:26`。打包单元名为 `protenix_service.service`，`User=root`，`ExecStart=/protenix_service/protenix_service`，`WorkingDirectory=/protenix_service`。打包配置中 `storage_cfg.primary` 为 sqlite，MySQL/Redis 未启用；`cli_cfg.protenix.version_runtimes.v2.binary` 为空。

## 4. 从官方 Release 下载 arm64 资产

未使用先前本地 `build/upload_tmp_dir/` 或其他本地打包目录作为安装源。下载目录：

`/home/certa/workspace/protenix_build/tmp/official-release-v0.0.2`

21:11:50–21:16:41 用 `curl -fL` 从上述官方 URL 下载，并以官方 `SHA256SUMS` 校验（21:16:42 全部成功）：

- `SHA256SUMS-linux_arm64.txt`
- `manifest-linux_arm64.json`
- `release-manifest.json`
- `protenix_linux_arm64_release_v0.0.2.zip`
- `pxdesign_linux_arm64_release_v0.0.2.zip`
- `protenix_service_linux_arm64_release_v0.0.2.zip`

本地文件大小与 GitHub Release 公布的 size 一致。`sha256sum -c` 对上述已下载条目均报告成功。

## 5. 正式安装（未执行）——硬阻塞：需要用户本人提供 sudo 密码

已研读发布包中的 `install_protenix_service.sh`：菜单选项 1 会把服务与两个运行时装到脚本指定的 `/protenix_service`，并安装/启动 systemd 系统单元 `protenix_service.service`。机器上当时尚不存在 `/protenix_service` 和 `/etc/systemd/system/protenix_service.service`。`uv` 位于 `/home/certa/.local/bin/uv`（0.13.7）。当前用户 `certa` 在 `sudo` 组。

未开始解压安装，也未改用 `~/.local/share` 或其他前缀，因为正式安装需要 root 写 `/protenix_service` 与 `/etc/systemd/system`。

21:17:35 在本任务使用的 agent shell 中：

```
NoNewPrivs: 1
sudo -n true
sudo: The "no new privileges" flag is set, which prevents sudo from running as root.
```

21:18:49 改用无 NoNewPrivs 的用户 systemd 会话复核（`systemd-run --user`，`NNP=0`）：

```
sudo -n true
sudo: 需要密码
sudo_n_rc=1
sudo -n whoami
sudo: 需要密码
whoami_rc=1
```

因此阻塞不是“猜安装路径”，而是当前用户执行 `sudo` 需要密码，而本任务无法取得该密码。按约定在此停止，没有改用非官方前缀绕过。

## 6. 业务验收（未执行）

因为第 5 步没有完成正式安装与 systemd 启动，没有提交官方 Protenix v1 或 PXDesign preview 任务，也就没有新的 task ID、产物下载或产物哈希。此前其他目录上的切片不能替代本次正式安装后的业务证据。

## 7. 官方卸载（未执行）

因为没有执行第 5 步的官方安装，没有运行 `install_protenix_service.sh` 菜单选项 3，也没有卸载后的单元/目录清理结果。检查时 `/protenix_service` 与 `/etc/systemd/system/protenix_service.service` 本来就不存在。

## 未覆盖项

- `/protenix_service` 正式安装、`protenix_service.service` 启用与常驻启动。
- 正式安装后的 Protenix v1 与 PXDesign preview 真实任务、产物下载与哈希。
- 官方脚本卸载及卸载后复核。
- 首次发版时 `git_tag.sh` 删除远端尚不存在的 `latest` 会失败；本次是事后补推，没有改脚本。
- 首次发版时 `RemoveOldRelease` 删除不存在的 `v0.0.0` 使 workflow 整体失败；未为这一点单独 bump 版本或改模板契约。
- 科学质量不作宣称。

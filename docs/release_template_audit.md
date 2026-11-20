# 发版链路模板审计与恢复（2026-11-20，Asia/Shanghai，UTC+8）

结论：**此前不能声称“复制 project_template_go 后只做小幅适配”。** 根 `git_tag.sh` 在 a08d91d 只放入禁止发版的占位脚本；6542f57 / 345fe16 又改成了另一套入口和工作流顺序。本次已在本地按模板底稿恢复无参默认发版与三段职责，并保留有依据的双架构/三产品适配。本会话没有执行 `git_tag.sh` 无参路径，也没有 push、打 tag、创建或删除 release。

## 1. 现场与来源

- 工作面：spark-901f（`61b34569-fa83-425a-af44-0bbe47070ddf`），仓库 `/home/certa/workspace/protenix_build`。用户消息中的 `9B5CEB2F-7789-4451-ADC9-5293C5184F60` 不在已注册机器列表中，与 spark 不是同一台已登记主机，未在其上操作或克隆。
- 修复前 HEAD 与 origin/main：`345fe16a1adb616761bced3a81f38323656e864f`；工作区干净；本地和远端均无 `v0.0.1` tag；此前获准的一次 publish **实际运行 0 次**。
- 模板仓库当前 HEAD：`09058578159c03fdfbf4310cb5149364b4e39e17`。已核对 `git_tag.sh`、`build.sh`、`.github/workflows/release.yml` 三个 blob 与历史 `ce470b1b6bcc15735bc9e7d067c2d0f26bedbf98` 相同。
- 模板 `git_tag.sh` 与实例 `protenix_service/git_tag.sh` 的 blob 同为 `fa69798f6c01d3675d2ac09614a62e52866145e8`，说明服务目录一直带着完整模板入口；偏离发生在**仓库根入口和工作流**，不是模板文件缺失。

## 2. 逐项对照（修复前）

| 项 | 模板实际行为 | 6542f57 / 345fe16 的根入口与工作流 | 判定 |
| --- | --- | --- | --- |
| 默认入口 | 无参（或未真正走到的 `auto`/`custom` 分支）即进入发版 | 默认 `--check`，必须 `--publish vX.Y.Z` | 未经授权的接口重设计 |
| 版本选择 | 读取当前版本并自动 +1（minor，满 1000 进位） | 不 bump，要求调用方给出已有版本 | 偏离 |
| changelog / API 文档 / 版本提交 | 写入 changelog、生成 API 文档、提交 `Release:--:` | 无 | 偏离 |
| tag 形态 | 轻量 tag `git tag vX.Y.Z` | annotated `git tag -a` | 偏离 |
| remotes | `for remote in $(git remote)` 逐个推送 | 只推 `origin` 的一个 tag refspec | 偏离 |
| latest / 旧 tag | `git tag -f latest`，先 `git push --delete ${remote} latest` 再推 latest；本地 `git tag -d` 上一版本号 | 明确取消这些行为 | 偏离。模板确实包含这些写操作；安全约束禁止在本次任务里机械执行或擅自改成禁用。脚本已按模板恢复，**本次不运行发版，因此这些远端写操作不会发生**。 |
| 工作流三段 | `release` → `Build` → `RemoveOldRelease` | `Build` → `Aggregate` → `release`（草稿校验后再公开），去掉第三段 | 偏离 |
| release 创建时机 | 先按 LATEST_TAG 创建 GitHub release（含 `mathieudutour/github-tag-action` 与 `ncipollo/release-action`） | 构建完成后再 `gh release create --draft` | 偏离 |
| 旧 release | 第三段删除 **version-2** 的 GitHub release 及其 tag | 占位/删除被去掉 | 偏离；同上，脚本已恢复，本次不执行 |
| `workflow_dispatch` | 与 tag push 走同一三段，包括创建 release 和清理旧版本 | 被改成“只构建、绝不发布” | 偏离 |
| `build.sh` 职责 | 只构建/打包 | 根 `build.sh` 在 6542f57/345fe16 **未被改动** | 保持 |
| 三产品打包与架构化文件名 | 模板是单二进制包；本仓库既有三包命名 | 既有业务，不是这次重设计引入 | 必要差异，保留 |
| 原生双架构 / CGo / 六包门禁 / Go 1.26.10 | 模板单 runner `ubuntu-latest` + 默认 `build.sh` | 这些校验本身有产品依据，但被放进了重排后的工作流 | 保留校验，放回三段结构 |

另外两项模板既有语义，恢复时不加“修正”：

- `handle_input` 在参数为 `auto` 时仍调用 `to_run "auto"`，会落到 “Invalid argument normal”；`custom` 交互分支写在 `to_run` 里，但 `handle_input` 不会把 `custom` 传进去。无参路径是实际发版入口。
- 模板的 `git push ${remote}` 依赖当前分支已设置 upstream；隔离测试必须先 `-u` 才能复现成功推送。第一次从 `v0.0.1` 发到 `v0.0.2` 时，若本地不存在将被删除的上一版本 tag，末尾的 `git tag -d` 可能因 `set -e` 失败——此时若前面的 push 已成功，需要人工检查，而不是在这次任务里改模板语义。

## 3. 已恢复项

根 `git_tag.sh` 以模板文件为底稿，只改了本仓库布局所需路径：

- 版本读取改为 `go run gen_release_assets.go version`，保证三份预构建 YAML 中的 `project.version` 为合法字符串且一致。
- `git_handle_ready` 仍按模板 `sed` 更新 `protenix_service/config/config.go` 的 `ProjectVersion`，并同步 `pre_build_config_template.yaml`，再跑既有 `gen_pre_build_config.sh all`，最后用 `version --tag` 复核。
- API 文档改为调用 `protenix_service/gen_api_docs.sh`；`git add` 增加两份生成 YAML、模板 YAML 和 `protenix_service/docs/api_methods.md`。
- 无参自动递增、轻量 tag、`latest` 强制移动、遍历所有 remotes、删除本地上一版本 tag、`Release:--:` 提交说明，均保持模板原样。

`.github/workflows/release.yml` 恢复三段名称与顺序：

1. `release`：与模板相同，读取 `LATEST_TAG`，使用同一组 release action 和 `changelog/${{ env.LATEST_TAG }}.md`。
2. `Build`：仍是模板的上传阶段；其前增加必要的 `build_native` 矩阵（不属于新的对外发版入口），原生构建后再做六包聚合校验，然后用模板的 `upload_github_release_assets_action` 上传。
3. `RemoveOldRelease`：按模板计算并删除 version-2 的 release 与 tag。

`workflow_dispatch` 再次与模板一样走完整三段（包括创建 release 和清理旧版本）。若只需要无发布的云构建，不能再把 dispatch 当成安全的“只构建”入口。

## 4. 仍保留的必要差异及理由

- **三产品与架构化文件名**：独立 Protenix、独立 PXDesign、整合 `protenix_service`，每个 arch 三个 zip。这是本仓库既有交付，不是模板单包能覆盖的。
- **原生矩阵**：`amd64` 用 `ubuntu-24.04`，`arm64` 用 `ubuntu-24.04-arm`。服务二进制依赖 CGo/SQLite，不能只用 `GOARCH` 在单一 `ubuntu-latest` 上冒充原生运行时构建。
- **不跑默认 `./build.sh` / `build.sh test`**：根 `build.sh` 默认 `all` 会安装仓库内 prefix（pip/权重/GPU 开发环境）。Actions 只跑 `FORCE_PACK=1 ./build.sh pack release linux_<arch>`。
- **Go 工具链**：`actions/setup-go` 读取 `protenix_service/go.mod`（当前 1.26.10），不是模板的 `>=1.25.0`。
- **子模块与完整性门禁**：checkout 需要 `submodules: recursive`；两架构都成功后才校验六包、BUILD_INFO、服务 ELF/CGo 与 SHA-256，再上传到已经由第一段创建的 release。不会在单个 arch job 里直接往 GitHub release 丢半套包。若 `release` 已创建而 `Build` 失败，可能留下没有完整附件的 release——这与模板“先建 release 再上传”是同一类风险，本次未改这个顺序。
- **上传目录**：在模板的 zip 上传之外，附带架构 manifest 与 SHA256SUMS，作为完整性记录。

根 `build.sh`、`build_srcs`、`project_template_go` 和受保护的 `config_local.yaml` 均未修改。

## 5. 操作说明（本次未执行发版）

真实发版入口与模板相同：

```bash
./git_tag.sh
```

不要使用已撤销的 `--publish` / `--check`。查询将删除的上一版本号（只读）：

```bash
./git_tag.sh -get_pre_del_tag_name
```

无参路径会递增版本、改文件、提交、打轻量 tag、按模板移动 latest、向全部 remote 推送 main/tag，并可能触发 Actions 三段（含删除 version-2）。只有在单独获得发版授权、且已理解上述远端写操作时才能运行。本次修复提交不推送，也不把这次恢复混入先前已推送的 `345fe16` 去发版。

## 6. 检查与证据

- `go run gen_release_assets.go self-test`：51 项 fixture/静态检查通过；其中包含工作流三段结构、原生矩阵、六包门禁字符串，以及在隔离 git 仓库中真实跑通一次无参 `git_tag.sh`（轻量 tag、latest 移动、版本文件/changelog/API 文档、向 **fixture remote** 推送）。该隔离运行不是产品仓库的发版。
- 产品仓库只读探测：`./git_tag.sh --publish v0.0.1` 输出 `Invalid argument`；`./git_tag.sh -get_pre_del_tag_name` 输出 `Pre Del Tag With  0.0.0`；HEAD 仍为 `345fe16`，未出现 `v0.0.1` tag。
- 原始日志：`tmp/release-template-audit-20261120/self-test.log`（ignored、可删）。

本次没有真实 Actions 双 runner 运行证据，不能宣称云端六包发版已验证。

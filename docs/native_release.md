# 原生双架构打包与发版操作

发版入口和工作流职责已按 project_template_go 的三段链路恢复，并保留本仓库必要的双架构/三产品适配。对照、越界项、已恢复项和模板自带的 latest/旧 release 清理语义见[模板审计与恢复说明](release_template_audit.md)。不要再使用 `--publish` / `--check`。本页只保留仍有效的打包与门禁边界。

## 原生矩阵与依赖边界

| matrix.arch | runs-on | 本 job 目标 |
| --- | --- | --- |
| amd64 | ubuntu-24.04 | linux_amd64 三包 |
| arm64 | ubuntu-24.04-arm | linux_arm64 三包 |

`check_native_build.sh` 同时核对 Linux/uname、Go host 与 target、CGo 开启、本机 gcc 的 GNU target；不是只改 GOARCH。服务构建入口仍强制 CGO_ENABLED=1（SQLite），门禁还检查 zip 内服务 ELF machine 与 Go build info 的 GOOS/GOARCH/CGO_ENABLED。

CI 使用 `protenix_service/go.mod`（当前要求 1.26.10）。打包依赖还包括原生 gcc/build-essential、pkg-config、zip/unzip、bun。Actions 只执行 `FORCE_PACK=1 ./build.sh pack release linux_<arch>`，不跑默认 `all`，避免把打包变成 prefix 安装或 GPU 任务。成功打包不等于目标机器已完成依赖安装或 GPU 推理。

## 六包与清单门禁

每架构固定三个文件（版本与预构建配置一致）：

```text
protenix_linux_<arch>_release_vX.Y.Z.zip
pxdesign_linux_<arch>_release_vX.Y.Z.zip
protenix_service_linux_<arch>_release_vX.Y.Z.zip
```

各原生 job 只上传自己的三个 zip 加本架构 manifest/checksum；`Build` 在两架构都成功后要求完整六包、当前提交、dirty=0、嵌套 runtime BUILD_INFO、ELF/CGo 和 SHA-256，再按模板上传到第一段已创建的 GitHub release。缺包或半套输入会被拒绝。

## 本地检查

```bash
go run gen_release_assets.go self-test
bash scripts/check_native_build.sh arm64  # amd64 原生机器使用 amd64
bash -n git_tag.sh build.sh scripts/check_native_build.sh
```

self-test 是 fixture/静态检查，另含隔离仓库中的无参 `git_tag.sh` 契约测试，不是真实产品发版，也不是 Actions 双 runner 实证。GPU/HTTP 成品验收见 [本轮产物验收](acceptance_product_matrix_20261119.md)。

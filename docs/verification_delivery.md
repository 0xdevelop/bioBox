# 验证码 INFO 日志与独立邮件能力

Go 服务 `protenix_service` 与模板采用同一实现：每次真实生码即写服务 INFO（所有运行模式），不设日志开关、不与邮件互斥。日志不表示邮件已发或验证码有效；验证码不能进入 API、聊天、截图或测试输出。

- 在实际安装的服务配置 `auth_cfg.email` 中显式写 `enabled: false` 关闭邮件，或 `true` 启用。不要编辑或展示仓库内的 `example_files/config_local.yaml`。新示例已明确关闭邮件；旧块无开关保留邮件意图，无块不发送。
- 邮件开启必须具备准确的供应商枚举、合法收发件人与凭证等依赖；配置加载与最终发送前分别校验。失败明确返回，不因 INFO 中有码宣称成功。成功响应 `sent=false` 表示未启用邮件，`sent=true` 表示供应商确认受理，不保证收件箱送达。
- 冷却统一使用 `verification.send_interval_seconds`；旧 `resend_interval_seconds` 只作迁移别名，同时存在必须相等。供应商只是 `email.provider` 的字符串枚举值，不使用供应商命名能力块。
- 运维只能从受权服务日志用 `delivery_id` 关联排查，须限制日志权限/保留周期；失败/回滚日志中的码不一定可用。邮件失败不释放频控预算，也不激活该码；清理失败另行告警，未激活门控仍拒绝消费。

完整配置片段、严格类型与错误分支、存储及单次消费语义、可重复的隔离 HTTP/SQLite 验证入口见 [服务操作说明](../protenix_service/docs/verification_delivery.md)。现有账户契约见 [ability_auth.md](../protenix_service/docs/ability_auth.md)，协议结构见 [api_methods.md](../protenix_service/docs/api_methods.md)。本次供应商测试使用本地替身，不等于真实发信验收，不涉及重装或 GPU 推理。

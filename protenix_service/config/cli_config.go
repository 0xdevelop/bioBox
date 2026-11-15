package config

// CLIConfig 是原生 CLI 接线的结构化配置。版本/模型/preset 仍走方法 arguments，不进路径。
type CLIConfig struct {
	WorkRoot              string           `yaml:"work_root" json:"work_root" toml:"work_root" comment:"每任务作业目录的根路径；输入会拷贝到子目录，不改原文件"`
	CancelGraceSeconds    int              `yaml:"cancel_grace_seconds" json:"cancel_grace_seconds" toml:"cancel_grace_seconds" comment:"向进程组发送 SIGTERM 后、再发 SIGKILL 的等待秒数"`
	ExecuteTimeoutSeconds int              `yaml:"execute_timeout_seconds" json:"execute_timeout_seconds" toml:"execute_timeout_seconds" comment:"单次 Execute 最长秒数；0 表示不限制。到期后取消进程组，任务记 system_error"`
	Protenix              *CLIBinaryConfig `yaml:"protenix" json:"protenix" toml:"protenix" comment:"protenix-run 可执行文件；路径为空时 Execute 拒绝启动进程"`
	PXDesign              *CLIBinaryConfig `yaml:"pxdesign" json:"pxdesign" toml:"pxdesign" comment:"pxdesign-run 可执行文件；路径为空时 Execute 拒绝启动进程"`
}

// CLIBinaryConfig 指向已安装的官方包装命令，不在代码里写死路径。
type CLIBinaryConfig struct {
	Binary          string                       `yaml:"binary" json:"binary" toml:"binary"`
	ExtraEnv        map[string]string            `yaml:"extra_env" json:"extra_env" toml:"extra_env" comment:"额外环境变量，例如 LAYERNORM_TYPE=torch"`
	VersionRuntimes map[string]*CLIRuntimeConfig `yaml:"version_runtimes" json:"version_runtimes" toml:"version_runtimes" comment:"按 arguments.version 选择独立 runtime。v1 未单独配置时回退到上级 binary；v2 必须单独配置，不会使用 v1 的 binary。version 不进 CLI 参数或 URL"`
}

// CLIRuntimeConfig 是某一产品版本对应的独立 runtime。
type CLIRuntimeConfig struct {
	Binary   string            `yaml:"binary" json:"binary" toml:"binary" comment:"该版本已安装包装命令的路径；空字符串表示该版本未配置"`
	ExtraEnv map[string]string `yaml:"extra_env" json:"extra_env" toml:"extra_env" comment:"该版本的额外环境变量；省略或 null 时沿用上级 extra_env，显式空映射表示不附加额外变量"`
}

func DefaultCLIConfig() *CLIConfig {
	return &CLIConfig{
		WorkRoot:              "./data/cli_jobs",
		CancelGraceSeconds:    10,
		ExecuteTimeoutSeconds: 0,
		Protenix:              &CLIBinaryConfig{ExtraEnv: map[string]string{}, VersionRuntimes: map[string]*CLIRuntimeConfig{}},
		PXDesign:              &CLIBinaryConfig{ExtraEnv: map[string]string{}},
	}
}

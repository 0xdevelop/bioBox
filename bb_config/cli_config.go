package bb_config

import (
	"fmt"
	"time"

	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
)

// CLIConfig 是原生 CLI 接线的结构化配置。版本/模型/preset 仍走方法 arguments，不进路径。
type CLIConfig struct {
	WorkRoot           string `yaml:"work_root" json:"work_root" toml:"work_root" comment:"每任务作业目录的根路径；输入会拷贝到子目录，不改原文件"`
	CancelGraceSeconds int    `yaml:"cancel_grace_seconds" json:"cancel_grace_seconds" toml:"cancel_grace_seconds" comment:"向进程组发送 SIGTERM 后、再发 SIGKILL 的等待秒数"`
	// ExecuteTimeoutEnabled 是单次 Execute 超时开关，必须显式写 true/false，默认 false；false 时忽略 execute_timeout_seconds。
	ExecuteTimeoutEnabled bool `yaml:"execute_timeout_enabled" json:"execute_timeout_enabled" toml:"execute_timeout_enabled" comment:"是否启用单次 Execute 超时；必须是 true/false。false 时忽略 execute_timeout_seconds，不限制单次 Execute 时长"`
	// ExecuteTimeoutSeconds 是单次 Execute 最长秒数，从本次认领开始执行起算；仅在 ExecuteTimeoutEnabled=true 时生效，且必须为正整数。
	ExecuteTimeoutSeconds int              `yaml:"execute_timeout_seconds" json:"execute_timeout_seconds" toml:"execute_timeout_seconds" comment:"单次 Execute 最长秒数（从本次开始执行起算）；仅在 execute_timeout_enabled=true 时生效且必须为正整数。到期后取消进程组，任务记 system_error"`
	Protenix              *CLIBinaryConfig `yaml:"protenix" json:"protenix" toml:"protenix" comment:"本套件自己的命令，与 pxdesign 无关。host 为空则本机 exec；非空则用本机 ssh 在那台机器上跑。路径为空只拒绝本套件"`
	PXDesign              *CLIBinaryConfig `yaml:"pxdesign" json:"pxdesign" toml:"pxdesign" comment:"本套件自己的命令，与 protenix 无关。host 为空则本机 exec；非空则用本机 ssh 在那台机器上跑。路径为空只拒绝本套件"`
}

// CLIBinaryConfig 是一套计算自己的命令。host 为空时在本机执行 binary；非空时 binary 是那台机器上的路径。
type CLIBinaryConfig struct {
	Resources       *bb_resource.Config          `yaml:"resources" json:"resources" toml:"resources"`
	Binary          string                       `yaml:"binary" json:"binary" toml:"binary" comment:"要启动的命令路径。host 为空时在本机 exec；host 非空时这是那台机器上的路径。两条互不要求相同。权重与运行环境由服务按该组件阈值先检查，通过后才 exec 或 ssh。空字符串表示未配置"`
	Host            string                       `yaml:"host" json:"host" toml:"host" comment:"可选。空则本机 exec binary。非空是本机 ssh 的目标（user@host 或 ssh config 里的 Host），只影响这一套。端口和密钥用本机 ssh 配置，不在这里写。另一套的 host 互不影响"`
	ExtraEnv        map[string]string            `yaml:"extra_env" json:"extra_env" toml:"extra_env" comment:"额外环境变量，例如 LAYERNORM_TYPE=torch"`
	VersionRuntimes map[string]*CLIRuntimeConfig `yaml:"version_runtimes" json:"version_runtimes" toml:"version_runtimes" comment:"按 arguments.version 选择独立 runtime。v1 未单独配置时回退到上级 binary；v2 必须单独配置，不会使用 v1 的 binary。version 不进 CLI 参数或 URL"`
}

// CLIRuntimeConfig 是某一产品版本对应的独立 runtime。
type CLIRuntimeConfig struct {
	Resources *bb_resource.Config `yaml:"resources" json:"resources" toml:"resources"`
	Binary    string              `yaml:"binary" json:"binary" toml:"binary" comment:"该版本已安装包装命令的路径；空字符串表示该版本未配置"`
	ExtraEnv  map[string]string   `yaml:"extra_env" json:"extra_env" toml:"extra_env" comment:"该版本的额外环境变量；省略或 null 时沿用上级 extra_env，显式空映射表示不附加额外变量"`
}

// Validate 校验 cli_cfg。execute_timeout_enabled=true 时 execute_timeout_seconds 必须为正整数；关闭时忽略 seconds。
func (c *CLIConfig) Validate() error {
	if c == nil {
		return nil
	}
	if c.ExecuteTimeoutEnabled && c.ExecuteTimeoutSeconds <= 0 {
		return fmt.Errorf("cli_cfg.execute_timeout_seconds must be a positive integer (seconds) when cli_cfg.execute_timeout_enabled=true, got %d", c.ExecuteTimeoutSeconds)
	}
	return nil
}

// CurrentExecuteTimeout 返回单次 Execute 超时；仅开关为 true 且 seconds 为正时返回正值，否则返回 0（不施加期限）。
func (c *CLIConfig) CurrentExecuteTimeout() time.Duration {
	if c == nil || !c.ExecuteTimeoutEnabled || c.ExecuteTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(c.ExecuteTimeoutSeconds) * time.Second
}

func DefaultCLIConfig() *CLIConfig {
	return &CLIConfig{
		WorkRoot:              "./data/cli_jobs",
		CancelGraceSeconds:    10,
		ExecuteTimeoutEnabled: false,
		ExecuteTimeoutSeconds: 3600,
		Protenix:              &CLIBinaryConfig{ExtraEnv: map[string]string{}, VersionRuntimes: map[string]*CLIRuntimeConfig{}},
		PXDesign:              &CLIBinaryConfig{ExtraEnv: map[string]string{}},
	}
}

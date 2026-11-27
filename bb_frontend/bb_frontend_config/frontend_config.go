package bb_frontend_config

import (
	"errors"
	"strings"
)

type FrontendConfig struct {
	Enabled     *bool  `yaml:"enabled" json:"enabled" toml:"enabled" comment:"是否提供前端静态页；缺省开；显式 true/false 覆盖缺省"`
	BindAddress string `yaml:"bind_address" json:"bind_address" toml:"bind_address" comment:"监听地址（启用时必填）"`
	Port        int    `yaml:"port" json:"port" toml:"port" comment:"监听端口（启用时必填，1-65535）"`
}

// FrontendPageEnabled 报告是否提供前端静态页。cfg 或 enabled 缺席时缺省为开；显式 false 为关。
func FrontendPageEnabled(cfg *FrontendConfig) bool {
	if cfg != nil && cfg.Enabled != nil {
		return *cfg.Enabled
	}
	return true
}

// Validate 只在显式 enabled 为 true 时检查监听地址和端口。缺字段不报错。
func (cfg *FrontendConfig) Validate() error {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.BindAddress) == "" {
		return errors.New("frontend_cfg.bind_address is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return errors.New("frontend_cfg.port must be between 1 and 65535")
	}
	return nil
}

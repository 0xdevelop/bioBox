// Package bb_ability_adapter_protenix 提供 Protenix 这一条的前置检查参数。
// v1/v2 是同一套检查里的版本阈值，不是两套检查。
package bb_ability_adapter_protenix

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/0xdevelop/bioBox/bb_cli_proc"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_runtime_resource"
)

var errCLINotConfigured = errors.New("protenix CLI binary is not configured")
var errV2CLINotConfigured = errors.New("protenix v2 CLI binary is not configured")

// Load 由 bb_ability 父包显式调用。本包不注册 API 方法。
func Load() {}

// Runtime 是这一次调用要核对的 Protenix 版本阈值。
type Runtime struct {
	Version   string
	SetID     string
	Binary    string
	Host      string
	ExtraEnv  map[string]string
	Resources *bb_resource.Config
}

// Resolve 按 arguments.version 取出已配置阈值。v1 未单独配置时用上级 binary 与 resources；v2 必须单独配置 binary，不回退 v1。
func Resolve(version string) (*Runtime, error) {
	binary, err := configuredBinary(version)
	if err != nil {
		return nil, err
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return nil, err
	}
	prepared, err := bb_runtime_resource.Compile("protenix", binary.Binary, root, binary.ExtraEnv, binary.Resources)
	if err != nil {
		return nil, err
	}
	if version != "v1" && filepath.Base(binary.Binary) == "protenix-run" {
		return nil, bb_runtime_resource.ErrUnsupported
	}
	return &Runtime{
		Version:   version,
		SetID:     "protenix." + version,
		Binary:    binary.Binary,
		Host:      strings.TrimSpace(binary.Host),
		ExtraEnv:  binary.ExtraEnv,
		Resources: prepared,
	}, nil
}

// Check 用统一前置检查核对这一版阈值。不符合时不表示进程已启动。
func Check(ctx context.Context, rt *Runtime, args []string) error {
	if rt == nil {
		return errCLINotConfigured
	}
	return bb_cli_proc.ConfirmCommandReady(ctx, bb_cli_proc.Spec{
		Name:        "protenix",
		Path:        rt.Binary,
		Args:        args,
		Env:         rt.ExtraEnv,
		Resources:   rt.Resources,
		ResourceSet: rt.SetID,
	})
}

func configuredBinary(version string) (*bb_config.CLIBinaryConfig, error) {
	if bb_config.GlobalConfig == nil || bb_config.GlobalConfig.CliCfg == nil || bb_config.GlobalConfig.CliCfg.Protenix == nil {
		return nil, errCLINotConfigured
	}
	base := bb_config.GlobalConfig.CliCfg.Protenix
	if override := versionRuntime(base, version); override != nil && strings.TrimSpace(override.Binary) != "" {
		resources := base.Resources
		if override.Resources != nil {
			resources = override.Resources
		}
		env := base.ExtraEnv
		if override.ExtraEnv != nil {
			env = override.ExtraEnv
		}
		return &bb_config.CLIBinaryConfig{
			Binary:    strings.TrimSpace(override.Binary),
			Host:      base.Host,
			Resources: resources,
			ExtraEnv:  env,
		}, nil
	}
	if version == "v2" {
		return nil, errV2CLINotConfigured
	}
	if base == nil || strings.TrimSpace(base.Binary) == "" {
		return nil, errCLINotConfigured
	}
	return base, nil
}

func versionRuntime(base *bb_config.CLIBinaryConfig, version string) *bb_config.CLIRuntimeConfig {
	if base == nil || base.VersionRuntimes == nil {
		return nil
	}
	return base.VersionRuntimes[version]
}

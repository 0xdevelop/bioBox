// Package bb_ability_adapter_pxdesign 提供 pxdesign 这一条的前置检查参数。
// 现阶段只有一套阈值，与 Protenix 共用同一套检查。
package bb_ability_adapter_pxdesign

import (
	"context"
	"errors"
	"strings"

	"github.com/0xdevelop/bioBox/bb_cli_proc"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_runtime_resource"
)

var errCLINotConfigured = errors.New("pxdesign CLI binary is not configured")

// Load 由 bb_ability 父包显式调用。本包不注册 API 方法。
func Load() {}

// Runtime 是这一次调用要核对的 pxdesign 阈值。现阶段只有 preview 这一套。
type Runtime struct {
	SetID     string
	Binary    string
	Host      string
	ExtraEnv  map[string]string
	Resources *bb_resource.Config
}

// Resolve 取出 cli_cfg.pxdesign 上已配置的那一套阈值。
func Resolve() (*Runtime, error) {
	binary, err := configuredBinary()
	if err != nil {
		return nil, err
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return nil, err
	}
	prepared, err := bb_runtime_resource.Compile("pxdesign", binary.Binary, root, binary.ExtraEnv, binary.Resources)
	if err != nil {
		return nil, err
	}
	return &Runtime{
		SetID:     "pxdesign.preview",
		Binary:    binary.Binary,
		Host:      strings.TrimSpace(binary.Host),
		ExtraEnv:  binary.ExtraEnv,
		Resources: prepared,
	}, nil
}

// Check 用统一前置检查核对这一套阈值。不符合时不表示进程已启动。
func Check(ctx context.Context, rt *Runtime, args []string) error {
	if rt == nil {
		return errCLINotConfigured
	}
	return bb_cli_proc.ConfirmCommandReady(ctx, bb_cli_proc.Spec{
		Name:        "pxdesign",
		Path:        rt.Binary,
		Args:        args,
		Env:         rt.ExtraEnv,
		Resources:   rt.Resources,
		ResourceSet: rt.SetID,
	})
}

func configuredBinary() (*bb_config.CLIBinaryConfig, error) {
	if bb_config.GlobalConfig == nil || bb_config.GlobalConfig.CliCfg == nil || bb_config.GlobalConfig.CliCfg.PXDesign == nil {
		return nil, errCLINotConfigured
	}
	binary := bb_config.GlobalConfig.CliCfg.PXDesign
	if strings.TrimSpace(binary.Binary) == "" {
		return nil, errCLINotConfigured
	}
	return binary, nil
}

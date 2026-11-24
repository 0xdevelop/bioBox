package config

import (
	"github.com/0xdevelop/bioBox/api/api_auth/api_auth_config"
	"github.com/0xdevelop/bioBox/common/app_path"
	"github.com/0xdevelop/bioBox/policy/policy_config"
	"path/filepath"
	"testing"
)

// 发布包里的 conf/config.yaml 来自 config_example.yaml；cli_cfg 必须指向
// 可迁移的二进制同根布局 runtime/<app>（正式根仍 /bb_service）。
func TestConfigExampleCLIPathsFollowInstallLayout(t *testing.T) {
	old, oldApp := GlobalConfig, CurrentApp
	oldAuth, oldPolicy := api_auth_config.CurrentCfgAuth, policy_config.CurrentCfgPolicy
	CurrentApp = &ExtendApp{}
	t.Cleanup(func() {
		GlobalConfig = old
		CurrentApp = oldApp
		api_auth_config.CurrentCfgAuth = oldAuth
		policy_config.CurrentCfgPolicy = oldPolicy
	})
	root, err := app_path.ExecutableDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := LoadConfig(filepath.Join("..", "example_files", "config_example.yaml")); err != nil {
		t.Fatalf("load config_example.yaml: %v", err)
	}
	cli := GlobalConfig.CliCfg
	if cli == nil || cli.Protenix == nil || cli.PXDesign == nil {
		t.Fatalf("cli_cfg incomplete: %+v", cli)
	}
	if got, want := cli.WorkRoot, filepath.Join(root, "data/cli_jobs"); got != want {
		t.Fatalf("work_root=%q want %q", got, want)
	}
	if got, want := cli.Protenix.Binary, "/bb_service/runtime/protenix/bin/protenix-run"; got != want {
		t.Fatalf("protenix.binary=%q want %q", got, want)
	}
	if got, want := cli.PXDesign.Binary, "/bb_service/runtime/pxdesign/bin/pxdesign-run"; got != want {
		t.Fatalf("pxdesign.binary=%q want %q", got, want)
	}
	v2 := cli.Protenix.VersionRuntimes["v2"]
	if v2 == nil || v2.Binary != "" {
		t.Fatalf("v2 runtime must stay explicitly unconfigured (empty binary), got %+v", v2)
	}
	if cli.ExecuteTimeoutEnabled {
		t.Fatal("execute_timeout_enabled must stay explicit false in the published example")
	}
}

func TestAuthSessionDefaultsPreservePublishedContract(t *testing.T) {
	if ProjectName != "bb_service" {
		t.Fatalf("ProjectName=%q want bb_service", ProjectName)
	}
	defCfg := generateDefaultConfig()
	if defCfg.AuthCfg == nil || defCfg.AuthCfg.Session == nil {
		t.Fatalf("default auth_cfg.session missing: %+v", defCfg.AuthCfg)
	}
	if defCfg.AuthCfg.Session.Issuer != "protenix_service" || defCfg.AuthCfg.Session.Audience != "protenix_service" {
		t.Fatalf("default session issuer/audience = (%q, %q), want (protenix_service, protenix_service)",
			defCfg.AuthCfg.Session.Issuer, defCfg.AuthCfg.Session.Audience)
	}

	old, oldApp := GlobalConfig, CurrentApp
	oldAuth, oldPolicy := api_auth_config.CurrentCfgAuth, policy_config.CurrentCfgPolicy
	CurrentApp = &ExtendApp{}
	t.Cleanup(func() {
		GlobalConfig = old
		CurrentApp = oldApp
		api_auth_config.CurrentCfgAuth = oldAuth
		policy_config.CurrentCfgPolicy = oldPolicy
	})
	if err := LoadConfig(filepath.Join("..", "example_files", "config_example.yaml")); err != nil {
		t.Fatalf("load config_example.yaml: %v", err)
	}
	if GlobalConfig.AuthCfg == nil || GlobalConfig.AuthCfg.Session == nil {
		t.Fatalf("example auth_cfg.session missing: %+v", GlobalConfig.AuthCfg)
	}
	if GlobalConfig.AuthCfg.Session.Issuer != "protenix_service" || GlobalConfig.AuthCfg.Session.Audience != "protenix_service" {
		t.Fatalf("example session issuer/audience = (%q, %q), want (protenix_service, protenix_service)",
			GlobalConfig.AuthCfg.Session.Issuer, GlobalConfig.AuthCfg.Session.Audience)
	}
}

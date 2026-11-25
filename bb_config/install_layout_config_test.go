package bb_config

import (
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_policy/bb_policy_config"
	"path/filepath"
	"testing"
)

// 发布包里的 conf/config.yaml 来自 config_example.yaml；cli_cfg 必须指向
// 可迁移的二进制同根布局 runtime/<app>（正式根为 /bioBox）。
func TestConfigExampleCLIPathsFollowInstallLayout(t *testing.T) {
	old, oldApp := GlobalConfig, CurrentApp
	oldAuth, oldPolicy := bb_api_auth_config.CurrentCfgAuth, bb_policy_config.CurrentCfgPolicy
	CurrentApp = &ExtendApp{}
	t.Cleanup(func() {
		GlobalConfig = old
		CurrentApp = oldApp
		bb_api_auth_config.CurrentCfgAuth = oldAuth
		bb_policy_config.CurrentCfgPolicy = oldPolicy
	})
	root, err := bb_app_path.ExecutableDir()
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
	if got, want := cli.Protenix.Binary, "/bioBox/runtime/protenix/bin/protenix-run"; got != want {
		t.Fatalf("protenix.binary=%q want %q", got, want)
	}
	if got, want := cli.PXDesign.Binary, "/bioBox/runtime/pxdesign/bin/pxdesign-run"; got != want {
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
	if ProjectName != "bioBox" {
		t.Fatalf("ProjectName=%q want bioBox", ProjectName)
	}
	if ProjectBundleID != "com.bioBox.bioBox" {
		t.Fatalf("ProjectBundleID=%q want com.bioBox.bioBox", ProjectBundleID)
	}
	if ProjectVersion != "v0.0.4" {
		t.Fatalf("ProjectVersion=%q want v0.0.4", ProjectVersion)
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
	oldAuth, oldPolicy := bb_api_auth_config.CurrentCfgAuth, bb_policy_config.CurrentCfgPolicy
	CurrentApp = &ExtendApp{}
	t.Cleanup(func() {
		GlobalConfig = old
		CurrentApp = oldApp
		bb_api_auth_config.CurrentCfgAuth = oldAuth
		bb_policy_config.CurrentCfgPolicy = oldPolicy
	})
	if err := LoadConfig(filepath.Join("..", "example_files", "config_example.yaml")); err != nil {
		t.Fatalf("load config_example.yaml: %v", err)
	}
	if defCfg.StorageCfg == nil || defCfg.StorageCfg.Sqlite == nil || defCfg.StorageCfg.Sqlite.DBFilePath != "./data/bioBox.db" {
		t.Fatalf("default sqlite path=%+v want ./data/bioBox.db", defCfg.StorageCfg)
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		t.Fatal(err)
	}
	if GlobalConfig.StorageCfg == nil || GlobalConfig.StorageCfg.Sqlite == nil || GlobalConfig.StorageCfg.Sqlite.DBFilePath != filepath.Join(root, "data/bioBox.db") {
		t.Fatalf("example sqlite path=%+v want %q", GlobalConfig.StorageCfg, filepath.Join(root, "data/bioBox.db"))
	}
	if GlobalConfig.AuthCfg == nil || GlobalConfig.AuthCfg.Session == nil {
		t.Fatalf("example auth_cfg.session missing: %+v", GlobalConfig.AuthCfg)
	}
	if GlobalConfig.AuthCfg.Session.Issuer != "protenix_service" || GlobalConfig.AuthCfg.Session.Audience != "protenix_service" {
		t.Fatalf("example session issuer/audience = (%q, %q), want (protenix_service, protenix_service)",
			GlobalConfig.AuthCfg.Session.Issuer, GlobalConfig.AuthCfg.Session.Audience)
	}
}

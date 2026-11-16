package config

import (
	"path/filepath"
	"testing"
)

// 发布包里的 conf/config.yaml 来自 config_example.yaml；cli_cfg 必须指向
// install_protenix_service.sh 安装的同根布局 /protenix_service/runtime/<app>。
func TestConfigExampleCLIPathsFollowInstallLayout(t *testing.T) {
	GlobalConfig = nil
	t.Cleanup(func() { GlobalConfig = nil })
	if err := LoadConfig(filepath.Join("..", "example_files", "config_example.yaml")); err != nil {
		t.Fatalf("load config_example.yaml: %v", err)
	}
	cli := GlobalConfig.CliCfg
	if cli == nil || cli.Protenix == nil || cli.PXDesign == nil {
		t.Fatalf("cli_cfg incomplete: %+v", cli)
	}
	if got, want := cli.WorkRoot, "/protenix_service/data/cli_jobs"; got != want {
		t.Fatalf("work_root=%q want %q", got, want)
	}
	if got, want := cli.Protenix.Binary, "/protenix_service/runtime/protenix/bin/protenix-run"; got != want {
		t.Fatalf("protenix.binary=%q want %q", got, want)
	}
	if got, want := cli.PXDesign.Binary, "/protenix_service/runtime/pxdesign/bin/pxdesign-run"; got != want {
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

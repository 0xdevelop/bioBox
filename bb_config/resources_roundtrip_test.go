package bb_config

import (
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_policy/bb_policy_config"
	"os"
	"path/filepath"
	"testing"
)

func TestResourceConsentNotPersisted(t *testing.T) {
	old := GlobalConfig
	oldAuth, oldPolicy := bb_api_auth_config.CurrentCfgAuth, bb_policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		GlobalConfig = old
		bb_api_auth_config.CurrentCfgAuth = oldAuth
		bb_policy_config.CurrentCfgPolicy = oldPolicy
	})
	r := &bb_resource.Config{Enabled: true, Files: []bb_resource.File{{ID: "model", Path: "data/model", SHA256: "operator-must-supply"}}, Download: bb_resource.Options{DownloadEnabled: true, Confirmed: true}}
	cfg := &FileConfig{CliCfg: &CLIConfig{Protenix: &CLIBinaryConfig{Resources: r}}}
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveConfig(file, cfg); err != nil {
		t.Fatal(err)
	}
	if err := LoadConfig(file); err != nil {
		t.Fatal(err)
	}
	loaded := GlobalConfig.CliCfg.Protenix.Resources
	if loaded == nil || loaded.Download.Confirmed || !loaded.Enabled || loaded.Files[0].ID != "model" {
		t.Fatal("consent persisted or manifest lost")
	}
}

func TestExecutableRootPreservesUnconfiguredOptionalPaths(t *testing.T) {
	oldApp, oldCfg := CurrentApp, GlobalConfig
	oldAuth, oldPolicy := bb_api_auth_config.CurrentCfgAuth, bb_policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		CurrentApp = oldApp
		GlobalConfig = oldCfg
		bb_api_auth_config.CurrentCfgAuth = oldAuth
		bb_policy_config.CurrentCfgPolicy = oldPolicy
	})
	CurrentApp = &ExtendApp{}
	file := filepath.Join(t.TempDir(), "config.json")
	raw := `{"storage_cfg":{"sqlite":{"enabled":false,"db_file_path":""}},"cli_cfg":{"protenix":{"binary":"","version_runtimes":{"v2":{"binary":""}}},"pxdesign":{"binary":""}}}`
	if err := os.WriteFile(file, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadConfig(file); err != nil {
		t.Fatalf("unused optional paths must not prevent app config loading: %v", err)
	}
	if GlobalConfig.StorageCfg != nil && GlobalConfig.StorageCfg.Sqlite != nil && GlobalConfig.StorageCfg.Sqlite.DBFilePath != "" {
		t.Fatal("empty disabled SQLite path became configured")
	}
	if GlobalConfig.CliCfg.Protenix.Binary != "" || GlobalConfig.CliCfg.PXDesign.Binary != "" || GlobalConfig.CliCfg.Protenix.VersionRuntimes["v2"].Binary != "" {
		t.Fatal("unconfigured CLI/version became configured")
	}
}

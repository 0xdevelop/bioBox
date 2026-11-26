package bb_ability_protenix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_pxdesign"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_config"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_executer"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/0xdevelop/bioBox/bb_storage/bb_storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
)

func TestExecuteValidatesAndRejectsUnconfigured(t *testing.T) {
	if _, err := Execute(context.Background(), map[string]interface{}{"out_dir": "out"}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("missing input must be invalid arguments: %v", err)
	}
	if _, err := Execute(context.Background(), map[string]interface{}{"input": "in.json", "out_dir": "out", "version": "v3"}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("invalid version must be rejected: %v", err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: bb_config.DefaultCLIConfig()}
	_, err := Execute(context.Background(), map[string]interface{}{"input": "in.json", "out_dir": "out", "version": "v1"})
	if err == nil || !strings.Contains(err.Error(), "CLI binary is not configured") {
		t.Fatalf("valid args without binary must return not-configured error: %v", err)
	}
}

func TestV2DoesNotFallBackToV1Binary(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "input.json")
	if err := os.WriteFile(src, []byte(`{"name":"src"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot: filepath.Join(work, "jobs"),
		Protenix: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: dummyRun(t)},
	}}
	_, err := Execute(context.Background(), map[string]interface{}{"version": "v2", "input": src, "out_dir": "out"})
	if err == nil || !strings.Contains(err.Error(), "protenix v2 CLI binary is not configured") {
		t.Fatalf("v2 without its own runtime must be rejected, err=%v", err)
	}
	if strings.Contains(err.Error(), "protenix CLI binary is not configured") && !strings.Contains(err.Error(), "v2") {
		t.Fatalf("v2 rejection must not be confused with v1 unconfigured: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(work, "jobs"))
	if len(entries) != 0 {
		t.Fatalf("rejected v2 must not stage a job directory, got %v", entries)
	}
}

func TestExecuteRunsConfiguredDummyAndReportsCIF(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "input.json")
	if err := os.WriteFile(src, []byte(`{"name":"src"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary:   dummyRun(t),
			ExtraEnv: map[string]string{"DUMMY_WRITE_CIF": "1"},
		},
	}}
	value, err := Execute(context.Background(), map[string]interface{}{
		"version": "v1",
		"input":   src,
		"out_dir": "out",
		"use_msa": false,
	})
	if err != nil {
		t.Fatalf("dummy execute: %v", err)
	}
	payload, ok := value.(map[string]interface{})
	if !ok {
		t.Fatalf("payload type: %T", value)
	}
	if payload["exit_code"] != 0 || payload["cancelled"] != false || payload["cif_count"] != 1 || payload["has_err"] != false {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestExecuteCancelStopsDummyProcess(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "input.json")
	if err := os.WriteFile(src, []byte(`{"name":"src"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary:   dummyRun(t),
			ExtraEnv: map[string]string{"DUMMY_SLEEP": "1"},
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	value, err := Execute(ctx, map[string]interface{}{"input": src, "out_dir": "out"})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancel, err=%v value=%v", err, value)
	}
	payload, _ := value.(map[string]interface{})
	if payload == nil || payload["cancelled"] != true {
		t.Fatalf("payload should mark cancelled: %+v", payload)
	}
}

func TestAsyncAcceptUsesSqliteWithoutMysql(t *testing.T) {
	previous := bb_config.GlobalConfig
	previousAuth := bb_api_auth_config.CurrentCfgAuth
	t.Cleanup(func() {
		_ = bb_storage.CloseStorage()
		bb_config.GlobalConfig = previous
		bb_api_auth_config.CurrentCfgAuth = previousAuth
	})

	bb_config.GlobalConfig = &bb_config.FileConfig{StorageCfg: &bb_storage_config.StorageConfig{
		Primary: "sqlite",
		Sqlite: &bb_storage_config.StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(t.TempDir(), "app.db")},
		},
	}}
	if err := bb_storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := bb_storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}

	bb_api_supported_methods.SupportedMethodsSetup()
	LoadAPIMethods()
	bb_ability_pxdesign.LoadAPIMethods()
	authEnabled := false
	bb_api_auth_config.CurrentCfgAuth = &bb_api_auth_config.AuthConfig{Enabled: &authEnabled}

	protenixMethod, ok := bb_api_supported_methods.Method(MethodProtenix)
	if !ok || !protenixMethod.Async || protenixMethod.Public {
		t.Fatalf("protenix must be a protected Async method: ok=%v method=%+v", ok, protenixMethod)
	}
	pxMethod, ok := bb_api_supported_methods.Method(bb_ability_pxdesign.MethodPXDesign)
	if !ok || !pxMethod.Async || pxMethod.Public {
		t.Fatalf("pxdesign must be a protected Async method: ok=%v method=%+v", ok, pxMethod)
	}

	ctx := context.Background()
	_, err := bb_api_executer.APIExecuter(ctx, bb_api_executer.ToolsCallMethod, map[string]interface{}{
		"name": MethodProtenix,
		"arguments": map[string]interface{}{
			"version":    "v1",
			"input":      "jobs/input.json",
			"out_dir":    "jobs/output",
			"model_name": "protenix_base_default_v1.0.0",
			"use_msa":    false,
			"seeds":      "101",
		},
	}, "")
	if err != nil {
		t.Fatalf("protenix async accept on SQLite: %v", err)
	}

	_, err = bb_api_executer.APIExecuter(ctx, bb_api_executer.ToolsCallMethod, map[string]interface{}{
		"name": bb_ability_pxdesign.MethodPXDesign,
		"arguments": map[string]interface{}{
			"input":    "jobs/task.yaml",
			"dump_dir": "jobs/output",
			"preset":   "preview",
			"N_sample": 2,
		},
	}, "")
	if err != nil {
		t.Fatalf("pxdesign async accept on SQLite: %v", err)
	}
}

func dummyRun(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "dummy-run.sh")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// Upstream can exit zero after invalid JSON; no structures must never mean success.
func TestExecuteRejectsZeroExitWithoutCIF(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "bad.json")
	if err := os.WriteFile(src, []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{WorkRoot: filepath.Join(work, "jobs"), Protenix: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: dummyRun(t)}}}
	value, err := Execute(context.Background(), map[string]interface{}{"input": src, "out_dir": "out"})
	if err == nil {
		t.Fatal("zero exit with no CIF was accepted as success")
	}
	payload := value.(map[string]interface{})
	if payload["exit_code"] != 0 || payload["cif_count"] != 0 {
		t.Fatalf("unexpected regression fixture result: %v", payload)
	}
}

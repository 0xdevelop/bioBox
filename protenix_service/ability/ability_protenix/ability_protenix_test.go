package ability_protenix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/protenix_service/ability/ability_pxdesign"
	"github.com/0xdevelop/protenix_service/api/api_auth/api_auth_config"
	"github.com/0xdevelop/protenix_service/api/api_error_code"
	"github.com/0xdevelop/protenix_service/api/api_executer"
	"github.com/0xdevelop/protenix_service/api/api_supported_methods"
	"github.com/0xdevelop/protenix_service/config"
	"github.com/0xdevelop/protenix_service/storage"
	"github.com/0xdevelop/protenix_service/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
)

func TestExecuteValidatesAndRejectsUnconfigured(t *testing.T) {
	if _, err := Execute(context.Background(), map[string]interface{}{"out_dir": "out"}); err != api_error_code.ErrInvalidArguments {
		t.Fatalf("missing input must be invalid arguments: %v", err)
	}
	if _, err := Execute(context.Background(), map[string]interface{}{"input": "in.json", "out_dir": "out", "version": "v3"}); err != api_error_code.ErrInvalidArguments {
		t.Fatalf("invalid version must be rejected: %v", err)
	}
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: config.DefaultCLIConfig()}
	_, err := Execute(context.Background(), map[string]interface{}{"input": "in.json", "out_dir": "out", "version": "v1"})
	if err == nil || !strings.Contains(err.Error(), "CLI binary is not configured") {
		t.Fatalf("valid args without binary must return not-configured error: %v", err)
	}
}

func TestBuildArgvOmitsVersionAndStagesInput(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "input.json")
	if err := os.WriteFile(src, []byte(`{"name":"src"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix:           &config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: dummyRun(t), ExtraEnv: map[string]string{"LAYERNORM_TYPE": "torch"}},
	}}
	argv, err := BuildArgvForTest(context.Background(), map[string]interface{}{
		"version":    "v1",
		"input":      src,
		"out_dir":    "caller-out",
		"model_name": "protenix_base_default_v1.0.0",
		"seeds":      "101",
		"use_msa":    false,
		"cycle":      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	if containsFlag(argv, "--version") || containsToken(argv, "v1") || strings.Contains(joined, "caller-out") {
		t.Fatalf("version or caller out_dir must not be CLI args: %s", joined)
	}
	if !containsPair(argv, "--model_name", "protenix_base_default_v1.0.0") || !containsPair(argv, "--seeds", "101") || !containsPair(argv, "--use_msa", "false") || !containsPair(argv, "--cycle", "10") {
		t.Fatalf("expected optional flags missing: %v", argv)
	}
	if !containsPair(argv, "--input", "") {
		// staged input path should exist and differ from original
	}
	inputPath := flagValue(argv, "--input")
	if inputPath == src || !strings.Contains(inputPath, filepath.Join(work, "jobs")) {
		t.Fatalf("input must be staged under work root, got %s", inputPath)
	}
	if _, err := os.Stat(inputPath); err != nil {
		t.Fatalf("staged input missing: %v", err)
	}
	original, _ := os.ReadFile(src)
	if err := os.WriteFile(inputPath, []byte(`{"name":"changed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(src)
	if string(after) != string(original) {
		t.Fatal("original input was modified")
	}
}

func TestVersionSelectsIndependentRuntime(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "input.json")
	if err := os.WriteFile(src, []byte(`{"name":"src"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runtimeOne := copyDummyRun(t, work, "runtime-one")
	runtimeTwo := copyDummyRun(t, work, "runtime-two")
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix: &config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary:   runtimeOne,
			ExtraEnv: map[string]string{"FROM": "base"},
			VersionRuntimes: map[string]*config.CLIRuntimeConfig{
				"v2": {Binary: runtimeTwo, ExtraEnv: map[string]string{"FROM": "v2"}},
			},
		},
	}}

	argvV1, err := BuildArgvForTest(context.Background(), map[string]interface{}{"input": src, "out_dir": "out"})
	if err != nil {
		t.Fatal(err)
	}
	if argvV1[0] != runtimeOne {
		t.Fatalf("omitted version must use v1 base binary, got %s", argvV1[0])
	}
	if containsFlag(argvV1, "--version") || containsToken(argvV1, "v1") || containsToken(argvV1, "v2") {
		t.Fatalf("version must not enter CLI args: %v", argvV1)
	}

	argvV2, err := BuildArgvForTest(context.Background(), map[string]interface{}{"version": "v2", "input": src, "out_dir": "out"})
	if err != nil {
		t.Fatal(err)
	}
	if argvV2[0] != runtimeTwo {
		t.Fatalf("version=v2 must use v2 runtime, got %s", argvV2[0])
	}
	if containsFlag(argvV2, "--version") || containsToken(argvV2, "v1") || containsToken(argvV2, "v2") {
		t.Fatalf("version must not enter CLI args: %v", argvV2)
	}

	override := copyDummyRun(t, work, "runtime-one-override")
	config.GlobalConfig.CliCfg.Protenix.VersionRuntimes["v1"] = &config.CLIRuntimeConfig{Binary: override}
	argvOverride, err := BuildArgvForTest(context.Background(), map[string]interface{}{"version": "v1", "input": src, "out_dir": "out"})
	if err != nil {
		t.Fatal(err)
	}
	if argvOverride[0] != override {
		t.Fatalf("version=v1 must use version_runtimes.v1 when set, got %s", argvOverride[0])
	}
}

func TestV2DoesNotFallBackToV1Binary(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "input.json")
	if err := os.WriteFile(src, []byte(`{"name":"src"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{
		WorkRoot: filepath.Join(work, "jobs"),
		Protenix: &config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: dummyRun(t)},
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
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix: &config.CLIBinaryConfig{Resources: fixtureResources(t),
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
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix: &config.CLIBinaryConfig{Resources: fixtureResources(t),
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
	previous := config.GlobalConfig
	previousAuth := api_auth_config.CurrentCfgAuth
	t.Cleanup(func() {
		_ = storage.CloseStorage()
		config.GlobalConfig = previous
		api_auth_config.CurrentCfgAuth = previousAuth
	})

	config.GlobalConfig = &config.FileConfig{StorageCfg: &storage_config.StorageConfig{
		Primary: "sqlite",
		Sqlite: &storage_config.StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(t.TempDir(), "app.db")},
		},
	}}
	if err := storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}

	api_supported_methods.SupportedMethodsSetup()
	LoadAPIMethods()
	ability_pxdesign.LoadAPIMethods()
	authEnabled := false
	api_auth_config.CurrentCfgAuth = &api_auth_config.AuthConfig{Enabled: &authEnabled}

	protenixMethod, ok := api_supported_methods.Method(MethodProtenix)
	if !ok || !protenixMethod.Async || protenixMethod.Public {
		t.Fatalf("protenix must be a protected Async method: ok=%v method=%+v", ok, protenixMethod)
	}
	pxMethod, ok := api_supported_methods.Method(ability_pxdesign.MethodPXDesign)
	if !ok || !pxMethod.Async || pxMethod.Public {
		t.Fatalf("pxdesign must be a protected Async method: ok=%v method=%+v", ok, pxMethod)
	}

	ctx := context.Background()
	_, err := api_executer.APIExecuter(ctx, api_executer.ToolsCallMethod, map[string]interface{}{
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

	_, err = api_executer.APIExecuter(ctx, api_executer.ToolsCallMethod, map[string]interface{}{
		"name": ability_pxdesign.MethodPXDesign,
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

func copyDummyRun(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(dummyRun(t))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
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

func containsPair(argv []string, name, value string) bool {
	return flagValue(argv, name) == value
}

func flagValue(argv []string, name string) string {
	for i, arg := range argv {
		if arg == name && i+1 < len(argv) {
			return argv[i+1]
		}
	}
	return ""
}

func containsFlag(argv []string, name string) bool {
	for _, arg := range argv {
		if arg == name || strings.HasPrefix(arg, name+"=") {
			return true
		}
	}
	return false
}

func containsToken(argv []string, token string) bool {
	for _, arg := range argv {
		if arg == token {
			return true
		}
	}
	return false
}

// Upstream can exit zero after invalid JSON; no structures must never mean success.
func TestExecuteRejectsZeroExitWithoutCIF(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "bad.json")
	if err := os.WriteFile(src, []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{WorkRoot: filepath.Join(work, "jobs"), Protenix: &config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: dummyRun(t)}}}
	value, err := Execute(context.Background(), map[string]interface{}{"input": src, "out_dir": "out"})
	if err == nil {
		t.Fatal("zero exit with no CIF was accepted as success")
	}
	payload := value.(map[string]interface{})
	if payload["exit_code"] != 0 || payload["cif_count"] != 0 {
		t.Fatalf("unexpected regression fixture result: %v", payload)
	}
}

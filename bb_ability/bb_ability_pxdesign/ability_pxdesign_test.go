package bb_ability_pxdesign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_config"
)

func TestExecuteValidatesAndRejectsUnconfigured(t *testing.T) {
	if _, err := Execute(context.Background(), map[string]interface{}{"dump_dir": "out"}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("missing input must be invalid arguments: %v", err)
	}
	if _, err := Execute(context.Background(), map[string]interface{}{"input": "in.yaml", "dump_dir": "out", "preset": "nope"}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("invalid preset must be rejected: %v", err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: bb_config.DefaultCLIConfig()}
	_, err := Execute(context.Background(), map[string]interface{}{"input": "in.yaml", "dump_dir": "out", "preset": "preview"})
	if err == nil || !strings.Contains(err.Error(), "CLI binary is not configured") {
		t.Fatalf("valid args without binary must return not-configured error: %v", err)
	}
}

func TestExecuteRunsConfiguredDummy(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "task.yaml")
	if err := os.WriteFile(src, []byte("name: src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		PXDesign: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary:   dummyRun(t),
			ExtraEnv: map[string]string{"DUMMY_WRITE_CIF": "1"},
		},
	}}
	value, err := Execute(context.Background(), map[string]interface{}{
		"input":    src,
		"dump_dir": "out",
		"preset":   "preview",
	})
	if err != nil {
		t.Fatalf("dummy execute: %v", err)
	}
	payload := value.(map[string]interface{})
	if payload["exit_code"] != 0 || payload["cif_count"] != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestExecuteCancelStopsDummyProcess(t *testing.T) {
	work := t.TempDir()
	src := filepath.Join(work, "task.yaml")
	if err := os.WriteFile(src, []byte("name: src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		PXDesign: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary:   dummyRun(t),
			ExtraEnv: map[string]string{"DUMMY_SLEEP": "1"},
		},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	value, err := Execute(ctx, map[string]interface{}{"input": src, "dump_dir": "out"})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancel, err=%v value=%v", err, value)
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

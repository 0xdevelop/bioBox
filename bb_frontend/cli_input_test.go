package bb_frontend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/bioBox/bb_config"
)

func TestLandCLIInputUsesWorkRootNotRuntimeCurrent(t *testing.T) {
	root := t.TempDir()
	runtimeDir := filepath.Join(root, "runtime")
	current := filepath.Join(runtimeDir, "current")
	binary := filepath.Join(runtimeDir, "bin", "pxdesign-run")
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "jobs")
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot: work,
		PXDesign: &bb_config.CLIBinaryConfig{Binary: binary},
	}}
	landed, err := landCLIInput("pxdesign", "task.json", []byte(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := filepath.Join(work, "frontend_inputs", "pxdesign")
	if !strings.HasPrefix(landed, wantPrefix) {
		t.Fatalf("landed %s, want under %s", landed, wantPrefix)
	}
	if strings.Contains(landed, current) {
		t.Fatalf("must not write into runtime current: %s", landed)
	}
}

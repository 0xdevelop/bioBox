package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/bioBox/common/resource"
	"github.com/0xdevelop/bioBox/runtime_resource"
	"github.com/goccy/go-yaml"
)

// Parse the shipped config, not a synthetic manifest. This is config/transport
// acceptance only; it neither installs an environment nor runs Python/inference.
func TestShippedOfflineResourceBaseline(t *testing.T) {
	raw, err := os.ReadFile("../example_files/config_example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "USER_EXISTING_LOCAL_SNAPSHOT_ONLY") {
		t.Fatal("missing provenance boundary")
	}
	var cfg FileConfig
	if err = yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, tc := range []struct {
		kind   string
		binary *CLIBinaryConfig
		count  int
	}{
		{"protenix", cfg.CliCfg.Protenix, 5}, {"pxdesign", cfg.CliCfg.PXDesign, 11},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			b := tc.binary
			if filepath.Base(b.Binary) != tc.kind+"-run" || b.Resources == nil || !b.Resources.Enabled || len(b.Resources.Files) != tc.count {
				t.Fatal("incomplete default config")
			}
			urlCount := 0
			for _, f := range b.Resources.Files {
				if f.URL != "" {
					urlCount++
					if !strings.HasPrefix(f.URL, "https://") {
						t.Fatal("non-HTTPS source")
					}
				}
				if (f.ID == "af2_model_1" || f.ID == "af2_model_1_ptm") && f.URL != "" {
					t.Fatal("unverified AF2 archive shortcut")
				}
			}
			wantURLs := tc.count
			if tc.kind == "pxdesign" {
				wantURLs -= 2
			}
			if urlCount != wantURLs {
				t.Fatalf("URL count=%d want %d", urlCount, wantURLs)
			}
			if b.Resources.Download.DownloadEnabled || b.Resources.Download.Confirmed {
				t.Fatal("offline baseline granted network consent")
			}
			// Default binary paths deliberately retain the official install root for
			// the installer's pre-copy coordinator. Explicit relative overrides
			// remain binary-rooted, not cwd-rooted.
			binary := "./runtime/" + tc.kind + "/bin/" + tc.kind + "-run"
			dir := filepath.Join(root, "runtime", tc.kind, "conf")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			env := "PROTENIX_MODEL=protenix_base_default_v1.0.0\nPROTENIX_ROOT_DIR=./data\n"
			if tc.kind == "pxdesign" {
				env = "PXDESIGN_DATA=./data\nPXDESIGN_CHECKPOINT_DIR=\nTOOL_WEIGHTS_ROOT=\nPROTENIX_DATA_ROOT_DIR=\n"
			}
			if err := os.WriteFile(filepath.Join(dir, tc.kind+".env"), []byte(env), 0644); err != nil {
				t.Fatal(err)
			}
			compiled, err := runtime_resource.Compile(tc.kind, binary, root, b.ExtraEnv, b.Resources)
			if err != nil {
				t.Fatal(err)
			}
			if err = runtime_resource.ValidateManifest(tc.kind, binary, root, b.ExtraEnv, compiled); err != nil {
				t.Fatal(err)
			}
			for _, f := range compiled.Files {
				if f.SHA256 == "" || f.Size <= 0 || !filepath.IsAbs(f.Path) || !strings.HasPrefix(f.Path, root+string(os.PathSeparator)) {
					t.Fatal("unsafe/incomplete baseline file")
				}
			}
			ctl := resource.Controller{Root: root, Sets: map[string]*resource.Config{"chosen": compiled}}
			status, err := ctl.Status(context.Background(), "chosen")
			if err != nil || status.State != "download_disabled" {
				t.Fatalf("missing resources must be explicit offline status: %+v %v", status, err)
			}
			if err = runtime_resource.ValidateArguments(tc.kind, []string{"pred", "--use_msa", "false"}); err != nil {
				t.Fatal(err)
			}
			if err = runtime_resource.ValidateArguments(tc.kind, []string{"pred"}); err == nil {
				t.Fatal("missing no-MSA selection silently accepted")
			}
			incomplete := *compiled
			incomplete.Files = incomplete.Files[1:]
			if err = runtime_resource.ValidateManifest(tc.kind, binary, root, b.ExtraEnv, &incomplete); err == nil {
				t.Fatal("incomplete manifest accepted")
			}
		})
	}
}

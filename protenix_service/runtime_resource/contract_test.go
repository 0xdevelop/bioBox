package runtime_resource

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/0xdevelop/protenix_service/common/resource"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialManifestCompletenessAndRelativeSymlinkRoots(t *testing.T) {
	app := t.TempDir()
	prefix := filepath.Join(app, "runtime/protenix")
	os.MkdirAll(filepath.Join(prefix, "conf"), 0700)
	os.MkdirAll(filepath.Join(prefix, "bin"), 0700)
	os.WriteFile(filepath.Join(prefix, "conf/protenix.env"), []byte("PROTENIX_ROOT_DIR=models\n"), 0600)
	existing := t.TempDir()
	if err := os.Symlink(existing, filepath.Join(prefix, "models")); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(prefix, "bin/protenix-run")
	files, err := Required("protenix", binary, app)
	if err != nil || len(files) != 5 {
		t.Fatal(files, err)
	}
	cfg := &resource.Config{Enabled: true}
	for _, p := range files {
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		body := []byte("fixture")
		os.WriteFile(p, body, 0600)
		sum := sha256.Sum256(body)
		cfg.Files = append(cfg.Files, resource.File{Path: p, SHA256: hex.EncodeToString(sum[:])})
	}
	if err = ValidateManifest("protenix", binary, app, nil, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Files = cfg.Files[:4]
	if err = ValidateManifest("protenix", binary, app, nil, cfg); err == nil {
		t.Fatal("incomplete official manifest accepted")
	}
	for _, args := range [][]string{{"pred"}, {"pred", "--use_msa", "true"}, {"pred", "--use_msa", "false", "--model_name", "protenix_mini_esm_v0.5.0"}, {"pred", "--use_msa", "false", "--use_template", "true"}} {
		if err = ValidateArguments("protenix", args); !errors.Is(err, ErrUnsupported) {
			t.Fatal(args, err)
		}
	}
	if err = ValidateArguments("protenix", []string{"pred", "--use_msa", "false"}); err != nil {
		t.Fatal(err)
	}
	if err = ValidateInstalled("protenix", binary, app); !errors.Is(err, ErrUnsupported) {
		t.Fatal("unknown runtime should block", err)
	}
}
func TestReadOnlyInstalledRuntimeEvidence(t *testing.T) {
	root := os.Getenv("READONLY_RUNTIME_EVIDENCE")
	if root == "" {
		t.Skip("explicit local installed-runtime evidence opt-in")
	}
	for _, kind := range []string{"protenix", "pxdesign"} {
		binary := filepath.Join(root, kind, "bin", kind+"-run")
		if err := ValidateInstalled(kind, binary, root); err != nil {
			t.Fatal(kind, err)
		}
	}
}

func TestRoleManifestFollowsSingleConfiguredRootValue(t *testing.T) {
	app := t.TempDir()
	prefix := filepath.Join(app, "runtime/protenix")
	os.MkdirAll(filepath.Join(prefix, "conf"), 0700)
	binary := filepath.Join(prefix, "bin/protenix-run")
	cfg := &resource.Config{Enabled: true}
	for _, id := range []string{"ccd_components_file", "ccd_components_rdkit_mol_file", "pdb_cluster_file", "obsolete_release_data_csv", "model"} {
		cfg.Files = append(cfg.Files, resource.File{ID: id, SHA256: "not-an-authenticity-claim"})
	}
	for _, dir := range []string{"existing_one", "existing_two"} {
		os.WriteFile(filepath.Join(prefix, "conf/protenix.env"), []byte("PROTENIX_ROOT_DIR="+dir+"\n"), 0600)
		compiled, err := Compile("protenix", binary, app, nil, cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range compiled.Files {
			if !strings.HasPrefix(file.Path, filepath.Join(prefix, dir)+string(filepath.Separator)) {
				t.Fatal(file.Path)
			}
		}
	}
	// Digests above are deliberately invalid: compilation binds paths, whereas
	// Preparer/Controller must reject them. No model checksum is invented here.
}
func TestRuntimeEnvRejectsShellSemanticMismatch(t *testing.T) {
	for _, value := range []string{"models # comment", "models; echo unsafe", `"models\\escaped"`, "$(false)", "'not-closed"} {
		if _, err := parseLiteral(value); err == nil {
			t.Fatal("accepted nonliteral", value)
		}
	}
	if v, err := parseLiteral(`"models with space"`); err != nil || v != "models with space" {
		t.Fatal(v, err)
	}
}

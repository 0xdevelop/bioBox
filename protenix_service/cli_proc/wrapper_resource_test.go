//go:build linux

package cli_proc

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOwnedWrappersRootResourcesAndRefuseUnreadyPython(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	for _, kind := range []string{"protenix", "pxdesign"} {
		t.Run(kind, func(t *testing.T) {
			product := t.TempDir()
			prefix := filepath.Join(product, "runtime", kind)
			marker := filepath.Join(product, "python-called")
			ready := filepath.Join(product, "ready")
			for _, dir := range []string{"bin", "conf", "venv/bin"} {
				if err := os.MkdirAll(filepath.Join(prefix, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			native := "#!/bin/sh\n[ \"$1\" = resource ] && [ \"$4\" = verify ] && [ -f '" + ready + "' ]\n"
			os.WriteFile(filepath.Join(product, "protenix_service"), []byte(native), 0700)
			fake := "#!/bin/sh\nprintf '%s|%s|%s|%s' \"${PROTENIX_ROOT_DIR:-}\" \"${TOOL_WEIGHTS_ROOT:-}\" \"${PROTENIX_DATA_ROOT_DIR:-}\" \"$*\" > '" + marker + "'\n"
			os.WriteFile(filepath.Join(prefix, "venv/bin", kind), []byte(fake), 0700)
			contents, err := os.ReadFile(filepath.Join(repo, "example_files", kind+"-run"))
			if err != nil {
				t.Fatal(err)
			}
			wrapper := filepath.Join(prefix, "bin", kind+"-run")
			os.WriteFile(wrapper, contents, 0700)
			conf := "PROTENIX_ROOT_DIR=models\n"
			command := "pred"
			if kind == "pxdesign" {
				conf = "PXDESIGN_DATA=storage\nTOOL_WEIGHTS_ROOT=tools\nPROTENIX_DATA_ROOT_DIR=ccd\nPXDESIGN_CHECKPOINT_DIR=checkpoints\n"
				command = "pipeline"
			}
			os.WriteFile(filepath.Join(prefix, "conf", kind+".env"), []byte(conf), 0600)
			run := func(dir string, args ...string) error {
				cmd := exec.Command("bash", append([]string{wrapper}, args...)...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "PROTENIX_ROOT_DIR=/unrelated-global", "TOOL_WEIGHTS_ROOT=/unrelated-global")
				return cmd.Run()
			}
			if err = run(t.TempDir(), command, "--use_msa", "false"); err == nil {
				t.Fatal("unready wrapper allowed Python")
			}
			if _, err = os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("Python started without ready")
			}
			os.WriteFile(ready, []byte("yes"), 0600)
			for i := 0; i < 2; i++ {
				if err = run(t.TempDir(), command, "--use_msa", "false"); err != nil {
					t.Fatal(err)
				}
				data, err := os.ReadFile(marker)
				if err != nil {
					t.Fatal(err)
				}
				text := string(data)
				if strings.Contains(text, "unrelated-global") {
					t.Fatal("global resource env leaked")
				}
				if kind == "protenix" && !strings.HasPrefix(text, filepath.Join(prefix, "models")+"|") {
					t.Fatal(text)
				}
				if kind == "pxdesign" && (!strings.Contains(text, filepath.Join(prefix, "tools")) || !strings.Contains(text, filepath.Join(prefix, "checkpoints")) || !strings.Contains(text, filepath.Join(prefix, "ccd"))) {
					t.Fatal(text)
				}
			}
			os.Remove(marker)
			if err = run(t.TempDir(), command, "--use_msa", "true"); err == nil {
				t.Fatal("unsupported MSA branch")
			}
			if _, err = os.Stat(marker); !os.IsNotExist(err) {
				t.Fatal("unsupported branch reached Python")
			}
		})
	}
}
func TestManagedInstallerHasNoSecondWeightTransport(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	for _, kind := range []string{"protenix", "pxdesign"} {
		p := filepath.Join(repo, "scripts", "install_prefix_"+kind+".sh")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		for _, bad := range []string{"curl -fL", "git clone -q --depth 1", "seed_tree_verified", "fetch_one()", "fetch()"} {
			if strings.Contains(s, bad) {
				t.Fatal("independent weight owner remains", kind, bad)
			}
		}
		if !strings.Contains(s, "resource prepare ") {
			t.Fatal("no native coordinator", kind)
		}
		cmd := exec.Command("bash", "-n", p)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
}

func TestProtenixInstallerCompletionMarkerHonorsEnvironmentOnly(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	data, err := os.ReadFile(filepath.Join(repo, "scripts/install_prefix_protenix.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.LastIndex(text, "echo \"${TAG} freeze ->")
	if start < 0 {
		t.Fatal("completion block missing")
	}
	tail := text[start:]
	for _, skip := range []string{"0", "1"} {
		t.Run(skip, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "install.ready")
			cmd := exec.Command("bash", "-eu", "-c", tail)
			cmd.Env = append(os.Environ(), "TAG=[fixture]", "PREFIX="+root, "READY_MARK="+marker, "SRC_SHA=test-source", "PROTENIX_MODEL=test-model", "PYV=3.14.0", "SKIP_WEIGHTS="+skip)
			output, e := cmd.CombinedOutput()
			if e != nil {
				t.Fatal(e, string(output))
			}
			contents, e := os.ReadFile(marker)
			if skip == "0" {
				if e != nil || !strings.Contains(string(contents), "src_sha=test-source") || !strings.Contains(string(contents), "python=3.14.0") {
					t.Fatal("successful full preparation lost ready marker", e, string(output))
				}
			} else {
				if !os.IsNotExist(e) || !strings.Contains(string(output), "status=environment_only") {
					t.Fatal("environment-only install claimed full readiness", e, string(output))
				}
			}
		})
	}
}

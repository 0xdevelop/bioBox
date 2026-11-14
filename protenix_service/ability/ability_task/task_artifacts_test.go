package ability_task

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xdevelop/protenix_service/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/protenix_service/config"
)

func TestCleanArtifactRelPath(t *testing.T) {
	allowed, ok := cleanArtifactRelPath("output/pred.cif")
	if !ok || allowed != "output/pred.cif" {
		t.Fatalf("allowed path rejected: %q %t", allowed, ok)
	}
	for _, raw := range []string{
		"", "../out.cif", "/tmp/a.cif", "output/../../etc/passwd", "input/secret.cif",
		"input", `output\pred.cif`, "output/pred.bin", "output/./../../etc/hosts",
	} {
		if _, ok := cleanArtifactRelPath(raw); ok {
			t.Fatalf("unsafe path accepted: %q", raw)
		}
	}
}

func TestListAndReadTaskArtifactsConfined(t *testing.T) {
	work := t.TempDir()
	taskID := "11111111-1111-1111-1111-111111111111"
	job := filepath.Join(work, taskID)
	output := filepath.Join(job, "output", "seed")
	inputDir := filepath.Join(job, "input")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cif := []byte("data_fixture\n")
	if err := os.WriteFile(filepath.Join(output, "pred.cif"), cif, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "confidence.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(job, "run.log"), []byte("log-line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "source.cif"), []byte("secret-input\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "weights.bin"), []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "escape.cif")
	if err := os.WriteFile(outside, []byte("escaped\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(output, "escape.cif")); err != nil {
		t.Fatal(err)
	}

	listed, err := listTaskArtifacts(job)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]artifactInfo{}
	for _, item := range listed {
		found[item.Path] = item
	}
	if _, ok := found["output/seed/pred.cif"]; !ok {
		t.Fatalf("missing cif: %+v", listed)
	}
	if _, ok := found["output/seed/confidence.json"]; !ok {
		t.Fatalf("missing json: %+v", listed)
	}
	if _, ok := found["run.log"]; !ok {
		t.Fatalf("missing log: %+v", listed)
	}
	if _, ok := found["input/source.cif"]; ok {
		t.Fatal("input file was listed")
	}
	if _, ok := found["output/seed/weights.bin"]; ok {
		t.Fatal("disallowed extension was listed")
	}
	if _, ok := found["output/seed/escape.cif"]; ok {
		t.Fatal("symlink escape was listed")
	}

	file, err := readTaskArtifact(job, "output/seed/pred.cif")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(file.Content)
	if err != nil || string(decoded) != string(cif) {
		t.Fatalf("content mismatch: %q", decoded)
	}
	if _, err = readTaskArtifact(job, "input/source.cif"); err == nil {
		t.Fatal("input read must fail")
	}
	if _, err = readTaskArtifact(job, "../"+filepath.Base(outside)); err == nil {
		t.Fatal("traversal read must fail")
	}
	if _, err = readTaskArtifact(job, "output/seed/escape.cif"); err == nil {
		t.Fatal("symlink read must fail")
	}

	previous := artifactMaxBytes
	artifactMaxBytes = 4
	t.Cleanup(func() { artifactMaxBytes = previous })
	if _, err = readTaskArtifact(job, "output/seed/pred.cif"); err == nil {
		t.Fatal("oversize read must fail")
	}
}

func TestVerifyJobDirRejectsForeignPath(t *testing.T) {
	work := t.TempDir()
	taskID := "22222222-2222-2222-2222-222222222222"
	if err := os.MkdirAll(filepath.Join(work, taskID), 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := t.TempDir()
	if _, err := verifyJobDir(work, taskID, foreign); err == nil {
		t.Fatal("foreign job dir accepted")
	}
	ok, err := verifyJobDir(work, taskID, filepath.Join(work, taskID))
	if err != nil || ok != filepath.Join(work, taskID) && filepath.Base(ok) != taskID {
		t.Fatalf("expected job dir: %s %v", ok, err)
	}
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig = &config.FileConfig{CliCfg: &config.CLIConfig{WorkRoot: work}}
	task := &ability_task_model.AsyncTask{
		TaskID:        taskID,
		RunStatus:     RunStatusDone,
		ResultPayload: `{"job_dir":"` + filepath.ToSlash(foreign) + `"}`,
	}
	if _, err = confinedJobDir(task); err == nil {
		t.Fatal("payload with foreign job_dir accepted")
	}
}

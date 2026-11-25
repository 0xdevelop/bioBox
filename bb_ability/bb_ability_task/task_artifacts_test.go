package bb_ability_task

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task/bb_ability_task_model"
	"github.com/0xdevelop/bioBox/bb_config"
)

func TestCleanArtifactRelPath(t *testing.T) {
	allowed, ok := cleanArtifactRelPath("output/pred.cif")
	if !ok || allowed != "output/pred.cif" {
		t.Fatalf("allowed path rejected: %q %t", allowed, ok)
	}
	for _, raw := range []string{
		"", "../out.cif", "/tmp/a.cif", "output/../../etc/passwd", "input/secret.cif", "examples/5o45.cif",
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
	examples := filepath.Join(job, "examples")
	if err := os.MkdirAll(examples, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(examples, "5o45.cif"), []byte("target-input\n"), 0o600); err != nil {
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
	if _, ok := found["examples/5o45.cif"]; ok {
		t.Fatal("staged official input sidecar was listed as an artifact")
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
	wantDigest := sha256.Sum256(cif)
	if file.SHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("sha256 mismatch: got %s", file.SHA256)
	}
	if _, err = readTaskArtifact(job, "input/source.cif"); err == nil {
		t.Fatal("input read must fail")
	}
	if _, err = readTaskArtifact(job, "examples/5o45.cif"); err == nil {
		t.Fatal("staged official input sidecar read must fail")
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
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{WorkRoot: work}}
	task := &bb_ability_task_model.AsyncTask{
		TaskID:        taskID,
		RunStatus:     RunStatusDone,
		ResultPayload: `{"job_dir":"` + filepath.ToSlash(foreign) + `"}`,
	}
	dir, err := confinedJobDir(task)
	if err != nil {
		t.Fatalf("foreign payload should be ignored in favor of work_root/task_id: %v", err)
	}
	if dir == foreign || dir == filepath.Clean(foreign) {
		t.Fatal("payload with foreign job_dir accepted")
	}
	if filepath.Base(dir) != taskID {
		t.Fatalf("confined fallback was not work_root/task_id: %s", dir)
	}

	running := &bb_ability_task_model.AsyncTask{TaskID: taskID, RunStatus: RunStatusRunning}
	if _, err = confinedJobDir(running); err == nil {
		t.Fatal("running task must not expose artifacts")
	}
	failed := &bb_ability_task_model.AsyncTask{TaskID: taskID, RunStatus: RunStatusDone, ProcessStatus: ProcessStatusSystemError}
	if dir, err = confinedJobDir(failed); err != nil || filepath.Base(dir) != taskID {
		t.Fatalf("done non-success without payload should still use work_root/task_id: %s %v", dir, err)
	}
}

func TestListArtifactsOnOfficialPXDesignJobIfPresent(t *testing.T) {
	job := os.Getenv("PXD_ACCEPT_JOB_DIR")
	if job == "" {
		t.Skip("set PXD_ACCEPT_JOB_DIR to list a completed official PXDesign job")
	}
	if _, err := os.Stat(job); err != nil {
		t.Fatalf("PXD_ACCEPT_JOB_DIR is not accessible: %v", err)
	}
	listed, err := listTaskArtifacts(job)
	if err != nil {
		t.Fatal(err)
	}
	var design string
	for _, item := range listed {
		if strings.HasPrefix(item.Path, "examples/") || strings.HasPrefix(item.Path, "input/") {
			t.Fatalf("input sidecar was listed: %s", item.Path)
		}
		if strings.HasPrefix(item.Path, "output/") && strings.HasSuffix(strings.ToLower(item.Path), ".cif") && strings.Contains(item.Path, "design_outputs") {
			design = item.Path
		}
	}
	if design == "" {
		t.Fatalf("official design cif was not listed: %+v", listed)
	}
	file, err := readTaskArtifact(job, design)
	if err != nil || file.Kind != "cif" || file.Size < 1000 {
		t.Fatalf("official design cif read failed: err=%v file=%+v", err, file)
	}
	t.Logf("official job artifacts listed design cif %s size=%d count=%d", design, file.Size, len(listed))
}

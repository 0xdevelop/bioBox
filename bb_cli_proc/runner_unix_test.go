//go:build unix

package bb_cli_proc

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunCompletesWithoutCancel(t *testing.T) {
	work := t.TempDir()
	result, err := Run(context.Background(), Spec{
		Path:    wrapperPath(t),
		Args:    []string{"ok"},
		LogPath: filepath.Join(work, "run.log"),
		Grace:   time.Second,
	})
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if result.Cancelled || result.ExitCode != 0 || result.PID <= 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	body, _ := os.ReadFile(result.LogPath)
	if !strings.Contains(string(body), "wrapper-ok") {
		t.Fatalf("log missing wrapper output: %s", body)
	}
}

func TestCancelSignalsWholeProcessGroup(t *testing.T) {
	work := t.TempDir()
	childPIDFile := filepath.Join(work, "child.pid")
	heartbeat := filepath.Join(work, "heartbeat.txt")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var result *Result
	var runErr error
	go func() {
		defer close(done)
		result, runErr = Run(ctx, Spec{
			Path: wrapperPath(t),
			Env: map[string]string{
				"CHILD_SCRIPT":   childPath(t),
				"CHILD_PID_FILE": childPIDFile,
				"HEARTBEAT_FILE": heartbeat,
			},
			LogPath: filepath.Join(work, "run.log"),
			Grace:   time.Second,
		})
	}()
	childPID := waitForPIDFile(t, childPIDFile)
	if !processExists(childPID) {
		t.Fatalf("child %d should be running before cancel", childPID)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "cancelled") || result == nil || !result.Cancelled {
		t.Fatalf("expected cancelled result, err=%v result=%+v", runErr, result)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(childPID) && (result.PID == 0 || !processExists(result.PID)) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("process group still alive after cancel: wrapper=%d child=%d", result.PID, childPID)
}

func TestSignalWrapperOnlyLeavesChildRunning(t *testing.T) {
	work := t.TempDir()
	childPIDFile := filepath.Join(work, "child.pid")
	heartbeat := filepath.Join(work, "heartbeat.txt")
	cmd := exec.Command(wrapperPath(t))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(),
		"CHILD_SCRIPT="+childPath(t),
		"CHILD_PID_FILE="+childPIDFile,
		"HEARTBEAT_FILE="+heartbeat,
	)
	cmd.Dir = work
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	childPID := waitForPIDFile(t, childPIDFile)
	wrapperPID := cmd.Process.Pid
	t.Cleanup(func() {
		_ = signalProcessGroup(wrapperPID, syscall.SIGKILL)
		_ = cmd.Wait()
	})
	if err := signalProcess(wrapperPID, syscall.SIGTERM); err != nil {
		t.Fatalf("signal wrapper: %v", err)
	}
	time.Sleep(400 * time.Millisecond)
	if !processExists(childPID) {
		t.Fatalf("child %d should still run after SIGTERM only to wrapper %d (old boundary)", childPID, wrapperPID)
	}
}

func TestRunReapsLeftoversWhenWrapperDies(t *testing.T) {
	work := t.TempDir()
	childPIDFile := filepath.Join(work, "child.pid")
	heartbeat := filepath.Join(work, "heartbeat.txt")
	done := make(chan struct{})
	var result *Result
	var runErr error
	go func() {
		defer close(done)
		result, runErr = Run(context.Background(), Spec{
			Path: wrapperPath(t),
			Env: map[string]string{
				"CHILD_SCRIPT":   childPath(t),
				"CHILD_PID_FILE": childPIDFile,
				"HEARTBEAT_FILE": heartbeat,
			},
			LogPath: filepath.Join(work, "run.log"),
			Grace:   time.Second,
		})
	}()
	childPID := waitForPIDFile(t, childPIDFile)
	wrapperPID := waitForWrapperPID(t, childPID)
	if err := signalProcess(wrapperPID, syscall.SIGTERM); err != nil {
		t.Fatalf("signal wrapper: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after wrapper exit")
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "leftover process group") || result == nil {
		t.Fatalf("expected leftover reap error, err=%v result=%+v", runErr, result)
	}
	if processExists(childPID) {
		t.Fatalf("child %d should have been reaped by Run", childPID)
	}
}

func TestStageInputCopiesAndLeavesOriginalUnchanged(t *testing.T) {
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "input.json")
	original := []byte(`{"name":"original"}`)
	if err := os.WriteFile(src, original, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	jobDir, staged, err := StageInput(work, "task-1", src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(staged, jobDir) {
		t.Fatalf("staged path must be under job dir: %s %s", staged, jobDir)
	}
	copied, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != string(original) {
		t.Fatalf("copied content mismatch: %s", copied)
	}
	if err := os.WriteFile(staged, []byte(`{"name":"changed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("original input was modified: %s", after)
	}
	later, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if later.ModTime().Unix() != info.ModTime().Unix() {
		t.Fatalf("original mtime changed: %v -> %v", info.ModTime(), later.ModTime())
	}
}

func TestInspectOutputCountsCIFAndERR(t *testing.T) {
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "a.cif"), []byte("data_"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(out, "ERR"), 0o755); err != nil {
		t.Fatal(err)
	}
	cif, hasERR, err := InspectOutput(out)
	if err != nil || cif != 1 || !hasERR {
		t.Fatalf("inspect: cif=%d hasERR=%v err=%v", cif, hasERR, err)
	}
}

func wrapperPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "wrapper.sh")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func childPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("testdata", "child.sh")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(body)))
			if convErr == nil && pid > 0 && processExists(pid) {
				return pid
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for pid file %s", path)
	return 0
}

func waitForWrapperPID(t *testing.T, childPID int) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		body, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(childPID), "stat"))
		if err == nil {
			fields := strings.Fields(string(body))
			if len(fields) > 3 {
				ppid, convErr := strconv.Atoi(fields[3])
				if convErr == nil && ppid > 1 && processExists(ppid) {
					return ppid
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out resolving wrapper pid for child %d", childPID)
	return 0
}

func TestStageRelativeSidecarsCopiesOfficialLayout(t *testing.T) {
	project := t.TempDir()
	examples := filepath.Join(project, "examples")
	msa := filepath.Join(examples, "msa", "PDL1", "0")
	if err := os.MkdirAll(msa, 0o755); err != nil {
		t.Fatal(err)
	}
	cif := []byte("data_official_sidecar\n")
	msaBody := []byte(">msa\nACDE\n")
	if err := os.WriteFile(filepath.Join(examples, "5o45.cif"), cif, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(msa, "pairing.a3m"), msaBody, 0o644); err != nil {
		t.Fatal(err)
	}
	yamlBody := []byte("target:\n  file: \"./examples/5o45.cif\"\n  chains:\n    A:\n      crop: [\"1-116\"]\n      msa: \"./examples/msa/PDL1/0\"\nbinder_length: 80\n")
	src := filepath.Join(examples, "PDL1_quick_start.yaml")
	if err := os.WriteFile(src, yamlBody, 0o644); err != nil {
		t.Fatal(err)
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	jobDir, staged, err := StageInput(work, "task-sidecar", src)
	if err != nil {
		t.Fatal(err)
	}
	if err = StageRelativeSidecars(jobDir, src, staged); err != nil {
		t.Fatal(err)
	}
	copiedYAML, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedYAML) != string(yamlBody) {
		t.Fatalf("staged yaml was rewritten: %s", copiedYAML)
	}
	copiedCIF, err := os.ReadFile(filepath.Join(jobDir, "examples", "5o45.cif"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedCIF) != string(cif) {
		t.Fatalf("cif not staged: %s", copiedCIF)
	}
	copiedMSA, err := os.ReadFile(filepath.Join(jobDir, "examples", "msa", "PDL1", "0", "pairing.a3m"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedMSA) != string(msaBody) {
		t.Fatalf("msa not staged: %s", copiedMSA)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "examples", "5o45.cif"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalCIF, err := os.ReadFile(filepath.Join(examples, "5o45.cif"))
	if err != nil {
		t.Fatal(err)
	}
	if string(originalCIF) != string(cif) {
		t.Fatal("original cif was modified")
	}
	later, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if later.ModTime().Unix() != srcInfo.ModTime().Unix() {
		t.Fatal("original yaml mtime changed")
	}
}

func TestStageRelativeSidecarsRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "proj")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(project, "task.yaml")
	if err := os.WriteFile(src, []byte("file: \"./../secret.txt\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	jobDir, staged, err := StageInput(work, "task-traversal", src)
	if err != nil {
		t.Fatal(err)
	}
	err = StageRelativeSidecars(jobDir, src, staged)
	if err == nil || !strings.Contains(err.Error(), "outside the job directory") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

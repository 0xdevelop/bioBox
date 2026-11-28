//go:build linux

package bb_cli_proc

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestProcessGroupAliveInDirRejectsPidReuse(t *testing.T) {
	jobDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		jobDir = t.TempDir()
	}
	cmd := exec.Command("sleep", "30")
	cmd.Dir = jobDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	}()
	pgid := cmd.Process.Pid
	if err := WriteJobProcessGroup(jobDir, pgid); err != nil {
		t.Fatal(err)
	}
	if !ProcessGroupAliveInDir(pgid, jobDir) {
		t.Fatal("sleep started in jobDir should be considered alive")
	}
	// The test process itself is alive but its cwd is not jobDir.
	self := os.Getpid()
	if ProcessGroupAliveInDir(self, jobDir) {
		t.Fatal("a live pid with a different cwd must not count as the job")
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	_, _ = cmd.Process.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for ProcessGroupAliveInDir(pgid, jobDir) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if ProcessGroupAliveInDir(pgid, jobDir) {
		t.Fatal("killed process group still looks alive")
	}
}

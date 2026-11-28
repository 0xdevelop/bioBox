//go:build unix

package bb_ability_task

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task/bb_ability_task_model"
	"github.com/0xdevelop/bioBox/bb_cli_proc"
	"github.com/0xdevelop/bioBox/bb_config"
)

func TestRequeueSkipsAliveProcessGroupAndSignalsCancel(t *testing.T) {
	db := setupTaskSQLite(t)
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		work = t.TempDir()
	}
	bb_config.GlobalConfig.CliCfg = &bb_config.CLIConfig{WorkRoot: work}
	registerProbe(t, "orphan-pgid", func(context.Context, interface{}) (interface{}, error) { return nil, nil })

	accepted, err := AcceptAsyncTask(context.Background(), "orphan-pgid", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	if _, err = claimQueuedTask(context.Background()); err != nil {
		t.Fatal(err)
	}
	unmarkTaskInFlight(id)

	jobDir := filepath.Join(work, id)
	if err = os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sleep", "30")
	cmd.Dir = jobDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_, _ = cmd.Process.Wait()
	}()
	if err = bb_cli_proc.WriteJobProcessGroup(jobDir, cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}

	if err = RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var task bb_ability_task_model.AsyncTask
	if err = db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusRunning {
		t.Fatalf("live process group must not be requeued: %+v", task)
	}

	if err = db.Model(&bb_ability_task_model.AsyncTask{}).Where("task_id = ?", id).
		Update("cancel_requested", true).Error; err != nil {
		t.Fatal(err)
	}
	if err = RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusRunning || !task.CancelRequested {
		t.Fatalf("cancel of a live leftover should signal and wait, not finish yet: %+v", task)
	}
	waitExited(t, cmd)

	if err = RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusDone || task.ProcessStatus != ProcessStatusCancelled {
		t.Fatalf("after the process exits, cancel should complete: %+v", task)
	}
}

func waitExited(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
	}
}

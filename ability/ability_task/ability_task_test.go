package ability_task

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/bioBox/api/api_supported_methods"
	"github.com/0xdevelop/bioBox/config"
	"github.com/0xdevelop/bioBox/policy/policy_config"
	"github.com/0xdevelop/bioBox/storage"
	"github.com/0xdevelop/bioBox/storage/storage_config"
	"gorm.io/gorm"
)

func setupTaskSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	previous := config.GlobalConfig
	previousPolicy := policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		_ = storage.CloseStorage()
		config.GlobalConfig = previous
		policy_config.CurrentCfgPolicy = previousPolicy
	})
	policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{}
	config.GlobalConfig = &config.FileConfig{StorageCfg: storage_config.Default(filepath.Join(t.TempDir(), "queue.db"))}
	if err := storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSQLiteConcurrentClaimsExecuteOnce(t *testing.T) {
	db := setupTaskSQLite(t)
	api_supported_methods.SupportedMethodsSetup()
	var mu sync.Mutex
	seen := map[string]int{}
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name: "queue-probe", Async: true, InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Execute: func(ctx context.Context, input interface{}) (interface{}, error) {
			info, _ := AsyncTaskInfoFromContext(ctx)
			mu.Lock()
			seen[info.TaskID]++
			mu.Unlock()
			time.Sleep(time.Millisecond)
			return map[string]interface{}{"ok": true}, nil
		},
	})
	for i := 0; i < 24; i++ {
		if _, err := AcceptAsyncTask(context.Background(), "queue-probe", map[string]interface{}{"n": i}); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				ok, err := ExecuteNextQueuedTask(context.Background())
				if err != nil {
					t.Error(err)
					return
				}
				if !ok {
					return
				}
			}
		}()
	}
	wg.Wait()
	var tasks []ability_task_model.AsyncTask
	if err := db.Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 24 || len(seen) != 24 {
		t.Fatalf("tasks=%d executions=%d", len(tasks), len(seen))
	}
	for _, task := range tasks {
		if seen[task.TaskID] != 1 || task.RunStatus != RunStatusDone || task.ProcessStatus != ProcessStatusSuccess {
			t.Fatalf("duplicate/incomplete task %s: %d %s/%s", task.TaskID, seen[task.TaskID], task.RunStatus, task.ProcessStatus)
		}
	}
	t.Log("SQLite: 24 durable tasks, 8 claimers, each Execute exactly once")
}

func TestSQLiteRecoveryAfterReopen(t *testing.T) {
	db := setupTaskSQLite(t)
	accepted, err := AcceptAsyncTask(context.Background(), "recovery-probe", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	task, err := claimQueuedTask(context.Background())
	if err != nil || task == nil {
		t.Fatalf("claim: %v", err)
	}
	// Simulate a dead daemon after claim; no Execute or CLI was started in this test.
	unmarkTaskInFlight(id)
	if err = storage.CloseStorage(); err != nil {
		t.Fatal(err)
	}
	if err = storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err = RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	api_supported_methods.SupportedMethodsSetup()
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{Name: "recovery-probe", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}, Execute: func(context.Context, interface{}) (interface{}, error) { return "recovered", nil }})
	if ok, err := ExecuteNextQueuedTask(context.Background()); err != nil || !ok {
		t.Fatalf("execute recovered: %t %v", ok, err)
	}
	db, _ = storage.GormDB(context.Background())
	var restored ability_task_model.AsyncTask
	if err = db.Where("task_id = ?", id).Take(&restored).Error; err != nil {
		t.Fatal(err)
	}
	if restored.ProcessStatus != ProcessStatusSuccess {
		t.Fatal(fmt.Sprintf("not recovered: %+v", restored))
	}
	t.Log("SQLite close/reopen: claimed orphan recovered and executed; this is DB recovery, not SIGKILL process-tree recovery")
}

func TestSQLiteRecoveryHonorsPersistedCancellation(t *testing.T) {
	db := setupTaskSQLite(t)
	accepted, err := AcceptAsyncTask(context.Background(), "cancel-probe", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	if err = db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).Updates(map[string]interface{}{"run_status": RunStatusRunning, "cancel_requested": true}).Error; err != nil {
		t.Fatal(err)
	}
	if err = RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var task ability_task_model.AsyncTask
	if err = db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusDone || task.ProcessStatus != ProcessStatusCancelled {
		t.Fatalf("cancel lost: %+v", task)
	}
}

func registerProbe(t *testing.T, name string, execute func(context.Context, interface{}) (interface{}, error)) {
	t.Helper()
	api_supported_methods.SupportedMethodsSetup()
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        name,
		Async:       true,
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
		Execute:     execute,
	})
}

func TestTaskAcceptsAndExecutesOnSqlite(t *testing.T) {
	db := setupTaskSQLite(t)
	registerProbe(t, "sqlite-probe", func(context.Context, interface{}) (interface{}, error) {
		return map[string]interface{}{"ok": true}, nil
	})
	accepted, err := AcceptAsyncTask(context.Background(), "sqlite-probe", map[string]interface{}{"n": 1})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	ok, err := ExecuteNextQueuedTask(context.Background())
	if err != nil || !ok {
		t.Fatalf("execute: ok=%t err=%v", ok, err)
	}
	var task ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusDone || task.ProcessStatus != ProcessStatusSuccess || task.ExecutionCount != 1 {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestClientDisconnectDoesNotFailPersistedTask(t *testing.T) {
	db := setupTaskSQLite(t)
	requestCtx, cancel := context.WithCancel(context.Background())
	accepted, err := AcceptAsyncTask(requestCtx, "disconnect-probe", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	cancel()
	var task ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusQueued || task.ProcessStatus != "" {
		t.Fatalf("disconnect changed task state: %+v", task)
	}
	registerProbe(t, "disconnect-probe", func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	})
	if ok, execErr := ExecuteNextQueuedTask(context.Background()); execErr != nil || !ok {
		t.Fatalf("reconnect path must query/execute the existing task, not require resubmit: %t %v", ok, execErr)
	}
	if err := db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.ProcessStatus != ProcessStatusSuccess {
		t.Fatalf("existing task was not completed after reconnect-style query/execute: %+v", task)
	}
}

func TestDefaultTimeoutDoesNotExpireQueuedTask(t *testing.T) {
	db := setupTaskSQLite(t)
	if _, err := AcceptAsyncTask(context.Background(), "timeout-off", map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ability_task_model.AsyncTask{}).Where("method = ?", "timeout-off").
		Update("created_at", time.Now().UTC().Add(-72*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := ExpireTimedOutTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var task ability_task_model.AsyncTask
	if err := db.Where("method = ?", "timeout-off").Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusQueued {
		t.Fatalf("timeout disabled by default must not expire tasks, even after 72h: %+v", task)
	}
}

func TestLifecycleTimeoutDisabledIgnoresSeconds(t *testing.T) {
	db := setupTaskSQLite(t)
	policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{TaskLifecycleTimeoutEnabled: false, TaskLifecycleTimeoutSeconds: 2}
	accepted, err := AcceptAsyncTask(context.Background(), "timeout-disabled", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	if err := db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).
		Update("created_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := ExpireTimedOutTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var task ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusQueued {
		t.Fatalf("enabled=false must ignore seconds and not expire: %+v", task)
	}
}

func TestLifecycleTimeoutExpiresQueuedTaskFromCreatedAt(t *testing.T) {
	db := setupTaskSQLite(t)
	policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{TaskLifecycleTimeoutEnabled: true, TaskLifecycleTimeoutSeconds: 2}
	accepted, err := AcceptAsyncTask(context.Background(), "timeout-on", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	if err := db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).
		Update("created_at", time.Now().UTC().Add(-3*time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if err := ExpireTimedOutTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var task ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusDone || task.ProcessStatus != ProcessStatusSystemError || task.ErrorMessage != "task lifecycle timed out" {
		t.Fatalf("queued task should expire from created_at: %+v", task)
	}
	ok, execErr := ExecuteNextQueuedTask(context.Background())
	if execErr != nil || ok {
		t.Fatalf("expired task must not be claimed: ok=%t err=%v", ok, execErr)
	}
}

func TestRetryLimitZeroDoesNotRerunWhenEnabled(t *testing.T) {
	db := setupTaskSQLite(t)
	policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{
		TaskExecuteRetryLimitEnabled: true,
		TaskExecuteRetryLimitCount:   0,
	}
	accepted, err := AcceptAsyncTask(context.Background(), "retry-zero", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	task, err := claimQueuedTask(context.Background())
	if err != nil || task == nil {
		t.Fatalf("claim: %v", err)
	}
	unmarkTaskInFlight(id)
	if err := RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var restored ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&restored).Error; err != nil {
		t.Fatal(err)
	}
	if restored.RunStatus != RunStatusDone || restored.ProcessStatus != ProcessStatusSystemError || restored.ErrorMessage != "task execute retry limit reached" {
		t.Fatalf("enabled limit 0 must not rerun: %+v", restored)
	}
}

func TestRetryLimitDisabledKeepsOrphanRecovery(t *testing.T) {
	db := setupTaskSQLite(t)
	accepted, err := AcceptAsyncTask(context.Background(), "retry-off", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	if _, err := claimQueuedTask(context.Background()); err != nil {
		t.Fatal(err)
	}
	unmarkTaskInFlight(id)
	if err := RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var restored ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&restored).Error; err != nil {
		t.Fatal(err)
	}
	if restored.RunStatus != RunStatusQueued {
		t.Fatalf("default (limit not enabled) must keep orphan recovery: %+v", restored)
	}
	registerProbe(t, "retry-off", func(context.Context, interface{}) (interface{}, error) { return "recovered", nil })
	if ok, execErr := ExecuteNextQueuedTask(context.Background()); execErr != nil || !ok {
		t.Fatalf("recover execute: %t %v", ok, execErr)
	}
}

func TestUncertainWritebackIsNotRerun(t *testing.T) {
	db := setupTaskSQLite(t)
	accepted, err := AcceptAsyncTask(context.Background(), "writeback", map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	id := accepted.(map[string]interface{})["task_id"].(string)
	if _, err := claimQueuedTask(context.Background()); err != nil {
		t.Fatal(err)
	}
	unmarkTaskInFlight(id)
	if err := db.Model(&ability_task_model.AsyncTask{}).Where("task_id = ?", id).
		Update("writeback_pending", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := RequeueOrphanedTasks(context.Background()); err != nil {
		t.Fatal(err)
	}
	var task ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", id).Take(&task).Error; err != nil {
		t.Fatal(err)
	}
	if task.RunStatus != RunStatusDone || task.ProcessStatus != ProcessStatusSystemError || task.WritebackPending || task.ErrorMessage != "execution result writeback was incomplete; not rerun" {
		t.Fatalf("uncertain writeback must not be rerun: %+v", task)
	}
}

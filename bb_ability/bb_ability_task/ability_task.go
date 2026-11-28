// Package ability_task manages durable asynchronous task state:
// 受理（APIExecuter Async 一次性接入的唯一收口）、Worker 执行、重启恢复与任务查询。
// 状态三字段核心 run_status/process_status/progress，枚举与 model 列 comment、契约文档三处同步。
package bb_ability_task

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task/bb_ability_task_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_cli_proc"
	"github.com/0xdevelop/bioBox/bb_common"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_policy/bb_policy_config"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 程序级运行状态（与 model 列 comment、docs/ability_task.md 同步维护）。
const (
	RunStatusQueued  = "queued"
	RunStatusRunning = "running"
	RunStatusDone    = "done"
)

// 业务级处理结局（空字符串 = 进行中；与 model 列 comment、docs/ability_task.md 同步维护）。
const (
	ProcessStatusSuccess     = "success"
	ProcessStatusFailed      = "failed"
	ProcessStatusCancelled   = "cancelled"
	ProcessStatusSystemError = "system_error"
)

const (
	MethodTaskGet    = "task.get"
	MethodTaskList   = "task.list"
	MethodTaskCancel = "task.cancel"

	// task.list 返回条数上限。
	taskListLimit = 50
)

var ErrTaskNotActive = errors.New("task is not active")

// inFlightTasks tracks owned tasks for recovery/cancellation. Claim and recovery share
// taskLifecycleMutex: ownership is registered before that lock is released.
var (
	inFlightTasksMutex sync.Mutex
	inFlightTasks      = map[string]context.CancelFunc{}
	// Serializes claim/recovery/cancel/finish lifecycle transitions within this daemon.
	taskLifecycleMutex sync.Mutex
)

func markTaskInFlight(taskID string) {
	inFlightTasksMutex.Lock()
	defer inFlightTasksMutex.Unlock()
	inFlightTasks[taskID] = nil
}

func unmarkTaskInFlight(taskID string) {
	inFlightTasksMutex.Lock()
	defer inFlightTasksMutex.Unlock()
	delete(inFlightTasks, taskID)
}

func inFlightTaskIDs() []string {
	inFlightTasksMutex.Lock()
	defer inFlightTasksMutex.Unlock()
	taskIDs := make([]string, 0, len(inFlightTasks))
	for taskID := range inFlightTasks {
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

func LoadAPIMethods() {
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodTaskGet,
		Description: "查询我的异步任务状态、步进（steps/current_step）与结果",
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"task_id": bb_api_auth_common.StringSchema("", 36, 36),
			},
			"task_id",
		),
		Execute: TaskGet,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodTaskList,
		Description: "列出我的异步任务（新→旧，最多 50 条）",
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{},
		),
		Execute: TaskList,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodTaskCancel,
		Description: "取消我的排队或执行中任务（执行中返回 cancelling，进程组退出后落终态）",
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"task_id": bb_api_auth_common.StringSchema("", 36, 36),
			},
			"task_id",
		),
		Execute: TaskCancel,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodTaskArtifacts,
		Description: "列出我的已完成任务在隔离作业目录内的产物（相对路径，不含原始输入）",
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"task_id": bb_api_auth_common.StringSchema("", 36, 36),
			},
			"task_id",
		),
		Execute: TaskArtifacts,
	})
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodTaskArtifact,
		Description: "读取我的已完成任务的单个产物内容（base64，仅限作业目录内允许的文件）",
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"task_id": bb_api_auth_common.StringSchema("", 36, 36),
				"path":    bb_api_auth_common.StringSchema("", 1, artifactMaxPathLength),
			},
			"task_id", "path",
		),
		Execute: TaskArtifact,
	})
}

// AcceptAsyncTask 是 Async 方法的统一受理：写入持久化排队区并返回 task_id。
// 只由 APIExecuter 调用（AGENTS 预留的一次性接入）；身份取门禁 context 快照，无身份任务属主为 NULL。
func AcceptAsyncTask(ctx context.Context, methodName string, abilityParams interface{}) (interface{}, error) {
	inputJSON, err := json.Marshal(abilityParams)
	if err != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	var userID *string
	if user, _, identityErr := bb_api_auth_session.AuthenticatedUser(ctx); identityErr == nil {
		userID = &user.UserID
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}

	task := &bb_ability_task_model.AsyncTask{
		Model:        gorm.Model{CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()},
		TaskID:       uuid.NewString(),
		UserID:       userID,
		Method:       methodName,
		RunStatus:    RunStatusQueued,
		InputPayload: string(inputJSON),
	}
	// 任务行与引擎 accepted 阶段同事务写入：返回 task_id 时步进起点已持久化。
	if err = db.Transaction(func(tx *gorm.DB) error {
		if createErr := tx.Create(task).Error; createErr != nil {
			return createErr
		}
		return appendTaskStep(tx, task.TaskID, 0, EngineStepAccepted, StepKindEngine, RunStatusQueued, "", task.CreatedAt.UTC())
	}); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"task_id":    task.TaskID,
		"run_status": task.RunStatus,
	}, nil
}

// AsyncTaskInfo 是 Worker 注入业务 Execute context 的任务身份快照；
// 与门禁在线身份（AuthenticatedUser）语义分离：异步执行不要求受理时的 session 仍存活。
type AsyncTaskInfo struct {
	TaskID         string
	UserID         *string
	ExecutionCount int
}

type asyncTaskInfoContextKey struct{}

// AsyncTaskInfoFromContext 供异步业务 Execute 读取当前任务身份快照。
func AsyncTaskInfoFromContext(ctx context.Context) (*AsyncTaskInfo, bool) {
	info, ok := ctx.Value(asyncTaskInfoContextKey{}).(*AsyncTaskInfo)
	return info, ok && info != nil
}

// ExecuteNextQueuedTask 从持久化排队区原子认领一条任务并执行。
// 本方法是单次 Worker，由 Policy 按空闲名额异步调用；无排队任务时返回 false。
func ExecuteNextQueuedTask(ctx context.Context) (bool, error) {
	task, err := claimQueuedTask(ctx)
	if err != nil || task == nil {
		return false, err
	}
	executeClaimedTask(ctx, task)
	return true, nil
}

// claimQueuedTask uses the primary DB. SQLite claims with one atomic UPDATE ... RETURNING;
// row-locking engines retain SKIP LOCKED. Recovery is single-daemon, not an HA lease.
func claimQueuedTask(ctx context.Context) (*bb_ability_task_model.AsyncTask, error) {
	taskLifecycleMutex.Lock()
	defer taskLifecycleMutex.Unlock()
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	if timeout := bb_policy_config.CurrentTaskLifecycleTimeout(); timeout > 0 {
		if expireErr := expireTimedOutQueuedTasks(ctx, db, timeout); expireErr != nil {
			return nil, expireErr
		}
	}
	task := &bb_ability_task_model.AsyncTask{}
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"run_status":      RunStatusRunning,
		"started_at":      &now,
		"execution_count": gorm.Expr("execution_count + 1"),
	}
	if db.Dialector.Name() == "sqlite" {
		claimed := false
		err = db.Transaction(func(tx *gorm.DB) error {
			candidate := tx.Model(&bb_ability_task_model.AsyncTask{}).Select("id").
				Where("run_status = ?", RunStatusQueued).Order("id ASC").Limit(1)
			result := tx.Model(task).Clauses(clause.Returning{}).
				Where("id = (?) AND run_status = ?", candidate, RunStatusQueued).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return nil
			}
			// in-flight 登记先于提交；claimed 阶段与认领同事务，attempt 即 RETURNING 回来的 execution_count
			markTaskInFlight(task.TaskID)
			claimed = true
			return appendTaskStep(tx, task.TaskID, task.ExecutionCount, EngineStepClaimed, StepKindEngine, RunStatusRunning, "", now)
		})
		if err != nil || !claimed {
			if claimed {
				unmarkTaskInFlight(task.TaskID)
			}
			return nil, err
		}
		return task, nil
	}
	claimed := false
	err = db.Transaction(func(tx *gorm.DB) error {
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("run_status = ?", RunStatusQueued).
			Order("id ASC").
			Take(task).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if updateErr := tx.Model(task).Updates(updates).Error; updateErr != nil {
			return updateErr
		}
		// gorm.Expr 自增不回填结构体，按同一条 UPDATE 的语义同步内存值
		task.ExecutionCount++
		// in-flight 登记必须先于事务提交：DB running 可见时本进程执行记录已存在，孤儿侦测无误判窗口
		markTaskInFlight(task.TaskID)
		claimed = true
		return appendTaskStep(tx, task.TaskID, task.ExecutionCount, EngineStepClaimed, StepKindEngine, RunStatusRunning, "", now)
	})
	if err != nil || !claimed {
		if claimed {
			unmarkTaskInFlight(task.TaskID)
		}
		return nil, err
	}
	return task, nil
}

func executeTimeout() time.Duration {
	if bb_config.GlobalConfig == nil || bb_config.GlobalConfig.CliCfg == nil {
		return 0
	}
	return bb_config.GlobalConfig.CliCfg.CurrentExecuteTimeout()
}

// executeClaimedTask 调用注册表里同一个 Execute 并落终态；panic、方法下线、序列化故障或 Execute 超时归 system_error。
func executeClaimedTask(ctx context.Context, task *bb_ability_task_model.AsyncTask) {
	defer unmarkTaskInFlight(task.TaskID)
	// Cancellation, optional Execute timeout, and optional lifecycle deadline all reach the existing CLI process-group runner.
	var runCtx context.Context
	var cancel context.CancelFunc
	if timeout := executeTimeout(); timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		runCtx, cancel = context.WithCancel(ctx)
	}
	lifecycleTimeout := bb_policy_config.CurrentTaskLifecycleTimeout()
	if lifecycleTimeout > 0 {
		deadline := task.CreatedAt.UTC().Add(lifecycleTimeout)
		if !time.Now().UTC().Before(deadline) {
			cancel()
			finishTask(context.WithoutCancel(ctx), task.TaskID, ProcessStatusSystemError, "", 0, "task lifecycle timed out")
			return
		}
		var lifeCancel context.CancelFunc
		runCtx, lifeCancel = context.WithDeadline(runCtx, deadline)
		innerCancel := cancel
		cancel = func() {
			lifeCancel()
			innerCancel()
		}
	}
	defer cancel()
	inFlightTasksMutex.Lock()
	inFlightTasks[task.TaskID] = cancel
	inFlightTasksMutex.Unlock()
	// A cancellation can arrive after claim but before this cancel function is installed.
	db, dbErr := bb_storage.GormDB(context.WithoutCancel(ctx))
	if dbErr != nil {
		gtbox_log.LogErrorf("async task storage unavailable before Execute: task_id=%s: %v", task.TaskID, dbErr)
		return
	}
	if dbErr == nil {
		var current bb_ability_task_model.AsyncTask
		if err := db.Where("task_id = ?", task.TaskID).Take(&current).Error; err != nil {
			finishTask(context.WithoutCancel(ctx), task.TaskID, ProcessStatusSystemError, "", 0, err.Error())
			return
		} else if current.CancelRequested {
			cancel()
		}
	}
	taskCtx := context.WithValue(
		runCtx,
		asyncTaskInfoContextKey{},
		&AsyncTaskInfo{TaskID: task.TaskID, UserID: task.UserID, ExecutionCount: task.ExecutionCount},
	)

	supportedMethod, ok := bb_api_supported_methods.Method(task.Method)
	if !ok {
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, "async method is no longer registered: "+task.Method)
		return
	}
	var input interface{}
	if err := json.Unmarshal([]byte(task.InputPayload), &input); err != nil {
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, "task input payload is not valid JSON")
		return
	}

	value, execErr := callExecuteRecovered(taskCtx, supportedMethod, task, input)
	resultJSON, marshalErr := marshalTaskResult(value)

	// A job that already succeeded is kept as success even if the timer fires in the same window.
	if execErr != nil || marshalErr != nil {
		if errors.Is(taskCtx.Err(), context.DeadlineExceeded) {
			message := "task execution timed out"
			if lifecycleTimeout > 0 && !time.Now().UTC().Before(task.CreatedAt.UTC().Add(lifecycleTimeout)) {
				message = "task lifecycle timed out"
			}
			finishTask(context.WithoutCancel(taskCtx), task.TaskID, ProcessStatusSystemError, resultJSON, 0, message)
			return
		}
		if taskCtx.Err() != nil {
			finishTask(context.WithoutCancel(taskCtx), task.TaskID, ProcessStatusCancelled, resultJSON, 0, "task execution cancelled")
			return
		}
	}
	if execErr != nil {
		if businessErr, isBusiness := bb_api_error_code.As(execErr); isBusiness {
			finishTask(taskCtx, task.TaskID, ProcessStatusFailed, resultJSON, businessErr.Code, businessErr.Message)
			return
		}
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, resultJSON, 0, execErr.Error())
		return
	}
	if marshalErr != nil {
		finishTask(taskCtx, task.TaskID, ProcessStatusSystemError, "", 0, "task result is not serializable")
		return
	}
	finishTask(taskCtx, task.TaskID, ProcessStatusSuccess, resultJSON, 0, "")
}

// callExecuteRecovered 执行业务函数；panic 经统一封装 common.PanicHandler
// 输出富日志并转为 err，任务据此落 system_error，Worker 循环不中断。
func callExecuteRecovered(
	ctx context.Context,
	supportedMethod *bb_api_supported_methods.SupportedMethod,
	task *bb_ability_task_model.AsyncTask,
	input interface{},
) (value interface{}, err error) {
	defer bb_common.PanicHandler(&err, "async task execute: method="+task.Method+" task_id="+task.TaskID)
	return supportedMethod.Execute(ctx, input)
}

// finishTask 把任务落到唯一终态 done + 对应业务结局；running 行由当前 worker 独占，直接条件更新。
func finishTask(ctx context.Context, taskID string, processStatus string, resultPayload string, errorCode int, errorMessage string) {
	taskLifecycleMutex.Lock()
	defer taskLifecycleMutex.Unlock()
	ctx = context.WithoutCancel(ctx)
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		gtbox_log.LogErrorf("async task finish failed: task_id=%s: %v", taskID, err)
		return
	}
	var current bb_ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", taskID).Take(&current).Error; err != nil {
		gtbox_log.LogErrorf("async task finish read failed: task_id=%s: %v", taskID, err)
		return
	}
	if current.CancelRequested {
		processStatus, errorCode, errorMessage = ProcessStatusCancelled, 0, "task execution cancelled"
	}
	// 先标记写回进行中：若随后的终态更新失败或进程在写回窗口崩溃，不得把该任务当孤儿重跑。
	pending := db.Model(&bb_ability_task_model.AsyncTask{}).
		Where("task_id = ? AND run_status = ?", taskID, RunStatusRunning).
		Update("writeback_pending", true)
	if pending.Error != nil {
		gtbox_log.LogErrorf("async task finish mark failed: task_id=%s: %v", taskID, pending.Error)
		return
	}
	if pending.RowsAffected != 1 {
		gtbox_log.LogWarnf("async task finish skipped, row not in running state: task_id=%s", taskID)
		return
	}
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"run_status":        RunStatusDone,
		"process_status":    processStatus,
		"completed_at":      &now,
		"error_code":        errorCode,
		"error_message":     errorMessage,
		"writeback_pending": false,
	}
	if processStatus == ProcessStatusSuccess {
		updates["progress"] = 100.0
	}
	if resultPayload != "" {
		updates["result_payload"] = resultPayload
	}
	// 终态与 finished 阶段同事务：任一失败整体回滚，writeback_pending 保持 true 交由恢复落 system_error。
	finished := false
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&bb_ability_task_model.AsyncTask{}).
			Where("task_id = ? AND run_status = ?", taskID, RunStatusRunning).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		finished = true
		return appendFinishedStep(tx, taskID, errorMessage, now)
	})
	if err != nil {
		gtbox_log.LogErrorf("async task finish failed: task_id=%s: %v", taskID, err)
		return
	}
	if !finished {
		gtbox_log.LogWarnf("async task finish skipped, row not in running state: task_id=%s", taskID)
	}
}

// RequeueOrphanedTasks 侦测未尽之事：把 DB 中 running 但不在本进程 in-flight 执行集合的孤儿任务
// 重置回 queued 拉起来继续。进程重启后第一轮侦测即捞回全部死进程遗留（此时 in-flight 为空）；
// 运行中意外产生的孤儿同样每轮自愈。重跑兜底依赖异步 Execute 幂等（契约明记，责任归业务函数）。
// 由 policy 周期维护大循环调用。
func RequeueOrphanedTasks(ctx context.Context) error {
	taskLifecycleMutex.Lock()
	defer taskLifecycleMutex.Unlock()
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return err
	}
	query := db.Model(&bb_ability_task_model.AsyncTask{}).
		Where("run_status = ?", RunStatusRunning)
	if inFlight := inFlightTaskIDs(); len(inFlight) > 0 {
		query = query.Where("task_id NOT IN ?", inFlight)
	}

	var orphans []bb_ability_task_model.AsyncTask
	if findErr := query.Find(&orphans).Error; findErr != nil {
		return findErr
	}
	if len(orphans) == 0 {
		return nil
	}

	requeued := 0
	for _, orphan := range orphans {
		if leftoverAlive, pgid := leftoverProcessGroup(orphan.TaskID); leftoverAlive {
			if orphan.CancelRequested {
				if sigErr := bb_cli_proc.SignalTermProcessGroup(pgid); sigErr != nil {
					gtbox_log.LogWarnf("orphan cancel signal failed: task_id=%s pgid=%d err=%v", orphan.TaskID, pgid, sigErr)
				}
			}
			// Leave the still-running job in place. Do not requeue or force-kill.
			continue
		}
		if orphan.CancelRequested {
			now := time.Now().UTC()
			if _, updErr := markTaskDoneWithStep(db, orphan.TaskID,
				"task_id = ? AND run_status = ?", []interface{}{orphan.TaskID, RunStatusRunning},
				map[string]interface{}{"run_status": RunStatusDone, "process_status": ProcessStatusCancelled, "completed_at": &now,
					"error_message": "task execution cancelled", "writeback_pending": false}, now); updErr != nil {
				return updErr
			}
			continue
		}
		if orphan.WritebackPending {
			now := time.Now().UTC()
			if _, updErr := markTaskDoneWithStep(db, orphan.TaskID,
				"task_id = ? AND run_status = ? AND writeback_pending = ?", []interface{}{orphan.TaskID, RunStatusRunning, true},
				map[string]interface{}{
					"run_status":        RunStatusDone,
					"process_status":    ProcessStatusSystemError,
					"completed_at":      &now,
					"error_message":     "execution result writeback was incomplete; not rerun",
					"writeback_pending": false,
				}, now); updErr != nil {
				return updErr
			}
			gtbox_log.LogWarnf("async task writeback uncertain, not rerun: task_id=%s", orphan.TaskID)
			continue
		}
		if timeout := bb_policy_config.CurrentTaskLifecycleTimeout(); timeout > 0 && !orphan.CreatedAt.UTC().Add(timeout).After(time.Now().UTC()) {
			now := time.Now().UTC()
			if _, updErr := markTaskDoneWithStep(db, orphan.TaskID,
				"task_id = ? AND run_status = ?", []interface{}{orphan.TaskID, RunStatusRunning},
				map[string]interface{}{
					"run_status":     RunStatusDone,
					"process_status": ProcessStatusSystemError,
					"completed_at":   &now,
					"error_message":  "task lifecycle timed out",
				}, now); updErr != nil {
				return updErr
			}
			continue
		}
		if bb_policy_config.TaskExecuteRetryLimitEnforced() && orphan.ExecutionCount > bb_policy_config.CurrentTaskExecuteRetryLimitCount() {
			now := time.Now().UTC()
			if _, updErr := markTaskDoneWithStep(db, orphan.TaskID,
				"task_id = ? AND run_status = ?", []interface{}{orphan.TaskID, RunStatusRunning},
				map[string]interface{}{
					"run_status":     RunStatusDone,
					"process_status": ProcessStatusSystemError,
					"completed_at":   &now,
					"error_message":  "task execute retry limit reached",
				}, now); updErr != nil {
				return updErr
			}
			gtbox_log.LogWarnf("async task retry limit reached, not rerun: task_id=%s execution_count=%d", orphan.TaskID, orphan.ExecutionCount)
			continue
		}
		// 重排队与 requeued 阶段同事务：旧轮次阶段只被关闭（completed_at），历史不删；attempt 记被孤儿化的轮次。
		now := time.Now().UTC()
		requeuedOne := false
		if txErr := db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&bb_ability_task_model.AsyncTask{}).
				Where("task_id = ? AND run_status = ? AND writeback_pending = ? AND cancel_requested = ?", orphan.TaskID, RunStatusRunning, false, false).
				Updates(map[string]interface{}{
					"run_status": RunStatusQueued,
					"started_at": nil,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return nil
			}
			requeuedOne = true
			return appendTaskStep(tx, orphan.TaskID, orphan.ExecutionCount, EngineStepRequeued, StepKindEngine, RunStatusQueued,
				"orphaned running task requeued", now)
		}); txErr != nil {
			return txErr
		}
		if requeuedOne {
			requeued++
		}
	}
	if requeued > 0 {
		gtbox_log.LogInfof("async task requeue: %d orphaned task(s) back to queued", requeued)
	}
	return nil
}

func leftoverProcessGroup(taskID string) (bool, int) {
	if strings.TrimSpace(taskID) == "" {
		return false, 0
	}
	workRoot, err := configuredWorkRoot()
	if err != nil {
		return false, 0
	}
	jobDir := filepath.Join(workRoot, taskID)
	pgid, err := bb_cli_proc.ReadJobProcessGroup(jobDir)
	if err != nil {
		return false, 0
	}
	if !bb_cli_proc.ProcessGroupAliveInDir(pgid, jobDir) {
		return false, pgid
	}
	return true, pgid
}

// ExpireTimedOutTasks 把已超过整体生命周期（从 created_at 起算）的排队任务落 system_error。
// task_lifecycle_timeout_enabled=false 时为 no-op（忽略 seconds），不启用任何默认期限。
func ExpireTimedOutTasks(ctx context.Context) error {
	timeout := bb_policy_config.CurrentTaskLifecycleTimeout()
	if timeout <= 0 {
		return nil
	}
	taskLifecycleMutex.Lock()
	defer taskLifecycleMutex.Unlock()
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return err
	}
	return expireTimedOutQueuedTasks(ctx, db, timeout)
}

func expireTimedOutQueuedTasks(_ context.Context, db *gorm.DB, timeout time.Duration) error {
	if timeout <= 0 {
		return nil
	}
	now := time.Now().UTC()
	deadline := now.Add(-timeout)
	var expiredIDs []string
	if err := db.Model(&bb_ability_task_model.AsyncTask{}).
		Where("run_status = ? AND created_at <= ?", RunStatusQueued, deadline).
		Pluck("task_id", &expiredIDs).Error; err != nil {
		return err
	}
	// 逐条条件置终态并追加 finished 阶段；并发下已被认领/取消的行条件不命中，不写阶段。
	for _, taskID := range expiredIDs {
		if _, err := markTaskDoneWithStep(db, taskID,
			"task_id = ? AND run_status = ? AND created_at <= ?", []interface{}{taskID, RunStatusQueued, deadline},
			map[string]interface{}{
				"run_status":     RunStatusDone,
				"process_status": ProcessStatusSystemError,
				"completed_at":   &now,
				"error_message":  "task lifecycle timed out",
			}, now); err != nil {
			return err
		}
	}
	return nil
}

// UpdateProgress 供异步业务 Execute 上报量化进度；仅活跃（非 done）任务可更新。
func UpdateProgress(ctx context.Context, taskID string, progress float64, message string) error {
	if taskID == "" || progress < 0 || progress > 100 {
		return errors.New("invalid task progress")
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		task := &bb_ability_task_model.AsyncTask{}
		findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ?", taskID).Take(task).Error
		if findErr != nil {
			return findErr
		}
		if task.RunStatus == RunStatusDone {
			return ErrTaskNotActive
		}
		return tx.Model(task).Updates(map[string]interface{}{
			"progress":         progress,
			"progress_message": message,
		}).Error
	})
}

func TaskGet(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	taskID, taskIDOK := bb_api_auth_common.RequiredString(params, "task_id")
	if !taskIDOK || uuid.Validate(taskID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	user, _, err := bb_api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}

	item, err := ownedTaskWithSteps(db, taskID, user.UserID)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"task": item}, nil
}

func TaskList(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	user, _, err := bb_api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}

	tasks := make([]*bb_ability_task_model.AsyncTask, 0)
	if listErr := db.Where("user_id = ?", user.UserID).
		Order("id DESC").
		Limit(taskListLimit).
		Find(&tasks).Error; listErr != nil {
		return nil, listErr
	}
	taskResults := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		taskResults = append(taskResults, taskResult(task, false))
	}
	return map[string]interface{}{"tasks": taskResults}, nil
}

// TaskCancel persists cancellation before signalling the owned running task.
// A running task is not done until Execute and process-group cleanup return.
func TaskCancel(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	taskID, ok := bb_api_auth_common.RequiredString(params, "task_id")
	if !ok || uuid.Validate(taskID) != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	user, _, err := bb_api_auth_session.AuthenticatedUser(ctx)
	if err != nil {
		return nil, err
	}
	taskLifecycleMutex.Lock()
	defer taskLifecycleMutex.Unlock()
	db, err := bb_storage.GormDB(ctx)
	if err != nil {
		return nil, err
	}
	var task bb_ability_task_model.AsyncTask
	if err = db.Where("task_id = ? AND user_id = ?", taskID, user.UserID).Take(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, bb_api_error_code.ErrInvalidArguments
		}
		return nil, err
	}
	if task.RunStatus == RunStatusDone {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	updates := map[string]interface{}{"cancel_requested": true}
	queued := task.RunStatus == RunStatusQueued
	if queued {
		now := time.Now().UTC()
		updates["run_status"], updates["process_status"], updates["completed_at"] = RunStatusDone, ProcessStatusCancelled, &now
	}
	changed := false
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&bb_ability_task_model.AsyncTask{}).Where("task_id = ? AND run_status = ?", taskID, task.RunStatus).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return bb_api_error_code.ErrInvalidArguments
		}
		changed = true
		if queued {
			return appendFinishedStep(tx, taskID, "", *updates["completed_at"].(*time.Time))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	inFlightTasksMutex.Lock()
	cancel := inFlightTasks[taskID]
	inFlightTasksMutex.Unlock()
	if cancel != nil {
		cancel()
	}
	return map[string]interface{}{"task_id": taskID, "cancelled": queued, "cancelling": !queued}, nil
}

func taskResult(task *bb_ability_task_model.AsyncTask, includeResult bool) map[string]interface{} {
	item := map[string]interface{}{
		"task_id":          task.TaskID,
		"method":           task.Method,
		"run_status":       task.RunStatus,
		"process_status":   task.ProcessStatus,
		"progress":         task.Progress,
		"progress_message": task.ProgressMessage,
		"cancel_requested": task.CancelRequested,
		"error_code":       task.ErrorCode,
		"error_message":    task.ErrorMessage,
		"created_at":       task.CreatedAt.UTC().Format(time.RFC3339),
	}
	if task.StartedAt != nil {
		item["started_at"] = task.StartedAt.UTC().Format(time.RFC3339)
	}
	if task.CompletedAt != nil {
		item["completed_at"] = task.CompletedAt.UTC().Format(time.RFC3339)
	}
	if includeResult && task.RunStatus == RunStatusDone && task.ResultPayload != "" {
		var resultValue interface{}
		if err := json.Unmarshal([]byte(task.ResultPayload), &resultValue); err == nil {
			item["result"] = resultValue
		}
	}
	return item
}

func marshalTaskResult(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func markTaskDoneWithStep(db *gorm.DB, taskID string, query string, args []interface{}, updates map[string]interface{}, now time.Time) (bool, error) {
	errorMessage, _ := updates["error_message"].(string)
	done := false
	err := db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&bb_ability_task_model.AsyncTask{}).Where(query, args...).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		done = true
		return appendFinishedStep(tx, taskID, errorMessage, now)
	})
	return done, err
}

func ownedTaskWithSteps(db *gorm.DB, taskID string, userID string) (map[string]interface{}, error) {
	task := &bb_ability_task_model.AsyncTask{}
	var steps []map[string]interface{}
	txErr := db.Transaction(func(tx *gorm.DB) error {
		if findErr := tx.Where("task_id = ? AND user_id = ?", taskID, userID).Take(task).Error; findErr != nil {
			return findErr
		}
		var stepErr error
		steps, stepErr = loadTaskSteps(tx, taskID)
		return stepErr
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if errors.Is(txErr, gorm.ErrRecordNotFound) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if txErr != nil {
		return nil, txErr
	}
	item := taskResult(task, true)
	item["steps"] = steps
	item["execution_count"] = task.ExecutionCount
	item["current_business_step"] = nil
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i]["step_kind"] == StepKindBusiness && steps[i]["attempt"] == task.ExecutionCount {
			item["current_business_step"] = steps[i]
			break
		}
	}
	if len(steps) > 0 {
		item["current_step"] = steps[len(steps)-1]
	} else {
		item["current_step"] = nil
	}
	return item, nil
}

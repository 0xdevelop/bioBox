package ability_pxdesign

import (
	"context"
	"github.com/0xdevelop/bioBox/ability/ability_task"
)

// 本业务的阶段及状态字符串枚举，只有本业务定义含义；通用 task 引擎仅注册/校验/承载。
const (
	StepInputPreparation     = "input_preparation"
	StepCLIExecution         = "cli_execution"
	StepArtifactCollection   = "artifact_collection"
	StatusArgumentsValidated = "arguments_validated"
	StatusInputStaged        = "input_staged"
	StatusPreparationFailed  = "preparation_failed"
	StatusProcessExited      = "process_exited"
	StatusProcessFailed      = "process_failed"
	StatusProcessCancelled   = "process_cancelled"
	StatusLaunchFailed       = "launch_failed"
	StatusInspected          = "inspected"
	StatusCIFFilesFound      = "cif_files_found"
	StatusNoCIFFiles         = "no_cif_files"
	StatusERRArtifactsFound  = "err_artifacts_found"
	StatusInspectionFailed   = "inspection_failed"
)

// 直接调用 Execute 无持久任务身份，保持独立业务函数用法、不伪造任务；
// 正式 Async Worker 路径必须持久化步进，写入错误原样返回，不吞错。
func beginBusinessStep(ctx context.Context, step, message string) error {
	if _, ok := ability_task.AsyncTaskInfoFromContext(ctx); !ok {
		return nil
	}
	return ability_task.BeginStep(ctx, step, message)
}
func reportBusinessStatus(ctx context.Context, status, message string) error {
	if _, ok := ability_task.AsyncTaskInfoFromContext(ctx); !ok {
		return nil
	}
	return ability_task.ReportStepBusinessStatus(ctx, status, message)
}

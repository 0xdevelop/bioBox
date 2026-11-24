// Package ability_pxdesign 注册固定功能入口 pxdesign（MCP tools/call 的 name）。
// preset、输入输出等一律放在 arguments，不进入 URL 或方法路径。
package ability_pxdesign

import (
	"context"
	"errors"
	"fmt"
	"github.com/0xdevelop/bioBox/common/app_path"
	"github.com/0xdevelop/bioBox/runtime_resource"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xdevelop/bioBox/ability/ability_task"
	"github.com/0xdevelop/bioBox/api/api_auth/api_auth_common"
	"github.com/0xdevelop/bioBox/api/api_error_code"
	"github.com/0xdevelop/bioBox/api/api_supported_methods"
	"github.com/0xdevelop/bioBox/cli_proc"
	"github.com/0xdevelop/bioBox/config"
	"github.com/google/uuid"
)

const MethodPXDesign = "pxdesign"

var errCLINotConfigured = errors.New("pxdesign CLI binary is not configured")

func LoadAPIMethods() {
	ability_task.RegisterBusinessSteps(MethodPXDesign,
		ability_task.BusinessStepSpec{Step: StepInputPreparation, BusinessStatuses: []string{StatusArgumentsValidated, StatusInputStaged, StatusPreparationFailed}},
		ability_task.BusinessStepSpec{Step: StepCLIExecution, BusinessStatuses: []string{StatusProcessExited, StatusProcessFailed, StatusProcessCancelled, StatusLaunchFailed}},
		ability_task.BusinessStepSpec{Step: StepArtifactCollection, BusinessStatuses: []string{StatusInspected, StatusCIFFilesFound, StatusNoCIFFiles, StatusERRArtifactsFound, StatusInspectionFailed}},
	)
	api_supported_methods.AddMethod(&api_supported_methods.SupportedMethod{
		Name:        MethodPXDesign,
		Description: "官方受管 runtime 当前仅核准 preview + 显式 use_msa=false，须可信完整资源集合已 ready；extended/custom 自动下载分支拒绝启动。提交 PXDesign 设计流水线任务（对应 CLI：pxdesign-run pipeline）。preset 走 arguments。异步队列使用配置的主关系库（支持 SQLite）。未配置 cli_cfg.pxdesign.binary 时 Execute 拒绝启动进程。",
		Async:       true,
		InputSchema: api_auth_common.InputSchema(
			map[string]interface{}{
				"input":      schemaWithDesc(api_auth_common.StringSchema("", 1, 4096), "输入 YAML/JSON 文件。对应 CLI -i/--input。"),
				"dump_dir":   schemaWithDesc(api_auth_common.StringSchema("", 1, 4096), "调用方给出的结果目录名；实际写入隔离作业目录下的 output/。对应 CLI -o/--dump_dir。"),
				"use_msa":    schemaWithDesc(map[string]interface{}{"type": "boolean"}, "是否使用 MSA；受管离线 preview 必须显式 false，未接管分支拒绝启动。"),
				"preset":     schemaWithDesc(api_auth_common.EnumStringSchema("preview", "extended", "custom"), "流水线预设。对应 CLI --preset。"),
				"N_sample":   schemaWithDesc(integerSchema(), "扩散样本数。对应 CLI --N_sample。"),
				"N_step":     schemaWithDesc(integerSchema(), "扩散步数。对应 CLI --N_step。"),
				"N_max_runs": schemaWithDesc(integerSchema(), "最大 design run 数。对应 CLI --N_max_runs。"),
				"dtype":      schemaWithDesc(api_auth_common.EnumStringSchema("fp32", "bf16"), "推理 dtype。对应 CLI --dtype。"),
				"eta_max":    schemaWithDesc(numberSchema(), "eta 调度最大值。对应 CLI --eta_max。"),
				"eta_min":    schemaWithDesc(numberSchema(), "eta 调度最小值。对应 CLI --eta_min。"),
				"eta_type":   schemaWithDesc(api_auth_common.StringSchema("", 1, 64), "eta 调度类型。对应 CLI --eta_type。"),
			},
			"input", "dump_dir",
		),
		Execute: Execute,
	})
}

func Execute(ctx context.Context, input interface{}) (interface{}, error) {
	if err := beginBusinessStep(ctx, StepInputPreparation, "校验参数、选择已配置 runtime 并暂存输入"); err != nil {
		return nil, err
	}
	if err := validatePXDesignArgs(input); err != nil {
		if reportErr := reportBusinessStatus(ctx, StatusPreparationFailed, "参数校验未通过"); reportErr != nil {
			return nil, errors.Join(err, reportErr)
		}
		return nil, err
	}
	if err := reportBusinessStatus(ctx, StatusArgumentsValidated, "参数校验完成"); err != nil {
		return nil, err
	}
	spec, jobDir, outDir, err := buildPXDesignSpec(ctx, input)
	if err != nil {
		return nil, errors.Join(err, reportBusinessStatus(ctx, StatusPreparationFailed, "runtime 选择或输入暂存失败"))
	}
	if err = os.MkdirAll(outDir, 0o755); err != nil {
		return nil, errors.Join(err, reportBusinessStatus(ctx, StatusPreparationFailed, "输出目录准备失败"))
	}
	if err = reportBusinessStatus(ctx, StatusInputStaged, "输入已暂存到隔离作业目录"); err != nil {
		return nil, err
	}
	if err = beginBusinessStep(ctx, StepCLIExecution, "进入 CLI 进程执行包装，不代表推理内部进度"); err != nil {
		return nil, err
	}
	result, runErr := cli_proc.Run(ctx, *spec)
	// Run 返回后再按实际进程结果上报；cancel 后也要把已发生的业务记录持久化。
	evidenceCtx := context.WithoutCancel(ctx)
	status := StatusLaunchFailed
	if result != nil && result.PID > 0 {
		switch {
		case result.Cancelled:
			status = StatusProcessCancelled
		case runErr != nil:
			status = StatusProcessFailed
		default:
			status = StatusProcessExited
		}
	}
	if err = reportBusinessStatus(evidenceCtx, status, "CLI 调用已返回，具体退出码见任务结果"); err != nil {
		return nil, errors.Join(runErr, err)
	}
	if err = beginBusinessStep(evidenceCtx, StepArtifactCollection, "检查作业目录产物；不推断内部折叠进度"); err != nil {
		return nil, errors.Join(runErr, err)
	}
	payload := executionPayload(jobDir, outDir, result)
	if runErr != nil && spec.Resources != nil {
		if state, e := cli_proc.ResourceStatus(evidenceCtx, *spec); e == nil {
			payload["resources"] = state
		}
	}
	count, hasERR, inspectErr := cli_proc.InspectOutput(outDir)
	if inspectErr != nil {
		return payload, errors.Join(runErr, inspectErr, reportBusinessStatus(evidenceCtx, StatusInspectionFailed, "产物检查失败"))
	}
	if err = reportBusinessStatus(evidenceCtx, StatusInspected, "产物目录检查完成"); err != nil {
		return payload, errors.Join(runErr, err)
	}
	cifStatus := StatusNoCIFFiles
	if count > 0 {
		cifStatus = StatusCIFFilesFound
	}
	if err = reportBusinessStatus(evidenceCtx, cifStatus, "仅检查产物存在性，不代表结构质量验收"); err != nil {
		return payload, errors.Join(runErr, err)
	}
	if hasERR {
		if err = reportBusinessStatus(evidenceCtx, StatusERRArtifactsFound, "发现 ERR 产物标记"); err != nil {
			return payload, errors.Join(runErr, err)
		}
	}
	if runErr != nil {
		return payload, runErr
	}
	_ = count
	_ = hasERR
	return payload, nil
}

func buildPXDesignSpec(ctx context.Context, input interface{}) (*cli_proc.Spec, string, string, error) {
	params, ok := api_auth_common.InputObject(input)
	if !ok {
		return nil, "", "", api_error_code.ErrInvalidArguments
	}
	cliCfg, binary, err := configuredBinary()
	if err != nil {
		return nil, "", "", err
	}
	root, err := app_path.ExecutableDir()
	if err != nil {
		return nil, "", "", err
	}
	preparedResources, err := runtime_resource.Compile("pxdesign", binary.Binary, root, binary.ExtraEnv, binary.Resources)
	if err != nil {
		return nil, "", "", err
	}
	inputPath, _ := api_auth_common.RequiredString(params, "input")
	taskID := executionTaskID(ctx)
	jobDir, stagedInput, err := cli_proc.StageInput(cliCfg.WorkRoot, taskID, inputPath)
	if err != nil {
		return nil, "", "", err
	}
	if err = cli_proc.StageRelativeSidecars(jobDir, inputPath, stagedInput); err != nil {
		return nil, "", "", err
	}
	outDir := filepath.Join(jobDir, "output")
	args := []string{"pipeline", "--input", stagedInput, "--dump_dir", outDir}
	args, err = appendPXDesignOptionalFlags(args, params)
	if err != nil {
		return nil, "", "", err
	}
	grace := time.Duration(cliCfg.CancelGraceSeconds) * time.Second
	if grace <= 0 {
		grace = 10 * time.Second
	}
	return &cli_proc.Spec{
		Resources:   preparedResources,
		ResourceSet: "pxdesign.preview",
		Name:        MethodPXDesign,
		Path:        binary.Binary,
		Args:        args,
		Env:         binary.ExtraEnv,
		Dir:         jobDir,
		LogPath:     filepath.Join(jobDir, "run.log"),
		Grace:       grace,
	}, jobDir, outDir, nil
}

func appendPXDesignOptionalFlags(args []string, params map[string]interface{}) ([]string, error) {
	if value, ok, err := cli_proc.OptionalBool(params, "use_msa"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendFlag(args, "--use_msa", fmt.Sprint(value))
	}
	if value, ok, err := cli_proc.OptionalString(params, "preset"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendFlag(args, "--preset", value)
	}
	if value, ok, err := cli_proc.OptionalInt(params, "N_sample"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendIntFlag(args, "--N_sample", value)
	}
	if value, ok, err := cli_proc.OptionalInt(params, "N_step"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendIntFlag(args, "--N_step", value)
	}
	if value, ok, err := cli_proc.OptionalInt(params, "N_max_runs"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendIntFlag(args, "--N_max_runs", value)
	}
	if value, ok, err := cli_proc.OptionalString(params, "dtype"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendFlag(args, "--dtype", value)
	}
	if value, ok, err := cli_proc.OptionalFloat(params, "eta_max"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendFloatFlag(args, "--eta_max", value)
	}
	if value, ok, err := cli_proc.OptionalFloat(params, "eta_min"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendFloatFlag(args, "--eta_min", value)
	}
	if value, ok, err := cli_proc.OptionalString(params, "eta_type"); err != nil {
		return nil, err
	} else if ok {
		args = cli_proc.AppendFlag(args, "--eta_type", value)
	}
	return args, nil
}

func configuredBinary() (*config.CLIConfig, *config.CLIBinaryConfig, error) {
	if config.GlobalConfig == nil || config.GlobalConfig.CliCfg == nil || config.GlobalConfig.CliCfg.PXDesign == nil {
		return nil, nil, errCLINotConfigured
	}
	cliCfg := config.GlobalConfig.CliCfg
	if strings.TrimSpace(cliCfg.WorkRoot) == "" {
		cliCfg.WorkRoot = "./data/cli_jobs"
	}
	binary := cliCfg.PXDesign
	if strings.TrimSpace(binary.Binary) == "" {
		return nil, nil, errCLINotConfigured
	}
	return cliCfg, binary, nil
}

func executionTaskID(ctx context.Context) string {
	if info, ok := ability_task.AsyncTaskInfoFromContext(ctx); ok && info.TaskID != "" {
		return info.TaskID
	}
	return "direct-" + uuid.NewString()
}

func executionPayload(jobDir, outDir string, result *cli_proc.Result) map[string]interface{} {
	payload := map[string]interface{}{
		"job_dir": jobDir,
		"out_dir": outDir,
	}
	if result != nil {
		payload["pid"] = result.PID
		payload["pgid"] = result.PGID
		payload["exit_code"] = result.ExitCode
		payload["cancelled"] = result.Cancelled
		payload["log_path"] = result.LogPath
	}
	cifCount, hasERR, err := cli_proc.InspectOutput(outDir)
	if err == nil {
		payload["cif_count"] = cifCount
		payload["has_err"] = hasERR
	}
	return payload
}

func validatePXDesignArgs(input interface{}) error {
	params, ok := api_auth_common.InputObject(input)
	if !ok {
		return api_error_code.ErrInvalidArguments
	}
	if _, ok = api_auth_common.RequiredString(params, "input"); !ok {
		return api_error_code.ErrInvalidArguments
	}
	if _, ok = api_auth_common.RequiredString(params, "dump_dir"); !ok {
		return api_error_code.ErrInvalidArguments
	}
	if preset, exists := params["preset"]; exists {
		presetText, isString := preset.(string)
		if !isString || (presetText != "preview" && presetText != "extended" && presetText != "custom") {
			return api_error_code.ErrInvalidArguments
		}
	}
	if dtype, exists := params["dtype"]; exists {
		dtypeText, isString := dtype.(string)
		if !isString || (dtypeText != "fp32" && dtypeText != "bf16") {
			return api_error_code.ErrInvalidArguments
		}
	}
	return nil
}

func schemaWithDesc(schema map[string]interface{}, description string) map[string]interface{} {
	schema["description"] = description
	return schema
}

func integerSchema() map[string]interface{} {
	return map[string]interface{}{"type": "integer"}
}

func numberSchema() map[string]interface{} {
	return map[string]interface{}{"type": "number"}
}

func BuildArgvForTest(ctx context.Context, input interface{}) ([]string, error) {
	spec, _, _, err := buildPXDesignSpec(ctx, input)
	if err != nil {
		return nil, err
	}
	return append([]string{spec.Path}, spec.Args...), nil
}

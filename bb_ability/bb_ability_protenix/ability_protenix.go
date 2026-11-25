// Package ability_protenix 注册固定功能入口 protenix（MCP tools/call 的 name）。
// 版本、模型、输入输出等一律放在 arguments，不进入 URL 或方法路径。
package bb_ability_protenix

import (
	"context"
	"errors"
	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_runtime_resource"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_cli_proc"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/google/uuid"
)

const MethodProtenix = "protenix"

var errCLINotConfigured = errors.New("protenix CLI binary is not configured")
var errV2CLINotConfigured = errors.New("protenix v2 CLI binary is not configured")

func LoadAPIMethods() {
	bb_ability_task.RegisterBusinessSteps(MethodProtenix,
		bb_ability_task.BusinessStepSpec{Step: StepInputPreparation, BusinessStatuses: []string{StatusArgumentsValidated, StatusInputStaged, StatusPreparationFailed}},
		bb_ability_task.BusinessStepSpec{Step: StepCLIExecution, BusinessStatuses: []string{StatusProcessExited, StatusProcessFailed, StatusProcessCancelled, StatusLaunchFailed}},
		bb_ability_task.BusinessStepSpec{Step: StepArtifactCollection, BusinessStatuses: []string{StatusInspected, StatusCIFFilesFound, StatusNoCIFFiles, StatusERRArtifactsFound, StatusInspectionFailed}},
	)
	bb_api_supported_methods.AddMethod(&bb_api_supported_methods.SupportedMethod{
		Name:        MethodProtenix,
		Description: "提交 Protenix 结构预测任务（对应 CLI：protenix-run pred）。官方受管 runtime 目前仅核准 v1 默认模型、显式 use_msa=false，且须可信完整资源清单已 ready；其余自动下载分支拒绝启动，不静默回退。版本/模型走 arguments。异步队列使用配置的主关系库（支持 SQLite）。version 缺省 v1，用于选择独立 runtime：v1 未单独配置时使用 cli_cfg.protenix.binary，v2 必须配置 cli_cfg.protenix.version_runtimes.v2.binary，不会回退到 v1。对应版本未配置时 Execute 拒绝启动进程。",
		Async:       true,
		InputSchema: bb_api_auth_common.InputSchema(
			map[string]interface{}{
				"version":            schemaWithDesc(bb_api_auth_common.EnumStringSchema("v1", "v2"), "产品版本（v1/v2），不是 URL 路径。缺省 v1。用于选择 cli_cfg.protenix.version_runtimes 中的独立 runtime；v1 未单独配置时回退到 cli_cfg.protenix.binary。v2 必须单独配置，不会使用 v1 的 binary，且还需要授权权重。不作为 CLI 开关传入。"),
				"input":              schemaWithDesc(bb_api_auth_common.StringSchema("", 1, 4096), "输入 JSON 文件或目录。对应 CLI -i/--input。"),
				"out_dir":            schemaWithDesc(bb_api_auth_common.StringSchema("", 1, 4096), "调用方给出的结果目录名；实际写入隔离作业目录下的 output/，不直接改原输入。对应 CLI -o/--out_dir。"),
				"model_name":         schemaWithDesc(bb_api_auth_common.StringSchema("", 1, 256), "模型检查点名。对应 CLI -n/--model_name，例如 protenix_base_default_v1.0.0。"),
				"seeds":              schemaWithDesc(bb_api_auth_common.StringSchema("", 1, 256), "逗号分隔随机种子。对应 CLI -s/--seeds。"),
				"cycle":              schemaWithDesc(integerSchema(), "Pairformer cycle 数。对应 CLI -c/--cycle。"),
				"step":               schemaWithDesc(integerSchema(), "扩散步数。对应 CLI -p/--step。"),
				"sample":             schemaWithDesc(integerSchema(), "样本数。对应 CLI -e/--sample。"),
				"dtype":              schemaWithDesc(bb_api_auth_common.EnumStringSchema("bf16", "fp32"), "推理 dtype。对应 CLI -d/--dtype。"),
				"use_msa":            schemaWithDesc(booleanSchema(), "是否使用 MSA。对应 CLI --use_msa。"),
				"use_default_params": schemaWithDesc(booleanSchema(), "是否使用模型推荐默认参数。对应 CLI --use_default_params。"),
			},
			"input", "out_dir",
		),
		Execute: Execute,
	})
}

func Execute(ctx context.Context, input interface{}) (interface{}, error) {
	if err := beginBusinessStep(ctx, StepInputPreparation, "校验参数、选择已配置 runtime 并暂存输入"); err != nil {
		return nil, err
	}
	if err := validateProtenixArgs(input); err != nil {
		if reportErr := reportBusinessStatus(ctx, StatusPreparationFailed, "参数校验未通过"); reportErr != nil {
			return nil, errors.Join(err, reportErr)
		}
		return nil, err
	}
	if err := reportBusinessStatus(ctx, StatusArgumentsValidated, "参数校验完成"); err != nil {
		return nil, err
	}
	spec, jobDir, outDir, err := buildProtenixSpec(ctx, input)
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
	result, runErr := bb_cli_proc.Run(ctx, *spec)
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
		if state, e := bb_cli_proc.ResourceStatus(evidenceCtx, *spec); e == nil {
			payload["resources"] = state
		}
	}
	count, hasERR, inspectErr := bb_cli_proc.InspectOutput(outDir)
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
	if count == 0 || hasERR {
		return payload, errors.New("protenix produced no valid CIF result or reported ERR artifacts")
	}
	return payload, nil
}

func buildProtenixSpec(ctx context.Context, input interface{}) (*bb_cli_proc.Spec, string, string, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok {
		return nil, "", "", bb_api_error_code.ErrInvalidArguments
	}
	version, err := requestedVersion(params)
	if err != nil {
		return nil, "", "", err
	}
	cliCfg, binary, err := configuredBinary(version)
	if err != nil {
		return nil, "", "", err
	}
	root, err := bb_app_path.ExecutableDir()
	if err != nil {
		return nil, "", "", err
	}
	preparedResources, err := bb_runtime_resource.Compile("protenix", binary.Binary, root, binary.ExtraEnv, binary.Resources)
	if err != nil {
		return nil, "", "", err
	}
	if version != "v1" && filepath.Base(binary.Binary) == "protenix-run" {
		return nil, "", "", bb_runtime_resource.ErrUnsupported
	}
	inputPath, _ := bb_api_auth_common.RequiredString(params, "input")
	taskID := executionTaskID(ctx)
	jobDir, stagedInput, err := bb_cli_proc.StageInput(cliCfg.WorkRoot, taskID, inputPath)
	if err != nil {
		return nil, "", "", err
	}
	outDir := filepath.Join(jobDir, "output")
	args := []string{"pred", "--input", stagedInput, "--out_dir", outDir}
	args, err = appendProtenixOptionalFlags(args, params)
	if err != nil {
		return nil, "", "", err
	}
	grace := time.Duration(cliCfg.CancelGraceSeconds) * time.Second
	if grace <= 0 {
		grace = 10 * time.Second
	}
	return &bb_cli_proc.Spec{
		Resources:   preparedResources,
		ResourceSet: "protenix." + version,
		Name:        MethodProtenix,
		Path:        binary.Binary,
		Args:        args,
		Env:         binary.ExtraEnv,
		Dir:         jobDir,
		LogPath:     filepath.Join(jobDir, "run.log"),
		Grace:       grace,
	}, jobDir, outDir, nil
}

func appendProtenixOptionalFlags(args []string, params map[string]interface{}) ([]string, error) {
	if value, ok, err := bb_cli_proc.OptionalString(params, "model_name"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendFlag(args, "--model_name", value)
	}
	if value, ok, err := bb_cli_proc.OptionalString(params, "seeds"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendFlag(args, "--seeds", value)
	}
	if value, ok, err := bb_cli_proc.OptionalInt(params, "cycle"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendIntFlag(args, "--cycle", value)
	}
	if value, ok, err := bb_cli_proc.OptionalInt(params, "step"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendIntFlag(args, "--step", value)
	}
	if value, ok, err := bb_cli_proc.OptionalInt(params, "sample"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendIntFlag(args, "--sample", value)
	}
	if value, ok, err := bb_cli_proc.OptionalString(params, "dtype"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendFlag(args, "--dtype", value)
	}
	if value, ok, err := bb_cli_proc.OptionalBool(params, "use_msa"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendBoolFlag(args, "--use_msa", value)
	}
	if value, ok, err := bb_cli_proc.OptionalBool(params, "use_default_params"); err != nil {
		return nil, err
	} else if ok {
		args = bb_cli_proc.AppendBoolFlag(args, "--use_default_params", value)
	}
	return args, nil
}

func requestedVersion(params map[string]interface{}) (string, error) {
	if version, exists := params["version"]; exists {
		versionText, isString := version.(string)
		if !isString || (versionText != "v1" && versionText != "v2") {
			return "", bb_api_error_code.ErrInvalidArguments
		}
		return versionText, nil
	}
	return "v1", nil
}

func configuredBinary(version string) (*bb_config.CLIConfig, *bb_config.CLIBinaryConfig, error) {
	if bb_config.GlobalConfig == nil || bb_config.GlobalConfig.CliCfg == nil || bb_config.GlobalConfig.CliCfg.Protenix == nil {
		return nil, nil, errCLINotConfigured
	}
	cliCfg := bb_config.GlobalConfig.CliCfg
	if strings.TrimSpace(cliCfg.WorkRoot) == "" {
		cliCfg.WorkRoot = "./data/cli_jobs"
	}
	binary, err := runtimeForVersion(cliCfg.Protenix, version)
	if err != nil {
		return nil, nil, err
	}
	return cliCfg, binary, nil
}

func runtimeForVersion(base *bb_config.CLIBinaryConfig, version string) (*bb_config.CLIBinaryConfig, error) {
	if override := versionRuntime(base, version); override != nil && strings.TrimSpace(override.Binary) != "" {
		resources := base.Resources
		if override.Resources != nil {
			resources = override.Resources
		}
		env := base.ExtraEnv
		if override.ExtraEnv != nil {
			env = override.ExtraEnv
		}
		return &bb_config.CLIBinaryConfig{
			Binary:    strings.TrimSpace(override.Binary),
			Resources: resources,
			ExtraEnv:  env,
		}, nil
	}
	if version == "v2" {
		return nil, errV2CLINotConfigured
	}
	if base == nil || strings.TrimSpace(base.Binary) == "" {
		return nil, errCLINotConfigured
	}
	return base, nil
}

func versionRuntime(base *bb_config.CLIBinaryConfig, version string) *bb_config.CLIRuntimeConfig {
	if base == nil || base.VersionRuntimes == nil {
		return nil
	}
	return base.VersionRuntimes[version]
}

func executionTaskID(ctx context.Context) string {
	if info, ok := bb_ability_task.AsyncTaskInfoFromContext(ctx); ok && info.TaskID != "" {
		return info.TaskID
	}
	return "direct-" + uuid.NewString()
}

func executionPayload(jobDir, outDir string, result *bb_cli_proc.Result) map[string]interface{} {
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
	cifCount, hasERR, err := bb_cli_proc.InspectOutput(outDir)
	if err == nil {
		payload["cif_count"] = cifCount
		payload["has_err"] = hasERR
	}
	return payload
}

func validateProtenixArgs(input interface{}) error {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok {
		return bb_api_error_code.ErrInvalidArguments
	}
	if _, ok = bb_api_auth_common.RequiredString(params, "input"); !ok {
		return bb_api_error_code.ErrInvalidArguments
	}
	if _, ok = bb_api_auth_common.RequiredString(params, "out_dir"); !ok {
		return bb_api_error_code.ErrInvalidArguments
	}
	if version, exists := params["version"]; exists {
		versionText, isString := version.(string)
		if !isString || (versionText != "v1" && versionText != "v2") {
			return bb_api_error_code.ErrInvalidArguments
		}
	}
	if dtype, exists := params["dtype"]; exists {
		dtypeText, isString := dtype.(string)
		if !isString || (dtypeText != "bf16" && dtypeText != "fp32") {
			return bb_api_error_code.ErrInvalidArguments
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

func booleanSchema() map[string]interface{} {
	return map[string]interface{}{"type": "boolean"}
}

// BuildArgvForTest 导出给测试：确认 version 不进入 CLI 参数。
func BuildArgvForTest(ctx context.Context, input interface{}) ([]string, error) {
	spec, _, _, err := buildProtenixSpec(ctx, input)
	if err != nil {
		return nil, err
	}
	return append([]string{spec.Path}, spec.Args...), nil
}

package bb_ability_task

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task/bb_ability_task_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_common"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_auth/bb_api_auth_session"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	MethodTaskArtifacts = "task.artifacts"
	MethodTaskArtifact  = "task.artifact"

	artifactListLimit       = 200
	artifactMaxPathLength   = 1024
	artifactMaxWalkDepth    = 8
	defaultArtifactMaxBytes = int64(32 << 20)
	artifactInputDir        = "input"
	artifactOutputDir       = "output"
	artifactRunLog          = "run.log"
)

var artifactMaxBytes = defaultArtifactMaxBytes

var artifactTypes = map[string]struct {
	kind        string
	contentType string
}{
	".cif":  {kind: "cif", contentType: "chemical/x-cif"},
	".pdb":  {kind: "pdb", contentType: "chemical/x-pdb"},
	".json": {kind: "json", contentType: "application/json"},
	".log":  {kind: "log", contentType: "text/plain"},
	".txt":  {kind: "txt", contentType: "text/plain"},
}

type artifactInfo struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

// TaskArtifacts 列出当前用户某条已完成任务在隔离作业目录内的可下载产物。
// 只返回相对路径，不含服务器绝对路径；仅暴露 output/ 与顶层 run.log，不暴露 input/ 或为官方 YAML 暂存的 examples/ 等输入侧车。
func TaskArtifacts(ctx context.Context, input interface{}) (interface{}, error) {
	task, err := loadOwnedTask(ctx, input, "task_id")
	if err != nil {
		return nil, err
	}
	artifacts := make([]artifactInfo, 0)
	jobDir, err := confinedJobDir(task)
	if err == nil && jobDir != "" {
		listed, listErr := listTaskArtifacts(jobDir)
		if listErr != nil {
			return nil, listErr
		}
		artifacts = listed
	}
	return map[string]interface{}{"task_id": task.TaskID, "artifacts": artifacts}, nil
}

// TaskArtifact 读取当前用户某条已完成任务的单个产物内容（base64）。
func TaskArtifact(ctx context.Context, input interface{}) (interface{}, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, "task_id", "path") {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	task, err := loadOwnedTaskFromParams(ctx, params)
	if err != nil {
		return nil, err
	}
	rel, ok := bb_api_auth_common.RequiredString(params, "path")
	if !ok {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	jobDir, err := confinedJobDir(task)
	if err != nil || jobDir == "" {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	file, err := readTaskArtifact(jobDir, rel)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"task_id":      task.TaskID,
		"path":         file.Path,
		"name":         file.Name,
		"kind":         file.Kind,
		"size":         file.Size,
		"content_type": file.ContentType,
		"sha256":       file.SHA256,
		"encoding":     "base64",
		"content":      file.Content,
	}, nil
}

func loadOwnedTask(ctx context.Context, input interface{}, keys ...string) (*bb_ability_task_model.AsyncTask, error) {
	params, ok := bb_api_auth_common.InputObject(input)
	if !ok || !bb_api_auth_common.HasOnlyKeys(params, keys...) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	return loadOwnedTaskFromParams(ctx, params)
}

func loadOwnedTaskFromParams(ctx context.Context, params map[string]interface{}) (*bb_ability_task_model.AsyncTask, error) {
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
	task := &bb_ability_task_model.AsyncTask{}
	findErr := db.Where("task_id = ? AND user_id = ?", taskID, user.UserID).Take(task).Error
	if errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if findErr != nil {
		return nil, findErr
	}
	return task, nil
}

func confinedJobDir(task *bb_ability_task_model.AsyncTask) (string, error) {
	if task == nil || task.RunStatus != RunStatusDone {
		return "", errNoArtifactRoot
	}
	workRoot, err := configuredWorkRoot()
	if err != nil {
		return "", err
	}
	if task.ResultPayload != "" {
		var payload map[string]interface{}
		if err = json.Unmarshal([]byte(task.ResultPayload), &payload); err == nil {
			if jobDir, ok := payload["job_dir"].(string); ok && strings.TrimSpace(jobDir) != "" {
				if verified, verifyErr := verifyJobDir(workRoot, task.TaskID, jobDir); verifyErr == nil {
					return verified, nil
				}
			}
		}
	}
	return verifyJobDir(workRoot, task.TaskID, filepath.Join(workRoot, task.TaskID))
}

var errNoArtifactRoot = errors.New("no confined artifact root")

func configuredWorkRoot() (string, error) {
	if bb_config.GlobalConfig == nil || bb_config.GlobalConfig.CliCfg == nil {
		return "", errNoArtifactRoot
	}
	work := strings.TrimSpace(bb_config.GlobalConfig.CliCfg.WorkRoot)
	if work == "" {
		work = "./data/cli_jobs"
	}
	abs, err := filepath.Abs(work)
	if err != nil {
		return "", errNoArtifactRoot
	}
	return resolveExisting(abs), nil
}

func verifyJobDir(workRoot, taskID, jobDir string) (string, error) {
	if taskID == "" || !filepath.IsAbs(jobDir) {
		return "", errNoArtifactRoot
	}
	jobAbs := resolveExisting(filepath.Clean(jobDir))
	expected := resolveExisting(filepath.Join(workRoot, taskID))
	if jobAbs != expected || filepath.Base(jobAbs) != taskID {
		return "", errNoArtifactRoot
	}
	info, err := os.Lstat(jobAbs)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		// If the path was already resolved through EvalSymlinks, Lstat of the
		// final path should be a directory. Reject missing or non-dir roots.
		if err != nil || !info.IsDir() {
			return "", errNoArtifactRoot
		}
	}
	return jobAbs, nil
}

func resolveExisting(p string) string {
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return resolved
}

func cleanArtifactRelPath(raw string) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" || len(text) > artifactMaxPathLength {
		return "", false
	}
	if strings.ContainsRune(text, 0) || strings.Contains(text, `\`) {
		return "", false
	}
	slash := filepath.ToSlash(text)
	if path.IsAbs(slash) {
		return "", false
	}
	cleaned := path.Clean(slash)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == "" || part == "." || part == ".." {
			return "", false
		}
	}
	if !allowedArtifactRel(cleaned) {
		return "", false
	}
	if _, ok := artifactType(cleaned); !ok {
		return "", false
	}
	return cleaned, true
}

func allowedArtifactRel(rel string) bool {
	if rel == artifactRunLog {
		return true
	}
	return rel == artifactOutputDir || strings.HasPrefix(rel, artifactOutputDir+"/")
}

func artifactType(rel string) (info struct {
	kind        string
	contentType string
}, ok bool) {
	ext := strings.ToLower(path.Ext(rel))
	info, ok = artifactTypes[ext]
	return info, ok
}

func listTaskArtifacts(jobDir string) ([]artifactInfo, error) {
	artifacts := make([]artifactInfo, 0)
	err := filepath.WalkDir(jobDir, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrPermission) || errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		rel, err := filepath.Rel(jobDir, full)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "." {
			return nil
		}
		if depthOf(relSlash) > artifactMaxWalkDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if relSlash != artifactOutputDir && !strings.HasPrefix(relSlash, artifactOutputDir+"/") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(artifacts) >= artifactListLimit {
			return fs.SkipAll
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		cleaned, ok := cleanArtifactRelPath(relSlash)
		if !ok {
			return nil
		}
		kind, _ := artifactType(cleaned)
		artifacts = append(artifacts, artifactInfo{
			Path:        cleaned,
			Name:        path.Base(cleaned),
			Kind:        kind.kind,
			Size:        info.Size(),
			ContentType: kind.contentType,
		})
		return nil
	})
	if err != nil && !errors.Is(err, fs.SkipAll) {
		return nil, err
	}
	return artifacts, nil
}

func depthOf(rel string) int {
	if rel == "" || rel == "." {
		return 0
	}
	return len(strings.Split(rel, "/"))
}

func readTaskArtifact(jobDir, rawPath string) (*struct {
	artifactInfo
	SHA256  string
	Content string
}, error) {
	rel, ok := cleanArtifactRelPath(rawPath)
	if !ok {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	full := filepath.Join(jobDir, filepath.FromSlash(rel))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	rootResolved := resolveExisting(jobDir)
	sep := string(os.PathSeparator)
	if resolved != filepath.Join(rootResolved, filepath.FromSlash(rel)) && !strings.HasPrefix(resolved, rootResolved+sep) {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if !strings.HasPrefix(resolved, rootResolved+sep) && resolved != rootResolved {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	if info.Size() > artifactMaxBytes {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	file, err := os.Open(full)
	if err != nil {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	defer file.Close()
	limited := io.LimitReader(file, artifactMaxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil || int64(len(data)) > artifactMaxBytes {
		return nil, bb_api_error_code.ErrInvalidArguments
	}
	kind, _ := artifactType(rel)
	digest := sha256.Sum256(data)
	return &struct {
		artifactInfo
		SHA256  string
		Content string
	}{
		artifactInfo: artifactInfo{
			Path:        rel,
			Name:        path.Base(rel),
			Kind:        kind.kind,
			Size:        int64(len(data)),
			ContentType: kind.contentType,
		},
		// SHA256 is computed over exactly the bytes returned in Content, so a
		// client can verify the decoded payload against the server-side file.
		SHA256:  hex.EncodeToString(digest[:]),
		Content: base64.StdEncoding.EncodeToString(data),
	}, nil
}

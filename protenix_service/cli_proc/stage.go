package cli_proc

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// StageInput 把输入文件或目录拷贝到 workRoot/taskID/input/ 下，不修改原路径。
func StageInput(workRoot, taskID, inputPath string) (jobDir, stagedInput string, err error) {
	if strings.TrimSpace(workRoot) == "" || strings.TrimSpace(taskID) == "" {
		return "", "", fmt.Errorf("job work root and task id are required")
	}
	absInput, err := filepath.Abs(inputPath)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(absInput)
	if err != nil {
		return "", "", fmt.Errorf("input path is not accessible: %w", err)
	}
	jobDir, err = filepath.Abs(filepath.Join(workRoot, taskID))
	if err != nil {
		return "", "", err
	}
	inputDir := filepath.Join(jobDir, "input")
	if err = os.MkdirAll(inputDir, 0o755); err != nil {
		return "", "", err
	}
	if info.IsDir() {
		stagedInput = filepath.Join(inputDir, "src")
		if err = copyDir(absInput, stagedInput); err != nil {
			return "", "", err
		}
		return jobDir, stagedInput, nil
	}
	stagedInput = filepath.Join(inputDir, filepath.Base(absInput))
	if err = copyFile(absInput, stagedInput); err != nil {
		return "", "", err
	}
	return jobDir, stagedInput, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("refusing to copy symlink: %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// StageRelativeSidecars 把输入文件中的相对路径资源拷到 jobDir，保持原文相对路径，
// 这样官方 YAML（例如 ./examples/5o45.cif）在进程工作目录为 jobDir 时能解析。
// 不修改原文件或已暂存的输入正文。绝对路径保持原样，由调用方自行指向可读位置。
func StageRelativeSidecars(jobDir, originalInput, stagedInput string) error {
	if strings.TrimSpace(jobDir) == "" || strings.TrimSpace(originalInput) == "" || strings.TrimSpace(stagedInput) == "" {
		return fmt.Errorf("job dir and input paths are required")
	}
	absJob, err := filepath.Abs(jobDir)
	if err != nil {
		return err
	}
	absOriginal, err := filepath.Abs(originalInput)
	if err != nil {
		return err
	}
	absStaged, err := filepath.Abs(stagedInput)
	if err != nil {
		return err
	}
	stagedInfo, err := os.Lstat(absStaged)
	if err != nil {
		return fmt.Errorf("staged input is not accessible: %w", err)
	}
	if stagedInfo.IsDir() || !stagedInfo.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(absStaged)
	if err != nil {
		return err
	}
	var doc interface{}
	if err = yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var first error
	walkStrings(doc, func(value string) {
		if first != nil {
			return
		}
		if err := stageOneRelativeSidecar(absJob, absOriginal, value); err != nil {
			first = err
		}
	})
	return first
}

func walkStrings(value interface{}, fn func(string)) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for _, child := range typed {
			walkStrings(child, fn)
		}
	case map[interface{}]interface{}:
		for _, child := range typed {
			walkStrings(child, fn)
		}
	case []interface{}:
		for _, child := range typed {
			walkStrings(child, fn)
		}
	case string:
		fn(typed)
	}
}

func looksLikeRelativePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "://") {
		return false
	}
	return strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") || strings.ContainsAny(value, `/\`)
}

func stageOneRelativeSidecar(jobDir, originalInput, raw string) error {
	rel := strings.TrimSpace(raw)
	if !looksLikeRelativePath(rel) {
		return nil
	}
	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to stage path outside the job directory: %s", rel)
	}
	dest := filepath.Join(jobDir, cleaned)
	relToJob, err := filepath.Rel(jobDir, dest)
	if err != nil || relToJob == ".." || strings.HasPrefix(relToJob, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("refusing to stage path outside the job directory: %s", rel)
	}
	source, err := resolveSidecarSource(originalInput, rel)
	if err != nil {
		return err
	}
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("relative input path is not accessible: %s: %w", rel, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to copy symlink: %s", source)
	}
	if _, err = os.Lstat(dest); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if info.IsDir() {
		return copyDir(source, dest)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("relative input path is not a regular file or directory: %s", rel)
	}
	return copyFile(source, dest)
}

func resolveSidecarSource(originalInput, rel string) (string, error) {
	roots := []string{filepath.Dir(originalInput)}
	parent := filepath.Dir(filepath.Dir(originalInput))
	if parent != filepath.Dir(originalInput) && parent != "" {
		roots = append(roots, parent)
	}
	for _, root := range roots {
		candidate := filepath.Clean(filepath.Join(root, rel))
		info, err := os.Lstat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing to copy symlink: %s", candidate)
		}
		return candidate, nil
	}
	return "", fmt.Errorf("relative input path not found: %s", rel)
}

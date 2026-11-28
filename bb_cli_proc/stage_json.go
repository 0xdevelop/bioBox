package bb_cli_proc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// StageJSONInput writes data as jobDir/input/input.json and returns those paths.
func StageJSONInput(workRoot, taskID string, data interface{}) (jobDir, stagedInput string, err error) {
	if strings.TrimSpace(workRoot) == "" || strings.TrimSpace(taskID) == "" {
		return "", "", fmt.Errorf("job work root and task id are required")
	}
	if data == nil {
		return "", "", fmt.Errorf("input_data is required")
	}
	jobDir, err = filepath.Abs(filepath.Join(workRoot, taskID))
	if err != nil {
		return "", "", err
	}
	inputDir := filepath.Join(jobDir, "input")
	if err = os.MkdirAll(inputDir, 0o755); err != nil {
		return "", "", err
	}
	stagedInput = filepath.Join(inputDir, "input.json")
	buf, err := json.Marshal(data)
	if err != nil {
		return "", "", fmt.Errorf("input_data is not valid JSON: %w", err)
	}
	tmp, err := os.CreateTemp(inputDir, ".input-*.json")
	if err != nil {
		return "", "", err
	}
	tmpName := tmp.Name()
	if _, err = tmp.Write(buf); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", "", err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", "", err
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", "", err
	}
	if err = os.Rename(tmpName, stagedInput); err != nil {
		_ = os.Remove(tmpName)
		return "", "", err
	}
	return jobDir, stagedInput, nil
}

// StageAttachments writes optional sidecar files under inputDir.
// attachments may be a map of relative path -> {content, encoding} or a list of
// {path, content, encoding} objects. encoding is utf8 (default) or base64.
func StageAttachments(inputDir string, attachments interface{}) error {
	if attachments == nil {
		return nil
	}
	absInput, err := filepath.Abs(inputDir)
	if err != nil {
		return err
	}
	switch typed := attachments.(type) {
	case map[string]interface{}:
		for rel, raw := range typed {
			if err := writeOneAttachment(absInput, rel, raw); err != nil {
				return err
			}
		}
		return nil
	case []interface{}:
		for _, raw := range typed {
			item, ok := raw.(map[string]interface{})
			if !ok {
				return fmt.Errorf("attachments entries must be objects")
			}
			rel, _ := item["path"].(string)
			if err := writeOneAttachment(absInput, rel, item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("attachments must be an object or an array")
	}
}

func writeOneAttachment(inputDir, rel string, raw interface{}) error {
	safe, err := confinedAttachmentPath(inputDir, rel)
	if err != nil {
		return err
	}
	item, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("attachment %q must be an object with content", rel)
	}
	content, _ := item["content"].(string)
	encoding, _ := item["encoding"].(string)
	encoding = strings.ToLower(strings.TrimSpace(encoding))
	if encoding == "" {
		encoding = "utf8"
	}
	var body []byte
	switch encoding {
	case "utf8", "utf-8":
		body = []byte(content)
	case "base64":
		body, err = base64.StdEncoding.DecodeString(content)
		if err != nil {
			return fmt.Errorf("attachment %q is not valid base64: %w", rel, err)
		}
	default:
		return fmt.Errorf("attachment %q has unsupported encoding %q", rel, encoding)
	}
	if err = os.MkdirAll(filepath.Dir(safe), 0o755); err != nil {
		return err
	}
	return os.WriteFile(safe, body, 0o644)
}

func confinedAttachmentPath(inputDir, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", fmt.Errorf("attachment path is required")
	}
	if strings.ContainsRune(rel, 0) || strings.Contains(rel, `\`) {
		return "", fmt.Errorf("attachment path is invalid: %s", rel)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("attachment path must be relative: %s", rel)
	}
	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("attachment path escapes the input directory: %s", rel)
	}
	dest := filepath.Join(inputDir, cleaned)
	relToInput, err := filepath.Rel(inputDir, dest)
	if err != nil || relToInput == ".." || strings.HasPrefix(relToInput, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("attachment path escapes the input directory: %s", rel)
	}
	return dest, nil
}

package bb_cli_proc

import (
	"fmt"
	"strings"
)

// SelectStagedInput stages either a legacy input path or JSON input_data.
// The two keys are mutually exclusive and one of them is required.
func SelectStagedInput(workRoot, taskID string, params map[string]interface{}) (jobDir, stagedInput string, err error) {
	path, hasPath := inputPathValue(params)
	data, hasData := inputDataValue(params)
	if hasPath && hasData {
		return "", "", fmt.Errorf("input and input_data cannot be set together")
	}
	if !hasPath && !hasData {
		return "", "", fmt.Errorf("input or input_data is required")
	}
	if hasData {
		return StageJSONInput(workRoot, taskID, data)
	}
	return StageInput(workRoot, taskID, path)
}

func inputPathValue(params map[string]interface{}) (string, bool) {
	raw, ok := params["input"]
	if !ok || raw == nil {
		return "", false
	}
	text, ok := raw.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	return text, true
}

func inputDataValue(params map[string]interface{}) (interface{}, bool) {
	raw, ok := params["input_data"]
	if !ok || raw == nil {
		return nil, false
	}
	switch raw.(type) {
	case map[string]interface{}, []interface{}:
		return raw, true
	default:
		return nil, false
	}
}

// InputSourceError reports a missing or conflicting input / input_data pair.
func InputSourceError(params map[string]interface{}) error {
	_, hasPath := inputPathValue(params)
	_, hasData := inputDataValue(params)
	if hasPath && hasData {
		return fmt.Errorf("input and input_data cannot be set together")
	}
	if !hasPath && !hasData {
		return fmt.Errorf("input or input_data is required")
	}
	if raw, ok := params["input_data"]; ok && raw != nil {
		if _, good := inputDataValue(params); !good {
			return fmt.Errorf("input_data must be a JSON object or array")
		}
	}
	if raw, ok := params["input"]; ok && raw != nil {
		if _, isString := raw.(string); !isString {
			return fmt.Errorf("input must be a path string")
		}
	}
	return nil
}

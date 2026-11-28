package bb_ability_protenix

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xdevelop/bioBox/bb_api/bb_api_error_code"
	"github.com/0xdevelop/bioBox/bb_config"
)

func TestValidateInputDataAndPathChoice(t *testing.T) {
	if err := validateProtenixArgs(map[string]interface{}{}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("missing both: %v", err)
	}
	if err := validateProtenixArgs(map[string]interface{}{"input": "in.json", "input_data": map[string]interface{}{"name": "x"}}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("both present: %v", err)
	}
	if err := validateProtenixArgs(map[string]interface{}{"input_data": map[string]interface{}{"name": "x"}}); err != nil {
		t.Fatalf("input_data only should pass, out_dir optional: %v", err)
	}
	if err := validateProtenixArgs(map[string]interface{}{"input": "in.json"}); err != nil {
		t.Fatalf("legacy input path should pass: %v", err)
	}
}

func TestExecuteInputDataWritesJobInputJSON(t *testing.T) {
	work := t.TempDir()
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	argvFile := filepath.Join(work, "argv.txt")
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		Protenix: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary: dummyRun(t),
			ExtraEnv: map[string]string{
				"DUMMY_WRITE_CIF": "1",
				"DUMMY_ARGV_FILE": argvFile,
			},
		},
	}}
	data := map[string]interface{}{"name": "ten_alanine", "sequences": []interface{}{"AAAAAAAAAA"}}
	value, err := Execute(context.Background(), map[string]interface{}{"input_data": data})
	if err != nil {
		t.Fatalf("execute input_data: %v", err)
	}
	payload := value.(map[string]interface{})
	jobDir, _ := payload["job_dir"].(string)
	staged := filepath.Join(jobDir, "input", "input.json")
	body, err := os.ReadFile(staged)
	if err != nil || !strings.Contains(string(body), "ten_alanine") {
		t.Fatalf("staged json: %s %v", body, err)
	}
	argv, err := os.ReadFile(argvFile)
	if err != nil || !strings.Contains(string(argv), staged) {
		t.Fatalf("argv should pass staged json, got %s err=%v", argv, err)
	}
}

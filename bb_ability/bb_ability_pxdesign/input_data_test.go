package bb_ability_pxdesign

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
	if err := validatePXDesignArgs(map[string]interface{}{}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("missing both: %v", err)
	}
	if err := validatePXDesignArgs(map[string]interface{}{"input": "in.yaml", "input_data": map[string]interface{}{"name": "x"}}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("both present: %v", err)
	}
	if err := validatePXDesignArgs(map[string]interface{}{"input_data": []interface{}{"not-object"}}); err != bb_api_error_code.ErrInvalidArguments {
		t.Fatalf("array input_data must be rejected: %v", err)
	}
	if err := validatePXDesignArgs(map[string]interface{}{"input_data": map[string]interface{}{"name": "x"}}); err != nil {
		t.Fatalf("object input_data should pass: %v", err)
	}
}

func TestExecuteInputDataWritesAttachmentsAndRejectsEscape(t *testing.T) {
	work := t.TempDir()
	previous := bb_config.GlobalConfig
	t.Cleanup(func() { bb_config.GlobalConfig = previous })
	bb_config.GlobalConfig = &bb_config.FileConfig{CliCfg: &bb_config.CLIConfig{
		WorkRoot:           filepath.Join(work, "jobs"),
		CancelGraceSeconds: 1,
		PXDesign: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t),
			Binary:   dummyRun(t),
			ExtraEnv: map[string]string{"DUMMY_WRITE_CIF": "1"},
		},
	}}
	value, err := Execute(context.Background(), map[string]interface{}{
		"input_data": map[string]interface{}{"name": "task", "ligand": "ligand.cif"},
		"attachments": []interface{}{
			map[string]interface{}{"path": "ligand.cif", "content": "data_lig", "encoding": "utf8"},
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	payload := value.(map[string]interface{})
	jobDir, _ := payload["job_dir"].(string)
	got, err := os.ReadFile(filepath.Join(jobDir, "input", "ligand.cif"))
	if err != nil || string(got) != "data_lig" {
		t.Fatalf("attachment: %s %v", got, err)
	}
	staged, err := os.ReadFile(filepath.Join(jobDir, "input", "input.json"))
	if err != nil || !strings.Contains(string(staged), "task") {
		t.Fatalf("input.json: %s %v", staged, err)
	}

	_, err = Execute(context.Background(), map[string]interface{}{
		"input_data": map[string]interface{}{"name": "bad"},
		"attachments": []interface{}{
			map[string]interface{}{"path": "../secret", "content": "x"},
		},
	})
	if err == nil {
		t.Fatal("path escape should fail staging")
	}
}

package bb_ability_protenix

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_pxdesign"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task"
	"github.com/0xdevelop/bioBox/bb_ability/bb_ability_task/bb_ability_task_model"
	"github.com/0xdevelop/bioBox/bb_api/bb_api_supported_methods"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_storage"
	"github.com/0xdevelop/bioBox/bb_storage/bb_storage_config"
	"github.com/goccy/go-json"
)

// Fixture-only CLI integration: no models, GPU or inference. Proves the two businesses
// write their own factual steps through the same generic task engine.
func TestAsyncBusinessStepsFixtureCLI(t *testing.T) {
	for _, method := range []string{MethodProtenix, bb_ability_pxdesign.MethodPXDesign} {
		for _, outcome := range []string{"success", "process_failure", "unconfigured"} {
			t.Run(method+"/"+outcome, func(t *testing.T) {
				work := t.TempDir()
				previous := bb_config.GlobalConfig
				t.Cleanup(func() { _ = bb_storage.CloseStorage(); bb_config.GlobalConfig = previous })
				input := filepath.Join(work, "input.json")
				if e := os.WriteFile(input, []byte(`{"fixture":true}`), 0600); e != nil {
					t.Fatal(e)
				}
				binary := filepath.Join(work, "fixture-cli.sh")
				script := `#!/bin/sh
set -eu
out="";prev=""
for arg in "$@"; do
 if [ "$prev" = "--out_dir" ] || [ "$prev" = "--dump_dir" ]; then out="$arg"; fi
 prev="$arg"
done
sleep 0.3
`
				if outcome == "success" {
					script += "mkdir -p \"$out\"\nprintf 'data_fixture_not_inference\\n' > \"$out/fixture.cif\"\n"
				} else {
					script += "echo fixture-cli-failure >&2\nexit 3\n"
				}
				if e := os.WriteFile(binary, []byte(script), 0700); e != nil {
					t.Fatal(e)
				}
				cli := &bb_config.CLIConfig{WorkRoot: filepath.Join(work, "jobs"), CancelGraceSeconds: 1, Protenix: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: binary}, PXDesign: &bb_config.CLIBinaryConfig{Resources: fixtureResources(t), Binary: binary}}
				if outcome == "unconfigured" {
					cli.Protenix.Binary = ""
					cli.PXDesign.Binary = ""
				}
				bb_config.GlobalConfig = &bb_config.FileConfig{StorageCfg: bb_storage_config.Default(filepath.Join(work, "app.db")), CliCfg: cli}
				if e := bb_storage.OpenStorage(); e != nil {
					t.Fatal(e)
				}
				if e := bb_storage.GormAutoMigrate(); e != nil {
					t.Fatal(e)
				}
				db, e := bb_storage.GormDB(context.Background())
				if e != nil {
					t.Fatal(e)
				}
				bb_api_supported_methods.SupportedMethodsSetup()
				LoadAPIMethods()
				bb_ability_pxdesign.LoadAPIMethods()
				args := map[string]interface{}{"input": input, "out_dir": "out"}
				if method == bb_ability_pxdesign.MethodPXDesign {
					args = map[string]interface{}{"input": input, "dump_dir": "out"}
				}
				accepted, e := bb_ability_task.AcceptAsyncTask(context.Background(), method, args)
				if e != nil {
					t.Fatal(e)
				}
				id := accepted.(map[string]interface{})["task_id"].(string)
				done := make(chan error, 1)
				go func() { _, e := bb_ability_task.ExecuteNextQueuedTask(context.Background()); done <- e }()
				sawRunningBusiness := false
				// Test synchronization only, not a product task/step timeout.
				for finished := false; !finished; {
					select {
					case e := <-done:
						if e != nil {
							t.Fatal(e)
						}
						finished = true
					case <-time.After(10 * time.Millisecond):
						var n int64
						if e := db.Model(&bb_ability_task_model.AsyncTaskStep{}).Where("task_id = ? AND step_kind = ? AND step = ? AND completed_at IS NULL", id, bb_ability_task.StepKindBusiness, StepCLIExecution).Count(&n).Error; e != nil {
							t.Fatal(e)
						}
						if n > 0 {
							sawRunningBusiness = true
						}
					}
				}
				if outcome != "unconfigured" && !sawRunningBusiness {
					t.Fatal("no read-only running business stage observed")
				}
				var task bb_ability_task_model.AsyncTask
				if e := db.Where("task_id = ?", id).Take(&task).Error; e != nil {
					t.Fatal(e)
				}
				var steps []bb_ability_task_model.AsyncTaskStep
				if e := db.Where("task_id = ?", id).Order("step_seq").Find(&steps).Error; e != nil {
					t.Fatal(e)
				}
				expected := bb_ability_task.ProcessStatusSuccess
				if outcome != "success" {
					expected = bb_ability_task.ProcessStatusSystemError
				}
				if task.RunStatus != bb_ability_task.RunStatusDone || task.ProcessStatus != expected {
					t.Fatalf("task: %+v", task)
				}
				business := []bb_ability_task_model.AsyncTaskStep{}
				for _, step := range steps {
					if step.CompletedAt == nil {
						t.Fatalf("open terminal step: %+v", step)
					}
					if step.StepKind == bb_ability_task.StepKindBusiness {
						business = append(business, step)
					}
				}
				if outcome == "unconfigured" {
					if len(business) != 1 || business[0].Step != StepInputPreparation {
						t.Fatal(business)
					}
					assertBusinessStatus(t, business[0], StatusPreparationFailed)
				} else {
					if len(business) != 3 || business[0].Step != StepInputPreparation || business[1].Step != StepCLIExecution || business[2].Step != StepArtifactCollection {
						t.Fatal(business)
					}
					assertBusinessStatus(t, business[0], StatusInputStaged)
					if outcome == "success" {
						assertBusinessStatus(t, business[1], StatusProcessExited)
						assertBusinessStatus(t, business[2], StatusCIFFilesFound)
					} else {
						assertBusinessStatus(t, business[1], StatusProcessFailed)
						assertBusinessStatus(t, business[2], StatusNoCIFFiles)
					}
				}
			})
		}
	}
}
func assertBusinessStatus(t *testing.T, step bb_ability_task_model.AsyncTaskStep, want string) {
	t.Helper()
	var statuses []string
	if e := json.Unmarshal([]byte(step.BusinessStatuses), &statuses); e != nil {
		t.Fatal(e)
	}
	for _, status := range statuses {
		if status == want {
			return
		}
	}
	t.Fatalf("step %s statuses %v lack %s", step.Step, statuses, want)
}

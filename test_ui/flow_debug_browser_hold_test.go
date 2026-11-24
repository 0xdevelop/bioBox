package test_ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/policy"
)

// TestHoldLiveProtocolFixtureForBrowser is skipped unless FLOW_DEBUG_HOLD=1.
// It starts the same-origin test_ui handlers plus the four loopback protocols,
// accepts one short-script success task that writes the official smoke CIF and
// one short-script failure that exits 7 and writes run.log, then waits for a
// stop file so a Linux Chrome click script can drive /flow_debug.html.
func TestHoldLiveProtocolFixtureForBrowser(t *testing.T) {
	if os.Getenv("FLOW_DEBUG_HOLD") == "" {
		t.Skip("optional fixture for a Linux Chrome click run; set FLOW_DEBUG_HOLD=1 and FLOW_DEBUG_HOLD_DIR")
	}
	outDir := os.Getenv("FLOW_DEBUG_HOLD_DIR")
	if outDir == "" {
		t.Fatal("FLOW_DEBUG_HOLD_DIR is required")
	}
	if err := os.MkdirAll(outDir, 0700); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path")
	}
	sampleCIF := filepath.Join(filepath.Dir(thisFile), "testdata", "sqlite_smoke_sample_0.cif")
	script := writeProxyFixtureFile(t, root, "hold-cli.sh", `#!/bin/sh
input=""; out=""
while [ "$#" -gt 0 ]; do
 case "$1" in --input) input="$2"; shift 2;; --out_dir) out="$2"; shift 2;; *) shift;; esac
done
read -r mode < "$input" || true
case "$mode" in
 fail) printf 'fixture_exit_7\n' >&2; exit 7;;
esac
mkdir -p "$out"
cp -- "`+sampleCIF+`" "$out/pred.cif"
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}

	fixture := newLiveProtocolFixture(t, script, root)
	stopPolicy := policy.PolicyServicesStart()
	t.Cleanup(stopPolicy)

	input := writeProxyFixtureFile(t, root, "quick.json", "ok\n")
	accepted := fixture.call(t, "jsonrpc", "protenix", map[string]interface{}{
		"input": input, "out_dir": "out", "version": "v1",
	}, true, false)
	taskID, _ := accepted["task_id"].(string)
	if taskID == "" || accepted["run_status"] != "queued" {
		t.Fatal("short-script task was not durably accepted")
	}
	if task := fixture.waitDone(t, taskID, 10*time.Second); task["process_status"] != "success" {
		t.Fatalf("fixture task status=%v", task["process_status"])
	}

	failInput := writeProxyFixtureFile(t, root, "fail.json", "fail\n")
	failAccepted := fixture.call(t, "jsonrpc", "protenix", map[string]interface{}{
		"input": failInput, "out_dir": "out", "version": "v1",
	}, true, false)
	failID, _ := failAccepted["task_id"].(string)
	if failID == "" || failAccepted["run_status"] != "queued" {
		t.Fatal("fail short-script task was not durably accepted")
	}
	failTask := fixture.waitDone(t, failID, 10*time.Second)
	if failTask["process_status"] != "system_error" {
		t.Fatalf("fixture fail task status=%v error=%v", failTask["process_status"], failTask["error_message"])
	}
	artifacts := fixture.call(t, "jsonrpc", "task.artifacts", map[string]interface{}{"task_id": failID}, true, false)
	items, _ := artifacts["artifacts"].([]interface{})
	hasLog := false
	for _, raw := range items {
		item, _ := raw.(map[string]interface{})
		if item["path"] == "run.log" {
			hasLog = true
			break
		}
	}
	if !hasLog {
		t.Fatalf("fixture fail task omitted run.log: %v", artifacts)
	}

	keyFile := filepath.Join(outDir, "key.ptxk")
	if err := os.WriteFile(keyFile, []byte(fixture.key), 0600); err != nil {
		t.Fatal(err)
	}
	stopFile := filepath.Join(outDir, "stop")
	manifest := map[string]string{
		"page_url":        fixture.proxy.URL + "/flow_debug.html",
		"task_id":         taskID,
		"failed_task_id":  failID,
		"input_path":      input,
		"fail_input_path": failInput,
		"key_file":        keyFile,
		"stop_file":       stopFile,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(outDir, "manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(stopFile); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("stop file was not written before hold timeout")
}

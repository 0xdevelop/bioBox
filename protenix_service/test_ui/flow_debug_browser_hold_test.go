package test_ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/0xdevelop/protenix_service/policy"
)

// TestHoldLiveProtocolFixtureForBrowser is skipped unless FLOW_DEBUG_HOLD=1.
// It starts the same-origin test_ui handlers plus the four loopback protocols,
// accepts one short-script task that writes the official smoke CIF, then waits
// for a stop file so a Linux Chrome click script can drive /flow_debug.html.
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
	script := writeProxyFixtureFile(t, root, "hold-cli.sh", "#!/bin/sh\n"+
		"input=\"\"; out=\"\"\n"+
		"while [ \"$#\" -gt 0 ]; do\n"+
		" case \"$1\" in --input) input=\"$2\"; shift 2;; --out_dir) out=\"$2\"; shift 2;; *) shift;; esac\n"+
		"done\n"+
		"mkdir -p \"$out\"\n"+
		"cp -- \""+sampleCIF+"\" \"$out/pred.cif\"\n")
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

	keyFile := filepath.Join(outDir, "key.ptxk")
	if err := os.WriteFile(keyFile, []byte(fixture.key), 0600); err != nil {
		t.Fatal(err)
	}
	stopFile := filepath.Join(outDir, "stop")
	manifest := map[string]string{
		"page_url":   fixture.proxy.URL + "/flow_debug.html",
		"task_id":    taskID,
		"input_path": input,
		"key_file":   keyFile,
		"stop_file":  stopFile,
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

package api_jsonRPC

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/protenix_service/ability"
	"github.com/0xdevelop/protenix_service/ability/ability_user"
	"github.com/0xdevelop/protenix_service/api/api_auth/api_auth_config"
	"github.com/0xdevelop/protenix_service/api/api_config"
	"github.com/0xdevelop/protenix_service/api/api_jsonRPC/api_jsonRPC_handler"
	"github.com/0xdevelop/protenix_service/config"
	"github.com/0xdevelop/protenix_service/policy"
	"github.com/0xdevelop/protenix_service/policy/policy_config"
	"github.com/0xdevelop/protenix_service/storage"
	"github.com/0xdevelop/protenix_service/storage/storage_config"
)

type queueHTTPFixture struct {
	server    *httptest.Server
	key, root string
}

func newQueueHTTPFixture(t *testing.T, binary, root string) *queueHTTPFixture {
	t.Helper()
	if root == "" {
		root = t.TempDir()
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	previousConfig, previousAuth, previousAPI, previousPolicy := config.GlobalConfig, api_auth_config.CurrentCfgAuth, api_config.CurrentApiCfg, policy_config.CurrentCfgPolicy
	t.Cleanup(func() {
		_ = storage.CloseStorage()
		config.GlobalConfig = previousConfig
		api_auth_config.CurrentCfgAuth = previousAuth
		api_config.CurrentApiCfg = previousAPI
		policy_config.CurrentCfgPolicy = previousPolicy
	})
	config.GlobalConfig = &config.FileConfig{
		StorageCfg: storage_config.Default(filepath.Join(root, "queue.db")),
		CliCfg:     &config.CLIConfig{WorkRoot: filepath.Join(root, "jobs"), CancelGraceSeconds: 1, Protenix: &config.CLIBinaryConfig{Binary: binary, ExtraEnv: map[string]string{}}},
	}
	api_auth_config.CurrentCfgAuth = &api_auth_config.AuthConfig{Session: &api_auth_config.SessionConfig{
		JWTSigningSecret: "disposable-test-signing-secret-at-least-32-bytes", Issuer: "queue-test", Audience: "queue-test-client", AccessTokenTTLSeconds: 600, RefreshTokenTTLSeconds: 1200,
	}}
	api_config.CurrentApiCfg = &api_config.ApiConfig{}
	policy_config.CurrentCfgPolicy = &policy_config.PolicyConfig{PolicyDuration: "30ms", WorkersScaller: 1, MaxWorkers: 2}
	if err := storage.OpenStorage(); err != nil {
		t.Fatal(err)
	}
	if err := storage.GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Existing template fixture path: seed only this disposable DB via the User API.
	// The actual calls below log in, issue a key and pass the normal HTTP gate.
	if _, err = ability_user.Create(db, "queue_fixture", "queue-fixture@example.invalid", "disposable-fixture-password", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	ability.LoadAbilityAPIMethods()
	server := httptest.NewServer(http.HandlerFunc(api_jsonRPC_handler.APIJsonRPCHandler))
	t.Cleanup(server.Close)
	f := &queueHTTPFixture{server: server, root: root}
	login := f.call(t, "auth.login.email", map[string]interface{}{"login_method": "password", "email": "queue-fixture@example.invalid", "password": "disposable-fixture-password"}, "", false)
	issued := f.call(t, "auth.api_key.issue", map[string]interface{}{"name": "queue-acceptance"}, login["jwt_token"].(string), false)
	f.key = issued["api_key"].(string)
	if !strings.HasPrefix(f.key, "ptxk_") {
		t.Fatal("instance credential prefix changed")
	}
	return f
}

func (f *queueHTTPFixture) call(t *testing.T, name string, args map[string]interface{}, credential string, wantError bool) map[string]interface{} {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]interface{}{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, f.server.URL+"/", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "sqlite-queue-acceptance/1.0")
	req.Header.Set("Content-Type", "application/json")
	if credential != "" {
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err = json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || len(envelope.Error) > 0 && string(envelope.Error) != "null" || envelope.Result.IsError != wantError || len(envelope.Result.Content) != 1 {
		// Do not log response bodies: auth responses can contain disposable credentials.
		t.Fatalf("%s: HTTP=%d protocol_error=%t isError=%t expected=%t content=%d", name, resp.StatusCode, len(envelope.Error) > 0, envelope.Result.IsError, wantError, len(envelope.Result.Content))
	}
	var value map[string]interface{}
	if err = json.Unmarshal([]byte(envelope.Result.Content[0].Text), &value); err != nil {
		t.Fatalf("%s JSON result: %v", name, err)
	}
	return value
}

func (f *queueHTTPFixture) task(t *testing.T, id string) map[string]interface{} {
	t.Helper()
	return f.call(t, "task.get", map[string]interface{}{"task_id": id}, f.key, false)["task"].(map[string]interface{})
}
func (f *queueHTTPFixture) waitDone(t *testing.T, id string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task := f.task(t, id)
		if task["run_status"] == "done" {
			return task
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("task %s did not finish within %s", id, timeout)
	return nil
}
func writeQueueInput(t *testing.T, root, name, value string) string {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.WriteFile(p, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSQLiteHTTPFaultAndProcessGroupCancellation(t *testing.T) {
	root := t.TempDir()
	script := writeQueueInput(t, root, "fixture-cli.sh", `#!/bin/sh
input=""; out=""
while [ "$#" -gt 0 ]; do
 case "$1" in --input) input="$2"; shift 2;; --out_dir) out="$2"; shift 2;; *) shift;; esac
done
read -r mode < "$input" || true
case "$mode" in
 fail) printf 'fixture_exit_7\n' >&2; exit 7;;
 cancel) echo $$ > parent.pid; sleep 300 & child=$!; echo "$child" > child.pid; wait "$child";;
 quick) sleep 0.3; mkdir -p "$out"; printf 'data_fixture\n' > "$out/fixture.cif";;
esac
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	f := newQueueHTTPFixture(t, script, root)
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ability_user.Create(db, "other_fixture", "other-fixture@example.invalid", "disposable-fixture-password", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	otherLogin := f.call(t, "auth.login.email", map[string]interface{}{"login_method": "password", "email": "other-fixture@example.invalid", "password": "disposable-fixture-password"}, "", false)
	otherIssued := f.call(t, "auth.api_key.issue", map[string]interface{}{"name": "other-fixture"}, otherLogin["jwt_token"].(string), false)
	otherKey := otherIssued["api_key"].(string)
	queuedInput := writeQueueInput(t, root, "queued.json", "quick\n")
	queued := f.call(t, "protenix", map[string]interface{}{"input": queuedInput, "out_dir": "out"}, f.key, false)["task_id"].(string)
	queuedCancel := f.call(t, "task.cancel", map[string]interface{}{"task_id": queued}, f.key, false)
	if queuedCancel["cancelled"] != true || f.task(t, queued)["process_status"] != "cancelled" {
		t.Fatal("queued cancellation did not persist")
	}
	f.call(t, "protenix", map[string]interface{}{"input": "unused", "out_dir": "out"}, "", true)
	f.call(t, "protenix", map[string]interface{}{"input": "unused", "out_dir": "out"}, "ptxk_invalid", true)
	stop := policy.PolicyServicesStart()
	t.Cleanup(stop)
	submit := func(mode string) string {
		input := writeQueueInput(t, root, mode+".json", mode+"\n")
		v := f.call(t, "protenix", map[string]interface{}{"input": input, "out_dir": "out", "version": "v1"}, f.key, false)
		if v["run_status"] != "queued" {
			t.Fatal("not durably accepted")
		}
		return v["task_id"].(string)
	}
	fault := submit("fail")
	if task := f.waitDone(t, fault, 10*time.Second); task["process_status"] != "system_error" {
		t.Fatalf("CLI nonzero falsely succeeded: %v", task["process_status"])
	} else {
		result, _ := task["result"].(map[string]interface{})
		if result == nil {
			t.Fatal("system_error task.get omitted Execute summary")
		}
		artifacts := f.call(t, "task.artifacts", map[string]interface{}{"task_id": fault}, f.key, false)
		items, _ := artifacts["artifacts"].([]interface{})
		var logPath string
		for _, raw := range items {
			item, _ := raw.(map[string]interface{})
			path, _ := item["path"].(string)
			if path == "run.log" {
				logPath = path
				break
			}
		}
		if logPath != "run.log" {
			t.Fatalf("failed task artifacts omitted run.log: %v", artifacts)
		}
		file := f.call(t, "task.artifact", map[string]interface{}{"task_id": fault, "path": logPath}, f.key, false)
		content, _ := file["content"].(string)
		decoded, decErr := base64.StdEncoding.DecodeString(content)
		if file["kind"] != "log" || file["encoding"] != "base64" || decErr != nil || !bytes.Equal(decoded, []byte("fixture_exit_7\n")) {
			t.Fatal("failed task log was not readable")
		}
	}
	cancelled := submit("cancel")
	quick := submit("quick")
	job := filepath.Join(root, "jobs", cancelled)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(job, "child.pid")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var pids []int
	for _, name := range []string{"parent.pid", "child.pid"} {
		b, err := os.ReadFile(filepath.Join(job, name))
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}
	f.call(t, "task.get", map[string]interface{}{"task_id": cancelled}, otherKey, true)
	f.call(t, "task.cancel", map[string]interface{}{"task_id": cancelled}, otherKey, true)
	if f.task(t, cancelled)["run_status"] != "running" {
		t.Fatal("foreign caller changed running task")
	}
	result := f.call(t, "task.cancel", map[string]interface{}{"task_id": cancelled}, f.key, false)
	if result["cancelling"] != true || result["cancelled"] != false {
		t.Fatal("running cancellation must not announce completion early")
	}
	if task := f.waitDone(t, cancelled, 10*time.Second); task["process_status"] != "cancelled" {
		t.Fatalf("cancel status %v", task["process_status"])
	}
	for _, pid := range pids {
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			fields := strings.Fields(string(b)[strings.LastIndex(string(b), ")")+1:])
			if len(fields) > 0 && fields[0] != "Z" {
				t.Fatalf("task process survived cancellation: %d", pid)
			}
		}
	}
	if task := f.waitDone(t, quick, 10*time.Second); task["process_status"] != "success" {
		t.Fatalf("other task affected: %v", task["process_status"])
	}
	t.Log("HTTP Bearer ptxk_ -> SQLite -> policy -> Execute: CLI exit 7 failed; running cancel stopped shell+child; other task completed")
}

// Opt in explicitly to one small real GPU prediction; the ordinary suite never runs a model.
func TestSQLiteHTTPOfficialInference(t *testing.T) {
	binary := os.Getenv("PTX_ACCEPT_BINARY")
	if binary == "" {
		t.Skip("set PTX_ACCEPT_BINARY for a bounded real inference")
	}
	root := os.Getenv("PTX_ACCEPT_ROOT")
	if root == "" {
		root = t.TempDir()
	}
	f := newQueueHTTPFixture(t, binary, root)
	config.GlobalConfig.CliCfg.Protenix.ExtraEnv = map[string]string{
		"PROTENIX_ROOT_DIR": os.Getenv("PTX_ACCEPT_DATA"), "LAYERNORM_TYPE": "torch", "OMP_NUM_THREADS": "2",
	}
	policy_config.CurrentCfgPolicy.MaxWorkers = 1
	stop := policy.PolicyServicesStart()
	t.Cleanup(stop)
	// A malformed JSON used to exit zero upstream. It must not become task success.
	bad := writeQueueInput(t, root, "bad.json", "{ not-json\n")
	start := time.Now()
	request := map[string]interface{}{"input": bad, "out_dir": "out", "version": "v1", "use_msa": false, "use_default_params": false, "cycle": 1, "step": 2, "sample": 1, "seeds": "101", "model_name": "protenix_base_default_v1.0.0"}
	id := f.call(t, "protenix", request, f.key, false)["task_id"].(string)
	task := f.waitDone(t, id, 2*time.Minute)
	if task["process_status"] == "success" {
		t.Fatal("malformed upstream result became success")
	}
	t.Logf("official malformed input: task=%s status=%s elapsed=%s", id, task["process_status"], time.Since(start).Round(time.Millisecond))
	input := writeQueueInput(t, root, "tiny.json", `[{"name":"sqlite_smoke","sequences":[{"proteinChain":{"sequence":"ACDEFGHIKLMNPQRSTVWY","count":1}}]}]`)
	request["input"] = input
	start = time.Now()
	id = f.call(t, "protenix", request, f.key, false)["task_id"].(string)
	task = f.waitDone(t, id, 3*time.Minute)
	if task["process_status"] != "success" {
		t.Fatalf("official inference status=%s error=%v task=%s", task["process_status"], task["error_message"], id)
	}
	result, ok := task["result"].(map[string]interface{})
	if !ok || result["cif_count"].(float64) < 1 || result["has_err"] != false || result["exit_code"].(float64) != 0 {
		t.Fatalf("invalid official artifacts: %v", result)
	}
	t.Logf("official HTTP inference: task=%s elapsed=%s CIF=%v out=%s (smoke settings, not scientific-quality prediction)", id, time.Since(start).Round(time.Millisecond), result["cif_count"], result["out_dir"])
}

func TestSQLiteHTTPPxdesignShortScript(t *testing.T) {
	root := t.TempDir()
	script := writeQueueInput(t, root, "pxdesign-cli.sh", `#!/bin/sh
input=""; out=""
while [ "$#" -gt 0 ]; do
 case "$1" in
  pipeline) shift;;
  --input) input="$2"; shift 2;;
  --dump_dir) out="$2"; shift 2;;
  --preset) shift 2;;
  *) shift;;
 esac
done
if [ -z "$out" ]; then
 exit 3
fi
mkdir -p "$out"
printf 'pxdesign_fixture_cif\n' > "$out/design.cif"
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	f := newQueueHTTPFixture(t, script, root)
	config.GlobalConfig.CliCfg.PXDesign = &config.CLIBinaryConfig{Binary: script, ExtraEnv: map[string]string{}}
	stop := policy.PolicyServicesStart()
	t.Cleanup(stop)

	input := writeQueueInput(t, root, "task.yaml", "name: pxdesign-http-fixture\n")
	f.call(t, "pxdesign", map[string]interface{}{"input": input, "dump_dir": "out", "preset": "preview"}, "", true)
	f.call(t, "pxdesign", map[string]interface{}{"input": input, "dump_dir": "out", "preset": "preview"}, "ptxk_invalid", true)

	accepted := f.call(t, "pxdesign", map[string]interface{}{"input": input, "dump_dir": "out", "preset": "preview"}, f.key, false)
	id, _ := accepted["task_id"].(string)
	if id == "" || accepted["run_status"] != "queued" {
		t.Fatal("pxdesign short-script task was not durably accepted")
	}
	task := f.waitDone(t, id, 10*time.Second)
	if task["process_status"] != "success" {
		t.Fatalf("pxdesign status=%v error=%v", task["process_status"], task["error_message"])
	}
	result, ok := task["result"].(map[string]interface{})
	if !ok || result["cif_count"].(float64) != 1 || result["has_err"] != false || result["exit_code"].(float64) != 0 {
		t.Fatalf("invalid pxdesign artifacts: %v", result)
	}
	artifacts := f.call(t, "task.artifacts", map[string]interface{}{"task_id": id}, f.key, false)
	items, _ := artifacts["artifacts"].([]interface{})
	var cifPath string
	for _, raw := range items {
		item, _ := raw.(map[string]interface{})
		path, _ := item["path"].(string)
		if strings.HasSuffix(strings.ToLower(path), ".cif") {
			cifPath = path
			break
		}
	}
	if cifPath != "output/design.cif" {
		t.Fatalf("unexpected cif path %q artifacts=%v", cifPath, artifacts)
	}
	file := f.call(t, "task.artifact", map[string]interface{}{"task_id": id, "path": cifPath}, f.key, false)
	// content is base64 in HTTP artifact method
	if file["kind"] != "cif" {
		t.Fatalf("artifact kind=%v", file["kind"])
	}
	t.Log("HTTP Bearer ptxk_ -> SQLite -> policy -> pxdesign Execute: short-script CIF listed and fetchable; missing and forged keys were rejected")
}

func TestSQLiteHTTPExecuteTimeoutStopsProcessGroup(t *testing.T) {
	root := t.TempDir()
	script := writeQueueInput(t, root, "timeout-cli.sh", `#!/bin/sh
input=""; out=""
while [ "$#" -gt 0 ]; do
 case "$1" in --input) input="$2"; shift 2;; --out_dir) out="$2"; shift 2;; *) shift;; esac
done
read -r mode < "$input" || true
case "$mode" in
 hang) echo $$ > parent.pid; sleep 300 & child=$!; echo "$child" > child.pid; wait "$child";;
 quick) sleep 0.2; mkdir -p "$out"; printf 'timeout_ok\n' > "$out/fixture.cif";;
esac
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	f := newQueueHTTPFixture(t, script, root)
	config.GlobalConfig.CliCfg.ExecuteTimeoutSeconds = 1
	stop := policy.PolicyServicesStart()
	t.Cleanup(stop)

	hangInput := writeQueueInput(t, root, "hang.json", "hang\n")
	hangID := f.call(t, "protenix", map[string]interface{}{"input": hangInput, "out_dir": "out", "version": "v1"}, f.key, false)["task_id"].(string)
	job := filepath.Join(root, "jobs", hangID)
	pidDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(pidDeadline) {
		if _, err := os.Stat(filepath.Join(job, "child.pid")); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var pids []int
	for _, name := range []string{"parent.pid", "child.pid"} {
		b, err := os.ReadFile(filepath.Join(job, name))
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}

	task := f.waitDone(t, hangID, 10*time.Second)
	if task["process_status"] != "system_error" || task["error_message"] != "task execution timed out" {
		t.Fatalf("timeout status=%v message=%v", task["process_status"], task["error_message"])
	}
	if task["cancel_requested"] == true {
		t.Fatal("timeout must not be recorded as a user cancel")
	}
	for _, pid := range pids {
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
			fields := strings.Fields(string(b)[strings.LastIndex(string(b), ")")+1:])
			if len(fields) > 0 && fields[0] != "Z" {
				t.Fatalf("task process survived execute timeout: %d", pid)
			}
		}
	}

	quickInput := writeQueueInput(t, root, "quick.json", "quick\n")
	quickID := f.call(t, "protenix", map[string]interface{}{"input": quickInput, "out_dir": "out", "version": "v1"}, f.key, false)["task_id"].(string)
	if quick := f.waitDone(t, quickID, 10*time.Second); quick["process_status"] != "success" {
		t.Fatalf("job within timeout was affected: %v %v", quick["process_status"], quick["error_message"])
	}
	t.Log("HTTP Bearer ptxk_ -> SQLite -> policy -> Execute: timeout stopped shell+child; shorter job still succeeded")
}

package api_jsonRPC

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/protenix_service/ability/ability_user"
	"github.com/0xdevelop/protenix_service/policy"
	"github.com/0xdevelop/protenix_service/storage"
)

func TestSQLiteHTTPTaskArtifactsOwnerScoped(t *testing.T) {
	root := t.TempDir()
	script := writeQueueInput(t, root, "artifact-cli.sh", `#!/bin/sh
input=""; out=""
while [ "$#" -gt 0 ]; do
 case "$1" in --input) input="$2"; shift 2;; --out_dir) out="$2"; shift 2;; *) shift;; esac
done
mkdir -p "$out/seed"
printf 'data_artifact\n' > "$out/seed/pred.cif"
printf '{"ok":true}\n' > "$out/seed/confidence.json"
printf 'not-an-artifact\n' > "$out/seed/weights.bin"
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	f := newQueueHTTPFixture(t, script, root)
	db, err := storage.GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ability_user.Create(db, "other_artifact", "other-artifact@example.invalid", "disposable-fixture-password", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	otherLogin := f.call(t, "auth.login.email", map[string]interface{}{"login_method": "password", "email": "other-artifact@example.invalid", "password": "disposable-fixture-password"}, "", false)
	otherKey := f.call(t, "auth.api_key.issue", map[string]interface{}{"name": "other-artifact"}, otherLogin["jwt_token"].(string), false)["api_key"].(string)

	stop := policy.PolicyServicesStart()
	t.Cleanup(stop)
	input := writeQueueInput(t, root, "artifact.json", "ok\n")
	id := f.call(t, "protenix", map[string]interface{}{"input": input, "out_dir": "out", "version": "v1"}, f.key, false)["task_id"].(string)
	if task := f.waitDone(t, id, 10*time.Second); task["process_status"] != "success" {
		t.Fatalf("fixture task status=%v", task["process_status"])
	}

	f.call(t, "task.artifacts", map[string]interface{}{"task_id": id}, "", true)
	f.call(t, "task.artifacts", map[string]interface{}{"task_id": id}, "ptxk_invalid", true)
	f.call(t, "task.artifacts", map[string]interface{}{"task_id": id}, otherKey, true)
	f.call(t, "task.artifact", map[string]interface{}{"task_id": id, "path": "output/seed/pred.cif"}, otherKey, true)

	listed := f.call(t, "task.artifacts", map[string]interface{}{"task_id": id}, f.key, false)
	items, _ := listed["artifacts"].([]interface{})
	paths := map[string]map[string]interface{}{}
	for _, raw := range items {
		item := raw.(map[string]interface{})
		paths[item["path"].(string)] = item
	}
	if _, ok := paths["output/seed/pred.cif"]; !ok {
		t.Fatalf("cif missing: %v", listed)
	}
	if _, ok := paths["output/seed/confidence.json"]; !ok {
		t.Fatalf("json missing: %v", listed)
	}
	if _, ok := paths["run.log"]; !ok {
		t.Fatalf("run.log missing: %v", listed)
	}
	for _, blocked := range []string{"input/" + filepath.Base(input), "output/seed/weights.bin"} {
		if _, ok := paths[blocked]; ok {
			t.Fatalf("blocked path listed: %s", blocked)
		}
	}
	for _, item := range items {
		if path, _ := item.(map[string]interface{})["path"].(string); filepath.IsAbs(path) {
			t.Fatalf("absolute path leaked: %s", path)
		}
	}

	got := f.call(t, "task.artifact", map[string]interface{}{"task_id": id, "path": "output/seed/pred.cif"}, f.key, false)
	decoded, err := base64.StdEncoding.DecodeString(got["content"].(string))
	if err != nil || string(decoded) != "data_artifact\n" || got["encoding"] != "base64" || got["kind"] != "cif" {
		t.Fatalf("artifact content mismatch: %v", got)
	}
	f.call(t, "task.artifact", map[string]interface{}{"task_id": id, "path": "../pred.cif"}, f.key, true)
	f.call(t, "task.artifact", map[string]interface{}{"task_id": id, "path": "input/" + filepath.Base(input)}, f.key, true)
	f.call(t, "task.artifact", map[string]interface{}{"task_id": id, "path": "output/seed/weights.bin"}, f.key, true)
	t.Log("HTTP Bearer ptxk_ task.artifacts/task.artifact: owner can list and read confined outputs; input, foreign user, and traversal are rejected")
}

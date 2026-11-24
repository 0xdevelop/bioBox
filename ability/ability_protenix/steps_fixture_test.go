package ability_protenix

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xdevelop/bioBox/ability/ability_user"
	"github.com/0xdevelop/bioBox/api/api_auth/api_auth_api_key"
	"github.com/0xdevelop/bioBox/api/api_auth/api_auth_config"
	"github.com/0xdevelop/bioBox/api/api_auth/api_auth_session"
	"github.com/0xdevelop/bioBox/config"
	"github.com/0xdevelop/bioBox/storage"
	"github.com/goccy/go-json"
)

// TestPrepareStepBrowserFixture prepares an isolated, explicitly non-inference service
// acceptance fixture on request. Normal suites skip it; it never starts a server,
// sends mail, edits real config, or prints a credential. Re-run into a NEW empty path.
func TestPrepareStepBrowserFixture(t *testing.T) {
	root := os.Getenv("STEP_FIXTURE_DIR")
	if root == "" {
		t.Skip("STEP_FIXTURE_DIR not set")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("STEP_FIXTURE_DIR must be absolute")
	}
	if _, e := os.Stat(filepath.Join(root, "conf", "config.yaml")); !os.IsNotExist(e) {
		t.Fatal("fixture already exists; use a new empty directory")
	}
	if e := os.MkdirAll(filepath.Join(root, "conf"), 0700); e != nil {
		t.Fatal(e)
	}
	previous, previousAuth := config.GlobalConfig, api_auth_config.CurrentCfgAuth
	t.Cleanup(func() {
		_ = storage.CloseStorage()
		config.GlobalConfig = previous
		api_auth_config.CurrentCfgAuth = previousAuth
	})
	secret := make([]byte, 32)
	if _, e := rand.Read(secret); e != nil {
		t.Fatal(e)
	}
	password := hex.EncodeToString(secret)
	cfg := map[string]interface{}{
		"storage_cfg": map[string]interface{}{"primary": "sqlite", "storage_sqlite": map[string]interface{}{"enabled": true, "path": filepath.Join(root, "fixture.db")}, "storage_mysql": map[string]interface{}{"enabled": false}, "storage_postgresql": map[string]interface{}{"enabled": false}, "storage_redis": map[string]interface{}{"enabled": false}},
		"api_cfg": map[string]interface{}{
			"api_cfg_jsonRPC":   map[string]interface{}{"enabled": true, "bind_address": "127.0.0.1", "port": 24001, "encryption_enabled": false},
			"api_cfg_mcp":       map[string]interface{}{"enabled": true, "bind_address": "127.0.0.1", "port": 24002, "mcp_transport_type": "streamable-http"},
			"api_cfg_websocket": map[string]interface{}{"enabled": true, "bind_address": "127.0.0.1", "port": 24004, "allowed_origins": []string{"127.0.0.1:*", "localhost:*"}},
			"api_cfg_grpc":      map[string]interface{}{"enabled": true, "bind_address": "127.0.0.1", "port": 24005}},
		"test_ui_cfg":  map[string]interface{}{"enabled": true, "bind_address": "127.0.0.1", "port": 24003},
		"frontend_cfg": map[string]interface{}{"enabled": false},
		"policy_cfg":   map[string]interface{}{"policy_duration": "1s", "workers_scaller": 1, "max_workers": 1, "task_lifecycle_timeout_enabled": false, "task_execute_retry_limit_enabled": false},
		"cli_cfg":      map[string]interface{}{"work_root": filepath.Join(root, "jobs"), "cancel_grace_seconds": 1, "execute_timeout_enabled": false, "protenix": map[string]interface{}{"binary": filepath.Join(root, "fixture-cli.sh")}, "pxdesign": map[string]interface{}{"binary": filepath.Join(root, "fixture-cli.sh")}},
		"auth_cfg":     map[string]interface{}{"enabled": true, "auth_type": "jwt", "session": map[string]interface{}{"jwt_signing_secret": password, "issuer": "step-fixture", "audience": "step-fixture", "access_token_ttl_seconds": 900, "refresh_token_ttl_seconds": 3600}},
	}
	raw, e := json.MarshalIndent(cfg, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	// JSON is a YAML subset. This is a NEW fixture config, not example_files/config_local.yaml.
	configPath := filepath.Join(root, "conf", "config.yaml")
	if e = os.WriteFile(configPath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if e = config.LoadConfig(configPath); e != nil {
		t.Fatal(e)
	}
	if e = storage.OpenStorage(); e != nil {
		t.Fatal(e)
	}
	if e = storage.GormAutoMigrate(); e != nil {
		t.Fatal(e)
	}
	db, e := storage.GormDB(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ability_user.Create(db, "step_fixture", "step-fixture@example.invalid", password, time.Now().UTC()); e != nil {
		t.Fatal(e)
	}
	login, e := api_auth_session.AuthLoginEmail(context.Background(), map[string]interface{}{"login_method": "password", "email": "step-fixture@example.invalid", "password": password})
	if e != nil {
		t.Fatal("fixture login failed")
	}
	token := login.(map[string]interface{})["jwt_token"].(string)
	ctx, e := api_auth_session.AuthenticateRequest(api_auth_session.WithBearerToken(context.Background(), "Bearer "+token), map[string]interface{}{})
	if e != nil {
		t.Fatal("fixture session auth failed")
	}
	issued, e := api_auth_api_key.AuthAPIKeyIssue(ctx, map[string]interface{}{"name": "step-fixture-only"})
	if e != nil {
		t.Fatal("fixture key issue failed")
	}
	key := issued.(map[string]interface{})["api_key"].(string)
	if e = os.WriteFile(filepath.Join(root, "fixture-api-key.txt"), []byte(key), 0600); e != nil {
		t.Fatal(e)
	}
	script := `#!/bin/sh
# NON-INFERENCE fixture: sleeps, optionally writes a tiny CIF marker, no GPU/model.
set -eu
out="";input="";prev=""
for arg in "$@";do
 if [ "$prev" = "--out_dir" ] || [ "$prev" = "--dump_dir" ];then out="$arg";fi
 if [ "$prev" = "--input" ];then input="$arg";fi
 prev="$arg"
done
printf 'fixture-cli: NON-INFERENCE; waiting 8 seconds\n'
sleep 8
case "$input" in *fail*) echo 'fixture-cli intentional exit 3' >&2;exit 3;; esac
mkdir -p "$out"
printf 'data_fixture_NOT_INFERENCE\n' > "$out/fixture.cif"
echo 'fixture-cli complete: marker only, not a predicted structure'
`
	if e = os.WriteFile(filepath.Join(root, "fixture-cli.sh"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"input-ok.json", "input-fail.json"} {
		if e = os.WriteFile(filepath.Join(root, name), []byte(`{"fixture":"NON-INFERENCE"}`), 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log("isolated non-inference fixture prepared; credential is in mode-0600 fixture-api-key.txt (not logged)")
}

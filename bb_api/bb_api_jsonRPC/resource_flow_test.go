package bb_api_jsonRPC

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
	"github.com/0xdevelop/bioBox/bb_config"
	"github.com/0xdevelop/bioBox/bb_policy"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestAuthenticatedResourceConfirmationToInference(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "cli-started")
	script := writeQueueInput(t, root, "fake-cli", `#!/bin/sh
out=""
while [ "$#" -gt 0 ];do case "$1" in --out_dir)out="$2";shift 2;;*)shift;;esac;done
printf started > '`+marker+`'
mkdir -p "$out";printf data_fixture > "$out/model.cif"
`)
	if err := os.Chmod(script, 0700); err != nil {
		t.Fatal(err)
	}
	f := newQueueHTTPFixture(t, script, root)
	body := bytes.Repeat([]byte("weights"), 1024)
	sum := sha256.Sum256(body)
	var gets, other atomic.Int64
	entered := make(chan struct{})
	release := make(chan struct{})
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		if r.Method == "HEAD" {
			return
		}
		if r.URL.Path != "/chosen" {
			other.Add(1)
			w.Write(body)
			return
		}
		gets.Add(1)
		w.Write(body[:512])
		w.(http.Flusher).Flush()
		close(entered)
		<-release
		w.Write(body[512:])
	}))
	defer origin.Close()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	cfg := &bb_resource.Config{Enabled: true, Files: []bb_resource.File{{Path: filepath.Join(root, "model"), URL: origin.URL + "/chosen", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}}, Download: bb_resource.Options{DownloadEnabled: true}}
	bb_config.GlobalConfig.CliCfg.Protenix.Resources = cfg
	bb_config.GlobalConfig.CliCfg.Protenix.VersionRuntimes = map[string]*bb_config.CLIRuntimeConfig{"v2": {Binary: script, Resources: &bb_resource.Config{Enabled: true, Files: []bb_resource.File{{Path: filepath.Join(root, "other"), URL: origin.URL + "/other", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}}, Download: bb_resource.Options{DownloadEnabled: true}}}}
	f.call(t, "resource.status", map[string]interface{}{"set": "protenix.v1"}, "", true)
	s := f.call(t, "resource.status", map[string]interface{}{"set": "protenix.v1"}, f.key, false)
	if s["state"] != "awaiting_confirmation" {
		t.Fatal("not pending")
	}
	token := s["confirmation_token"].(string)
	if gets.Load()+other.Load() != 0 {
		t.Fatal("status downloaded")
	}
	stop := bb_policy.PolicyServicesStart()
	defer stop()
	ids := []string{}
	for i := 0; i < 2; i++ {
		accepted := f.call(t, "resource.prepare", map[string]interface{}{"set": "protenix.v1", "confirmation_token": token}, f.key, false)
		ids = append(ids, accepted["task_id"].(string))
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no resource worker")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("inference during download")
	}
	deadline := time.Now().Add(3 * time.Second)
	observed := false
	for time.Now().Before(deadline) {
		s = f.call(t, "resource.status", map[string]interface{}{"set": "protenix.v1"}, f.key, false)
		if s["state"] == "downloading" && s["downloaded_bytes"].(float64) > 0 {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatal("no observable real byte progress")
	}
	close(release)
	released = true
	for _, id := range ids {
		done := f.waitDone(t, id, 5*time.Second)
		if done["process_status"] != "success" {
			t.Fatal("resource task failed")
		}
	}
	if gets.Load() != 1 || other.Load() != 0 {
		t.Fatal("resource ownership/scope violated", gets.Load(), other.Load())
	}
	s = f.call(t, "resource.status", map[string]interface{}{"set": "protenix.v1"}, f.key, false)
	if s["state"] != "ready" {
		t.Fatal("not ready")
	}
	input := writeQueueInput(t, root, "input.json", "[]")
	accepted := f.call(t, "protenix", map[string]interface{}{"input": input, "out_dir": "result"}, f.key, false)
	task := f.waitDone(t, accepted["task_id"].(string), 5*time.Second)
	if task["process_status"] != "success" {
		t.Fatal("ready inference failed")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("CLI never started", err)
	}
	if gets.Load() != 1 || other.Load() != 0 {
		t.Fatal("inference redownloaded")
	}
}

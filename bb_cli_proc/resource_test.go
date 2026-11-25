//go:build linux

package bb_cli_proc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
)

func TestResourceGateBeforeFakeCLI(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "started")
	script := filepath.Join(root, "cli")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf started > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	body := bytes.Repeat([]byte("verified"), 128)
	sum := sha256.Sum256(body)
	entered := make(chan struct{})
	release := make(chan struct{})
	var gets atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		if r.Method == "HEAD" {
			return
		}
		gets.Add(1)
		close(entered)
		<-release
		w.Write(body)
	}))
	defer srv.Close()
	cfg := &bb_resource.Config{Enabled: true, Files: []bb_resource.File{{Path: "weights/file", URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}}, Download: bb_resource.Options{DownloadEnabled: true}}
	s := Spec{Path: script, Dir: root, ResourceRoot: root, Resources: cfg}
	if _, err := Run(context.Background(), s); !errors.Is(err, bb_resource.ErrConfirmationRequired) {
		t.Fatal(err)
	}
	if gets.Load() != 0 {
		t.Fatal("downloaded without confirmation")
	}
	cfg.Download.Confirmed = true
	done := make(chan error, 1)
	go func() { _, err := Run(context.Background(), s); done <- err }()
	<-entered
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("CLI launched during download")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("CLI did not launch after ready", err)
	}
	if gets.Load() != 1 {
		t.Fatal(gets.Load())
	}
}

func TestCancellationAtReadyDoesNotStartCLI(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "started")
	script := filepath.Join(root, "cli")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf started > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	body := []byte("ready")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		if r.Method != "HEAD" {
			w.Write(body)
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := &bb_resource.Config{Enabled: true, Files: []bb_resource.File{{Path: "weight", URL: srv.URL, SHA256: hex.EncodeToString(sum[:])}}, Download: bb_resource.Options{DownloadEnabled: true, Confirmed: true, Progress: func(p bb_resource.Progress) {
		if p.Phase == "ready" {
			cancel()
		}
	}}}
	result, err := Run(ctx, Spec{Path: script, ResourceRoot: root, Resources: cfg})
	if result != nil {
		t.Fatalf("cancelled gate created process: pid=%d", result.PID)
	}
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("cancelled ready callback launched CLI")
	}
}

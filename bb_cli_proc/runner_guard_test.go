package bb_cli_proc

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRunRejectsRemoteHost(t *testing.T) {
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip(err)
	}
	_, err = Run(context.Background(), Spec{Name: "protenix", Path: truePath, Host: "gpu.example"})
	if err == nil || !strings.Contains(err.Error(), "cross-host file transfer") || !strings.Contains(err.Error(), "gpu.example") {
		t.Fatalf("expected remote-host guard, got %v", err)
	}
}

func TestRunRecordsProcessGroup(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("unix job directory layout")
	}
	truePath, err := exec.LookPath("true")
	if err != nil {
		t.Skip(err)
	}
	jobDir := t.TempDir()
	_, err = Run(context.Background(), Spec{Path: truePath, Dir: jobDir})
	if err != nil {
		t.Fatalf("local run: %v", err)
	}
	pgid, err := ReadJobProcessGroup(jobDir)
	if err != nil || pgid <= 0 {
		t.Fatalf("cli.pgid: pgid=%d err=%v", pgid, err)
	}
	if ProcessGroupAliveInDir(pgid, jobDir) {
		t.Fatal("finished /bin/true should not look alive")
	}
}

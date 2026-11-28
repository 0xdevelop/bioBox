//go:build unix && !linux

package bb_cli_proc

import (
	"strings"
	"syscall"
)

// ProcessGroupAliveInDir degrades to a process-group liveness probe on non-Linux
// Unix. Those platforms do not expose a stable /proc/<pgid>/cwd check.
func ProcessGroupAliveInDir(pgid int, jobDir string) bool {
	if pgid <= 0 || strings.TrimSpace(jobDir) == "" {
		return false
	}
	err := syscall.Kill(-pgid, 0)
	return err == nil || err == syscall.EPERM
}

// SignalTermProcessGroup sends SIGTERM to a local process group without waiting.
func SignalTermProcessGroup(pgid int) error {
	return signalProcessGroup(pgid, termSignal())
}

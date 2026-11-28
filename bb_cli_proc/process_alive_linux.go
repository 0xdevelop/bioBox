//go:build linux

package bb_cli_proc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// ProcessGroupAliveInDir reports whether the process group is still alive and
// the group leader's cwd is the given job directory. The cwd check prevents a
// recycled pid from being treated as the original job.
func ProcessGroupAliveInDir(pgid int, jobDir string) bool {
	if pgid <= 0 || strings.TrimSpace(jobDir) == "" {
		return false
	}
	if err := syscall.Kill(-pgid, 0); err != nil && err != syscall.EPERM {
		return false
	}
	cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pgid))
	if err != nil {
		return false
	}
	return samePath(cwd, jobDir)
}

// SignalTermProcessGroup sends SIGTERM to a local process group without waiting.
func SignalTermProcessGroup(pgid int) error {
	return signalProcessGroup(pgid, termSignal())
}

func samePath(left, right string) bool {
	resolve := func(p string) string {
		cleaned := filepath.Clean(p)
		resolved, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			return cleaned
		}
		return resolved
	}
	return resolve(left) == resolve(right)
}

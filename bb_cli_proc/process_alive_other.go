//go:build !unix

package bb_cli_proc

import "fmt"

// ProcessGroupAliveInDir is not implemented off Unix; recovery falls back to
// the historical requeue path when liveness cannot be proven.
func ProcessGroupAliveInDir(pgid int, jobDir string) bool { return false }

// SignalTermProcessGroup is not implemented off Unix.
func SignalTermProcessGroup(pgid int) error {
	return fmt.Errorf("process-group signals are only implemented on unix")
}

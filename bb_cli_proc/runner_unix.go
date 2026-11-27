//go:build unix

package bb_cli_proc

import (
	"syscall"
	"time"
)

func newSysProcAttr() (*syscall.SysProcAttr, error) {
	return &syscall.SysProcAttr{Setpgid: true}, nil
}

func termSignal() syscall.Signal { return syscall.SIGTERM }

func killSignal() syscall.Signal { return syscall.SIGKILL }

func signalProcessGroup(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}
	err := syscall.Kill(-pid, sig)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func signalProcess(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}
	err := syscall.Kill(pid, sig)
	if err == syscall.ESRCH {
		return nil
	}
	return err
}

func reapLeftoverProcessGroup(pgid int, grace time.Duration) bool {
	if !processGroupHasMembers(pgid) {
		return false
	}
	_ = signalProcessGroup(pgid, termSignal())
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !processGroupHasMembers(pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = signalProcessGroup(pgid, killSignal())
	deadline = time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if !processGroupHasMembers(pgid) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

func processGroupHasMembers(pgid int) bool {
	if pgid <= 0 {
		return false
	}
	err := syscall.Kill(-pgid, 0)
	return err == nil || err == syscall.EPERM
}

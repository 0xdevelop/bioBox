package bb_cli_proc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/0xdevelop/bioBox/bb_common/bb_app_path"
	"github.com/0xdevelop/bioBox/bb_common/bb_resource"
)

var ErrCancelled = errors.New("cli process group cancelled")

// Spec 描述一次独立进程组内的命令执行。
type Spec struct {
	ResourceSet  string
	Resources    *bb_resource.Config
	ResourceRoot string
	Name         string
	Path         string
	Host         string
	Args         []string
	Env          map[string]string
	Dir          string
	LogPath      string
	Grace        time.Duration
}

// Result 是命令结束后的实测摘要，成功不能只看退出码。
type Result struct {
	PID       int
	PGID      int
	ExitCode  int
	Cancelled bool
	Duration  time.Duration
	LogPath   string
}

// InspectOutput 统计输出目录中的 CIF 与 ERR，供原生结果核验。
func InspectOutput(outDir string) (cifCount int, hasERR bool, err error) {
	if strings.TrimSpace(outDir) == "" {
		return 0, false, nil
	}
	info, statErr := os.Stat(outDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return 0, false, nil
		}
		return 0, false, statErr
	}
	if !info.IsDir() {
		return 0, false, fmt.Errorf("output path is not a directory: %s", outDir)
	}
	walkErr := filepath.WalkDir(outDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		name := entry.Name()
		if entry.IsDir() && name == "ERR" {
			hasERR = true
			return nil
		}
		if !entry.IsDir() && filepath.Ext(name) == ".cif" {
			cifCount++
		}
		return nil
	})
	return cifCount, hasERR, walkErr
}

// Run 在新进程组中启动命令。context 取消时先向整个进程组发 SIGTERM，超时后再发 SIGKILL。
// 这不是对包装子壳单独发信号。调用方若传入不会取消的 context，则本函数不会主动取消进程组。
func Run(ctx context.Context, spec Spec) (*Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: %v", ErrCancelled, ctx.Err())
	}
	if strings.TrimSpace(spec.Path) == "" {
		if strings.TrimSpace(spec.Host) != "" {
			return nil, fmt.Errorf("%s: cli binary path is required", suiteLabel(spec))
		}
		return nil, fmt.Errorf("cli binary path is required")
	}
	if err := ConfirmCommandReady(ctx, spec); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%w: %v", ErrCancelled, ctx.Err())
	}
	grace := spec.Grace
	if grace <= 0 {
		grace = 10 * time.Second
	}
	attr, err := newSysProcAttr()
	if err != nil {
		return nil, err
	}
	host := strings.TrimSpace(spec.Host)
	cmd, err := commandFor(spec)
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = attr
	var remoteErrNotes *prefixWriter
	if host != "" {
		remoteErrNotes = &prefixWriter{max: 2048}
	}
	if strings.TrimSpace(spec.LogPath) != "" {
		if err = os.MkdirAll(filepath.Dir(spec.LogPath), 0o755); err != nil {
			return nil, err
		}
		logFile, openErr := os.OpenFile(spec.LogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if openErr != nil {
			return nil, openErr
		}
		defer logFile.Close()
		if remoteErrNotes != nil {
			cmd.Stdout = logFile
			cmd.Stderr = io.MultiWriter(logFile, remoteErrNotes)
		} else {
			cmd.Stdout = logFile
			cmd.Stderr = logFile
		}
	} else if remoteErrNotes != nil {
		cmd.Stderr = remoteErrNotes
	}
	started := time.Now()
	if err = cmd.Start(); err != nil {
		if host != "" {
			return nil, fmt.Errorf("%s: cannot start ssh for %s: %w", suiteLabel(spec), host, err)
		}
		return nil, err
	}
	result := &Result{PID: cmd.Process.Pid, PGID: cmd.Process.Pid, LogPath: spec.LogPath}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case waitErr := <-done:
		reaped := reapLeftoverProcessGroup(cmd.Process.Pid, grace)
		result.Duration = time.Since(started)
		result.ExitCode = exitCode(waitErr)
		return finishRun(spec, result, waitErr, remoteErrNotes, reaped)
	case <-ctx.Done():
		result.Cancelled = true
		_ = signalProcessGroup(cmd.Process.Pid, termSignal())
		select {
		case waitErr := <-done:
			_ = reapLeftoverProcessGroup(cmd.Process.Pid, grace)
			result.Duration = time.Since(started)
			result.ExitCode = exitCode(waitErr)
			if result.ExitCode == 0 && waitErr != nil {
				return result, cancelErr(spec, waitErr)
			}
			return result, cancelErr(spec, nil)
		case <-time.After(grace):
			_ = signalProcessGroup(cmd.Process.Pid, killSignal())
			waitErr := <-done
			result.Duration = time.Since(started)
			result.ExitCode = exitCode(waitErr)
			return result, cancelErr(spec, nil)
		}
	}
}

func commandFor(spec Spec) (*exec.Cmd, error) {
	if strings.TrimSpace(spec.Host) == "" {
		cmd := exec.Command(spec.Path, spec.Args...)
		cmd.Dir = spec.Dir
		cmd.Env = os.Environ()
		for key, value := range spec.Env {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
		return cmd, nil
	}
	return sshCommand(spec)
}

// sshCommand 用本机 ssh 在 spec.Host 上执行 spec.Path。BatchMode 避免密码提示卡住；ConnectTimeout 只限制握手。
func sshCommand(spec Spec) (*exec.Cmd, error) {
	host := strings.TrimSpace(spec.Host)
	sshPath, err := exec.LookPath("ssh")
	if err != nil {
		return nil, fmt.Errorf("%s: host %q is set but the local ssh client was not found", suiteLabel(spec), host)
	}
	cmd := exec.Command(sshPath, "-o", "BatchMode=yes", "-o", "ConnectTimeout=15", "--", host, remoteShellCommand(spec))
	cmd.Env = os.Environ()
	return cmd, nil
}

func remoteShellCommand(spec Spec) string {
	var b strings.Builder
	if dir := strings.TrimSpace(spec.Dir); dir != "" {
		b.WriteString("cd ")
		b.WriteString(posixQuote(dir))
		b.WriteString(" && ")
	}
	if len(spec.Env) > 0 {
		keys := make([]string, 0, len(spec.Env))
		for key := range spec.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("env --")
		for _, key := range keys {
			b.WriteByte(' ')
			b.WriteString(posixQuote(key + "=" + spec.Env[key]))
		}
		b.WriteByte(' ')
	}
	b.WriteString(posixQuote(spec.Path))
	for _, arg := range spec.Args {
		b.WriteByte(' ')
		b.WriteString(posixQuote(arg))
	}
	return b.String()
}

func posixQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func suiteLabel(spec Spec) string {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return "cli"
	}
	return name
}

func finishRun(spec Spec, result *Result, waitErr error, notes *prefixWriter, reaped bool) (*Result, error) {
	host := strings.TrimSpace(spec.Host)
	if host == "" {
		if reaped {
			return result, fmt.Errorf("cli wrapper exited but leftover process group members were reaped: pgid=%d", result.PGID)
		}
		if waitErr != nil {
			return result, waitErr
		}
		return result, nil
	}
	if reaped {
		return result, fmt.Errorf("%s: ssh to %s exited but leftover process group members were reaped: pgid=%d", suiteLabel(spec), host, result.PGID)
	}
	if waitErr != nil {
		return result, remoteFailure(spec, host, waitErr, notes)
	}
	return result, nil
}

func cancelErr(spec Spec, extra error) error {
	if strings.TrimSpace(spec.Host) == "" {
		if extra != nil {
			return fmt.Errorf("%w: %v", ErrCancelled, extra)
		}
		return fmt.Errorf("%w", ErrCancelled)
	}
	if extra != nil {
		return fmt.Errorf("%s: %w: %v", suiteLabel(spec), ErrCancelled, extra)
	}
	return fmt.Errorf("%s: %w", suiteLabel(spec), ErrCancelled)
}

func remoteFailure(spec Spec, host string, waitErr error, notes *prefixWriter) error {
	line := ""
	if notes != nil {
		line = firstUsefulLine(notes.buf.String())
	}
	code := exitCode(waitErr)
	if code == 255 && (line == "" || sshClientDiagnostic(line)) {
		if line != "" {
			return fmt.Errorf("%s: cannot connect to %s: %s", suiteLabel(spec), host, line)
		}
		return fmt.Errorf("%s: cannot connect to %s: %w", suiteLabel(spec), host, waitErr)
	}
	if line != "" {
		return fmt.Errorf("%s: remote command on %s failed (exit %d): %s", suiteLabel(spec), host, code, line)
	}
	return fmt.Errorf("%s: remote command on %s failed: %w", suiteLabel(spec), host, waitErr)
}

func sshClientDiagnostic(line string) bool {
	if strings.HasPrefix(line, "ssh:") {
		return true
	}
	for _, mark := range []string{
		"Connection refused",
		"Connection timed out",
		"Connection closed",
		"Could not resolve",
		"No route to host",
		"Permission denied",
		"Host key verification failed",
		"Operation timed out",
	} {
		if strings.Contains(line, mark) {
			return true
		}
	}
	return false
}

func firstUsefulLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(line)
	if len(line) > 400 {
		line = line[:400]
	}
	return line
}

type prefixWriter struct {
	buf bytes.Buffer
	max int
}

func (w *prefixWriter) Write(p []byte) (int, error) {
	n := len(p)
	if w == nil || w.max <= 0 || w.buf.Len() >= w.max {
		return n, nil
	}
	if remain := w.max - w.buf.Len(); len(p) > remain {
		p = p[:remain]
	}
	_, _ = w.buf.Write(p)
	return n, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

// ConfirmCommandReady 在 exec 或 ssh 之前核对这一条命令带上的权重与运行环境阈值。
// 符合返回 nil。不符合返回错误，调用方不得启动该条进程。缺文件时先走 resource.prepare。
func ConfirmCommandReady(ctx context.Context, spec Spec) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %v", ErrCancelled, ctx.Err())
	}
	named := spec.Name == "protenix" || spec.Name == "pxdesign"
	if spec.Resources == nil || !spec.Resources.Enabled || len(spec.Resources.Files) == 0 {
		if named {
			return bb_resource.ErrSetUnavailable
		}
		return nil
	}
	if err := validateOfficialResources(spec); err != nil {
		return err
	}
	root := spec.ResourceRoot
	if root == "" {
		var err error
		root, err = bb_app_path.ExecutableDir()
		if err != nil {
			return err
		}
	}
	setID := strings.TrimSpace(spec.ResourceSet)
	if setID == "" {
		setID = spec.Name
	}
	snap, err := (bb_resource.Controller{Root: root, Sets: map[string]*bb_resource.Config{setID: spec.Resources}}).Status(ctx, setID)
	if err != nil {
		return fmt.Errorf("%s resource set %s does not match configured thresholds: %w; run resource.prepare for this set before calling; process was not started", spec.Name, setID, err)
	}
	if snap.State != "ready" {
		path := ""
		if snap.File >= 0 && snap.File < len(spec.Resources.Files) {
			path = spec.Resources.Files[snap.File].Path
		}
		return fmt.Errorf("%s resource set %s is %s at %s; run resource.prepare for this set before calling; process was not started", spec.Name, setID, snap.State, path)
	}
	return nil
}

// ResourceStatus reports preparation separately from inference task progress.
func ResourceStatus(ctx context.Context, s Spec) (bb_resource.Snapshot, error) {
	root := s.ResourceRoot
	var err error
	if root == "" {
		root, err = bb_app_path.ExecutableDir()
		if err != nil {
			return bb_resource.Snapshot{}, err
		}
	}
	return (bb_resource.Controller{Root: root, Sets: map[string]*bb_resource.Config{s.ResourceSet: s.Resources}}).Status(ctx, s.ResourceSet)
}

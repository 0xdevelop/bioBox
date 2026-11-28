package bb_cli_proc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// JobPgidFileName is the durable local process-group record under a job directory.
const JobPgidFileName = "cli.pgid"

// WriteJobProcessGroup atomically records the local process group id for a job
// directory. The file is kept after exit so recovery can still observe a leftover group.
func WriteJobProcessGroup(jobDir string, pgid int) error {
	if strings.TrimSpace(jobDir) == "" || pgid <= 0 {
		return fmt.Errorf("job directory and process group id are required")
	}
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(jobDir, JobPgidFileName)
	tmp, err := os.CreateTemp(jobDir, ".cli.pgid-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err = fmt.Fprintf(tmp, "%d\n", pgid); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}

// ReadJobProcessGroup returns the recorded process group id.
// Missing or empty files surface os.ErrNotExist.
func ReadJobProcessGroup(jobDir string) (int, error) {
	data, err := os.ReadFile(filepath.Join(jobDir, JobPgidFileName))
	if err != nil {
		return 0, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return 0, os.ErrNotExist
	}
	pgid, err := strconv.Atoi(text)
	if err != nil || pgid <= 0 {
		return 0, fmt.Errorf("invalid %s", JobPgidFileName)
	}
	return pgid, nil
}

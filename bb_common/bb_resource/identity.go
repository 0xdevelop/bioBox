package bb_resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hashCopy is the SHA-256 read path. Tests may replace it to observe short-circuiting.
var hashCopy = io.Copy

type identityStampLoc struct {
	Root  string
	Token string
}

type fileIdentityStamp struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	MtimeNsec int64  `json:"mtime_nsec"`
	CtimeNsec int64  `json:"ctime_nsec"`
	Inode     uint64 `json:"inode"`
	SHA256    string `json:"sha256"`
}

func identityStampPath(loc identityStampLoc, filePath string) string {
	if strings.TrimSpace(loc.Root) == "" || strings.TrimSpace(loc.Token) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(filePath))
	return filepath.Join(loc.Root, "data", "resource_state", loc.Token+".id", hex.EncodeToString(sum[:])+".json")
}

func currentIdentity(path string, st os.FileInfo) (fileIdentityStamp, error) {
	id := fileIdentityStamp{
		Path:      path,
		Size:      st.Size(),
		MtimeNsec: st.ModTime().UnixNano(),
		SHA256:    "",
	}
	applyPlatformIdentity(&id, st)
	return id, nil
}

func identityMatches(got, saved fileIdentityStamp) bool {
	return got.Path == saved.Path &&
		got.Size == saved.Size &&
		got.MtimeNsec == saved.MtimeNsec &&
		got.CtimeNsec == saved.CtimeNsec &&
		got.Inode == saved.Inode
}

func loadIdentityStamp(path string) (fileIdentityStamp, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileIdentityStamp{}, false
	}
	var stamp fileIdentityStamp
	if err = json.Unmarshal(data, &stamp); err != nil {
		return fileIdentityStamp{}, false
	}
	return stamp, true
}

func writeIdentityStamp(path string, stamp fileIdentityStamp) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	buf, err := json.Marshal(stamp)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".identity-")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(buf); err != nil {
		_ = tmp.Close()
		return
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return
	}
	if err = tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmpName, path)
}

func verifyWithIdentity(ctx context.Context, path string, f File, loc identityStampLoc) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	in, err := openResourceFile(path)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%w: not a regular file", ErrIntegrity)
	}
	if f.Size > 0 && st.Size() != f.Size {
		return fmt.Errorf("%w: size", ErrIntegrity)
	}
	want := strings.ToLower(f.SHA256)
	got, err := currentIdentity(path, st)
	if err != nil {
		return err
	}
	stampFile := identityStampPath(loc, path)
	if stampFile != "" {
		if saved, ok := loadIdentityStamp(stampFile); ok && identityMatches(got, saved) && strings.ToLower(saved.SHA256) == want {
			return nil
		}
	}
	h := sha256.New()
	if _, err = hashCopy(h, contextReader{ctx, in}); err != nil {
		return err
	}
	sum := strings.ToLower(hex.EncodeToString(h.Sum(nil)))
	if sum != want {
		return fmt.Errorf("%w: sha256", ErrIntegrity)
	}
	got.SHA256 = sum
	writeIdentityStamp(stampFile, got)
	return nil
}

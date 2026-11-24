package api_jsonRPC

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/0xdevelop/bioBox/common/resource"
	"os"
	"path/filepath"
	"testing"
)

// A fake CLI consumes this verified fixture; it does not claim a real model manifest.
func fixtureResources(t *testing.T) *resource.Config {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ready")
	body := []byte("fake-cli-resource")
	if err := os.WriteFile(p, body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	return &resource.Config{Enabled: true, Files: []resource.File{{Path: p, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}}}
}

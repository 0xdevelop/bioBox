package bb_resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestIdentityStampShortCircuitAndInvalidate(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "weight.bin")
	body := []byte("identity-weight-body")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	file := File{Path: path, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}
	loc := identityStampLoc{Root: root, Token: "tok-a"}

	var copies atomic.Int64
	previous := hashCopy
	t.Cleanup(func() { hashCopy = previous })
	hashCopy = func(dst io.Writer, src io.Reader) (int64, error) {
		copies.Add(1)
		return previous(dst, src)
	}

	if err := verifyWithIdentity(context.Background(), path, file, loc); err != nil {
		t.Fatalf("first hash: %v", err)
	}
	if copies.Load() != 1 {
		t.Fatalf("first verify should hash once, copies=%d", copies.Load())
	}
	stamp := identityStampPath(loc, path)
	if _, err := os.Stat(stamp); err != nil {
		t.Fatalf("stamp not written under resource_state: %v", err)
	}
	if err := verifyWithIdentity(context.Background(), path, file, loc); err != nil {
		t.Fatalf("stamp hit: %v", err)
	}
	if copies.Load() != 1 {
		t.Fatalf("identical identity should skip io.Copy, copies=%d", copies.Load())
	}

	// Different token must not reuse the previous stamp.
	if err := verifyWithIdentity(context.Background(), path, file, identityStampLoc{Root: root, Token: "tok-b"}); err != nil {
		t.Fatalf("new token: %v", err)
	}
	if copies.Load() != 2 {
		t.Fatalf("new token should rehash, copies=%d", copies.Load())
	}

	// Metadata change (content rewrite) must rehash and fail against the old digest.
	if err := os.WriteFile(path, []byte("identity-weight-XXXX"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifyWithIdentity(context.Background(), path, file, loc)
	if err == nil {
		t.Fatal("changed bytes must invalidate the stamp and fail sha256")
	}
	if copies.Load() != 3 {
		t.Fatalf("changed file should rehash, copies=%d", copies.Load())
	}
}

func TestIdentityStampWriteFailureStillHashes(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "data")
	if err := os.WriteFile(blocked, []byte("not-a-dir"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "weight.bin")
	body := []byte("ok")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	file := File{Path: path, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(body))}
	if err := verifyWithIdentity(context.Background(), path, file, identityStampLoc{Root: root, Token: "tok"}); err != nil {
		t.Fatalf("unwritable stamp dir must still verify: %v", err)
	}
}

package bb_cli_proc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageJSONInputAndAttachments(t *testing.T) {
	work := t.TempDir()
	jobDir, staged, err := StageJSONInput(work, "task-1", map[string]interface{}{"name": "ten_alanine", "file": "ligand.cif"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(jobDir, "input", "input.json")
	if staged != want {
		t.Fatalf("staged path: %s", staged)
	}
	body, err := os.ReadFile(staged)
	if err != nil || !strings.Contains(string(body), "ten_alanine") {
		t.Fatalf("input.json: %s %v", body, err)
	}
	inputDir := filepath.Join(jobDir, "input")
	if err := StageAttachments(inputDir, []interface{}{
		map[string]interface{}{"path": "ligand.cif", "content": "data_lig", "encoding": "utf8"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(inputDir, "ligand.cif"))
	if err != nil || string(got) != "data_lig" {
		t.Fatalf("utf8 attachment: %s %v", got, err)
	}
	if err := StageAttachments(inputDir, map[string]interface{}{
		"sub/a.txt": map[string]interface{}{"content": "ZGF0YQ==", "encoding": "base64"},
	}); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(filepath.Join(inputDir, "sub", "a.txt"))
	if err != nil || string(got) != "data" {
		t.Fatalf("base64 attachment: %s %v", got, err)
	}
}

func TestStageAttachmentsRejectsPathEscape(t *testing.T) {
	inputDir := filepath.Join(t.TempDir(), "input")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"../secret", "/etc/passwd", `..\windows`} {
		err := StageAttachments(inputDir, []interface{}{
			map[string]interface{}{"path": rel, "content": "x"},
		})
		if err == nil {
			t.Fatalf("path %q must be rejected", rel)
		}
	}
}

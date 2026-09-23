package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileFailurePreservesPreviousFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := WriteFile(path, []byte("valid"), 0600); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	originalRename := renameFile
	renameFile = func(_, _ string) error { return errors.New("simulated interruption") }
	t.Cleanup(func() { renameFile = originalRename })
	if err := WriteFile(path, []byte("new"), 0600); err == nil {
		t.Fatal("expected simulated write failure")
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != string(original) {
		t.Fatalf("previous file changed after failed write: %q", current)
	}
}

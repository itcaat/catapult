package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveStateIsAtomicAndPrivate(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "state.json")
	fm := NewFileManager(tempDir)
	if err := fm.SaveState(statePath); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("state permissions = %o, want 600", got)
	}

	if err := os.WriteFile(statePath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := fm.LoadState(statePath); err == nil || !strings.Contains(err.Error(), "corrupt file moved") {
		t.Fatalf("expected recoverable corrupt state error, got %v", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("corrupt state was not moved aside: %v", err)
	}
}

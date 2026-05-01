//go:build !windows

package report

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestWriteResolvedFileRejectsNonRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("mkfifo unavailable: %v", err)
	}
	if err := WriteResolvedFile(path, []byte(`{"ok":true}`)); err == nil {
		t.Fatal("expected non-regular report path to be rejected")
	}
}

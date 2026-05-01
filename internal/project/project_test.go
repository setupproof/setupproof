package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveConfigTargetRejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	resolver, err := NewResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveConfigTarget(filepath.Join(dir, "README.md")); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveConfigTargetRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	resolver, err := NewResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveConfigTarget("../README.md"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolvePositionalTargetRejectsOutsideSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "README.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	resolver, err := NewResolver(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolvePositionalTarget("README.md"); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindRootStopsAfterSensibleDepth(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	deep := dir
	for i := 0; i < maxRootWalkDepth+1; i++ {
		deep = filepath.Join(deep, "nested")
	}
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := findRoot(deep); got != deep {
		t.Fatalf("findRoot walked beyond depth cap: got %q, want %q", got, deep)
	}
}

//go:build windows

package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNativeWindowsWorkspaceCopiesGitLayoutsAndCleansTempDirs(t *testing.T) {
	dir := gitRepo(t)
	tempParent := t.TempDir()
	t.Cleanup(func() {
		_ = makeTreeWritable(dir)
		_ = makeTreeWritable(tempParent)
	})

	writeFile(t, dir, ".gitignore", "ignored.txt\n")
	writeFile(t, dir, "README.md", "```sh setupproof id=copy\ntrue\n```\n")
	writeFile(t, dir, "tracked.txt", "tracked\n")
	writeFile(t, dir, "readonly.txt", "readonly\n")
	writeFile(t, dir, "ignored.txt", "ignored\n")
	writeFile(t, dir, "untracked.txt", "untracked\n")
	gitAdd(t, dir, ".gitignore", "README.md", "tracked.txt", "readonly.txt")
	if err := os.Chmod(filepath.Join(dir, "readonly.txt"), 0o400); err != nil {
		t.Fatal(err)
	}

	source, err := loadWorkspaceSource(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if source.untrackedIncluded || source.untrackedFileCount != 0 {
		t.Fatalf("untracked source metadata = %#v", source)
	}
	for _, rel := range []string{"README.md", "tracked.txt", "readonly.txt"} {
		if !workspaceFileListed(source.files, rel) {
			t.Fatalf("tracked file %q missing from source files %#v", rel, source.files)
		}
	}
	for _, rel := range []string{"ignored.txt", "untracked.txt", ".git"} {
		if workspaceFileListed(source.files, rel) {
			t.Fatalf("excluded file %q was listed in source files %#v", rel, source.files)
		}
	}

	ws, err := createWorkspaceInDir(source, tempParent, "setupproof-local-")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(ws.repoRoot, "tracked.txt")); got != "tracked\n" {
		t.Fatalf("tracked copy = %q", got)
	}
	if _, err := os.Stat(filepath.Join(ws.repoRoot, "ignored.txt")); !os.IsNotExist(err) {
		t.Fatalf("ignored file should not be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.repoRoot, "untracked.txt")); !os.IsNotExist(err) {
		t.Fatalf("untracked file should not be copied by default: %v", err)
	}
	if err := os.Chmod(filepath.Join(ws.repoRoot, "readonly.txt"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := ws.cleanup(false); err != nil {
		t.Fatal(err)
	}
	assertNoWorkspaces(t, tempParent)

	source, err = loadWorkspaceSource(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if !source.untrackedIncluded || source.untrackedFileCount != 1 {
		t.Fatalf("include-untracked source metadata = %#v", source)
	}
	if !workspaceFileListed(source.files, "untracked.txt") {
		t.Fatalf("untracked file missing from source files %#v", source.files)
	}
	if workspaceFileListed(source.files, "ignored.txt") {
		t.Fatalf("ignored file was listed with include-untracked: %#v", source.files)
	}
}

func TestNativeWindowsWorkspaceCopiesSymlinksWhenAvailable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "target.txt", "target\n")
	if err := os.Symlink("target.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	ws, err := createWorkspaceInDir(workspaceSource{
		root:  dir,
		files: []string{"target.txt", "link.txt"},
	}, t.TempDir(), "setupproof-local-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ws.cleanup(false)
	}()

	target, err := os.Readlink(filepath.Join(ws.repoRoot, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(target) != "target.txt" {
		t.Fatalf("copied symlink target = %q", target)
	}
}

func TestNativeWindowsWorkspaceSupportsLinkedWorktreeRoots(t *testing.T) {
	root := gitRepo(t)
	configureGitIdentity(t, root)
	writeFile(t, root, "README.md", "linked root\n")
	gitAdd(t, root, "README.md")
	runGit(t, root, "commit", "-m", "initial")

	linked := filepath.Join(t.TempDir(), "linked")
	runGit(t, root, "worktree", "add", "-b", "linked-windows-test", linked)
	writeFile(t, linked, "README.md", "linked worktree\n")
	gitAdd(t, linked, "README.md")

	discovered, err := discoverGitRoot(linked)
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(t, discovered, linked) {
		t.Fatalf("discovered root = %q, want %q", discovered, linked)
	}
	source, err := loadWorkspaceSource(discovered, false)
	if err != nil {
		t.Fatal(err)
	}
	if !workspaceFileListed(source.files, "README.md") || source.trackedChanged != true {
		t.Fatalf("linked worktree source = %#v", source)
	}
}

func TestNativeWindowsWorkspaceOmitsSubmoduleGitlinks(t *testing.T) {
	submodule := gitRepo(t)
	configureGitIdentity(t, submodule)
	writeFile(t, submodule, "README.md", "submodule\n")
	gitAdd(t, submodule, "README.md")
	runGit(t, submodule, "commit", "-m", "initial")

	parent := gitRepo(t)
	configureGitIdentity(t, parent)
	writeFile(t, parent, "README.md", "parent\n")
	gitAdd(t, parent, "README.md")
	runGit(t, parent, "commit", "-m", "initial")
	runGit(t, parent, "-c", "protocol.file.allow=always", "submodule", "add", submodule, "vendor/submodule")

	source, err := loadWorkspaceSource(parent, false)
	if err != nil {
		t.Fatal(err)
	}
	if source.submoduleFileCount != 1 {
		t.Fatalf("submodule count = %d, want 1 in %#v", source.submoduleFileCount, source)
	}
	if workspaceFileListed(source.files, "vendor/submodule") {
		t.Fatalf("submodule gitlink should not be copied as a file: %#v", source.files)
	}
	if !workspaceFileListed(source.files, ".gitmodules") {
		t.Fatalf(".gitmodules should remain copied: %#v", source.files)
	}

	ws, err := createWorkspaceInDir(source, t.TempDir(), "setupproof-local-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ws.cleanup(false)
	}()
	if _, err := os.Stat(filepath.Join(ws.repoRoot, "vendor", "submodule")); !os.IsNotExist(err) {
		t.Fatalf("submodule checkout should be omitted from copied workspace: %v", err)
	}
}

func samePath(t *testing.T, left string, right string) bool {
	t.Helper()
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

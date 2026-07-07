package runner

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceSourceOmitsSubmoduleGitlinks(t *testing.T) {
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
	if _, err := filepath.EvalSymlinks(filepath.Join(ws.repoRoot, "vendor", "submodule")); err == nil {
		t.Fatal("submodule checkout should be omitted from copied workspace")
	}
}

func configureGitIdentity(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.email", "setupproof@example.invalid")
	runGit(t, dir, "config", "user.name", "SetupProof Tests")
}

func workspaceFileListed(files []string, rel string) bool {
	want := filepath.ToSlash(rel)
	for _, file := range files {
		if strings.EqualFold(filepath.ToSlash(file), want) {
			return true
		}
	}
	return false
}

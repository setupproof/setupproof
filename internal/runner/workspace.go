package runner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type workspaceSource struct {
	root               string
	files              []string
	trackedChanged     bool
	untrackedIncluded  bool
	untrackedFileCount int
}

type workspace struct {
	tempRoot string
	repoRoot string
	homeDir  string
	tmpDir   string
}

func discoverGitRoot(cwd string) (string, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	cmd := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", gitWorkingTreeError(stderr.String())
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", gitWorkingTreeError(stderr.String())
	}
	return filepath.Abs(root)
}

func gitWorkingTreeError(detail string) error {
	message := "execution requires a Git working tree; use --dry-run --json, --list, review, suggest, or doctor for non-executing inspection"
	detail = strings.TrimSpace(detail)
	if detail != "" {
		message += ": " + detail
	}
	return fmt.Errorf("%s", message)
}

func loadWorkspaceSource(root string, includeUntracked bool) (workspaceSource, error) {
	tracked, err := gitListFiles(root, "--cached")
	if err != nil {
		return workspaceSource{}, err
	}

	files := append([]string{}, tracked...)
	untrackedCount := 0
	if includeUntracked {
		untracked, err := gitListFiles(root, "--others", "--exclude-standard")
		if err != nil {
			return workspaceSource{}, err
		}
		files = append(files, untracked...)
		untrackedCount = len(untracked)
	}

	sort.Strings(files)
	files = uniqueStrings(files)

	changed, err := trackedDiffersFromHEAD(root)
	if err != nil {
		return workspaceSource{}, err
	}

	return workspaceSource{
		root:               root,
		files:              files,
		trackedChanged:     changed,
		untrackedIncluded:  includeUntracked,
		untrackedFileCount: untrackedCount,
	}, nil
}

func gitListFiles(root string, args ...string) ([]string, error) {
	fullArgs := append([]string{"-C", root, "ls-files", "-z"}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list Git workspace files: %w", err)
	}
	parts := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(part)))
	}
	return files, nil
}

func trackedDiffersFromHEAD(root string) (bool, error) {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=no")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to inspect tracked workspace changes: %w", err)
	}
	return len(bytes.TrimSpace(out)) > 0, nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	unique := values[:0]
	var previous string
	for i, value := range values {
		if i == 0 || value != previous {
			unique = append(unique, value)
			previous = value
		}
	}
	return unique
}

func createWorkspace(source workspaceSource) (*workspace, error) {
	return createWorkspaceWithPrefix(source, "setupproof-local-")
}

func createDockerWorkspace(source workspaceSource) (*workspace, error) {
	return createWorkspaceInDir(source, dockerWorkspaceTempParent(source.root), ".setupproof-docker-")
}

func createWorkspaceWithPrefix(source workspaceSource, prefix string) (*workspace, error) {
	return createWorkspaceInDir(source, "", prefix)
}

func createWorkspaceInDir(source workspaceSource, dir string, prefix string) (*workspace, error) {
	tempRoot, err := os.MkdirTemp(dir, prefix)
	if err != nil {
		return nil, err
	}
	ws := &workspace{
		tempRoot: tempRoot,
		repoRoot: filepath.Join(tempRoot, "repo"),
		homeDir:  filepath.Join(tempRoot, "home"),
		tmpDir:   filepath.Join(tempRoot, "tmp"),
	}
	if err := os.MkdirAll(ws.repoRoot, 0o700); err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, err
	}
	if err := os.MkdirAll(ws.homeDir, 0o700); err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, err
	}
	if err := os.MkdirAll(ws.tmpDir, 0o700); err != nil {
		_ = os.RemoveAll(tempRoot)
		return nil, err
	}

	for _, rel := range source.files {
		if err := copyWorkspaceEntry(source.root, ws.repoRoot, rel); err != nil {
			_ = os.RemoveAll(tempRoot)
			return nil, err
		}
	}
	return ws, nil
}

func dockerWorkspaceTempParent(sourceRoot string) string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		parent := filepath.Join(cacheDir, "setupproof", "docker-workspaces")
		if err := os.MkdirAll(parent, 0o700); err == nil {
			return parent
		}
	}
	parent := filepath.Dir(sourceRoot)
	if parent == "." || parent == string(filepath.Separator) {
		return ""
	}
	if info, err := os.Stat(parent); err == nil && info.IsDir() {
		return parent
	}
	return ""
}

func copyWorkspaceEntry(sourceRoot string, destinationRoot string, rel string) error {
	cleanRel, err := cleanGitRelativePath(rel)
	if err != nil {
		return err
	}
	sourcePath := filepath.Join(sourceRoot, filepath.FromSlash(cleanRel))
	destinationPath := filepath.Join(destinationRoot, filepath.FromSlash(cleanRel))

	info, err := os.Lstat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copyWorkspaceSymlink(sourceRoot, destinationRoot, sourcePath, destinationPath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file or symlink", rel)
	}
	return copyRegularFile(sourcePath, destinationPath, info.Mode().Perm())
}

func cleanGitRelativePath(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("git returned an empty file path")
	}
	if strings.ContainsRune(rel, 0) {
		return "", fmt.Errorf("git returned path %q containing a NUL byte", rel)
	}
	cleaned := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("git returned path %q outside the repository root", rel)
	}
	return filepath.ToSlash(cleaned), nil
}

func copyWorkspaceSymlink(sourceRoot string, destinationRoot string, sourcePath string, destinationPath string) error {
	target, err := os.Readlink(sourcePath)
	if err != nil {
		return err
	}
	targetPath := target
	if !filepath.IsAbs(targetPath) {
		targetPath = filepath.Join(filepath.Dir(sourcePath), targetPath)
	}
	evaluatedTarget, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return fmt.Errorf("copied symlink %s cannot be resolved: %w", sourcePath, err)
	}
	evaluatedRoot, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return err
	}
	if !pathInside(evaluatedRoot, evaluatedTarget) {
		return fmt.Errorf("copied symlink %s resolves outside the repository root", sourcePath)
	}
	if filepath.IsAbs(target) {
		relTarget, err := filepath.Rel(evaluatedRoot, evaluatedTarget)
		if err != nil {
			return err
		}
		targetInCopy := filepath.Join(destinationRoot, relTarget)
		target, err = filepath.Rel(filepath.Dir(destinationPath), targetInCopy)
		if err != nil {
			return err
		}
	}
	return os.Symlink(target, destinationPath)
}

func copyRegularFile(sourcePath string, destinationPath string, mode os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (ws *workspace) cleanup(keep bool) error {
	if keep {
		return nil
	}
	if err := makeTreeWritable(ws.tempRoot); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(ws.tempRoot)
}

func makeTreeWritable(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if entry.IsDir() {
			mode |= 0o700
		} else {
			mode |= 0o600
		}
		return os.Chmod(path, mode)
	})
}

func pathInside(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

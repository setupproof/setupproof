package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ConfigFileName = "setupproof.yml"
const maxRootWalkDepth = 64

type Resolver struct {
	Root string
	CWD  string
}

type ResolvedFile struct {
	Input string
	Abs   string
	Rel   string
}

func NewResolver(cwd string) (Resolver, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return Resolver{}, err
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return Resolver{}, err
	}
	return Resolver{Root: findRoot(abs), CWD: abs}, nil
}

func (r Resolver) DiscoverConfig() (ResolvedFile, bool, error) {
	path := filepath.Join(r.Root, ConfigFileName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ResolvedFile{}, false, nil
		}
		return ResolvedFile{}, false, err
	}
	resolved, err := r.resolveExisting(path, path)
	if err != nil {
		return ResolvedFile{}, false, err
	}
	return resolved, true, nil
}

func (r Resolver) ResolveConfig(input string) (ResolvedFile, error) {
	if input == "" {
		return ResolvedFile{}, fmt.Errorf("config path must not be empty")
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.CWD, path)
	}
	return r.resolveExisting(input, path)
}

func (r Resolver) ResolvePositionalTarget(input string) (ResolvedFile, error) {
	if input == "" {
		return ResolvedFile{}, fmt.Errorf("target path must not be empty")
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.CWD, path)
	}
	return r.resolveExisting(input, path)
}

func (r Resolver) ResolveConfigTarget(input string) (ResolvedFile, error) {
	if input == "" {
		return ResolvedFile{}, fmt.Errorf("config file entry must not be empty")
	}
	if filepath.IsAbs(input) {
		return ResolvedFile{}, fmt.Errorf("config file entry %q must be repository-root-relative", input)
	}
	cleaned := filepath.Clean(input)
	if cleaned == "." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return ResolvedFile{}, fmt.Errorf("config file entry %q escapes the repository root", input)
	}
	return r.resolveExisting(input, filepath.Join(r.Root, cleaned))
}

func (r Resolver) RelForConfigPath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("config block file must not be empty")
	}
	if filepath.IsAbs(input) {
		return "", fmt.Errorf("config block file %q must be repository-root-relative", input)
	}
	cleaned := filepath.Clean(input)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("config block file %q escapes the repository root", input)
	}
	return filepath.ToSlash(cleaned), nil
}

func (r Resolver) resolveExisting(input string, path string) (ResolvedFile, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ResolvedFile{}, err
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return ResolvedFile{}, err
	}
	root, err := filepath.EvalSymlinks(r.Root)
	if err != nil {
		return ResolvedFile{}, err
	}
	if !inside(root, evaluated) {
		return ResolvedFile{}, fmt.Errorf("%q resolves outside the repository root", input)
	}
	info, err := os.Stat(evaluated)
	if err != nil {
		return ResolvedFile{}, err
	}
	if info.IsDir() {
		return ResolvedFile{}, fmt.Errorf("%q is a directory, not a file", input)
	}
	rel, err := filepath.Rel(root, evaluated)
	if err != nil {
		return ResolvedFile{}, err
	}
	return ResolvedFile{
		Input: input,
		Abs:   evaluated,
		Rel:   filepath.ToSlash(rel),
	}, nil
}

func findRoot(cwd string) string {
	dir := cwd
	for depth := 0; depth < maxRootWalkDepth; depth++ {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd
		}
		dir = parent
	}
	return cwd
}

func inside(root string, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func DetectRoot(ctx context.Context, start string) (string, bool, error) {
	if start == "" {
		start = "."
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false, fmt.Errorf("absolute root: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	isGit, err := ProbeGitRepository(ctx, abs)
	if err != nil {
		return "", false, err
	}
	if isGit {
		if root, ok := gitRoot(ctx, abs); ok {
			return root, true, nil
		}
		return "", false, errors.New("cannot resolve Git repository root")
	}
	real, err := canonicalOrAbsolute(abs)
	if err != nil {
		return "", false, fmt.Errorf("resolve root: %w", err)
	}
	return real, false, nil
}

func gitRoot(ctx context.Context, start string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", "-C", start, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	root, err := filepath.Abs(strings.TrimSpace(string(out)))
	if err != nil {
		return "", false
	}
	return filepath.Clean(root), true
}

func IsGitRepository(ctx context.Context, root string) bool {
	found, _ := ProbeGitRepository(ctx, root)
	return found
}

// ProbeGitRepository distinguishes a missing/broken Git installation from a
// genuinely non-Git directory. Never weaken Git's exclusions after a failure.
func ProbeGitRepository(ctx context.Context, root string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--is-inside-work-tree").Output()
	if err == nil {
		return strings.TrimSpace(string(out)) == "true", nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	abs, pathErr := filepath.Abs(root)
	if pathErr != nil {
		return false, pathErr
	}
	for directory := abs; ; directory = filepath.Dir(directory) {
		if _, markerErr := os.Lstat(filepath.Join(directory, ".git")); markerErr == nil {
			return false, fmt.Errorf("probe Git repository: %w", err)
		} else if !errors.Is(markerErr, os.ErrNotExist) {
			return false, markerErr
		}
		if filepath.Dir(directory) == directory {
			break
		}
	}
	var exitErr *exec.ExitError
	if errors.Is(err, exec.ErrNotFound) || (errors.As(err, &exitErr) && exitErr.ExitCode() == 128) {
		return false, nil
	}
	return false, fmt.Errorf("probe Git repository: %w", err)
}

var errOutsideRoot = errors.New("path is outside repository root")

func ContainedPath(root, candidate string) (string, error) {
	realRoot, err := canonicalOrAbsolute(root)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	realCandidate, err := canonicalOrAbsolute(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve candidate: %w", err)
	}
	rel, err := filepath.Rel(realRoot, realCandidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errOutsideRoot
	}
	return filepath.Clean(realCandidate), nil
}

func canonicalOrAbsolute(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(real), nil
	}
	if _, err := os.Stat(abs); err != nil {
		return "", err
	}
	// A few Windows-protected temporary locations deny the final-path query
	// while allowing normal file access. Preserve containment checks with the
	// absolute path when canonicalization is unavailable.
	return filepath.Clean(abs), nil
}

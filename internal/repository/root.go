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
	if gitRoot, ok := gitRoot(ctx, abs); ok {
		return gitRoot, true, nil
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
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && string(out) == "true\n"
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

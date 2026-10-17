package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareIndexDirectory resolves existing parents before creating anything.
// Returning the real directory keeps DB and lock users on the same path.
func PrepareIndexDirectory(ctx context.Context, root, relative string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if relative == "" || filepath.IsAbs(relative) {
		return "", errors.New("index directory must be relative to repository root")
	}
	realRoot, err := physicalPath(root)
	if err != nil {
		return "", err
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(realRoot, filepath.Clean(relative))
	inside := func(path string) bool {
		rel, err := filepath.Rel(realRoot, path)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if !inside(candidate) {
		return "", errors.New("index directory escapes repository root")
	}
	parent := candidate
	for {
		if _, err = os.Lstat(parent); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", errors.New("index directory has no existing parent")
		}
		parent = next
	}
	resolvedParent, err := physicalPath(parent)
	if err != nil {
		return "", fmt.Errorf("resolve index parent: %w", err)
	}
	suffix, err := filepath.Rel(parent, candidate)
	if err != nil {
		return "", err
	}
	candidate = filepath.Join(resolvedParent, suffix)
	if !inside(candidate) {
		return "", errors.New("index directory resolves outside repository root")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(candidate, 0700); err != nil {
		return "", fmt.Errorf("create index directory: %w", err)
	}
	resolved, err := physicalPath(candidate)
	if err != nil {
		return "", err
	}
	if !inside(resolved) {
		return "", errors.New("index directory resolves outside repository root")
	}
	return resolved, nil
}

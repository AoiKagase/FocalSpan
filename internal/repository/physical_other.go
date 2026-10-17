//go:build !windows

package repository

import "path/filepath"

func physicalPath(path string) (string, error) { return filepath.EvalSymlinks(path) }

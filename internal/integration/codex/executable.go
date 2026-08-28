package codex

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ResolveExecutable(command string, allowGoBuild bool) (string, string, error) {
	if strings.IndexByte(command, 0) >= 0 {
		return "", "", errors.New("executable path contains NUL")
	}
	var path string
	if command != "" {
		if strings.ContainsAny(command, `/\`) {
			abs, err := filepath.Abs(command)
			if err != nil {
				return "", "", fmt.Errorf("resolve executable path: %w", err)
			}
			path = abs
		} else {
			found, err := exec.LookPath(command)
			if err != nil {
				return "", "", fmt.Errorf("find executable %q: %w", command, err)
			}
			path = found
		}
	} else {
		found, err := os.Executable()
		if err != nil {
			return "", "", fmt.Errorf("resolve current executable: %w", err)
		}
		path = found
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("absolute executable path: %w", err)
	}
	path = filepath.Clean(abs)
	if strings.IndexByte(path, 0) >= 0 {
		return "", "", errors.New("executable path contains NUL")
	}
	if isGoBuildPath(path) {
		if !allowGoBuild {
			return "", "", fmt.Errorf("refusing temporary go-build executable %q; build or install a persistent binary and pass it with --command", path)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", fmt.Errorf("stat executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("executable path is not a regular file: %q", path)
	}
	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		return "", "", fmt.Errorf("executable is not executable: %q", path)
	}
	warning := ""
	if isGoBuildPath(path) {
		warning = "resolved executable is a temporary go-build binary; use a persistent binary for registration"
	}
	return path, warning, nil
}

func isGoBuildPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.HasPrefix(strings.ToLower(part), "go-build") {
			return true
		}
	}
	return false
}

package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type CommandResult struct {
	Stdout []byte
	Stderr []byte
}

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (CommandResult, error)
}

type StreamCommandRunner interface {
	Stream(ctx context.Context, dir, name string, stdout io.Writer, args ...string) ([]byte, error)
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, dir, name string, args ...string) (CommandResult, error) {
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return CommandResult{Stdout: stdout.Bytes(), Stderr: capped(stderr.Bytes(), 16<<10)}, err
}

func (ExecCommandRunner) Stream(ctx context.Context, dir, name string, stdout io.Writer, args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	command.Stdout = stdout
	command.Stderr = &stderr
	err := command.Run()
	return capped(stderr.Bytes(), 16<<10), err
}

func ResolveCommit(ctx context.Context, runner CommandRunner, repositoryPath, ref string) (string, error) {
	result, err := runner.Run(ctx, repositoryPath, "git", "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve git ref: %w: %s", err, strings.TrimSpace(string(capped(result.Stderr, 16<<10))))
	}
	commit := strings.TrimSpace(string(result.Stdout))
	if len(commit) != 40 {
		return "", fmt.Errorf("resolve git ref: unexpected commit id %q", commit)
	}
	return commit, nil
}

func capped(value []byte, limit int) []byte {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

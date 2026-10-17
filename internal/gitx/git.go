package gitx

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Client struct{ Root string }

func NewClient(root string) *Client { return &Client{Root: filepath.Clean(root)} }

func (c *Client) ListFiles(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", c.Root, "ls-files", "-co", "--exclude-standard", "-z")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	files := make([]string, 0)
	for _, part := range strings.Split(string(out), "\x00") {
		if part != "" {
			files = append(files, filepath.ToSlash(filepath.Clean(part)))
		}
	}
	sort.Strings(files)
	return files, nil
}

type DiffRequest struct {
	Base     string
	Head     string
	Staged   bool
	Unstaged bool
}

func (c *Client) Diff(ctx context.Context, req DiffRequest) ([]ChangedFile, error) {
	args := []string{"-C", c.Root, "diff", "--unified=0", "--no-ext-diff"}
	if req.Base != "" || req.Head != "" {
		if req.Base != "" {
			args = append(args, req.Base)
		}
		if req.Head != "" {
			args = append(args, req.Head)
		}
	}
	args = append(args, "--")
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	files, err := ParseUnifiedZeroDiff(out)
	if err != nil {
		return nil, err
	}
	if req.Base == "" && req.Head == "" && !req.Staged {
		return files, nil
	}
	if req.Base == "" && req.Head == "" && req.Staged {
		cmd = exec.CommandContext(ctx, "git", "-C", c.Root, "diff", "--cached", "--unified=0", "--no-ext-diff", "--")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git staged diff: %w", err)
		}
		return ParseUnifiedZeroDiff(out)
	}
	return files, nil
}

// WorkingTreeDiff compares the current source to HEAD so staged and unstaged
// changes share the current worktree's line coordinates.
func (c *Client) WorkingTreeDiff(ctx context.Context) ([]ChangedFile, error) {
	err := exec.CommandContext(ctx, "git", "-C", c.Root, "rev-parse", "--verify", "--quiet", "HEAD").Run()
	if err == nil {
		return c.Diff(ctx, DiffRequest{Base: "HEAD"})
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		return nil, fmt.Errorf("resolve Git HEAD: %w", err)
	}
	// An unborn repository has no baseline: every tracked file is new, so all
	// current lines are changed, including lines added after staging the file.
	staged, err := c.Diff(ctx, DiffRequest{Staged: true})
	if err != nil {
		return nil, err
	}
	unstaged, err := c.Diff(ctx, DiffRequest{})
	if err != nil {
		return nil, err
	}
	files := append(staged, unstaged...)
	for i := range files {
		files[i].Ranges = []LineRange{{Start: 1, End: int(^uint(0) >> 1)}}
	}
	return files, nil
}

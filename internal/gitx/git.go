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
	args := []string{"-C", c.Root, "diff", "--unified=0", "--no-ext-diff", "--no-textconv"}
	if req.Base != "" || req.Head != "" {
		if req.Base != "" {
			base, err := c.resolveCommit(ctx, req.Base)
			if err != nil {
				return nil, err
			}
			args = append(args, base)
		}
		if req.Head != "" {
			head, err := c.resolveCommit(ctx, req.Head)
			if err != nil {
				return nil, err
			}
			args = append(args, head)
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
		cmd = exec.CommandContext(ctx, "git", "-C", c.Root, "diff", "--cached", "--unified=0", "--no-ext-diff", "--no-textconv", "--")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git staged diff: %w", err)
		}
		return ParseUnifiedZeroDiff(out)
	}
	return files, nil
}

func (c *Client) resolveCommit(ctx context.Context, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") || strings.ContainsRune(ref, 0) {
		return "", fmt.Errorf("invalid Git reference %q", ref)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", c.Root, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}").Output()
	if err != nil {
		return "", fmt.Errorf("resolve Git reference %q: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// WorkingTreeDiff compares the current source to HEAD so staged and unstaged
// changes share the current worktree's line coordinates.
func (c *Client) WorkingTreeDiff(ctx context.Context) ([]ChangedFile, error) {
	err := exec.CommandContext(ctx, "git", "-C", c.Root, "rev-parse", "--verify", "--quiet", "HEAD").Run()
	if err == nil {
		files, err := c.Diff(ctx, DiffRequest{Base: "HEAD"})
		if err != nil {
			return nil, err
		}
		return c.withUntracked(ctx, files)
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
	return c.withUntracked(ctx, files)
}

func (c *Client) withUntracked(ctx context.Context, files []ChangedFile) ([]ChangedFile, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", c.Root, "ls-files", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git untracked files: %w", err)
	}
	result := make([]ChangedFile, 0, len(files))
	seen := make(map[string]int)
	for _, file := range files {
		if index, ok := seen[file.Path]; ok {
			result[index].Ranges = append(result[index].Ranges, file.Ranges...)
		} else {
			seen[file.Path] = len(result)
			result = append(result, file)
		}
	}
	for _, path := range strings.Split(string(out), "\x00") {
		if path == "" {
			continue
		}
		path = filepath.ToSlash(filepath.Clean(path))
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = len(result)
		result = append(result, ChangedFile{Path: path, Kind: ChangeAdd, Ranges: []LineRange{{Start: 1, End: int(^uint(0) >> 1)}}})
	}
	return result, nil
}

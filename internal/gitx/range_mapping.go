package gitx

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type lineEdit struct{ oldStart, oldCount, newStart, newCount int }
type fileLineMapping struct {
	path    string
	deleted bool
	edits   []lineEdit
}

var mappingHunk = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// DiffForImpact pins both refs before computing head-to-current coordinates.
// One-ref comparisons already refer to worktree lines and must not be remapped.
func (c *Client) DiffForImpact(ctx context.Context, base, head string) ([]ChangedFile, error) {
	if base == "" || head == "" {
		return c.Diff(ctx, DiffRequest{Base: base, Head: head})
	}
	baseID, err := c.resolveCommit(ctx, base)
	if err != nil {
		return nil, err
	}
	headID, err := c.resolveCommit(ctx, head)
	if err != nil {
		return nil, err
	}
	changed, err := c.Diff(ctx, DiffRequest{Base: baseID, Head: headID})
	if err != nil {
		return nil, err
	}
	return c.MapRangesToWorktree(ctx, headID, changed)
}

func (c *Client) MapRangesToWorktree(ctx context.Context, head string, changed []ChangedFile) ([]ChangedFile, error) {
	headID, err := c.resolveCommit(ctx, head)
	if err != nil {
		return nil, err
	}
	out, err := exec.CommandContext(ctx, "git", "-C", c.Root, "diff", "--unified=0", "--no-ext-diff", "--no-textconv", "--find-renames", headID, "--").Output()
	if err != nil {
		return nil, fmt.Errorf("align Git impact source: %w", err)
	}
	files, err := ParseUnifiedZeroDiff(out)
	if err != nil {
		return nil, fmt.Errorf("align Git impact source: %w", err)
	}
	mappings := make([]fileLineMapping, len(files))
	byPath := make(map[string]int, len(files))
	for i, file := range files {
		old := file.OldPath
		if old == "" {
			old = file.Path
		}
		byPath[old] = i
		mappings[i] = fileLineMapping{path: file.Path, deleted: file.Kind == ChangeDelete}
	}
	index := -1
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			index++
			continue
		}
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		if index < 0 || index >= len(mappings) {
			return nil, fmt.Errorf("align Git impact: hunk outside file")
		}
		match := mappingHunk.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("align Git impact: invalid hunk")
		}
		numbers := [4]int{}
		for n := 0; n < 4; n++ {
			if match[n+1] == "" {
				numbers[n] = 1
				continue
			}
			numbers[n], err = strconv.Atoi(match[n+1])
			if err != nil {
				return nil, err
			}
		}
		mappings[index].edits = append(mappings[index].edits, lineEdit{numbers[0], numbers[1], numbers[2], numbers[3]})
	}
	result := make([]ChangedFile, 0, len(changed))
	for _, file := range changed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if i, ok := byPath[file.Path]; ok {
			mapping := mappings[i]
			if mapping.deleted {
				continue
			}
			ranges, err := mapLineRanges(file.Ranges, mapping.edits)
			if err != nil {
				return nil, fmt.Errorf("align Git impact %s: %w", file.Path, err)
			}
			if file.Path != mapping.path {
				file.OldPath = file.Path
				file.Kind = ChangeRename
			}
			file.Path = mapping.path
			file.Ranges = ranges
		}
		result = append(result, file)
	}
	return result, nil
}

func mapLineRanges(ranges []LineRange, edits []lineEdit) ([]LineRange, error) {
	previousEnd, delta := 0, 0
	for _, edit := range edits {
		start := edit.oldStart
		if edit.oldCount == 0 {
			start++
		}
		newBoundary := edit.newStart
		if edit.newCount == 0 {
			newBoundary++
		}
		if edit.oldCount < 0 || edit.newCount < 0 || start < 1 || newBoundary < 1 || start < previousEnd || newBoundary != start+delta {
			return nil, fmt.Errorf("inconsistent or overlapping line edits")
		}
		previousEnd = start + edit.oldCount
		delta += edit.newCount - edit.oldCount
	}
	result := []LineRange{}
	add := func(start, end int) {
		if start < 1 {
			start = 1
		}
		if end >= start {
			result = append(result, LineRange{start, end})
		}
	}
	for _, r := range ranges {
		if r.Start < 1 || r.End < r.Start {
			return nil, fmt.Errorf("invalid source line range")
		}
		cursor, offset := r.Start, 0
		for _, edit := range edits {
			start := edit.oldStart
			if edit.oldCount == 0 {
				start++
			}
			end := start + edit.oldCount
			if start > r.End {
				break
			}
			if edit.oldCount > 0 && end <= cursor {
				offset += edit.newCount - edit.oldCount
				continue
			}
			if cursor < start {
				last := min(r.End, start-1)
				add(cursor+offset, last+offset)
				cursor = last + 1
			}
			if edit.oldCount == 0 {
				if start > r.Start && start <= r.End {
					add(edit.newStart, edit.newStart+edit.newCount-1)
				}
				offset += edit.newCount
				continue
			}
			if cursor <= r.End && end > cursor {
				if edit.newCount > 0 {
					add(edit.newStart, edit.newStart+edit.newCount-1)
				} else {
					anchor := max(1, edit.newStart)
					add(anchor, anchor)
				}
				cursor = max(cursor, end)
			}
			offset += edit.newCount - edit.oldCount
			if cursor > r.End {
				break
			}
		}
		if cursor <= r.End {
			add(cursor+offset, r.End+offset)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Start != result[j].Start {
			return result[i].Start < result[j].Start
		}
		return result[i].End < result[j].End
	})
	merged := result[:0]
	for _, r := range result {
		if len(merged) > 0 && r.Start <= merged[len(merged)-1].End+1 {
			merged[len(merged)-1].End = max(merged[len(merged)-1].End, r.End)
		} else {
			merged = append(merged, r)
		}
	}
	return merged, nil
}

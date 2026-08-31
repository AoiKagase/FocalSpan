package benchmark

import (
	"context"
	"fmt"
	"sort"

	"github.com/focalspan/focalspan/internal/gitx"
)

type LineRange struct{ Start, End int }
type ChangedFile struct {
	OldPath string
	NewPath string
	Status  string
	Binary  bool
	Ranges  []LineRange
}
type ChangeSet struct {
	BaseCommit   string
	TargetCommit string
	Files        []ChangedFile
}

func CollectChanges(ctx context.Context, runner CommandRunner, repositoryPath, baseRef, targetRef string) (ChangeSet, error) {
	base, err := ResolveCommit(ctx, runner, repositoryPath, baseRef)
	if err != nil {
		return ChangeSet{}, err
	}
	target, err := ResolveCommit(ctx, runner, repositoryPath, targetRef)
	if err != nil {
		return ChangeSet{}, err
	}
	result, err := runner.Run(ctx, repositoryPath, "git", "diff", "--unified=0", "--no-ext-diff", "--find-renames", base, target, "--")
	if err != nil {
		return ChangeSet{}, fmt.Errorf("git diff: %w", err)
	}
	parsed, err := gitx.ParseUnifiedZeroDiff(result.Stdout)
	if err != nil {
		return ChangeSet{}, err
	}
	files := make([]ChangedFile, 0, len(parsed))
	for _, file := range parsed {
		ranges := make([]LineRange, len(file.Ranges))
		for i, lineRange := range file.Ranges {
			ranges[i] = LineRange{Start: lineRange.Start, End: lineRange.End}
		}
		files = append(files, ChangedFile{OldPath: file.OldPath, NewPath: file.Path, Status: string(file.Kind), Binary: file.Binary, Ranges: ranges})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].NewPath < files[j].NewPath })
	return ChangeSet{BaseCommit: base, TargetCommit: target, Files: files}, nil
}

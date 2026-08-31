package benchmark

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Snapshot struct {
	RepositoryID    string
	Commit          string
	Root            string
	SkippedSymlinks []string
	FileCount       int
}

type Snapshotter interface {
	Materialize(ctx context.Context, repositoryID, repositoryPath, ref, destination string) (Snapshot, error)
}

type gitSnapshotter struct{ runner CommandRunner }

func NewGitSnapshotter(runner CommandRunner) Snapshotter { return &gitSnapshotter{runner: runner} }

func (s *gitSnapshotter) Materialize(ctx context.Context, repositoryID, repositoryPath, ref, destination string) (Snapshot, error) {
	commit, err := ResolveCommit(ctx, s.runner, repositoryPath, ref)
	if err != nil {
		return Snapshot{}, err
	}
	streamer, ok := s.runner.(StreamCommandRunner)
	if !ok {
		return Snapshot{}, fmt.Errorf("git snapshot runner does not support streaming")
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return Snapshot{}, err
	}
	reader, writer := io.Pipe()
	var stderr []byte
	var commandErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		stderr, commandErr = streamer.Stream(ctx, repositoryPath, "git", writer, "archive", "--format=tar", commit)
		_ = writer.CloseWithError(commandErr)
	}()
	count, skipped, extractErr := ExtractGitArchive(reader, destination)
	// tar.Reader stops at the end-of-archive marker while git may still be
	// writing record padding. Drain the pipe so the subprocess can exit.
	_, drainErr := io.Copy(io.Discard, reader)
	<-done
	if extractErr == nil && drainErr != nil && commandErr == nil {
		extractErr = drainErr
	}
	if extractErr != nil || commandErr != nil {
		_ = os.RemoveAll(destination)
		if extractErr != nil {
			return Snapshot{}, extractErr
		}
		return Snapshot{}, fmt.Errorf("git archive: %w: %s", commandErr, strings.TrimSpace(string(stderr)))
	}
	return Snapshot{RepositoryID: repositoryID, Commit: commit, Root: destination, SkippedSymlinks: skipped, FileCount: count}, nil
}

func ExtractGitArchive(reader io.Reader, destination string) (int, []string, error) {
	archive := tar.NewReader(reader)
	count := 0
	var skipped []string
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return count, skipped, nil
		}
		if err != nil {
			return 0, nil, fmt.Errorf("read git archive: %w", err)
		}
		clean, err := safeArchivePath(header.Name)
		if err != nil {
			return 0, nil, err
		}
		target := filepath.Join(destination, filepath.FromSlash(clean))
		switch header.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			skipped = append(skipped, clean)
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return 0, nil, err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return 0, nil, err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return 0, nil, err
			}
			_, copyErr := io.Copy(file, archive)
			closeErr := file.Close()
			if copyErr != nil {
				return 0, nil, copyErr
			}
			if closeErr != nil {
				return 0, nil, closeErr
			}
			count++
		}
	}
}

func safeArchivePath(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, 0) || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	depth := 0
	for _, part := range strings.Split(name, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			depth--
			if depth < 0 {
				return "", fmt.Errorf("unsafe archive path %q", name)
			}
		default:
			depth++
		}
	}
	clean := filepath.ToSlash(filepath.Clean(name))
	if clean == "." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return clean, nil
}

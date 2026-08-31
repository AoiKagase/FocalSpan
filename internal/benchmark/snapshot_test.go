package benchmark

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGitSnapshotAndDiffLeaveRepositoryUnchanged(t *testing.T) {
	repository := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repository}, args...)...)
		command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Benchmark Test", "GIT_AUTHOR_EMAIL=test@example.invalid", "GIT_COMMITTER_NAME=Benchmark Test", "GIT_COMMITTER_EMAIL=test@example.invalid")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	runGit("init", "-q")
	if err := os.MkdirAll(filepath.Join(repository, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "src", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "--", "src/main.go")
	runGit("commit", "-q", "-m", "base")
	base := runGit("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repository, "src", "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "--", "src/main.go")
	runGit("commit", "-q", "-m", "target")
	target := runGit("rev-parse", "HEAD")
	before := runGit("status", "--porcelain=v1") + runGit("rev-parse", "HEAD")

	snapshot, err := NewGitSnapshotter(ExecCommandRunner{}).Materialize(context.Background(), "fixture", repository, base, filepath.Join(t.TempDir(), "snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Commit != base || snapshot.FileCount != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	changes, err := CollectChanges(context.Background(), ExecCommandRunner{}, repository, base, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Files) != 1 || changes.Files[0].NewPath != "src/main.go" || changes.Files[0].Status != "modify" {
		t.Fatalf("changes = %+v", changes)
	}
	after := runGit("status", "--porcelain=v1") + runGit("rev-parse", "HEAD")
	if before != after {
		t.Fatalf("repository changed: before=%q after=%q", before, after)
	}
}

func TestExtractGitArchiveRejectsTraversal(t *testing.T) {
	for _, name := range []string{"/absolute", "C:/drive", "../escape", "safe/../../escape"} {
		t.Run(name, func(t *testing.T) {
			var data bytes.Buffer
			writer := tar.NewWriter(&data)
			_ = writer.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 1})
			_, _ = writer.Write([]byte("x"))
			_ = writer.Close()
			if _, _, err := ExtractGitArchive(bytes.NewReader(data.Bytes()), t.TempDir()); err == nil {
				t.Fatal("expected unsafe archive error")
			}
		})
	}
	if _, err := safeArchivePath("bad\x00name"); err == nil {
		t.Fatal("expected NUL path error")
	}
}

func TestExtractGitArchiveSkipsSymlink(t *testing.T) {
	var data bytes.Buffer
	writer := tar.NewWriter(&data)
	_ = writer.WriteHeader(&tar.Header{Name: "safe/link", Typeflag: tar.TypeSymlink, Linkname: "../target"})
	_ = writer.WriteHeader(&tar.Header{Name: "safe/file", Mode: 0o644, Size: 2})
	_, _ = writer.Write([]byte("ok"))
	_ = writer.Close()
	destination := t.TempDir()
	count, skipped, err := ExtractGitArchive(bytes.NewReader(data.Bytes()), destination)
	if err != nil || count != 1 || !reflect.DeepEqual(skipped, []string{"safe/link"}) {
		t.Fatalf("count=%d skipped=%v err=%v", count, skipped, err)
	}
	content, err := os.ReadFile(filepath.Join(destination, "safe", "file"))
	if err != nil || string(content) != "ok" {
		t.Fatalf("content=%q err=%v", content, err)
	}
}

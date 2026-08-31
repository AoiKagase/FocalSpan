package benchmark

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
)

type scriptedGitRunner struct{ calls int }

func (r *scriptedGitRunner) Run(_ context.Context, _ string, _ string, args ...string) (CommandResult, error) {
	r.calls++
	if args[0] == "rev-parse" {
		if r.calls == 1 {
			return CommandResult{Stdout: []byte("1111111111111111111111111111111111111111\n")}, nil
		}
		return CommandResult{Stdout: []byte("2222222222222222222222222222222222222222\n")}, nil
	}
	return CommandResult{Stdout: []byte("diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\ndiff --git a/add.txt b/add.txt\nnew file mode 100644\n--- /dev/null\n+++ b/add.txt\n@@ -0,0 +1 @@\n+x\ndiff --git a/bin.dat b/bin.dat\nBinary files a/bin.dat and b/bin.dat differ\n")}, nil
}

func TestCollectChangesClassifiesDiff(t *testing.T) {
	changes, err := CollectChanges(context.Background(), &scriptedGitRunner{}, "repo", "base", "target")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Files) != 3 || changes.Files[0].Status != "add" || changes.Files[1].Binary != true || changes.Files[2].Status != "rename" {
		t.Fatalf("changes = %+v", changes)
	}
}

type canceledStreamRunner struct{}

func (canceledStreamRunner) Run(context.Context, string, string, ...string) (CommandResult, error) {
	return CommandResult{Stdout: []byte("1111111111111111111111111111111111111111\n")}, nil
}
func (canceledStreamRunner) Stream(ctx context.Context, _ string, _ string, _ io.Writer, _ ...string) ([]byte, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestSnapshotCancellationCleansDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	destination := t.TempDir() + "/snapshot"
	_, err := NewGitSnapshotter(canceledStreamRunner{}).Materialize(ctx, "repo", ".", "HEAD", destination)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("destination remains: %v", statErr)
	}
}

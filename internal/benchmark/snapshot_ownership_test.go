package benchmark

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type canceledArchiveRunner struct{ cancel context.CancelFunc }

func (c canceledArchiveRunner) Run(context.Context, string, string, ...string) (CommandResult, error) {
	return CommandResult{Stdout: []byte(strings.Repeat("a", 40))}, nil
}
func (c canceledArchiveRunner) Stream(_ context.Context, _, _ string, w io.Writer, _ ...string) ([]byte, error) {
	err := tar.NewWriter(w).Close()
	c.cancel()
	return nil, err
}
func TestSnapshotCancellationRemovesOwnedDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	destination := filepath.Join(t.TempDir(), "snapshot")
	_, err := NewGitSnapshotter(canceledArchiveRunner{cancel}).Materialize(ctx, "repo", "unused", "HEAD", destination)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation ignored: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("cancelled partial snapshot remains: %v", err)
	}
}

type brokenArchiveRunner struct{}

func (brokenArchiveRunner) Run(context.Context, string, string, ...string) (CommandResult, error) {
	return CommandResult{Stdout: []byte(strings.Repeat("a", 40))}, nil
}
func (brokenArchiveRunner) Stream(context.Context, string, string, io.Writer, ...string) ([]byte, error) {
	return nil, errors.New("injected archive failure")
}
func TestSnapshotPreservesExistingDestination(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "populated", true: "empty"}[empty], func(t *testing.T) {
			destination := t.TempDir()
			sentinel := filepath.Join(destination, "user-data.txt")
			if !empty {
				if err := os.WriteFile(sentinel, []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err := NewGitSnapshotter(brokenArchiveRunner{}).Materialize(context.Background(), "repo", "unused", "HEAD", destination)
			if err == nil {
				t.Fatal("existing destination accepted")
			}
			if _, err := os.Stat(destination); err != nil {
				t.Fatalf("existing destination deleted: %v", err)
			}
			if !empty {
				if data, err := os.ReadFile(sentinel); err != nil || string(data) != "preserve" {
					t.Fatalf("existing file lost: %s %v", data, err)
				}
			}
		})
	}
}
func TestFailedSnapshotRemovesOnlyOwnedDestination(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "new")
	_, err := NewGitSnapshotter(brokenArchiveRunner{}).Materialize(context.Background(), "repo", "unused", "HEAD", destination)
	if err == nil {
		t.Fatal("archive error missing")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("owned partial snapshot remains: %v", err)
	}
}

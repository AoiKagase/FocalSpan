package indexlock

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigurationProcessHelper(t *testing.T) {
	path := os.Getenv("FOCALSPAN_CONFIG_LOCK_HELPER")
	if path == "" {
		return
	}
	lease, err := AcquireConfiguration(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	fmt.Println("locked")
	var input [1]byte
	_, _ = os.Stdin.Read(input[:])
}
func TestConfigurationLockAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.lock")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConfigurationProcessHelper$")
	cmd.Env = append(os.Environ(), "FOCALSPAN_CONFIG_LOCK_HELPER="+path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatalf("handshake %q %v", line, err)
	}
	started := time.Now()
	lease, err := AcquireConfiguration(context.Background(), path)
	if lease != nil {
		lease.Close()
	}
	if !errors.Is(err, ErrBusy) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unbounded or bypassed process lock: %v", err)
	}
	if elapsed := time.Since(started); elapsed < 4*time.Second || elapsed > 8*time.Second {
		t.Fatalf("unexpected wait %s", elapsed)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if _, err := AcquireConfiguration(canceled, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	lease, err = AcquireConfiguration(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	lease.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persistent lock removed: %v", err)
	}
}

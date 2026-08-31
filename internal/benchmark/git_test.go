package benchmark

import (
	"context"
	"reflect"
	"testing"
)

type recordingRunner struct{ args []string }

func (r *recordingRunner) Run(_ context.Context, dir, name string, args ...string) (CommandResult, error) {
	r.args = append([]string{dir, name}, args...)
	return CommandResult{Stdout: []byte("0123456789012345678901234567890123456789\n")}, nil
}

func TestGitArgumentsRemainSeparate(t *testing.T) {
	runner := &recordingRunner{}
	_, err := ResolveCommit(context.Background(), runner, "repo", "name; echo pwned")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"repo", "git", "rev-parse", "--verify", "name; echo pwned^{commit}"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

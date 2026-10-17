package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExplicitQueryNormalization(t *testing.T) {
	for _, tc := range []struct{ input, want []string }{{[]string{"--query", "needle", "--root", "repo"}, []string{"--query", "needle", "--root", "repo"}}, {[]string{"-query", "needle"}, []string{"-query", "needle"}}, {[]string{"--query=needle"}, []string{"--query=needle"}}, {[]string{"hello", "world", "--path", "src"}, []string{"--path", "src", "--query", "hello world"}}, {[]string{"--query", "--hidden"}, []string{"--query", "--hidden"}}, {[]string{"--", "--hidden", "value"}, []string{"--query", "--hidden value"}}} {
		if got := normalizeQueryArgs(tc.input); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("input=%q got=%q want=%q", tc.input, got, tc.want)
		}
	}
}
func TestExplicitQueryCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("needle source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := Run(context.Background(), []string{"--query", "needle", "--root", root}, &out, &errout); code != 0 {
		t.Fatalf("code=%d err=%s", code, errout.String())
	}
	out.Reset()
	errout.Reset()
	if code := Run(context.Background(), []string{"--query", "needle", "extra", "--root", root}, &out, &errout); code == 0 {
		t.Fatal("explicit query + positional text accepted")
	}
}

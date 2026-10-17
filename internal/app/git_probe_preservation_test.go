package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGitProbeFailurePreservesIgnoredSourceAndIndex(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	writeAppFile(t, filepath.Join(root, "auth.go"), "package p\nfunc Validate(){}\n")
	writeAppFile(t, filepath.Join(root, ".gitignore"), "private/\n")
	if err := os.Mkdir(filepath.Join(root, "private"), 0700); err != nil {
		t.Fatal(err)
	}
	writeAppFile(t, filepath.Join(root, "private", "customer.txt"), "dummy private data\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err = s.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	keys := []string{"last_revision", "last_successful_index", "configuration_hash", "extractor_version", "extraction_configuration_hash", "linking_pending"}
	before := map[string]string{}
	for _, key := range keys {
		value, err := s.Store.Meta(ctx, key)
		if err != nil {
			t.Fatal(err)
		}
		before[key] = value
	}
	hash, found, err := s.Store.FileHash(ctx, "auth.go")
	if err != nil || !found {
		t.Fatal("source fixture not indexed")
	}
	t.Setenv("PATH", t.TempDir())
	if _, err = s.Index(ctx, false); err == nil {
		t.Fatal("Git failure accepted")
	}
	for _, key := range keys {
		value, err := s.Store.Meta(ctx, key)
		if err != nil || value != before[key] {
			t.Fatalf("metadata changed: %s %q -> %q err=%v", key, before[key], value, err)
		}
	}
	after, found, err := s.Store.FileHash(ctx, "auth.go")
	if err != nil || !found || after != hash {
		t.Fatal("existing source lost")
	}
	private, err := s.Store.CandidatesForPath(ctx, "private/customer.txt")
	if err != nil || len(private) != 0 {
		t.Fatalf("ignored source stored: %+v %v", private, err)
	}
}

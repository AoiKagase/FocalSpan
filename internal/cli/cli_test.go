package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupStatusAndBareQueryJSON(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package sample\n\nfunc Hello() string { return \"hi\" }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"setup", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("setup code=%d stderr=%s", code, errOut.String())
	}
	var setup map[string]any
	if err := json.Unmarshal(out.Bytes(), &setup); err != nil || setup["files_seen"] == nil {
		t.Fatalf("setup=%v err=%v output=%s", setup, err, out.String())
	}

	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"status", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("status code=%d stderr=%s", code, errOut.String())
	}
	var status map[string]any
	if err := json.Unmarshal(out.Bytes(), &status); err != nil || status["ready"] != true || status["config_valid"] != true {
		t.Fatalf("status=%v err=%v output=%s", status, err, out.String())
	}

	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"Hello", "function", "--root", root, "--token-budget", "512", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("query code=%d stderr=%s", code, errOut.String())
	}
	var query map[string]any
	if err := json.Unmarshal(out.Bytes(), &query); err != nil || query["items"] == nil {
		t.Fatalf("query=%v err=%v output=%s", query, err, out.String())
	}
}

func TestUpdateIfRepoIsQuietOutsideGit(t *testing.T) {
	root := t.TempDir()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"update", "--root", root, "--if-repo", "--quiet"}, &out, &errOut); code != 0 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errOut.String())
	}
}

func TestUpdateRebuildQuietSuppressesOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "main.go"), "package sample\n\nfunc Ready() bool { return true }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"update", "--rebuild", "--root", root, "--quiet"}, &out, &errOut); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut.String())
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", out.String(), errOut.String())
	}
}

func TestRetiredCommandsDoNotDispatch(t *testing.T) {
	for _, command := range []string{"init", "index", "query", "q", "search", "explain", "expand", "impact", "eval", "doctor", "install", "uninstall"} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), []string{command}, &out, &errOut); code == 0 || !strings.Contains(errOut.String(), "unknown command") {
			t.Fatalf("command=%s code=%d stdout=%q stderr=%q", command, code, out.String(), errOut.String())
		}
	}
}

func TestBareQueryAcceptsRepeatedPathFilters(t *testing.T) {
	root := t.TempDir()
	writeCLIFile(t, filepath.Join(root, "first", "marker.go"), "package first\n\nfunc SharedMarker() string { return \"first\" }\n")
	writeCLIFile(t, filepath.Join(root, "second", "marker.go"), "package second\n\nfunc SharedMarker() string { return \"second\" }\n")
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"setup", "--root", root, "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("setup code=%d stderr=%s", code, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	args := []string{"--root", root, "--auto-update=false", "--json", "--path", "first", "--path", "second", "SharedMarker"}
	if code := Run(context.Background(), args, &out, &errOut); code != 0 {
		t.Fatalf("query code=%d stderr=%s", code, errOut.String())
	}
	var result struct {
		Items []struct {
			Path string `json:"path"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode query: %v output=%s", err, out.String())
	}
	found := map[string]bool{}
	for _, span := range result.Items {
		found[filepath.ToSlash(span.Path)] = true
	}
	if !found["first/marker.go"] || !found["second/marker.go"] {
		t.Fatalf("repeated --path filters were not preserved: paths=%v output=%s", found, out.String())
	}
}

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

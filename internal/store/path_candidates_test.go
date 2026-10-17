package store

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestCandidatesForPathIsExactAndUncapped(t *testing.T) {
	s := openTestStore(t)
	chunks := []model.Chunk{
		{Handle: "later", FilePath: "auth.go", Language: "go", Kind: "function", StartLine: 8, EndLine: 9, Content: "later", ContentHash: "later"},
		{Handle: "first", FilePath: "auth.go", Language: "go", Kind: "function", StartLine: 1, EndLine: 2, Content: "first", ContentHash: "first"},
	}
	if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: "auth.go", Language: "go", SHA256: "auth"}, model.Extraction{Chunks: chunks}); err != nil {
		t.Fatal(err)
	}
	storeSeedFile(t, s, "auth.go.extra", "go", "extra", "Other", "Other", "other", 1, 1, 1)
	got, err := s.CandidatesForPath(context.Background(), "auth.go")
	if err != nil || len(got) != 2 || got[0].Handle != "first" || got[1].Handle != "later" {
		t.Fatalf("exact path candidates=%+v err=%v", got, err)
	}
	if got, err := s.CandidatesForPath(context.Background(), "missing.go"); err != nil || len(got) != 0 {
		t.Fatalf("missing path=%+v err=%v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.CandidatesForPath(ctx, "auth.go"); err == nil {
		t.Fatal("cancelled path query returned success")
	}
}

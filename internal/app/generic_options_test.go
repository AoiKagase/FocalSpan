package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/config"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericConfiguredWindowOptions(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		lines, size, overlap, count, last int
	}{{"small", 10, 2, 0, 5, 9}, {"no_overlap", 90, 80, 0, 2, 81}, {"overlap", 90, 80, 10, 2, 71}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeAppFile(t, filepath.Join(root, "notes.txt"), strings.Repeat("plain text\n", tc.lines))
			cfg := config.Default()
			cfg.GenericChunkLines = tc.size
			cfg.GenericChunkOverlap = tc.overlap
			s, err := NewWithConfig(root, cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err = s.Index(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			cs, err := s.Store.CandidatesForPath(context.Background(), "notes.txt")
			if err != nil {
				t.Fatal(err)
			}
			if len(cs) != tc.count {
				t.Fatalf("chunks=%d want=%d", len(cs), tc.count)
			}
			last := 0
			for _, c := range cs {
				if c.StartLine > last {
					last = c.StartLine
				}
				if c.EndLine-c.StartLine+1 > tc.size {
					t.Fatalf("oversize chunk %+v", c)
				}
			}
			if last != tc.last {
				t.Fatalf("last start=%d want=%d", last, tc.last)
			}
		})
	}
}

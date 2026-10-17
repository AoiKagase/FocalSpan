package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIncrementalPreservesIncomingRelations(t *testing.T) {
	for _, cycle := range []bool{false, true} {
		t.Run(map[bool]string{false: "chain", true: "cycle"}[cycle], func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			target := filepath.Join(root, "defs.go")
			body := "package p\nfunc Target() int { return 1 }\n"
			if cycle {
				body = "package p\nfunc Target() int { return Caller() }\n"
			}
			writeAppFile(t, target, body)
			writeAppFile(t, filepath.Join(root, "sources.go"), "package p\nfunc Caller() int { return Target() }\n")
			writeAppFile(t, filepath.Join(root, "entry.go"), "package p\nfunc Entry() int { return Caller() }\n")
			s, err := New(root)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if _, err = s.Index(ctx, true); err != nil {
				t.Fatal(err)
			}
			writeAppFile(t, target, body+"// changed\n")
			run, err := s.Index(ctx, false)
			if err != nil {
				t.Fatal(err)
			}
			if run.FilesChanged != 1 {
				t.Fatalf("content change count=%d", run.FilesChanged)
			}
			incremental, err := s.Store.Relations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.Index(ctx, true); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := s.Store.Relations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(incremental, rebuilt) {
				t.Fatalf("incremental=%+v rebuilt=%+v", incremental, rebuilt)
			}
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if _, err = s.Index(ctx, false); err != nil {
				t.Fatal(err)
			}
			deleted, err := s.Store.Relations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			unresolved := false
			for _, r := range deleted {
				if r.Kind == "calls" && r.UnresolvedTo == "Target" {
					unresolved = true
				}
			}
			if !unresolved {
				t.Fatalf("deleted target lost source relation: %+v", deleted)
			}
			writeAppFile(t, target, body)
			if _, err = s.Index(ctx, false); err != nil {
				t.Fatal(err)
			}
			readded, err := s.Store.Relations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			resolved := 0
			for _, r := range readded {
				if r.Kind == "calls" && r.ToHandle != "" {
					resolved++
				}
			}
			if resolved < 2 {
				t.Fatalf("relations after re-add: %+v", readded)
			}
		})
	}
}

package indexer

import (
	"context"
	"database/sql"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/store"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestR11ContentChangeCounts(t *testing.T) {
	t.Run("single-consumer", func(t *testing.T) { checkR11ContentChangeCounts(t, false) })
	t.Run("multiple-paths-and-chain", func(t *testing.T) { checkR11ContentChangeCounts(t, true) })
}

func checkR11ContentChangeCounts(t *testing.T, multiple bool) {
	root := t.TempDir()
	ctx := context.Background()
	write(t, filepath.Join(root, "a.go"), "package p\nfunc Target() {}\n")
	write(t, filepath.Join(root, "b.go"), "package p\nfunc Caller() { Target() }\n")
	unchanged := 2
	if multiple {
		write(t, filepath.Join(root, "d.go"), "package p\nfunc Other() { Target(); Caller() }\n")
		write(t, filepath.Join(root, "e.go"), "package p\nfunc Further() { Other(); Caller(); Target() }\n")
		unchanged = 4
	}
	cfg := config.Default()
	s, e := store.Open(root, cfg.IndexDirectory)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ix := New(root, cfg, s, extract.NewRegistry(goast.NewExtractor()))
	if _, e = ix.Run(ctx, true); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", s.DBPath())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for phase, want := range [][4]int{{1, 0, unchanged, 0}, {0, 1, unchanged, 0}, {0, 0, unchanged, 1}, {1, 0, unchanged, 0}, {1, 0, unchanged + 1, 0}} {
		switch phase {
		case 0:
			write(t, filepath.Join(root, "c.go"), "package q\nfunc Target() {}\n")
		case 1:
			write(t, filepath.Join(root, "a.go"), "package p\nfunc Target() { println(1) }\n")
		case 2:
			if e = os.Remove(filepath.Join(root, "c.go")); e != nil {
				t.Fatal(e)
			}
		case 3:
			write(t, filepath.Join(root, "c.go"), "package q\nfunc Target() {}\n")
		case 4:
			write(t, filepath.Join(root, "f.go"), "package r\nfunc Target() {}\n")
		}
		run, e := ix.Run(ctx, false)
		if e != nil {
			t.Fatal(e)
		}
		got := [4]int{run.FilesAdded, run.FilesChanged, run.FilesUnchanged, run.FilesDeleted}
		if got != want {
			t.Errorf("phase %d stats=%v want %v", phase, got, want)
		}
		if run.FilesSeen != run.FilesAdded+run.FilesChanged+run.FilesUnchanged {
			t.Error("classification counts disagree with seen files")
		}
		var saved [4]int
		if e = db.QueryRow(`SELECT files_added,files_changed,files_unchanged,files_deleted FROM index_runs ORDER BY id DESC LIMIT 1`).Scan(&saved[0], &saved[1], &saved[2], &saved[3]); e != nil {
			t.Fatal(e)
		}
		if saved != want {
			t.Errorf("saved run=%v want %v", saved, want)
		}
		before, e := s.Relations(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = ix.Run(ctx, true); e != nil {
			t.Fatal(e)
		}
		after, e := s.Relations(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(before, after) {
			t.Errorf("phase %d incremental graph differs from full", phase)
		}
	}
}

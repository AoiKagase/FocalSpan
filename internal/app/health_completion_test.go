package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestHealthRejectsIncompleteOrOldMetadataAndRecovers(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() {}\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fresh, err := s.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh.Stale || fresh.Ready || fresh.IndexFresh {
		t.Errorf("missing completion metadata accepted: %+v", fresh)
	}
	for _, tc := range []struct{ key, value string }{{"linking_pending", "1"}, {"extractor_version", "old"}, {"extractor_version", ""}, {"configuration_hash", ""}, {"last_successful_index", ""}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			if _, err = s.Index(ctx, false); err != nil {
				t.Fatal(err)
			}
			if err = s.Store.SetMeta(ctx, tc.key, tc.value); err != nil {
				t.Fatal(err)
			}
			health, err := s.Health(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !health.Stale || health.Ready || health.IndexFresh {
				t.Fatalf("incomplete metadata accepted: %+v", health)
			}
			if _, err = s.QueryEvidence(ctx, EvidenceQueryRequest{Query: "ValidateToken", NoUpdate: true}); err == nil {
				t.Fatal("stale evidence read succeeded")
			}
			if _, err = s.Index(ctx, false); err != nil {
				t.Fatal(err)
			}
			health, err = s.Health(ctx)
			if err != nil || health.Stale || !health.Ready || !health.IndexFresh {
				t.Fatalf("recovery=%+v err=%v", health, err)
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = s.Health(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("metadata cancellation hidden: %v", err)
	}
}

func TestHealthMarksInterruptedLinkingNotReady(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\nfunc ValidateToken() {}\n")
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	interrupted, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err = s.IndexWithProgress(interrupted, true, func(p IndexProgress) {
		if p.Phase == IndexPhaseLinking {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("interruption=%v", err)
	}
	health, err := s.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if health.Ready || health.IndexFresh || !health.Stale {
		t.Fatalf("unfinished index reported ready: %+v", health)
	}
}

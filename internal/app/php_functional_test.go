package app

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
)

func TestPHPFunctionalPublicContextAndIncrementalLinks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	for _, name := range []string{"Entry.php", "Dependencies.php", "bootstrap.inc"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "repos", "phpfixsample", name))
		if err != nil {
			t.Fatal(err)
		}
		writeAppFile(t, filepath.Join(root, name), string(data))
	}
	runAppGit(t, root, "init")
	runAppGit(t, root, "add", ".")
	runAppGit(t, root, "-c", "user.name=focalspan", "-c", "user.email=focalspan@example.invalid", "commit", "-m", "fixture")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err = a.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	symbols, err := a.Store.Symbols(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var run, name model.Symbol
	for _, s := range symbols {
		if s.QualifiedName == `App\Run` {
			run = s
		}
		if s.QualifiedName == `App\Settings::NAME` {
			name = s
		}
	}
	if run.Handle == "" || name.Handle == "" {
		t.Fatalf("symbols missing run=%+v name=%+v", run, name)
	}
	query, err := a.Query(ctx, QueryRequest{Query: "NAME", TokenBudget: 1200, Mode: "source", NoUpdate: true})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range query.Items {
		if item.Symbol == "NAME" && strings.Contains(item.Content, "public const string NAME") {
			found = true
		}
	}
	if !found {
		t.Fatal("typed constant absent from Query")
	}
	self, err := a.Expand(ctx, []string{name.Handle}, "self", 1200)
	if err != nil || len(self.Items) != 1 {
		t.Fatalf("constant self=%+v err=%v", self, err)
	}
	constSource := "#[Marker]\n    public const string NAME = 'x', OTHER = 'y';"
	if strings.TrimSpace(self.Items[0].Content) != constSource {
		t.Errorf("constant source=%q", self.Items[0].Content)
	}
	for _, limit := range []int{512, 1200} {
		packet, err := a.QueryEvidence(ctx, EvidenceQueryRequest{Query: "NAME", TokenBudget: limit, Mode: evidence.ModeSource, NoUpdate: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = evidence.Validate(packet.Packet); err != nil {
			t.Fatal(err)
		}
		used := evidence.MeasureModelVisible(packet.Packet, nil)
		if used > limit || packet.Packet.Budget.Used != used {
			t.Errorf("wire budget %d actual %d reported %d", limit, used, packet.Packet.Budget.Used)
		}
		found = false
		for _, item := range packet.Packet.Evidence {
			if item.Symbol == "NAME" {
				found = true
				if item.Fidelity != evidence.FidelityVerbatim || strings.TrimSpace(item.Source) != constSource {
					t.Errorf("constant evidence=%+v", item)
				}
			}
		}
		if !found {
			t.Fatalf("constant evidence absent at %d", limit)
		}
	}
	checkFallback := func(want string) {
		t.Helper()
		symbols, err := a.Store.Symbols(ctx)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]string{}
		for _, s := range symbols {
			names[s.Handle] = s.QualifiedName
		}
		relations, err := a.Store.Relations(ctx)
		if err != nil {
			t.Fatal(err)
		}
		seen := false
		shared := false
		for _, rel := range relations {
			if rel.FromHandle != run.Handle || rel.Kind != "calls" {
				continue
			}
			if rel.Source == "fallback()" {
				seen = true
				if names[rel.ToHandle] != want {
					t.Errorf("fallback got %q want %q (%+v)", names[rel.ToHandle], want, rel)
				}
			}
			if rel.Source == "Shared()" {
				shared = names[rel.ToHandle] == `Vendor\process`
			}
			if strings.Contains(rel.Source, "unknown") || strings.Contains(rel.Source, "Missing") {
				if rel.ToHandle != "" {
					t.Errorf("unknown call linked: %+v", rel)
				}
			}
		}
		if !seen || !shared {
			t.Errorf("calls missing fallback=%v imported=%v", seen, shared)
		}
	}
	snapshot := func() []string {
		t.Helper()
		symbols, err := a.Store.Symbols(ctx)
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]string{}
		for _, s := range symbols {
			names[s.Handle] = s.FilePath + ":" + s.QualifiedName
		}
		relations, err := a.Store.Relations(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var result []string
		for _, r := range relations {
			if r.FromHandle == run.Handle && r.Kind == "calls" {
				result = append(result, r.Source+"|"+names[r.ToHandle]+"|"+r.UnresolvedTo)
			}
		}
		sort.Strings(result)
		return result
	}
	parity := func() {
		t.Helper()
		before := snapshot()
		if _, err = a.Index(ctx, true); err != nil {
			t.Fatal(err)
		}
		if after := snapshot(); !reflect.DeepEqual(before, after) {
			t.Errorf("incremental/full mismatch\nbefore=%v\nafter=%v", before, after)
		}
	}
	checkFallback("fallback")
	callees, err := a.Expand(ctx, []string{run.Handle}, "callees", 2400)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range callees.Items {
		if item.Symbol == "process" {
			found = true
		}
	}
	if !found {
		t.Fatal("imported grouped function absent from Expand")
	}
	// Target-only additions must supersede global fallback, then invalidate it
	// when another definition makes the preferred lookup level ambiguous.
	writeAppFile(t, filepath.Join(root, "Local.inc"), "<?php namespace App; function fallback(): int { return 2; }\n")
	if _, err = a.Index(ctx, false); err != nil {
		t.Fatal(err)
	}
	checkFallback(`App\fallback`)
	parity()
	writeAppFile(t, filepath.Join(root, "Duplicate.inc"), "<?php namespace App; function fallback(): int { return 3; }\n")
	if _, err = a.Index(ctx, false); err != nil {
		t.Fatal(err)
	}
	checkFallback("")
	parity()
	if err = os.Remove(filepath.Join(root, "Duplicate.inc")); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Index(ctx, false); err != nil {
		t.Fatal(err)
	}
	checkFallback(`App\fallback`)
	parity()
	writeAppFile(t, filepath.Join(root, "bootstrap.inc"), "<?php\nfunction fallback(): int { return 4; }\n")
	impact, err := a.Impact(ctx, "", "", 2000)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, item := range impact.Items {
		if item.Symbol == "fallback" && item.Path == "bootstrap.inc" {
			found = true
		}
	}
	if !found {
		t.Fatalf("changed INC function absent from Impact: %+v", impact.Items)
	}
}

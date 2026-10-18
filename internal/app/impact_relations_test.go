package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/evidence"
)

func r09GitID(t *testing.T, root string) string {
	t.Helper()
	out, e := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if e != nil {
		t.Fatal(e)
	}
	return strings.TrimSpace(string(out))
}
func r09Fixture(t *testing.T, auto bool) (*Service, string, string, string) {
	t.Helper()
	root := t.TempDir()
	runAppGit(t, root, "init")
	runAppGit(t, root, "config", "user.name", "Test")
	runAppGit(t, root, "config", "user.email", "test@example.invalid")
	source, e := os.ReadFile("../../testdata/regressions/residual11/impact-target.go")
	if e != nil {
		t.Fatal(e)
	}
	writeAppFile(t, filepath.Join(root, "target.go"), string(source))
	writeAppFile(t, filepath.Join(root, "caller.go"), "package p\nfunc Caller() int { return Target() }\n")
	writeAppFile(t, filepath.Join(root, "target_test.go"), "package p\nimport \"testing\"\nfunc TestTarget(t *testing.T) { Target() }\n")
	writeAppFile(t, filepath.Join(root, "indirect.go"), "package p\nfunc Indirect() { Caller() }\n")
	writeAppFile(t, filepath.Join(root, "unrelated.go"), "package p\nfunc Unrelated() { p() }\n")
	runAppGit(t, root, "add", ".")
	runAppGit(t, root, "commit", "-m", "base")
	base := r09GitID(t, root)
	writeAppFile(t, filepath.Join(root, "target.go"), strings.ReplaceAll(string(source), "return 1", "return 2"))
	runAppGit(t, root, "add", "target.go")
	runAppGit(t, root, "commit", "-m", "target change")
	head := r09GitID(t, root)
	cfg := config.Default()
	cfg.AutoUpdateBeforeQuery = auto
	s, e := NewWithConfig(root, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	if _, e = s.Index(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	return s, root, base, head
}

func checkR09Packet(t *testing.T, root string, p evidence.Packet) {
	t.Helper()
	if e := evidence.Validate(p); e != nil {
		t.Fatal(e)
	}
	measured := evidence.MeasureModelVisible(p, budget.NewEstimator())
	if measured != p.Budget.Used || measured > p.Budget.Limit {
		t.Errorf("wire usage=%d budget=%+v", measured, p.Budget)
	}
	for _, item := range p.Evidence {
		data, e := os.ReadFile(filepath.Join(root, item.Location.Path))
		if e != nil {
			t.Fatal(e)
		}
		lines := strings.SplitAfter(string(data), "\n")
		span := strings.Join(lines[item.Location.Lines[0]-1:item.Location.Lines[1]], "")
		if item.Source != "" && !strings.Contains(span, item.Source) {
			t.Errorf("source mismatch: %+v", item)
		}
		for _, seg := range item.Segments {
			if seg.Kind == evidence.SegmentSource {
				span := strings.Join(lines[seg.Lines[0]-1:seg.Lines[1]], "")
				if !strings.Contains(span, seg.Text) {
					t.Errorf("segment mismatch: %+v", seg)
				}
			}
		}
	}
}

func TestR09ImpactRelations(t *testing.T) {
	s, root, base, head := r09Fixture(t, true)
	ctx := context.Background()
	targets, e := s.Store.SearchExactSymbols(ctx, []string{"Target"}, 10)
	if e != nil || len(targets) == 0 {
		t.Fatalf("target: %v", e)
	}
	for _, rel := range []string{"callers", "tests"} {
		hits, e := s.Store.RelatedCandidateHits(ctx, []string{targets[0].Handle}, rel)
		if e != nil || len(hits) == 0 {
			t.Fatalf("graph control %s: %v", rel, e)
		}
	}
	legacy, e := s.Impact(ctx, base, head, 4000)
	if e != nil {
		t.Fatal(e)
	}
	paths := map[string]bool{}
	for _, v := range legacy.Items {
		paths[v.Path] = true
	}
	for _, p := range []string{"target.go", "caller.go", "target_test.go"} {
		if !paths[p] {
			t.Errorf("legacy impact omitted %s: %+v", p, legacy.Items)
		}
	}
	var known []string
	for _, limit := range []int{256, 512, 1200, 4000} {
		result, e := s.ImpactEvidence(ctx, EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: limit, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		checkR09Packet(t, root, result.Packet)
		paths := map[string]bool{}
		for _, v := range result.Packet.Evidence {
			paths[v.Location.Path] = true
			if v.Location.Path == "indirect.go" || v.Location.Path == "unrelated.go" {
				t.Error("impact exceeded one hop")
			}
			if limit == 4000 {
				known = append(known, v.Handle)
				want := evidence.RoleDependent
				if v.Location.Path == "target.go" {
					want = evidence.RoleChange
				}
				if v.Role != want {
					t.Errorf("role %s=%s want %s", v.Location.Path, v.Role, want)
				}
			}
		}
		if limit == 4000 {
			for _, p := range []string{"target.go", "caller.go", "target_test.go"} {
				if !paths[p] {
					t.Errorf("evidence impact omitted %s", p)
				}
			}
		}
	}
	delta, e := s.ImpactEvidence(ctx, EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: 1200, KnownHandles: known})
	if e != nil {
		t.Fatal(e)
	}
	checkR09Packet(t, root, delta.Packet)
	for _, v := range delta.Packet.Evidence {
		for _, h := range known {
			if h == v.Handle {
				t.Error("known handle repeated")
			}
		}
	}
}

func TestR09ImpactRelatedFreshness(t *testing.T) {
	for _, auto := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual", true: "automatic"}[auto], func(t *testing.T) {
			s, root, base, head := r09Fixture(t, auto)
			old, _, e := s.Store.FileHash(context.Background(), "caller.go")
			if e != nil {
				t.Fatal(e)
			}
			writeAppFile(t, filepath.Join(root, "caller.go"), "package p\nfunc Caller() int { return Target() + 3 }\n")
			result, e := s.ImpactEvidence(context.Background(), EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: 4000, Mode: evidence.ModeSource})
			if !auto {
				if e == nil {
					t.Error("stale dependent source accepted")
				}
				after, _, err := s.Store.FileHash(context.Background(), "caller.go")
				if err != nil || after != old {
					t.Error("manual mode updated the index")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, v := range result.Packet.Evidence {
				if v.Location.Path == "caller.go" && strings.Contains(v.Source, "+ 3") {
					found = true
				}
			}
			if !found {
				t.Error("current dependent source missing")
			}
		})
	}
}

func TestR09ImpactModuleDependency(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	runAppGit(t, root, "config", "user.name", "Test")
	runAppGit(t, root, "config", "user.email", "test@example.invalid")
	writeAppFile(t, filepath.Join(root, "dep.ts"), "export function work() { return 1; }\n")
	writeAppFile(t, filepath.Join(root, "main.ts"), "import './dep.ts';\n")
	runAppGit(t, root, "add", ".")
	runAppGit(t, root, "commit", "-m", "base")
	writeAppFile(t, filepath.Join(root, "dep.ts"), "export function work() { return 2; }\n")
	s, e := New(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := s.ImpactEvidence(context.Background(), EvidenceImpactRequest{TokenBudget: 4000, Mode: evidence.ModeSource})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, v := range r.Packet.Evidence {
		if v.Location.Path == "main.ts" {
			found = true
		}
	}
	if !found {
		t.Error("module importer missing from function change impact")
	}
}

func TestR09DeletedRelatedSource(t *testing.T) {
	s, root, base, head := r09Fixture(t, false)
	if e := os.Remove(filepath.Join(root, "caller.go")); e != nil {
		t.Fatal(e)
	}
	result, e := s.ImpactEvidence(context.Background(), EvidenceImpactRequest{BaseRef: base, HeadRef: head, TokenBudget: 4000, Mode: evidence.ModeSource})
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range result.Packet.Evidence {
		if item.Location.Path == "caller.go" {
			t.Error("deleted dependent returned old source")
		}
	}
	checkR09Packet(t, root, result.Packet)
}

func TestR09MultipleAnchorsAndCycle(t *testing.T) {
	root := t.TempDir()
	runAppGit(t, root, "init")
	runAppGit(t, root, "config", "user.name", "Test")
	runAppGit(t, root, "config", "user.email", "test@example.invalid")
	writeAppFile(t, filepath.Join(root, "target.go"), "package p\nfunc Target() int { return Caller() + 1 }\nfunc Second() int { return 1 }\n")
	writeAppFile(t, filepath.Join(root, "caller.go"), "package p\nfunc Caller() int { return Target() + Second() }\n")
	runAppGit(t, root, "add", ".")
	runAppGit(t, root, "commit", "-m", "base")
	writeAppFile(t, filepath.Join(root, "target.go"), "package p\nfunc Target() int { return Caller() + 2 }\nfunc Second() int { return 2 }\n")
	s, e := New(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if _, e = s.Index(context.Background(), true); e != nil {
		t.Fatal(e)
	}
	candidates, e := s.impactCandidates(context.Background(), "", "")
	if e != nil {
		t.Fatal(e)
	}
	callers, changed := 0, 0
	for _, v := range candidates.Candidates {
		if v.Path == "caller.go" {
			callers++
			if v.Changed || v.RelationContext == nil {
				t.Errorf("dependent provenance lost: %+v", v)
			}
		}
		if v.Changed {
			changed++
		}
	}
	if callers != 1 || changed != 2 {
		t.Errorf("callers=%d changed=%d candidates=%+v", callers, changed, candidates.Candidates)
	}
}

package app

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCPP20_Refresh(t *testing.T) {
	ctx := context.Background()
	src := "void target(){} void caller(void(*target)()){target();}\n"
	s, _ := cppAppAuditService(t, map[string]string{"a.cpp": src})
	hash, _, e := s.Store.FileHash(ctx, "a.cpp")
	if e != nil {
		t.Fatal(e)
	}
	file := model.SourceFile{Path: "a.cpp", Language: "cpp", Content: []byte(src), SHA256: hash, SizeBytes: int64(len(src))}
	x, e := cpp.NewExtractor().Extract(ctx, file)
	if e != nil {
		t.Fatal(e)
	}
	var from, to string
	for _, v := range x.Symbols {
		if v.Name == "caller" {
			from = v.Handle
		}
		if v.Name == "target" {
			to = v.Handle
		}
	}
	if from == "" || to == "" {
		t.Fatal("symbols missing")
	}
	x.Relations = append(x.Relations, model.Relation{FromHandle: from, ToHandle: to, Kind: "calls", Source: "cpp-call", Confidence: .95})
	if e = s.Store.ReplaceFile(ctx, file, x); e != nil {
		t.Fatal(e)
	}
	if e = s.Store.SetMeta(ctx, "extractor_version", "extractors-v19-residual11-functional"); e != nil {
		t.Fatal(e)
	}
	status, e := s.Status(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if !status.Stale {
		t.Error("old C++ extractor accepted as fresh")
	}
	if _, e = s.Index(ctx, false); e != nil {
		t.Fatal(e)
	}
	_, rs := cppAppAuditGraph(t, s)
	for _, r := range rs {
		if r.FromHandle == from && r.ToHandle == to && r.Kind == "calls" {
			t.Error("old false edge survived refresh")
		}
	}
	run, e := s.Index(ctx, false)
	if e != nil {
		t.Fatal(e)
	}
	if run.FilesChanged != 0 || run.FilesUnchanged != 1 {
		t.Fatal("refresh repeated", run)
	}
}
func TestCPP20_ModuleIncrementalParity(t *testing.T) {
	ctx := context.Background()
	s, root := cppAppAuditService(t, map[string]string{"odd.cppm": "export module logical;\nexport void helper(){}\n", "main.cpp": "import logical;\nvoid caller(){}\n"})
	check := func(want bool) {
		t.Helper()
		syms, rs := cppAppAuditGraph(t, s)
		ok := false
		for _, r := range rs {
			if r.Kind == "imports" && syms[r.FromHandle].FilePath == "main.cpp" && syms[r.ToHandle].FilePath == "odd.cppm" {
				ok = true
			}
		}
		if ok != want {
			t.Fatalf("module link=%v want %v", ok, want)
		}
	}
	check(true)
	writeAppFile(t, filepath.Join(root, "dup.cppm"), "export module logical;\n")
	if _, e := s.Index(ctx, false); e != nil {
		t.Fatal(e)
	}
	check(false)
	_, incremental := cppAppAuditGraph(t, s)
	if _, e := s.Index(ctx, true); e != nil {
		t.Fatal(e)
	}
	_, full := cppAppAuditGraph(t, s)
	if !reflect.DeepEqual(incremental, full) {
		t.Fatal("module duplicate incremental/full mismatch")
	}
	if e := os.Remove(filepath.Join(root, "dup.cppm")); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Index(ctx, false); e != nil {
		t.Fatal(e)
	}
	check(true)
	writeAppFile(t, filepath.Join(root, "odd.cppm"), "export module renamed;\nexport void helper(){}\n")
	if _, e := s.Index(ctx, false); e != nil {
		t.Fatal(e)
	}
	check(false)
}
func TestCPP20_EvidenceContract(t *testing.T) {
	ctx := context.Background()
	source := "\ufeffnamespace n{struct Item{explicit Item(int x):v{x}{consume();} int v; int operator[](int i){return i;}}; Item make(){return Item{1};}}\r\n"
	s, root := cppAppAuditService(t, map[string]string{"main.cpp": source, "odd.cppm": "export module logical;\nexport void helper(){}\n", "consumer.cpp": "import logical;\nvoid run(){}\n"})
	check := func(p evidence.Packet) {
		t.Helper()
		if e := evidence.Validate(p); e != nil {
			t.Fatal(e)
		}
		if n := evidence.MeasureModelVisible(p, budget.NewEstimator()); n != p.Budget.Used || n > p.Budget.Limit {
			t.Fatalf("wire budget measured=%d %+v", n, p.Budget)
		}
		wire, e := json.Marshal(p)
		if e != nil || len(wire) == 0 {
			t.Fatal(e)
		}
		for _, v := range p.Evidence {
			if v.Source == "" {
				continue
			}
			data, e := os.ReadFile(filepath.Join(root, v.Location.Path))
			if e != nil {
				t.Fatal(e)
			}
			lines := strings.Split(string(data), "\n")
			if v.Location.Lines[0] < 1 || v.Location.Lines[1] > len(lines) {
				t.Fatal(v.Location)
			}
			if !strings.Contains(strings.Join(lines[v.Location.Lines[0]-1:v.Location.Lines[1]], "\n"), v.Source) {
				t.Fatalf("source/line mismatch %+v", v)
			}
		}
		if _, e = evidence.EncodeContextV2(p, budget.NewEstimator()); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range []string{"make", "Item", "operator[]"} {
		for _, limit := range []int{500, 1600, 4000} {
			for _, mode := range []evidence.Mode{evidence.ModeFocused, evidence.ModeSource, evidence.ModeOutline} {
				r, e := s.QueryEvidence(ctx, EvidenceQueryRequest{Query: name, Paths: []string{"main.cpp"}, TokenBudget: limit, Mode: mode, NoUpdate: true})
				if e != nil {
					t.Fatal(e)
				}
				check(r.Packet)
				if limit == 4000 && mode == evidence.ModeSource && len(r.Packet.Evidence) == 0 {
					t.Fatal("source result omitted", name)
				}
				known := []string{}
				for _, v := range r.Packet.Evidence {
					known = append(known, v.Handle)
				}
				r, e = s.QueryEvidence(ctx, EvidenceQueryRequest{Query: name, TokenBudget: limit, Mode: mode, NoUpdate: true, KnownHandles: known})
				if e != nil {
					t.Fatal(e)
				}
				check(r.Packet)
			}
		}
	}
	syms, e := s.Store.Symbols(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, sy := range syms {
		if sy.FilePath == "consumer.cpp" && sy.Kind == "translation_unit" {
			r, e := s.ExpandEvidence(ctx, EvidenceExpandRequest{Handles: []string{sy.Handle}, Relation: "imports", TokenBudget: 4000, Mode: evidence.ModeSource})
			if e != nil {
				t.Fatal(e)
			}
			check(r.Packet)
		}
	}
	legacy, e := s.Query(ctx, QueryRequest{Query: "make", TokenBudget: 4000, Mode: "source", NoUpdate: true})
	if e != nil || len(legacy.Items) == 0 {
		t.Fatal("legacy query", e)
	}
}

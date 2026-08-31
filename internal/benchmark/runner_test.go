package benchmark

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/search"
)

type fakeSnapshotter struct {
	calls int
	root  string
}

func TestValidateLabelsAtBaseIncludesExpansionLabels(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/anchor.go", []byte("package fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ValidateLabelsAtBase(root, Case{Expand: []ExpandExpectation{{
		From:            SymbolExpectation{Path: "anchor.go", Name: "Anchor"},
		RequiredPaths:   []string{"missing-related.go"},
		RequiredSymbols: []SymbolExpectation{{Path: "missing-symbol.go", Name: "Related"}},
	}}})
	if err == nil || !strings.Contains(err.Error(), "missing-related.go") {
		t.Fatalf("error=%v", err)
	}
}

func (f *fakeSnapshotter) Materialize(context.Context, string, string, string, string) (Snapshot, error) {
	f.calls++
	return Snapshot{Commit: "base", Root: f.root}, nil
}

type fakeEngineFactory struct {
	opens, builds, queries, closes int
	retrievalModes                 []search.RetrievalMode
	indexed                        []AttributionIdentity
	attributed                     app.AttributedEvidenceResult
	indexedCalls                   int
}

func (f *fakeEngineFactory) Open(string) (Engine, error) {
	f.opens++
	return &fakeEngine{factory: f}, nil
}

type fakeEngine struct{ factory *fakeEngineFactory }

func (f *fakeEngine) Build(context.Context) (IndexMeasurement, error) {
	f.factory.builds++
	return IndexMeasurement{}, nil
}
func (f *fakeEngine) QueryLegacy(_ context.Context, req app.QueryRequest) (model.ContextBundle, error) {
	f.factory.queries++
	f.factory.retrievalModes = append(f.factory.retrievalModes, req.RetrievalMode)
	return model.ContextBundle{}, nil
}
func (f *fakeEngine) QueryEvidence(_ context.Context, req app.EvidenceQueryRequest) (evidence.Packet, error) {
	f.factory.queries++
	f.factory.retrievalModes = append(f.factory.retrievalModes, req.RetrievalMode)
	return evidence.Packet{Schema: evidence.SchemaContextV1, Intent: "definition", Mode: req.Mode, Budget: evidence.Budget{Limit: req.TokenBudget}, Evidence: []evidence.Item{}}, nil
}

func (f *fakeEngine) QueryEvidenceAttributed(ctx context.Context, req app.EvidenceQueryRequest) (app.AttributedEvidenceResult, error) {
	if f.factory.attributed.Compile.Packet.Schema != "" {
		f.factory.queries++
		f.factory.retrievalModes = append(f.factory.retrievalModes, req.RetrievalMode)
		return f.factory.attributed, nil
	}
	packet, err := f.QueryEvidence(ctx, req)
	return app.AttributedEvidenceResult{Compile: evidence.CompileResult{Packet: packet}}, err
}
func (f *fakeEngine) AttributionIdentities(context.Context, []AttributionExpectation) ([]AttributionIdentity, error) {
	f.factory.indexedCalls++
	return append([]AttributionIdentity(nil), f.factory.indexed...), nil
}
func (f *fakeEngine) ExpandEvidence(context.Context, app.EvidenceExpandRequest) (evidence.Packet, error) {
	return evidence.Packet{}, nil
}
func (f *fakeEngine) Close() error { f.factory.closes++; return nil }

func TestRunnerSequencesCasesProfilesBudgetsAndRepeats(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(root+"/required.go", []byte("package fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshotter := &fakeSnapshotter{root: root}
	factory := &fakeEngineFactory{}
	runner := Runner{Snapshotter: snapshotter, EngineFactory: factory}
	profiles := []Profile{{Name: "one", RetrievalMode: search.RetrievalFull, Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{512, 1024}}, {Name: "two", RetrievalMode: search.RetrievalFTSOnly, Contract: "legacy", EvidenceMode: evidence.ModeSource, Budgets: []int{512, 1024}}}
	report, err := runner.Run(context.Background(), RunRequest{
		Suite: Suite{Name: "suite", Cases: []Case{{
			ID: "case", Repository: "self", BaseRef: "base", TargetRef: "target",
			Query: "query", Budgets: []int{512, 1024}, RequiredPaths: []string{"required.go"},
		}}},
		Repositories: map[string]string{"self": root}, Profiles: profiles,
		Repeat: 2, Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotter.calls != 1 || factory.opens != 1 || factory.builds != 1 || factory.queries != 8 || factory.closes != 1 || len(report.Runs) != 4 {
		t.Fatalf("snapshot=%d opens=%d builds=%d queries=%d closes=%d runs=%d", snapshotter.calls, factory.opens, factory.builds, factory.queries, factory.closes, len(report.Runs))
	}
	wantModes := []search.RetrievalMode{
		search.RetrievalFull, search.RetrievalFull, search.RetrievalFull, search.RetrievalFull,
		search.RetrievalFTSOnly, search.RetrievalFTSOnly, search.RetrievalFTSOnly, search.RetrievalFTSOnly,
	}
	if len(factory.retrievalModes) != len(wantModes) {
		t.Fatalf("retrieval modes = %v", factory.retrievalModes)
	}
	for index := range wantModes {
		if factory.retrievalModes[index] != wantModes[index] {
			t.Fatalf("retrieval mode[%d] = %q, want %q", index, factory.retrievalModes[index], wantModes[index])
		}
	}
	if report.Runs[0].Budget != 512 || report.Runs[1].Budget != 1024 || report.Runs[2].Profile != "two" {
		t.Fatalf("order = %+v", report.Runs)
	}
	if len(report.Performance) != 4 || len(report.Performance[0].QueryMS) != 2 {
		t.Fatalf("performance = %+v", report.Performance)
	}
}

func TestRunnerAttributesEveryRequiredLabelAndExpansionAnchor(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"required.go", "anchor.go"} {
		if err := os.WriteFile(root+"/"+path, []byte("package fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	factory := &fakeEngineFactory{
		indexed: []AttributionIdentity{{Path: "required.go", Symbol: "Target", Kind: "function"}, {Path: "anchor.go", Symbol: "Anchor", Kind: "function"}},
		attributed: app.AttributedEvidenceResult{
			Compile: evidence.CompileResult{Packet: evidence.Packet{Schema: evidence.SchemaContextV1, Intent: "definition", Mode: evidence.ModeFocused, Budget: evidence.Budget{Limit: 1024}, Evidence: []evidence.Item{{Location: evidence.Location{Path: "required.go"}, Symbol: "Target", Kind: "function"}}}},
			Trace: search.SearchTrace{
				Retrieved:  []search.StageCandidateTrace{{Retriever: search.RetrieverSymbol, Position: 1, Path: "required.go", Symbol: "Target", Kind: "function"}},
				Candidates: []search.CandidateTrace{{Path: "required.go", Symbol: "Target", Kind: "function", RankedPosition: 1}},
			},
		},
	}
	runner := Runner{Snapshotter: &fakeSnapshotter{root: root}, EngineFactory: factory}
	report, err := runner.Run(context.Background(), RunRequest{
		Suite: Suite{Name: "suite", Cases: []Case{{
			ID: "case", Repository: "self", BaseRef: "base", TargetRef: "target", Query: "Target", Budgets: []int{1024},
			RequiredPaths: []string{"required.go"}, RequiredSymbols: []SymbolExpectation{{Path: "required.go", Name: "Target", Kind: "function"}},
			Expand: []ExpandExpectation{{Relation: "callers", From: SymbolExpectation{Path: "anchor.go", Name: "Anchor", Kind: "function"}, Budget: 512}},
		}}},
		Repositories: map[string]string{"self": root}, Profiles: []Profile{
			{Name: "evidence", Contract: "evidence", EvidenceMode: evidence.ModeFocused, Budgets: []int{1024}},
			{Name: "legacy", Contract: "legacy", EvidenceMode: evidence.ModeFocused, Budgets: []int{1024}},
		},
		Repeat: 1, Workspace: t.TempDir(), Attribution: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if factory.indexedCalls != 1 || len(report.Attributions) != 1 || len(report.Attributions[0].Labels) != 3 {
		t.Fatalf("indexed calls=%d attributions=%+v", factory.indexedCalls, report.Attributions)
	}
	want := []string{StagePacked, StagePacked, StageRetrievalMissing}
	for index, stage := range want {
		if report.Attributions[0].Labels[index].TerminalStage != stage {
			t.Fatalf("label %d=%+v, want %s", index, report.Attributions[0].Labels[index], stage)
		}
	}
}

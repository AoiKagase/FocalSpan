package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestGenericSourceEvidencePreservesCRLFAndRefreshesOldIndexes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	const source = "function ValidateToken {\r\n    return $true\r\n}"
	const prefix = "\xef\xbb\xbf# note\r\n"
	raw := prefix + source + "\r\n"
	writeAppFile(t, filepath.Join(root, "auth.ps1"), raw)
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(ctx, true); err != nil {
		t.Fatal(err)
	}
	// Existing v5 indexes must refresh even though file content is unchanged.
	if err := service.Store.SetMeta(ctx, "extractor_version", "extractors-v5-polyglot"); err != nil {
		t.Fatal(err)
	}
	run, err := service.Index(ctx, false)
	if err != nil || run.FilesChanged != 1 || run.FilesUnchanged != 0 {
		t.Fatalf("old normalized index not refreshed: run=%+v err=%v", run, err)
	}
	next, err := service.Index(ctx, false)
	if err != nil || next.FilesChanged != 0 || next.FilesUnchanged != 1 {
		t.Fatalf("refresh must happen only once: run=%+v err=%v", next, err)
	}
	candidates, err := service.Store.CandidatesForPath(ctx, "auth.ps1")
	if err != nil || len(candidates) != 1 {
		t.Fatalf("candidates=%+v err=%v", candidates, err)
	}
	candidate := candidates[0]
	if candidate.StartByte != len(prefix) || candidate.EndByte != len(prefix)+len(source) || candidate.Content != source || raw[candidate.StartByte:candidate.EndByte] != source {
		t.Fatalf("stored generic source normalized or shifted: %+v", candidate)
	}
	got, err := service.QueryEvidence(ctx, EvidenceQueryRequest{Query: "ValidateToken", NoUpdate: true, TokenBudget: 1200, Mode: evidence.ModeSource})
	if err != nil {
		t.Fatal(err)
	}
	packet := got.Packet
	if err := evidence.Validate(packet); err != nil {
		t.Fatal(err)
	}
	if used := evidence.MeasureModelVisible(packet, nil); used != packet.Budget.Used || used > 1200 {
		t.Fatalf("wire usage=%d budget=%+v", used, packet.Budget)
	}
	if len(packet.Evidence) != 1 || packet.Evidence[0].Source != source || packet.Evidence[0].Location.Lines != [2]int{2, 4} || packet.Evidence[0].Fidelity != evidence.FidelityVerbatim {
		t.Fatalf("generic Evidence is not original source: %+v", packet)
	}
}

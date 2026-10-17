package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestBOMOnlyChangesReindexWithExactSourcePositions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "auth.go")
	const function = "func ValidateToken() bool { return true }"
	const plain = "package auth\r\n\r\n" + function + "\r\n"
	writeAppFile(t, path, plain)
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	previous, err := service.Index(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"\xef\xbb\xbf", ""} {
		raw := prefix + plain
		writeAppFile(t, path, raw)
		run, err := service.Index(ctx, false)
		if err != nil || run.FilesChanged != 1 || run.Revision == previous.Revision {
			t.Fatalf("BOM transition not indexed: run=%+v previous=%+v err=%v", run, previous, err)
		}
		previous = run
		digest := sha256.Sum256([]byte(raw))
		if hash, found, err := service.Store.FileHash(ctx, "auth.go"); err != nil || !found || hash != hex.EncodeToString(digest[:]) {
			t.Fatalf("stored hash differs from original bytes: %q found=%v err=%v", hash, found, err)
		}
		candidates, err := service.Store.CandidatesForPath(ctx, "auth.go")
		if err != nil || len(candidates) != 1 {
			t.Fatalf("candidates=%+v err=%v", candidates, err)
		}
		candidate := candidates[0]
		if candidate.StartByte != len(prefix)+len("package auth\r\n\r\n") || candidate.EndByte > len(raw) || candidate.Content != function || raw[candidate.StartByte:candidate.EndByte] != function {
			t.Fatalf("indexed source position mismatch: %+v", candidate)
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
		if len(packet.Evidence) != 1 || packet.Evidence[0].Source != function || packet.Evidence[0].Location.Lines != [2]int{3, 3} || packet.Evidence[0].Fidelity != evidence.FidelityVerbatim {
			t.Fatalf("BOM source Evidence changed: %+v", packet)
		}
	}
}

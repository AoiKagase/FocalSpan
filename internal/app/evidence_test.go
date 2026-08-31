package app

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/evidence"
)

func TestQueryEvidenceReturnsFocusedPacketAndPreservesLegacyQuery(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\n// ValidateToken rejects expired tokens.\nfunc ValidateToken(token string) error { return nil }\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	legacy, err := service.Query(context.Background(), QueryRequest{Query: "ValidateToken", TokenBudget: 512, Mode: "outline"})
	if err != nil || len(legacy.Items) == 0 {
		t.Fatalf("legacy=%+v err=%v", legacy, err)
	}
	result, err := service.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: "ValidateToken", TokenBudget: 1200, NoUpdate: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Packet.Schema != evidence.SchemaContextV1 || result.Packet.Mode != evidence.ModeFocused || len(result.Packet.Evidence) == 0 {
		t.Fatalf("evidence=%+v", result.Packet)
	}
	if result.Packet.Evidence[0].Location.Path != legacy.Items[0].Path {
		t.Fatalf("retrieval diverged: evidence=%s legacy=%s", result.Packet.Evidence[0].Location.Path, legacy.Items[0].Path)
	}
	attributed, err := service.QueryEvidenceAttributed(context.Background(), EvidenceQueryRequest{Query: "ValidateToken", TokenBudget: 1200, NoUpdate: true})
	if err != nil {
		t.Fatal(err)
	}
	normalJSON, err := json.Marshal(result.Packet)
	if err != nil {
		t.Fatal(err)
	}
	attributedJSON, err := json.Marshal(attributed.Compile.Packet)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(normalJSON, attributedJSON) || attributed.Trace.Candidates[0].RankedPosition != 1 || len(attributed.Trace.Retrieved) == 0 {
		t.Fatalf("attributed result diverged: packet=%s trace=%+v", attributedJSON, attributed.Trace)
	}
	for _, forbidden := range []string{"trace", "retrieved", "ranked_position", "candidate", "token_savings", "debug"} {
		if strings.Contains(string(normalJSON), forbidden) {
			t.Fatalf("normal packet exposed %q: %s", forbidden, normalJSON)
		}
	}
}

func TestExpandEvidenceSuppressesKnownHandlesButUsesAnchor(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	writeAppFile(t, filepath.Join(root, "http.go"), "package auth\n\nfunc Authenticate() error { return ValidateToken() }\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}

	queryResult, err := service.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: "ValidateToken", TokenBudget: 512, Mode: evidence.ModeFocused, NoUpdate: true})
	if err != nil || len(queryResult.Packet.Evidence) == 0 {
		t.Fatalf("query=%+v err=%v", queryResult.Packet, err)
	}
	anchor := queryResult.Packet.Evidence[0].Handle
	known := []string{anchor}
	expanded, err := service.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{anchor}, Relation: "callers", TokenBudget: 1200, Mode: evidence.ModeFocused, KnownHandles: known})
	if err != nil {
		t.Fatal(err)
	}
	knownSet := map[string]bool{}
	for _, handle := range known {
		knownSet[handle] = true
	}
	for _, item := range expanded.Packet.Evidence {
		if knownSet[item.Handle] {
			t.Fatalf("known handle %q retransmitted", item.Handle)
		}
	}
	if expanded.Packet.SkippedKnown == 0 || !containsEvidenceLimitation(expanded.Packet.Limitations, "known_anchor_not_repeated") {
		t.Fatalf("delta metadata absent: %+v", expanded.Packet)
	}
}

func TestImpactEvidenceAddsSyntaxOnlyLimitation(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error { return nil }\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	result, err := service.ImpactEvidence(context.Background(), EvidenceImpactRequest{TokenBudget: 512})
	if err != nil {
		t.Fatal(err)
	}
	if !containsEvidenceLimitation(result.Packet.Limitations, "syntax_only_impact") {
		t.Fatalf("syntax-only limitation absent: %+v", result.Packet)
	}
}

func TestExpandEvidenceSelfKeepsAnchorAsTargetInSourceMode(t *testing.T) {
	root := t.TempDir()
	writeAppFile(t, filepath.Join(root, "auth.go"), "package auth\n\nfunc ValidateToken() error {\n\treturn nil\n}\n")
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.Index(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	queryResult, err := service.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: "ValidateToken", TokenBudget: 512, NoUpdate: true})
	if err != nil || len(queryResult.Packet.Evidence) == 0 {
		t.Fatalf("query=%+v err=%v", queryResult.Packet, err)
	}
	result, err := service.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{queryResult.Packet.Evidence[0].Handle}, Relation: "self", TokenBudget: 1200, Mode: evidence.ModeSource})
	if err != nil || len(result.Packet.Evidence) != 1 {
		t.Fatalf("expand=%+v err=%v", result.Packet, err)
	}
	if result.Packet.Evidence[0].Role != evidence.RoleTarget || result.Packet.Evidence[0].Fidelity != evidence.FidelityVerbatim {
		t.Fatalf("self anchor=%+v", result.Packet.Evidence[0])
	}
}

func TestQueryEvidencePropagatesCancellation(t *testing.T) {
	root := t.TempDir()
	service, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.QueryEvidence(ctx, EvidenceQueryRequest{Query: "ValidateToken"}); err == nil {
		t.Fatal("canceled QueryEvidence succeeded")
	}
}

func containsEvidenceLimitation(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

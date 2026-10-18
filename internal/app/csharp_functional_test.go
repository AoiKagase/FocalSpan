package app

import (
	"context"
	"github.com/focalspan/focalspan/internal/evidence"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSharpFunctionalFixesPublicContext(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "repos", "csharpfixsample", "Service.cs"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	writeAppFile(t, filepath.Join(root, "Service.cs"), source)
	runAppGit(t, root, "init")
	runAppGit(t, root, "add", "Service.cs")
	runAppGit(t, root, "-c", "user.name=focalspan", "-c", "user.email=focalspan@example.invalid", "commit", "-m", "fixture")
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	property, err := a.Query(ctx, QueryRequest{Query: "DisplayName", TokenBudget: 1200, Mode: "source"})
	if err != nil {
		t.Fatal(err)
	}
	var handle string
	for _, item := range property.Items {
		if item.Symbol == "DisplayName" {
			handle = item.Handle
			if strings.TrimSpace(item.Content) != "public required string DisplayName { get; init; }" {
				t.Errorf("property source=%q", item.Content)
			}
		}
	}
	if handle == "" {
		t.Fatal("DisplayName missing from Query")
	}
	self, err := a.Expand(ctx, []string{handle}, "self", 1200)
	if err != nil || len(self.Items) != 1 {
		t.Fatalf("self=%+v err=%v", self, err)
	}
	if strings.TrimSpace(self.Items[0].Content) != "public required string DisplayName { get; init; }" {
		t.Errorf("self source=%q", self.Items[0].Content)
	}
	caller, err := a.Query(ctx, QueryRequest{Query: "Run", TokenBudget: 1200, Mode: "source"})
	if err != nil {
		t.Fatal(err)
	}
	var runHandle string
	for _, item := range caller.Items {
		if item.Symbol == "Run" {
			runHandle = item.Handle
		}
	}
	if runHandle == "" {
		t.Fatal("Run missing")
	}
	callees, err := a.Expand(ctx, []string{runHandle}, "callees", 1200)
	if err != nil {
		t.Fatal(err)
	}
	save := false
	for _, item := range callees.Items {
		if item.Symbol == "Save" {
			save = true
		}
	}
	if !save {
		t.Fatalf("expression callee absent: %+v", callees.Items)
	}
	for _, limit := range []int{512, 1200} {
		packet, err := a.QueryEvidence(ctx, EvidenceQueryRequest{Query: "DisplayName", TokenBudget: limit, Mode: evidence.ModeSource, NoUpdate: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := evidence.Validate(packet.Packet); err != nil {
			t.Fatal(err)
		}
		if used := evidence.MeasureModelVisible(packet.Packet, nil); used > limit || packet.Packet.Budget.Used != used {
			t.Errorf("wire budget limit=%d used=%d reported=%d", limit, used, packet.Packet.Budget.Used)
		}
		found := false
		for _, item := range packet.Packet.Evidence {
			if item.Symbol == "DisplayName" {
				found = true
				if item.Fidelity != evidence.FidelityVerbatim || strings.TrimSpace(item.Source) != "public required string DisplayName { get; init; }" {
					t.Errorf("property evidence=%+v", item)
				}
			}
		}
		if !found {
			t.Fatalf("property Evidence missing at budget %d", limit)
		}
	}
	writeAppFile(t, filepath.Join(root, "Service.cs"), strings.Replace(source, "return value;", "return default;", 1))
	impact, err := a.Impact(ctx, "", "", 1600)
	if err != nil {
		t.Fatal(err)
	}
	echo := false
	for _, item := range impact.Items {
		if item.Symbol == "Echo" {
			echo = true
		}
	}
	if !echo {
		t.Fatalf("generic method absent from Impact: %+v", impact.Items)
	}
}

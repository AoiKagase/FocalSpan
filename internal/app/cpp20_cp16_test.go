package app

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var _ = os.WriteFile
var _ = filepath.Join
var _ = config.Default
var _ = model.Symbol{}
var _ = reflect.DeepEqual
var _ = strings.Contains
var _ = json.Marshal
var _ = evidence.ModeSource

func TestCPP20_CP16(t *testing.T) {
	t.Run("C17_module_expansion", func(t *testing.T) {
		s, _ := cppAppAuditService(t, map[string]string{"utilities.cppm": "export module utilities;\nexport void helper() {}\n", "main.cpp": "import utilities;\nvoid run() {}\n"})
		by, _ := cppAppAuditGraph(t, s)
		var from string
		for h, v := range by {
			if v.FilePath == "main.cpp" && v.Kind == "translation_unit" {
				from = h
			}
		}
		if from == "" {
			t.Fatal("importing unit missing")
		}
		p, e := s.ExpandEvidence(context.Background(), EvidenceExpandRequest{Handles: []string{from}, Relation: "imports", TokenBudget: 4000, Mode: evidence.ModeSource})
		if e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(p.Packet)
		t.Log("PACKET " + string(b))
		found := false
		for _, v := range p.Packet.Evidence {
			if v.Location.Path == "utilities.cppm" {
				found = true
			}
		}
		if !found {
			t.Error("C++ module import expansion omits provider")
		}
	})
}

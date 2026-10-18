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

func TestCPP20_CP03(t *testing.T) {
	t.Run("C03_custom_return_query", func(t *testing.T) {
		s, _ := cppAppAuditService(t, map[string]string{"main.cpp": "struct Item {};\nItem make() { return {}; }\nint known_function() { return 1; }\n"})
		for _, name := range []string{"known_function", "make"} {
			p, e := s.QueryEvidence(context.Background(), EvidenceQueryRequest{Query: name, TokenBudget: 4000, Mode: evidence.ModeSource})
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, v := range p.Packet.Evidence {
				if strings.Contains(v.Source, name+"(") {
					found = true
				}
			}
			if !found {
				t.Errorf("query %s did not return its source", name)
			}
		}
	})
}

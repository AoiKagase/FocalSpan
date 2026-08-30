package evidence

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

func FuzzCompile(f *testing.F) {
	seeds := []string{
		"",
		"func Ready() bool",
		"func Validate() error {\n" + strings.Repeat("\twork()\n", 200) + "}\n",
		`const char* raw = R"tag({"token":"expired"})tag";`,
		`var text = $$"""token = {{value}}""";`,
		"const view = <Panel>{`token ${value}`}</Panel>;",
		"$value = <<<TOKEN\nexpired\nTOKEN;",
		"{block name=\"auth\"}<script>const token = `expired`;</script>{/block}",
		"func CRLF() {\r\n\treturn\r\n}\r\n",
		"// 期限切れトークン\nfunc 認証する() bool { return false }\n",
		string([]byte{'a', 0xff, 'b'}),
	}
	for _, seed := range seeds {
		f.Add(seed, 1200, false)
	}
	f.Fuzz(func(t *testing.T, source string, requested int, known bool) {
		if !utf8.ValidString(source) {
			source = strings.ToValidUTF8(source, "�")
		}
		if len(source) > 128*1024 {
			t.Skip()
		}
		requested = normalizeFuzzBudget(requested)
		candidate := model.RankedCandidate{Handle: "target", Path: "src/fuzz.go", Language: "go", Kind: "function", Symbol: "FuzzTarget", Signature: "func FuzzTarget()", StartLine: 1, EndLine: maxInt(1, 1+strings.Count(source, "\n")), Content: source, ContentHash: fmt.Sprintf("%x", len(source)), Reasons: []model.ScoreReason{{Code: "symbol-exact"}}}
		req := CompileRequest{Plan: query.Plan{RawQuery: "FuzzTarget", PrimaryIntent: query.IntentDefinition, Intents: []query.Intent{query.IntentDefinition}, Anchors: []string{"FuzzTarget"}}, TokenBudget: requested, Mode: ModeFocused, Candidates: []model.RankedCandidate{candidate}}
		if known {
			req.KnownHandles = []string{"target"}
		}
		compiler := NewCompiler(nil)
		first, err := compiler.Compile(req)
		if err != nil {
			t.Skip()
		}
		second, err := compiler.Compile(req)
		if err != nil {
			t.Fatalf("repeat compile: %v", err)
		}
		assertFuzzPacket(t, first.Packet, requested, req.KnownHandles)
		firstJSON, _ := json.Marshal(first.Packet)
		secondJSON, _ := json.Marshal(second.Packet)
		if string(firstJSON) != string(secondJSON) {
			t.Fatal("compile output is nondeterministic")
		}
	})
}

func FuzzValidate(f *testing.F) {
	valid := Packet{Schema: SchemaContextV1, Mode: ModeFocused, Budget: Budget{Limit: 256, Used: 100}, Evidence: []Item{}}
	data, _ := json.Marshal(valid)
	f.Add(data)
	f.Add([]byte(`{"schema":"focalspan.context.v1","mode":"focused","budget":{"limit":256,"used":10},"evidence":[]}`))
	f.Add([]byte{0xff, 0x00, '{'})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 128*1024 {
			t.Skip()
		}
		var packet Packet
		if err := json.Unmarshal(data, &packet); err != nil {
			return
		}
		if err := Validate(packet); err != nil {
			return
		}
		encoded, err := json.Marshal(packet)
		if err != nil || !json.Valid(encoded) {
			t.Fatalf("valid packet did not serialize: %v", err)
		}
	})
}

func assertFuzzPacket(t *testing.T, packet Packet, requested int, known []string) {
	t.Helper()
	if err := Validate(packet); err != nil {
		t.Fatalf("invalid packet: %v", err)
	}
	if packet.Budget.Used != MeasureModelVisible(packet, budget.NewEstimator()) || packet.Budget.Used > packet.Budget.Limit {
		t.Fatalf("wire budget mismatch: %+v", packet.Budget)
	}
	wantLimit := requested
	if wantLimit < budget.MinBudget {
		wantLimit = budget.MinBudget
	}
	if wantLimit > budget.MaxBudget {
		wantLimit = budget.MaxBudget
	}
	if packet.Budget.Limit != wantLimit {
		t.Fatalf("limit=%d want=%d", packet.Budget.Limit, wantLimit)
	}
	knownSet := map[string]bool{}
	for _, handle := range known {
		knownSet[handle] = true
	}
	ids := map[string]bool{}
	for index, item := range packet.Evidence {
		if item.ID != fmt.Sprintf("e%d", index+1) || knownSet[item.Handle] {
			t.Fatalf("invalid local id or known resend: %+v", item)
		}
		ids[item.ID] = true
		if !utf8.ValidString(item.Source) {
			t.Fatal("invalid UTF-8 source")
		}
		for _, segment := range item.Segments {
			if !utf8.ValidString(segment.Text) {
				t.Fatal("invalid UTF-8 segment")
			}
		}
	}
	for _, edge := range packet.Relations {
		if !ids[edge.From] || !ids[edge.To] {
			t.Fatalf("non-local edge: %+v", edge)
		}
	}
	payload, _ := json.Marshal(packet)
	if hasForbiddenFuzzKey(payload) {
		t.Fatalf("forbidden output key: %s", payload)
	}
}

func hasForbiddenFuzzKey(payload []byte) bool {
	var value any
	if json.Unmarshal(payload, &value) != nil {
		return true
	}
	forbidden := map[string]bool{"score": true, "retrieval_score": true, "weight": true, "detail": true, "token_savings": true, "baseline_tokens": true, "saved_tokens": true, "savings_ratio": true}
	var walk func(any) bool
	walk = func(value any) bool {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if forbidden[key] || walk(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if walk(child) {
					return true
				}
			}
		}
		return false
	}
	return walk(value)
}

func normalizeFuzzBudget(value int) int {
	if value < -1000 {
		return -1000
	}
	if value > 65000 {
		return 65000
	}
	return value
}

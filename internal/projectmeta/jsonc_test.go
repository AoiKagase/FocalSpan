package projectmeta

import (
	"context"
	"encoding/json"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestTSConfigAcceptsJSONCWithoutChangingStrings(t *testing.T) {
	text := "\ufeff{\n// comment\n\"compilerOptions\":{\"baseUrl\":\"src\",/* block */\"paths\":{\"@app/*\":[\"https://example.invalid/a//b/*\",],},},\n}"
	facts, diagnostics, err := ParseFile(context.Background(), "", model.SourceFile{Path: "tsconfig.json", Content: []byte(text)})
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("valid JSONC: %v %+v", err, diagnostics)
	}
	for _, f := range facts {
		if f.Kind == "path-alias" {
			return
		}
	}
	t.Fatalf("JSONC alias missing: %+v", facts)
}
func TestPackageJSONRemainsStrict(t *testing.T) {
	_, diagnostics, err := ParseFile(context.Background(), "", model.SourceFile{Path: "package.json", Content: []byte("{\"name\":\"package\",}")})
	if err != nil || len(diagnostics) == 0 {
		t.Fatalf("non-JSON package accepted: %v %+v", err, diagnostics)
	}
}

func TestJSONCPreservesStringContents(t *testing.T) {
	data, err := jsonc(`{"url":"https://host/a//b/*",/*x*/"literal":",}","escaped":"a\\\"b",}`)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	if values["url"] != "https://host/a//b/*" || values["literal"] != ",}" {
		t.Fatalf("strings changed: %+v", values)
	}
	if _, err := jsonc(`{"a":1,/* unclosed`); err == nil {
		t.Fatal("unclosed comment accepted")
	}
}

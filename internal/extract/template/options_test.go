package template

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

func TestEmbeddedFallbackUsesConfiguredWindows(t *testing.T) {
	content := "<script>\n" + strings.Repeat("x;\n", 10) + "</script>\n"
	got, err := NewExtractorWithChunkOptions(2, 0).Extract(context.Background(), model.SourceFile{Path: "page.tpl", Language: "template", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, c := range got.Chunks {
		if c.Language == "javascript" {
			count++
			if c.EndLine-c.StartLine+1 > 2 {
				t.Fatalf("oversize embedded fallback %+v", c)
			}
		}
	}
	if count != 5 {
		t.Fatalf("embedded chunks=%d want=5", count)
	}
}

package generic

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestCLikeIgnoresBracesInStringsAndComments(t *testing.T) {
	content := "class Service {\n  string text = \"}\";\n  /* { ignored */\n  int Validate() { return 1; }\n};\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "service.cs", Language: "csharp", Content: []byte(content)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Chunks) != 1 || !strings.Contains(got.Chunks[0].Content, "Validate") || got.Chunks[0].EndLine != 5 {
		t.Fatalf("chunks=%+v", got.Chunks)
	}
}

func TestPythonAndMarkdownBoundaries(t *testing.T) {
	py := "class Service:\n    def validate(self):\n        return True\n\ndef helper():\n    return False\n"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "service.py", Language: "python", Content: []byte(py)})
	if err != nil || len(got.Chunks) != 2 {
		t.Fatalf("python chunks=%+v err=%v", got.Chunks, err)
	}
	md := "# Auth\nintro\n\n## Expired tokens\nreject expired\n"
	got, err = NewExtractor().Extract(context.Background(), model.SourceFile{Path: "README.md", Language: "markdown", Content: []byte(md)})
	if err != nil || len(got.Chunks) != 2 || got.Chunks[1].StartLine != 4 {
		t.Fatalf("markdown chunks=%+v err=%v", got.Chunks, err)
	}
}

func TestFallbackWindowsOverlapAndLineSafety(t *testing.T) {
	lines := make([]string, 0, 175)
	for i := 0; i < 175; i++ {
		lines = append(lines, "line")
	}
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "notes.txt", Language: "text", Content: []byte(strings.Join(lines, "\n"))})
	if err != nil || len(got.Chunks) < 2 {
		t.Fatalf("chunks=%+v err=%v", got.Chunks, err)
	}
	for _, chunk := range got.Chunks {
		if chunk.EndLine-chunk.StartLine+1 > 160 || !strings.HasSuffix(chunk.Content, "line") {
			t.Fatalf("unsafe chunk=%+v", chunk)
		}
	}
}

package generic

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"strings"
	"testing"
)

func TestConfiguredFallbackWindows(t *testing.T) {
	for _, language := range []string{"text", "markdown", "csharp", "javascript", "c", "python"} {
		t.Run(language, func(t *testing.T) {
			source := strings.Repeat("plain text\r\n", 10)
			got, err := NewExtractorWithOptions(2, 0).Extract(context.Background(), model.SourceFile{Path: "notes.txt", Language: language, Content: []byte(source)})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Chunks) != 5 {
				t.Fatalf("chunks=%d", len(got.Chunks))
			}
			for _, c := range got.Chunks {
				if c.EndLine-c.StartLine+1 > 2 || string([]byte(source)[c.StartByte:c.EndByte]) != c.Content {
					t.Fatalf("invalid configured chunk %+v", c)
				}
			}
		})
	}
}

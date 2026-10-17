package generic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestGenericChunksPreserveOriginalSourceBytes(t *testing.T) {
	for _, tc := range []struct {
		name, language, raw, want string
		start, first, last        int
	}{
		{"LF", "text", "alpha\nbeta\n", "alpha\nbeta", 0, 1, 2},
		{"CRLF declaration", "powershell", "# note\r\nfunction ValidateToken {\r\n    return $true\r\n}\r\n", "function ValidateToken {\r\n    return $true\r\n}", 8, 2, 4},
		{"mixed UTF8 EOF", "text", "日本語\r\none\nlast", "日本語\r\none\nlast", 0, 1, 3},
		{"bare CR", "text", "alpha\rbeta", "alpha\rbeta", 0, 1, 1},
		{"empty", "text", "", "", 0, 1, 1},
		{"empty CRLF line", "text", "\r\n", "", 0, 1, 1},
		{"blank last selected line", "markdown", "# Title\r\nbody\r\n\r\n", "# Title\r\nbody\r\n", 0, 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "fixture", Language: tc.language, Content: []byte(tc.raw)})
			if err != nil || len(got.Chunks) != 1 || len(got.Symbols) != 1 {
				t.Fatalf("extraction=%+v err=%v", got, err)
			}
			chunk, symbol := got.Chunks[0], got.Symbols[0]
			if chunk.Content != tc.want || chunk.StartByte != tc.start || chunk.EndByte != tc.start+len(tc.want) || chunk.StartLine != tc.first || chunk.EndLine != tc.last {
				t.Fatalf("source/position mismatch: chunk=%+v want=%q start=%d lines=%d:%d", chunk, tc.want, tc.start, tc.first, tc.last)
			}
			if tc.raw[chunk.StartByte:chunk.EndByte] != chunk.Content || symbol.StartByte != chunk.StartByte || symbol.EndByte != chunk.EndByte {
				t.Fatalf("symbol/chunk range does not select original source: %+v %+v", symbol, chunk)
			}
			digest := sha256.Sum256([]byte(tc.want))
			if chunk.ContentHash != hex.EncodeToString(digest[:]) {
				t.Fatalf("hash does not describe original content: %q", chunk.ContentHash)
			}
		})
	}
}

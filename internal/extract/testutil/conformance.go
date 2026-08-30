package testutil

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/sourceutil"
	"github.com/focalspan/focalspan/internal/model"
)

// AssertExtraction checks the source, handle, relation, and diagnostic invariants
// shared by every extractor.
func AssertExtraction(t testing.TB, file model.SourceFile, got model.Extraction) {
	t.Helper()
	byHandle := make(map[string]struct{}, len(got.Symbols))
	for _, symbol := range got.Symbols {
		if symbol.Handle == "" {
			t.Errorf("symbol %q has empty handle", symbol.Name)
		}
		if _, exists := byHandle[symbol.Handle]; exists {
			t.Errorf("duplicate symbol handle %q", symbol.Handle)
		}
		byHandle[symbol.Handle] = struct{}{}
		assertSpan(t, file, symbol.FilePath, symbol.StartByte, symbol.EndByte, symbol.StartLine, symbol.EndLine, "symbol "+symbol.Name)
		if symbol.Confidence < 0 || symbol.Confidence > 1 {
			t.Errorf("symbol %q confidence=%v", symbol.Name, symbol.Confidence)
		}
		if symbol.ParentHandle != "" {
			if _, exists := byHandle[symbol.ParentHandle]; !exists {
				t.Errorf("symbol %q has unknown parent %q", symbol.Name, symbol.ParentHandle)
			}
		}
	}
	for _, chunk := range got.Chunks {
		if chunk.Handle == "" {
			t.Errorf("chunk %q has empty handle", chunk.SymbolName)
		}
		if _, exists := byHandle[chunk.Handle]; exists {
			t.Errorf("chunk handle %q collides with symbol handle", chunk.Handle)
		}
		byHandle[chunk.Handle] = struct{}{}
		if chunk.SymbolHandle == "" {
			t.Errorf("chunk %q has empty symbol handle", chunk.Handle)
		} else if _, exists := byHandle[chunk.SymbolHandle]; !exists {
			t.Errorf("chunk %q references unknown symbol %q", chunk.Handle, chunk.SymbolHandle)
		}
		assertSpan(t, file, chunk.FilePath, chunk.StartByte, chunk.EndByte, chunk.StartLine, chunk.EndLine, "chunk "+chunk.Handle)
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			if !strings.Contains(strings.ToLower(chunk.Signature), "synthetic") {
				t.Errorf("zero-byte chunk %q is not marked synthetic", chunk.Handle)
			}
			continue
		}
		if got := string(file.Content[chunk.StartByte:chunk.EndByte]); got != chunk.Content {
			t.Errorf("chunk %q content does not match source: got=%q want=%q", chunk.Handle, chunk.Content, got)
		}
	}
	seenRelations := make(map[string]struct{}, len(got.Relations))
	for _, relation := range got.Relations {
		if _, exists := byHandle[relation.FromHandle]; !exists {
			t.Errorf("relation %q has unknown from handle %q", relation.Kind, relation.FromHandle)
		}
		if relation.ToHandle != "" && relation.UnresolvedTo != "" {
			t.Errorf("relation %q has both resolved and unresolved target", relation.Kind)
		}
		if relation.ToHandle != "" {
			if _, exists := byHandle[relation.ToHandle]; !exists {
				t.Errorf("relation %q has unknown to handle %q", relation.Kind, relation.ToHandle)
			}
		}
		if relation.Confidence < 0 || relation.Confidence > 1 {
			t.Errorf("relation %q confidence=%v", relation.Kind, relation.Confidence)
		}
		key := fmt.Sprintf("%s\x00%s\x00%s\x00%s", relation.FromHandle, relation.ToHandle, relation.UnresolvedTo, relation.Kind)
		if _, exists := seenRelations[key]; exists {
			t.Errorf("duplicate relation %q", key)
		}
		seenRelations[key] = struct{}{}
	}
	for _, diagnostic := range got.Diagnostics {
		if len(file.Content) > 0 && strings.Contains(diagnostic.Message, string(file.Content)) {
			t.Errorf("diagnostic %q contains complete source", diagnostic.Code)
		}
	}
}

// AssertNoSourceDuplication bounds the aggregate size of source-backed chunks.
func AssertNoSourceDuplication(t testing.TB, file model.SourceFile, got model.Extraction, maxRatio float64) {
	t.Helper()
	if len(file.Content) == 0 {
		return
	}
	total := 0
	for _, chunk := range got.Chunks {
		if chunk.StartByte != 0 || chunk.EndByte != 0 {
			total += len(chunk.Content)
		}
	}
	if ratio := float64(total) / float64(len(file.Content)); ratio > maxRatio {
		t.Fatalf("source-backed chunk ratio=%0.3f exceeds %0.3f", ratio, maxRatio)
	}
}

// AssertDeterministic runs one extractor twice and compares the complete result.
func AssertDeterministic(t testing.TB, extractor extract.Extractor, file model.SourceFile) {
	t.Helper()
	first, firstErr := extractor.Extract(context.Background(), file)
	second, secondErr := extractor.Extract(context.Background(), file)
	if (firstErr == nil) != (secondErr == nil) || firstErr != nil && firstErr.Error() != secondErr.Error() {
		t.Fatalf("extractor errors differ: first=%v second=%v", firstErr, secondErr)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("extractor output is not deterministic: first=%+v second=%+v", first, second)
	}
}

func assertSpan(t testing.TB, file model.SourceFile, path string, startByte, endByte, startLine, endLine int, label string) {
	t.Helper()
	if path != file.Path {
		t.Errorf("%s path=%q want=%q", label, path, file.Path)
	}
	if startByte < 0 || endByte < startByte || endByte > len(file.Content) {
		t.Errorf("%s invalid byte span [%d,%d) for %d bytes", label, startByte, endByte, len(file.Content))
	}
	if startLine < 1 || endLine < startLine || endLine > sourceutil.NewSourceMap(file.Content).LineCount() {
		t.Errorf("%s invalid line span [%d,%d]", label, startLine, endLine)
	}
	if !sourceutil.ValidUTF8Boundary(file.Content, startByte) || !sourceutil.ValidUTF8Boundary(file.Content, endByte) {
		t.Errorf("%s splits UTF-8 at [%d,%d)", label, startByte, endByte)
	}
}

package linker

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/store"
)

func TestSchemaV2CurrentScaleBenchmark(t *testing.T) {
	if os.Getenv("FOCALSPAN_SCHEMA_V2_BENCH") != "1" {
		t.Skip("set FOCALSPAN_SCHEMA_V2_BENCH=1 to run the wall-clock benchmark")
	}
	const (
		fileCount       = 450
		symbolCount     = 5000
		relationCount   = 28000
		unresolvedCount = 21676
	)
	root := t.TempDir()
	s, err := store.Open(root, ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := seedCurrentScaleFixture(s, fileCount, symbolCount, relationCount, unresolvedCount); err != nil {
		t.Fatal(err)
	}
	linker := &Linker{Store: s}

	started := time.Now()
	if err := linker.LinkWithScope(context.Background(), nil, store.LinkScope{}, nil); err != nil {
		t.Fatal(err)
	}
	unchanged := time.Since(started)

	started = time.Now()
	if err := linker.LinkWithScope(context.Background(), nil, store.LinkScope{ChangedPaths: []string{"src/file-000.go"}}, nil); err != nil {
		t.Fatal(err)
	}
	small := time.Since(started)

	started = time.Now()
	if err := linker.LinkWithScope(context.Background(), nil, store.LinkScope{Full: true}, nil); err != nil {
		t.Fatal(err)
	}
	full := time.Since(started)
	t.Logf("schema-v2 current-scale files=%d symbols=%d relations=%d unresolved=%d unchanged=%s small-related=%s full=%s", fileCount, symbolCount, relationCount, unresolvedCount, unchanged, small, full)
}

func seedCurrentScaleFixture(s *store.Store, fileCount, symbolCount, relationCount, unresolvedCount int) error {
	if fileCount <= 0 || symbolCount < fileCount || relationCount < 0 || unresolvedCount < 0 || unresolvedCount > relationCount {
		return fmt.Errorf("invalid current-scale fixture dimensions")
	}
	baseSymbols := symbolCount / fileCount
	extraSymbols := symbolCount % fileCount
	relationsLeft := relationCount
	unresolvedLeft := unresolvedCount
	for fileIndex := 0; fileIndex < fileCount; fileIndex++ {
		count := baseSymbols
		if fileIndex < extraSymbols {
			count++
		}
		path := fmt.Sprintf("src/file-%03d.go", fileIndex)
		symbols := make([]model.Symbol, 0, count)
		for symbolIndex := 0; symbolIndex < count; symbolIndex++ {
			handle := fmt.Sprintf("symbol-%04d-%02d", fileIndex, symbolIndex)
			symbols = append(symbols, model.Symbol{Handle: handle, FilePath: path, Language: "go", Kind: "function", Name: handle, QualifiedName: "fixture." + handle, Signature: "func " + handle + "()", StartLine: symbolIndex + 1, EndLine: symbolIndex + 1, Confidence: 1})
		}
		relations := make([]model.Relation, 0, relationsLeft/fileCount+1)
		for relationsLeft > 0 && len(relations) < (relationCount/fileCount+1) {
			currentIndex := relationCount - relationsLeft
			relation := model.Relation{FromHandle: symbols[0].Handle, Kind: "calls", Confidence: 0.5, Source: "benchmark"}
			if unresolvedLeft > 0 {
				relation.UnresolvedTo = fmt.Sprintf("missing.%05d", currentIndex)
				unresolvedLeft--
			} else {
				relation.ToHandle = "symbol-0000-00"
			}
			relations = append(relations, relation)
			relationsLeft--
		}
		if err := s.ReplaceFile(context.Background(), model.SourceFile{Path: path, Language: "go", SHA256: path}, model.Extraction{Symbols: symbols, Relations: relations}); err != nil {
			return fmt.Errorf("seed %s: %w", path, err)
		}
	}
	return nil
}

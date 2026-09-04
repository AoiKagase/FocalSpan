package linker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
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

func TestDurationSpeedupHandlesZeroCandidateDuration(t *testing.T) {
	if got := durationSpeedup(10*time.Millisecond, 0); got < 10 {
		t.Fatalf("durationSpeedup() = %f, want at least 10", got)
	}
	if got := durationSpeedup(0, time.Millisecond); got != 0 {
		t.Fatalf("durationSpeedup() = %f, want 0 for zero baseline", got)
	}
}

func TestRelationRowsEqualDetectsDifferences(t *testing.T) {
	left := []model.Relation{{FromHandle: "a", ToHandle: "b", Kind: "calls", Confidence: 1, Source: "test"}}
	right := append([]model.Relation(nil), left...)
	if !relationRowsEqual(left, right) {
		t.Fatal("identical relation rows were reported as different")
	}
	right[0].ToHandle = "c"
	if relationRowsEqual(left, right) {
		t.Fatal("different relation rows were reported as equal")
	}
}

func TestSchemaV2CurrentScaleComparator(t *testing.T) {
	if os.Getenv("FOCALSPAN_SCHEMA_V2_COMPARATOR") != "1" {
		t.Skip("set FOCALSPAN_SCHEMA_V2_COMPARATOR=1 to run the v1/v2 comparator")
	}
	const (
		fileCount       = 450
		symbolCount     = 5000
		relationCount   = 28000
		unresolvedCount = 21676
	)
	type scenario struct {
		name         string
		scope        store.LinkScope
		maximum      time.Duration
		minimumRatio float64
	}
	scenarios := []scenario{
		{name: "unchanged", scope: store.LinkScope{}, maximum: 250 * time.Millisecond, minimumRatio: 10},
		{name: "small-related", scope: store.LinkScope{ChangedPaths: []string{"src/file-000.go"}}, maximum: time.Second, minimumRatio: 5},
		{name: "full", scope: store.LinkScope{Full: true}, maximum: 5 * time.Second, minimumRatio: 2},
	}

	for _, test := range scenarios {
		t.Run(test.name, func(t *testing.T) {
			legacy := measureCurrentScaleLink(t, fileCount, symbolCount, relationCount, unresolvedCount, func(ctx context.Context, s *store.Store) (int, error) {
				relations, err := s.Relations(ctx)
				if err != nil {
					return 0, err
				}
				return len(relations), legacyLinkAll(ctx, s)
			})
			candidate := measureCurrentScaleLink(t, fileCount, symbolCount, relationCount, unresolvedCount, func(ctx context.Context, s *store.Store) (int, error) {
				relations, err := s.RelationsForLink(ctx, test.scope)
				if err != nil {
					return 0, err
				}
				return len(relations), (&Linker{Store: s}).LinkWithScope(ctx, nil, test.scope, nil)
			})
			ratio := durationSpeedup(legacy.duration, candidate.duration)
			equal := relationRowsEqual(legacy.relations, candidate.relations)
			t.Logf("schema-v2 comparator scenario=%s v1=%s v2=%s speedup=%.2fx v1_candidates=%d v2_candidates=%d rows=%d relation_hash=%s equal=%t", test.name, legacy.duration, candidate.duration, ratio, legacy.candidates, candidate.candidates, len(candidate.relations), relationRowsHash(candidate.relations), equal)
			if !equal {
				t.Fatal("v1/v2 relation rows differ")
			}
			if candidate.duration > test.maximum {
				t.Errorf("v2 duration %s exceeds %s", candidate.duration, test.maximum)
			}
			if ratio < test.minimumRatio {
				t.Errorf("speedup %.2fx is below %.2fx", ratio, test.minimumRatio)
			}
		})
	}
}

type currentScaleMeasurement struct {
	duration   time.Duration
	candidates int
	relations  []model.Relation
}

func measureCurrentScaleLink(t *testing.T, fileCount, symbolCount, relationCount, unresolvedCount int, run func(context.Context, *store.Store) (int, error)) currentScaleMeasurement {
	t.Helper()
	s, err := store.Open(t.TempDir(), ".focalspan")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := seedCurrentScaleFixture(s, fileCount, symbolCount, relationCount, unresolvedCount); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	started := time.Now()
	candidates, err := run(ctx, s)
	duration := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	relations, err := s.Relations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return currentScaleMeasurement{duration: duration, candidates: candidates, relations: relations}
}

func legacyLinkAll(ctx context.Context, s *store.Store) error {
	symbols, err := s.Symbols(ctx)
	if err != nil {
		return err
	}
	relations, err := s.Relations(ctx)
	if err != nil {
		return err
	}
	byHandle := make(map[string]model.Symbol, len(symbols))
	for _, symbol := range symbols {
		byHandle[symbol.Handle] = symbol
	}
	for _, relation := range relations {
		if err := ctx.Err(); err != nil {
			return err
		}
		if relation.UnresolvedTo == "" || relation.ToHandle != "" {
			continue
		}
		from := byHandle[relation.FromHandle]
		candidates := resolveCandidates(from, relation.UnresolvedTo, symbols, nil)
		if len(candidates) != 1 {
			continue
		}
		if err := s.LinkRelation(ctx, relation.FromHandle, relation.UnresolvedTo, relation.Kind, candidates[0].Handle); err != nil {
			return err
		}
	}
	return nil
}

func durationSpeedup(baseline, candidate time.Duration) float64 {
	if baseline <= 0 {
		return 0
	}
	if candidate <= 0 {
		return math.Inf(1)
	}
	return float64(baseline) / float64(candidate)
}

func relationRowsEqual(left, right []model.Relation) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func relationRowsHash(relations []model.Relation) string {
	payload, err := json.Marshal(relations)
	if err != nil {
		return "marshal-error"
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
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

package store

import (
	"context"
	"fmt"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestCandidateScopeBatchesLiteralPathsAndRanges(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	storeSeedFile(t, s, "dir%_/auth.go", "go", "selected", "ValidateToken", "ValidateToken", "ValidateToken selected", 10, 12, 1)
	storeSeedFile(t, s, "dirXY/auth.go", "go", "wildcard", "ValidateToken", "ValidateToken", "ValidateToken wildcard", 10, 12, 1)
	storeSeedFile(t, s, "dir%_extra/auth.go", "go", "boundary", "ValidateToken", "ValidateToken", "ValidateToken boundary", 10, 12, 1)
	paths := make([]string, 0, 1002)
	changed := make(map[string][]model.LineRange)
	for n := 0; n < 1001; n++ {
		p := fmt.Sprintf("absent/%d", n)
		paths = append(paths, p)
		changed[p] = []model.LineRange{{Start: 1, End: 2}}
	}
	paths = append(paths, "dir%_")
	changed["dir%_/auth.go"] = []model.LineRange{{Start: 12, End: 12}}
	reader := s.ScopedCandidates(model.CandidateScope{Paths: paths, ChangedOnly: true, Changed: changed})
	got, err := reader.SearchFTS(ctx, "ValidateToken", 1)
	if err != nil || len(got) != 1 || got[0].Path != "dir%_/auth.go" {
		t.Fatalf("batched scope=%+v err=%v", got, err)
	}
	for _, scope := range []model.CandidateScope{{ChangedOnly: true}, {Paths: []string{"DIR%_"}}, {ChangedOnly: true, Changed: map[string][]model.LineRange{"dir%_/auth.go": {{Start: 13, End: 13}}}}} {
		got, err := s.ScopedCandidates(scope).SearchExactSymbols(ctx, []string{"ValidateToken"}, 50)
		if err != nil || len(got) != 0 {
			t.Fatalf("scope leaked: %+v err=%v", got, err)
		}
	}
}

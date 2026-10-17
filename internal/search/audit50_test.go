package search

import (
	"context"
	"github.com/focalspan/focalspan/internal/query"
	"testing"
)

func TestF23NoSearchableTerms(t *testing.T) {
	for _, mode := range []RetrievalMode{RetrievalFull, RetrievalFTSOnly, RetrievalNoRelations} {
		reader := &retrievalRecordingStore{errFor: RetrieverFTS}
		if _, err := NewRetrieverSet(reader).Retrieve(context.Background(), query.Plan{}, SearchRequest{Query: "!!!", Mode: mode}); err != nil {
			t.Fatal(err)
		}
		for _, id := range reader.called {
			if id == RetrieverFTS {
				t.Fatalf("empty normalized query invoked FTS in mode %s", mode)
			}
		}
	}
}

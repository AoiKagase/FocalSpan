package model

import "context"

type LineRange struct{ Start, End int }

type CandidateScope struct {
	Paths       []string
	ChangedOnly bool
	Changed     map[string][]LineRange
}

type CandidateReader interface {
	SearchFTS(context.Context, string, int) ([]RankedCandidate, error)
	SearchQualifiedSymbols(context.Context, []string, int) ([]RankedCandidate, error)
	SearchExactSymbols(context.Context, []string, int) ([]RankedCandidate, error)
	SearchSymbolPrefixes(context.Context, []string, int) ([]RankedCandidate, error)
	SearchPaths(context.Context, []string, int) ([]RankedCandidate, error)
	RelatedCandidates(context.Context, []string, string) ([]RankedCandidate, error)
	RelatedCandidateHits(context.Context, []string, string) ([]RelationHit, error)
}

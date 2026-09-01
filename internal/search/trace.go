package search

import (
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

type RetrievalMode string
type RetrieverID string

const (
	RetrievalFull        RetrievalMode = "full"
	RetrievalFTSOnly     RetrievalMode = "fts-only"
	RetrievalNoRelations RetrievalMode = "no-relations"
)

const (
	RetrieverQualified      RetrieverID = "qualified-symbol"
	RetrieverSymbol         RetrieverID = "symbol-exact"
	RetrieverPrefix         RetrieverID = "symbol-prefix"
	RetrieverFTS            RetrieverID = "fts"
	RetrieverPath           RetrieverID = "path"
	RetrieverIdentityBridge RetrieverID = "identity-bridge"
	RetrieverRelation       RetrieverID = "relation"
)

type RankedList struct {
	Retriever RetrieverID
	Items     []model.RankedCandidate
}

type RetrievalContribution struct {
	Retriever    RetrieverID `json:"retriever"`
	Rank         int         `json:"rank"`
	Weight       float64     `json:"weight"`
	Contribution float64     `json:"contribution"`
}

type CandidateTrace struct {
	Handle         string                  `json:"handle"`
	Path           string                  `json:"path"`
	Symbol         string                  `json:"symbol"`
	Kind           string                  `json:"kind"`
	StartLine      int                     `json:"start_line"`
	EndLine        int                     `json:"end_line"`
	RankedPosition int                     `json:"ranked_position"`
	Contributions  []RetrievalContribution `json:"contributions"`
	FusionScore    float64                 `json:"fusion_score"`
	FinalScore     float64                 `json:"final_score"`
	Reasons        []model.ScoreReason     `json:"reasons"`
}

type StageCandidateTrace struct {
	Retriever        RetrieverID `json:"retriever"`
	Position         int         `json:"position"`
	Path             string      `json:"path"`
	Symbol           string      `json:"symbol"`
	Kind             string      `json:"kind"`
	Relation         string      `json:"relation,omitempty"`
	RelationResolved bool        `json:"relation_resolved,omitempty"`
}

type RetrieverSummary struct {
	Retriever RetrieverID `json:"retriever"`
	Count     int         `json:"count"`
}

type SearchTrace struct {
	Mode       RetrievalMode         `json:"mode"`
	Lists      []RetrieverSummary    `json:"lists"`
	Retrieved  []StageCandidateTrace `json:"retrieved"`
	Candidates []CandidateTrace      `json:"candidates"`
}

type SearchResult struct {
	Plan       query.Plan
	Candidates []model.RankedCandidate
	Trace      *SearchTrace
}

package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/gitx"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
	"github.com/focalspan/focalspan/internal/rank"
	"github.com/focalspan/focalspan/internal/repository"
	"github.com/focalspan/focalspan/internal/search"
)

type candidateResult struct {
	Plan        query.Plan
	Revision    string
	Candidates  []model.RankedCandidate
	Diagnostics []string
}

type EvidenceQueryRequest struct {
	Query         string
	TokenBudget   int
	Mode          evidence.Mode
	ChangedOnly   bool
	Paths         []string
	NoUpdate      bool
	RetrievalMode search.RetrievalMode
	KnownHandles  []string
}

type EvidenceExpandRequest struct {
	Handles      []string
	Relation     string
	TokenBudget  int
	Mode         evidence.Mode
	KnownHandles []string
}

type EvidenceImpactRequest struct {
	BaseRef      string
	HeadRef      string
	TokenBudget  int
	Mode         evidence.Mode
	KnownHandles []string
}

func (s *Service) queryCandidates(ctx context.Context, req QueryRequest) (candidateResult, error) {
	if err := ctx.Err(); err != nil {
		return candidateResult{}, err
	}
	if strings.TrimSpace(req.Query) == "" {
		return candidateResult{}, errors.New("query must not be blank")
	}
	if !req.NoUpdate && s.Config.AutoUpdateBeforeQuery {
		if _, err := s.Index(ctx, false); err != nil {
			return candidateResult{}, fmt.Errorf("auto-update index: %w", err)
		}
	}
	retrievalMode := req.RetrievalMode
	if retrievalMode == "" {
		retrievalMode = search.RetrievalFull
	}
	result, err := s.searcher.SearchDetailed(ctx, search.SearchRequest{Query: req.Query, Paths: req.Paths, ChangedOnly: req.ChangedOnly, Changed: s.changedRanges(ctx, req.ChangedOnly), Limit: s.Config.MaxCandidates, Mode: retrievalMode})
	if err != nil {
		return candidateResult{}, err
	}
	return candidateResult{Plan: result.Plan, Revision: s.revision(ctx), Candidates: result.Candidates}, nil
}

func (s *Service) QueryEvidence(ctx context.Context, req EvidenceQueryRequest) (evidence.CompileResult, error) {
	known, err := evidence.NormalizeKnownHandles(req.KnownHandles)
	if err != nil {
		return evidence.CompileResult{}, err
	}
	if req.TokenBudget == 0 {
		req.TokenBudget = s.Config.DefaultTokenBudget
	}
	result, err := s.queryCandidates(ctx, QueryRequest{Query: req.Query, TokenBudget: req.TokenBudget, ChangedOnly: req.ChangedOnly, Paths: req.Paths, NoUpdate: req.NoUpdate, RetrievalMode: req.RetrievalMode})
	if err != nil {
		return evidence.CompileResult{}, err
	}
	return s.evidenceCompiler.Compile(evidence.CompileRequest{Plan: result.Plan, Revision: result.Revision, TokenBudget: req.TokenBudget, Mode: req.Mode, Candidates: result.Candidates, KnownHandles: known})
}

func (s *Service) ExpandEvidence(ctx context.Context, req EvidenceExpandRequest) (evidence.CompileResult, error) {
	if err := ctx.Err(); err != nil {
		return evidence.CompileResult{}, err
	}
	if len(req.Handles) == 0 {
		return evidence.CompileResult{}, errors.New("at least one handle is required")
	}
	known, err := evidence.NormalizeKnownHandles(req.KnownHandles)
	if err != nil {
		return evidence.CompileResult{}, err
	}
	if req.TokenBudget == 0 {
		req.TokenBudget = s.Config.DefaultTokenBudget
	}
	relation := req.Relation
	if relation == "" {
		relation = "self"
	}
	result, err := s.expandCandidates(ctx, req.Handles, relation, true)
	if err != nil {
		return evidence.CompileResult{}, err
	}
	return s.evidenceCompiler.Compile(evidence.CompileRequest{Plan: result.Plan, Revision: result.Revision, TokenBudget: req.TokenBudget, Mode: req.Mode, Candidates: result.Candidates, KnownHandles: known, ExpansionAnchors: req.Handles})
}

func (s *Service) ImpactEvidence(ctx context.Context, req EvidenceImpactRequest) (evidence.CompileResult, error) {
	known, err := evidence.NormalizeKnownHandles(req.KnownHandles)
	if err != nil {
		return evidence.CompileResult{}, err
	}
	if req.TokenBudget == 0 {
		req.TokenBudget = s.Config.DefaultTokenBudget
	}
	result, err := s.impactCandidates(ctx, req.BaseRef, req.HeadRef)
	if err != nil {
		return evidence.CompileResult{}, err
	}
	return s.evidenceCompiler.Compile(evidence.CompileRequest{Plan: result.Plan, Revision: result.Revision, TokenBudget: req.TokenBudget, Mode: req.Mode, Candidates: result.Candidates, KnownHandles: known})
}

func (s *Service) expandCandidates(ctx context.Context, handles []string, relation string, includeAnchors bool) (candidateResult, error) {
	if len(handles) == 0 {
		return candidateResult{}, errors.New("at least one handle is required")
	}
	hits := make([]model.RelationHit, 0)
	if includeAnchors && relation != "self" {
		anchors, err := s.Store.RelatedCandidateHits(ctx, handles, "self")
		if err != nil {
			return candidateResult{}, err
		}
		for index := range anchors {
			anchors[index].Candidate.Reasons = append(anchors[index].Candidate.Reasons, model.ScoreReason{Code: "symbol-exact"})
		}
		hits = append(hits, anchors...)
	}
	related, err := s.Store.RelatedCandidateHits(ctx, handles, relation)
	if err != nil {
		return candidateResult{}, err
	}
	hits = append(hits, related...)
	candidates := make([]model.RankedCandidate, 0, len(hits))
	seen := make(map[string]bool)
	for _, hit := range hits {
		candidate := hit.Candidate
		candidate.Relation = hit.Context.Kind
		contextCopy := hit.Context
		candidate.RelationContext = &contextCopy
		if includeAnchors && hit.Context.Kind == "self" {
			candidate.Relation = ""
			candidate.RelationContext = nil
			candidate.Reasons = append(candidate.Reasons, model.ScoreReason{Code: "symbol-exact"})
		}
		if !seen[candidate.Handle] {
			seen[candidate.Handle] = true
			candidates = append(candidates, candidate)
		}
	}
	plan := planForRelation(relation)
	return candidateResult{Plan: plan, Revision: s.revision(ctx), Candidates: candidates}, nil
}

func (s *Service) impactCandidates(ctx context.Context, base, head string) (candidateResult, error) {
	if err := ctx.Err(); err != nil {
		return candidateResult{}, err
	}
	plan := planForImpact()
	var changed []gitx.ChangedFile
	var err error
	if base != "" || head != "" {
		changed, err = gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{Base: base, Head: head})
	} else {
		unstaged, unstagedErr := gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{})
		staged, stagedErr := gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{Staged: true})
		if unstagedErr != nil && stagedErr != nil {
			if !repository.IsGitRepository(ctx, s.Root) {
				return candidateResult{Plan: plan, Revision: s.revision(ctx), Diagnostics: []string{"impact unavailable: repository is not a Git repository"}}, nil
			}
			return candidateResult{}, fmt.Errorf("read Git changes: %w", unstagedErr)
		}
		changed = append(unstaged, staged...)
	}
	all, err := s.Store.AllCandidates(ctx, s.Config.MaxCandidates)
	if err != nil {
		return candidateResult{}, err
	}
	selected := make([]model.RankedCandidate, 0)
	for _, candidate := range all {
		for _, file := range changed {
			if candidate.Path == file.Path && candidateOverlaps(candidate.StartLine, candidate.EndLine, file.Ranges) {
				candidate.Changed = true
				candidate.Score = 25
				selected = append(selected, candidate)
				break
			}
		}
	}
	return candidateResult{Plan: plan, Revision: s.revision(ctx), Candidates: rank.RankWithPlan(selected, plan)}, nil
}

func planForRelation(relation string) query.Plan {
	intent := query.IntentDefinition
	switch relation {
	case "callers":
		intent = query.IntentCallers
	case "callees":
		intent = query.IntentCallees
	case "tests":
		intent = query.IntentTests
	case "imports", "exports":
		intent = query.IntentImports
	case "references":
		intent = query.IntentReferences
	}
	return query.Plan{RawQuery: relation, Intents: []query.Intent{intent}, PrimaryIntent: intent, Relations: []string{relation}, Profile: string(intent)}
}

func planForImpact() query.Plan {
	return query.Plan{RawQuery: "Git impact", Intents: []query.Intent{query.IntentImpact}, PrimaryIntent: query.IntentImpact, Profile: "impact"}
}

func (s *Service) revision(ctx context.Context) string {
	revision, _ := s.Store.Meta(ctx, "last_revision")
	return revision
}

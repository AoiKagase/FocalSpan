package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/config"
	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/generic"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	luaextract "github.com/focalspan/focalspan/internal/extract/lua"
	pawnextract "github.com/focalspan/focalspan/internal/extract/pawn"
	"github.com/focalspan/focalspan/internal/extract/php"
	pythonstr "github.com/focalspan/focalspan/internal/extract/python"
	"github.com/focalspan/focalspan/internal/extract/resx"
	rubystructural "github.com/focalspan/focalspan/internal/extract/ruby"
	rustructural "github.com/focalspan/focalspan/internal/extract/rust"
	templateextract "github.com/focalspan/focalspan/internal/extract/template"
	"github.com/focalspan/focalspan/internal/extract/xaml"
	"github.com/focalspan/focalspan/internal/gitx"
	"github.com/focalspan/focalspan/internal/indexer"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
	"github.com/focalspan/focalspan/internal/rank"
	"github.com/focalspan/focalspan/internal/repository"
	"github.com/focalspan/focalspan/internal/search"
	"github.com/focalspan/focalspan/internal/store"
)

type Service struct {
	Root     string
	Config   config.Config
	Store    *store.Store
	indexer  *indexer.Indexer
	searcher *search.Searcher
	packer   *budget.Packer
}

func New(root string) (*Service, error) {
	cfg, _, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return NewWithConfig(root, cfg)
}

func NewWithConfig(root string, cfg config.Config) (*Service, error) {
	st, err := store.Open(root, cfg.IndexDirectory)
	if err != nil {
		return nil, err
	}
	registry := newExtractorRegistry()
	service := &Service{Root: root, Config: cfg, Store: st, packer: budget.NewPacker(budget.NewEstimator())}
	service.indexer = indexer.New(root, cfg, st, registry)
	service.searcher = search.New(st)
	return service, nil
}

func newExtractorRegistry() *extract.Registry {
	return extract.NewRegistry(
		goast.NewExtractor(),
		php.NewExtractor(),
		cpp.NewExtractor(),
		csharp.NewExtractor(),
		jsts.NewExtractor(),
		luaextract.NewExtractor(),
		pawnextract.NewExtractor(),
		rustructural.NewExtractor(),
		pythonstr.NewExtractor(),
		rubystructural.NewExtractor(),
		templateextract.NewExtractor(),
		xaml.NewExtractor(),
		resx.NewExtractor(),
		generic.NewExtractor(),
	)
}

func (s *Service) Close() error { return s.Store.Close() }

func (s *Service) Index(ctx context.Context, full bool) (model.IndexRun, error) {
	return s.IndexWithProgress(ctx, full, nil)
}

type IndexProgress = indexer.Progress
type IndexProgressFunc = indexer.ProgressFunc

const (
	IndexPhaseScanning = indexer.PhaseScanning
	IndexPhaseChecking = indexer.PhaseChecking
	IndexPhaseParsing  = indexer.PhaseParsing
	IndexPhaseWriting  = indexer.PhaseWriting
	IndexPhaseComplete = indexer.PhaseComplete
)

func (s *Service) IndexWithProgress(ctx context.Context, full bool, progress IndexProgressFunc) (model.IndexRun, error) {
	return s.indexer.RunWithProgress(ctx, full, progress)
}

type QueryRequest struct {
	Query         string
	TokenBudget   int
	Mode          string
	ChangedOnly   bool
	Paths         []string
	NoUpdate      bool
	RetrievalMode search.RetrievalMode
}

func (s *Service) Query(ctx context.Context, req QueryRequest) (model.ContextBundle, error) {
	if strings.TrimSpace(req.Query) == "" {
		return model.ContextBundle{}, errors.New("query must not be blank")
	}
	if req.TokenBudget == 0 {
		req.TokenBudget = s.Config.DefaultTokenBudget
	}
	if req.Mode == "" {
		req.Mode = "source"
	}
	if !req.NoUpdate && s.Config.AutoUpdateBeforeQuery {
		if _, err := s.Index(ctx, false); err != nil {
			return model.ContextBundle{}, fmt.Errorf("auto-update index: %w", err)
		}
	}
	changed := s.changedRanges(ctx, req.ChangedOnly)
	retrievalMode := req.RetrievalMode
	if retrievalMode == "" {
		retrievalMode = search.RetrievalFull
	}
	result, err := s.searcher.SearchDetailed(ctx, search.SearchRequest{Query: req.Query, Paths: req.Paths, ChangedOnly: req.ChangedOnly, Changed: changed, Limit: s.Config.MaxCandidates, Mode: retrievalMode})
	if err != nil {
		return model.ContextBundle{}, err
	}
	return s.packWithSavings(ctx, model.PackRequest{Query: req.Query, TokenBudget: req.TokenBudget, Mode: req.Mode, Candidates: result.Candidates, IntentHints: result.Plan.IntentStrings()})
}

func (s *Service) changedRanges(ctx context.Context, changedOnly bool) map[string][]search.LineRange {
	changed := map[string][]search.LineRange{}
	if !changedOnly {
		return changed
	}
	files, err := gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{})
	if err != nil {
		return changed
	}
	for _, file := range files {
		for _, r := range file.Ranges {
			changed[file.Path] = append(changed[file.Path], search.LineRange{Start: r.Start, End: r.End})
		}
	}
	return changed
}

func (s *Service) Status(ctx context.Context) (model.Status, error) {
	status, err := s.Store.Status(ctx, s.Root)
	if err != nil {
		return status, err
	}
	hash, err := s.Store.Meta(ctx, "configuration_hash")
	if err == nil {
		status.Stale = hash != "" && hash != s.Config.Hash()
	}
	return status, nil
}

func (s *Service) Health(ctx context.Context) (model.HealthStatus, error) {
	status, err := s.Status(ctx)
	report := model.HealthStatus{
		Status:             status,
		RepositoryDetected: repository.IsGitRepository(ctx, s.Root),
		ConfigValid:        true,
		DBOpen:             true,
		FTS5:               true,
		MCPReady:           true,
	}
	if info, statErr := os.Stat(s.Root); statErr == nil && info.IsDir() {
		report.PathPermissions = true
	}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, err.Error())
		return report, err
	}
	report.IndexFresh = status.LastRevision != "" && !status.Stale
	report.Ready = report.ConfigValid && report.DBOpen && report.FTS5 && report.PathPermissions && report.MCPReady && report.IndexFresh
	return report, nil
}

func (s *Service) Expand(ctx context.Context, handles []string, relation string, tokenBudget int) (model.ContextBundle, error) {
	if len(handles) == 0 {
		return model.ContextBundle{}, errors.New("at least one handle is required")
	}
	if tokenBudget == 0 {
		tokenBudget = s.Config.DefaultTokenBudget
	}
	candidates, err := s.Store.RelatedCandidates(ctx, handles, relation)
	if err != nil {
		return model.ContextBundle{}, err
	}
	return s.packWithSavings(ctx, model.PackRequest{Query: relation, TokenBudget: tokenBudget, Mode: "source", Candidates: candidates, IntentHints: []string{relation}})
}

func (s *Service) Impact(ctx context.Context, base, head string, tokenBudget int) (model.ContextBundle, error) {
	if tokenBudget == 0 {
		tokenBudget = s.Config.DefaultTokenBudget
	}
	var changed []gitx.ChangedFile
	var err error
	if base != "" || head != "" {
		changed, err = gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{Base: base, Head: head})
	} else {
		unstaged, unstagedErr := gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{})
		staged, stagedErr := gitx.NewClient(s.Root).Diff(ctx, gitx.DiffRequest{Staged: true})
		if unstagedErr != nil && stagedErr != nil {
			if !repository.IsGitRepository(ctx, s.Root) {
				bundle := s.pack(ctx, model.PackRequest{Query: "Git impact", TokenBudget: tokenBudget, Mode: "source"})
				bundle.Diagnostics = append(bundle.Diagnostics, "impact unavailable: repository is not a Git repository")
				return bundle, nil
			}
			return model.ContextBundle{}, fmt.Errorf("read Git changes: %w", unstagedErr)
		}
		changed = append(changed, unstaged...)
		changed = append(changed, staged...)
	}
	all, err := s.Store.AllCandidates(ctx, s.Config.MaxCandidates)
	if err != nil {
		return model.ContextBundle{}, err
	}
	selected := make([]model.RankedCandidate, 0)
	for _, candidate := range all {
		for _, file := range changed {
			if candidate.Path != file.Path || !candidateOverlaps(candidate.StartLine, candidate.EndLine, file.Ranges) {
				continue
			}
			candidate.Changed = true
			candidate.Score = 25
			selected = append(selected, candidate)
			break
		}
	}
	plan := query.Plan{RawQuery: "Git impact", Intents: []query.Intent{query.IntentImpact}, PrimaryIntent: query.IntentImpact, Profile: "impact"}
	selected = rank.RankWithPlan(selected, plan)
	bundle, err := s.packWithSavings(ctx, model.PackRequest{Query: "Git impact", TokenBudget: tokenBudget, Mode: "source", Candidates: selected})
	if err != nil {
		return model.ContextBundle{}, err
	}
	bundle.Diagnostics = append(bundle.Diagnostics, "impact analysis is syntax-only; unresolved calls may be omitted")
	return bundle, nil
}

func (s *Service) pack(ctx context.Context, req model.PackRequest) model.ContextBundle {
	if revision, err := s.Store.Meta(ctx, "last_revision"); err == nil {
		req.IndexRevision = revision
	}
	return s.packer.Pack(req)
}

func (s *Service) packWithSavings(ctx context.Context, req model.PackRequest) (model.ContextBundle, error) {
	bundle := s.pack(ctx, req)
	baseline, err := s.baselineTokensForCandidates(ctx, req.Candidates)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return model.ContextBundle{}, ctxErr
		}
		bundle.Diagnostics = append(bundle.Diagnostics, "token savings unavailable: "+err.Error())
		return bundle, nil
	}
	if baseline <= 0 {
		return bundle, nil
	}
	saved := baseline - bundle.EstimatedTokens
	bundle.Savings = &model.TokenSavings{
		BaselineTokens: baseline,
		SavedTokens:    saved,
		SavingsRatio:   float64(saved) / float64(baseline),
	}
	return bundle, nil
}

func (s *Service) baselineTokensForCandidates(ctx context.Context, candidates []model.RankedCandidate) (int, error) {
	estimator := budget.NewEstimator()
	seen := make(map[string]bool)
	total := 0
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if seen[candidate.Path] {
			continue
		}
		seen[candidate.Path] = true
		full, err := repository.ContainedPath(s.Root, filepath.Join(s.Root, filepath.FromSlash(candidate.Path)))
		if err != nil {
			return 0, fmt.Errorf("resolve candidate %q: %w", candidate.Path, err)
		}
		content, err := os.ReadFile(full)
		if err != nil {
			return 0, fmt.Errorf("read candidate %q: %w", candidate.Path, err)
		}
		total += estimator.Estimate(string(content))
	}
	return total, nil
}

func (s *Service) BaselineTokens(ctx context.Context, rawQuery string) (int, error) {
	return s.BaselineTokensForRequest(ctx, QueryRequest{Query: rawQuery})
}

func (s *Service) BaselineTokensForRequest(ctx context.Context, req QueryRequest) (int, error) {
	retrievalMode := req.RetrievalMode
	if retrievalMode == "" {
		retrievalMode = search.RetrievalFull
	}
	candidates, err := s.searcher.Search(ctx, search.SearchRequest{Query: req.Query, Paths: req.Paths, ChangedOnly: req.ChangedOnly, Changed: s.changedRanges(ctx, req.ChangedOnly), Limit: s.Config.MaxCandidates, Mode: retrievalMode})
	if err != nil {
		return 0, err
	}
	estimator := budget.NewEstimator()
	seen := make(map[string]bool)
	total := 0
	for _, candidate := range candidates {
		if seen[candidate.Path] {
			continue
		}
		seen[candidate.Path] = true
		full, err := repository.ContainedPath(s.Root, filepath.Join(s.Root, filepath.FromSlash(candidate.Path)))
		if err != nil {
			continue
		}
		content, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		total += estimator.Estimate(string(content))
	}
	return total, nil
}

func candidateOverlaps(start, end int, ranges []gitx.LineRange) bool {
	for _, r := range ranges {
		if start <= r.End && end >= r.Start {
			return true
		}
	}
	return false
}

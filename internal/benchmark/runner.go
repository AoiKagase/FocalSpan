package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/evidence"
	"github.com/focalspan/focalspan/internal/model"
)

type RunRequest struct {
	Suite        Suite
	Repositories map[string]string
	Profiles     []Profile
	Repeat       int
	Workspace    string
}

type Runner struct {
	Snapshotter   Snapshotter
	EngineFactory EngineFactory
	GitRunner     CommandRunner
}

type CaseRun struct {
	CaseID        string
	Profile       string
	Budget        int
	Intent        string
	Deterministic bool
	Packet        *evidence.Packet
	Legacy        *model.ContextBundle
	Changes       ChangeSet
	Index         IndexMeasurement
}

type RunReport struct {
	Schema          string              `json:"schema"`
	Suite           string              `json:"suite"`
	FocalSpanCommit string              `json:"focalspan_commit"`
	Quality         []QualityResult     `json:"quality"`
	Aggregate       AggregateQuality    `json:"aggregate"`
	Performance     []PerformanceResult `json:"performance,omitempty"`
	Runs            []CaseRun           `json:"-"`
}

const ReportSchemaV1 = "focalspan.benchmark-report.v1"

func (runner *Runner) Run(ctx context.Context, request RunRequest) (RunReport, error) {
	if request.Repeat <= 0 {
		request.Repeat = 2
	}
	if runner.Snapshotter == nil || runner.EngineFactory == nil {
		return RunReport{}, fmt.Errorf("runner dependencies are required")
	}
	report := RunReport{Schema: ReportSchemaV1, Suite: request.Suite.Name}
	for _, benchmarkCase := range request.Suite.Cases {
		repositoryPath, exists := request.Repositories[benchmarkCase.Repository]
		if !exists {
			return RunReport{}, fmt.Errorf("case %q: repository %q is not resolved", benchmarkCase.ID, benchmarkCase.Repository)
		}
		caseID, err := safeWorkspaceID(benchmarkCase.ID)
		if err != nil {
			return RunReport{}, err
		}
		repositoryID, err := safeWorkspaceID(benchmarkCase.Repository)
		if err != nil {
			return RunReport{}, err
		}
		destination := filepath.Join(request.Workspace, "snapshots", repositoryID, caseID)
		snapshotStarted := time.Now()
		snapshot, err := runner.Snapshotter.Materialize(ctx, benchmarkCase.Repository, repositoryPath, benchmarkCase.BaseRef, destination)
		snapshotDuration := time.Since(snapshotStarted)
		if err != nil {
			return RunReport{}, fmt.Errorf("case %q snapshot: %w", benchmarkCase.ID, err)
		}
		if err := validateLabelsAtBase(snapshot.Root, benchmarkCase); err != nil {
			return RunReport{}, fmt.Errorf("case %q: %w", benchmarkCase.ID, err)
		}
		var changes ChangeSet
		if runner.GitRunner != nil {
			changes, err = CollectChanges(ctx, runner.GitRunner, repositoryPath, benchmarkCase.BaseRef, benchmarkCase.TargetRef)
			if err != nil {
				return RunReport{}, err
			}
		}
		for _, profile := range request.Profiles {
			engine, openErr := runner.EngineFactory.Open(snapshot.Root, profile.RetrievalMode)
			if openErr != nil {
				return RunReport{}, openErr
			}
			measurement, buildErr := engine.Build(ctx)
			if buildErr != nil {
				_ = engine.Close()
				return RunReport{}, buildErr
			}
			for _, budget := range profile.Budgets {
				run := CaseRun{CaseID: benchmarkCase.ID, Profile: profile.Name, Budget: budget, Changes: changes, Index: measurement, Deterministic: true}
				var canonical []byte
				queryDurations := make([]int64, 0, request.Repeat)
				for repeat := 0; repeat < request.Repeat; repeat++ {
					queryStarted := time.Now()
					if profile.Contract == "legacy" {
						bundle, queryErr := engine.QueryLegacy(ctx, app.QueryRequest{Query: benchmarkCase.Query, TokenBudget: budget, Mode: string(profile.EvidenceMode), NoUpdate: true})
						if queryErr != nil {
							_ = engine.Close()
							return RunReport{}, queryErr
						}
						run.Legacy = &bundle
						encoded, _ := json.Marshal(bundle)
						if repeat > 0 && string(encoded) != string(canonical) {
							run.Deterministic = false
						}
						canonical = encoded
					} else {
						packet, queryErr := engine.QueryEvidence(ctx, app.EvidenceQueryRequest{Query: benchmarkCase.Query, TokenBudget: budget, Mode: profile.EvidenceMode, NoUpdate: true})
						if queryErr != nil {
							_ = engine.Close()
							return RunReport{}, queryErr
						}
						run.Packet = &packet
						run.Intent = packet.Intent
						encoded, _ := json.Marshal(packet)
						if repeat > 0 && string(encoded) != string(canonical) {
							run.Deterministic = false
						}
						canonical = encoded
					}
					queryDurations = append(queryDurations, time.Since(queryStarted).Milliseconds())
				}
				report.Runs = append(report.Runs, run)
				if run.Packet != nil {
					changedPaths := make([]string, 0, len(changes.Files))
					for _, changed := range changes.Files {
						if changed.Status != "add" {
							changedPaths = append(changedPaths, changed.NewPath)
						}
					}
					quality := MeasurePacket(benchmarkCase, profile.Name, budget, *run.Packet, run.Deterministic, changedPaths)
					if profile.RunExpansion {
						for _, expectation := range benchmarkCase.Expand {
							anchor, anchorErr := FindExpansionAnchor(*run.Packet, expectation.From)
							if anchorErr != nil {
								quality.FailureCodes = append(quality.FailureCodes, "expansion_anchor_missing")
								continue
							}
							knownHandles := make([]string, 0, len(run.Packet.Evidence))
							for _, item := range run.Packet.Evidence {
								knownHandles = append(knownHandles, item.Handle)
							}
							knownPacket, expandErr := engine.ExpandEvidence(ctx, app.EvidenceExpandRequest{Handles: []string{anchor}, Relation: expectation.Relation, TokenBudget: expectation.Budget, Mode: profile.EvidenceMode, KnownHandles: knownHandles})
							if expandErr != nil {
								_ = engine.Close()
								return RunReport{}, expandErr
							}
							controlPacket, controlErr := engine.ExpandEvidence(ctx, app.EvidenceExpandRequest{Handles: []string{anchor}, Relation: expectation.Relation, TokenBudget: expectation.Budget, Mode: profile.EvidenceMode})
							if controlErr != nil {
								_ = engine.Close()
								return RunReport{}, controlErr
							}
							metrics := MeasureExpansion(*run.Packet, knownPacket, controlPacket, expectation)
							quality.ExpandRequiredPathRecall = metrics.RequiredPathRecall
							quality.ExpandRequiredSymbolRecall = metrics.RequiredSymbolRecall
							quality.ExpandForbiddenViolations += metrics.ForbiddenViolations
							quality.ExpandRelationValid = metrics.RelationValid
							quality.CumulativeWireTokens = metrics.CumulativeWireTokens
							quality.CumulativeWireTokensWithoutKnown = metrics.CumulativeWireTokensWithoutKnown
							quality.DeltaTokenRatio = metrics.DeltaTokenRatio
							quality.KnownResendCount += metrics.KnownResendCount
						}
					}
					explicitCodes := append([]string(nil), quality.FailureCodes...)
					quality.FailureCodes = FailureCodes(quality)
					for _, code := range explicitCodes {
						if !containsString(quality.FailureCodes, code) {
							quality.FailureCodes = append(quality.FailureCodes, code)
						}
					}
					report.Quality = append(report.Quality, quality)
				} else {
					report.Quality = append(report.Quality, QualityResult{CaseID: benchmarkCase.ID, Profile: profile.Name, Budget: budget, BudgetCompliant: 1, Deterministic: boolInt(run.Deterministic), RelationValid: 1})
				}
				report.Performance = append(report.Performance, PerformanceResult{CaseID: benchmarkCase.ID, Profile: profile.Name, Budget: budget, SnapshotMS: snapshotDuration.Milliseconds(), IndexMS: measurement.Duration.Milliseconds(), QueryMS: queryDurations, DatabaseBytes: measurement.DatabaseBytes, Files: measurement.Files, Symbols: measurement.Symbols, Chunks: measurement.Chunks, Relations: measurement.Relations})
			}
			if closeErr := engine.Close(); closeErr != nil {
				return RunReport{}, closeErr
			}
		}
	}
	report.Aggregate = AggregateResults(report.Quality)
	return report, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

var workspaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func safeWorkspaceID(value string) (string, error) {
	if !workspaceIDPattern.MatchString(value) {
		return "", fmt.Errorf("unsafe workspace id %q", value)
	}
	return value, nil
}

func validateLabelsAtBase(root string, benchmarkCase Case) error {
	paths := append([]string{}, benchmarkCase.RequiredPaths...)
	paths = append(paths, benchmarkCase.OptionalPaths...)
	paths = append(paths, benchmarkCase.ForbiddenPaths...)
	for _, symbol := range benchmarkCase.RequiredSymbols {
		paths = append(paths, symbol.Path)
	}
	for _, expansion := range benchmarkCase.Expand {
		paths = append(paths, expansion.From.Path)
		paths = append(paths, expansion.RequiredPaths...)
		paths = append(paths, expansion.ForbiddenPaths...)
		for _, symbol := range expansion.RequiredSymbols {
			paths = append(paths, symbol.Path)
		}
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			return fmt.Errorf("label path %q is unavailable at base", path)
		}
	}
	return nil
}

func ValidateLabelsAtBase(root string, benchmarkCase Case) error {
	return validateLabelsAtBase(root, benchmarkCase)
}

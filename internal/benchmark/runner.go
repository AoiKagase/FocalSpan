package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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
		snapshot, err := runner.Snapshotter.Materialize(ctx, benchmarkCase.Repository, repositoryPath, benchmarkCase.BaseRef, destination)
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
				for repeat := 0; repeat < request.Repeat; repeat++ {
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
				}
				report.Runs = append(report.Runs, run)
				if run.Packet != nil {
					changedPaths := make([]string, 0, len(changes.Files))
					for _, changed := range changes.Files {
						if changed.Status != "add" {
							changedPaths = append(changedPaths, changed.NewPath)
						}
					}
					report.Quality = append(report.Quality, MeasurePacket(benchmarkCase, profile.Name, budget, *run.Packet, run.Deterministic, changedPaths))
				} else {
					report.Quality = append(report.Quality, QualityResult{CaseID: benchmarkCase.ID, Profile: profile.Name, Budget: budget, BudgetCompliant: 1, Deterministic: boolInt(run.Deterministic), RelationValid: 1})
				}
				report.Performance = append(report.Performance, PerformanceResult{CaseID: benchmarkCase.ID, Profile: profile.Name, Budget: budget, IndexMS: measurement.Duration.Milliseconds(), DatabaseBytes: measurement.DatabaseBytes, Files: measurement.Files, Symbols: measurement.Symbols, Chunks: measurement.Chunks, Relations: measurement.Relations})
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
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
			return fmt.Errorf("label path %q is unavailable at base", path)
		}
	}
	return nil
}

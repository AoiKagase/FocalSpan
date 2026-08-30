package evalcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/eval"
	"github.com/focalspan/focalspan/internal/repository"
	"github.com/focalspan/focalspan/internal/search"
)

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if err := run(ctx, args, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "focalspan-eval: %v\n", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("focalspan-eval", flag.ContinueOnError)
	fs.SetOutput(stdout)
	rootArg := fs.String("root", ".", "repository root")
	casesPath := fs.String("cases", "testdata/eval/cases.jsonl", "evaluation cases")
	ablation := fs.String("ablation", "full", "full, fts-only, no-relations, or all")
	contract := fs.String("contract", "legacy", "legacy, evidence, or compare")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *contract != "legacy" && *contract != "evidence" && *contract != "compare" {
		return fmt.Errorf("unknown contract %q", *contract)
	}
	if *contract != "legacy" && *ablation != "full" {
		return fmt.Errorf("--ablation is available only with --contract legacy")
	}
	modes, err := parseAblation(*ablation)
	if err != nil {
		return err
	}
	root, err := resolveRoot(ctx, *rootArg)
	if err != nil {
		return err
	}
	service, err := app.New(root)
	if err != nil {
		return err
	}
	defer service.Close()
	if *contract != "legacy" {
		cases, err := eval.LoadEvidenceCases(*casesPath)
		if err != nil {
			return err
		}
		report, err := eval.EvaluateEvidence(ctx, service, cases, *contract == "compare")
		if err != nil {
			return err
		}
		if *jsonOutput {
			encoder := json.NewEncoder(stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		}
		return writeEvidenceReport(stdout, *contract, report)
	}
	cases, err := eval.LoadCases(*casesPath)
	if err != nil {
		return err
	}
	reports := make(map[string]eval.Report, len(modes))
	for _, mode := range modes {
		report, err := eval.EvaluateMode(ctx, service, cases, mode)
		if err != nil {
			return err
		}
		reports[string(mode)] = report
	}
	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if *ablation == "all" {
			return encoder.Encode(struct {
				Reports map[string]eval.Report `json:"reports"`
			}{Reports: reports})
		}
		return encoder.Encode(reports[string(modes[0])])
	}
	for _, mode := range modes {
		if err := writeReport(stdout, mode, reports[string(mode)]); err != nil {
			return err
		}
	}
	return nil
}

func writeEvidenceReport(w io.Writer, contract string, report eval.EvidenceReport) error {
	_, err := fmt.Fprintf(w, "contract: %s\ncases: %d\ncoverage: %.2f\nrole accuracy: %.2f\nwire budget compliance: %.2f\ndeterministic: %.2f\nmetadata overhead median: %.4f\ndelta token ratio median: %.4f\n", contract, len(report.Cases), report.ExpectedCoverage, report.RoleAccuracy, report.WireBudgetCompliance, report.DeterministicOutput, report.MedianMetadataOverheadRatio, report.MedianDeltaTokenRatio)
	return err
}

func resolveRoot(ctx context.Context, start string) (string, error) {
	if start == "" || start == "." {
		root, _, err := repository.DetectRoot(ctx, start)
		return root, err
	}
	return filepath.Abs(start)
}

func parseAblation(value string) ([]search.RetrievalMode, error) {
	if value == "all" {
		return []search.RetrievalMode{search.RetrievalFull, search.RetrievalFTSOnly, search.RetrievalNoRelations}, nil
	}
	for _, mode := range []search.RetrievalMode{search.RetrievalFull, search.RetrievalFTSOnly, search.RetrievalNoRelations} {
		if value == string(mode) {
			return []search.RetrievalMode{mode}, nil
		}
	}
	return nil, fmt.Errorf("unknown ablation %q", value)
}

func writeReport(w io.Writer, mode search.RetrievalMode, report eval.Report) error {
	_, err := fmt.Fprintf(w, "mode: %s\ncases: %d\nhit@5: %.2f\nbudget compliance: %.2f\nreduction ratio: %.2f\ndeterministic: %.2f\n", mode, len(report.Cases), report.HitAt5, report.BudgetCompliance, report.MedianReductionRatio, report.DeterministicOutput)
	return err
}

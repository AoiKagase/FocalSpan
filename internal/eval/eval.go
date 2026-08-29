package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/focalspan/focalspan/internal/app"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
	"github.com/focalspan/focalspan/internal/search"
)

type Case struct {
	Name              string   `json:"name"`
	Query             string   `json:"query"`
	TokenBudget       int      `json:"token_budget"`
	ExpectedSymbols   []string `json:"expected_symbols"`
	ExpectedPaths     []string `json:"expected_paths"`
	ForbiddenPaths    []string `json:"forbidden_paths"`
	ExpectedIntents   []string `json:"expected_intents,omitempty"`
	ExpectedRelations []string `json:"expected_relations,omitempty"`
	ExpectedKinds     []string `json:"expected_kinds,omitempty"`
}

type Queryer interface {
	Query(context.Context, app.QueryRequest) (model.ContextBundle, error)
	BaselineTokensForRequest(context.Context, app.QueryRequest) (int, error)
}

type CaseResult struct {
	Name                string  `json:"name"`
	HitAt1              int     `json:"hit_at_1"`
	HitAt3              int     `json:"hit_at_3"`
	HitAt5              int     `json:"hit_at_5"`
	SymbolRecall        float64 `json:"symbol_recall"`
	PathRecall          float64 `json:"path_recall"`
	ForbiddenViolations int     `json:"forbidden_path_violations"`
	BudgetCompliant     int     `json:"budget_compliant"`
	EstimatedTokens     int     `json:"estimated_tokens"`
	ReductionRatio      float64 `json:"reduction_ratio"`
	Deterministic       int     `json:"deterministic"`
	IntentRecall        float64 `json:"intent_recall,omitempty"`
	RelationRecall      float64 `json:"relation_recall,omitempty"`
	KindRecall          float64 `json:"kind_recall,omitempty"`
	RetrievalMode       string  `json:"retrieval_mode"`
}

type IntentReport struct {
	Cases          int     `json:"cases"`
	IntentRecall   float64 `json:"intent_recall"`
	RelationRecall float64 `json:"relation_recall"`
	KindRecall     float64 `json:"kind_recall"`
}

type Report struct {
	Cases                   []CaseResult            `json:"cases"`
	HitAt1                  float64                 `json:"hit_at_1"`
	HitAt3                  float64                 `json:"hit_at_3"`
	HitAt5                  float64                 `json:"hit_at_5"`
	SymbolRecall            float64                 `json:"symbol_recall"`
	PathRecall              float64                 `json:"path_recall"`
	BudgetCompliance        float64                 `json:"budget_compliance"`
	ForbiddenPathViolations int                     `json:"forbidden_path_violations"`
	MedianEstimatedTokens   int                     `json:"median_estimated_tokens"`
	MedianReductionRatio    float64                 `json:"median_reduction_ratio"`
	DeterministicOutput     float64                 `json:"deterministic_output"`
	IntentRecall            float64                 `json:"intent_recall,omitempty"`
	RelationRecall          float64                 `json:"relation_recall,omitempty"`
	KindRecall              float64                 `json:"kind_recall,omitempty"`
	ByPrimaryIntent         map[string]IntentReport `json:"by_primary_intent,omitempty"`
}

func LoadCases(path string) ([]Case, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read evaluation cases: %w", err)
	}
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "[") {
		var cases []Case
		if err := json.Unmarshal([]byte(trimmed), &cases); err != nil {
			return nil, fmt.Errorf("parse evaluation JSON: %w", err)
		}
		return cases, nil
	}
	scanner := bufio.NewScanner(strings.NewReader(trimmed))
	cases := make([]Case, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var item Case
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("parse evaluation JSONL: %w", err)
		}
		cases = append(cases, item)
	}
	return cases, scanner.Err()
}

func Evaluate(ctx context.Context, queryer Queryer, cases []Case) (Report, error) {
	return EvaluateMode(ctx, queryer, cases, search.RetrievalFull)
}

func EvaluateMode(ctx context.Context, queryer Queryer, cases []Case, mode search.RetrievalMode) (Report, error) {
	if mode == "" {
		mode = search.RetrievalFull
	}
	if mode != search.RetrievalFull && mode != search.RetrievalFTSOnly && mode != search.RetrievalNoRelations {
		return Report{}, fmt.Errorf("unknown retrieval mode %q", mode)
	}
	report := Report{Cases: make([]CaseResult, 0, len(cases))}
	for _, item := range cases {
		request := app.QueryRequest{Query: item.Query, TokenBudget: item.TokenBudget, Mode: "source", RetrievalMode: mode}
		first, err := queryer.Query(ctx, request)
		if err != nil {
			return report, fmt.Errorf("evaluate %s: %w", item.Name, err)
		}
		second, err := queryer.Query(ctx, request)
		if err != nil {
			return report, fmt.Errorf("repeat evaluate %s: %w", item.Name, err)
		}
		firstJSON, _ := json.Marshal(first)
		secondJSON, _ := json.Marshal(second)
		result := CaseResult{Name: item.Name, EstimatedTokens: first.EstimatedTokens, RetrievalMode: string(mode)}
		if string(firstJSON) == string(secondJSON) {
			result.Deterministic = 1
		}
		if item.TokenBudget <= 0 || first.EstimatedTokens <= item.TokenBudget {
			result.BudgetCompliant = 1
		}
		result.HitAt1 = hitAt(first.Items, item.ExpectedSymbols, item.ExpectedPaths, 1)
		result.HitAt3 = hitAt(first.Items, item.ExpectedSymbols, item.ExpectedPaths, 3)
		result.HitAt5 = hitAt(first.Items, item.ExpectedSymbols, item.ExpectedPaths, 5)
		result.SymbolRecall = recallSymbols(first.Items, item.ExpectedSymbols)
		result.PathRecall = recallPaths(first.Items, item.ExpectedPaths)
		result.ForbiddenViolations = forbidden(first.Items, item.ForbiddenPaths)
		plan := query.PlanQuery(item.Query)
		result.IntentRecall = recallStrings(plan.IntentStrings(), item.ExpectedIntents)
		result.RelationRecall = recallStrings(itemRelations(first.Items), item.ExpectedRelations)
		result.KindRecall = recallStrings(itemKinds(first.Items), item.ExpectedKinds)
		baseline, err := queryer.BaselineTokensForRequest(ctx, request)
		if err != nil {
			return report, err
		}
		if baseline > 0 {
			result.ReductionRatio = float64(first.EstimatedTokens) / float64(baseline)
		}
		report.Cases = append(report.Cases, result)
	}
	if len(report.Cases) > 0 {
		for _, item := range report.Cases {
			report.HitAt1 += float64(item.HitAt1)
			report.HitAt3 += float64(item.HitAt3)
			report.HitAt5 += float64(item.HitAt5)
			report.SymbolRecall += item.SymbolRecall
			report.PathRecall += item.PathRecall
			report.IntentRecall += item.IntentRecall
			report.RelationRecall += item.RelationRecall
			report.KindRecall += item.KindRecall
			report.BudgetCompliance += float64(item.BudgetCompliant)
			report.DeterministicOutput += float64(item.Deterministic)
			report.ForbiddenPathViolations += item.ForbiddenViolations
		}
		count := float64(len(report.Cases))
		report.HitAt1 /= count
		report.HitAt3 /= count
		report.HitAt5 /= count
		report.SymbolRecall /= count
		report.PathRecall /= count
		report.IntentRecall /= count
		report.RelationRecall /= count
		report.KindRecall /= count
		report.BudgetCompliance /= count
		report.DeterministicOutput /= count
		report.MedianEstimatedTokens = medianInts(report.Cases, func(item CaseResult) int { return item.EstimatedTokens })
		report.MedianReductionRatio = medianFloats(report.Cases, func(item CaseResult) float64 { return item.ReductionRatio })
		report.ByPrimaryIntent = make(map[string]IntentReport)
		for index, item := range cases {
			plan := query.PlanQuery(item.Query)
			key := string(plan.PrimaryIntent)
			intentReport := report.ByPrimaryIntent[key]
			intentReport.Cases++
			caseResult := report.Cases[index]
			intentReport.IntentRecall += caseResult.IntentRecall
			intentReport.RelationRecall += caseResult.RelationRecall
			intentReport.KindRecall += caseResult.KindRecall
			report.ByPrimaryIntent[key] = intentReport
		}
		for key, intentReport := range report.ByPrimaryIntent {
			count := float64(intentReport.Cases)
			intentReport.IntentRecall /= count
			intentReport.RelationRecall /= count
			intentReport.KindRecall /= count
			report.ByPrimaryIntent[key] = intentReport
		}
	}
	return report, nil
}

func recallStrings(found, expected []string) float64 {
	if len(expected) == 0 {
		return 1
	}
	seen := make(map[string]bool, len(found))
	for _, value := range found {
		seen[strings.ToLower(strings.TrimSpace(value))] = true
	}
	foundCount := 0
	for _, value := range expected {
		if seen[strings.ToLower(strings.TrimSpace(value))] {
			foundCount++
		}
	}
	return float64(foundCount) / float64(len(expected))
}

func itemRelations(items []model.ContextItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item.Relation != "" {
			result = append(result, item.Relation)
		}
	}
	return result
}

func itemKinds(items []model.ContextItem) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind != "" {
			result = append(result, item.Kind)
		}
	}
	return result
}

func hitAt(items []model.ContextItem, expectedSymbols, expectedPaths []string, n int) int {
	if len(items) > n {
		items = items[:n]
	}
	if len(expectedSymbols) > 0 {
		for _, item := range items {
			for _, symbol := range expectedSymbols {
				if item.Symbol == symbol {
					return 1
				}
			}
		}
		return 0
	}
	for _, item := range items {
		for _, path := range expectedPaths {
			if item.Path == path {
				return 1
			}
		}
	}
	return 0
}

func recallSymbols(items []model.ContextItem, expected []string) float64 {
	if len(expected) == 0 {
		return 1
	}
	found := 0
	for _, symbol := range expected {
		for _, item := range items {
			if item.Symbol == symbol {
				found++
				break
			}
		}
	}
	return float64(found) / float64(len(expected))
}

func recallPaths(items []model.ContextItem, expected []string) float64 {
	if len(expected) == 0 {
		return 1
	}
	found := 0
	for _, path := range expected {
		for _, item := range items {
			if item.Path == path {
				found++
				break
			}
		}
	}
	return float64(found) / float64(len(expected))
}

func forbidden(items []model.ContextItem, forbiddenPaths []string) int {
	violations := 0
	for _, item := range items {
		for _, path := range forbiddenPaths {
			if item.Path == path {
				violations++
			}
		}
	}
	return violations
}

func medianInts(items []CaseResult, get func(CaseResult) int) int {
	values := make([]int, 0, len(items))
	for _, item := range items {
		values = append(values, get(item))
	}
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
	return values[len(values)/2]
}

func medianFloats(items []CaseResult, get func(CaseResult) float64) float64 {
	values := make([]float64, 0, len(items))
	for _, item := range items {
		values = append(values, get(item))
	}
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
	return values[len(values)/2]
}

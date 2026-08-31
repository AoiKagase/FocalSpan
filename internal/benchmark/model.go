package benchmark

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

const SuiteSchemaV1 = "focalspan.benchmark.v1"

type Suite struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Cases       []Case `json:"cases"`
}

type Case struct {
	ID              string              `json:"id"`
	Repository      string              `json:"repository"`
	BaseRef         string              `json:"base_ref"`
	TargetRef       string              `json:"target_ref"`
	Query           string              `json:"query"`
	Budgets         []int               `json:"budgets"`
	ExpectedIntent  string              `json:"expected_intent,omitempty"`
	RequiredPaths   []string            `json:"required_paths,omitempty"`
	OptionalPaths   []string            `json:"optional_paths,omitempty"`
	ForbiddenPaths  []string            `json:"forbidden_paths,omitempty"`
	RequiredSymbols []SymbolExpectation `json:"required_symbols,omitempty"`
	Expand          []ExpandExpectation `json:"expand,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
}

type SymbolExpectation struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
	Role string `json:"role,omitempty"`
}

type ExpandExpectation struct {
	Relation        string              `json:"relation"`
	From            SymbolExpectation   `json:"from"`
	Budget          int                 `json:"budget"`
	RequiredPaths   []string            `json:"required_paths,omitempty"`
	RequiredSymbols []SymbolExpectation `json:"required_symbols,omitempty"`
	ForbiddenPaths  []string            `json:"forbidden_paths,omitempty"`
}

type RepositoryRegistry struct {
	Schema       string            `json:"schema"`
	Repositories map[string]string `json:"repositories"`
}

func LoadSuite(filename string) (Suite, error) {
	var suite Suite
	if err := decodeJSONFile(filename, &suite); err != nil {
		return Suite{}, fmt.Errorf("load benchmark suite: %w", err)
	}
	suite = NormalizeSuite(suite)
	if err := ValidateSuite(suite); err != nil {
		return Suite{}, err
	}
	return suite, nil
}

func LoadRegistry(filename string) (RepositoryRegistry, error) {
	var registry RepositoryRegistry
	if err := decodeJSONFile(filename, &registry); err != nil {
		return RepositoryRegistry{}, fmt.Errorf("load repository registry: %w", err)
	}
	registry.Schema = strings.TrimSpace(registry.Schema)
	if registry.Schema != SuiteSchemaV1 {
		return RepositoryRegistry{}, fmt.Errorf("repository registry schema must be %q", SuiteSchemaV1)
	}
	if registry.Repositories == nil {
		return RepositoryRegistry{}, errors.New("repository registry has no repositories")
	}
	return registry, nil
}

func decodeJSONFile(filename string, target any) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func NormalizeSuite(suite Suite) Suite {
	suite.Schema = strings.TrimSpace(suite.Schema)
	suite.Name = strings.TrimSpace(suite.Name)
	suite.Description = strings.TrimSpace(suite.Description)
	for caseIndex := range suite.Cases {
		caseValue := &suite.Cases[caseIndex]
		caseValue.ID = strings.TrimSpace(caseValue.ID)
		caseValue.Repository = strings.TrimSpace(caseValue.Repository)
		caseValue.BaseRef = strings.TrimSpace(caseValue.BaseRef)
		caseValue.TargetRef = strings.TrimSpace(caseValue.TargetRef)
		caseValue.Query = strings.TrimSpace(caseValue.Query)
		caseValue.ExpectedIntent = strings.TrimSpace(caseValue.ExpectedIntent)
		caseValue.Tags = sortedUniqueTrimmed(caseValue.Tags)
		normalizeSymbols(caseValue.RequiredSymbols)
		for expandIndex := range caseValue.Expand {
			expand := &caseValue.Expand[expandIndex]
			expand.Relation = strings.TrimSpace(expand.Relation)
			normalizeSymbol(&expand.From)
			normalizeSymbols(expand.RequiredSymbols)
		}
	}
	return suite
}

func normalizeSymbols(symbols []SymbolExpectation) {
	for index := range symbols {
		normalizeSymbol(&symbols[index])
	}
}

func normalizeSymbol(symbol *SymbolExpectation) {
	symbol.Path = strings.TrimSpace(symbol.Path)
	symbol.Name = strings.TrimSpace(symbol.Name)
	symbol.Kind = strings.TrimSpace(symbol.Kind)
	symbol.Role = strings.TrimSpace(symbol.Role)
}

func sortedUniqueTrimmed(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func ValidateSuite(suite Suite) error {
	var problems []string
	if suite.Schema != SuiteSchemaV1 {
		problems = append(problems, fmt.Sprintf("schema must be %q", SuiteSchemaV1))
	}
	if suite.Name == "" {
		problems = append(problems, "name must not be empty")
	}
	if len(suite.Cases) == 0 {
		problems = append(problems, "cases must not be empty")
	}
	seenIDs := map[string]int{}
	for index, caseValue := range suite.Cases {
		label := fmt.Sprintf("case[%d]", index)
		if caseValue.ID == "" {
			problems = append(problems, label+": id must not be empty")
		} else if first, exists := seenIDs[caseValue.ID]; exists {
			problems = append(problems, fmt.Sprintf("%s: duplicate case id %q (first at case[%d])", label, caseValue.ID, first))
		} else {
			seenIDs[caseValue.ID] = index
			label = fmt.Sprintf("case %q", caseValue.ID)
		}
		problems = append(problems, validateCase(label, caseValue)...)
	}
	if len(problems) != 0 {
		return errors.New("benchmark suite validation failed: " + strings.Join(problems, "; "))
	}
	return nil
}

func validateCase(label string, caseValue Case) []string {
	var problems []string
	for _, value := range []struct{ name, value string }{{"repository", caseValue.Repository}, {"base_ref", caseValue.BaseRef}, {"target_ref", caseValue.TargetRef}, {"query", caseValue.Query}} {
		if value.value == "" {
			problems = append(problems, fmt.Sprintf("%s: %s must not be empty", label, value.name))
		}
	}
	if caseValue.BaseRef != "" && caseValue.BaseRef == caseValue.TargetRef {
		problems = append(problems, label+": base_ref and target_ref must differ")
	}
	if len(caseValue.Budgets) == 0 {
		problems = append(problems, label+": budgets must not be empty")
	}
	previous := 0
	for index, budget := range caseValue.Budgets {
		if budget < 256 || budget > 64000 {
			problems = append(problems, fmt.Sprintf("%s: budget[%d] must be between 256 and 64000", label, index))
		}
		if index > 0 && budget <= previous {
			problems = append(problems, label+": budgets must be sorted and unique")
		}
		previous = budget
	}
	problems = append(problems, validatePathSets(label, caseValue.RequiredPaths, caseValue.OptionalPaths, caseValue.ForbiddenPaths)...)
	problems = append(problems, validateSymbols(label+" required_symbols", caseValue.RequiredSymbols)...)
	for index, expand := range caseValue.Expand {
		expandLabel := fmt.Sprintf("%s expand[%d]", label, index)
		if !supportedRelation(expand.Relation) {
			problems = append(problems, expandLabel+": unsupported relation "+fmt.Sprintf("%q", expand.Relation))
		}
		if expand.Budget < 256 || expand.Budget > 64000 {
			problems = append(problems, expandLabel+": expansion budget must be between 256 and 64000")
		}
		problems = append(problems, validateSymbols(expandLabel+" from", []SymbolExpectation{expand.From})...)
		problems = append(problems, validatePathSets(expandLabel, expand.RequiredPaths, nil, expand.ForbiddenPaths)...)
		problems = append(problems, validateSymbols(expandLabel+" required_symbols", expand.RequiredSymbols)...)
	}
	return problems
}

func validatePathSets(label string, required, optional, forbidden []string) []string {
	var problems []string
	sets := []struct {
		name   string
		values []string
	}{{"required_paths", required}, {"optional_paths", optional}, {"forbidden_paths", forbidden}}
	seenBySet := make([]map[string]struct{}, len(sets))
	for setIndex, set := range sets {
		seenBySet[setIndex] = map[string]struct{}{}
		for valueIndex, value := range set.values {
			if issue := pathIssue(value); issue != "" {
				problems = append(problems, fmt.Sprintf("%s %s[%d]: %s", label, set.name, valueIndex, issue))
			}
			if _, exists := seenBySet[setIndex][value]; exists {
				problems = append(problems, fmt.Sprintf("%s %s: duplicate expectation %q", label, set.name, value))
			}
			seenBySet[setIndex][value] = struct{}{}
		}
	}
	for value := range seenBySet[0] {
		if _, exists := seenBySet[2][value]; exists {
			problems = append(problems, fmt.Sprintf("%s: path %q is both required and forbidden", label, value))
		}
	}
	return problems
}

func pathIssue(value string) string {
	if value == "" {
		return "path must not be empty"
	}
	if strings.Contains(value, "\\") {
		return "path must use forward slashes"
	}
	if strings.HasPrefix(value, "/") || (len(value) >= 2 && value[1] == ':') {
		return "absolute path is not allowed"
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "path must not contain .."
		}
	}
	if path.Clean(value) != value || value == "." {
		return "path is not normalized"
	}
	return ""
}

func validateSymbols(label string, symbols []SymbolExpectation) []string {
	var problems []string
	seen := map[string]struct{}{}
	for index, symbol := range symbols {
		if issue := pathIssue(symbol.Path); issue != "" {
			problems = append(problems, fmt.Sprintf("%s[%d] symbol path: %s", label, index, issue))
		}
		if symbol.Name == "" {
			problems = append(problems, fmt.Sprintf("%s[%d] symbol name must not be empty", label, index))
		}
		if symbol.Role != "" && !supportedRole(symbol.Role) {
			problems = append(problems, fmt.Sprintf("%s[%d] unsupported role %q", label, index, symbol.Role))
		}
		key := strings.Join([]string{symbol.Path, symbol.Name, symbol.Kind, symbol.Role}, "\x00")
		if _, exists := seen[key]; exists {
			problems = append(problems, fmt.Sprintf("%s: duplicate expectation %q", label, symbol.Path+":"+symbol.Name))
		}
		seen[key] = struct{}{}
	}
	return problems
}

func supportedRole(value string) bool {
	switch value {
	case "target", "definition", "declaration", "implementation", "caller", "callee", "test", "type", "import", "export", "reference", "config", "template", "documentation", "change", "dependent", "context":
		return true
	default:
		return false
	}
}

func supportedRelation(value string) bool {
	switch value {
	case "callers", "callees", "imports", "references", "tests", "children":
		return true
	default:
		return false
	}
}

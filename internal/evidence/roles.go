package evidence

import (
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

type PresentationKey struct {
	RolePriority int
	OriginalRank int
	Path         string
	StartLine    int
	Handle       string
}

type ClassifiedCandidate struct {
	Candidate       model.RankedCandidate
	OriginalRank    int
	Role            Role
	Certainty       Certainty
	Why             []string
	PresentationKey PresentationKey
}

func Classify(plan query.Plan, candidates []model.RankedCandidate) []ClassifiedCandidate {
	targetIndex := exactTargetIndex(plan, candidates)
	result := make([]ClassifiedCandidate, len(candidates))
	for index, source := range candidates {
		candidate := source
		candidate.Reasons = append([]model.ScoreReason(nil), source.Reasons...)
		if source.RelationContext != nil {
			contextCopy := *source.RelationContext
			candidate.RelationContext = &contextCopy
		}
		role := classifyRole(plan, candidate, index == targetIndex)
		certainty := Certainty("")
		if candidate.RelationContext != nil {
			certainty = certaintyFor(*candidate.RelationContext)
		}
		result[index] = ClassifiedCandidate{
			Candidate:    candidate,
			OriginalRank: index,
			Role:         role,
			Certainty:    certainty,
			Why:          whyCodes(candidate),
		}
	}
	return result
}

func SortForPresentation(plan query.Plan, candidates []ClassifiedCandidate) {
	priorities := presentationPriorities(plan)
	for index := range candidates {
		priority, ok := priorities[candidates[index].Role]
		if !ok {
			priority = len(priorities) + 1
		}
		candidates[index].PresentationKey = PresentationKey{
			RolePriority: priority,
			OriginalRank: candidates[index].OriginalRank,
			Path:         candidates[index].Candidate.Path,
			StartLine:    candidates[index].Candidate.StartLine,
			Handle:       candidates[index].Candidate.Handle,
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i].PresentationKey, candidates[j].PresentationKey
		if left.RolePriority != right.RolePriority {
			return left.RolePriority < right.RolePriority
		}
		if left.OriginalRank != right.OriginalRank {
			return left.OriginalRank < right.OriginalRank
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.StartLine != right.StartLine {
			return left.StartLine < right.StartLine
		}
		return left.Handle < right.Handle
	})
}

func certaintyFor(context model.RelationContext) Certainty {
	if !context.Resolved {
		return CertaintyLexical
	}
	if context.Confidence >= .90 {
		return CertaintyExact
	}
	return CertaintyScoped
}

func exactTargetIndex(plan query.Plan, candidates []model.RankedCandidate) int {
	anchors := make(map[string]bool, len(plan.Anchors))
	for _, anchor := range plan.Anchors {
		anchor = strings.ToLower(strings.TrimSpace(anchor))
		if anchor != "" {
			anchors[anchor] = true
		}
	}
	for index, candidate := range candidates {
		if candidate.Relation != "" || candidate.RelationContext != nil {
			continue
		}
		if hasReasonCode(candidate.Reasons, "symbol-exact") || hasReasonCode(candidate.Reasons, "qualified-symbol") {
			return index
		}
		symbol := strings.ToLower(strings.TrimSpace(candidate.Symbol))
		if anchors[symbol] {
			return index
		}
	}
	return -1
}

func classifyRole(plan query.Plan, candidate model.RankedCandidate, target bool) Role {
	if plan.PrimaryIntent == query.IntentImpact {
		if candidate.Changed {
			return RoleChange
		}
		if candidate.Relation != "" || candidate.RelationContext != nil {
			return RoleDependent
		}
	}
	if target {
		return RoleTarget
	}
	relation := candidate.Relation
	direction := model.RelationRelated
	if candidate.RelationContext != nil {
		relation = candidate.RelationContext.Kind
		direction = candidate.RelationContext.Direction
	}
	switch relation {
	case "callers":
		return RoleCaller
	case "callees":
		return RoleCallee
	case "tests":
		return RoleTest
	case "imports":
		if direction == model.RelationIncoming {
			return RoleDependent
		}
		return RoleImport
	case "exports":
		if direction == model.RelationIncoming {
			return RoleDependent
		}
		return RoleExport
	case "references":
		return RoleReference
	}

	kind := strings.ToLower(candidate.Kind)
	path := strings.ToLower(strings.ReplaceAll(candidate.Path, `\`, "/"))
	if isTestEvidence(kind, path, candidate.Symbol) {
		return RoleTest
	}
	if isTemplateEvidence(kind, candidate.Language) {
		return RoleTemplate
	}
	if isConfigEvidence(path, kind) {
		return RoleConfig
	}
	if isDocumentationEvidence(path, kind, candidate.Language) {
		return RoleDocumentation
	}
	if isHeaderDeclaration(path, kind, candidate.Language) {
		return RoleDeclaration
	}
	if isDeclarationEvidence(kind) {
		return RoleDeclaration
	}
	if isTypeEvidence(kind) {
		return RoleType
	}
	if kind == "definition" {
		return RoleDefinition
	}
	if isImplementationEvidence(kind) {
		return RoleImplementation
	}
	return RoleContext
}

func isHeaderDeclaration(path, kind, language string) bool {
	if language != "c" && language != "cpp" {
		return false
	}
	header := strings.HasSuffix(path, ".h") || strings.HasSuffix(path, ".hh") || strings.HasSuffix(path, ".hpp") || strings.HasSuffix(path, ".hxx")
	return header && (kind == "function" || kind == "method" || kind == "prototype")
}

func whyCodes(candidate model.RankedCandidate) []string {
	available := make(map[string]bool)
	for _, reason := range candidate.Reasons {
		switch reason.Code {
		case "symbol-exact":
			available["exact_symbol"] = true
		case "qualified-symbol", "symbol-prefix":
			available["qualified_symbol"] = true
		case "path":
			available["path_match"] = true
		case "lexical", "retrieval-fusion":
			available["lexical_match"] = true
		case "changed-file":
			available["changed_span"] = true
		case "same-symbol":
			available["same_symbol"] = true
		case "same-file":
			available["same_file"] = true
		}
	}
	if candidate.Changed {
		available["changed_span"] = true
	}
	relation := candidate.Relation
	direction := model.RelationRelated
	if candidate.RelationContext != nil {
		relation = candidate.RelationContext.Kind
		direction = candidate.RelationContext.Direction
	}
	switch relation {
	case "callers":
		available["direct_caller"] = true
	case "callees":
		available["direct_callee"] = true
	case "tests":
		available["related_test"] = true
	case "imports":
		if direction == model.RelationIncoming {
			available["imported_by"] = true
		} else {
			available["imports_target"] = true
		}
	case "references":
		available["references_target"] = true
	case "children":
		available["contains_target"] = true
	case "parent":
		available["parent_context"] = true
	}
	priority := []string{
		"exact_symbol", "qualified_symbol",
		"direct_caller", "direct_callee", "related_test", "imports_target", "imported_by", "references_target", "contains_target", "parent_context",
		"changed_span",
		"path_match", "lexical_match",
		"same_symbol", "same_file",
	}
	result := make([]string, 0, 4)
	for _, code := range priority {
		if available[code] {
			result = append(result, code)
			if len(result) == 4 {
				break
			}
		}
	}
	return result
}

func hasReasonCode(reasons []model.ScoreReason, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func presentationPriorities(plan query.Plan) map[Role]int {
	roles := []Role{RoleTarget, RoleImplementation, RoleDefinition, RoleDeclaration, RoleType, RoleCaller, RoleCallee, RoleTest, RoleImport, RoleReference, RoleConfig, RoleTemplate, RoleChange, RoleDependent, RoleContext, RoleDocumentation}
	switch {
	case plan.Profile == "template":
		roles = []Role{RoleTarget, RoleTemplate, RoleImport, RoleImplementation, RoleCaller, RoleTest, RoleConfig, RoleContext, RoleDocumentation}
	case plan.PrimaryIntent == query.IntentCallers:
		roles = []Role{RoleTarget, RoleCaller, RoleImplementation, RoleTest, RoleType, RoleImport, RoleReference, RoleDependent, RoleContext, RoleDocumentation}
	case plan.PrimaryIntent == query.IntentCallees:
		roles = []Role{RoleTarget, RoleCallee, RoleType, RoleImport, RoleImplementation, RoleTest, RoleReference, RoleContext, RoleDocumentation}
	case plan.PrimaryIntent == query.IntentTests:
		roles = []Role{RoleTarget, RoleTest, RoleImplementation, RoleType, RoleConfig, RoleContext, RoleDocumentation}
	case plan.PrimaryIntent == query.IntentImports || plan.PrimaryIntent == query.IntentExports:
		roles = []Role{RoleTarget, RoleImport, RoleExport, RoleDependent, RoleImplementation, RoleType, RoleConfig, RoleContext, RoleDocumentation}
	case plan.PrimaryIntent == query.IntentReferences:
		roles = []Role{RoleTarget, RoleReference, RoleType, RoleImplementation, RoleDependent, RoleTest, RoleContext, RoleDocumentation}
	case plan.PrimaryIntent == query.IntentImpact:
		roles = []Role{RoleChange, RoleTarget, RoleDependent, RoleCaller, RoleReference, RoleTest, RoleImplementation, RoleType, RoleConfig, RoleContext, RoleDocumentation}
	}
	result := make(map[Role]int, len(roles))
	for index, role := range roles {
		result[role] = index
	}
	return result
}

func isTestEvidence(kind, path, symbol string) bool {
	return kind == "test" || kind == "test-suite" || strings.HasPrefix(strings.ToLower(symbol), "test") || strings.Contains(path, "/test") || strings.HasPrefix(path, "test") || strings.Contains(path, "_test.") || strings.Contains(path, ".test.") || strings.Contains(path, ".spec.")
}

func isTemplateEvidence(kind, language string) bool {
	language = strings.ToLower(language)
	return strings.HasPrefix(kind, "template") || kind == "block" || kind == "template-function" || language == "smarty" || language == "template"
}

func isConfigEvidence(path, kind string) bool {
	if kind == "config" || kind == "manifest" || kind == "project" {
		return true
	}
	base := path
	if index := strings.LastIndexByte(base, '/'); index >= 0 {
		base = base[index+1:]
	}
	if base == "go.mod" || base == "cargo.toml" || base == "package.json" || base == "pyproject.toml" || base == "gemfile" || strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".vbproj") || strings.HasSuffix(base, ".json") || strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".toml") {
		return true
	}
	return strings.HasPrefix(path, "config/") || strings.Contains(path, "/config/")
}

func isDocumentationEvidence(path, kind, language string) bool {
	language = strings.ToLower(language)
	return kind == "heading" || kind == "documentation" || language == "markdown" || language == "text" || strings.HasSuffix(path, ".md")
}

func isDeclarationEvidence(kind string) bool {
	return kind == "prototype" || kind == "declaration" || kind == "ambient" || strings.Contains(kind, "declaration")
}

func isTypeEvidence(kind string) bool {
	kind = strings.TrimSuffix(kind, "-outline")
	switch kind {
	case "class", "interface", "struct", "record", "trait", "type", "enum", "protocol", "union":
		return true
	default:
		return false
	}
}

func isImplementationEvidence(kind string) bool {
	switch kind {
	case "function", "method", "constructor", "destructor", "operator", "implementation", "impl", "sub", "event-handler", "arrow_function", "macro":
		return true
	default:
		return false
	}
}

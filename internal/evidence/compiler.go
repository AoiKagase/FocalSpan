package evidence

import (
	"errors"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/focalspan/focalspan/internal/budget"
	"github.com/focalspan/focalspan/internal/model"
	"github.com/focalspan/focalspan/internal/query"
)

const (
	rankBaseUtility       = 100.0
	newRoleBonus          = 18.0
	newPathBonus          = 10.0
	directRelationBonus   = 16.0
	resolvedRelationBonus = 8.0
	exactIdentityBonus    = 14.0
	changedSpanBonus      = 12.0
	repeatedPathPenalty   = 7.0
)

type CompileRequest struct {
	Plan             query.Plan
	Revision         string
	TokenBudget      int
	Mode             Mode
	Candidates       []model.RankedCandidate
	KnownHandles     []string
	ExpansionAnchors []string
}

type Stats struct {
	WireTokens           int
	EvidenceTokens       int
	MetadataTokens       int
	DuplicateSourceBytes int
	Selected             int
	Omitted              int
	SkippedKnown         int
}

type CompileResult struct {
	Packet Packet
	Stats  Stats
}

type Compiler struct {
	estimator budget.TokenEstimator
}

type preparedCandidate struct {
	classified ClassifiedCandidate
	variants   []ContentVariant
}

type selectedCandidate struct {
	prepared preparedCandidate
	variant  ContentVariant
	utility  float64
}

func NewCompiler(estimator budget.TokenEstimator) *Compiler {
	if estimator == nil {
		estimator = budget.NewEstimator()
	}
	return &Compiler{estimator: estimator}
}

func (c *Compiler) Compile(req CompileRequest) (CompileResult, error) {
	limit := clampBudget(req.TokenBudget)
	mode := req.Mode
	if mode == "" {
		mode = ModeFocused
	}
	if !validMode(mode) {
		return CompileResult{}, errors.New("unsupported evidence mode")
	}
	prepared, duplicateOmitted, skippedKnown := c.preprocess(req, mode)
	baseOmitted := duplicateOmitted + skippedKnown
	selected := make([]selectedCandidate, 0, len(prepared))
	selectedHandles := make(map[string]bool, len(prepared))

	anchorIndex := -1
	for index := range prepared {
		if prepared[index].classified.Role == RoleTarget || prepared[index].classified.Role == RoleChange {
			anchorIndex = index
			break
		}
	}
	if anchorIndex >= 0 {
		anchor := prepared[anchorIndex]
		for index := len(anchor.variants) - 1; index >= 0; index-- {
			trial := appendSelected(selected, selectedCandidate{prepared: anchor, variant: anchor.variants[index], utility: candidateUtility(req.Plan, anchor.classified, nil, nil)})
			packet := buildPacket(req, mode, limit, trial, len(prepared)-len(trial)+baseOmitted, skippedKnown, c.estimator)
			if packet.Budget.Used <= limit {
				selected = trial
				selectedHandles[anchor.classified.Candidate.Handle] = true
				break
			}
		}
	}

	for {
		roles, paths := selectedDiversity(selected)
		current := buildPacket(req, mode, limit, selected, len(prepared)-len(selected)+baseOmitted, skippedKnown, c.estimator)
		bestIndex, bestVariant := -1, -1
		bestRatio := -1.0
		bestUtility := 0.0
		for candidateIndex := range prepared {
			candidate := prepared[candidateIndex]
			if selectedHandles[candidate.classified.Candidate.Handle] {
				continue
			}
			baseUtility := candidateUtility(req.Plan, candidate.classified, roles, paths)
			for variantIndex, variant := range candidate.variants {
				quality := variantQuality(variant.Fidelity)
				utility := baseUtility * quality
				trial := appendSelected(selected, selectedCandidate{prepared: candidate, variant: variant, utility: utility})
				packet := buildPacket(req, mode, limit, trial, len(prepared)-len(trial)+baseOmitted, skippedKnown, c.estimator)
				if packet.Budget.Used > limit {
					continue
				}
				incremental := packet.Budget.Used - current.Budget.Used
				if incremental < 1 {
					incremental = 1
				}
				ratio := utility / float64(incremental)
				if ratio > bestRatio || ratio == bestRatio && betterOption(candidate, variantIndex, prepared[bestIndexSafe(bestIndex)], bestVariant) {
					bestIndex, bestVariant, bestRatio, bestUtility = candidateIndex, variantIndex, ratio, utility
				}
			}
		}
		if bestIndex < 0 {
			break
		}
		choice := prepared[bestIndex]
		selected = append(selected, selectedCandidate{prepared: choice, variant: choice.variants[bestVariant], utility: bestUtility})
		selectedHandles[choice.classified.Candidate.Handle] = true
	}

	omitted := len(prepared) - len(selected) + baseOmitted
	packet := buildPacket(req, mode, limit, selected, omitted, skippedKnown, c.estimator)
	for packet.Budget.Used > limit && len(selected) > 0 {
		remove := lowestUtilityNonAnchor(selected)
		selected = append(selected[:remove], selected[remove+1:]...)
		omitted++
		packet = buildPacket(req, mode, limit, selected, omitted, skippedKnown, c.estimator)
	}
	if packet.Budget.Used > limit {
		return CompileResult{}, errors.New("empty evidence packet does not fit clamped budget")
	}
	if err := Validate(packet); err != nil {
		return CompileResult{}, err
	}
	evidenceTokens := selectedEvidenceTokens(selected)
	stats := Stats{
		WireTokens: packet.Budget.Used, EvidenceTokens: evidenceTokens,
		MetadataTokens: maxInt(0, packet.Budget.Used-evidenceTokens), DuplicateSourceBytes: duplicateSourceBytes(packet.Evidence),
		Selected: len(packet.Evidence), Omitted: omitted, SkippedKnown: skippedKnown,
	}
	return CompileResult{Packet: packet, Stats: stats}, nil
}

func (c *Compiler) preprocess(req CompileRequest, mode Mode) ([]preparedCandidate, int, int) {
	candidates := append([]model.RankedCandidate(nil), req.Candidates...)
	for index := range candidates {
		if candidates[index].Handle == "" && candidates[index].Path != "" {
			candidates[index].Handle = model.StableHandle("evidence", candidates[index].Path, candidates[index].Kind, candidates[index].Symbol, candidates[index].ContentHash, lineIdentity(candidates[index]))
		}
		candidates[index].Path = strings.ReplaceAll(candidates[index].Path, `\`, "/")
	}
	normalized := make([]model.RankedCandidate, 0, len(candidates))
	byHandle := make(map[string]int, len(candidates))
	preDuplicates := 0
	for _, candidate := range candidates {
		if index, exists := byHandle[candidate.Handle]; candidate.Handle != "" && exists {
			preDuplicates++
			if strongerCompilerProvenance(candidate.RelationContext, normalized[index].RelationContext) {
				normalized[index] = candidate
			}
			continue
		}
		if candidate.Handle != "" {
			byHandle[candidate.Handle] = len(normalized)
		}
		normalized = append(normalized, candidate)
	}
	classified := Classify(req.Plan, normalized)
	SortForPresentation(req.Plan, classified)
	known := make(map[string]bool, len(req.KnownHandles))
	for _, handle := range req.KnownHandles {
		known[strings.TrimSpace(handle)] = true
	}
	seenHandles := make(map[string]bool)
	seenHashes := make(map[string]bool)
	seenSpans := make(map[string]bool)
	prepared := make([]preparedCandidate, 0, len(classified))
	duplicates, skipped := preDuplicates, 0
	for _, candidate := range classified {
		value := candidate.Candidate
		if value.Handle == "" || value.Path == "" || strings.HasPrefix(value.Path, "/") || path.Clean(value.Path) != value.Path || value.StartLine < 1 || value.EndLine < value.StartLine {
			continue
		}
		if seenHandles[value.Handle] {
			duplicates++
			continue
		}
		seenHandles[value.Handle] = true
		hashKey := value.Path + "\x00" + value.ContentHash
		if value.ContentHash != "" && seenHashes[hashKey] {
			duplicates++
			continue
		}
		spanKey := lineIdentity(value)
		if seenSpans[spanKey] {
			duplicates++
			continue
		}
		seenSpans[spanKey] = true
		if value.ContentHash != "" {
			seenHashes[hashKey] = true
		}
		if known[value.Handle] {
			skipped++
			continue
		}
		variants := BuildVariants(candidate, req.Plan, mode, c.estimator)
		if len(variants) == 0 {
			continue
		}
		prepared = append(prepared, preparedCandidate{classified: candidate, variants: variants})
	}
	return prepared, duplicates, skipped
}

func buildPacket(req CompileRequest, mode Mode, limit int, selected []selectedCandidate, omitted, skippedKnown int, estimator budget.TokenEstimator) Packet {
	ordered := append([]selectedCandidate(nil), selected...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i].prepared.classified.PresentationKey, ordered[j].prepared.classified.PresentationKey
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
	items := make([]Item, 0, len(ordered))
	for _, selectedItem := range ordered {
		candidate := selectedItem.prepared.classified
		item := Item{Handle: candidate.Candidate.Handle, Role: candidate.Role, Location: Location{Path: candidate.Candidate.Path, Lines: [2]int{candidate.Candidate.StartLine, candidate.Candidate.EndLine}}, Language: candidate.Candidate.Language, Kind: candidate.Candidate.Kind, Symbol: candidate.Candidate.Symbol, Fidelity: selectedItem.variant.Fidelity, Why: append([]string(nil), candidate.Why...)}
		item.Source, item.Segments, item.Outline, item.Signature = selectedItem.variant.Source, append([]Segment(nil), selectedItem.variant.Segments...), selectedItem.variant.Outline, selectedItem.variant.Signature
		items = append(items, item)
	}
	ids := AssignLocalIDs(items)
	packet := Packet{Schema: SchemaContextV1, Revision: req.Revision, Intent: string(req.Plan.PrimaryIntent), Mode: mode, Budget: Budget{Limit: limit, Truncated: omitted > 0, Omitted: omitted}, Evidence: items, SkippedKnown: skippedKnown}
	packet.Relations = selectedEdges(ordered, ids)
	settleWireUsage(&packet, estimator)
	return packet
}

func selectedEdges(selected []selectedCandidate, ids map[string]string) []Edge {
	result := make([]Edge, 0)
	seen := make(map[Edge]bool)
	for _, item := range selected {
		context := item.prepared.classified.Candidate.RelationContext
		if context == nil || context.Direction == model.RelationRelated {
			continue
		}
		candidateID, candidateOK := ids[item.prepared.classified.Candidate.Handle]
		anchorID, anchorOK := ids[context.AnchorHandle]
		if !candidateOK || !anchorOK || candidateID == anchorID {
			continue
		}
		edge := Edge{Kind: edgeKind(context.Kind), Certainty: certaintyFor(*context)}
		if context.Direction == model.RelationIncoming {
			edge.From, edge.To = candidateID, anchorID
		} else {
			edge.From, edge.To = anchorID, candidateID
		}
		if edge.Kind != "" && !seen[edge] {
			seen[edge] = true
			result = append(result, edge)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].From != result[j].From {
			return result[i].From < result[j].From
		}
		if result[i].To != result[j].To {
			return result[i].To < result[j].To
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Certainty < result[j].Certainty
	})
	return result
}

func edgeKind(relation string) string {
	switch relation {
	case "callers", "callees":
		return "calls"
	case "parent", "children":
		return "contains"
	case "tests", "imports", "exports", "references":
		return relation
	default:
		return ""
	}
}

func candidateUtility(plan query.Plan, candidate ClassifiedCandidate, roles map[Role]bool, paths map[string]bool) float64 {
	utility := rankBaseUtility / float64(1+candidate.OriginalRank)
	utility += roleWeight(plan, candidate.Role)
	if roles != nil && !roles[candidate.Role] {
		utility += newRoleBonus
	}
	if paths != nil {
		if !paths[candidate.Candidate.Path] {
			utility += newPathBonus
		} else {
			utility -= repeatedPathPenalty
		}
	}
	if candidate.Candidate.RelationContext != nil {
		utility += directRelationBonus
		if candidate.Candidate.RelationContext.Resolved {
			utility += resolvedRelationBonus
		}
	}
	if containsString(candidate.Why, "exact_symbol") || containsString(candidate.Why, "qualified_symbol") {
		utility += exactIdentityBonus
	}
	if candidate.Candidate.Changed {
		utility += changedSpanBonus
	}
	return utility
}

func roleWeight(plan query.Plan, role Role) float64 {
	profiles := map[query.Intent]map[Role]float64{
		query.IntentDefinition: {RoleTarget: 45, RoleImplementation: 34, RoleDefinition: 30, RoleDeclaration: 24, RoleType: 18},
		query.IntentCallers:    {RoleTarget: 40, RoleCaller: 46, RoleTest: 20, RoleImplementation: 18},
		query.IntentCallees:    {RoleTarget: 40, RoleCallee: 46, RoleType: 20, RoleImport: 18},
		query.IntentTests:      {RoleTarget: 36, RoleTest: 50, RoleImplementation: 18, RoleConfig: 12},
		query.IntentImports:    {RoleTarget: 34, RoleImport: 46, RoleExport: 38, RoleDependent: 24, RoleConfig: 14},
		query.IntentExports:    {RoleTarget: 34, RoleImport: 46, RoleExport: 38, RoleDependent: 24, RoleConfig: 14},
		query.IntentReferences: {RoleTarget: 38, RoleReference: 44, RoleType: 30, RoleDependent: 20},
		query.IntentImpact:     {RoleChange: 50, RoleDependent: 46, RoleCaller: 30, RoleTest: 28, RoleTarget: 26},
	}
	if weight := profiles[plan.PrimaryIntent][role]; weight > 0 {
		return weight
	}
	return 8
}

func variantQuality(fidelity Fidelity) float64 {
	switch fidelity {
	case FidelityVerbatim:
		return 1.30
	case FidelityExcerpt:
		return 1.20
	case FidelitySynthetic:
		return 1.05
	default:
		return 1
	}
}

func selectedDiversity(selected []selectedCandidate) (map[Role]bool, map[string]bool) {
	roles, paths := make(map[Role]bool), make(map[string]bool)
	for _, item := range selected {
		roles[item.prepared.classified.Role] = true
		paths[item.prepared.classified.Candidate.Path] = true
	}
	return roles, paths
}

func selectedEvidenceTokens(selected []selectedCandidate) int {
	total := 0
	for _, item := range selected {
		total += item.variant.EvidenceTokens
	}
	return total
}
func appendSelected(values []selectedCandidate, value selectedCandidate) []selectedCandidate {
	result := append([]selectedCandidate(nil), values...)
	return append(result, value)
}
func lowestUtilityNonAnchor(selected []selectedCandidate) int {
	index := len(selected) - 1
	lowest := math.MaxFloat64
	for i, item := range selected {
		if (item.prepared.classified.Role == RoleTarget || item.prepared.classified.Role == RoleChange) && len(selected) > 1 {
			continue
		}
		if item.utility < lowest {
			lowest, index = item.utility, i
		}
	}
	return index
}
func clampBudget(value int) int {
	if value < budget.MinBudget {
		return budget.MinBudget
	}
	if value > budget.MaxBudget {
		return budget.MaxBudget
	}
	return value
}
func lineIdentity(candidate model.RankedCandidate) string {
	return candidate.Path + "\x00" + strconv.Itoa(candidate.StartLine) + "\x00" + strconv.Itoa(candidate.EndLine)
}

func strongerCompilerProvenance(left, right *model.RelationContext) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}
	if left.Resolved != right.Resolved {
		return left.Resolved
	}
	if left.Confidence != right.Confidence {
		return left.Confidence > right.Confidence
	}
	leftDirect, rightDirect := left.Direction != model.RelationRelated, right.Direction != model.RelationRelated
	if leftDirect != rightDirect {
		return leftDirect
	}
	leftKey := left.Kind + "\x00" + string(left.Direction) + "\x00" + left.AnchorHandle + "\x00" + left.Source
	rightKey := right.Kind + "\x00" + string(right.Direction) + "\x00" + right.AnchorHandle + "\x00" + right.Source
	return leftKey < rightKey
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
func bestIndexSafe(index int) int {
	if index < 0 {
		return 0
	}
	return index
}
func betterOption(left preparedCandidate, leftVariant int, right preparedCandidate, rightVariant int) bool {
	if rightVariant < 0 {
		return true
	}
	lk, rk := left.classified.PresentationKey, right.classified.PresentationKey
	if lk.RolePriority != rk.RolePriority {
		return lk.RolePriority < rk.RolePriority
	}
	if lk.OriginalRank != rk.OriginalRank {
		return lk.OriginalRank < rk.OriginalRank
	}
	if leftVariant != rightVariant {
		return leftVariant < rightVariant
	}
	return left.classified.Candidate.Handle < right.classified.Candidate.Handle
}
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func duplicateSourceBytes(items []Item) int {
	total := 0
	seen := make(map[string]bool)
	for _, item := range items {
		texts := []string{item.Source}
		for _, segment := range item.Segments {
			if segment.Kind == SegmentSource {
				texts = append(texts, segment.Text)
			}
		}
		for _, text := range texts {
			if text == "" {
				continue
			}
			if seen[text] {
				total += len(text)
			} else {
				seen[text] = true
			}
		}
	}
	return total
}

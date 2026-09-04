package evidence

import (
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/query"
)

var limitationOrder = []string{
	"budget_limited",
	"source_reduced_to_excerpt",
	"source_reduced_to_signature",
	"additional_callers_omitted",
	"additional_callees_omitted",
	"additional_tests_omitted",
	"additional_imports_omitted",
	"additional_references_omitted",
	"dynamic_dispatch_unresolved",
	"lexical_relation_only",
	"known_anchor_not_repeated",
	"no_relevant_source_found",
	"syntax_only_impact",
}

var nextReasonOrder = []string{
	"more_callers_omitted",
	"more_callees_omitted",
	"more_tests_omitted",
	"more_imports_omitted",
	"more_references_omitted",
	"parent_context_available",
	"children_available",
	"source_body_omitted",
}

type GuidanceSelection struct {
	Candidate ClassifiedCandidate
	Fidelity  Fidelity
}

type GuidanceInput struct {
	Plan             query.Plan
	Selected         []GuidanceSelection
	Omitted          []ClassifiedCandidate
	KnownHandles     []string
	ExpansionAnchors []string
	Truncated        bool
}

func BuildGuidance(input GuidanceInput) ([]string, []NextAction) {
	limitations := make(map[string]bool)
	actions := make(map[string]NextAction)
	selectedHandles := make(map[string]bool, len(input.Selected))
	knownHandles := make(map[string]bool, len(input.KnownHandles))
	for _, handle := range input.KnownHandles {
		knownHandles[handle] = true
	}
	if input.Truncated {
		limitations["budget_limited"] = true
	}
	if len(input.Selected) == 0 {
		limitations["no_relevant_source_found"] = true
	}
	if input.Plan.PrimaryIntent == query.IntentImpact {
		limitations["syntax_only_impact"] = true
	}

	target := ""
	for _, selected := range input.Selected {
		handle := selected.Candidate.Candidate.Handle
		selectedHandles[handle] = true
		if target == "" && (selected.Candidate.Role == RoleTarget || selected.Candidate.Role == RoleChange) {
			target = handle
		}
		switch selected.Fidelity {
		case FidelityExcerpt:
			limitations["source_reduced_to_excerpt"] = true
		case FidelitySignature:
			limitations["source_reduced_to_signature"] = true
			addGuidanceAction(actions, NextAction{Handle: handle, Relation: "self", Reason: "source_body_omitted"})
		}
		context := selected.Candidate.Candidate.RelationContext
		if context == nil {
			continue
		}
		if !context.Resolved {
			limitations["lexical_relation_only"] = true
			if context.Kind == "callers" || context.Kind == "callees" {
				limitations["dynamic_dispatch_unresolved"] = true
			}
		}
		if knownHandles[context.AnchorHandle] && !selectedHandles[context.AnchorHandle] {
			limitations["known_anchor_not_repeated"] = true
		}
		switch context.Kind {
		case "parent":
			addGuidanceAction(actions, NextAction{Handle: context.AnchorHandle, Relation: "parent", Reason: "parent_context_available"})
		case "children":
			addGuidanceAction(actions, NextAction{Handle: context.AnchorHandle, Relation: "children", Reason: "children_available"})
		}
	}
	if target == "" && len(input.ExpansionAnchors) > 0 {
		target = input.ExpansionAnchors[0]
	}
	if target == "" && len(input.KnownHandles) > 0 {
		target = input.KnownHandles[0]
	}

	for _, omitted := range input.Omitted {
		context := omitted.Candidate.RelationContext
		anchor := target
		if context != nil && context.AnchorHandle != "" {
			anchor = context.AnchorHandle
		}
		if context != nil && !context.Resolved {
			limitations["lexical_relation_only"] = true
			if context.Kind == "callers" || context.Kind == "callees" {
				limitations["dynamic_dispatch_unresolved"] = true
			}
		}
		switch omitted.Role {
		case RoleCaller:
			limitations["additional_callers_omitted"] = true
			addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "callers", Reason: "more_callers_omitted"})
		case RoleCallee:
			limitations["additional_callees_omitted"] = true
			addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "callees", Reason: "more_callees_omitted"})
		case RoleTest:
			limitations["additional_tests_omitted"] = true
			addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "tests", Reason: "more_tests_omitted"})
		case RoleImport, RoleExport:
			limitations["additional_imports_omitted"] = true
			addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "imports", Reason: "more_imports_omitted"})
		case RoleReference:
			limitations["additional_references_omitted"] = true
			addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "references", Reason: "more_references_omitted"})
		}
		if context != nil {
			switch context.Kind {
			case "parent":
				addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "parent", Reason: "parent_context_available"})
			case "children":
				addGuidanceAction(actions, NextAction{Handle: anchor, Relation: "children", Reason: "children_available"})
			}
		}
	}

	orderedLimitations := make([]string, 0, 8)
	for _, code := range limitationOrder {
		if limitations[code] {
			orderedLimitations = append(orderedLimitations, code)
			if len(orderedLimitations) == 8 {
				break
			}
		}
	}
	orderedActions := make([]NextAction, 0, len(actions))
	for _, action := range actions {
		if action.Handle != "" {
			orderedActions = append(orderedActions, action)
		}
	}
	reasonPriority := make(map[string]int, len(nextReasonOrder))
	for index, reason := range nextReasonOrder {
		reasonPriority[reason] = index
	}
	sort.Slice(orderedActions, func(i, j int) bool {
		left, right := orderedActions[i], orderedActions[j]
		if reasonPriority[left.Reason] != reasonPriority[right.Reason] {
			return reasonPriority[left.Reason] < reasonPriority[right.Reason]
		}
		if left.Handle != right.Handle {
			return left.Handle < right.Handle
		}
		return left.Relation < right.Relation
	})
	if len(orderedActions) > 4 {
		orderedActions = orderedActions[:4]
	}
	return orderedLimitations, orderedActions
}

// pruneKnownDeltaGuidance removes envelope text that only explains why an
// already-known anchor was not repeated. It is intentionally limited to
// known-handle expansions with no other relation edge or next relation action;
// actionable and safety-related guidance remains untouched.
func pruneKnownDeltaGuidance(packet Packet, knownValues []string, limitations []string, actions []NextAction) ([]string, []NextAction) {
	if len(knownValues) == 0 || len(packet.Relations) > 0 {
		return limitations, actions
	}
	known := make(map[string]bool, len(knownValues))
	for _, value := range knownValues {
		if handle := strings.TrimSpace(value); handle != "" {
			known[handle] = true
		}
	}
	if len(known) == 0 {
		return limitations, actions
	}
	for _, action := range actions {
		if action.Relation != "" && action.Relation != "self" {
			return limitations, actions
		}
	}
	knownOnlyEmpty := len(packet.Evidence) == 0 && packet.SkippedKnown > 0
	filteredLimitations := make([]string, 0, len(limitations))
	for _, limitation := range limitations {
		if limitation == "known_anchor_not_repeated" || knownOnlyEmpty && limitation == "no_relevant_source_found" {
			continue
		}
		if limitation != "known_anchor_not_repeated" {
			filteredLimitations = append(filteredLimitations, limitation)
		}
	}
	filteredActions := make([]NextAction, 0, len(actions))
	for _, action := range actions {
		if action.Relation == "self" && known[action.Handle] {
			continue
		}
		filteredActions = append(filteredActions, action)
	}
	return filteredLimitations, filteredActions
}

func addGuidanceAction(actions map[string]NextAction, action NextAction) {
	if action.Handle == "" || action.Relation == "" || action.Reason == "" {
		return
	}
	key := action.Handle + "\x00" + action.Relation
	if _, exists := actions[key]; !exists {
		actions[key] = action
	}
}

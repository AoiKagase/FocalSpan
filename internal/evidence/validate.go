package evidence

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

var windowsDrivePath = regexp.MustCompile(`^[A-Za-z]:/`)

func Validate(packet Packet) error {
	if packet.Schema != SchemaContextV1 {
		return fmt.Errorf("invalid schema %q", packet.Schema)
	}
	if !validMode(packet.Mode) {
		return fmt.Errorf("unsupported mode %q", packet.Mode)
	}
	if packet.Budget.Limit < 0 || packet.Budget.Used < 0 || packet.Budget.Used > packet.Budget.Limit || packet.Budget.Omitted < 0 {
		return fmt.Errorf("invalid budget: used %d, limit %d, omitted %d", packet.Budget.Used, packet.Budget.Limit, packet.Budget.Omitted)
	}
	if packet.SkippedKnown < 0 {
		return fmt.Errorf("skipped_known must not be negative")
	}
	if len(packet.Limitations) > 8 {
		return fmt.Errorf("limitations has %d entries; maximum is 8", len(packet.Limitations))
	}
	if len(packet.Next) > 4 {
		return fmt.Errorf("next has %d entries; maximum is 4", len(packet.Next))
	}

	ids := make(map[string]struct{}, len(packet.Evidence))
	handles := make(map[string]struct{}, len(packet.Evidence))
	for i, item := range packet.Evidence {
		wantID := fmt.Sprintf("e%d", i+1)
		if item.ID != wantID {
			return fmt.Errorf("evidence IDs must be sequential: item %d has %q, want %q", i, item.ID, wantID)
		}
		if item.Handle == "" {
			return fmt.Errorf("evidence %s has blank handle", item.ID)
		}
		if _, exists := handles[item.Handle]; exists {
			return fmt.Errorf("duplicate handle %q", item.Handle)
		}
		handles[item.Handle] = struct{}{}
		ids[item.ID] = struct{}{}
		if err := validateItem(item); err != nil {
			return fmt.Errorf("evidence %s: %w", item.ID, err)
		}
	}

	edges := make(map[Edge]struct{}, len(packet.Relations))
	for _, edge := range packet.Relations {
		if _, ok := ids[edge.From]; !ok {
			return fmt.Errorf("relation from references missing local ID %q", edge.From)
		}
		if _, ok := ids[edge.To]; !ok {
			return fmt.Errorf("relation to references missing local ID %q", edge.To)
		}
		if edge.From == edge.To && edge.Kind != "self" {
			return fmt.Errorf("relation has invalid self-edge %q", edge.Kind)
		}
		if edge.Kind == "" {
			return fmt.Errorf("relation kind is blank")
		}
		if !validCertainty(edge.Certainty) {
			return fmt.Errorf("relation has unknown certainty %q", edge.Certainty)
		}
		if _, exists := edges[edge]; exists {
			return fmt.Errorf("duplicate edge %s -> %s (%s/%s)", edge.From, edge.To, edge.Kind, edge.Certainty)
		}
		edges[edge] = struct{}{}
	}
	return nil
}

func validateItem(item Item) error {
	if !validRole(item.Role) {
		return fmt.Errorf("unknown role %q", item.Role)
	}
	if !validFidelity(item.Fidelity) {
		return fmt.Errorf("unknown fidelity %q", item.Fidelity)
	}
	if err := validateLocation(item.Location); err != nil {
		return err
	}
	if len(item.Why) > 4 {
		return fmt.Errorf("why has %d entries; maximum is 4", len(item.Why))
	}

	switch item.Fidelity {
	case FidelityVerbatim:
		if item.Source == "" || len(item.Segments) != 0 || item.Outline != "" || item.Signature != "" {
			return fmt.Errorf("verbatim fidelity requires source only")
		}
	case FidelityExcerpt:
		if item.Source != "" || item.Outline != "" || item.Signature != "" || len(item.Segments) == 0 {
			return fmt.Errorf("excerpt fidelity requires segments only")
		}
		hasSource := false
		for _, segment := range item.Segments {
			if err := validateSegment(segment, item.Location.Lines); err != nil {
				return err
			}
			if segment.Kind == SegmentSource {
				hasSource = true
			}
		}
		if !hasSource {
			return fmt.Errorf("excerpt requires at least one source segment")
		}
	case FidelitySignature:
		if item.Signature == "" || item.Source != "" || len(item.Segments) != 0 || item.Outline != "" {
			return fmt.Errorf("signature fidelity requires signature only")
		}
	case FidelitySynthetic:
		if item.Outline == "" || item.Source != "" || len(item.Segments) != 0 || item.Signature != "" {
			return fmt.Errorf("synthetic fidelity requires outline only")
		}
	}
	return nil
}

func validateLocation(location Location) error {
	if location.Path == "" || strings.HasPrefix(location.Path, "/") || windowsDrivePath.MatchString(location.Path) {
		return fmt.Errorf("location path must be repository-relative: %q", location.Path)
	}
	if strings.Contains(location.Path, `\`) {
		return fmt.Errorf("location path must be slash-normalized: %q", location.Path)
	}
	if path.Clean(location.Path) != location.Path || location.Path == "." || strings.HasPrefix(location.Path, "../") {
		return fmt.Errorf("location path must be repository-relative and normalized: %q", location.Path)
	}
	if !validLines(location.Lines) {
		return fmt.Errorf("invalid line range %v", location.Lines)
	}
	return nil
}

func validateSegment(segment Segment, itemLines [2]int) error {
	if !validLines(segment.Lines) || segment.Lines[0] < itemLines[0] || segment.Lines[1] > itemLines[1] {
		return fmt.Errorf("segment has invalid line range %v", segment.Lines)
	}
	switch segment.Kind {
	case SegmentSource:
		if segment.Text == "" {
			return fmt.Errorf("source segment text is blank")
		}
	case SegmentOmitted:
		if segment.Text != "" {
			return fmt.Errorf("omitted segment must not contain text")
		}
	default:
		return fmt.Errorf("unknown segment kind %q", segment.Kind)
	}
	return nil
}

func validLines(lines [2]int) bool {
	return lines[0] >= 1 && lines[0] <= lines[1]
}

func validMode(mode Mode) bool {
	return mode == ModeOutline || mode == ModeFocused || mode == ModeSource
}

func validFidelity(fidelity Fidelity) bool {
	return fidelity == FidelityVerbatim || fidelity == FidelityExcerpt || fidelity == FidelitySignature || fidelity == FidelitySynthetic
}

func validCertainty(certainty Certainty) bool {
	return certainty == CertaintyExact || certainty == CertaintyScoped || certainty == CertaintyLexical
}

func validRole(role Role) bool {
	switch role {
	case RoleTarget, RoleDefinition, RoleDeclaration, RoleImplementation,
		RoleCaller, RoleCallee, RoleTest, RoleType, RoleImport, RoleExport,
		RoleReference, RoleConfig, RoleTemplate, RoleDocumentation, RoleChange,
		RoleDependent, RoleContext:
		return true
	default:
		return false
	}
}

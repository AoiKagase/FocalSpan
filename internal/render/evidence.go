package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/evidence"
)

func EvidenceJSON(packet evidence.Packet) ([]byte, error) {
	if err := evidence.Validate(packet); err != nil {
		return nil, err
	}
	return json.MarshalIndent(packet, "", "  ")
}

func EvidenceCompact(packet evidence.Packet) string {
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\nintent: %s\nmode: %s\nbudget: %d/%d\n", packet.Schema, packet.Intent, packet.Mode, packet.Budget.Used, packet.Budget.Limit)
	for _, item := range packet.Evidence {
		fmt.Fprintf(&out, "\n[%s %s] %s:%d-%d\n", item.ID, item.Role, item.Location.Path, item.Location.Lines[0], item.Location.Lines[1])
		if item.Symbol != "" {
			out.WriteString(item.Symbol)
			out.WriteByte('\n')
		}
		if len(item.Why) > 0 {
			fmt.Fprintf(&out, "why: %s\n", strings.Join(item.Why, ", "))
		}
		out.WriteString("------------------------------------------------\n")
		renderEvidenceContent(&out, item)
	}
	if len(packet.Relations) > 0 {
		out.WriteString("\nrelations:\n")
		for _, edge := range packet.Relations {
			fmt.Fprintf(&out, "  %s -%s/%s-> %s\n", edge.From, edge.Kind, edge.Certainty, edge.To)
		}
	}
	if len(packet.Limitations) > 0 {
		out.WriteString("\nlimitations:\n")
		for _, limitation := range packet.Limitations {
			fmt.Fprintf(&out, "  %s\n", limitation)
		}
	}
	if len(packet.Next) > 0 {
		out.WriteString("\nnext:\n")
		for _, action := range packet.Next {
			fmt.Fprintf(&out, "  %s %s (%s)\n", action.Relation, action.Handle, action.Reason)
		}
	}
	return out.String()
}

func renderEvidenceContent(out *strings.Builder, item evidence.Item) {
	switch item.Fidelity {
	case evidence.FidelityVerbatim:
		out.WriteString(item.Source)
		ensureEvidenceNewline(out, item.Source)
	case evidence.FidelityExcerpt:
		for _, segment := range item.Segments {
			if segment.Kind == evidence.SegmentOmitted {
				fmt.Fprintf(out, "--- lines %d-%d omitted ---\n", segment.Lines[0], segment.Lines[1])
				continue
			}
			out.WriteString(segment.Text)
			ensureEvidenceNewline(out, segment.Text)
		}
	case evidence.FidelitySignature:
		out.WriteString(item.Signature)
		ensureEvidenceNewline(out, item.Signature)
	case evidence.FidelitySynthetic:
		out.WriteString(item.Outline)
		ensureEvidenceNewline(out, item.Outline)
	}
}

func ensureEvidenceNewline(out *strings.Builder, value string) {
	if value != "" && !strings.HasSuffix(value, "\n") {
		out.WriteByte('\n')
	}
}

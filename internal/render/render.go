package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

func Compact(bundle model.ContextBundle) string {
	var b strings.Builder
	fmt.Fprintf(&b, "query: %s\n", bundle.Query)
	fmt.Fprintf(&b, "budget: %d\n", bundle.BudgetTokens)
	fmt.Fprintf(&b, "estimated: %d\n", bundle.EstimatedTokens)
	if bundle.Savings != nil {
		fmt.Fprintf(&b, "baseline: %d\n", bundle.Savings.BaselineTokens)
		fmt.Fprintf(&b, "saved: %d tokens (%.1f%%)\n", bundle.Savings.SavedTokens, bundle.Savings.SavingsRatio*100)
	}
	if bundle.IndexRevision != "" {
		fmt.Fprintf(&b, "revision: %s\n", bundle.IndexRevision)
	}
	for _, item := range bundle.Items {
		fmt.Fprintf(&b, "\n[%s] %s:%d-%d\n", item.Handle, item.Path, item.StartLine, item.EndLine)
		if item.Symbol != "" {
			fmt.Fprintln(&b, item.Symbol)
		}
		if len(item.Reasons) > 0 {
			codes := make([]string, 0, len(item.Reasons))
			for _, reason := range item.Reasons {
				codes = append(codes, reason.Code)
			}
			fmt.Fprintf(&b, "reason: %s\n", strings.Join(codes, ", "))
		}
		fmt.Fprintln(&b, "------------------------------------------------")
		if item.Content != "" {
			b.WriteString(item.Content)
			if !strings.HasSuffix(item.Content, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

func CompactDebug(bundle model.ContextBundle) string {
	output := Compact(bundle)
	var b strings.Builder
	b.WriteString(output)
	for _, item := range bundle.Items {
		fmt.Fprintf(&b, "score: %.3f %s\n", item.Score, item.Handle)
		for _, reason := range item.Reasons {
			fmt.Fprintf(&b, "  %s: %.3f\n", reason.Code, reason.Weight)
		}
	}
	return b.String()
}

func JSON(bundle model.ContextBundle) ([]byte, error) { return json.MarshalIndent(bundle, "", "  ") }

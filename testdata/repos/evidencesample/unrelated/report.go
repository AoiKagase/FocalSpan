package unrelated

import "strings"

func BuildReport(values []string) string {
	var report strings.Builder
	for _, value := range values {
		report.WriteString(value)
		report.WriteByte('\n')
	}
	return report.String()
}

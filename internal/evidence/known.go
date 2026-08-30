package evidence

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func NormalizeKnownHandles(values []string) ([]string, error) {
	if len(values) > 512 {
		return nil, fmt.Errorf("known_handles has %d entries; maximum is 512", len(values))
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !utf8.ValidString(value) {
			return nil, fmt.Errorf("known_handles[%d] is not valid UTF-8", index)
		}
		if len(value) > 256 {
			return nil, fmt.Errorf("known_handles[%d] exceeds 256 bytes", index)
		}
		for _, r := range value {
			if r < 0x20 {
				return nil, fmt.Errorf("known_handles[%d] contains a control character", index)
			}
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result, nil
}

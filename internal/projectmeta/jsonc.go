package projectmeta

import (
	"fmt"
	"strings"
)

// jsonc removes comments/trailing commas without altering quoted JSON values.
func jsonc(text string) ([]byte, error) {
	data := []byte(strings.TrimPrefix(text, "\uFEFF"))
	quoted := false
	for i := 0; i < len(data); i++ {
		if quoted {
			if data[i] == '\\' {
				i++
			} else if data[i] == '"' {
				quoted = false
			}
			continue
		}
		if data[i] == '"' {
			quoted = true
			continue
		}
		if data[i] != '/' || i+1 >= len(data) {
			continue
		}
		if data[i+1] == '/' {
			data[i] = ' '
			i++
			data[i] = ' '
			for i+1 < len(data) && data[i+1] != '\n' && data[i+1] != '\r' {
				i++
				data[i] = ' '
			}
			continue
		}
		if data[i+1] == '*' {
			start := i
			data[i] = ' '
			i++
			data[i] = ' '
			closed := false
			for i+1 < len(data) {
				i++
				if data[i] == '*' && i+1 < len(data) && data[i+1] == '/' {
					data[i] = ' '
					i++
					data[i] = ' '
					closed = true
					break
				}
				if data[i] != '\n' && data[i] != '\r' {
					data[i] = ' '
				}
			}
			if !closed {
				return nil, fmt.Errorf("unclosed JSONC comment at byte %d", start)
			}
		}
	}
	quoted = false
	for i := 0; i < len(data); i++ {
		if quoted {
			if data[i] == '\\' {
				i++
			} else if data[i] == '"' {
				quoted = false
			}
			continue
		}
		if data[i] == '"' {
			quoted = true
			continue
		}
		if data[i] == ',' {
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\r' || data[j] == '\n') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				data[i] = ' '
			}
		}
	}
	return data, nil
}

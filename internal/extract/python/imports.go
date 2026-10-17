package python

import "strings"

func pythonImportRecords(lines []pyLine) map[int][]string {
	result := make(map[int][]string)
	for at := 0; at < len(lines); at++ {
		text := strings.TrimSpace(lines[at].Text)
		words := strings.Fields(text)
		if len(words) == 0 || (words[0] != "import" && words[0] != "from") {
			continue
		}
		start := at
		depth := strings.Count(text, "(") - strings.Count(text, ")")
		for (depth > 0 || strings.HasSuffix(strings.TrimSpace(text), `\`)) && at+1 < len(lines) {
			at++
			next := strings.TrimSpace(lines[at].Text)
			text += " " + next
			depth += strings.Count(next, "(") - strings.Count(next, ")")
		}
		text = strings.NewReplacer("(", " ", ")", " ", `\`, " ").Replace(text)
		words = strings.Fields(text)
		module := ""
		offset := 1
		if words[0] == "from" {
			if len(words) < 4 || words[2] != "import" {
				continue
			}
			module = words[1]
			offset = 3
		}
		for _, part := range strings.Split(strings.Join(words[offset:], " "), ",") {
			fields := strings.Fields(part)
			if len(fields) == 0 {
				continue
			}
			target := fields[0]
			if module != "" {
				if target == "*" {
					target = module
				} else {
					target = module + "." + target
				}
			}
			result[start] = append(result[start], target)
		}
	}
	return result
}

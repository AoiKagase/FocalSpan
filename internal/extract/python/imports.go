package python

import (
	"sort"
	"strings"
)

func pythonImportRecords(lines []pyLine) map[int][]string {
	result := make(map[int][]string)
	var code strings.Builder
	starts := make([]int, 0, len(lines))
	for _, line := range lines {
		starts = append(starts, code.Len())
		code.WriteString(line.Text)
		code.WriteByte('\n')
	}
	for _, statement := range pythonSimpleStatements(code.String()) {
		text := strings.TrimSpace(statement.text)
		words := strings.Fields(text)
		if len(words) == 0 || (words[0] != "import" && words[0] != "from") {
			continue
		}
		start := sort.Search(len(starts), func(i int) bool { return starts[i] > statement.offset }) - 1
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

type pythonStatement struct {
	text   string
	offset int
}

// Input is the masked code view: string/comment punctuation cannot introduce
// statement boundaries. Offsets still identify the original physical line.
func pythonSimpleStatements(code string) []pythonStatement {
	result := make([]pythonStatement, 0)
	start, depth := 0, 0
	emit := func(end int) {
		text := code[start:end]
		trimmed := strings.TrimLeft(text, " \t\r\n")
		if trimmed != "" {
			result = append(result, pythonStatement{trimmed, start + len(text) - len(trimmed)})
		}
	}
	for at := 0; at < len(code); at++ {
		switch code[at] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ':':
			if depth == 0 && pythonSuitePrefix(strings.TrimSpace(code[start:at])) {
				start = at + 1
			}
		case ';':
			if depth == 0 {
				emit(at)
				start = at + 1
			}
		case '\n':
			prior := at - 1
			for prior >= start && (code[prior] == ' ' || code[prior] == '\t' || code[prior] == '\r') {
				prior--
			}
			if depth == 0 && (prior < start || code[prior] != '\\') {
				emit(at)
				start = at + 1
			}
		}
	}
	if start < len(code) {
		emit(len(code))
	}
	return result
}

func pythonSuitePrefix(text string) bool {
	words := strings.Fields(text)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "if", "elif", "else", "for", "while", "with", "try", "except", "finally", "def", "class", "match", "case":
		return true
	case "async":
		return len(words) > 1 && (words[1] == "for" || words[1] == "with" || words[1] == "def")
	}
	return false
}

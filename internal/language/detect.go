package language

import (
	"path"
	"path/filepath"
	"strings"
)

type Detection struct {
	Language   string
	Reason     string
	Confidence float64
}

func Detect(filePath string, content []byte, overrides map[string]string) Detection {
	if override, ok := selectOverride(filePath, overrides); ok {
		return Detection{Language: override, Reason: "override", Confidence: 1}
	}

	normalized := strings.ReplaceAll(filepath.ToSlash(filePath), "\\", "/")
	base := strings.ToLower(path.Base(normalized))
	lowerPath := strings.ToLower(normalized)
	if language, ok := specialSuffixProfile(lowerPath, base); ok {
		return Detection{Language: language, Reason: "extension", Confidence: .95}
	}

	switch strings.ToLower(path.Ext(base)) {
	case ".tpl":
		if containsSmartyMarker(content) {
			return Detection{Language: "smarty", Reason: "content", Confidence: .9}
		}
		if containsPHPTag(content) {
			return Detection{Language: "php", Reason: "content", Confidence: .85}
		}
		return Detection{Language: "template", Reason: "extension", Confidence: .75}
	case ".smarty":
		return Detection{Language: "smarty", Reason: "extension", Confidence: .95}
	case ".inc":
		if containsPHPTag(content) {
			return Detection{Language: "php", Reason: "content", Confidence: .95}
		}
		if scorePawn(content) >= pawnThreshold {
			return Detection{Language: "pawn", Reason: "content", Confidence: .8}
		}
		return Detection{Language: "text", Reason: "fallback", Confidence: .3}
	}

	if language, ok := extensionProfiles[strings.ToLower(path.Ext(base))]; ok {
		return Detection{Language: language, Reason: "extension", Confidence: .95}
	}
	return Detection{Language: "text", Reason: "fallback", Confidence: .2}
}

var extensionProfiles = map[string]string{
	".go": "go", ".c": "c", ".cc": "cpp", ".cpp": "cpp", ".cxx": "cpp", ".c++": "cpp",
	".h": "cpp", ".hh": "cpp", ".hpp": "cpp", ".hxx": "cpp", ".inl": "cpp", ".ipp": "cpp",
	".tpp": "cpp", ".ixx": "cpp", ".cppm": "cpp", ".cs": "csharp", ".csx": "csharp",
	".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
	".ts": "typescript", ".tsx": "typescript", ".mts": "typescript", ".cts": "typescript",
	".php": "php", ".phtml": "php", ".php3": "php", ".php4": "php", ".php5": "php",
	".php7": "php", ".php8": "php", ".phps": "php", ".rs": "rust", ".py": "python",
	".pyw": "python", ".pyi": "python", ".rb": "ruby", ".rake": "ruby", ".gemspec": "ruby",
	".nim": "nim", ".nims": "nim", ".nimble": "nim", ".zig": "zig", ".bas": "vb6",
	".cls": "vb6", ".frm": "vb6", ".ctl": "vb6", ".pag": "vb6", ".vbp": "vb6-project",
	".vb": "vbnet", ".xaml": "xaml", ".resx": "dotnet-resource", ".settings": "dotnet-resource",
	".lua": "lua", ".rockspec": "lua", ".sma": "pawn", ".pwn": "pawn", ".md": "markdown",
	".markdown": "markdown", ".json": "config", ".yaml": "config", ".yml": "config", ".toml": "config",
	".xml": "config", ".ps1": "powershell", ".sh": "shell", ".bash": "shell",
}

func specialSuffixProfile(lowerPath, base string) (string, bool) {
	switch {
	case strings.HasSuffix(lowerPath, ".xaml.cs"):
		return "csharp", true
	case strings.HasSuffix(lowerPath, ".xaml.vb"):
		return "vbnet", true
	case strings.HasSuffix(base, ".d.ts"), strings.HasSuffix(base, ".d.mts"), strings.HasSuffix(base, ".d.cts"):
		return "typescript", true
	case base == "rakefile", base == "gemfile", strings.HasSuffix(base, ".gemspec"):
		return "ruby", true
	case base == "build.nims":
		return "nim", true
	case base == "build.zig":
		return "zig", true
	}
	return "", false
}

const pawnThreshold = 3

func scorePawn(content []byte) int {
	code := stripCommentsAndStrings(content)
	score := 0
	for _, word := range []struct {
		name  string
		value int
	}{
		{"#include", 3}, {"include", 3}, {"public", 2}, {"stock", 2}, {"native", 2},
		{"forward", 2}, {"plugin_init", 3}, {"plugin_precache", 3}, {"register_plugin", 3},
		{"new", 1}, {"enum", 1},
	} {
		if containsCodeWord(code, word.name) {
			score += word.value
		}
	}
	return score
}

func containsCodeWord(code []byte, word string) bool {
	for at := 0; at+len(word) <= len(code); at++ {
		if !strings.EqualFold(string(code[at:at+len(word)]), word) {
			continue
		}
		beforeOK := at == 0 || !isWordByte(code[at-1])
		after := at + len(word)
		afterOK := after == len(code) || !isWordByte(code[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func stripCommentsAndStrings(content []byte) []byte {
	result := append([]byte(nil), content...)
	for at := 0; at < len(result); {
		if result[at] == '/' && at+1 < len(result) && result[at+1] == '/' {
			result[at], result[at+1] = ' ', ' '
			at += 2
			for at < len(result) && result[at] != '\n' {
				result[at] = ' '
				at++
			}
			continue
		}
		if result[at] == '/' && at+1 < len(result) && result[at+1] == '*' {
			result[at], result[at+1] = ' ', ' '
			at += 2
			for at+1 < len(result) && !(result[at] == '*' && result[at+1] == '/') {
				if result[at] != '\n' {
					result[at] = ' '
				}
				at++
			}
			if at+1 < len(result) {
				result[at], result[at+1] = ' ', ' '
				at += 2
			}
			continue
		}
		if result[at] == '\'' || result[at] == '"' {
			quote := result[at]
			result[at] = ' '
			at++
			for at < len(result) {
				if result[at] == '\\' {
					result[at] = ' '
					if at+1 < len(result) {
						result[at+1] = ' '
					}
					at += 2
					continue
				}
				if result[at] == quote {
					result[at] = ' '
					at++
					break
				}
				if result[at] != '\n' {
					result[at] = ' '
				}
				at++
			}
			continue
		}
		at++
	}
	return result
}

func isWordByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}

func containsPHPTag(content []byte) bool {
	lower := strings.ToLower(string(content))
	for offset := 0; offset+1 < len(lower); offset++ {
		if lower[offset:offset+2] != "<?" {
			continue
		}
		rest := lower[offset+2:]
		if strings.HasPrefix(rest, "php") {
			if len(rest) == 3 || !isWordByte(rest[3]) {
				return true
			}
			continue
		}
		if strings.HasPrefix(rest, "xml") {
			continue
		}
		return true
	}
	return false
}

func containsSmartyMarker(content []byte) bool {
	for at := 0; at+1 < len(content); at++ {
		if hasPrefixFold(content, at, "<!--") {
			if end := indexFold(content, at+4, "-->"); end >= 0 {
				at = end + 2
				continue
			}
			return false
		}
		if hasPrefixFold(content, at, "<script") || hasPrefixFold(content, at, "<style") {
			name := "script"
			if !hasPrefixFold(content, at, "<script") {
				name = "style"
			}
			if end := indexFold(content, at+len(name)+1, "</"+name); end >= 0 {
				at = end + len(name) + 1
				continue
			}
			return false
		}
		if content[at] != '{' {
			continue
		}
		if content[at+1] == '{' {
			if end := findDoubleCurlyEnd(content, at); end >= 0 {
				at = end + 1
			} else {
				at = len(content)
			}
			continue
		}
		if content[at+1] == '*' || content[at+1] == '$' {
			return true
		}
		if content[at+1] == '/' && at+2 < len(content) && isSmartyTagName(content[at+2:]) {
			return true
		}
		if isASCIIAlpha(content[at+1]) {
			end := at + 1
			for end < len(content) && (isASCIIAlpha(content[end]) || content[end] == '_') {
				end++
			}
			name := strings.ToLower(string(content[at+1 : end]))
			switch name {
			case "block", "extends", "include", "function", "foreach", "section", "if", "for", "while", "capture", "call", "elseif", "else", "ldelim", "rdelim":
				if end == len(content) || content[end] == '}' || content[end] == ' ' || content[end] == '\t' || content[end] == '\r' || content[end] == '\n' {
					return true
				}
			}
		}
	}
	return false
}

func findDoubleCurlyEnd(content []byte, at int) int {
	quote := byte(0)
	escaped := false
	for index := at + 2; index+1 < len(content); index++ {
		if escaped {
			escaped = false
			continue
		}
		if quote != 0 {
			if content[index] == '\\' {
				escaped = true
			} else if content[index] == quote {
				quote = 0
			}
			continue
		}
		if content[index] == '\'' || content[index] == '"' {
			quote = content[index]
			continue
		}
		if content[index] == '}' && content[index+1] == '}' {
			return index
		}
	}
	return -1
}

func isSmartyTagName(content []byte) bool {
	end := 0
	for end < len(content) && isASCIIAlpha(content[end]) {
		end++
	}
	if end == 0 {
		return false
	}
	switch strings.ToLower(string(content[:end])) {
	case "block", "extends", "include", "function", "foreach", "section", "if", "for", "while", "capture", "call", "elseif", "else", "ldelim", "rdelim":
		return true
	}
	return false
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func hasPrefixFold(source []byte, at int, value string) bool {
	return at >= 0 && at+len(value) <= len(source) && strings.EqualFold(string(source[at:at+len(value)]), value)
}

func indexFold(source []byte, at int, value string) int {
	for index := at; index+len(value) <= len(source); index++ {
		if hasPrefixFold(source, index, value) {
			return index
		}
	}
	return -1
}

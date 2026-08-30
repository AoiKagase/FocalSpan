package projectmeta

import (
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type staticProvider struct{}

func DefaultProviders() []Provider { return []Provider{staticProvider{}} }

func (staticProvider) Supports(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if base == "go.mod" || base == "go.work" || base == "cargo.toml" || base == "package.json" || base == "tsconfig.json" || base == "jsconfig.json" || base == "composer.json" || base == "pyproject.toml" || base == "setup.cfg" || base == "gemfile" || strings.HasSuffix(base, ".vbp") || base == "build.zig.zon" {
		return true
	}
	ext := strings.ToLower(filepath.Ext(base))
	return ext == ".gemspec" || ext == ".rockspec" || ext == ".nimble" || ext == ".csproj" || ext == ".vbproj"
}

func ParseFile(ctx context.Context, root string, file model.SourceFile) ([]Fact, []model.Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	for _, provider := range DefaultProviders() {
		if provider.Supports(file.Path) {
			return provider.Parse(ctx, root, file)
		}
	}
	return []Fact{}, []model.Diagnostic{}, nil
}

func Discover(ctx context.Context, root string, files []model.SourceFile) ([]Fact, []model.Diagnostic, error) {
	facts := make([]Fact, 0)
	diagnostics := make([]model.Diagnostic, 0)
	ordered := append([]model.SourceFile(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for _, file := range ordered {
		if err := ctx.Err(); err != nil {
			return facts, diagnostics, err
		}
		parsed, parsedDiagnostics, err := ParseFile(ctx, root, file)
		if err != nil {
			return facts, diagnostics, err
		}
		facts = append(facts, parsed...)
		diagnostics = append(diagnostics, parsedDiagnostics...)
	}
	sortFacts(facts)
	return facts, diagnostics, nil
}

func (staticProvider) Parse(ctx context.Context, _ string, file model.SourceFile) ([]Fact, []model.Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	text := string(file.Content)
	base := strings.ToLower(filepath.Base(file.Path))
	facts := make([]Fact, 0)
	add := func(kind, name, target string, confidence float64) {
		facts = append(facts, Fact{SourcePath: file.Path, Kind: kind, Name: name, Target: target, Confidence: confidence})
	}
	switch {
	case base == "go.mod" || base == "go.work":
		parseGoManifest(text, base, add)
	case base == "cargo.toml":
		parseCargo(text, add)
	case base == "package.json" || base == "tsconfig.json" || base == "jsconfig.json" || base == "composer.json":
		if err := parseJSONMetadata(text, base, add); err != nil {
			return facts, []model.Diagnostic{{FilePath: file.Path, Level: "warning", Code: "metadata_json_invalid", Message: "metadata JSON could not be parsed"}}, nil
		}
	case base == "pyproject.toml" || base == "setup.cfg":
		parsePythonManifest(text, base, add)
	case base == "gemfile" || strings.HasSuffix(base, ".gemspec"):
		parseRubyManifest(text, add)
	case strings.HasSuffix(base, ".rockspec"):
		parseLuaManifest(text, add)
	case strings.HasSuffix(base, ".vbp"):
		parseVB6Manifest(text, add)
	case strings.HasSuffix(base, ".nimble"):
		parseNimManifest(text, add)
	case base == "build.zig.zon":
		parseZigManifest(text, add)
	case strings.HasSuffix(base, ".csproj") || strings.HasSuffix(base, ".vbproj"):
		parseDotNetManifest(text, add)
	}
	sortFacts(facts)
	return facts, nil, nil
}

func parseGoManifest(text, base string, add func(string, string, string, float64)) {
	inUseBlock := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		fields := strings.Fields(trimmed)
		if base == "go.work" && trimmed == "use (" {
			inUseBlock = true
			continue
		}
		if base == "go.work" && inUseBlock && trimmed == ")" {
			inUseBlock = false
			continue
		}
		if len(fields) >= 2 && fields[0] == "module" {
			add("module", "", fields[1], 1)
		}
		if base == "go.work" && inUseBlock && len(fields) == 1 {
			add("workspace-use", "", fields[0], .9)
		}
		if len(fields) >= 2 && fields[0] == "use" && fields[1] != "(" {
			add("workspace-use", "", strings.Trim(fields[1], "()"), .9)
		}
		if index := strings.Index(trimmed, "=>"); index >= 0 {
			targetFields := strings.Fields(strings.TrimSpace(trimmed[index+2:]))
			if len(targetFields) > 0 {
				add("replace", strings.TrimSpace(strings.TrimPrefix(trimmed, "replace")), targetFields[0], .9)
			}
		}
	}
	_ = base
}

func parseCargo(text string, add func(string, string, string, float64)) {
	section := ""
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[]")
		}
		if value := quotedAssignment(trimmed, "name"); value != "" {
			kind := "package"
			if section == "lib" || section == "[lib" {
				kind = "lib-path"
			}
			add(kind, "", value, .9)
		}
		if value := quotedAssignment(trimmed, "path"); value != "" {
			add("path-dependency", "", value, .8)
		}
		if strings.Contains(trimmed, "members") {
			for _, value := range quotedValues(trimmed) {
				add("workspace-member", "", value, .8)
			}
		}
	}
}

func parseJSONMetadata(text, base string, add func(string, string, string, float64)) error {
	var value map[string]any
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return err
	}
	if name, ok := value["name"].(string); ok {
		kind := "package"
		if base == "composer.json" {
			kind = "package"
		}
		add(kind, "", name, 1)
	}
	for _, key := range []string{"type", "main", "module", "types", "baseUrl"} {
		if item, ok := value[key].(string); ok {
			add(key, "", item, .9)
		}
	}
	if workspaces, ok := value["workspaces"].([]any); ok {
		for _, item := range workspaces {
			if target, ok := item.(string); ok {
				add("workspace", "", target, .8)
			}
		}
	}
	if paths, ok := value["paths"].(map[string]any); ok {
		for name, raw := range paths {
			if values, ok := raw.([]any); ok {
				for _, item := range values {
					if target, ok := item.(string); ok {
						add("path-alias", name, target, .8)
					}
				}
			}
		}
	}
	if autoload, ok := value["autoload"].(map[string]any); ok {
		parseComposerAutoload(autoload, add)
	}
	if exports, ok := value["exports"].(map[string]any); ok {
		for name, raw := range exports {
			if target, ok := raw.(string); ok {
				add("export", name, target, .8)
			}
		}
	}
	return nil
}

func parseComposerAutoload(value map[string]any, add func(string, string, string, float64)) {
	for _, key := range []string{"psr-4", "psr-0", "classmap", "files"} {
		if raw, ok := value[key]; ok {
			switch item := raw.(type) {
			case map[string]any:
				for name, target := range item {
					if target, ok := target.(string); ok {
						add(key, name, target, .9)
					}
				}
			case []any:
				for _, target := range item {
					if target, ok := target.(string); ok {
						add(key, "", target, .8)
					}
				}
			}
		}
	}
}

func parseDotNetManifest(text string, add func(string, string, string, float64)) {
	patterns := []struct{ kind, tag string }{{"namespace", "RootNamespace"}, {"assembly", "AssemblyName"}, {"project-reference", "ProjectReference"}, {"compile", "Compile"}, {"page", "Page"}, {"application-definition", "ApplicationDefinition"}, {"embedded-resource", "EmbeddedResource"}, {"dependent-upon", "DependentUpon"}}
	for _, pattern := range patterns {
		tag := regexp.QuoteMeta(pattern.tag)
		for _, match := range regexp.MustCompile(`(?is)<`+tag+`\b[^>]*>([^<]+)</`+tag+`>|<`+tag+`\b[^>]*\bInclude\s*=\s*"([^"]+)"[^>]*/?>`).FindAllStringSubmatch(text, -1) {
			target := match[1]
			if target == "" {
				target = match[2]
			}
			if target != "" {
				add(pattern.kind, "", strings.TrimSpace(target), .9)
			}
		}
	}
}

func parsePythonManifest(text, base string, add func(string, string, string, float64)) {
	pendingPackageDir := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if value := assignmentValue(trimmed, "name"); value != "" {
			add("package", "", value, .9)
		}
		if value := assignmentValue(trimmed, "where"); value != "" {
			add("package-dir", "", value, .8)
		}
		if strings.HasPrefix(strings.ToLower(trimmed), "package_dir") {
			pendingPackageDir = true
			if value := assignmentValue(trimmed, "package_dir"); value != "" {
				add("package-dir", "", value, .8)
				pendingPackageDir = false
			}
			continue
		}
		if pendingPackageDir && strings.HasPrefix(trimmed, "=") {
			if value := strings.TrimSpace(strings.TrimPrefix(trimmed, "=")); value != "" {
				add("package-dir", "", strings.Trim(value, "\"'"), .8)
			}
			pendingPackageDir = false
		}
	}
	_ = base
}

func parseRubyManifest(text string, add func(string, string, string, float64)) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "gem ") {
			if target := quotedValueAfterKey(trimmed, "path"); target != "" {
				add("gem-path", "", target, .8)
			}
		}
		if strings.HasPrefix(trimmed, "require_relative") {
			if target := firstQuoted(trimmed); target != "" {
				add("require-relative", "", target, .9)
			}
		}
		if strings.Contains(trimmed, "s.name") || strings.HasPrefix(trimmed, "name =") {
			if target := firstQuoted(trimmed); target != "" {
				add("gem", "", target, .9)
			}
		}
		if strings.Contains(trimmed, "require_paths") {
			if target := firstQuoted(trimmed); target != "" {
				add("require-path", "", target, .8)
			}
		}
	}
}

func parseLuaManifest(text string, add func(string, string, string, float64)) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package") || strings.HasPrefix(trimmed, "source") {
			if target := firstQuoted(trimmed); target != "" {
				add("package", "", target, .8)
			}
		}
		if strings.Contains(trimmed, "module") || strings.Contains(trimmed, "dir") {
			target := quotedValueAfterKey(trimmed, "dir")
			if target == "" {
				target = firstQuoted(trimmed)
			}
			if target != "" {
				add("path", "", target, .7)
			}
		}
	}
}

func parseVB6Manifest(text string, add func(string, string, string, float64)) {
	for _, line := range strings.Split(text, "\n") {
		if fields := strings.SplitN(strings.TrimSpace(line), "=", 2); len(fields) == 2 && map[string]bool{"form": true, "class": true, "module": true, "usercontrol": true}[strings.ToLower(fields[0])] {
			add("component", fields[0], strings.TrimSpace(fields[1]), .9)
		}
	}
}

func parseNimManifest(text string, add func(string, string, string, float64)) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name") || strings.HasPrefix(trimmed, "srcDir") {
			if target := firstQuoted(trimmed); target != "" {
				kind := "package"
				if strings.HasPrefix(trimmed, "srcDir") {
					kind = "src-dir"
				}
				add(kind, "", target, .8)
			}
		}
		if strings.HasPrefix(trimmed, "requires") {
			for _, target := range quotedValues(trimmed) {
				if strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/") {
					add("path-dependency", "", target, .7)
				}
			}
		}
	}
}

func parseZigManifest(text string, add func(string, string, string, float64)) {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, ".name") || strings.Contains(line, ".path") {
			if target := firstQuoted(line); target != "" {
				kind := "package"
				if strings.Contains(line, ".path") {
					kind = "path"
				}
				add(kind, "", target, .8)
			}
		}
	}
}

func quotedAssignment(line, key string) string {
	if key != "" {
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(key) + `\s*=\s*["']([^"']+)["']`)
		if match := pattern.FindStringSubmatch(line); len(match) == 2 {
			return match[1]
		}
	}
	return firstQuoted(line)
}

func assignmentValue(line, key string) string {
	pattern := regexp.MustCompile(`(?i)^\s*` + regexp.QuoteMeta(key) + `\s*=\s*(?:["']([^"']+)["']|([^#;\s]+))`)
	match := pattern.FindStringSubmatch(line)
	if len(match) == 3 {
		if match[1] != "" {
			return match[1]
		}
		return match[2]
	}
	return ""
}

func quotedValueAfterKey(line, key string) string {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(key) + `\s*(?::|=)\s*["']([^"']+)["']`)
	match := pattern.FindStringSubmatch(line)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func quotedValues(line string) []string {
	matches := regexp.MustCompile(`["']([^"']+)["']`).FindAllStringSubmatch(line, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 2 {
			values = append(values, match[1])
		}
	}
	return values
}

func firstQuoted(line string) string {
	match := regexp.MustCompile(`["']([^"']+)["']`).FindStringSubmatch(line)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func sortFacts(facts []Fact) {
	sort.SliceStable(facts, func(i, j int) bool {
		if facts[i].SourcePath != facts[j].SourcePath {
			return facts[i].SourcePath < facts[j].SourcePath
		}
		if facts[i].Kind != facts[j].Kind {
			return facts[i].Kind < facts[j].Kind
		}
		if facts[i].Name != facts[j].Name {
			return facts[i].Name < facts[j].Name
		}
		return facts[i].Target < facts[j].Target
	})
}

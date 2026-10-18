package projectmeta

import (
	"path"
	"strings"
)

// ApplicablePathAliases selects the nearest configuration, including an empty
// or invalid one. The latter is a boundary, never implicit inheritance.
func ApplicablePathAliases(sourcePath string, facts []Fact) []Fact {
	sourcePath = path.Clean(strings.ReplaceAll(sourcePath, `\`, "/"))
	selected, selectedDir := "", ""
	for _, f := range facts {
		if f.Kind != "project-config" && f.Kind != "path-alias" {
			continue
		}
		config := path.Clean(strings.ReplaceAll(f.SourcePath, `\`, "/"))
		base := path.Base(config)
		if base != "tsconfig.json" && base != "jsconfig.json" {
			continue
		}
		dir := path.Dir(config)
		if dir != "." && sourcePath != dir && !strings.HasPrefix(sourcePath, dir+"/") {
			continue
		}
		depth, selectedDepth := len(dir), len(selectedDir)
		if dir == "." {
			depth = 0
		}
		if selectedDir == "." {
			selectedDepth = 0
		}
		if selected == "" || depth > selectedDepth || dir == selectedDir && base == "tsconfig.json" {
			selected, selectedDir = config, dir
		}
	}
	result := make([]Fact, 0)
	for _, f := range facts {
		if f.Kind == "path-alias" && path.Clean(strings.ReplaceAll(f.SourcePath, `\`, "/")) == selected {
			result = append(result, f)
		}
	}
	return result
}

package testutil

import (
	"sort"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

// SourceFiles turns deterministic inline fixture content into SourceFiles.
func SourceFiles(t testing.TB, files map[string]struct{ Language, Content string }) []model.SourceFile {
	t.Helper()
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]model.SourceFile, 0, len(paths))
	for _, path := range paths {
		fixture := files[path]
		result = append(result, model.SourceFile{Path: path, Language: fixture.Language, Content: []byte(fixture.Content)})
	}
	return result
}

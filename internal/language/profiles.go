package language

import "sort"

var knownProfiles = map[string]struct{}{
	"c": {}, "cpp": {}, "csharp": {}, "config": {}, "dotnet-resource": {},
	"go": {}, "javascript": {}, "lua": {}, "markdown": {}, "nim": {},
	"pawn": {}, "php": {}, "powershell": {}, "python": {}, "ruby": {},
	"rust": {}, "shell": {}, "smarty": {}, "template": {}, "text": {},
	"typescript": {}, "vb6": {}, "vb6-project": {}, "vbnet": {}, "xaml": {},
	"zig": {},
}

// KnownLanguages returns the stable, sorted set of language identifiers accepted
// by the detector and language override configuration.
func KnownLanguages() []string {
	result := make([]string, 0, len(knownProfiles))
	for name := range knownProfiles {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func IsKnown(language string) bool {
	_, ok := knownProfiles[language]
	return ok
}

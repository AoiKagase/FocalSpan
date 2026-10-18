package linker

import "github.com/focalspan/focalspan/internal/model"

func resolveCPPModuleImport(target string, symbols []model.Symbol) []model.Symbol {
	var provider []model.Symbol
	for _, s := range symbols {
		if s.Language == "cpp" && s.Kind == "module" && s.QualifiedName == target {
			provider = append(provider, s)
		}
	}
	if len(provider) != 1 {
		return nil
	}
	for _, s := range symbols {
		if s.Handle == provider[0].ParentHandle && s.Kind == "translation_unit" && s.FilePath == provider[0].FilePath {
			return []model.Symbol{s}
		}
	}
	return nil
}

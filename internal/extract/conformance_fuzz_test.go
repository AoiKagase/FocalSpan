package extract_test

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/extract/php"
	"github.com/focalspan/focalspan/internal/extract/template"
	"github.com/focalspan/focalspan/internal/extract/testutil"
	"github.com/focalspan/focalspan/internal/model"
)

func FuzzExistingExtractorInvariantSeeds(f *testing.F) {
	f.Add(0, []byte(`const char *raw = R"tag({ not a brace })tag";`))
	f.Add(1, []byte(`var value = $"""{interpolation}""";`))
	f.Add(2, []byte("const value = `template ${call({ brace: true })}`"))
	f.Add(3, []byte("<?php\n$value = <<<TXT\nheredoc\nTXT;\n"))
	f.Add(4, []byte("{literal}{{ opaque }}{/literal}"))
	f.Fuzz(func(t *testing.T, which int, content []byte) {
		factories := []struct {
			path, language string
			extractor      extract.Extractor
		}{
			{"fuzz.cpp", "cpp", cpp.NewExtractor()},
			{"fuzz.cs", "csharp", csharp.NewExtractor()},
			{"fuzz.ts", "typescript", jsts.NewExtractor()},
			{"fuzz.php", "php", php.NewExtractor()},
			{"fuzz.tpl", "smarty", template.NewExtractor()},
		}
		if which < 0 {
			which = -which
		}
		fixture := factories[which%len(factories)]
		got, err := fixture.extractor.Extract(context.Background(), model.SourceFile{Path: fixture.path, Language: fixture.language, Content: content})
		if err != nil {
			return
		}
		testutil.AssertExtraction(t, model.SourceFile{Path: fixture.path, Language: fixture.language, Content: content}, got)
	})
}

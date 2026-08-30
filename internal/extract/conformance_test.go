package extract_test

import (
	"context"
	"testing"

	"github.com/focalspan/focalspan/internal/extract"
	"github.com/focalspan/focalspan/internal/extract/cpp"
	"github.com/focalspan/focalspan/internal/extract/csharp"
	"github.com/focalspan/focalspan/internal/extract/goast"
	"github.com/focalspan/focalspan/internal/extract/jsts"
	"github.com/focalspan/focalspan/internal/extract/php"
	"github.com/focalspan/focalspan/internal/extract/template"
	"github.com/focalspan/focalspan/internal/extract/testutil"
	"github.com/focalspan/focalspan/internal/model"
)

func TestExistingExtractorsSatisfySharedConformance(t *testing.T) {
	fixtures := []struct {
		name, path, language, content string
		extractor                     extract.Extractor
	}{
		{"go", "auth/service.go", "go", "package auth\n\nfunc ValidateToken() error { return nil }\n", goast.NewExtractor()},
		{"php", "src/Auth.php", "php", "<?php\nclass Auth { public function validateToken(): bool { return true; } }\n", php.NewExtractor()},
		{"cpp", "src/auth.cpp", "cpp", "namespace auth { class Service { public: bool validateToken() { return true; } }; }\n", cpp.NewExtractor()},
		{"csharp", "src/Auth.cs", "csharp", "namespace Auth { public class Service { public bool ValidateToken() { return true; } } }\n", csharp.NewExtractor()},
		{"jsts", "src/auth.ts", "typescript", "export function validateToken(): boolean { return true; }\n", jsts.NewExtractor()},
		{"template", "views/auth.tpl", "smarty", "{extends file=\"layout.tpl\"}\n{block name=\"content\"}token{/block}\n", template.NewExtractor()},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			file := model.SourceFile{Path: fixture.path, Language: fixture.language, Content: []byte(fixture.content)}
			got, err := fixture.extractor.Extract(context.Background(), file)
			if err != nil {
				t.Fatal(err)
			}
			testutil.AssertExtraction(t, file, got)
			testutil.AssertNoSourceDuplication(t, file, got, 4)
			testutil.AssertDeterministic(t, fixture.extractor, file)
		})
	}
}

func TestExistingExtractorRecoverySeedsDoNotReturnInvalidExtraction(t *testing.T) {
	fixtures := []struct {
		name, path, language, content string
		extractor                     extract.Extractor
	}{
		{"cpp-raw-string", "broken.cpp", "cpp", "const char *raw = R\"tag({ not a brace })tag\"; void Broken( {\n", cpp.NewExtractor()},
		{"csharp-interpolated-raw", "broken.cs", "csharp", "class Broken { string value = $\"\"\"{ missing }\"\"\";\n", csharp.NewExtractor()},
		{"jsts-template", "broken.ts", "typescript", "const value = `template ${call({ brace: true })}`; function Broken( {\n", jsts.NewExtractor()},
		{"php-heredoc", "broken.php", "php", "<?php\n$value = <<<TXT\nmissing terminator\n", php.NewExtractor()},
		{"smarty-literal", "broken.tpl", "smarty", "{literal}{{ opaque }{/literal}\n", template.NewExtractor()},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			file := model.SourceFile{Path: fixture.path, Language: fixture.language, Content: []byte(fixture.content)}
			got, err := fixture.extractor.Extract(context.Background(), file)
			if err != nil {
				return
			}
			testutil.AssertExtraction(t, file, got)
		})
	}
}

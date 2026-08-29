package query

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeMixedJapaneseAndIdentifier(t *testing.T) {
	got := Normalize(`ValidateToken の呼び出し元を探して`)
	if !containsString(got.Identifiers, "ValidateToken") {
		t.Fatalf("identifiers=%v, want ValidateToken", got.Identifiers)
	}
	if !containsString(got.Words, "validatetoken") {
		t.Fatalf("words=%v, want validatetoken", got.Words)
	}
	if !containsString(got.UnicodeRuns, "呼び出し元") {
		t.Fatalf("unicode runs=%v, want 呼び出し元", got.UnicodeRuns)
	}
}

func TestNormalizeQualifiedSymbolsAndPaths(t *testing.T) {
	got := Normalize(`App\Auth\TokenService::ValidateToken in src/Auth/TokenService.php`)
	if !containsString(got.Identifiers, `App\Auth\TokenService::ValidateToken`) {
		t.Fatalf("identifiers=%v, want qualified PHP symbol", got.Identifiers)
	}
	if !containsString(got.Symbols, `App\Auth\TokenService::ValidateToken`) {
		t.Fatalf("symbols=%v, want qualified PHP symbol", got.Symbols)
	}
	if !containsString(got.Paths, "src/Auth/TokenService.php") {
		t.Fatalf("paths=%v, want normalized path", got.Paths)
	}
}

func TestNormalizeIsDeterministicAndDeduplicated(t *testing.T) {
	first := Normalize(`ValidateToken ValidateToken validate_token`)
	second := Normalize(`ValidateToken ValidateToken validate_token`)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("normalization is not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if countString(first.Identifiers, "ValidateToken") != 1 {
		t.Fatalf("identifiers=%v, want one ValidateToken", first.Identifiers)
	}
	if countString(first.Words, "validate") != 1 || countString(first.Words, "token") != 1 {
		t.Fatalf("words=%v, want deduplicated identifier parts", first.Words)
	}
}

func TestNormalizeRecognizesPhrasesAndIdentifierStyles(t *testing.T) {
	got := Normalize(`"expired token" snake_case camelCase PascalCase ns::Thing CSharp.Type`)
	if !containsString(got.Phrases, "expired token") {
		t.Fatalf("phrases=%v", got.Phrases)
	}
	for _, want := range []string{"snake_case", "camelCase", "PascalCase", "ns::Thing", "CSharp.Type"} {
		if !containsString(got.Identifiers, want) {
			t.Fatalf("identifiers=%v, want %q", got.Identifiers, want)
		}
	}
	for _, want := range []string{"snake", "case", "camel", "pascal", "thing"} {
		if !containsString(got.Words, want) {
			t.Fatalf("words=%v, want %q", got.Words, want)
		}
	}
}

func TestNormalizeHandlesUnicodePunctuationNULAndBounds(t *testing.T) {
	long := strings.Repeat("あ", 129)
	got := Normalize("  日本語、emoji🚀\x00" + long)
	if containsString(got.Words, "") || containsString(got.UnicodeRuns, "") {
		t.Fatalf("empty terms leaked: %+v", got)
	}
	for _, value := range append(append([]string{}, got.Words...), got.UnicodeRuns...) {
		if strings.ContainsRune(value, '\x00') {
			t.Fatalf("NUL leaked in term %q", value)
		}
		if !utf8.ValidString(value) {
			t.Fatalf("invalid UTF-8 term %q", value)
		}
	}
	for _, value := range got.UnicodeRuns {
		if utf8.RuneCountInString(value) > maxTokenRunes {
			t.Fatalf("unicode run exceeds bound: %d", utf8.RuneCountInString(value))
		}
	}
}

func TestNormalizeEmptyQueryIsEmpty(t *testing.T) {
	got := Normalize(" \t\n")
	if !reflect.DeepEqual(got, Terms{}) {
		t.Fatalf("got=%+v, want empty terms", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

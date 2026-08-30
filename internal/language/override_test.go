package language

import "testing"

func TestDetectUsesMostSpecificOverrideIndependentOfMapIteration(t *testing.T) {
	overrides := map[string]string{
		"**/*.go":         "python",
		"src/*.go":        "ruby",
		"src/???.go":      "vb6",
		"legacy/**/*.tpl": "smarty",
	}
	if got := Detect("src/app.go", nil, overrides); got.Language != "ruby" {
		t.Fatalf("same-specificity override=%+v, want ruby from lexicographically first pattern", got)
	}
	if got := Detect("lib/long.go", nil, overrides); got.Language != "python" {
		t.Fatalf("non-matching fixed-width override=%+v, want python", got)
	}
	if got := Detect("legacy/templates/page.tpl", []byte("<p>plain</p>"), overrides); got.Language != "smarty" {
		t.Fatalf("explicit override=%+v, want smarty", got)
	}
}

func TestDetectOverrideWinsOverContentAwareDetection(t *testing.T) {
	overrides := map[string]string{"templates/**/*.tpl": "php"}
	got := Detect("templates/page.tpl", []byte(`{block name="content"}{/block}`), overrides)
	if got.Language != "php" || got.Reason != "override" {
		t.Fatalf("override result=%+v, want php/override", got)
	}
}

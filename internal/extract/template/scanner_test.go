package template

import (
	"context"
	"strings"
	"testing"
)

func TestScanSmartyCommentsLiteralAndEmbeddedRegions(t *testing.T) {
	source := "前置\r\n{* {block name=\"fake\"} *}\r\n{literal}\r\n{$notSmarty}\r\n{/literal}\r\n<script type=\"module\">\r\nfunction validateLogin() { return \"</script>\"; }\r\n</script>\r\n<style>.login { color: red; }</style>"
	regions, err := Scan(context.Background(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if hasRegionKind(regions, KindSmartyTag) {
		t.Fatalf("literal/comment fake tags were scanned as Smarty: %+v", regions)
	}
	if !hasRegionKind(regions, KindSmartyComment) || !hasRegionKind(regions, KindSmartyLiteral) {
		t.Fatalf("regions=%+v", regions)
	}
	if !hasRegionKind(regions, KindScriptBody) || !hasRegionKind(regions, KindStyleBody) {
		t.Fatalf("regions=%+v", regions)
	}
	for _, region := range regions {
		if region.StartByte < 0 || region.EndByte < region.StartByte || region.EndByte > len(source) {
			t.Fatalf("invalid span=%+v", region)
		}
		if string(region.Content) != source[region.StartByte:region.EndByte] {
			t.Fatalf("content does not match source for %+v", region)
		}
	}
}

func TestScanQuotedHTMLAttributeAndDataScript(t *testing.T) {
	source := `<div data-value="a > b {$title}"></div><script type="application/ld+json">{"name":"x"}</script><script src="/assets/app.js"></script>`
	regions, err := Scan(context.Background(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if !hasRegionKind(regions, KindDataScript) {
		t.Fatalf("data script missing: %+v", regions)
	}
	if !hasRegionKind(regions, KindScriptOpen) || !hasRegionKind(regions, KindScriptClose) {
		t.Fatalf("script tag regions missing: %+v", regions)
	}
	for _, region := range regions {
		if region.Kind == KindSmartyTag && strings.Contains(string(region.Content), "title") {
			t.Fatalf("attribute expression should not be mistaken for a Smarty tag: %+v", region)
		}
	}
}

func TestScanDoubleCurlyTemplateTagAsOpaqueRegion(t *testing.T) {
	source := `<div>{{ "not }} the end" }}{{block}}</div>`
	regions, err := Scan(context.Background(), []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	for _, region := range regions {
		if region.Kind == KindTemplateTag {
			tags = append(tags, string(region.Content))
		}
		if region.Kind == KindSmartyTag {
			t.Fatalf("double-curly tag was split into Smarty syntax: %+v", regions)
		}
	}
	if len(tags) != 2 || tags[0] != `{{ "not }} the end" }}` || tags[1] != "{{block}}" {
		t.Fatalf("double-curly tag missing: %+v", regions)
	}
}

func TestScanUnclosedDoubleCurlyTemplateTagRecovers(t *testing.T) {
	_, diagnostics, err := scan(context.Background(), []byte("{{ user.name"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0] != "template_unclosed_template_tag" {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}

func TestScanCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Scan(ctx, []byte(strings.Repeat("x", 100))); err == nil {
		t.Fatal("expected cancellation")
	}
}

func hasRegionKind(regions []Region, want Kind) bool {
	for _, region := range regions {
		if region.Kind == want {
			return true
		}
	}
	return false
}

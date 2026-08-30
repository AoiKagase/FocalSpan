package template

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractorBuildsTemplateStructureRelationsAndEmbeddedJavaScript(t *testing.T) {
	source := "<!doctype html>\n{extends file=\"../layouts/base.tpl\"}\n{block name=\"content\"}\n<form>{$user.name}</form>\n{block name=\"inner\"}inner{/block}\n{/block}\n<script type=\"module\">\nfunction validateLogin(form) { return form; }\nconst arrow = (value) => value;\n</script>\n<style>.form { color: red; }</style>"
	file := model.SourceFile{Path: "pages/login.tpl", Language: "smarty", Content: []byte(source)}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	root := findSymbol(got.Symbols, "pages/login.tpl", "template")
	content := findSymbol(got.Symbols, "content", "template-block")
	inner := findSymbol(got.Symbols, "inner", "template-block")
	if root.Handle == "" || content.Handle == "" || inner.Handle == "" {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	if inner.ParentHandle != content.Handle {
		t.Fatalf("inner parent=%q, want %q", inner.ParentHandle, content.Handle)
	}
	if !hasRelation(got.Relations, root.Handle, content.Handle, "contains") || !hasRelation(got.Relations, content.Handle, inner.Handle, "contains") {
		t.Fatalf("contains relations=%+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, root.Handle, "layouts/base.tpl", "imports") {
		t.Fatalf("extends relation=%+v", got.Relations)
	}
	validate := findSymbol(got.Symbols, "validateLogin", "function")
	if validate.Handle == "" || validate.ParentHandle != root.Handle || validate.StartLine != 8 {
		t.Fatalf("embedded symbol=%+v", validate)
	}
	if !hasChunkKind(got.Chunks, "style") || !hasChunkKind(got.Chunks, "template-outline") {
		t.Fatalf("chunks=%+v", got.Chunks)
	}
	for _, chunk := range got.Chunks {
		if chunk.Kind == "template-outline" && strings.Contains(chunk.Content, "<form>{$user.name}</form>") {
			t.Fatal("template outline contains a full named body")
		}
	}
}

func TestExtractorRecoversMalformedTemplateAndIgnoresCommentsAndLiteral(t *testing.T) {
	source := "{block name=\"good\"}ok{/block}\n{* {block name=\"fake\"} *}\n{literal}{$notASymbol}{/literal}\n{block name=\"broken\"}tail"
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "broken.tpl", Language: "smarty", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	if findSymbol(got.Symbols, "fake", "template-block").Handle != "" || findSymbol(got.Symbols, "notASymbol", "template-block").Handle != "" {
		t.Fatalf("false symbols=%+v", got.Symbols)
	}
	if findSymbol(got.Symbols, "good", "template-block").Handle == "" || len(got.Diagnostics) == 0 {
		t.Fatalf("recovery lost symbols or diagnostics: %+v", got)
	}
}

func TestExtractorResolvesStaticCallsAndKeepsDynamicTargetsUnresolved(t *testing.T) {
	source := `{function "renderAlert"}{/function}{call "renderAlert"}{call name=$functionName}{include file=$currentTemplate}`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "components/alerts.tpl", Language: "smarty", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	function := findSymbol(got.Symbols, "renderAlert", "template-function")
	if function.Handle == "" {
		t.Fatalf("symbols=%+v", got.Symbols)
	}
	if !hasRelation(got.Relations, findSymbol(got.Symbols, "components/alerts.tpl", "template").Handle, function.Handle, "calls") {
		t.Fatalf("static call was not resolved: %+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, findSymbol(got.Symbols, "components/alerts.tpl", "template").Handle, "$functionName", "calls") {
		t.Fatalf("dynamic call was resolved unexpectedly: %+v", got.Relations)
	}
	if !hasUnresolved(got.Relations, findSymbol(got.Symbols, "components/alerts.tpl", "template").Handle, "$currentTemplate", "imports") {
		t.Fatalf("dynamic include was resolved unexpectedly: %+v", got.Relations)
	}
}

func TestExtractorRemapsCRLFAndUTF8EmbeddedOffsetsAndAvoidsScriptCollisions(t *testing.T) {
	source := "日本語\r\n<script>\r\nfunction same() { return 1; }\r\n</script>\r\n<script>\r\nfunction same() { return 2; }\r\n</script>"
	file := model.SourceFile{Path: "scripts.tpl", Language: "template", Content: []byte(source)}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	var matches []model.Symbol
	for _, symbol := range got.Symbols {
		if symbol.Name == "same" {
			matches = append(matches, symbol)
		}
	}
	if len(matches) != 2 || matches[0].Handle == matches[1].Handle {
		t.Fatalf("same-name scripts collided: %+v", matches)
	}
	wantStart := strings.Index(source, "function same")
	for _, symbol := range matches {
		if symbol.StartByte < wantStart || !strings.Contains(source[symbol.StartByte:symbol.EndByte], "function same") || symbol.StartLine < 3 {
			t.Fatalf("offset was not remapped to original source: %+v", symbol)
		}
	}
}

func TestExtractorKeepsEmbeddedPHPSearchable(t *testing.T) {
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "mixed.tpl", Language: "smarty", Content: []byte(`{block name="content"}<?php echo $title; ?>{/block}`)})
	if err != nil {
		t.Fatal(err)
	}
	if !hasChunkKind(got.Chunks, "embedded-php") {
		t.Fatalf("chunks=%+v", got.Chunks)
	}
}

func TestExtractorKeepsDoubleCurlyTagsSearchableWithoutSmartySymbols(t *testing.T) {
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "double.tpl", Language: "template", Content: []byte(`<h1>{{ page.title }}</h1>`)})
	if err != nil {
		t.Fatal(err)
	}
	for _, symbol := range got.Symbols {
		if symbol.Kind != "template" {
			t.Fatalf("opaque double-curly tag became structural symbol: %+v", symbol)
		}
	}
	found := false
	for _, chunk := range got.Chunks {
		if strings.Contains(chunk.Content, "{{ page.title }}") {
			found = true
		}
	}
	if !found {
		t.Fatalf("double-curly source was not retained: %+v", got.Chunks)
	}
}

func TestExtractorCoversCaptureMultipleScriptsAndOpaquePluginTags(t *testing.T) {
	source := `{capture name="notice"}<p>{$message}</p>{/capture}
{custom_widget value=$message}
<script type="text/typescript">
export function validateSettings(value: string): boolean { return value.length > 0; }
</script>
<script>
function secondHandler(value) { return value; }
</script>
{literal}{$notSmarty}{/literal}`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "components/settings.tpl", Language: "smarty", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	if findSymbol(got.Symbols, "notice", "template-capture").Handle == "" {
		t.Fatalf("capture symbol missing: %+v", got.Symbols)
	}
	if findSymbol(got.Symbols, "custom_widget", "template-custom_widget").Handle != "" {
		t.Fatalf("custom plugin tag became a structural symbol: %+v", got.Symbols)
	}
	if !hasChunkKind(got.Chunks, "literal") || !hasChunkKind(got.Chunks, "static") {
		t.Fatalf("literal/static coverage missing: %+v", got.Chunks)
	}
	for _, name := range []string{"validateSettings", "secondHandler"} {
		found := false
		for _, symbol := range got.Symbols {
			if symbol.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("embedded script symbol %q missing: %+v", name, got.Symbols)
		}
	}
}

func TestExtractorTreatsVerbatimAsOpaqueLiteral(t *testing.T) {
	source := `{verbatim}{block name="not-a-block"}{$raw}{/block}{/verbatim}`
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "verbatim.tpl", Language: "smarty", Content: []byte(source)})
	if err != nil {
		t.Fatal(err)
	}
	if findSymbol(got.Symbols, "not-a-block", "template-block").Handle != "" {
		t.Fatalf("verbatim content became a structural block: %+v", got.Symbols)
	}
	if !hasChunkKind(got.Chunks, "literal") {
		t.Fatalf("verbatim content was not retained as literal: %+v", got.Chunks)
	}
	found := false
	for _, chunk := range got.Chunks {
		if chunk.Kind == "literal" && strings.Contains(chunk.Content, source) {
			found = true
		}
	}
	if !found {
		t.Fatalf("verbatim source was not retained: %+v", got.Chunks)
	}
}

func findSymbol(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasChunkKind(chunks []model.Chunk, kind string) bool {
	for _, chunk := range chunks {
		if chunk.Kind == kind {
			return true
		}
	}
	return false
}

func hasRelation(relations []model.Relation, from, to, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.ToHandle == to && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

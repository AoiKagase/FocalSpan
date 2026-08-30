package resx

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractorExtractsResourceKeysMetadataAndSettingsWithoutBlobChunks(t *testing.T) {
	content := []byte(`<?xml version="1.0" encoding="utf-8"?>
<root>
  <resheader name="resmimetype"><value>text/microsoft-resx</value></resheader>
  <data name="PrimaryBrush" type="System.String"><value>#FF00AA</value></data>
  <data name="Logo" mimetype="application/octet-stream"><value>QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=</value></data>
  <metadata name="GeneratedBy"><value>tool</value></metadata>
  <data name="External"><value><ResXFileRef>..\\Resources\\logo.png;System.Drawing.Bitmap</ResXFileRef></value></data>
</root>
`)
	file := model.SourceFile{Path: "Forms/MainForm.resx", Language: "dotnet-resource", Content: content}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	owner := findResX(got.Symbols, "resx_document", "Forms/MainForm.resx")
	if owner.Handle == "" {
		t.Fatalf("owner missing: %+v", got.Symbols)
	}
	for _, name := range []string{"PrimaryBrush", "Logo", "External", "GeneratedBy"} {
		if findResXByName(got.Symbols, name).Handle == "" {
			t.Errorf("resource symbol %q missing: %+v", name, got.Symbols)
		}
	}
	if !hasResXUnresolved(got.Relations, owner.Handle, "..\\Resources\\logo.png", "imports") {
		t.Fatalf("ResXFileRef import missing: %+v", got.Relations)
	}
	primary := findResXByName(got.Symbols, "PrimaryBrush")
	logo := findResXByName(got.Symbols, "Logo")
	if !hasResXUnresolved(got.Relations, primary.Handle, "System.String", "references") {
		t.Fatalf("resource type reference missing: %+v", got.Relations)
	}
	if !hasResXUnresolved(got.Relations, logo.Handle, "application/octet-stream", "references") {
		t.Fatalf("resource mimetype reference missing: %+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if strings.Contains(chunk.Content, "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=") {
			t.Fatalf("binary blob was copied to a chunk: %+v", chunk)
		}
		if chunk.StartByte != 0 || chunk.EndByte != 0 {
			if string(content[chunk.StartByte:chunk.EndByte]) != chunk.Content {
				t.Fatalf("RESX chunk source mismatch: %+v", chunk)
			}
		}
	}
}

func TestSettingsExtractorExtractsGeneratedSettingReference(t *testing.T) {
	content := []byte("<SettingsFile><Setting Name=\"ApiUrl\" Type=\"System.String\" Scope=\"Application\"><Value Profile=\"(Default)\">https://example.invalid</Value></Setting></SettingsFile>")
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Properties/Settings.settings", Language: "dotnet-resource", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if findResXByName(got.Symbols, "ApiUrl").Handle == "" {
		t.Fatalf("setting missing: %+v", got.Symbols)
	}
	setting := findResXByName(got.Symbols, "ApiUrl")
	if !hasResXUnresolved(got.Relations, setting.Handle, "System.String", "references") || !hasResXUnresolved(got.Relations, setting.Handle, "Application", "references") {
		t.Fatalf("setting metadata references missing: %+v", got.Relations)
	}
}

func findResX(symbols []model.Symbol, kind, name string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Kind == kind && symbol.Name == name {
			return symbol
		}
	}
	return model.Symbol{}
}

func findResXByName(symbols []model.Symbol, name string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasResXUnresolved(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

package xaml

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/model"
)

func TestExtractorSupportsXAMLAndExtractsBindingsEventsAndResources(t *testing.T) {
	content := []byte("<?xml version=\"1.0\"?>\r\n<Window x:Class=\"Demo.MainWindow\" xmlns=\"http://schemas.microsoft.com/winfx/2006/xaml/presentation\"\r\n        xmlns:x=\"http://schemas.microsoft.com/winfx/2006/xaml\">\r\n  <Window.Resources>\r\n    <SolidColorBrush x:Key=\"PrimaryBrush\" Color=\"Blue\" />\r\n  </Window.Resources>\r\n  <Grid>\r\n    <Button x:Name=\"SaveButton\" Click=\"OnSaveClick\" Background=\"{StaticResource PrimaryBrush}\" />\r\n    <TextBox Name=\"UserNameBox\" Text=\"{Binding UserName}\" />\r\n  </Grid>\r\n</Window>\r\n")
	file := model.SourceFile{Path: "Views/MainWindow.xaml", Language: "xaml", Content: content}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	owner := findXAML(got.Symbols, "xaml_document", "Views/MainWindow.xaml")
	if owner.Handle == "" {
		t.Fatalf("document owner missing: %+v", got.Symbols)
	}
	for _, name := range []string{"SaveButton", "UserNameBox", "PrimaryBrush"} {
		if findXAMLByName(got.Symbols, name).Handle == "" {
			t.Errorf("named XAML symbol %q missing: %+v", name, got.Symbols)
		}
	}
	saveButton := findXAMLByName(got.Symbols, "SaveButton")
	if !hasUnresolvedXAML(got.Relations, owner.Handle, "Demo.MainWindow", "references") || !hasUnresolvedXAML(got.Relations, saveButton.Handle, "OnSaveClick", "references") || !hasUnresolvedXAML(got.Relations, findXAMLByName(got.Symbols, "UserNameBox").Handle, "UserName", "references") {
		t.Fatalf("XAML references missing: %+v", got.Relations)
	}
	resource := findXAMLByName(got.Symbols, "PrimaryBrush")
	if !hasResolvedXAML(got.Relations, saveButton.Handle, resource.Handle, "references") {
		t.Fatalf("StaticResource was not resolved in-file: %+v", got.Relations)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if string(content[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("XAML chunk source mismatch: %+v", chunk)
		}
	}
}

func TestExtractorRecordsDynamicBindingsAndDictionaryImports(t *testing.T) {
	content := []byte("<Page x:Class=\"Demo.SettingsPage\" DataContext=\"{x:Bind ViewModel}\">\n  <Page.Resources>\n    <ResourceDictionary Source=\"/Themes/Colors.xaml\" />\n  </Page.Resources>\n  <![CDATA[<not-an-element>]]>\n  <TextBlock Text=\"{x:Bind UserName}\" Foreground=\"{DynamicResource PrimaryBrush}\" />\n</Page>\n")
	file := model.SourceFile{Path: "Views/SettingsPage.xaml", Language: "xaml", Content: content}
	got, err := NewExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	owner := findXAML(got.Symbols, "xaml_document", file.Path)
	text := findXAMLByName(got.Symbols, "TextBlock")
	if owner.Handle == "" || text.Handle == "" {
		t.Fatalf("symbols missing: %+v", got.Symbols)
	}
	if !hasUnresolvedXAML(got.Relations, text.Handle, "UserName", "references") {
		t.Fatalf("x:Bind relation missing: %+v", got.Relations)
	}
	if !hasUnresolvedXAML(got.Relations, text.Handle, "PrimaryBrush", "references") {
		t.Fatalf("DynamicResource relation missing: %+v", got.Relations)
	}
	if !hasUnresolvedXAML(got.Relations, owner.Handle, "/Themes/Colors.xaml", "imports") {
		t.Fatalf("dictionary import missing: %+v", got.Relations)
	}
}

func TestExtractorRecoversMalformedXAMLWithoutCopyingComments(t *testing.T) {
	content := []byte("<!-- do not become a symbol -->\n<Window x:Class=\"Demo.Broken\"><Button x:Name=\"Save\"\n")
	got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "Broken.xaml", Language: "xaml", Content: content})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Symbols) == 0 {
		t.Fatalf("malformed XAML was not recovered: %+v", got)
	}
	hasApproximateDiagnostic := false
	for _, diagnostic := range got.Diagnostics {
		if strings.Contains(diagnostic.Message, "approximate") {
			hasApproximateDiagnostic = true
			break
		}
	}
	if !hasApproximateDiagnostic {
		t.Fatalf("malformed XAML diagnostic missing: %+v", got.Diagnostics)
	}
}

func findXAML(symbols []model.Symbol, kind, name string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Kind == kind && symbol.Name == name {
			return symbol
		}
	}
	return model.Symbol{}
}

func findXAMLByName(symbols []model.Symbol, name string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasUnresolvedXAML(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasResolvedXAML(relations []model.Relation, from, to, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.ToHandle == to && relation.Kind == kind {
			return true
		}
	}
	return false
}

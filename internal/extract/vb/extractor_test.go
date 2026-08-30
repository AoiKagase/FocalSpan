package vb

import (
	"context"
	"strings"
	"testing"

	"github.com/focalspan/focalspan/internal/extract/testutil"
	"github.com/focalspan/focalspan/internal/model"
)

func TestLexVBRecognizesCommentsStringsContinuationsDirectivesAndColonStatements(t *testing.T) {
	source := []byte("Option Explicit ' apostrophe comment\r\nREM another comment\r\n#If DEBUG Then\r\nvalue = \"a\"\"b\" : value = value _\r\n    & \"c\"\r\n#Else\r\nvalue = \"release\"\r\n#End If\r\nAttribute VB_Name = \"AuthModule\"\r\n")
	tokens, diagnostics, err := Lex(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", diagnostics)
	}
	seen := map[TokenKind]bool{}
	for _, token := range tokens {
		seen[token.Kind] = true
		if token.StartByte < 0 || token.EndByte < token.StartByte || token.EndByte > len(source) || string(source[token.StartByte:token.EndByte]) != token.Text {
			t.Fatalf("invalid token=%+v", token)
		}
	}
	for _, kind := range []TokenKind{Comment, String, Continuation, Directive, Identifier, Separator} {
		if !seen[kind] {
			t.Fatalf("token kind %q missing: %+v", kind, tokens)
		}
	}
	stringsSeen := 0
	for _, token := range tokens {
		if token.Kind == String && token.Text == `"a""b"` {
			stringsSeen++
		}
	}
	if stringsSeen != 1 {
		t.Fatalf("doubled-quote string was not kept intact: %+v", tokens)
	}
}

func TestVB6ExtractorBuildsDeclarationsRelationsAndFormLayout(t *testing.T) {
	source := []byte(`VERSION 5.00
Begin VB.Form MainForm
   Caption         =   "Login"
   ClientHeight    =   3000
End
Attribute VB_Name = "MainForm"
Option Explicit
Implements IAuthorizer
Private WithEvents service As AuthService
Public Event Authorized(user As String)
Private Type Credentials
    UserName As String
End Type
Private Enum AuthState
    LoggedOut = 0
End Enum
Private Const DefaultRole As String = "user"
Private Declare Function GetTickCount Lib "kernel32" () As Long

Private Sub service_Login(ByVal token As String)
    If ValidateToken(token) Then RaiseEvent Authorized(token)
End Sub

Public Function ValidateToken(ByVal token As String) As Boolean
    ValidateToken = Len(token) > 0
End Function

Public Property Get CurrentUser() As String
    CurrentUser = "guest"
End Property
`)
	file := model.SourceFile{Path: "MainForm.frm", Language: "vb6", Content: source}
	got, err := NewVB6Extractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"MainForm", "form"}, {"service_Login", "event-handler"}, {"ValidateToken", "function"},
		{"CurrentUser", "property-get"}, {"Authorized", "event"}, {"Credentials", "type"},
		{"AuthState", "enum"}, {"DefaultRole", "constant"}, {"GetTickCount", "declare"},
	} {
		if !hasVBSymbol(got.Symbols, want.name, want.kind) {
			t.Errorf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	owner := findVBSymbol(got.Symbols, "MainForm", "form")
	if owner.Handle == "" || !hasVBRelation(got.Relations, owner.Handle, "IAuthorizer", "references") {
		t.Fatalf("Implements relation missing: %+v", got.Relations)
	}
	if !hasVBRelationKind(got.Relations, "calls") || !hasVBRelationKind(got.Relations, "contains") {
		t.Fatalf("VB6 relations missing: %+v", got.Relations)
	}
	if !hasVBChunkKind(got.Chunks, "form-layout") {
		t.Fatalf("form-layout chunk missing: %+v", got.Chunks)
	}
	for _, chunk := range got.Chunks {
		if chunk.StartByte == 0 && chunk.EndByte == 0 {
			continue
		}
		if chunk.EndByte > len(source) || string(source[chunk.StartByte:chunk.EndByte]) != chunk.Content {
			t.Fatalf("invalid source chunk=%+v", chunk)
		}
		if strings.Contains(strings.ToLower(chunk.Content), ".frx") {
			t.Fatalf("binary FRX reference leaked into chunk=%+v", chunk)
		}
	}
	testutil.AssertExtraction(t, file, got)
}

func TestVBNetExtractorBuildsPartialGenericsEventsAndRelations(t *testing.T) {
	source := []byte(`Imports System
Imports System.Threading.Tasks
Namespace Demo
Public Interface IAuthorizer
    Function ValidateToken(token As String) As Boolean
End Interface
Partial Public Class MainWindow
    Inherits WindowBase
    Implements IAuthorizer
    Public Event Authorized As EventHandler
    Public Property CurrentUser As String
    Public Async Function ValidateTokenAsync(Of T)(token As T) As Task(Of Boolean)
        Return Await ValidateToken(token.ToString())
    End Function
    Private Sub Button_Click(sender As Object, e As EventArgs) Handles Button.Click
        AddHandler Button.Click, AddressOf Button_Click
        RemoveHandler Button.Closed, AddressOf Button_Click
    End Sub
    Public Function ValidateToken(token As String) As Boolean
        Return Not String.IsNullOrEmpty(token)
    End Function
End Class
End Namespace
`)
	file := model.SourceFile{Path: "Views/MainWindow.xaml.vb", Language: "vbnet", Content: source}
	got, err := NewVBNetExtractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ name, kind string }{
		{"Demo", "namespace"}, {"IAuthorizer", "interface"}, {"MainWindow", "class"},
		{"ValidateTokenAsync", "function"}, {"CurrentUser", "property"}, {"Authorized", "event"},
		{"Button_Click", "event-handler"}, {"ValidateToken", "function"},
	} {
		if !hasVBSymbol(got.Symbols, want.name, want.kind) {
			t.Errorf("missing %s %q: %+v", want.kind, want.name, got.Symbols)
		}
	}
	mainWindow := findVBSymbol(got.Symbols, "MainWindow", "class")
	if mainWindow.Handle == "" || !hasVBRelation(got.Relations, mainWindow.Handle, "WindowBase", "references") || !hasVBRelation(got.Relations, mainWindow.Handle, "IAuthorizer", "references") {
		t.Fatalf("inheritance/implements relations missing: %+v", got.Relations)
	}
	handler := findVBSymbol(got.Symbols, "Button_Click", "event-handler")
	if handler.Handle == "" || !hasVBRelation(got.Relations, handler.Handle, "Button.Click", "references") || !hasVBRelationKind(got.Relations, "calls") {
		t.Fatalf("VB.NET event/call relations missing: %+v", got.Relations)
	}
	testutil.AssertExtraction(t, file, got)
}

func TestVBExtractorsRecoverMalformedBlocksAndSelectOnlyTheirProfile(t *testing.T) {
	if !NewVB6Extractor().Supports("Form.frm", "vb6") || NewVB6Extractor().Supports("Form.vb", "vbnet") {
		t.Fatal("VB6 Supports mismatch")
	}
	if !NewVBNetExtractor().Supports("Form.vb", "vbnet") || NewVBNetExtractor().Supports("Form.frm", "vb6") {
		t.Fatal("VB.NET Supports mismatch")
	}
	file := model.SourceFile{Path: "Broken.bas", Language: "vb6", Content: []byte("Attribute VB_Name = \"Broken\"\nPublic Sub Unclosed(\n  x = 1\n")}
	got, err := NewVB6Extractor().Extract(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if !hasVBSymbol(got.Symbols, "Unclosed", "sub") || len(got.Diagnostics) == 0 {
		t.Fatalf("malformed block was not recovered: %+v", got)
	}
	testutil.AssertExtraction(t, file, got)
}

func hasVBSymbol(symbols []model.Symbol, name, kind string) bool {
	return findVBSymbol(symbols, name, kind).Handle != ""
}

func findVBSymbol(symbols []model.Symbol, name, kind string) model.Symbol {
	for _, symbol := range symbols {
		if symbol.Name == name && symbol.Kind == kind {
			return symbol
		}
	}
	return model.Symbol{}
}

func hasVBRelation(relations []model.Relation, from, target, kind string) bool {
	for _, relation := range relations {
		if relation.FromHandle == from && relation.UnresolvedTo == target && relation.Kind == kind {
			return true
		}
	}
	return false
}

func hasVBRelationKind(relations []model.Relation, kind string) bool {
	for _, relation := range relations {
		if relation.Kind == kind && (relation.ToHandle != "" || relation.UnresolvedTo != "") {
			return true
		}
	}
	return false
}

func hasVBChunkKind(chunks []model.Chunk, kind string) bool {
	for _, chunk := range chunks {
		if chunk.Kind == kind {
			return true
		}
	}
	return false
}

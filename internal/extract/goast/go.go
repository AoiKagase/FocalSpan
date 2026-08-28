package goast

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	"github.com/focalspan/focalspan/internal/model"
)

type Extractor struct{}

func NewExtractor() Extractor { return Extractor{} }

func (Extractor) Name() string { return "go-ast" }

func (Extractor) Supports(path, language string) bool {
	return language == "go" || strings.EqualFold(filepath.Ext(path), ".go")
}

func (Extractor) Extract(ctx context.Context, file model.SourceFile) (model.Extraction, error) {
	if err := ctx.Err(); err != nil {
		return model.Extraction{}, err
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file.Path, file.Content, parser.ParseComments)
	if err != nil {
		return model.Extraction{Diagnostics: []model.Diagnostic{{FilePath: file.Path, Level: "warning", Code: "go_parse", Message: err.Error()}}}, fmt.Errorf("parse Go source %s: %w", file.Path, err)
	}
	result := model.Extraction{}
	allocator := model.NewHandleAllocator()
	packageHandle := allocator.Allocate("sym", file.Path, "go", "package", parsed.Name.Name, parsed.Name.Name)
	packageStart, packageEnd := span(fset, parsed)
	result.Symbols = append(result.Symbols, model.Symbol{Handle: packageHandle, FilePath: file.Path, Language: "go", Kind: "package", Name: parsed.Name.Name, QualifiedName: parsed.Name.Name, Signature: "package " + parsed.Name.Name, StartLine: packageStart.line, EndLine: packageEnd.line, StartByte: packageStart.offset, EndByte: packageEnd.offset, Confidence: 1})
	for _, spec := range parsed.Imports {
		path := strings.Trim(spec.Path.Value, "\"")
		result.Relations = append(result.Relations, model.Relation{FromHandle: packageHandle, UnresolvedTo: path, Kind: "imports", Confidence: 1, Source: "go-ast"})
	}

	topHandles := make(map[string]string)
	for _, decl := range parsed.Decls {
		if err := ctx.Err(); err != nil {
			return model.Extraction{}, err
		}
		switch d := decl.(type) {
		case *ast.FuncDecl:
			symbol := buildFuncSymbol(file, fset, d, allocator)
			result.Symbols = append(result.Symbols, symbol)
			topHandles[symbol.Name] = symbol.Handle
			result.Relations = append(result.Relations, model.Relation{FromHandle: packageHandle, ToHandle: symbol.Handle, Kind: "contains", Confidence: 1, Source: "go-ast"})
			if d.Recv != nil && len(d.Recv.List) > 0 {
				receiver := receiverName(d.Recv.List[0].Type)
				if receiver != "" {
					result.Relations = append(result.Relations, model.Relation{FromHandle: symbol.Handle, UnresolvedTo: receiver, Kind: "method_of", Confidence: .85, Source: "go-ast"})
				}
			}
			result.Relations = append(result.Relations, functionRelations(symbol, d, file, fset)...)
			result.Chunks = append(result.Chunks, chunkForSymbol(symbol, sourceForNode(file.Content, fset, d)))
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				symbol, ok := buildDeclSymbol(file, fset, d, spec, allocator)
				if !ok {
					continue
				}
				result.Symbols = append(result.Symbols, symbol)
				topHandles[symbol.Name] = symbol.Handle
				result.Relations = append(result.Relations, model.Relation{FromHandle: packageHandle, ToHandle: symbol.Handle, Kind: "contains", Confidence: 1, Source: "go-ast"})
				result.Chunks = append(result.Chunks, chunkForSymbol(symbol, sourceForNode(file.Content, fset, spec)))
			}
		}
	}
	for i := range result.Symbols {
		if result.Symbols[i].Kind == "test" {
			name := strings.TrimPrefix(result.Symbols[i].Name, "Test")
			if name != "" {
				result.Relations = append(result.Relations, model.Relation{FromHandle: result.Symbols[i].Handle, UnresolvedTo: name, Kind: "tests", Confidence: .4, Source: "go-ast"})
			}
		}
	}
	sort.Slice(result.Symbols, func(i, j int) bool {
		return result.Symbols[i].StartByte < result.Symbols[j].StartByte || result.Symbols[i].Name < result.Symbols[j].Name
	})
	sort.Slice(result.Chunks, func(i, j int) bool {
		return result.Chunks[i].StartByte < result.Chunks[j].StartByte || result.Chunks[i].Handle < result.Chunks[j].Handle
	})
	sort.Slice(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		if result.Relations[i].Kind != result.Relations[j].Kind {
			return result.Relations[i].Kind < result.Relations[j].Kind
		}
		return result.Relations[i].UnresolvedTo < result.Relations[j].UnresolvedTo
	})
	return result, nil
}

type sourceSpan struct{ line, offset int }

func span(fset *token.FileSet, node interface {
	Pos() token.Pos
	End() token.Pos
}) (sourceSpan, sourceSpan) {
	start := fset.PositionFor(node.Pos(), true)
	end := fset.PositionFor(node.End(), true)
	return sourceSpan{start.Line, start.Offset}, sourceSpan{end.Line, end.Offset}
}

func buildFuncSymbol(file model.SourceFile, fset *token.FileSet, decl *ast.FuncDecl, allocator *model.HandleAllocator) model.Symbol {
	start, end := span(fset, decl)
	name := decl.Name.Name
	kind := "function"
	qualified := name
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		kind = "method"
		receiver := receiverName(decl.Recv.List[0].Type)
		qualified = receiver + "." + name
	}
	if strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example") {
		kind = "test"
	}
	signature := funcSignature(file.Content, fset, decl)
	handle := allocator.Allocate("sym", file.Path, "go", kind, qualified, model.NormalizeSignature(signature))
	return model.Symbol{Handle: handle, FilePath: file.Path, Language: "go", Kind: kind, Name: name, QualifiedName: qualified, Signature: signature, StartLine: start.line, EndLine: end.line, StartByte: start.offset, EndByte: end.offset, Confidence: 1}
}

func buildDeclSymbol(file model.SourceFile, fset *token.FileSet, decl *ast.GenDecl, spec ast.Spec, allocator *model.HandleAllocator) (model.Symbol, bool) {
	var name, kind string
	switch s := spec.(type) {
	case *ast.TypeSpec:
		name = s.Name.Name
		kind = "type"
		switch s.Type.(type) {
		case *ast.StructType:
			kind = "struct"
		case *ast.InterfaceType:
			kind = "interface"
		}
	case *ast.ValueSpec:
		if len(s.Names) == 0 {
			return model.Symbol{}, false
		}
		name = s.Names[0].Name
		kind = strings.ToLower(decl.Tok.String())
	default:
		return model.Symbol{}, false
	}
	start, end := span(fset, spec)
	signature := strings.TrimSpace(sourceForNode(file.Content, fset, spec))
	if i := strings.IndexByte(signature, '\n'); i >= 0 {
		signature = signature[:i]
	}
	handle := allocator.Allocate("sym", file.Path, "go", kind, name, model.NormalizeSignature(signature))
	return model.Symbol{Handle: handle, FilePath: file.Path, Language: "go", Kind: kind, Name: name, QualifiedName: name, Signature: signature, StartLine: start.line, EndLine: end.line, StartByte: start.offset, EndByte: end.offset, Confidence: 1}, true
}

func funcSignature(content []byte, fset *token.FileSet, decl *ast.FuncDecl) string {
	start := fset.PositionFor(decl.Pos(), true).Offset
	end := fset.PositionFor(decl.End(), true).Offset
	if decl.Body != nil {
		end = fset.PositionFor(decl.Body.Pos(), true).Offset
	}
	if start < 0 || end < start || end > len(content) {
		return "func " + decl.Name.Name
	}
	return model.NormalizeSignature(string(content[start:end]))
}

func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.ParenExpr:
		return receiverName(e.X)
	default:
		return ""
	}
}

func functionRelations(symbol model.Symbol, decl *ast.FuncDecl, file model.SourceFile, fset *token.FileSet) []model.Relation {
	result := make([]model.Relation, 0)
	if decl.Body == nil {
		return result
	}
	ast.Inspect(decl.Body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			name, confidence := callName(n.Fun)
			if name != "" {
				result = append(result, model.Relation{FromHandle: symbol.Handle, UnresolvedTo: name, Kind: "calls", Confidence: confidence, Source: "go-ast"})
			}
		case *ast.SelectorExpr:
			if _, ok := n.X.(*ast.Ident); ok {
				result = append(result, model.Relation{FromHandle: symbol.Handle, UnresolvedTo: n.Sel.Name, Kind: "refers_to", Confidence: .35, Source: "go-ast"})
			}
		}
		return true
	})
	return result
}

func callName(expr ast.Expr) (string, float64) {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name, .45
	case *ast.SelectorExpr:
		return e.Sel.Name, .3
	default:
		return "", 0
	}
}

func sourceForNode(content []byte, fset *token.FileSet, node interface {
	Pos() token.Pos
	End() token.Pos
}) string {
	start := fset.PositionFor(node.Pos(), true).Offset
	end := fset.PositionFor(node.End(), true).Offset
	if start < 0 || end < start || end > len(content) {
		return ""
	}
	return string(content[start:end])
}

func chunkForSymbol(symbol model.Symbol, content string) model.Chunk {
	h := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, symbol.Kind, content), FilePath: symbol.FilePath, Language: symbol.Language, Kind: symbol.Kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: symbol.Signature, StartLine: symbol.StartLine, EndLine: symbol.EndLine, StartByte: symbol.StartByte, EndByte: symbol.EndByte, Content: content, ContentHash: hex.EncodeToString(h[:])}
}

func formatExpr(expr ast.Expr) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return b.String()
}

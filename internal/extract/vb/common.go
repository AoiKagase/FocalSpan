package vb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/focalspan/focalspan/internal/model"
)

type TokenKind string

const (
	Comment      TokenKind = "comment"
	String       TokenKind = "string"
	Continuation TokenKind = "continuation"
	Directive    TokenKind = "directive"
	Identifier   TokenKind = "identifier"
	Separator    TokenKind = "separator"
	Punctuation  TokenKind = "punctuation"
)

type Token struct {
	Kind      TokenKind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

func Lex(ctx context.Context, source []byte) ([]Token, []model.Diagnostic, error) {
	tokens := make([]Token, 0, len(source)/4)
	diagnostics := make([]model.Diagnostic, 0)
	for at := 0; at < len(source); {
		if err := ctx.Err(); err != nil {
			return tokens, diagnostics, err
		}
		if source[at] == '\r' || source[at] == '\n' || source[at] == ' ' || source[at] == '\t' {
			at++
			continue
		}
		lineStart := at == 0 || source[at-1] == '\n'
		if lineStart {
			probe := at
			for probe < len(source) && (source[probe] == ' ' || source[probe] == '\t') {
				probe++
			}
			if probe < len(source) && source[probe] == '#' {
				end := vbLineEnd(source, probe)
				tokens = append(tokens, vbToken(Directive, source, probe, end))
				at = end
				continue
			}
			if probe+3 <= len(source) && strings.EqualFold(string(source[probe:probe+3]), "rem") && (probe+3 == len(source) || isVBSpace(source[probe+3])) {
				end := vbLineEnd(source, probe)
				tokens = append(tokens, vbToken(Comment, source, probe, end))
				at = end
				continue
			}
		}
		if source[at] == '\'' {
			end := vbLineEnd(source, at)
			tokens = append(tokens, vbToken(Comment, source, at, end))
			at = end
			continue
		}
		if source[at] == '"' {
			start := at
			at++
			closed := false
			for at < len(source) {
				if source[at] != '"' {
					at++
					continue
				}
				if at+1 < len(source) && source[at+1] == '"' {
					at += 2
					continue
				}
				at++
				closed = true
				break
			}
			tokens = append(tokens, vbToken(String, source, start, at))
			if !closed {
				diagnostics = append(diagnostics, model.Diagnostic{Level: "warning", Code: "vb_unclosed_string", Message: "VB string is not closed"})
			}
			continue
		}
		if source[at] == '_' && vbContinuation(source, at) {
			tokens = append(tokens, vbToken(Continuation, source, at, at+1))
			at++
			continue
		}
		if source[at] == ':' {
			tokens = append(tokens, vbToken(Separator, source, at, at+1))
			at++
			continue
		}
		if isVBIdentifierStart(source[at]) {
			start := at
			at++
			for at < len(source) && isVBIdentifierPart(source[at]) {
				at++
			}
			tokens = append(tokens, vbToken(Identifier, source, start, at))
			continue
		}
		start := at
		at++
		tokens = append(tokens, vbToken(Punctuation, source, start, at))
	}
	return tokens, diagnostics, nil
}

func vbContinuation(source []byte, at int) bool {
	for probe := at + 1; probe < len(source); probe++ {
		switch source[probe] {
		case ' ', '\t', '\r':
			continue
		case '\n':
			return true
		default:
			return false
		}
	}
	return true
}

func isVBSpace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func isVBIdentifierStart(value byte) bool {
	return value == '_' || value >= 0x80 || unicode.IsLetter(rune(value))
}

func isVBIdentifierPart(value byte) bool {
	return isVBIdentifierStart(value) || value >= '0' && value <= '9'
}

func vbToken(kind TokenKind, source []byte, start, end int) Token {
	return Token{Kind: kind, Text: string(source[start:end]), StartByte: start, EndByte: end, StartLine: vbLineNumber(source, start), EndLine: vbLineNumber(source, end)}
}

func vbLineEnd(source []byte, start int) int {
	for at := start; at < len(source); at++ {
		if source[at] == '\n' {
			return at
		}
	}
	return len(source)
}

type vbLine struct {
	Start int
	End   int
	Text  string
}

func vbLines(source []byte) []vbLine {
	starts := []int{0}
	for at, value := range source {
		if value == '\n' && at+1 < len(source) {
			starts = append(starts, at+1)
		}
	}
	lines := make([]vbLine, 0, len(starts))
	for index, start := range starts {
		end := len(source)
		if index+1 < len(starts) {
			end = starts[index+1]
		}
		for end > start && (source[end-1] == '\n' || source[end-1] == '\r') {
			end--
		}
		lines = append(lines, vbLine{Start: start, End: end, Text: string(source[start:end])})
	}
	return lines
}

func vbLineNumber(source []byte, offset int) int {
	if offset > len(source) {
		offset = len(source)
	}
	return 1 + strings.Count(string(source[:offset]), "\n")
}

func vbLineCount(source []byte) int {
	return 1 + strings.Count(string(source), "\n")
}

func vbCode(text string) string {
	for at := 0; at < len(text); at++ {
		if text[at] != '"' {
			if text[at] == '\'' {
				return text[:at]
			}
			continue
		}
		at++
		for at < len(text) {
			if text[at] == '"' {
				if at+1 < len(text) && text[at+1] == '"' {
					at += 2
					continue
				}
				break
			}
			at++
		}
	}
	trimmed := strings.TrimSpace(text)
	if len(trimmed) >= 3 && strings.EqualFold(trimmed[:3], "rem") && (len(trimmed) == 3 || isVBSpace(trimmed[3])) {
		return ""
	}
	return text
}

func vbNameFromAttribute(lines []vbLine) string {
	for _, line := range lines {
		text := strings.TrimSpace(vbCode(line.Text))
		lower := strings.ToLower(text)
		if !strings.HasPrefix(lower, "attribute vb_name") {
			continue
		}
		if start := strings.Index(text, "\""); start >= 0 {
			if end := strings.Index(text[start+1:], "\""); end >= 0 {
				return text[start+1 : start+1+end]
			}
		}
	}
	return ""
}

type vbDecl struct {
	Kind      string
	Name      string
	Qualified string
	Header    string
	StartLine int
	EndLine   int
	Parent    *vbDecl
	Block     bool
}

type vbParseResult struct {
	Owner       *vbDecl
	Decls       []*vbDecl
	Lines       []vbLine
	LayoutEnd   int
	Diagnostics []model.Diagnostic
}

func vbSourceChunk(file model.SourceFile, symbol model.Symbol, kind string, start, end, startLine, endLine int) model.Chunk {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}
	if end > len(file.Content) {
		end = len(file.Content)
	}
	content := string(file.Content[start:end])
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, fmt.Sprint(start), fmt.Sprint(end)), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: strings.TrimSpace(content), StartLine: startLine, EndLine: endLine, StartByte: start, EndByte: end, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func vbSyntheticChunk(file model.SourceFile, symbol model.Symbol, kind, content string) model.Chunk {
	digest := sha256.Sum256([]byte(content))
	return model.Chunk{Handle: model.StableHandle("chunk", symbol.Handle, kind, content), FilePath: file.Path, Language: file.Language, Kind: kind, SymbolHandle: symbol.Handle, SymbolName: symbol.Name, Signature: "synthetic outline (not a source slice): " + symbol.Signature, StartLine: 1, EndLine: 1, Content: content, ContentHash: hex.EncodeToString(digest[:])}
}

func addVBRelation(result *model.Extraction, relation model.Relation) {
	if relation.FromHandle == "" || relation.FromHandle == relation.ToHandle || relation.ToHandle != "" && relation.UnresolvedTo != "" {
		return
	}
	if relation.Confidence < 0 {
		relation.Confidence = 0
	}
	if relation.Confidence > 1 {
		relation.Confidence = 1
	}
	for _, old := range result.Relations {
		if old.FromHandle == relation.FromHandle && old.ToHandle == relation.ToHandle && old.UnresolvedTo == relation.UnresolvedTo && old.Kind == relation.Kind {
			return
		}
	}
	result.Relations = append(result.Relations, relation)
}

func sortVB(result *model.Extraction) {
	sort.SliceStable(result.Symbols, func(i, j int) bool {
		if result.Symbols[i].StartByte != result.Symbols[j].StartByte {
			return result.Symbols[i].StartByte < result.Symbols[j].StartByte
		}
		return result.Symbols[i].Handle < result.Symbols[j].Handle
	})
	sort.SliceStable(result.Chunks, func(i, j int) bool {
		if result.Chunks[i].StartByte != result.Chunks[j].StartByte {
			return result.Chunks[i].StartByte < result.Chunks[j].StartByte
		}
		return result.Chunks[i].Handle < result.Chunks[j].Handle
	})
	sort.SliceStable(result.Relations, func(i, j int) bool {
		if result.Relations[i].FromHandle != result.Relations[j].FromHandle {
			return result.Relations[i].FromHandle < result.Relations[j].FromHandle
		}
		if result.Relations[i].Kind != result.Relations[j].Kind {
			return result.Relations[i].Kind < result.Relations[j].Kind
		}
		return result.Relations[i].UnresolvedTo < result.Relations[j].UnresolvedTo
	})
}

func vbOwner(file model.SourceFile, kind, name string) *vbDecl {
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path))
	}
	return &vbDecl{Kind: kind, Name: name, Qualified: name, Header: kind + " " + name, StartLine: 1, EndLine: vbLineCount(file.Content)}
}

func currentVBDecl(decls []*vbDecl, line int) *vbDecl {
	var current *vbDecl
	for _, decl := range decls {
		if decl.StartLine <= line && line <= decl.EndLine {
			if current == nil || decl.StartLine >= current.StartLine && decl.EndLine <= current.EndLine {
				current = decl
			}
		}
	}
	return current
}

func vbIndex(symbols []model.Symbol) map[string][]model.Symbol {
	index := make(map[string][]model.Symbol)
	for _, symbol := range symbols {
		index[strings.ToLower(symbol.Name)] = append(index[strings.ToLower(symbol.Name)], symbol)
	}
	return index
}

func vbUnique(index map[string][]model.Symbol, name string) (model.Symbol, bool) {
	items := index[strings.ToLower(strings.TrimSpace(name))]
	if len(items) != 1 {
		return model.Symbol{}, false
	}
	return items[0], true
}

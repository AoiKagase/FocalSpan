package jsts

type TokenKind string

const (
	Identifier    TokenKind = "identifier"
	Keyword       TokenKind = "keyword"
	Number        TokenKind = "number"
	Punctuation   TokenKind = "punctuation"
	Operator      TokenKind = "operator"
	Whitespace    TokenKind = "whitespace"
	LineComment   TokenKind = "line-comment"
	BlockComment  TokenKind = "block-comment"
	StringLiteral TokenKind = "string-literal"
	Template      TokenKind = "template-literal"
	RegexLiteral  TokenKind = "regex-literal"
	JSX           TokenKind = "jsx"
)

type Token struct {
	Kind      TokenKind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

func (t Token) significant() bool {
	return t.Kind != Whitespace && t.Kind != LineComment && t.Kind != BlockComment
}

var keywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "case": true, "catch": true,
	"class": true, "const": true, "continue": true, "debugger": true, "default": true,
	"delete": true, "do": true, "else": true, "enum": true, "export": true, "extends": true,
	"false": true, "finally": true, "for": true, "from": true, "function": true, "get": true,
	"if": true, "implements": true, "import": true, "in": true, "instanceof": true,
	"interface": true, "let": true, "new": true, "null": true, "of": true, "private": true,
	"protected": true, "public": true, "return": true, "set": true, "static": true,
	"super": true, "switch": true, "this": true, "throw": true, "true": true, "try": true,
	"type": true, "typeof": true, "undefined": true, "var": true, "void": true, "while": true,
	"with": true, "yield": true, "declare": true, "namespace": true, "module": true, "readonly": true,
}

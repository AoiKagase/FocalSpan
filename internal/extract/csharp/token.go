package csharp

type TokenKind string

const (
	Identifier         TokenKind = "identifier"
	Keyword            TokenKind = "keyword"
	Number             TokenKind = "number"
	Punctuation        TokenKind = "punctuation"
	Operator           TokenKind = "operator"
	Whitespace         TokenKind = "whitespace"
	LineComment        TokenKind = "line-comment"
	BlockComment       TokenKind = "block-comment"
	XMLDocComment      TokenKind = "xml-doc-comment"
	NormalString       TokenKind = "normal-string"
	VerbatimString     TokenKind = "verbatim-string"
	InterpolatedString TokenKind = "interpolated-string"
	RawString          TokenKind = "raw-string"
	CharLiteral        TokenKind = "character-literal"
	Preprocessor       TokenKind = "preprocessor"
	Attribute          TokenKind = "attribute"
)

type Token struct {
	Kind      TokenKind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
	Active    bool
}

var keywords = map[string]bool{
	"abstract": true, "as": true, "async": true, "await": true, "base": true,
	"bool": true, "break": true, "byte": true, "case": true, "catch": true,
	"char": true, "checked": true, "class": true, "const": true, "continue": true,
	"decimal": true, "default": true, "delegate": true, "do": true, "double": true,
	"else": true, "enum": true, "event": true, "explicit": true, "extern": true,
	"false": true, "finally": true, "fixed": true, "float": true, "for": true,
	"foreach": true, "from": true, "get": true, "global": true, "goto": true,
	"if": true, "implicit": true, "in": true, "int": true, "interface": true,
	"internal": true, "is": true, "lock": true, "long": true, "namespace": true,
	"new": true, "null": true, "object": true, "operator": true, "out": true,
	"override": true, "params": true, "private": true, "protected": true,
	"public": true, "readonly": true, "record": true, "ref": true, "remove": true,
	"return": true, "sbyte": true, "sealed": true, "set": true, "short": true,
	"sizeof": true, "stackalloc": true, "static": true, "string": true,
	"struct": true, "switch": true, "this": true, "throw": true, "true": true,
	"try": true, "typeof": true, "uint": true, "ulong": true, "unchecked": true,
	"unsafe": true, "ushort": true, "using": true, "value": true, "var": true,
	"virtual": true, "void": true, "volatile": true, "when": true, "where": true,
	"while": true, "with": true, "yield": true,
}

func (t Token) significant() bool {
	return t.Active && t.Kind != Whitespace && t.Kind != LineComment && t.Kind != BlockComment && t.Kind != XMLDocComment && t.Kind != Preprocessor
}

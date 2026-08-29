package cpp

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
	CharLiteral   TokenKind = "character-literal"
	RawString     TokenKind = "raw-string-literal"
	Preprocessor  TokenKind = "preprocessor"
	Attribute     TokenKind = "attribute"
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
	"alignas": true, "alignof": true, "and": true, "asm": true, "auto": true,
	"bitand": true, "bitor": true, "bool": true, "break": true, "case": true,
	"catch": true, "char": true, "char8_t": true, "char16_t": true, "char32_t": true,
	"class": true, "compl": true, "concept": true, "const": true, "consteval": true,
	"constexpr": true, "constinit": true, "const_cast": true, "continue": true,
	"co_await": true, "co_return": true, "co_yield": true, "decltype": true,
	"default": true, "delete": true, "do": true, "double": true, "dynamic_cast": true,
	"else": true, "enum": true, "explicit": true, "export": true, "extern": true,
	"false": true, "float": true, "for": true, "friend": true, "goto": true,
	"if": true, "inline": true, "int": true, "long": true, "mutable": true,
	"namespace": true, "new": true, "noexcept": true, "not": true, "nullptr": true,
	"operator": true, "or": true, "private": true, "protected": true, "public": true,
	"register": true, "reinterpret_cast": true, "requires": true, "return": true,
	"short": true, "signed": true, "sizeof": true, "static": true, "static_assert": true,
	"static_cast": true, "struct": true, "switch": true, "template": true, "this": true,
	"thread_local": true, "throw": true, "true": true, "try": true, "typedef": true,
	"typename": true, "union": true, "unsigned": true, "using": true, "virtual": true,
	"void": true, "volatile": true, "wchar_t": true, "while": true, "xor": true,
}

func (t Token) significant() bool {
	return t.Active && t.Kind != Whitespace && t.Kind != LineComment && t.Kind != BlockComment && t.Kind != Preprocessor
}

package rust

type TokenKind string

const (
	Identifier       TokenKind = "identifier"
	Keyword          TokenKind = "keyword"
	Number           TokenKind = "number"
	Punctuation      TokenKind = "punctuation"
	Operator         TokenKind = "operator"
	Whitespace       TokenKind = "whitespace"
	LineComment      TokenKind = "line-comment"
	BlockComment     TokenKind = "block-comment"
	String           TokenKind = "string"
	ByteString       TokenKind = "byte-string"
	RawString        TokenKind = "raw-string"
	Character        TokenKind = "character"
	Lifetime         TokenKind = "lifetime"
	Attribute        TokenKind = "attribute"
	MacroPunctuation TokenKind = "macro-punctuation"
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

var rustKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "const": true, "continue": true,
	"crate": true, "dyn": true, "else": true, "enum": true, "extern": true, "false": true,
	"fn": true, "for": true, "if": true, "impl": true, "in": true, "let": true, "loop": true,
	"match": true, "mod": true, "move": true, "mut": true, "pub": true, "ref": true,
	"return": true, "self": true, "Self": true, "static": true, "struct": true, "super": true,
	"trait": true, "true": true, "type": true, "unsafe": true, "use": true, "where": true,
	"while": true, "union": true,
}

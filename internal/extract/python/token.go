package python

type TokenKind string

const (
	Identifier   TokenKind = "identifier"
	Number       TokenKind = "number"
	Operator     TokenKind = "operator"
	Punctuation  TokenKind = "punctuation"
	Indent       TokenKind = "indent"
	Dedent       TokenKind = "dedent"
	String       TokenKind = "string"
	TripleString TokenKind = "triple-string"
	FString      TokenKind = "f-string"
	Comment      TokenKind = "comment"
	TypeComment  TokenKind = "type-comment"
	Decorator    TokenKind = "decorator"
)

type Token struct {
	Kind      TokenKind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

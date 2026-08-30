package lua

type TokenKind string

const (
	Identifier   TokenKind = "identifier"
	String       TokenKind = "string"
	LongString   TokenKind = "long-string"
	Comment      TokenKind = "comment"
	LongComment  TokenKind = "long-comment"
	BlockKeyword TokenKind = "block-keyword"
	Punctuation  TokenKind = "punctuation"
)

type Token struct {
	Kind               TokenKind
	Text               string
	StartByte, EndByte int
	StartLine, EndLine int
}

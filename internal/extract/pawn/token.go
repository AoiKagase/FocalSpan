package pawn

type TokenKind string

const (
	Identifier   TokenKind = "identifier"
	Keyword      TokenKind = "keyword"
	Number       TokenKind = "number"
	String       TokenKind = "string"
	Char         TokenKind = "char"
	Directive    TokenKind = "directive"
	Comment      TokenKind = "comment"
	BlockComment TokenKind = "block-comment"
	Punctuation  TokenKind = "punctuation"
)

type Token struct {
	Kind               TokenKind
	Text               string
	StartByte, EndByte int
	StartLine, EndLine int
}

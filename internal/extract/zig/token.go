package zig

type TokenKind string

const (
	Comment         TokenKind = "comment"
	String          TokenKind = "string"
	MultilineString TokenKind = "multiline-string"
	Character       TokenKind = "character"
	Builtin         TokenKind = "builtin"
	Comptime        TokenKind = "comptime"
	Operator        TokenKind = "operator"
	Identifier      TokenKind = "identifier"
	Punctuation     TokenKind = "punctuation"
)

type Token struct {
	Kind      TokenKind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

package nim

type TokenKind string

const (
	Comment            TokenKind = "comment"
	LongComment        TokenKind = "long-comment"
	String             TokenKind = "string"
	RawString          TokenKind = "raw-string"
	TripleString       TokenKind = "triple-string"
	Pragma             TokenKind = "pragma"
	BacktickIdentifier TokenKind = "backtick-identifier"
	Continuation       TokenKind = "continuation"
	Identifier         TokenKind = "identifier"
	Punctuation        TokenKind = "punctuation"
)

type Token struct {
	Kind      TokenKind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

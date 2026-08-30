package ruby

type TokenKind string

const (
	Identifier     TokenKind = "identifier"
	String         TokenKind = "string"
	Interpolation  TokenKind = "interpolation"
	Regex          TokenKind = "regex"
	Symbol         TokenKind = "symbol"
	PercentLiteral TokenKind = "percent-literal"
	Heredoc        TokenKind = "heredoc"
	Comment        TokenKind = "comment"
	BlockComment   TokenKind = "block-comment"
	BlockKeyword   TokenKind = "block-keyword"
	Punctuation    TokenKind = "punctuation"
)

type Token struct {
	Kind               TokenKind
	Text               string
	StartByte, EndByte int
	StartLine, EndLine int
}

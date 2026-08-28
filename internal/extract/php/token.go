package php

import "github.com/focalspan/focalspan/internal/model"

type Kind string

const (
	KindUnknown            Kind = "unknown"
	KindOpenTag            Kind = "open_tag"
	KindCloseTag           Kind = "close_tag"
	KindIdentifier         Kind = "identifier"
	KindVariable           Kind = "variable"
	KindKeyword            Kind = "keyword"
	KindPunctuation        Kind = "punctuation"
	KindOperator           Kind = "operator"
	KindWhitespace         Kind = "whitespace"
	KindLineComment        Kind = "line_comment"
	KindBlockComment       Kind = "block_comment"
	KindDocComment         Kind = "doc_comment"
	KindSingleQuotedString Kind = "single_quoted_string"
	KindDoubleQuotedString Kind = "double_quoted_string"
	KindBacktickString     Kind = "backtick_string"
	KindHeredoc            Kind = "heredoc"
	KindNowdoc             Kind = "nowdoc"
	KindInlineHTML         Kind = "inline_html"
)

type Token struct {
	Kind      Kind
	Text      string
	StartByte int
	EndByte   int
	StartLine int
	EndLine   int
}

type lexResult struct {
	Tokens      []Token
	Diagnostics []model.Diagnostic
}

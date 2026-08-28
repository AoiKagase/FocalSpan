package php

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/focalspan/focalspan/internal/model"
)

func TestLexMixedPHPHTMLAndStatefulTokens(t *testing.T) {
	content := "<div>\r\n<?PHP\r\n/** docs { */\r\n$single = 'escaped\\' }';\r\n$double = \"brace }\"; // }\r\n# hash {\r\n/* multi\r\n { comment */\r\n$here = <<<HEREDOC\r\n{ heredoc }\r\nHEREDOC;\r\n$now = <<<'NOW'\r\n{ nowdoc }\r\nNOW;\r\n#[Attribute]\r\n?>\r\n<span>UTF-8 日本語</span>"
	tokens, diagnostics, err := Lex(context.Background(), []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diagnostics)
	}
	assertTokenKindPrefix(t, tokens, KindInlineHTML, "<div>")
	assertTokenKindText(t, tokens, KindOpenTag, "<?PHP")
	assertTokenKindText(t, tokens, KindDocComment, "/** docs { */")
	assertTokenKindText(t, tokens, KindSingleQuotedString, "'escaped\\' }'")
	assertTokenKindText(t, tokens, KindDoubleQuotedString, "\"brace }\"")
	assertTokenKindText(t, tokens, KindHeredoc, "<<<HEREDOC\r\n{ heredoc }\r\nHEREDOC")
	assertTokenKindText(t, tokens, KindNowdoc, "<<<'NOW'\r\n{ nowdoc }\r\nNOW")
	assertTokenKindText(t, tokens, KindCloseTag, "?>")
	if tokens[0].StartByte != 0 || tokens[0].StartLine != 1 {
		t.Fatalf("first token=%+v", tokens[0])
	}
	if tokens[len(tokens)-1].EndByte != len(content) {
		t.Fatalf("last token=%+v want end=%d", tokens[len(tokens)-1], len(content))
	}
	if tokens[len(tokens)-1].EndLine != 17 {
		t.Fatalf("last token line=%d want 17", tokens[len(tokens)-1].EndLine)
	}
}

func TestLexMultiplePHPBlocksAndAttributes(t *testing.T) {
	content := "before<?php echo 1; ?>middle<?= $value ?>after<? echo 2; ?>"
	tokens, diagnostics, err := Lex(context.Background(), []byte(content))
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("err=%v diagnostics=%+v", err, diagnostics)
	}
	if countTokenKind(tokens, KindOpenTag) != 3 || countTokenKind(tokens, KindCloseTag) != 3 {
		t.Fatalf("open/close tokens=%d/%d tokens=%+v", countTokenKind(tokens, KindOpenTag), countTokenKind(tokens, KindCloseTag), tokens)
	}
	if countTokenKind(tokens, KindInlineHTML) < 3 {
		t.Fatalf("inline HTML tokens=%+v", tokens)
	}

	tokens, diagnostics, err = Lex(context.Background(), []byte("<?php #[Attribute] class Example {}"))
	if err != nil || len(diagnostics) != 0 || countTokenText(tokens, "#") != 1 || countTokenText(tokens, "[") != 1 {
		t.Fatalf("attribute tokens=%+v diagnostics=%+v err=%v", tokens, diagnostics, err)
	}
}

func TestLexMalformedInputReportsAndContinues(t *testing.T) {
	tests := []struct {
		name, content, code string
	}{
		{"string", "<?php $value = 'unterminated", "php_unclosed_string"},
		{"comment", "<?php /* unterminated", "php_unclosed_comment"},
		{"heredoc", "<?php <<<END\nbody\n", "php_unclosed_heredoc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, diagnostics, err := Lex(context.Background(), []byte(tt.content))
			if err != nil {
				t.Fatal(err)
			}
			if len(tokens) == 0 || !hasDiagnosticCode(diagnostics, tt.code) {
				t.Fatalf("tokens=%+v diagnostics=%+v", tokens, diagnostics)
			}
		})
	}
}

func TestLexHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Lex(ctx, []byte("<?php "+strings.Repeat("$x = 1; ", 1000))); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want context cancellation", err)
	}
}

func TestLexerStopsLongCommentScanWhenContextIsCanceled(t *testing.T) {
	content := []byte("/*" + strings.Repeat("x", 1<<20))
	lexer := &lexer{ctx: cancelOnFirstCheckContext{}, content: content, line: 1, inPHP: true}
	lexer.scanPHP()
	if lexer.offset >= len(content) {
		t.Fatalf("comment scan consumed canceled input: offset=%d length=%d", lexer.offset, len(content))
	}
}

type cancelOnFirstCheckContext struct{}

func (cancelOnFirstCheckContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (cancelOnFirstCheckContext) Done() <-chan struct{}       { return nil }
func (cancelOnFirstCheckContext) Err() error                  { return context.Canceled }
func (cancelOnFirstCheckContext) Value(any) any               { return nil }

func assertTokenKindText(t *testing.T, tokens []Token, kind Kind, text string) {
	t.Helper()
	for _, token := range tokens {
		if token.Kind == kind && token.Text == text {
			return
		}
	}
	t.Fatalf("token %s %q not found in %+v", kind, text, tokens)
}

func assertTokenKindPrefix(t *testing.T, tokens []Token, kind Kind, prefix string) {
	t.Helper()
	for _, token := range tokens {
		if token.Kind == kind && strings.HasPrefix(token.Text, prefix) {
			return
		}
	}
	t.Fatalf("token %s with prefix %q not found in %+v", kind, prefix, tokens)
}

func countTokenKind(tokens []Token, kind Kind) int {
	count := 0
	for _, token := range tokens {
		if token.Kind == kind {
			count++
		}
	}
	return count
}

func countTokenText(tokens []Token, text string) int {
	count := 0
	for _, token := range tokens {
		if token.Text == text {
			count++
		}
	}
	return count
}

func hasDiagnosticCode(diagnostics []model.Diagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

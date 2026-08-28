# PHP Structural Extraction Design

Date: 2026-08-28

## Goal

Add first-class, pure-Go PHP extraction to FocalSpan without changing the
SQLite schema, CLI/MCP contracts, token-budget contract, or CGO-free build
boundary. The extractor is deliberately structural and error-tolerant. It
must produce useful source spans and conservative lexical relations without
executing PHP, Composer, repository code, or an external parser.

## Selected approach

The implementation uses a new `internal/extract/php` package with four small
responsibilities:

- `lexer.go` scans the complete byte stream and emits stateful PHP, HTML,
  trivia, string, comment, heredoc/nowdoc, identifier, operator, and
  punctuation tokens with UTF-8 byte and one-based line spans.
- `parser.go` consumes tokens with balanced-brace and declaration-aware
  recovery. It tracks namespace blocks, imports/aliases, class scope, method
  scope, attributes, and declaration boundaries. Unknown tokens are retained
  rather than treated as fatal syntax errors.
- `builder.go` converts parser nodes to model symbols, chunks, relations, and
  diagnostics. It resolves only references that are unambiguous from the
  current file; all other targets remain short canonical `UnresolvedTo`
  values.
- `extractor.go` owns the public `extract.Extractor` implementation and
  coordinates cancellation, sorting, fallback windows, and deterministic
  output.

Extending `generic` was rejected because it cannot preserve PHP lexical state
or declaration relationships. An external PHP parser was rejected by the
task constraints and would add runtime/dependency behavior outside the
product boundary.

## Language detection

`repository.DetectLanguage` remains path-only and case-insensitive for
backward compatibility. `.php`, `.phtml`, `.php3`, `.php4`, `.php5`, `.php7`,
`.php8`, and `.phps` return `php`; `.inc` remains `text` in the path-only
function. A new `DetectLanguageContent(path, content)` delegates to the
path-only result except for `.inc` and recognizes, case-insensitively, the
first PHP opening tag among `<?php`, `<?=`, and `<?`. `<?xml` and
`<?xml-stylesheet` are explicitly excluded, so an XML-only `.inc` remains
`text`. The scanner calls the content-aware function after reading and
validating the file.

## Lexer and spans

The lexer has PHP and inline-HTML modes. It recognizes PHP opening/closing
tags, identifiers, variables, keywords, punctuation, operators, whitespace,
line comments (`//` and `#`), block/doc comments, single/double/backtick
strings, heredoc, nowdoc, and inline HTML. It carries block-comment,
string, and heredoc state over line boundaries and accepts LF and CRLF without
counting CR as a separate line. Escapes, braces in strings/comments,
attributes (`#[...]`), multiple PHP blocks, and UTF-8 text are handled before
ordinary punctuation is considered.

Every token uses half-open `[StartByte, EndByte)` UTF-8 byte offsets and
one-based inclusive `StartLine`/`EndLine`. The lexer periodically checks the
context and returns `ctx.Err()` on cancellation. Unclosed strings, comments,
or heredocs produce tokens through EOF plus bounded diagnostics; they never
panic or discard the file.

## Symbols and names

The parser creates namespace, class, interface, trait, enum, named function,
method, property, and constant symbols. It recognizes public/protected/private,
static, abstract, final, and readonly modifiers, multiline signatures,
attributes, doc comments, reference returns, nullable/union/intersection
types, constructor promotion, and semicolon-terminated abstract/interface
methods. Anonymous functions and arrow functions are not symbols; their
tokens remain in the owning function/procedural chunk.

Canonical names use `\` separators and omit a leading global `\`:

- `App\\Auth\\TokenService`
- `App\\Auth\\TokenService::validate`
- `App\\Auth\\validate_token`

Symbol handles use `model.HandleAllocator` and path/language/kind/canonical
name/normalized signature identity, never line numbers. A method's
`ParentHandle` is its class handle. Class-like and member relations use
`contains`.

## Chunks and coverage

Class-like symbols receive `class-outline`, `interface-outline`,
`trait-outline`, or `enum-outline` chunks containing attributes, doc comment,
modifiers, declaration header, inheritance clauses, and the opening-brace
boundary. Property and constant outlines are separate bounded chunks, which
keeps their byte ranges exact and prevents method bodies from being copied
into the class outline. Named functions and methods receive separate
`function`, `method`, or `test` chunks. Abstract/interface methods contain
their signatures only.

Top-level executable content and include/require statements are stored in
bounded `procedural` chunks. Inline HTML outside named declarations is stored
in bounded `template` chunks. The extractor computes useful token intervals
outside named declaration chunks and emits line-bounded fallback windows for
uncovered material; comments and whitespace alone do not create chunks.
Large windows use the configured generic 80-line/10-line-overlap policy and
never exceed the existing safe line bound. No unconditional full-file chunk
is added alongside named source chunks.

## Relations and conservative resolution

The extractor emits `contains`, `imports`, `references`, `calls`, `tests`,
and trait-use `references` relations. Local function calls, `$this->method`,
`self::method`, `static::method`, matching local aliases, and unambiguous
same-file named declarations receive `ToHandle`. Receiver-unknown calls,
dynamic calls, variable class names, magic methods, and service-container
lookups stay unresolved with the terminal name (`validateToken`) and a short
lexical `Source`.

Namespace `use`, grouped imports, function/constant imports, and aliases are
recorded in a per-file alias table. A class-scope `use SomeTrait` is always a
trait-use `references` relation, never a namespace import. Extends,
implements, parameter/return/property types, attributes, `new`, and static
class references produce conservative `references` relations.

Static include/require expressions are reduced only from quoted strings,
`__DIR__`, `__FILE__`, `dirname(__FILE__)`, and concatenation. Normalized
repository-relative paths are used when they remain inside the root; dynamic
or escaping expressions remain short unresolved values and are never linked
to a guessed file. Existing containment/path utilities and an index-time
root check guard persisted include targets. No schema migration is needed.

The existing store relation table remains authoritative. Local handles are
stored in `ToHandle`; canonical cross-file names and safe include paths are
stored in `UnresolvedTo`. Store-side relation lookup is extended to match
unresolved names against `symbols.name`, `symbols.qualified_name`, and safe
repository file paths. This preserves Go unresolved relation behavior while
allowing PHP `imports`, `references`, `callers`, and `tests` expansion.

## PHPUnit recognition

A class is PHPUnit-oriented when its resolved `extends` target is
`PHPUnit\\Framework\\TestCase` or an imported/aliased `TestCase`. Methods
whose names begin with `test` or that carry a `Test` attribute are emitted as
kind `test`. Calls from test methods produce `tests` relations in addition to
conservative target information, including unresolved simple names so the
existing query and expansion paths can find them.

## Error recovery

Diagnostics use bounded messages and codes such as
`php_unclosed_string`, `php_unclosed_comment`, `php_unclosed_heredoc`,
`php_unbalanced_brace`, `php_malformed_declaration`, and
`php_partial_extraction`. A malformed later declaration does not remove
earlier symbols or chunks. The parser closes open scopes at EOF, preserves
already-built declarations, and emits fallback chunks for useful uncovered
source. Cancellation is the only extraction condition returned as an error;
ordinary PHP parse problems remain indexable with diagnostics.

## Verification and documentation

Tests cover detection, lexer state and spans, declaration/chunk extraction,
relations, error recovery, cancellation, registry ordering, store lookup, and
existing Go behavior. A checked-in `testdata/repos/phpsample` fixture and
PHP evaluation cases exercise expired-token search, callers, PHPUnit tests,
static `.inc` bootstrap inclusion, mixed HTML/PHP, and unrelated large-file
avoidance. Existing Go cases are run unchanged and PHP metrics are reported
separately.

README, design, implementation plan, and evaluation documentation will state
that PHP is first-class structural extraction, `.inc` detection is
content-aware, mixed HTML/PHP is searchable, PHP source is stored in SQLite,
and complete type inference, dynamic dispatch, service-container resolution,
Composer, and PHP runtime execution are out of scope.


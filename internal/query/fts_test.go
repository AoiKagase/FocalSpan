package query

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestBuildFTSEscapesSyntax(t *testing.T) {
	got := BuildFTS(Normalize(`foo" OR * NEAR(bar) (unsafe)`))
	if got == "" {
		t.Fatal("expected a safe lexical query")
	}
	if strings.Contains(got, ` OR * `) || strings.Contains(got, "NEAR(") || strings.Contains(got, "(") {
		t.Fatalf("raw FTS syntax leaked: %q", got)
	}
	if !strings.Contains(got, `"foo"`) {
		t.Fatalf("safe token missing: %q", got)
	}
}

func TestBuildFTSIsBoundedAndDeterministic(t *testing.T) {
	terms := Terms{
		Words:       []string{"beta", "alpha", "alpha"},
		Identifiers: []string{"Gamma", "alpha"},
		Phrases:     []string{"exact phrase"},
		UnicodeRuns: []string{"日本語"},
	}
	first := BuildFTS(terms)
	second := BuildFTS(terms)
	if first != second {
		t.Fatalf("BuildFTS is not deterministic: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, `"exact phrase"`) {
		t.Fatalf("phrases should be retained first: %q", first)
	}
	if strings.Count(first, `"`) > maxFTSTerms*2 {
		t.Fatalf("FTS query exceeds term cap: %q", first)
	}
}

func TestBuildFTSIsAcceptedBySQLiteForUntrustedInputs(t *testing.T) {
	db, err := sql.Open("sqlite", "file:query-fts-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `CREATE VIRTUAL TABLE query_fts USING fts5(content)`); err != nil {
		t.Fatal(err)
	}
	inputs := []string{`unmatched " quote`, `parentheses (unsafe)`, `*`, `field:value`, `日本語🚀`, `src\\Auth\\Token.php`, ""}
	for _, input := range inputs {
		query := BuildFTS(Normalize(input))
		if query == "" {
			continue
		}
		if _, err := db.ExecContext(context.Background(), `SELECT rowid FROM query_fts WHERE query_fts MATCH ?`, query); err != nil {
			t.Fatalf("input=%q query=%q: %v", input, query, err)
		}
	}
}

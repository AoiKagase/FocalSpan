package rust

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestTestAttributeRequiresAttributePath(t *testing.T) {
	for _, tc := range []struct {
		attribute string
		isTest    bool
	}{{"#[cfg(not(test))]", false}, {"#[cfg(test)]", false}, {"#[test]", true}, {"#[tokio::test]", true}, {"#[cfg_attr(test, inline)]", false}} {
		got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.rs", Language: "rust", Content: []byte(tc.attribute + "\nfn production() {}\n")})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, s := range got.Symbols {
			if s.Name == "production" {
				found = true
				if (s.Kind == "test") != tc.isTest {
					t.Fatalf("attribute=%s kind=%s", tc.attribute, s.Kind)
				}
			}
		}
		if !found {
			t.Fatal("function missing")
		}
	}
}

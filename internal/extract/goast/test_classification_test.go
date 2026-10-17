package goast

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"testing"
)

func TestProductionFunctionTestPrefixIsNotTest(t *testing.T) {
	for _, path := range []string{"connection.go", "connection_test.go"} {
		got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: path, Language: "go", Content: []byte("package p\nfunc TestConnection() bool {return true}\n")})
		if err != nil {
			t.Fatal(err)
		}
		s := findSymbol(got.Symbols, "TestConnection")
		if s == nil || s.Kind == "test" {
			t.Fatalf("ordinary function mislabeled: %+v", s)
		}
	}
}

func TestGoTestDeclarationSignatureControls(t *testing.T) {
	for _, tc := range []struct {
		declaration string
		want        bool
	}{{"func TestValid(t *testing.T) {}", true}, {"func Testlower(t *testing.T) {}", false}, {"func TestBad(t *testing.B) {}", false}, {"func TestMany(t,u *testing.T) {}", false}, {"func (s *Service) TestMethod(t *testing.T) {}", false}, {"func TestResult(t *testing.T) bool {return true}", false}, {"func FuzzValid(f *testing.F) {}", true}, {"func BenchmarkValid(b *testing.B) {}", true}, {"func ExampleValid() {}", true}, {"func TestAlias(t *tt.T) {}", true}} {
		got, err := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a_test.go", Language: "go", Content: []byte("package p\nimport tt \"testing\"\n" + tc.declaration + "\n")})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, s := range got.Symbols {
			if s.Kind == "function" || s.Kind == "method" || s.Kind == "test" {
				found = true
				if (s.Kind == "test") != tc.want {
					t.Fatalf("%s kind=%s", tc.declaration, s.Kind)
				}
			}
		}
		if !found {
			t.Fatal("missing declaration")
		}
	}
}

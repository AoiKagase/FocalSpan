package python

import (
	"context"
	"github.com/focalspan/focalspan/internal/model"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestR08SemicolonImports(t *testing.T) {
	source, e := os.ReadFile("../../../testdata/regressions/residual11/python-imports.py")
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		code string
		want []string
	}{
		{string(source), []string{"another", "optional", "os", "pkg.one", "pkg.two", "real", "sys"}},
		{"import os, sys; x = 1\n", []string{"os", "sys"}},
		{"from pkg import (\n one,\n two,\n); import os\n", []string{"os", "pkg.one", "pkg.two"}},
		{"import os, \\\n sys; import json\n", []string{"json", "os", "sys"}},
		{"# import phantom; import fake\ntext = '; import phantom'\nimport real\n", []string{"real"}},
	} {
		x, e := NewExtractor().Extract(context.Background(), model.SourceFile{Path: "a.py", Language: "python", Content: []byte(tc.code)})
		if e != nil {
			t.Fatal(e)
		}
		got := []string{}
		for _, r := range x.Relations {
			if r.Kind == "imports" {
				got = append(got, r.UnresolvedTo)
			}
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: imports=%v want %v", tc.code, got, tc.want)
		}
		for _, c := range x.Chunks {
			if c.EndByte > c.StartByte && c.Content != tc.code[c.StartByte:c.EndByte] {
				t.Errorf("source changed: %+v", c)
			}
		}
	}
}
